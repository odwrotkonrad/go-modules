package che

// [>] 🤖🤖

import (
	"errors"

	"gitlab.com/konradodwrot/go-modules/che/internal/database"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
)

// Uninstaller backs out everything the ledger marks installed onto this host,
// across every run: it reverts each recorded dest (restore its pre-install
// backup, or remove it — snapshotting first so uninstall is itself reversible),
// recording inverse remove ops under its own run. It reuses ProfileReady as the
// execution surface (FS writer, reader, ledger recording), so its inverse ops
// record in the same normalized shape as installs.
type Uninstaller struct {
	p      *ProfileReady
	dryRun bool
}

// NewUninstaller builds an Uninstaller: resolve the invoking home, open the
// seams (FS writer + reader + ledger), and open this uninstall run's own
// SpecDone + ProfileDone so its inverse ops record in the same shape.
func NewUninstaller(ctx Context, opts options.Options) (*Uninstaller, error) {
	home, err := resolveInvokingHome(ctx)
	if err != nil {
		return nil, err
	}
	seams := NewSeams(home)
	spec, err := seams.Ledger.StartSpec(ctx.RunID, "", "uninstall")
	if err != nil {
		log.Debug("ledger", "uninstall start spec: "+err.Error())
	}
	prof, err := seams.Ledger.StartProfile(spec, "uninstall", "uninstall", "", home)
	if err != nil {
		log.Debug("ledger", "uninstall start profile: "+err.Error())
	}
	p := &ProfileReady{
		ref:         "uninstall",
		home:        home,
		opts:        opts,
		runID:       ctx.RunID,
		specDone:    spec,
		profileDone: prof,
		Seams:       seams,
	}
	return &Uninstaller{p: p, dryRun: opts.DryRun != options.DryRun.Off}, nil
}

// Uninstall reverts every ledger-installed dest, newest-first (so nested dests
// unwind cleanly). A failing item does not stop the rest: failures collect,
// report, and join. dry-run logs the actions and writes no rows.
func (u *Uninstaller) Uninstall() error {
	p := u.p
	installed, err := p.Ledger.Installed()
	if err != nil {
		return err
	}
	var errs []error
	for _, op := range installed {
		if err := u.revert(op); err != nil {
			errs = append(errs, p.failItem("uninstall", op.Dest, err))
		}
	}
	for _, err := range errs {
		log.Msg("uninstall(report)", "fail "+err.Error())
	}
	return errors.Join(errs...)
}

// revert backs out one recorded op: skip when the live dest drifted from the
// op's Next (don't clobber user changes), else snapshot the dest into a fresh
// uninstall backup and dispatch on op.Kind, recording an inverse remove op.
func (u *Uninstaller) revert(op database.OperationDone) error {
	p := u.p
	live := p.classifyDest(op.Dest)
	if driftedFromNext(live, op.Next) {
		p.logMsg("uninstall(drift)", op.Dest)
		return nil
	}
	if u.dryRun {
		p.logMsg("uninstall", op.Dest)
		return nil
	}
	if err := p.snapshotForRemoval("uninstall", []string{op.Dest}); err != nil {
		return err
	}
	if err := u.revertKind(op); err != nil {
		return err
	}
	p.logMsg("uninstall", op.Dest)
	p.recordRemoval(op.Kind, op.Dest, live)
	return nil
}

// revertKind dispatches the inverse of op by its Kind: created dests (link/copy/
// render/dir) restore their pre-install backup or are removed; chmod reapplies
// the prev mode (dest stays); chown is a no-op (prev owner not recorded); rm
// restores from backup if present.
func (u *Uninstaller) revertKind(op database.OperationDone) error {
	p := u.p
	switch op.Kind {
	case "link", "copy", "render":
		return u.restoreOrRemove(op)
	case "dir":
		return u.removeDir(op.Dest) // [why] che-created dir: rmdir only-if-empty, backup holds no dir entry
	case "chmod":
		if op.Prev.Mode == "" || !op.Prev.Present {
			return nil
		}
		return p.FS.ChangeMode(op.Prev.Mode, op.Dest)
	case "chown":
		return nil // [why] prev owner is not captured in Object; nothing to reapply
	case "rm":
		_, err := u.restoreFromBackup(op)
		return err
	default:
		return nil
	}
}

// removeDir removes a che-created dir, tolerating a non-empty dir (still holds
// other content) as a skip, not a failure — the safe uninstall guardrail.
func (u *Uninstaller) removeDir(dest string) error {
	err := u.p.FS.RemoveDir(dest)
	if err != nil && u.p.Reader != nil {
		if entries, derr := u.p.Reader.ReadDirectory(dest); derr == nil && len(entries) > 0 {
			u.p.logMsg("uninstall(keep-nonempty)", dest)
			return nil
		}
	}
	return err
}

// restoreOrRemove restores the op's pre-install backup entry onto the dest when
// one exists, else removes the dest (che had created it fresh).
func (u *Uninstaller) restoreOrRemove(op database.OperationDone) error {
	restored, err := u.restoreFromBackup(op)
	if err != nil {
		return err
	}
	if restored {
		return nil
	}
	return u.p.FS.RemoveFile(op.Dest)
}

// restoreFromBackup writes the op's pre-install backup entry back onto the dest
// through FS (sudo/dry-run honored). Returns false when the op has no backup or
// the archive holds no entry for the dest (it pre-existed absent).
func (u *Uninstaller) restoreFromBackup(op database.OperationDone) (bool, error) {
	if op.Backup == nil {
		return false, nil
	}
	body, mode, found, err := fsutil.ReadFromArchive(op.Backup.Path, op.Dest)
	if err != nil || !found {
		return false, err
	}
	if err := u.p.FS.InstallFile(op.Dest, body, mode, ""); err != nil {
		return false, err
	}
	return true, nil
}

// driftedFromNext reports whether the live dest no longer matches the op's
// recorded Next state (kind/target/mode), the "look before you overwrite"
// guardrail. An absent dest that the op left present is drift too.
func driftedFromNext(live, next database.Object) bool {
	return live.Kind != next.Kind || live.Target != next.Target || live.Mode != next.Mode
}

// Close releases the uninstaller's ledger handle.
func (u *Uninstaller) Close() error { return u.p.Ledger.Close() }

// [<] 🤖🤖
