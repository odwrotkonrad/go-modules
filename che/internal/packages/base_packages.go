package packages

// [>] 🤖🤖

import (
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) ensureBasePackages(mgr string) error {
	if in.Opts.DryRun {
		return nil
	}
	if in.baseDone == nil {
		in.baseDone = map[string]bool{}
	}
	for _, group := range []string{BaseCommon, installerKey(mgr)} {
		if in.baseDone[group] {
			continue
		}
		in.baseDone[group] = true
		pkgs := in.File.BasePackages[group]
		if len(pkgs) == 0 {
			continue
		}
		in.emit(log.Levels.Debug, "base-packages", group+": ensuring "+strings.Join(pkgs, ", "))
		if err := in.installBasePackages(pkgs); err != nil {
			return err
		}
	}
	return nil
}

func (in *Installer) installBasePackages(names []string) error {
	sub := *in
	sub.baseInstalling = true
	err := sub.InstallRequests(Requests(names))
	sub.baseInstalling = false
	*in = sub
	return err
}

// [<] 🤖🤖
