package host

// [>] 🤖🤖

import (
	"bytes"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

// Bin is che's binary name, carried in per-run backup archive filenames.
const Bin = "che"

// archiveBefore snapshots every existing dest into one per-run .tar.bz2 under
// the XDG backups dir before a mutating op runs (sub = op identity).
func (h Host) archiveBefore(sub string, dests []string) error {
	ts := time.Now().Format(fsutil.TsLayout)
	path := fsutil.BackupArchivePath(h.Home, Bin, sub, ts)
	return h.mutate("archive", path, func() error { return h.fs.ArchiveDests(path, dests) })
}

// failItem logs "<op>(fail): <dest>: <err>" and returns err, the per-item
// continue-on-error hook: ops collect these and errors.Join at the end.
func (h Host) failItem(op, dest string, err error) error {
	h.log(op+"(fail)", dest+": "+err.Error())
	return err
}

// MkDirs creates the profile's dirs, one list: items without Dests are
// repo-tree ancestor dirs (path in Rel, parents first, umask mode), items with
// Dests are mkdirs extra-dirs (perms applied, -p).
func (h Host) MkDirs(dirs []spec.FileItem) error {
	var errs []error
	for _, item := range dirs {
		if len(item.Dests) == 0 {
			errs = append(errs, h.ensureConfigDir(item.Rel))
			continue
		}
		dest := h.ToDest(item.Dests[0].Path)
		if err := h.upsertExtraDir(item, dest); err != nil {
			errs = append(errs, h.failItem("mkdir", dest, err))
		}
	}
	return errors.Join(errs...)
}

func (h Host) upsertExtraDir(item spec.FileItem, dest string) error {
	if h.isDirSettled(dest) {
		return h.fixPerms("mkdir", dest, item)
	}
	return h.mkExtraDir(item, dest)
}

// ensureConfigDirs creates repo-tree ancestor dirs (parents first), no spec
// perms: mode 0 -> mkdir honors umask.
func (h Host) ensureConfigDirs(dirRels []string) error {
	var errs []error
	for _, rel := range dirRels {
		errs = append(errs, h.ensureConfigDir(rel))
	}
	return errors.Join(errs...)
}

func (h Host) ensureConfigDir(rel string) error {
	dest := h.ToDest(rel)
	if h.isDirSettled(dest) {
		return nil
	}
	err := h.mutate("mkdir(create)", dest, func() error { return h.fs.Mkdir(dest, 0, false) })
	if err != nil {
		return h.failItem("mkdir", dest, err)
	}
	return nil
}

// isDirSettled reports whether dest already exists as a dir and may be skipped
// (dry-run=all forces every dest to report, so it never skips).
func (h Host) isDirSettled(dest string) bool {
	if h.cfg.DryRun == options.DryRun.All {
		return false
	}
	fi, err := h.reader.Stat(dest)
	return err == nil && fi.IsDir()
}

// mkExtraDir creates one extra-dir with -p. Owner applied via chown (not mkdir
// -u). Mode 0 -> umask. Set-bits (>0777) reapplied via chmod since mkdir -m may
// drop them.
func (h Host) mkExtraDir(item spec.FileItem, dest string) error {
	mode, _ := parseMode(item.Chmod)
	err := h.mutate("mkdir(create)", dest, func() error { return h.fs.Mkdir(dest, mode, true) })
	if err != nil {
		return err
	}
	if mode > 0o777 {
		if err := h.chmod("mkdir(chmod)", mode, dest); err != nil {
			return err
		}
	}
	return h.chownIfSet("mkdir(chown)", item, dest)
}

func (h Host) chmod(title string, mode os.FileMode, dest string) error {
	arg := fsutil.ModeArg(mode)
	return h.mutate(title, arg+" "+dest, func() error { return h.fs.Chmod(arg, dest) })
}

func (h Host) chown(title, owner, dest string) error {
	return h.mutate(title, owner+" "+dest, func() error { return h.fs.Chown(owner, dest) })
}

func (h Host) chownIfSet(title string, item spec.FileItem, dest string) error {
	if owner := ownerSpec(item); owner != "" {
		return h.chown(title, owner, dest)
	}
	return nil
}

// fixPerms applies spec mode/owner to an existing dest when they drift, labeling
// the fixes with the owning op ("<op>(chmod)" / "<op>(chown)"). In dry-run=delta
// these lines report the drift; off they correct it. A settled dest (no drift)
// emits nothing. Dry-run=all never reaches here (dests are re-created).
func (h Host) fixPerms(op, dest string, item spec.FileItem) error {
	needChmod, needChown := h.permsDrift(dest, item)
	if needChmod {
		mode, _ := parseMode(item.Chmod)
		if err := h.chmod(op+"(chmod)", mode, dest); err != nil {
			return err
		}
	}
	if needChown {
		if err := h.chown(op+"(chown)", ownerSpec(item), dest); err != nil {
			return err
		}
	}
	return nil
}

// permsDrift reports whether dest's live mode/owner differ from the spec. Only
// spec-set fields are enforced (empty Chmod/owner -> no drift). Missing dest ->
// no drift (the create path handles it).
func (h Host) permsDrift(dest string, item spec.FileItem) (needChmod, needChown bool) {
	fi, err := h.reader.Lstat(dest)
	if err != nil {
		return false, false
	}
	if mode, ok := parseMode(item.Chmod); ok {
		mask := modeMask(mode)
		needChmod = mode&mask != unixMode(fi.Mode())&mask
	}
	if owner := ownerSpec(item); owner != "" {
		needChown = isOwnerDrifted(fi, owner)
	}
	return needChmod, needChown
}

// modeMask is the raw-unix bit set the spec controls: perm bits always, plus
// setuid/setgid/sticky when the spec mode carries them (>0777, matching mkExtraDir).
func modeMask(mode os.FileMode) os.FileMode {
	if mode > 0o777 {
		return 0o7777
	}
	return 0o777
}

// unixMode maps an os.FileMode's Go-encoded special bits (ModeSetuid/Setgid/
// Sticky live in high bits, not 0o7000) down to raw-unix perm+special bits, so
// it compares equal to a parseMode octal like 0o2775. Perm bits pass through.
func unixMode(m os.FileMode) os.FileMode {
	u := m.Perm()
	if m&os.ModeSetuid != 0 {
		u |= 0o4000
	}
	if m&os.ModeSetgid != 0 {
		u |= 0o2000
	}
	if m&os.ModeSticky != 0 {
		u |= 0o1000
	}
	return u
}

// isOwnerDrifted reports whether fi's live uid/gid differ from the "owner[:group]"
// spec. Unresolvable spec names or a missing Stat_t -> no drift (can't compare).
func isOwnerDrifted(fi os.FileInfo, owner string) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	name, group, _ := strings.Cut(owner, ":")
	uid, uidOK := lookupID(name, fsutil.UserLookup, func(u *user.User) string { return u.Uid })
	gid, gidOK := lookupID(group, fsutil.GroupLookup, func(g *user.Group) string { return g.Gid })
	return (uidOK && uid != st.Uid) || (gidOK && gid != st.Gid)
}

// lookupID resolves name to a numeric id: empty or unresolvable -> ok=false (no drift).
func lookupID[T any](name string, lookup func(string) (T, error), idOf func(T) string) (uint32, bool) {
	if name == "" {
		return 0, false
	}
	rec, err := lookup(name)
	if err != nil {
		return 0, false
	}
	id, err := strconv.ParseUint(idOf(rec), 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(id), true
}

// runFileOp is the shared shape of the archiving file ops: ensure config dirs,
// archive every dest upfront (failure aborts), then settle each item/dest pair.
func (h Host) runFileOp(archiveSub, failOp string, dirRels []string, items []spec.FileItem,
	destsOf func(spec.FileItem) []string, settle func(spec.FileItem, string) error,
) error {
	errs := []error{h.ensureConfigDirs(dirRels)}
	var dests []string
	for _, item := range items {
		dests = append(dests, destsOf(item)...)
	}
	if err := h.archiveBefore(archiveSub, dests); err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, item := range items {
		for _, dest := range destsOf(item) {
			if err := settle(item, dest); err != nil {
				errs = append(errs, h.failItem(failOp, dest, err))
			}
		}
	}
	return errors.Join(errs...)
}

// MkLinks symlinks each config into its live dest (ln -fhs), archiving existing
// dests upfront, skipping links already pointing into the repo.
func (h Host) MkLinks(links []spec.FileItem, dirRels []string) error {
	return h.runFileOp("link", "ln", dirRels, links,
		func(item spec.FileItem) []string { return []string{h.ToDest(spec.LinkDestRel(item))} },
		h.linkOne)
}

func (h Host) linkOne(item spec.FileItem, dest string) error {
	src := h.Src(item.Rel)
	if h.isLinkSettled(src, dest) {
		return nil
	}
	return h.mutate("ln(create)", dest, func() error { return h.fs.Symlink(src, dest) })
}

// isLinkSettled reports whether dest already resolves to src (skippable). Dry-run=all
// forces every dest to report, so it never skips.
func (h Host) isLinkSettled(src, dest string) bool {
	if h.cfg.DryRun == options.DryRun.All {
		return false
	}
	destResolved, err := h.reader.EvalSymlinks(dest)
	if err != nil {
		return false
	}
	srcResolved, err := h.reader.EvalSymlinks(src)
	return err == nil && destResolved == srcResolved
}

// MkCopies copies each *.ontoHost.cp to its dest(s) (marker stripped, or explicit
// dest) when contents differ, archiving existing dests upfront, applying spec
// perms (else default).
func (h Host) MkCopies(copies []spec.FileItem, dirRels []string) error {
	return h.runFileOp("copy", "cp", dirRels, copies, h.copyDests, h.copyOne)
}

func (h Host) copyOne(item spec.FileItem, dest string) error {
	src := h.Src(item.Rel)
	if h.cfg.DryRun != options.DryRun.All && h.isSameContent(src, dest) {
		return h.fixPerms("cp", dest, item)
	}
	mode, _ := parseMode(item.Chmod)
	err := h.mutate("cp(create)", dest, func() error { return h.fs.Copy(src, dest, mode) })
	if err != nil {
		return err
	}
	return h.chownIfSet("cp(chown)", item, dest)
}

// copyDests returns the explicit dests (~/ resolved), else the marker-stripped derived dest.
func (h Host) copyDests(item spec.FileItem) []string {
	if len(item.Dests) == 0 {
		return []string{h.ToDest(strings.TrimSuffix(item.Rel, spec.CpExt))}
	}
	out := make([]string, len(item.Dests))
	for i, d := range item.Dests {
		out[i] = h.expandHome(d.Path)
	}
	return out
}

// ownerSpec combines owner + owner-group into "owner:group" for fs.Chown ("" -> no chown).
func ownerSpec(item spec.FileItem) string {
	if item.Owner == "" {
		return ""
	}
	if item.OwnerGroup == "" {
		return item.Owner
	}
	return item.Owner + ":" + item.OwnerGroup
}

// parseMode parses an octal chmod string ("" -> not set).
func parseMode(s string) (os.FileMode, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, false
	}
	return os.FileMode(n), true
}

// expandHome expands env vars ($HOME bound to h.Home) then the ~/ prefix, so a
// dest may be written with $VAR / $HOME or ~/.
func (h Host) expandHome(p string) string {
	return fsutil.ExpandHome(h.expandEnv(p), h.Home)
}

func (h Host) isSameContent(a, b string) bool {
	x, err := h.reader.ReadFile(a)
	if err != nil {
		return false
	}
	y, err := h.reader.ReadFile(b)
	if err != nil {
		return false
	}
	return bytes.Equal(x, y)
}

// PruneBrokenLinks removes broken symlinks in config-set dirs (live dests).
func (h Host) PruneBrokenLinks(dirRels []string) error {
	h.log("prune-links", h.Root)
	var errs []error
	seen := map[string]bool{}
	for _, rel := range dirRels {
		dest := h.ToDest(rel)
		if seen[dest] {
			continue
		}
		seen[dest] = true
		entries, derr := h.reader.ReadDir(dest)
		if derr != nil {
			continue // [why] dir may not exist on host yet
		}
		for _, e := range entries {
			p := filepath.Join(dest, e.Name())
			if !h.isBrokenRepoLink(p) {
				continue
			}
			err := h.mutate("rm", p, func() error { return h.fs.Remove(p) })
			if err != nil {
				errs = append(errs, h.failItem("prune-links", p, err))
			}
		}
	}
	return errors.Join(errs...)
}

// isBrokenRepoLink: p is a symlink into root/ whose target is gone.
func (h Host) isBrokenRepoLink(p string) bool {
	target, err := h.reader.Readlink(p) // [what] non-symlink -> err
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(p), target)
	}
	target = filepath.Clean(target)
	if !fsutil.IsUnder(target, h.Root) {
		return false
	}
	_, err = h.reader.Stat(p) // [what] broken
	return err != nil
}

// [<] 🤖🤖
