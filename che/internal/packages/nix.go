package packages

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"path"
	"regexp"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) nixBin() string {
	if in.Host.HasCmd("nix") {
		return "nix"
	}
	if p := in.Host.NixBin(); p != "" {
		return p
	}
	return "nix"
}

func (in *Installer) nixArgv(args ...string) []string {
	return append([]string{in.nixBin(), "--extra-experimental-features", "nix-command flakes", "profile"}, args...)
}

func (in *Installer) nixRegistryFor(pkg string, n *NixSpec) (*NixRegistrySpec, error) {
	name := cmp.Or(n.FromRegistry, DefaultNixRegistry)
	reg, err := in.File.nixRegistry(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", pkg, err)
	}
	if reg.URL == "" {
		return nil, fmt.Errorf("%s: nix registry %q requires name and url", pkg, name)
	}
	return reg, nil
}

func nixRef(reg *NixRegistrySpec, attr, rev string) string {
	switch {
	case rev != "":
		return reg.URL + "/" + rev + "#" + attr
	case reg.Ref != "":
		return reg.URL + "/" + reg.Ref + "#" + attr
	}
	return reg.URL + "#" + attr
}

func (in *Installer) nixPins(pkg string, n *NixSpec) (binPin, rev string, err error) {
	if r, ok := in.requested[pkg]; ok && len(r.Versions) > 0 {
		v := r.globalVersion()
		if rv, ok := n.VersionMap[v]; ok {
			return v, rv, nil
		}
		return "", "", fmt.Errorf("%s: requested version %s has no nixpkgs revision in the nix item's versionMap", pkg, v)
	}
	for bin, rv := range n.VersionMap {
		return bin, rv, nil
	}
	if pin := in.pinFor(pkg, ""); pin != "" {
		return "", "", fmt.Errorf("%s: version %s pinned but the nix item's versionMap has no nixpkgs revision for it", pkg, pin)
	}
	return "", "", nil
}

func (in *Installer) installNixSpec(pkg string, n *NixSpec) error {
	if n == nil {
		n = &NixSpec{}
	}
	attr := n.packageName(pkg)
	reg, err := in.nixRegistryFor(pkg, n)
	if err != nil {
		return err
	}
	binPin, rev, err := in.nixPins(pkg, n)
	if err != nil {
		return err
	}
	installed := in.isInstalled(pkg, "nix", attr)
	if installed && in.Opts.Update && binPin == "" {
		return in.update(pkg, "nix", attr)
	}
	if in.skipInstalledOrEmitReinstall(pkg, "nix", binPin, binPin,
		func() bool { return installed },
		func() bool { return in.pinSatisfied(pkg, "nix", attr, binPin) },
		func() string { return in.installedVersion("nix", attr) }) {
		return nil
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", labelWithVersion(in.pkgLabel(pkg), binPin)+" via nix ("+reg.Name+")")
		return nil
	}
	if installed {
		if err := in.exec(in.nixArgv("remove", attr)); err != nil {
			return err
		}
	}
	if err := in.exec(in.nixArgv("install", nixRef(reg, attr, rev))); err != nil {
		return err
	}
	ver := binPin
	if ver == "" {
		ver = in.installedVersion("nix", attr)
	}
	in.emit(log.Levels.Info, "installed", labelWithVersion(in.pkgLabel(pkg), ver)+" via nix ("+reg.Name+")")
	return nil
}

var nixStoreVersionRe = regexp.MustCompile(`^[0-9][0-9A-Za-z.+_]*`)

func nixProfileEntry(out, attr string) (found bool, version string) {
	for field := range strings.FieldsSeq(out) {
		if !strings.HasPrefix(field, "/nix/store/") {
			continue
		}
		base := path.Base(field)
		if strings.HasSuffix(base, "-"+attr) || strings.HasSuffix(base, "-"+attr+"-bin") {
			found = true
			continue
		}
		_, rest, ok := strings.Cut(base, "-"+attr+"-")
		if !ok {
			continue
		}
		found = true
		if v := nixStoreVersionRe.FindString(rest); v != "" {
			version = v
		}
	}
	return found, version
}

func (in *Installer) nixProfileList() (string, bool) {
	return in.output(in.nixArgv("list"))
}

var nixInstallRoutine = managerRoutine{
	install: func(in *Installer, _, name, _ string) error {
		reg, err := in.nixRegistryFor(name, &NixSpec{})
		if err != nil {
			return err
		}
		return in.exec(in.nixArgv("install", nixRef(reg, name, "")))
	},
	update: func(in *Installer, _, base string) error {
		return in.exec(in.nixArgv("upgrade", base))
	},
}

// [<] 🤖🤖
