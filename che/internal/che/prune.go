package che

// [>] 🤖🤖

import (
	"errors"

	"gitlab.com/konradodwrot/go-modules/che/internal/database"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (p *ProfileReady) sweepStale(kind string, produced []string) error {
	if p.Ledger == nil || p.profileDone == nil {
		return nil
	}
	installed, err := p.Ledger.InstalledForProfile(p.ref)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, d := range produced {
		keep[d] = true
	}
	var errs []error
	for _, op := range installed {
		if op.Kind != kind || keep[op.Dest] {
			continue
		}
		if err := p.removeStale(op); err != nil {
			errs = append(errs, p.failItem("prune-"+kind, op.Dest, err))
		}
	}
	return errors.Join(errs...)
}

func (p *ProfileReady) removeStale(op database.OperationDone) error {
	dest := op.Dest
	if err := p.snapshotForRemoval("prune", []string{dest}); err != nil {
		return err
	}
	prev := p.classifyDest(dest)
	if prev.Present {
		if err := p.FS.RemoveFile(dest); err != nil {
			return err
		}
	}
	p.emit(log.Levels.Info, "prune-"+op.Kind, "pruned", dest)
	p.recordRemoval(op.Kind, dest, prev)
	return nil
}

func (p *ProfileReady) snapshotForRemoval(sub string, dests []string) error {
	path := p.resolveArchivePath(sub)
	p.currentArchive = path
	p.currentSub = sub
	return p.FS.ArchiveDestinations(path, dests)
}

func (p *ProfileReady) recordRemoval(kind, dest string, prev database.Object) {
	if p.Ledger == nil || p.profileDone == nil {
		return
	}
	op := database.OperationDone{
		OpType: "remove",
		Kind:   kind,
		Dest:   dest,
		Prev:   prev,
		Next:   database.Object{Kind: "absent"},
	}
	if backup, err := p.Ledger.EnsureBackup(p.specDone, p.currentArchive, p.currentSub); err == nil && backup != nil {
		op.BackupID = &backup.ID
	}
	if err := p.Ledger.RecordOperation(p.profileDone, op); err != nil {
		return
	}
}

// [<] 🤖🤖
