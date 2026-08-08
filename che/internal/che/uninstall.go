package che

// [>] 🤖🤖

import (
	"errors"

	"gitlab.com/konradodwrot/go-modules/che/internal/database"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
)

type Uninstaller struct {
	p      *ProfileReady
	dryRun bool
}

func NewUninstaller(ctx Context, opts options.Options) (*Uninstaller, error) {
	p, err := newLedgerProfile(ctx, opts, "uninstall", "uninstall")
	if err != nil {
		return nil, err
	}
	return &Uninstaller{p: p, dryRun: opts.DryRun != options.DryRun.Off}, nil
}

func (u *Uninstaller) Uninstall() error {
	p := u.p
	installed, err := p.Ledger.Installed()
	if err != nil {
		return err
	}
	var errs []error
	lastRef := ""
	for _, op := range installed {
		if op.ProfileRef != lastRef {
			log.EmitHeading(log.Levels.Info, 2, "uninstall", "uninstalling", "profile "+op.ProfileRef)
			lastRef = op.ProfileRef
		}
		if err := u.revert(op); err != nil {
			errs = append(errs, p.failItem("uninstall", op.Dest, err))
		}
	}
	for _, err := range errs {
		log.EmitError("uninstall", "fail", err.Error())
	}
	return errors.Join(errs...)
}

func (u *Uninstaller) revert(op database.OperationDone) error {
	p := u.p
	live := p.classifyDest(op.Dest)
	if driftedFromNext(live, op.Next) {
		p.emit(log.Levels.Debug, "uninstall", "remove", op.Dest, "dest drifted from the recorded state")
		return nil
	}
	if u.dryRun {
		p.emitDryRun("uninstall", "remove", op.Dest)
		return nil
	}
	if err := p.snapshotForRemoval("uninstall", []string{op.Dest}); err != nil {
		return err
	}
	if err := u.revertKind(op); err != nil {
		return err
	}
	p.emit(log.Levels.Info, "uninstall", "removed", op.Dest)
	p.recordRemoval(op.Kind, op.Dest, live)
	return nil
}

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
		_, err := u.p.restoreFromBackup(op)
		return err
	default:
		return nil
	}
}

func (u *Uninstaller) removeDir(dest string) error {
	err := u.p.FS.RemoveDir(dest)
	if err != nil && u.p.Reader != nil {
		if entries, derr := u.p.Reader.ReadDirectory(dest); derr == nil && len(entries) > 0 {
			u.p.emit(log.Levels.Debug, "uninstall", "remove-dir", dest, "directory not empty")
			return nil
		}
	}
	return err
}

func (u *Uninstaller) restoreOrRemove(op database.OperationDone) error {
	restored, err := u.p.restoreFromBackup(op)
	if err != nil {
		return err
	}
	if restored {
		return nil
	}
	return u.p.FS.RemoveFile(op.Dest)
}

func (u *Uninstaller) Close() error { return u.p.Ledger.Close() }

// [<] 🤖🤖
