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
	OS        string
	Arch      string
	Distro    string
	Euid      int
	LookPath  func(string) (string, error)
	PathDirs  func() []string
	FpathDirs func() []string
	Getenv    func(string) string
}

func NewHost() Host {
	h := Host{
		OS: runtime.GOOS, Arch: runtime.GOARCH,
		Euid:      os.Geteuid(),
		LookPath:  exec.LookPath,
		PathDirs:  func() []string { return filepath.SplitList(os.Getenv("PATH")) },
		FpathDirs: fpathDirs,
		Getenv:    os.Getenv,
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

func (h Host) expandAs(s, version, arch string) string {
	return strings.NewReplacer("{version}", version, "{os}", h.OS, "{arch}", arch).Replace(s)
}

func (h Host) applicable(pkg string, it Item) (bool, error) {
	switch it.Mgr {
	case "brew", "cask":
		return h.OS == "darwin" && h.HasCmd("brew"), nil
	case "apt":
		return h.OS == "linux" && h.HasCmd("apt-get"), nil
	case "npm":
		return h.HasCmd("npm"), nil
	case "vscode":
		return h.HasCmd("code"), nil
	case "gem":
		return h.HasCmd("gem"), nil
	case "go":
		if h.HasCmd("go") {
			return true, nil
		}
		fi, err := os.Stat("/usr/local/go/bin/go")
		return err == nil && fi.Mode()&0o111 != 0, nil
	case "binariesRemoteArchive":
		if it.BinariesRemoteArchive == nil {
			return false, fmt.Errorf("package %s: binariesRemoteArchive item missing props", pkg)
		}
		return slices.Contains(it.BinariesRemoteArchive.PlatformEligibility.Names, h.PlatformKey()), nil
	case "pyenv", "nvm":
		if it.VersionManager == nil || len(it.VersionManager.Versions) == 0 {
			return false, fmt.Errorf("package %s: %s item requires versions", pkg, it.Mgr)
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

func (h Host) pickPreferred(pkg string, entry Entry, preferred, allowed []string) (Item, bool, error) {
	for _, it := range orderByPreference(entry.Items, preferred) {
		ok, err := h.applicable(pkg, it)
		if err != nil {
			return Item{}, false, err
		}
		if ok && len(allowed) > 0 && !slices.Contains(allowed, methodFamily(it.Mgr)) {
			ok = false
		}
		if ok {
			return it, true, nil
		}
	}
	return Item{}, false, nil
}

func methodFamily(mgr string) string {
	switch mgr {
	case "cask":
		return "brew/cask"
	case "vscode":
		return "brew/vscode"
	}
	return mgr
}

var KnownManagers = []string{"brew", "cask", "apt", "npm", "go", "gem", "binariesRemoteArchive", "script", "vscode", "pyenv", "nvm"}

var PlatformMethods = []string{"brew", "brew/cask", "brew/vscode", "apt", "npm", "go", "gem", "binariesRemoteArchive", "script", "pyenv", "nvm"}

var DefaultPreferredMethods = []string{"brew", "cask", "apt", "binariesRemoteArchive", "script", "npm", "go", "gem", "vscode"}

var DefaultBinariesRemoteArchiveDestinationCandidates = []string{"~/.local/bin", "~/bin"}

var DefaultCompletionsDestinationCandidates = []string{"~/.local/share/zsh/site-functions", "~/.zfunc"}

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
