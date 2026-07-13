package che

// [>] 🤖🤖

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

// Bin is che's binary name, carried in per-run backup archive filenames.
const Bin = "che"

// archiveBefore snapshots every existing dest into one per-run .tar.bz2 under
// the XDG backups dir before a mutating op runs (sub = op identity).
func (p *ProfileReady) archiveBefore(sub string, dests []string) error {
	ts := time.Now().Format(fsutil.TsLayout)
	path := fsutil.ResolveBackupArchivePath(p.home, Bin, sub, ts)
	return p.mutate("archive", path, func() error { return p.FS.ArchiveDestinations(path, dests) })
}

// failItem logs "<op>(fail): <dest>: <err>" and returns err, the per-item
// continue-on-error hook: ops collect these and errors.Join at the end.
func (p *ProfileReady) failItem(op, dest string, err error) error {
	p.logMsg(op+"(fail)", dest+": "+err.Error())
	return err
}

// makeDirs creates the profile's dirs, one list: items without Dests are
// repo-tree ancestor dirs (path in Rel, parents first, umask mode), items with
// Dests are makeDirs extra-dirs (perms applied, -p).
func (p *ProfileReady) makeDirs(dirs []spec.FileItem) error {
	var errs []error
	for _, item := range dirs {
		if len(item.Dests) == 0 {
			errs = append(errs, p.ensureConfigDir(item.Rel))
			continue
		}
		dest := p.toDest(item.Dests[0].Path)
		if err := p.upsertExtraDir(item, dest); err != nil {
			errs = append(errs, p.failItem("make-dirs", dest, err))
		}
	}
	return errors.Join(errs...)
}

func (p *ProfileReady) upsertExtraDir(item spec.FileItem, dest string) error {
	if fsutil.IsDirSettled(p.Reader, dest, p.isDryRunAll()) {
		return p.fixPerms("make-dirs", dest, item)
	}
	return p.makeExtraDir(item, dest)
}

func (p *ProfileReady) ensureConfigDir(relativePath string) error {
	dest := p.toDest(relativePath)
	if fsutil.IsDirSettled(p.Reader, dest, p.isDryRunAll()) {
		return nil
	}
	err := p.mutate("make-dirs(create)", dest, func() error { return p.FS.MakeDir(dest, 0, false) })
	if err != nil {
		return p.failItem("make-dirs", dest, err)
	}
	return nil
}

// makeExtraDir creates one extra-dir with -p. Owner applied via chown (not mkdir
// -u). Mode 0 -> umask. Set-bits (>0777) reapplied via chmod since mkdir -m may
// drop them.
func (p *ProfileReady) makeExtraDir(item spec.FileItem, dest string) error {
	mode, _ := fsutil.ParseMode(item.Chmod)
	err := p.mutate("make-dirs(create)", dest, func() error { return p.FS.MakeDir(dest, mode, true) })
	if err != nil {
		return err
	}
	if mode > 0o777 {
		if err := p.chmod("make-dirs(chmod)", mode, dest); err != nil {
			return err
		}
	}
	return p.chownIfSet("make-dirs(chown)", item, dest)
}

func (p *ProfileReady) chmod(title string, mode os.FileMode, dest string) error {
	arg := fsutil.FormatModeArg(mode)
	return p.mutate(title, arg+" "+dest, func() error { return p.FS.ChangeMode(arg, dest) })
}

func (p *ProfileReady) chown(title, owner, dest string) error {
	return p.mutate(title, owner+" "+dest, func() error { return p.FS.ChangeOwner(owner, dest) })
}

func (p *ProfileReady) chownIfSet(title string, item spec.FileItem, dest string) error {
	if owner := formatOwnerSpec(item); owner != "" {
		return p.chown(title, owner, dest)
	}
	return nil
}

// fixPerms applies spec mode/owner to an existing dest when they drift, labeling
// the fixes with the owning op ("<op>(chmod)" / "<op>(chown)"). In dry-run=delta
// these lines report the drift; off they correct it. A settled dest (no drift)
// emits nothing. Dry-run=all never reaches here (dests are re-created).
func (p *ProfileReady) fixPerms(op, dest string, item spec.FileItem) error {
	needChmod, needChown := fsutil.DetectPermsDrift(p.Reader, dest, item.Chmod, formatOwnerSpec(item))
	if needChmod {
		mode, _ := fsutil.ParseMode(item.Chmod)
		if err := p.chmod(op+"(chmod)", mode, dest); err != nil {
			return err
		}
	}
	if needChown {
		if err := p.chown(op+"(chown)", formatOwnerSpec(item), dest); err != nil {
			return err
		}
	}
	return nil
}

// runFileOp is the shared shape of the archiving file ops: ensure config dirs,
// archive every dest upfront (failure aborts), then settle each item/dest pair.
func (p *ProfileReady) runFileOp(archiveSub, failOp string, dirRelativePaths []string, items []spec.FileItem,
	destsOf func(spec.FileItem) []string, settle func(spec.FileItem, string) error,
) error {
	var errs []error
	for _, relativePath := range dirRelativePaths {
		errs = append(errs, p.ensureConfigDir(relativePath))
	}
	var dests []string
	for _, item := range items {
		dests = append(dests, destsOf(item)...)
	}
	if err := p.archiveBefore(archiveSub, dests); err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, item := range items {
		for _, dest := range destsOf(item) {
			if err := settle(item, dest); err != nil {
				errs = append(errs, p.failItem(failOp, dest, err))
			}
		}
	}
	return errors.Join(errs...)
}

// makeLinks symlinks each config into its live dest (ln -fhs), archiving existing
// dests upfront, skipping links already pointing into the repo.
func (p *ProfileReady) makeLinks(links []spec.FileItem, dirRelativePaths []string) error {
	return p.runFileOp("make-links", "make-links", dirRelativePaths, links,
		func(item spec.FileItem) []string { return []string{p.toDest(spec.LinkDestRel(item))} },
		p.makeLink)
}

func (p *ProfileReady) makeLink(item spec.FileItem, dest string) error {
	src := p.resolveSrc(item.Rel)
	if fsutil.IsLinkSettled(p.Reader, src, dest, p.isDryRunAll()) {
		return nil
	}
	return p.mutate("make-links(create)", dest, func() error { return p.FS.MakeSymlink(src, dest) })
}

// makeCopies copies each *.ontoHost.cp to its dest(s) (marker stripped, or explicit
// dest) when contents differ, archiving existing dests upfront, applying spec
// perms (else default).
func (p *ProfileReady) makeCopies(copies []spec.FileItem, dirRelativePaths []string) error {
	return p.runFileOp("make-copies", "make-copies", dirRelativePaths, copies, p.resolveCopyDests, p.makeCopy)
}

func (p *ProfileReady) makeCopy(item spec.FileItem, dest string) error {
	src := p.resolveSrc(item.Rel)
	if !p.isDryRunAll() && fsutil.IsSameContent(p.Reader, src, dest) {
		return p.fixPerms("make-copies", dest, item)
	}
	mode, _ := fsutil.ParseMode(item.Chmod)
	err := p.mutate("make-copies(create)", dest, func() error { return p.FS.CopyFile(src, dest, mode) })
	if err != nil {
		return err
	}
	return p.chownIfSet("make-copies(chown)", item, dest)
}

// resolveCopyDests returns the explicit dests (~/ resolved), else the marker-stripped derived dest.
func (p *ProfileReady) resolveCopyDests(item spec.FileItem) []string {
	if len(item.Dests) == 0 {
		return []string{p.toDest(strings.TrimSuffix(item.Rel, spec.CpExt))}
	}
	out := make([]string, len(item.Dests))
	for i, d := range item.Dests {
		out[i] = p.expandHome(d.Path)
	}
	return out
}

// formatOwnerSpec combines owner + owner-group into "owner:group" for fs.Chown ("" -> no chown).
func formatOwnerSpec(item spec.FileItem) string {
	if item.Owner == "" {
		return ""
	}
	if item.OwnerGroup == "" {
		return item.Owner
	}
	return item.Owner + ":" + item.OwnerGroup
}

// pruneBrokenLinks removes broken symlinks in config-set dirs (live dests).
func (p *ProfileReady) pruneBrokenLinks(dirRelativePaths []string) error {
	p.logMsg("prune-links", p.resolveRoot())
	var errs []error
	seen := map[string]bool{}
	for _, relativePath := range dirRelativePaths {
		dest := p.toDest(relativePath)
		if seen[dest] {
			continue
		}
		seen[dest] = true
		entries, derr := p.Reader.ReadDirectory(dest)
		if derr != nil {
			continue // [why] dir may not exist on host yet
		}
		for _, e := range entries {
			path := filepath.Join(dest, e.Name())
			if !p.isBrokenRepoLink(path) {
				continue
			}
			err := p.mutate("rm", path, func() error { return p.FS.RemoveFile(path) })
			if err != nil {
				errs = append(errs, p.failItem("prune-links", path, err))
			}
		}
	}
	return errors.Join(errs...)
}

// isBrokenRepoLink: path is a symlink into root/ whose target is gone.
func (p *ProfileReady) isBrokenRepoLink(path string) bool {
	target, err := p.Reader.ReadLink(path) // [what] non-symlink -> err
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	target = filepath.Clean(target)
	if !fsutil.IsUnder(target, p.resolveRoot()) {
		return false
	}
	_, err = p.Reader.StatPath(path) // [what] broken
	return err != nil
}

// [<] 🤖🤖
