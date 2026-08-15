package packages

// [>] 🤖🤖🤖

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) CheckPresent(pkgs []string) []string {
	var missing []string
	for _, pkg := range pkgs {
		if !in.verifySpec(pkg).ChecksPath() {
			in.emit(log.Levels.Debug, "present", pkg+" (checkInPath disabled)")
			continue
		}
		if path, err := in.Host.LookPath(pkg); err == nil {
			in.emit(log.Levels.Debug, "present", pkg+" at "+path)
			continue
		}
		in.emit(log.Levels.Warn, "missing", pkg)
		missing = append(missing, pkg)
	}
	return missing
}

func (in *Installer) verifySpec(pkg string) *VerifySpec {
	if in.File == nil {
		return nil
	}
	entry, ok := in.File.Packages[pkg]
	if !ok {
		return nil
	}
	if it, picked, err := in.pickItem(pkg, entry); err == nil && picked && it.Verify != nil {
		return it.Verify
	}
	return entry.Verify
}

func (in *Installer) CheckUpgradable(pkgs []string) error {
	for _, pkg := range pkgs {
		entry, err := in.findEntry(pkg)
		if err != nil {
			return err
		}
		it, ok, err := in.pickItem(pkg, entry)
		if err != nil || !ok {
			continue
		}
		if pin := in.pinFor(pkg, pinnedVersion(it)); pin != "" {
			if in.hasCmd(pkg) && !in.versionOutputHasPin(pkg, pin) {
				in.emit(log.Levels.Warn, "upgradable", pkg+" via "+it.Mgr+": yaml pins "+pin)
			}
			continue
		}
		if it.Mgr == "binariesRemoteArchive" || it.Mgr == "script" {
			continue
		}
		name := it.Name
		if name == "" {
			name = pkg
		}
		if in.managerOutdated(it.Mgr)[path.Base(name)] {
			in.emit(log.Levels.Warn, "upgradable", pkg+" via "+it.Mgr)
		}
	}
	return nil
}

func pinnedVersion(it Item) string {
	switch {
	case it.BinariesRemoteArchive != nil:
		return it.BinariesRemoteArchive.Version
	case it.Script != nil:
		return it.Script.Version
	case it.Apt != nil:
		for bin := range it.Apt.VersionMap {
			return bin
		}
	case it.Nix != nil:
		for bin := range it.Nix.VersionMap {
			return bin
		}
	}
	return it.Version
}

func (in *Installer) managerOutdated(mgr string) map[string]bool {
	if names, ok := in.outdated[mgr]; ok {
		return names
	}
	if in.outdated == nil {
		in.outdated = map[string]map[string]bool{}
	}
	names := map[string]bool{}
	switch mgr {
	case "brew", "cask":
		if in.Host.OS == "darwin" && in.Host.HasCmd("brew") {
			if o, ok := in.output([]string{"brew", "outdated", "--quiet"}); ok {
				for l := range strings.FieldsSeq(o) {
					names[path.Base(l)] = true
				}
			}
		}
		in.outdated["brew"], in.outdated["cask"] = names, names
	case "apt":
		if in.Host.OS == "linux" && in.Host.HasCmd("apt-get") {
			if o, ok := in.output([]string{"apt", "list", "--upgradable"}); ok {
				for line := range strings.Lines(o) {
					if name, _, found := strings.Cut(line, "/"); found {
						names[strings.TrimSpace(name)] = true
					}
				}
			}
		}
		in.outdated[mgr] = names
	case "npm":
		if in.Host.HasCmd("npm") {
			if o, ok := in.output([]string{"npm", "outdated", "--global", "--parseable"}); ok {
				for line := range strings.Lines(o) {
					parts := strings.Split(strings.TrimSpace(line), ":")
					if len(parts) >= 2 {
						base, _ := splitPin(parts[1])
						names[path.Base(base)] = true
					}
				}
			}
		}
		in.outdated[mgr] = names
	default:
		in.outdated[mgr] = names
	}
	return names
}

func (in *Installer) CheckNotShadowed(pkgs []string) error {
	for _, pkg := range pkgs {
		entry, err := in.findEntry(pkg)
		if err != nil {
			return err
		}
		it, ok, err := in.pickItem(pkg, entry)
		if err != nil || !ok {
			continue
		}
		dir := in.managerBinDir(it.Mgr)
		if dir == "" {
			continue
		}
		expected := filepath.Join(dir, pkg)
		if _, err := os.Stat(expected); err != nil {
			continue
		}
		first, err := in.Host.LookPath(pkg)
		if err != nil {
			continue
		}
		if first != expected {
			in.emit(log.Levels.Warn, "shadowed", fmt.Sprintf("%s shadowed by %s (expected %s)", pkg, first, expected))
		}
	}
	return nil
}

func (in *Installer) managerBinDir(mgr string) string {
	switch mgr {
	case "brew":
		if o, ok := in.output([]string{"brew", "--prefix"}); ok {
			return filepath.Join(strings.TrimSpace(o), "bin")
		}
	case "apt":
		return "/usr/bin"
	case "npm":
		if o, ok := in.output([]string{"npm", "prefix", "--global"}); ok {
			return filepath.Join(strings.TrimSpace(o), "bin")
		}
	case "go":
		if d := in.Host.Getenv("GOBIN"); d != "" {
			return d
		}
		if d := in.Host.Getenv("GOPATH"); d != "" {
			return filepath.Join(d, "bin")
		}
		return filepath.Join(in.Host.Getenv("HOME"), "go", "bin")
	case "binariesRemoteArchive":
		return in.userBinDir()
	}
	return ""
}

func (in *Installer) CheckSinglePresent(pkgs []string) error {
	for _, pkg := range pkgs {
		var hits []string
		for _, dir := range in.Host.PathDirs() {
			p := filepath.Join(dir, pkg)
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
				hits = append(hits, p)
			}
		}
		if len(hits) > 1 {
			in.emit(log.Levels.Warn, "multiple-present", pkg+": "+strings.Join(hits, ", "))
		}
	}
	return nil
}

// [<] 🤖🤖🤖
