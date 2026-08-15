package packages

// [>] 🤖🤖

import (
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) ensureBasePackages(mgr, forPkg string) error {
	if in.Opts.DryRun {
		return nil
	}
	if in.baseDone == nil {
		in.baseDone = map[string]bool{}
	}
	for _, group := range []string{BaseCommon, makeInstallerKey(mgr)} {
		if in.baseDone[group] {
			continue
		}
		in.baseDone[group] = true
		pkgs := slices.DeleteFunc(slices.Clone(in.File.BasePackages[group]), func(p string) bool { return p == forPkg })
		if len(pkgs) == 0 {
			continue
		}
		if in.requiredBy == nil {
			in.requiredBy = map[string]string{}
		}
		for _, p := range pkgs {
			if _, ok := in.requiredBy[p]; !ok {
				in.requiredBy[p] = forPkg
			}
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
	sub.Opts.OnlyMethods = nil
	err := sub.InstallRequests(Requests(names))
	sub.baseInstalling = false
	sub.Opts.OnlyMethods = in.Opts.OnlyMethods
	*in = sub
	return err
}

// [<] 🤖🤖
