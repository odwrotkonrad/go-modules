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
	name := a.packageName(pkg)
	src, err := in.ensureAptRegistry(pkg, a)
	if err != nil {
		return err
	}
	binPin, pkgPin := in.resolveAptPins(pkg, a)
	if in.skipInstalledOrEmitReinstall(pkg, "apt", pkgPin, binPin,
		func() bool { return in.isInstalled(pkg, "apt", name) },
		func() bool { return in.installedVersion("apt", name) == pkgPin },
		func() string { return in.installedVersion("apt", name) }) {
		return nil
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", labelWithVersion(in.labelPkg(pkg), binPin)+" via apt")
		return nil
	}
	if err := in.aptUpdate(); err != nil {
		return err
	}
	argv := []string{"apt-get", "install", "--yes", "--no-install-recommends"}
	if src != nil && src.Suites != "" {
		argv = append(argv, "-t", src.Suites)
	}
	if pkgPin != "" {
		argv = append(argv, "--allow-downgrades", name+"="+pkgPin)
	} else {
		argv = append(argv, name)
	}
	if err := in.exec(in.sudo(argv...)); err != nil {
		return err
	}
	ver := binPin
	if ver == "" {
		ver = in.installedVersion("apt", name)
	}
	in.emit(log.Levels.Info, "installed", labelWithVersion(in.labelPkg(pkg), ver)+" via apt")
	return nil
}

func (in *Installer) ensureAptRegistry(pkg string, a *AptSpec) (*AptRepoSpec, error) {
	if a.FromRegistry == "" {
		return nil, nil
	}
	src, err := in.File.aptRegistry(a.FromRegistry)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", pkg, err)
	}
	if src.URL == "" || src.VerificationKey == "" {
		return nil, fmt.Errorf("%s: apt registry %q requires url and verificationKey", pkg, a.FromRegistry)
	}
	if !in.Opts.DryRun {
		if err := in.ensureAptRepo(makeAptSlug(src), src); err != nil {
			return nil, err
		}
	}
	return src, nil
}

func (in *Installer) resolveAptPins(pkg string, a *AptSpec) (binPin, pkgPin string) {
	if r, ok := in.requested[pkg]; ok && len(r.Versions) > 0 {
		v := r.globalVersion()
		return v, v
	}
	for bin, pkgVer := range a.VersionMap {
		return bin, pkgVer
	}
	pin := in.resolvePin(pkg, "")
	return pin, pin
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
		b, err := in.Host.ReadFile("/etc/os-release")
		codename := parseVersionCodename(b)
		if err != nil || codename == "" {
			return fmt.Errorf("%s: apt repo suites not set and VERSION_CODENAME unavailable", name)
		}
		suites = codename
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
		if err := in.download(r.VerificationKey, asc); err != nil {
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

func parseVersionCodename(b []byte) string {
	for line := range strings.Lines(string(b)) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "VERSION_CODENAME="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// [<] 🤖🤖
