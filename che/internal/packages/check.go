package packages

// [>] 🤖🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) CheckPresent(pkgs []string) []string {
	var missing []string
	for _, pkg := range pkgs {
		if in.isCodeManaged(pkg) {
			if in.codeExtensions()[strings.ToLower(pkg)] {
				in.emit(log.Levels.Debug, "present", pkg+" (code extension)")
				continue
			}
			in.emit(log.Levels.Warn, "missing", pkg)
			missing = append(missing, pkg)
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

func (in *Installer) isCodeManaged(pkg string) bool {
	if in.File == nil {
		return false
	}
	entry, ok := in.File.Packages[pkg]
	if !ok {
		return false
	}
	it, picked, err := in.Host.pickPreferred(pkg, entry, in.Opts.PreferredMethods)
	return err == nil && picked && it.Mgr == "code"
}

func (in *Installer) CheckUpgradable(pkgs []string) error {
	outdated := in.managerOutdated()
	for _, pkg := range pkgs {
		entry, err := in.File.Find(pkg, in.FilePath)
		if err != nil {
			return err
		}
		it, ok, err := in.Host.pickPreferred(pkg, entry, in.Opts.PreferredMethods)
		if err != nil || !ok {
			continue
		}
		if pin := pinnedVersion(it); pin != "" && (it.Mgr == "binary" || it.Mgr == "script" || it.Mgr == "pkg") {
			if in.Host.HasCmd(pkg) && !in.versionOutputHasPin(pkg, pin) {
				in.emit(log.Levels.Warn, "upgradable", pkg+" via "+it.Mgr+": yaml pins "+pin)
			}
			continue
		}
		if it.Mgr == "binary" || it.Mgr == "script" || it.Mgr == "pkg" {
			continue
		}
		name := it.Name
		if name == "" {
			name = pkg
		}
		base, _ := splitPin(it.Mgr, name)
		if outdated[it.Mgr][tail(base)] {
			in.emit(log.Levels.Warn, "upgradable", pkg+" via "+it.Mgr)
		}
	}
	return nil
}

func pinnedVersion(it Item) string {
	switch {
	case it.Binary != nil:
		return it.Binary.Version
	case it.Script != nil:
		return it.Script.Version
	case it.Pkg != nil:
		return it.Pkg.Version
	}
	return ""
}

func (in *Installer) managerOutdated() map[string]map[string]bool {
	out := map[string]map[string]bool{}
	if in.Host.OS == "darwin" && in.Host.HasCmd("brew") {
		names := map[string]bool{}
		if o, ok := in.output([]string{"brew", "outdated", "--quiet"}); ok {
			for l := range strings.FieldsSeq(o) {
				names[tail(l)] = true
			}
		}
		out["brew"], out["cask"] = names, names
	}
	if in.Host.OS == "linux" && in.Host.HasCmd("apt-get") {
		names := map[string]bool{}
		if o, ok := in.output([]string{"apt", "list", "--upgradable"}); ok {
			for line := range strings.Lines(o) {
				if name, _, found := strings.Cut(line, "/"); found {
					names[strings.TrimSpace(name)] = true
				}
			}
		}
		out["apt"] = names
	}
	if in.Host.HasCmd("npm") {
		names := map[string]bool{}
		if o, ok := in.output([]string{"npm", "outdated", "--global", "--parseable"}); ok {
			for line := range strings.Lines(o) {
				parts := strings.Split(strings.TrimSpace(line), ":")
				if len(parts) >= 2 {
					base, _ := splitPin("npm", parts[1])
					names[tail(base)] = true
				}
			}
		}
		out["npm"] = names
	}
	return out
}

func (in *Installer) CheckNotShadowed(pkgs []string) error {
	for _, pkg := range pkgs {
		entry, err := in.File.Find(pkg, in.FilePath)
		if err != nil {
			return err
		}
		it, ok, err := in.Host.pickPreferred(pkg, entry, in.Opts.PreferredMethods)
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
	case "binary":
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
