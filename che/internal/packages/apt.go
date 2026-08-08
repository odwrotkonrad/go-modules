package packages

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) installAptSpec(pkg string, a *AptSpec) error {
	pkgs := a.InstallPackages.Names
	if len(pkgs) == 0 {
		pkgs = []string{pkg}
	}
	// [why] before the installed-skip: a pruned repo file heals, and repo-only entries
	//   (debian-backports) configure their source even when their package is present
	if a.FromSource != nil && !in.Opts.DryRun {
		if err := in.ensureAptRepo(pkg, a.FromSource); err != nil {
			return err
		}
	}
	binPin, pkgPin := in.aptPins(pkg, a)
	if len(pkgs) != 1 {
		binPin, pkgPin = "", ""
	}
	if in.aptAllInstalled(pkgs) {
		if pkgPin == "" || in.aptVersionInstalled(pkgs[0], pkgPin) {
			in.emitSkip(log.Levels.Debug, pkg, "already installed via apt")
			return nil
		}
		in.emit(log.Levels.Info, "reinstall", pkg+": -> "+binPin)
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", pkg+" via apt")
		return nil
	}
	if err := in.aptUpdate(); err != nil {
		return err
	}
	argv := []string{"apt-get", "install", "--yes", "--no-install-recommends"}
	if pkgPin != "" {
		argv = append(argv, "--allow-downgrades", pkgs[0]+"="+pkgPin)
	} else {
		argv = append(argv, pkgs...)
	}
	if err := in.exec(in.sudo(argv...)); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "installed", pkg+" via apt")
	return nil
}

// [why] the map key is the binary version the package delivers, the value the exact
//
//	debian package string apt pins to (epoch/revision decorated)
func (in *Installer) aptPins(pkg string, a *AptSpec) (string, string) {
	if r, ok := in.requested[pkg]; ok && len(r.Versions) > 0 {
		v := r.globalVersion()
		return v, v
	}
	for bin, pkgVer := range a.Versions {
		return bin, pkgVer
	}
	pin := in.pinFor(pkg, "")
	return pin, pin
}

func (in *Installer) aptVersionInstalled(name, pkgVer string) bool {
	out, ok := in.output([]string{"dpkg-query", "-W", "-f=${Version}", name})
	return ok && strings.TrimSpace(out) == pkgVer
}

func (in *Installer) aptAllInstalled(pkgs []string) bool {
	_, ok := in.output(append([]string{"dpkg", "-s"}, pkgs...))
	return ok
}

func (in *Installer) ensureAptRepo(name string, r *AptRepoSpec) error {
	key := r.VerificationKey
	download := strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://")
	if download {
		key = "/etc/apt/keyrings/" + name + ".asc"
	}
	src := "/etc/apt/sources.list.d/" + name + ".sources"
	if (!download || fileExists(key)) && fileExists(src) {
		return nil
	}
	suites := r.Suites
	if suites == "" {
		out, ok := in.output([]string{"sh", "-ec", ". /etc/os-release && echo \"$VERSION_CODENAME\""})
		if !ok || strings.TrimSpace(out) == "" {
			return fmt.Errorf("%s: apt repo suites not set and VERSION_CODENAME unavailable", name)
		}
		suites = strings.TrimSpace(out)
	}
	arch, ok := in.output([]string{"dpkg", "--print-architecture"})
	if !ok {
		return fmt.Errorf("%s: dpkg --print-architecture failed", name)
	}
	tmp, err := os.MkdirTemp("", "che-apt-repo-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if download {
		asc := filepath.Join(tmp, name+".asc")
		if err := in.exec(curlArgv(r.VerificationKey, asc)); err != nil {
			return err
		}
		if err := in.exec(in.sudo("install", "-m", "0755", "-d", "/etc/apt/keyrings")); err != nil {
			return err
		}
		if err := in.exec(in.sudo("install", "-m", "0644", asc, key)); err != nil {
			return err
		}
	}
	sources := filepath.Join(tmp, name+".sources")
	content := fmt.Sprintf("Types: deb\nURIs: %s\nSuites: %s\nComponents: %s\nArchitectures: %s\nSigned-By: %s\n",
		r.URL, suites, cmp.Or(r.Components, "stable"), strings.TrimSpace(arch), key)
	if err := os.WriteFile(sources, []byte(content), 0o644); err != nil {
		return err
	}
	if err := in.exec(in.sudo("install", "-m", "0644", sources, src)); err != nil {
		return err
	}
	in.aptUpdated = false
	in.emit(log.Levels.Info, "configured", name+" apt repo "+r.URL)
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// [<] 🤖🤖
