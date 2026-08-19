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

func (p *ProfileReady) resolveArchivePath(op string) string {
	return fsutil.ResolveBackupArchivePath(p.home, fsutil.SlugRef(p.ref), op, p.runTs, newID())
}

func (p *ProfileReady) archiveBefore(sub string, dests []string) error {
	if p.opts.BackupAutoCreateDisabled {
		p.currentArchive = ""
		p.currentSub = sub
		return nil
	}
	if p.backedUp {
		p.currentArchive = p.backupArchive // [why] "" when the stage archived nothing: records carry no backup ref
		p.currentSub = "backup"
		return nil
	}
	path := p.resolveArchivePath(sub)
	p.currentArchive = path
	p.currentSub = sub
	return p.mutate("backup", "create", path, "", opInfo{}, func() error { return p.FS.ArchiveDestinations(path, dests) })
}

func (p *ProfileReady) backupDests() []string {
	var out []string
	for _, op := range p.commandOps("backup") {
		if o, ok := op.(interface {
			unsettledDests(*ProfileReady) (int, int, []string)
		}); ok {
			_, _, unsettled := o.unsettledDests(p)
			out = append(out, unsettled...)
		}
	}
	return out
}

func (p *ProfileReady) existingDests(dests []string) int {
	n := 0
	for _, dest := range dests {
		if _, err := p.Reader.LstatPath(dest); err == nil {
			n++
		}
	}
	return n
}

func (p *ProfileReady) ExecBackup() error { return p.execBackup(p.backupDests()) }

func (p *ProfileReady) execBackup(dests []string) error {
	defer func() { p.backedUp = true }()
	defer p.enterOp("backup")()
	p.emit(log.Levels.Info, "backup", "backup-delta", p.describeOpDeltas("backup"))
	if p.existingDests(dests) == 0 {
		return nil
	}
	path := p.resolveArchivePath("backup")
	p.currentArchive = path
	p.currentSub = "backup"
	if p.isDryRun() { // [why] prediction only: no archive is written
		p.emitDryRun("backup", "create", path)
		return nil
	}
	if err := p.FS.ArchiveDestinations(path, dests); err != nil {
		return err
	}
	// [why] the standalone archive carries no op records, so its ledger Backup row is written here
	if _, err := p.Ledger.EnsureBackup(p.specDone, path, "backup"); err != nil {
		log.EmitTrace("ledger", "error", "ensure backup: "+err.Error())
	}
	p.backupArchive = path
	p.emit(log.Levels.Info, "backup", "created", humanSize(archiveSize(path))+", "+path)
	return nil
}

func archiveSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%dB", n)
}

func (p *ProfileReady) failItem(op, dest string, err error) error {
	p.emit(log.Levels.Error, op, "fail", dest+": "+err.Error())
	return err
}

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
			p.emit(log.Levels.Info, "make-dirs", p.wouldAction(dest), dest, "already exists")
		}
		return p.fixPerms("make-dirs", dest, item)
	}
	return p.makeExtraDir(item, dest)
}

func (p *ProfileReady) ensureConfigDir(relativePath string) error {
	dest := p.toDest(relativePath)
	if fsutil.IsDirSettled(p.Reader, dest) {
		if p.isDryRunAll() {
			p.emit(log.Levels.Info, "make-dirs", p.wouldAction(dest), dest, "already exists")
		}
		return nil
	}
	err := p.mutate("make-dirs", "create", dest, dest, opInfo{kind: "dir"}, func() error { return p.FS.MakeDir(dest, 0, false) })
	if err != nil {
		return p.failItem("make-dirs", dest, err)
	}
	return nil
}

func (p *ProfileReady) makeExtraDir(item spec.FileItem, dest string) error {
	mode, _ := fsutil.ParseMode(item.Chmod)
	err := p.mutate("make-dirs", "create", dest, dest, opInfo{kind: "dir", mode: item.Chmod}, func() error { return p.FS.MakeDir(dest, mode, true) })
	if err != nil {
		return err
	}
	if mode > 0o777 {
		if err := p.chmod("make-dirs", mode, dest); err != nil {
			return err
		}
	}
	return p.chownIfSet("make-dirs", item, dest)
}

func (p *ProfileReady) chmod(scope string, mode os.FileMode, dest string) error {
	arg := fsutil.FormatModeArg(mode)
	return p.mutate(scope, "chmod", arg+" "+dest, dest, opInfo{kind: "chmod", mode: arg}, func() error { return p.FS.ChangeMode(arg, dest) })
}

func (p *ProfileReady) chown(scope, owner, dest string) error {
	return p.mutate(scope, "chown", owner+" "+dest, dest, opInfo{kind: "chown", owner: owner}, func() error { return p.FS.ChangeOwner(owner, dest) })
}

func (p *ProfileReady) chownIfSet(scope string, item spec.FileItem, dest string) error {
	if owner := formatOwnerSpec(item); owner != "" {
		return p.chown(scope, owner, dest)
	}
	return nil
}

func (p *ProfileReady) fixPerms(op, dest string, item spec.FileItem) error {
	needChmod, needChown := fsutil.DetectPermsDrift(p.Reader, dest, item.Chmod, formatOwnerSpec(item))
	if needChmod {
		mode, _ := fsutil.ParseMode(item.Chmod)
		if err := p.chmod(op, mode, dest); err != nil {
			return err
		}
	} else if p.isDryRunAll() && item.Chmod != "" {
		p.emit(log.Levels.Info, op, "chmod", dest, "already set")
	}
	if needChown {
		if err := p.chown(op, formatOwnerSpec(item), dest); err != nil {
			return err
		}
	} else if p.isDryRunAll() && formatOwnerSpec(item) != "" {
		p.emit(log.Levels.Info, op, "chown", dest, "already set")
	}
	return nil
}

func (p *ProfileReady) runFileOp(op, kind string, dirRelativePaths []string, items []spec.FileItem,
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
	if err := p.archiveBefore(op, dests); err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, item := range items {
		for _, dest := range destsOf(item) {
			if err := settle(item, dest); err != nil {
				errs = append(errs, p.failItem(op, dest, err))
			}
		}
	}
	if !p.isDryRun() {
		errs = append(errs, p.sweepStale(kind, dests))
	}
	return errors.Join(errs...)
}

func (p *ProfileReady) makeLinks(links []spec.FileItem, dirRelativePaths []string) error {
	return p.runFileOp("make-links", "link", dirRelativePaths, links, p.resolveLinkDests, p.makeLink)
}

func (p *ProfileReady) resolveLinkDests(item spec.FileItem) []string {
	if len(item.Dests) == 0 || item.Derived {
		return []string{p.toDest(spec.DestRel(item))}
	}
	out := make([]string, len(item.Dests))
	for i, d := range item.Dests {
		out[i] = p.expandHome(d.Path)
	}
	return out
}

func (p *ProfileReady) makeLink(item spec.FileItem, dest string) error {
	src := p.resolveSrc(item.Rel)
	if fsutil.IsLinkSettled(p.Reader, src, dest) {
		if p.isDryRunAll() {
			p.emit(log.Levels.Info, "make-links", p.wouldAction(dest), dest, "already linked")
		}
		return nil
	}
	return p.mutate("make-links", "create", dest, dest, opInfo{kind: "link", target: src, srcRel: item.Rel}, func() error { return p.FS.MakeSymlink(src, dest) })
}

func (p *ProfileReady) makeCopies(copies []spec.FileItem, dirRelativePaths []string) error {
	return p.runFileOp("make-copies", "copy", dirRelativePaths, copies, p.resolveCopyDests, p.makeCopy)
}

func (p *ProfileReady) makeCopy(item spec.FileItem, dest string) error {
	src := p.resolveSrc(item.Rel)
	if fsutil.IsSameContent(p.Reader, src, dest) {
		if p.isDryRunAll() {
			p.emit(log.Levels.Info, "make-copies", p.wouldAction(dest), dest, "same content")
		}
		return p.fixPerms("make-copies", dest, item)
	}
	mode, _ := fsutil.ParseMode(item.Chmod)
	err := p.mutate("make-copies", "create", dest, dest, opInfo{kind: "copy", srcRel: item.Rel, mode: item.Chmod}, func() error { return p.FS.CopyFile(src, dest, mode) })
	if err != nil {
		return err
	}
	return p.chownIfSet("make-copies", item, dest)
}

func (p *ProfileReady) resolveCopyDests(item spec.FileItem) []string {
	if len(item.Dests) == 0 {
		return []string{p.toDest(strings.TrimSuffix(item.Rel, spec.CpExt))}
	}
	if item.Derived {
		return []string{p.toDest(item.Dests[0].Path)}
	}
	out := make([]string, len(item.Dests))
	for i, d := range item.Dests {
		out[i] = p.expandHome(d.Path)
	}
	return out
}

func formatOwnerSpec(item spec.FileItem) string {
	if item.Owner == "" {
		return ""
	}
	if item.OwnerGroup == "" {
		return item.Owner
	}
	return item.Owner + ":" + item.OwnerGroup
}

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

func (p *ProfileReady) linkSourcePresent(op database.OperationDone) bool {
	if op.Target == "" {
		return true // [why] no recorded source: leave it, only prune known-gone
	}
	_, err := p.Reader.StatPath(op.Target)
	return err == nil
}

// [<] 🤖🤖
