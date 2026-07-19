package che

// [>] 🤖🤖

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/database"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

// Bin is che's binary name, carried in per-run backup archive filenames.
const Bin = "che"

// archiveBefore snapshots every existing dest into one per-run .tar.bz2 under
// the XDG backups dir before a mutating op runs (sub = op identity). The archive
// path/sub are stashed on the profile so the op's following mutate calls
// reference this run's Backup row. Uses the run's shared TsLayout stamp so the
// filename and the ledger run line up. Under `run` the backup stage already
// archived every op dest (spec/che/BackupCmdBehavior.md): the op skips its own
// archive and points its records at the stage archive.
func (p *ProfileReady) archiveBefore(sub string, dests []string) error {
	if p.backedUp {
		p.currentArchive = fsutil.ResolveBackupArchivePath(p.home, Bin, "backup", p.runID)
		p.currentSub = "backup"
		return nil
	}
	path := fsutil.ResolveBackupArchivePath(p.home, Bin, sub, p.runID)
	p.currentArchive = path
	p.currentSub = sub
	return p.mutate("backup", path, "", opInfo{}, func() error { return p.FS.ArchiveDestinations(path, dests) })
}

// backupDests lists the dests the profile's file ops WOULD CHANGE (unsettled
// links, differing copies, differing renders), run order: only they need a
// pre-mutation snapshot, settled dests are untouched.
func (p *ProfileReady) backupDests() []string {
	var out []string
	for _, op := range p.commandOps("backup") {
		switch o := op.(type) {
		case *MakeLinksOperationReady:
			for _, item := range o.Links {
				dest := p.toDest(spec.DestRel(item))
				if !fsutil.IsLinkSettled(p.Reader, p.resolveSrc(item.Rel), dest) {
					out = append(out, dest)
				}
			}
		case *MakeCopiesOperationReady:
			for _, item := range o.Copies {
				src := p.resolveSrc(item.Rel)
				for _, dest := range p.resolveCopyDests(item) {
					if !fsutil.IsSameContent(p.Reader, src, dest) {
						out = append(out, dest)
					}
				}
			}
		case *RenderTemplatesOperationReady:
			for _, item := range o.Templates {
				hash, err := p.mockRenderHash(item)
				for _, d := range p.resolveTemplateDests(item) {
					if !d.host {
						continue
					}
					if err != nil || p.readRenderHash(d.path) != hash {
						out = append(out, d.path)
					}
				}
			}
		}
	}
	return out
}

// existingBackupDests counts the backup dests currently present on the host.
func (p *ProfileReady) existingBackupDests() int {
	n := 0
	for _, dest := range p.backupDests() {
		if _, err := p.Reader.LstatPath(dest); err == nil {
			n++
		}
	}
	return n
}

// ExecBackup archives every existing would-change dest into one per-run backup
// archive and marks the profile backed up, so the following ops skip their own
// archives. The showDelta line always logs (spec/che/BackupCmdBehavior.md);
// the created line reports the written archive with its size, nothing to back
// up writes and logs nothing more.
func (p *ProfileReady) ExecBackup() error {
	defer func() { p.backedUp = true }()
	p.logMsg("backup(showDelta)", p.backupDeltaSummary())
	if p.existingBackupDests() == 0 {
		return nil
	}
	dests := p.backupDests()
	path := fsutil.ResolveBackupArchivePath(p.home, Bin, "backup", p.runID)
	p.currentArchive = path
	p.currentSub = "backup"
	if p.isDryRun() {
		p.logMsg("backup(created)", path)
		return nil
	}
	if err := p.FS.ArchiveDestinations(path, dests); err != nil {
		return err
	}
	p.logMsg("backup(created)", humanSize(archiveSize(path))+", "+path)
	return nil
}

// backupDeltaSummary lists the backed-up file ops with their discover deltas:
// "op(delta),op(delta)".
func (p *ProfileReady) backupDeltaSummary() string {
	var parts []string
	for _, op := range p.commandOps("backup") {
		_, delta := op.counts(p)
		parts = append(parts, fmt.Sprintf("%s(%d)", op.Name(), delta))
	}
	return strings.Join(parts, ",")
}

// archiveSize is the written archive's size in bytes (0 when unreadable, e.g.
// record-only test writers).
func archiveSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

// humanSize renders bytes human-readable (B/KB/MB, one decimal above KB).
func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%dB", n)
}

// ExecBackupStage is ExecBackup as the run sequence's backup stage: announced
// like the other wrapped ops (skippedDue[NoDelta] when nothing needs backing up).
func (p *ProfileReady) ExecBackupStage() error {
	if p.existingBackupDests() == 0 {
		log.Msg(skipTitle("run", "runOp", "NoDelta"), "backup")
	} else {
		log.Msg("run(runOp)", "backup")
	}
	return p.ExecBackup()
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
	if fsutil.IsDirSettled(p.Reader, dest) {
		if p.isDryRunAll() {
			p.logMsg(skipTitle("make-dirs", p.wouldAction(dest), p.skipReasons("AlreadyExists")...), dest)
		}
		return p.fixPerms("make-dirs", dest, item)
	}
	return p.makeExtraDir(item, dest)
}

func (p *ProfileReady) ensureConfigDir(relativePath string) error {
	dest := p.toDest(relativePath)
	if fsutil.IsDirSettled(p.Reader, dest) {
		if p.isDryRunAll() {
			p.logMsg(skipTitle("make-dirs", p.wouldAction(dest), p.skipReasons("AlreadyExists")...), dest)
		}
		return nil
	}
	err := p.mutate("make-dirs(create)", dest, dest, opInfo{kind: "dir"}, func() error { return p.FS.MakeDir(dest, 0, false) })
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
	err := p.mutate("make-dirs(create)", dest, dest, opInfo{kind: "dir", mode: item.Chmod}, func() error { return p.FS.MakeDir(dest, mode, true) })
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
	return p.mutate(title, arg+" "+dest, dest, opInfo{kind: "chmod", mode: arg}, func() error { return p.FS.ChangeMode(arg, dest) })
}

func (p *ProfileReady) chown(title, owner, dest string) error {
	return p.mutate(title, owner+" "+dest, dest, opInfo{kind: "chown", owner: owner}, func() error { return p.FS.ChangeOwner(owner, dest) })
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
// emits nothing, except under dry-run=all, where the already-set state logs
// (spec/che/LogBehavior.md).
func (p *ProfileReady) fixPerms(op, dest string, item spec.FileItem) error {
	needChmod, needChown := fsutil.DetectPermsDrift(p.Reader, dest, item.Chmod, formatOwnerSpec(item))
	if needChmod {
		mode, _ := fsutil.ParseMode(item.Chmod)
		if err := p.chmod(op+"(chmod)", mode, dest); err != nil {
			return err
		}
	} else if p.isDryRunAll() && item.Chmod != "" {
		p.logMsg(skipTitle(op, "chmod", p.skipReasons("AlreadySet")...), dest)
	}
	if needChown {
		if err := p.chown(op+"(chown)", formatOwnerSpec(item), dest); err != nil {
			return err
		}
	} else if p.isDryRunAll() && formatOwnerSpec(item) != "" {
		p.logMsg(skipTitle(op, "chown", p.skipReasons("AlreadySet")...), dest)
	}
	return nil
}

// runFileOp is the shared shape of the archiving file ops: ensure config dirs,
// archive every dest upfront (failure aborts), settle each item/dest pair, then
// ledger-sweep any prior dest of kind this profile no longer produces.
func (p *ProfileReady) runFileOp(archiveSub, failOp, kind string, dirRelativePaths []string, items []spec.FileItem,
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
	if !p.isDryRun() {
		errs = append(errs, p.sweepStale(kind, dests))
	}
	return errors.Join(errs...)
}

// makeLinks symlinks each config into its live dest (ln -fhs), archiving existing
// dests upfront, skipping links already pointing into the repo.
func (p *ProfileReady) makeLinks(links []spec.FileItem, dirRelativePaths []string) error {
	return p.runFileOp("make-links", "make-links", "link", dirRelativePaths, links,
		func(item spec.FileItem) []string { return []string{p.toDest(spec.DestRel(item))} },
		p.makeLink)
}

func (p *ProfileReady) makeLink(item spec.FileItem, dest string) error {
	src := p.resolveSrc(item.Rel)
	if fsutil.IsLinkSettled(p.Reader, src, dest) {
		if p.isDryRunAll() {
			p.logMsg(skipTitle("make-links", p.wouldAction(dest), p.skipReasons("AlreadyLinked")...), dest)
		}
		return nil
	}
	return p.mutate("make-links(create)", dest, dest, opInfo{kind: "link", target: src, srcRel: item.Rel}, func() error { return p.FS.MakeSymlink(src, dest) })
}

// makeCopies copies each *.ontoHost.cp to its dest(s) (marker stripped, or explicit
// dest) when contents differ, archiving existing dests upfront, applying spec
// perms (else default).
func (p *ProfileReady) makeCopies(copies []spec.FileItem, dirRelativePaths []string) error {
	return p.runFileOp("make-copies", "make-copies", "copy", dirRelativePaths, copies, p.resolveCopyDests, p.makeCopy)
}

func (p *ProfileReady) makeCopy(item spec.FileItem, dest string) error {
	src := p.resolveSrc(item.Rel)
	if fsutil.IsSameContent(p.Reader, src, dest) {
		if p.isDryRunAll() {
			p.logMsg(skipTitle("make-copies", p.wouldAction(dest), p.skipReasons("SameContent")...), dest)
		}
		return p.fixPerms("make-copies", dest, item)
	}
	mode, _ := fsutil.ParseMode(item.Chmod)
	err := p.mutate("make-copies(create)", dest, dest, opInfo{kind: "copy", srcRel: item.Rel, mode: item.Chmod}, func() error { return p.FS.CopyFile(src, dest, mode) })
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
	if item.Derived { // glob dest rewrite: derived dest through the host mapping
		return []string{p.toDest(item.Dests[0].Path)}
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

// installedLinks lists the link dests the ledger recorded for this profile.
// Records-off (nil ledger) -> empty.
func (p *ProfileReady) installedLinks() ([]database.OperationDone, error) {
	if p.Ledger == nil || p.profileDone == nil {
		return nil, nil
	}
	installed, err := p.Ledger.InstalledForProfile(p.ref)
	if err != nil {
		return nil, err
	}
	var links []database.OperationDone
	for _, op := range installed {
		if op.Kind == "link" {
			links = append(links, op)
		}
	}
	return links, nil
}

// scanBrokenLinks lists the recorded links whose source (repo file the symlink
// points at) no longer exists.
func (p *ProfileReady) scanBrokenLinks() ([]database.OperationDone, error) {
	links, err := p.installedLinks()
	if err != nil {
		return nil, err
	}
	var broken []database.OperationDone
	for _, op := range links {
		if !p.linkSourcePresent(op) {
			broken = append(broken, op)
		}
	}
	return broken, nil
}

// pruneBrokenLinks removes link dests the ledger recorded for this profile whose
// source (repo file the symlink points at) no longer exists — the ledger, not
// git, is the source of truth. dry-run / records-off -> nothing pruned.
func (p *ProfileReady) pruneBrokenLinks() error {
	if p.Ledger == nil || p.profileDone == nil || p.isDryRun() {
		return nil
	}
	broken, err := p.scanBrokenLinks()
	if err != nil {
		return err
	}
	var errs []error
	for _, op := range broken {
		if err := p.removeStale(op); err != nil {
			errs = append(errs, p.failItem("prune-broken-links", op.Dest, err))
		}
	}
	return errors.Join(errs...)
}

// linkSourcePresent reports whether a recorded link op's source (the symlink
// target it was created with) still exists on disk.
func (p *ProfileReady) linkSourcePresent(op database.OperationDone) bool {
	if op.Target == "" {
		return true // [why] no recorded source: leave it, only prune known-gone
	}
	_, err := p.Reader.StatPath(op.Target)
	return err == nil
}

// [<] 🤖🤖
