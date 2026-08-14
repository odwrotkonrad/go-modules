package packages

// [>] 🤖🤖🤖

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) installBinariesRemoteArchive(pkg string, b *BinariesRemoteArchiveSpec) error {
	pin := in.pinFor(pkg, b.Version)
	if in.skipInstalledOrEmitReinstall(pkg, "binariesRemoteArchive", pin, pin,
		func() bool { return in.hasCmd(pkg) },
		func() bool { return in.versionOutputHasPin(pkg, pin) }, nil) {
		return nil
	}
	if err := in.requestedOverridesPin(pkg, b.Version); err != nil {
		return err
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", labelWithVersion(in.pkgLabel(pkg), pin)+" via binariesRemoteArchive")
		return nil
	}
	version, err := in.resolveArchiveVersion(pkg, b)
	if err != nil {
		return err
	}
	arch, err := in.resolveArch(b.ArchScheme)
	if err != nil {
		return fmt.Errorf("%s: %w", pkg, err)
	}
	url := in.Host.expandAs(b.URL, version, arch)
	asset, cached, cleanup, err := in.fetchAsset(url)
	if err != nil {
		return err
	}
	defer cleanup()
	if want, ok := b.PlatformEligibility.Checksums[in.Host.PlatformKey()]; ok {
		if err := in.verifyChecksum(pkg, asset, want); err != nil {
			if cached {
				_ = os.Remove(asset)
			}
			return err
		}
	} else {
		in.emit(log.Levels.Warn, "unverified", pkg+": no checksum declared for "+in.Host.PlatformKey()+", skipping verification")
	}
	if err := in.installMembers(pkg, asset, version, arch, b); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "installed", labelWithVersion(in.pkgLabel(pkg), version)+" via binariesRemoteArchive")
	return nil
}

func (in *Installer) fetchAsset(url string) (string, bool, func(), error) {
	if in.Opts.DownloadCacheDir != "" {
		dir := in.expandPath(in.Opts.DownloadCacheDir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", false, nil, err
		}
		asset := filepath.Join(dir, fmt.Sprintf("%x-%s", sha256.Sum256([]byte(url)), path.Base(url)))
		if _, err := os.Stat(asset); err == nil {
			return asset, true, func() {}, nil
		}
		if err := in.download(url, asset); err != nil {
			return "", false, nil, err
		}
		return asset, true, func() {}, nil
	}
	tmp, err := os.MkdirTemp("", "che-packages-")
	if err != nil {
		return "", false, nil, err
	}
	asset := filepath.Join(tmp, path.Base(url))
	if err := in.download(url, asset); err != nil {
		_ = os.RemoveAll(tmp)
		return "", false, nil, err
	}
	return asset, false, func() { _ = os.RemoveAll(tmp) }, nil
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

func (in *Installer) verifyChecksum(pkg, asset, want string) error {
	hex, err := parseChecksumHex(want)
	if err != nil {
		return fmt.Errorf("%s: %w", pkg, err)
	}
	argv := []string{"shasum", "-a", "256", asset}
	if in.Host.HasCmd("sha256sum") {
		argv = []string{"sha256sum", asset}
	}
	out, ok := in.output(argv)
	if !ok {
		return fmt.Errorf("%s: sha256 of %s failed", pkg, asset)
	}
	fields := strings.Fields(out)
	if len(fields) == 0 || fields[0] != hex {
		return fmt.Errorf("%s: sha256 mismatch for %s: want %s", pkg, asset, hex)
	}
	return nil
}

func (in *Installer) resolveArch(scheme string) (string, error) {
	if scheme == "" {
		return in.Host.Arch, nil
	}
	if v, ok := in.File.archSchemesOrBuiltin()[scheme][in.Host.Arch]; ok {
		return strings.ReplaceAll(v, "{os}", in.Host.OS), nil
	}
	return "", fmt.Errorf("unknown archScheme %q for arch %s (declare it under archSchemes)", scheme, in.Host.Arch)
}

func (in *Installer) expandMembers(pkg, version, arch string, b *BinariesRemoteArchiveSpec) []string {
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

func (in *Installer) installMembers(pkg, asset, version, arch string, b *BinariesRemoteArchiveSpec) error {
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
	for _, m := range in.expandMembers(pkg, version, arch, b) {
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
		if err := unzip(asset, dest); err != nil {
			return fmt.Errorf("unzip %s: %w", asset, err)
		}
		return nil
	}
	return in.exec([]string{"tar", "-x", "-C", dest, "-f", asset})
}

func unzip(asset, dest string) error {
	r, err := zip.OpenReader(asset)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	for _, f := range r.File {
		target, err := sanitizeArchivePath(dest, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode().Perm()|0o200)
		if err != nil {
			_ = src.Close()
			return err
		}
		_, err = io.Copy(out, src)
		_ = src.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func sanitizeArchivePath(dest, name string) (string, error) {
	target := filepath.Join(dest, name)
	if target != dest && !strings.HasPrefix(target, dest+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive member escapes destination: %s", name)
	}
	return target, nil
}

func (in *Installer) userBinDir() string {
	return in.resolveDestDir(&in.binDir, in.Opts.BinariesRemoteArchiveDestinationCandidates, DefaultBinariesRemoteArchiveDestinationCandidates,
		in.Opts.BinariesRemoteArchiveCheckPresentOnPath, in.Host.PathDirs,
		"packages.binariesRemoteArchive.installDestinationCandidates", "PATH")
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
