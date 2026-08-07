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

func (in *Installer) installBinary(pkg string, b *BinarySpec) error {
	if in.Host.HasCmd(pkg) {
		if b.Version == "" || in.versionOutputHasPin(pkg, b.Version) {
			in.emitSkip(log.Levels.Debug, pkg, "already installed via binary")
			return nil
		}
		in.emit(log.Levels.Info, "reinstall", pkg+": -> "+b.Version)
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", pkg+" via binary")
		return nil
	}
	url := in.Host.expand(b.URL, b.Version)
	tmp, err := os.MkdirTemp("", "che-packages-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	asset := filepath.Join(tmp, path.Base(url))
	if err := in.exec([]string{"curl", "-fsSL", "--connect-timeout", "30", "--retry", "10", "--retry-delay", "30", "--retry-all-errors", "-o", asset, url}); err != nil {
		return err
	}
	if err := in.verifySha256(pkg, asset, b.Sha256[in.Host.ShaKey()]); err != nil {
		return err
	}
	if err := in.installMembers(pkg, asset, b); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "installed", pkg+" via binary")
	return nil
}

func (in *Installer) installPkg(pkg string, p *PkgSpec) error {
	if in.Host.HasCmd(pkg) {
		if p.Version == "" || in.versionOutputHasPin(pkg, p.Version) {
			in.emitSkip(log.Levels.Debug, pkg, "already installed via pkg")
			return nil
		}
		in.emit(log.Levels.Info, "reinstall", pkg+": -> "+p.Version)
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", pkg+" via pkg")
		return nil
	}
	url := in.Host.expand(p.URL, p.Version)
	tmp, err := os.MkdirTemp("", "che-packages-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	asset := filepath.Join(tmp, path.Base(url))
	if err := in.exec([]string{"curl", "-fsSL", "--connect-timeout", "30", "--retry", "10", "--retry-delay", "30", "--retry-all-errors", "-o", asset, url}); err != nil {
		return err
	}
	if want, ok := p.Sha256[in.Host.ShaKey()]; ok {
		if err := in.verifySha256(pkg, asset, want); err != nil {
			return err
		}
	}
	argv := []string{"installer", "-pkg", asset, "-target", "/"}
	if in.Host.Euid != 0 {
		argv = append([]string{"sudo"}, argv...)
	}
	if err := in.exec(argv); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "installed", pkg+" via pkg")
	return nil
}

func (in *Installer) versionOutputHasPin(pkg, pin string) bool {
	if out, ok := in.output([]string{pkg, "--version"}); ok && strings.Contains(out, pin) {
		return true
	}
	out, ok := in.output([]string{pkg, "version"})
	return ok && strings.Contains(out, pin)
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

func (in *Installer) members(pkg string, b *BinarySpec) []string {
	spec := b.Bin
	if spec == "" {
		spec = pkg
	}
	fields := strings.Fields(spec)
	out := make([]string, len(fields))
	for i, m := range fields {
		out[i] = in.Host.expand(m, b.Version)
	}
	return out
}

func (in *Installer) installMembers(pkg, asset string, b *BinarySpec) error {
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
	for _, m := range in.members(pkg, b) {
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
	if in.binDir != "" {
		return in.binDir
	}
	candidates := in.Opts.BinaryDestinationCandidates
	if len(candidates) == 0 {
		candidates = []string{filepath.Join(in.Host.Getenv("HOME"), ".local", "bin")}
	}
	expanded := make([]string, len(candidates))
	for i, c := range candidates {
		expanded[i] = in.expandPath(c)
	}
	in.binDir = expanded[0]
	if in.Opts.BinaryCheckInPath {
		onPath := slices.IndexFunc(expanded, func(d string) bool { return slices.Contains(in.Host.PathDirs(), d) })
		if onPath >= 0 {
			in.binDir = expanded[onPath]
		} else {
			in.emit(log.Levels.Warn, "not-on-path",
				"no packages.binary.installDestinationCandidates entry is on PATH ("+strings.Join(expanded, ", ")+"), using "+in.binDir)
		}
	}
	return in.binDir
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
