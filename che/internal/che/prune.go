package che

// [>] 🤖🤖

import (
	"errors"

	"gitlab.com/konradodwrot/go-modules/che/internal/database"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
)

// sweepStale removes every dest of kind the ledger recorded for THIS profile
// that the current run no longer produced (produced holds this run's dests).
// Each stale dest is archived into a fresh prune backup, removed through FS, and
// an inverse remove op recorded — the ledger is the source of truth, git no
// longer classifies staleness. Records-off (nil ledger) / dry-run -> no-op sweep.
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

// removeStale archives the stale dest into a fresh prune backup, removes it, and
// records the inverse remove op (so Installed() drops it and the prune is itself
// reversible). Absent dest -> only the inverse row, nothing to remove.
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
	p.logMsg("prune-"+op.Kind, dest)
	p.recordRemoval(op.Kind, dest, prev)
	return nil
}

// snapshotForRemoval writes a fresh backup archive (sub) of dests through FS and
// stashes its path/sub as the in-flight Backup, so the recorded remove op points
// at a snapshot that makes the removal reversible.
func (p *ProfileReady) snapshotForRemoval(sub string, dests []string) error {
	path := fsutil.ResolveBackupArchivePath(p.home, Bin, sub, p.runID)
	p.currentArchive = path
	p.currentSub = sub
	return p.FS.ArchiveDestinations(path, dests)
}

// recordRemoval writes an inverse remove OperationDone for a swept/uninstalled
// dest, its Backup the just-written snapshot.
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
