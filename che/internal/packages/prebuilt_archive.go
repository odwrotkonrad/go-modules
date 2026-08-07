package packages

// [>] 🤖🤖🤖

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) installPrebuiltArchive(pkg string, b *PrebuiltArchiveSpec) error {
	if in.hasCmd(pkg) {
		if pin := in.pinFor(pkg, b.Version); pin == "" || in.versionOutputHasPin(pkg, pin) {
			in.emitSkip(log.Levels.Debug, pkg, "already installed via prebuiltArchive")
			return nil
		}
		in.emit(log.Levels.Info, "reinstall", pkg+": -> "+in.pinFor(pkg, b.Version))
	}
	if err := in.requestedOverridesPin(pkg, b.Version); err != nil {
		return err
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", pkg+" via prebuiltArchive")
		return nil
	}
	version, err := in.resolveArchiveVersion(pkg, b)
	if err != nil {
		return err
	}
	arch, err := in.archFor(b.ArchConvention)
	if err != nil {
		return fmt.Errorf("%s: %w", pkg, err)
	}
	url := in.Host.expandAs(b.URL, version, arch)
	tmp, err := os.MkdirTemp("", "che-packages-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	asset := filepath.Join(tmp, path.Base(url))
	if err := in.exec(curlArgv(url, asset)); err != nil {
		return err
	}
	if want, ok := b.Sha256[in.Host.ShaKey()]; ok {
		if err := in.verifySha256(pkg, asset, want); err != nil {
			return err
		}
	} else {
		in.emit(log.Levels.Warn, "unverified", pkg+": no sha256 declared for "+in.Host.ShaKey()+", skipping verification")
	}
	if err := in.installMembers(pkg, asset, version, arch, b); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "installed", pkg+" via prebuiltArchive")
	return nil
}

func (in *Installer) versionOutputHasPin(pkg, pin string) bool {
	if e, ok := in.File.Packages[pkg]; ok && e.VersionCommand != "" {
		out, ok := in.output(strings.Fields(e.VersionCommand))
		return ok && PinMatches(out, pin)
	}
	cmd := in.cmdFor(pkg)
	if out, ok := in.output([]string{cmd, "--version"}); ok && PinMatches(out, pin) {
		return true
	}
	out, ok := in.output([]string{cmd, "version"})
	return ok && PinMatches(out, pin)
}

func (in *Installer) verifySha256(pkg, asset, want string) error {
	argv := []string{"shasum", "-a", "256", asset}
	if in.Host.HasCmd("sha256sum") {
		argv = []string{"sha256sum", asset}
	}
	out, ok := in.output(argv)
	if !ok {
		return fmt.Errorf("%s: sha256 of %s failed", pkg, asset)
	}
	fields := strings.Fields(out)
	if len(fields) == 0 || fields[0] != want {
		return fmt.Errorf("%s: sha256 mismatch for %s: want %s", pkg, asset, want)
	}
	return nil
}

func (in *Installer) archFor(convention string) (string, error) {
	if convention == "" {
		return in.Host.Arch, nil
	}
	if v, ok := in.File.archNameConventionsOrBuiltin()[convention][in.Host.Arch]; ok {
		return strings.ReplaceAll(v, "{os}", in.Host.OS), nil
	}
	return "", fmt.Errorf("unknown archConvention %q for arch %s (declare it under archNameConventions)", convention, in.Host.Arch)
}

func (in *Installer) members(pkg, version, arch string, b *PrebuiltArchiveSpec) []string {
	names := b.ExtractBinaries
	if len(names) == 0 {
		names = Strings{pkg}
	}
	out := make([]string, len(names))
	for i, m := range names {
		out[i] = in.Host.expandAs(m, version, arch)
	}
	return out
}

func (in *Installer) installMembers(pkg, asset, version, arch string, b *PrebuiltArchiveSpec) error {
	binDir := in.userBinDir()
	if err := in.exec([]string{"mkdir", "-p", binDir}); err != nil {
		return err
	}
	if !isArchive(asset) {
		return in.installBin(asset, filepath.Join(binDir, pkg))
	}
	opt := in.userOptDir(pkg)
	if err := in.exec([]string{"rm", "-rf", opt}); err != nil {
		return err
	}
	if err := in.exec([]string{"mkdir", "-p", opt}); err != nil {
		return err
	}
	if err := in.extract(asset, opt); err != nil {
		return err
	}
	for _, m := range in.members(pkg, version, arch, b) {
		if err := in.exec([]string{"ln", "-sf", filepath.Join(opt, m), filepath.Join(binDir, path.Base(m))}); err != nil {
			return err
		}
	}
	return nil
}

func isArchive(asset string) bool {
	for _, s := range []string{".tar.gz", ".tgz", ".tar.xz", ".zip"} {
		if strings.HasSuffix(asset, s) {
			return true
		}
	}
	return false
}

func (in *Installer) extract(asset, dest string) error {
	if strings.HasSuffix(asset, ".zip") {
		return in.exec([]string{"unzip", "-oq", asset, "-d", dest})
	}
	return in.exec([]string{"tar", "-x", "-C", dest, "-f", asset})
}

func (in *Installer) userBinDir() string {
	return in.resolveDestDir(&in.binDir, in.Opts.PrebuiltArchiveDestinationCandidates, DefaultPrebuiltArchiveDestinationCandidates,
		in.Opts.PrebuiltArchiveCheckPresentOnPath, in.Host.PathDirs,
		"packages.prebuiltArchive.installDestinationCandidates", "PATH")
}

func (in *Installer) resolveDestDir(cached *string, candidates, defaults []string, check bool, dirs func() []string, optName, pathLabel string) string {
	if *cached != "" {
		return *cached
	}
	if len(candidates) == 0 {
		candidates = defaults
	}
	expanded := make([]string, len(candidates))
	for i, c := range candidates {
		expanded[i] = in.expandPath(c)
	}
	*cached = expanded[0]
	if check {
		on := dirs()
		if i := slices.IndexFunc(expanded, func(d string) bool { return slices.Contains(on, d) }); i >= 0 {
			*cached = expanded[i]
		} else {
			in.emit(log.Levels.Warn, "not-on-"+strings.ToLower(pathLabel),
				"no "+optName+" entry is on "+pathLabel+" ("+strings.Join(expanded, ", ")+"), using "+*cached)
		}
	}
	return *cached
}

func (in *Installer) expandPath(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		p = filepath.Join(in.Host.Getenv("HOME"), strings.TrimPrefix(p, "~"))
	}
	return os.Expand(p, in.Host.Getenv)
}

func (in *Installer) userOptDir(pkg string) string {
	return filepath.Join(in.Host.Getenv("HOME"), ".local", "opt", pkg)
}

func (in *Installer) installBin(src, dest string) error {
	return in.exec([]string{"install", "-m", "0755", src, dest})
}

// [<] 🤖🤖🤖
