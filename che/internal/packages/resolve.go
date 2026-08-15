package packages

// [>] 🤖🤖🤖

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

type Host struct {
	OS          string
	Arch        string
	Distro      string
	Euid        int
	LookPath    func(string) (string, error)
	PathDirs    func() []string
	FpathDirs   func() []string
	ManpathDirs func() []string
	Getenv      func(string) string
	Stat        func(string) (os.FileInfo, error)
	ReadFile    func(string) ([]byte, error)
}

func NewHost() Host {
	h := Host{
		OS: runtime.GOOS, Arch: runtime.GOARCH,
		Euid:        os.Geteuid(),
		LookPath:    exec.LookPath,
		PathDirs:    func() []string { return filepath.SplitList(os.Getenv("PATH")) },
		FpathDirs:   fpathDirs,
		ManpathDirs: manpathDirs,
		Getenv:      os.Getenv,
		Stat:        os.Stat,
		ReadFile:    os.ReadFile,
	}
	if h.OS == "linux" {
		h.Distro = linuxDistro()
	}
	return h
}

func linuxDistro() string {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for line := range strings.Lines(string(b)) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "ID="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

func (h Host) eligibilityKeys() []string {
	var keys []string
	if h.Distro != "" {
		keys = append(keys, h.OS+"-"+h.Distro+"-"+h.Arch, h.OS+"-"+h.Distro)
	}
	return append(keys, h.OS+"-"+h.Arch, h.OS)
}

func fpathDirs() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "zsh", "-ic", "print -rl -- $fpath").Output(); err == nil {
		var dirs []string
		for l := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if l != "" {
				dirs = append(dirs, l)
			}
		}
		if len(dirs) > 0 {
			return dirs
		}
	}
	if v := os.Getenv("FPATH"); v != "" {
		return filepath.SplitList(v)
	}
	return nil
}

func manpathDirs() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "manpath").Output(); err == nil {
		var dirs []string
		for _, d := range filepath.SplitList(strings.TrimSpace(string(out))) {
			if d != "" {
				dirs = append(dirs, d)
			}
		}
		if len(dirs) > 0 {
			return dirs
		}
	}
	if v := os.Getenv("MANPATH"); v != "" {
		return slices.DeleteFunc(filepath.SplitList(v), func(d string) bool { return d == "" })
	}
	return nil
}

func (h Host) nvmDir() string {
	if d := h.Getenv("NVM_DIR"); d != "" {
		return d
	}
	if x := h.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "nvm")
	}
	return filepath.Join(h.Getenv("HOME"), ".nvm")
}

func (h Host) PlatformKey() string { return h.OS + "-" + h.Arch }

func (h Host) HasCmd(name string) bool {
	_, err := h.LookPath(name)
	return err == nil
}

func (h Host) BrewBin() string {
	if p, err := h.LookPath("brew"); err == nil {
		return p
	}
	if h.Stat == nil {
		return ""
	}
	for _, p := range []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"} {
		if fi, err := h.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

func (h Host) NixBin() string {
	if p, err := h.LookPath("nix"); err == nil {
		return p
	}
	if h.Stat == nil {
		return ""
	}
	for _, p := range []string{"/nix/var/nix/profiles/default/bin/nix", filepath.Join(h.Getenv("HOME"), ".nix-profile", "bin", "nix")} {
		if fi, err := h.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}

func (h Host) expandTokens(s, version, arch string) string {
	return strings.NewReplacer("{version}", version, "{os}", h.OS, "{arch}", arch).Replace(s)
}

func (h Host) applicable(pkg string, it Item, strict bool) (bool, error) {
	switch it.Mgr {
	case "brew", "cask":
		return h.OS == "darwin" && (!strict || h.BrewBin() != ""), nil
	case "apt":
		return h.OS == "linux" && h.HasCmd("apt-get"), nil
	case "npm":
		return !strict || h.HasCmd("npm"), nil
	case "nix":
		return !strict || h.NixBin() != "", nil
	case "gem":
		return !strict || h.HasCmd("gem"), nil
	case "go":
		if !strict || h.HasCmd("go") {
			return true, nil
		}
		fi, err := os.Stat("/usr/local/go/bin/go")
		return err == nil && fi.Mode()&0o111 != 0, nil
	case "binariesRemoteArchive":
		if it.BinariesRemoteArchive == nil {
			return false, fmt.Errorf("package %s: binariesRemoteArchive item missing props", pkg)
		}
		return slices.Contains(it.BinariesRemoteArchive.PlatformEligibility.Names, h.PlatformKey()), nil
	case "buildFromSource":
		if it.BuildFromSource == nil {
			return false, fmt.Errorf("package %s: buildFromSource item missing props", pkg)
		}
		names := it.BuildFromSource.PlatformEligibility.Names
		return len(names) == 0 || slices.Contains(names, h.PlatformKey()), nil
	case "pyenv", "nvm":
		if it.VersionManager == nil || len(it.VersionManager.Versions) == 0 {
			return false, fmt.Errorf("package %s: %s item requires versions", pkg, it.Mgr)
		}
		if !strict {
			return true, nil
		}
		if it.Mgr == "pyenv" {
			return h.HasCmd("pyenv"), nil
		}
		_, err := os.Stat(filepath.Join(h.nvmDir(), "nvm.sh"))
		return err == nil, nil
	case "script":
		if it.Script == nil || (it.Script.Run == "" && it.Script.Path == "" && it.Script.URL == "") {
			return false, fmt.Errorf("package %s: script item missing run, path, or url", pkg)
		}
		if len(it.Script.PlatformEligibility.Names) > 0 && !slices.Contains(it.Script.PlatformEligibility.Names, h.PlatformKey()) {
			return false, nil
		}
		return it.Script.OS == "" || it.Script.OS == h.OS, nil
	default:
		return false, fmt.Errorf("unknown manager for %s: %s", pkg, it.Mgr)
	}
}

func (h Host) pickPreferred(pkg string, entry Entry, preferred, only, allowed []string, strictOnly bool) (Item, bool, error) {
	passes := []bool{true, false}
	if strictOnly {
		passes = []bool{true}
	}
	for _, strict := range passes {
		for _, it := range orderByPreference(entry.Items, preferred) {
			ok, err := h.applicable(pkg, it, strict)
			if err != nil {
				return Item{}, false, err
			}
			if ok && len(only) > 0 && !slices.Contains(only, it.Mgr) {
				ok = false
			}
			if ok && len(allowed) > 0 && !slices.Contains(allowed, makeInstallerKey(it.Mgr)) {
				ok = false
			}
			if ok {
				return it, true, nil
			}
		}
	}
	return Item{}, false, nil
}

var KnownManagers = []string{"brew", "cask", "apt", "npm", "go", "gem", "binariesRemoteArchive", "script", "buildFromSource", "pyenv", "nvm", "nix"}

var PlatformMethods = []string{"brew", "brew/cask", "apt", "npm", "go", "gem", "binariesRemoteArchive", "script", "buildFromSource", "pyenv", "nvm", "nix"}

var DefaultPreferredMethods = []string{"brew", "cask", "apt", "binariesRemoteArchive", "script", "buildFromSource", "npm", "go", "gem", "nix"}

var DefaultBinariesRemoteArchiveDestinationCandidates = []string{"~/.local/bin", "~/bin"}

var DefaultCompletionsDestinationCandidates = []string{"~/.local/share/zsh/site-functions", "~/.zfunc"}

var DefaultManpagesDestinationCandidates = []string{"~/.local/share/man"}

func ValidateManagers(names []string) error {
	for _, n := range names {
		if !slices.Contains(KnownManagers, n) {
			return fmt.Errorf("unknown installation method %q: want one of %s", n, strings.Join(KnownManagers, ", "))
		}
	}
	return nil
}

func orderByPreference(items []Item, preferred []string) []Item {
	if len(preferred) == 0 {
		return items
	}
	rank := func(it Item) int {
		if i := slices.Index(preferred, it.Mgr); i >= 0 {
			return i
		}
		return len(preferred)
	}
	out := slices.Clone(items)
	slices.SortStableFunc(out, func(a, b Item) int { return rank(a) - rank(b) })
	return out
}

// [<] 🤖🤖🤖
