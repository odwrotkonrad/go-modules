package packages

// [>] 🤖🤖

import (
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

// [why] a method's base packages are its own prerequisites (apt repos need gnupg + ca-certificates),
//
//	installed once per run before that method first runs
func (in *Installer) ensureBasePackages(mgr string) error {
	if in.Opts.DryRun {
		return nil
	}
	if in.baseDone == nil {
		in.baseDone = map[string]bool{}
	}
	for _, group := range []string{BaseCommon, itemKey(mgr)} {
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

// [why] base packages install through the same entries, minus their own base-package pass
func (in *Installer) installBasePackages(names []string) error {
	sub := *in
	sub.baseInstalling = true
	err := sub.InstallRequests(Requests(names))
	sub.baseInstalling = false
	*in = sub
	return err
}

// [<] 🤖🤖
