package packages

// [>] 🤖🤖🤖

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ulikunitz/xz"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) installBinariesRemoteArchive(pkg string, b *BinariesRemoteArchiveSpec) error {
	pin := in.resolvePin(pkg, b.Version)
	if in.skipInstalledOrEmitReinstall(pkg, "binariesRemoteArchive", pin, pin,
		func() bool { return in.hasCmd(pkg) },
		func() bool { return in.versionOutputHasPin(pkg, pin) }, nil) {
		return nil
	}
	if err := in.checkRequestedPin(pkg, b.Version); err != nil {
		return err
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", labelWithVersion(in.labelPkg(pkg), pin)+" via binariesRemoteArchive")
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
	url := in.Host.expandTokens(b.URL, version, arch)
	asset, inCache, cleanup, err := in.fetchAsset(url)
	if err != nil {
		return err
	}
	defer cleanup()
	if want, ok := b.PlatformEligibility.Checksums[in.Host.PlatformKey()]; ok {
		if err := in.verifyChecksum(pkg, asset, want); err != nil {
			if inCache {
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
	in.emit(log.Levels.Info, "installed", labelWithVersion(in.labelPkg(pkg), version)+" via binariesRemoteArchive")
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
	cmd := in.resolveCmd(pkg)
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
	f, err := os.Open(asset)
	if err != nil {
		return fmt.Errorf("%s: sha256 of %s failed: %w", pkg, asset, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("%s: sha256 of %s failed: %w", pkg, asset, err)
	}
	if fmt.Sprintf("%x", h.Sum(nil)) != hex {
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
		out[i] = in.Host.expandTokens(m, version, arch)
	}
	return out
}

func (in *Installer) installMembers(pkg, asset, version, arch string, b *BinariesRemoteArchiveSpec) error {
	binDir := in.resolveBinDir()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	if !isArchive(asset) {
		if len(b.ExtractManpages) > 0 {
			return fmt.Errorf("%s: extractManpages requires an archive asset: %s", pkg, asset)
		}
		return installFile(asset, filepath.Join(binDir, pkg), 0o755)
	}
	opt := in.resolveOptDir(pkg)
	if err := os.RemoveAll(opt); err != nil {
		return err
	}
	if err := os.MkdirAll(opt, 0o755); err != nil {
		return err
	}
	if err := in.extract(asset, opt); err != nil {
		return err
	}
	for _, m := range in.expandMembers(pkg, version, arch, b) {
		if err := makeSymlink(filepath.Join(opt, m), filepath.Join(binDir, path.Base(m))); err != nil {
			return err
		}
	}
	return in.installManpageMembers(pkg, opt, version, arch, b)
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
	if err := extractTar(asset, dest); err != nil {
		return fmt.Errorf("extract %s: %w", asset, err)
	}
	return nil
}

func extractTar(asset, dest string) error {
	f, err := os.Open(asset)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	var r io.Reader
	switch {
	case strings.HasSuffix(asset, ".tar.gz"), strings.HasSuffix(asset, ".tgz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer func() { _ = gz.Close() }()
		r = gz
	case strings.HasSuffix(asset, ".tar.xz"):
		xr, err := xz.NewReader(f)
		if err != nil {
			return err
		}
		r = xr
	default:
		return fmt.Errorf("unsupported archive suffix: %s", asset)
	}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := sanitizeArchivePath(dest, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode).Perm()|0o200)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			if cerr := out.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
			if !hdr.ModTime.IsZero() {
				if err := os.Chtimes(target, hdr.ModTime, hdr.ModTime); err != nil {
					return err
				}
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := makeSymlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
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

func (in *Installer) resolveBinDir() string {
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

func (in *Installer) resolveOptDir(pkg string) string {
	return filepath.Join(in.Host.Getenv("HOME"), ".local", "opt", pkg)
}

func makeSymlink(src, dest string) error {
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(src, dest)
}

func installFile(src, dest string, mode os.FileMode) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dest, b, mode); err != nil {
		return err
	}
	return os.Chmod(dest, mode)
}

// [<] 🤖🤖🤖
