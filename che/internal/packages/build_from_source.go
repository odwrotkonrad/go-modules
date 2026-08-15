package packages

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) installBuildFromSource(pkg string, b *BuildFromSourceSpec) error {
	pin := in.resolvePin(pkg, b.Version)
	if in.skipInstalledOrEmitReinstall(pkg, "buildFromSource", pin, pin,
		func() bool { return in.hasCmd(pkg) },
		func() bool { return in.versionOutputHasPin(pkg, pin) }, nil) {
		return nil
	}
	if err := in.checkRequestedPin(pkg, b.Version); err != nil {
		return err
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", labelWithVersion(in.labelPkg(pkg), pin)+" via buildFromSource")
		return nil
	}
	if pin == "" && strings.Contains(b.URL, "{version}") {
		return fmt.Errorf("%s: no version pinned: set version on the entry or the buildFromSource item", pkg)
	}
	url := in.Host.expandTokens(b.URL, pin, "")
	asset, inCache, cleanup, err := in.fetchAsset(url)
	if err != nil {
		return err
	}
	defer cleanup()
	if b.Checksum != "" {
		if err := in.verifyChecksum(pkg, asset, b.Checksum); err != nil {
			if inCache {
				_ = os.Remove(asset)
			}
			return err
		}
	} else {
		in.emit(log.Levels.Warn, "unverified", pkg+": no checksum declared, skipping verification")
	}
	if err := in.buildFromSource(pkg, asset, b); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "installed", labelWithVersion(in.labelPkg(pkg), pin)+" via buildFromSource")
	return nil
}

func (in *Installer) buildFromSource(pkg, asset string, b *BuildFromSourceSpec) error {
	tmp, err := os.MkdirTemp("", "che-build-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := in.extract(asset, tmp); err != nil {
		return err
	}
	src, err := findSourceDir(tmp)
	if err != nil {
		return fmt.Errorf("%s: %w", pkg, err)
	}
	prefix := filepath.Dir(in.resolveBinDir())
	install := []string{"make", "install"}
	if !in.underHome(prefix) {
		install = in.sudo(install...)
	}
	steps := [][]string{
		append([]string{"./configure", "--prefix=" + prefix}, b.ConfigureArgs...),
		{"make", "-j" + strconv.Itoa(runtime.NumCPU())},
		install,
	}
	for _, argv := range steps {
		if err := in.runCmd(execx.Cmd{Argv: argv, Dir: src}); err != nil {
			return fmt.Errorf("%s: %s: %w", pkg, strings.Join(argv, " "), err)
		}
	}
	return nil
}

func (in *Installer) underHome(p string) bool {
	home := in.Host.Getenv("HOME")
	return home != "" && (p == home || strings.HasPrefix(p, home+string(os.PathSeparator)))
}

func findSourceDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return "", fmt.Errorf("source archive must contain a single top-level directory")
	}
	return filepath.Join(dir, entries[0].Name()), nil
}

// [<] 🤖🤖
