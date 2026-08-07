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
	Euid      int
	LookPath  func(string) (string, error)
	PathDirs  func() []string
	FpathDirs func() []string
	Getenv    func(string) string
}

func NewHost() Host {
	return Host{
		OS: runtime.GOOS, Arch: runtime.GOARCH,
		Euid:      os.Geteuid(),
		LookPath:  exec.LookPath,
		PathDirs:  func() []string { return filepath.SplitList(os.Getenv("PATH")) },
		FpathDirs: fpathDirs,
		Getenv:    os.Getenv,
	}
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

func (h Host) ShaKey() string { return h.OS + "-" + h.Arch }

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
		if it.Apt != nil && it.Apt.Repo != nil && (it.Apt.Repo.URL == "" || it.Apt.Repo.GpgURL == "") {
			return false, fmt.Errorf("package %s: apt repo requires url and gpgUrl", pkg)
		}
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
	case "prebuiltArchive":
		if it.PrebuiltArchive == nil {
			return false, fmt.Errorf("package %s: prebuiltArchive item missing props", pkg)
		}
		if _, ok := it.PrebuiltArchive.Sha256[h.ShaKey()]; ok {
			return true, nil
		}
		for _, p := range it.PrebuiltArchive.Platforms {
			if p == h.ShaKey() || p == h.OS {
				return true, nil
			}
		}
		return false, nil
	case "versionManager":
		vm := it.VersionManager
		if vm == nil || len(vm.Versions) == 0 {
			return false, fmt.Errorf("package %s: versionManager item requires versions", pkg)
		}
		switch vm.Tool {
		case "pyenv":
			return h.HasCmd("pyenv"), nil
		case "nvm":
			_, err := os.Stat(filepath.Join(h.nvmDir(), "nvm.sh"))
			return err == nil, nil
		default:
			return false, fmt.Errorf("package %s: unknown versionManager tool %q (want pyenv or nvm)", pkg, vm.Tool)
		}
	case "script":
		if it.Script == nil || (it.Script.Run == "" && it.Script.Path == "" && it.Script.URL == "") {
			return false, fmt.Errorf("package %s: script item missing run, path, or remoteUrl", pkg)
		}
		if len(it.Script.Sha256) > 0 {
			if _, ok := it.Script.Sha256[h.ShaKey()]; !ok {
				return false, nil
			}
		}
		return it.Script.OS == "" || it.Script.OS == h.OS, nil
	default:
		return false, fmt.Errorf("unknown manager for %s: %s", pkg, it.Mgr)
	}
}

func (h Host) pick(pkg string, entry Entry) (Item, bool, error) {
	return h.pickPreferred(pkg, entry, nil, nil)
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

var KnownManagers = []string{"brew", "cask", "apt", "npm", "go", "gem", "prebuiltArchive", "script", "vscode", "versionManager"}

var PlatformMethods = []string{"brew", "brew/cask", "brew/vscode", "apt", "npm", "go", "gem", "prebuiltArchive", "script", "versionManager"}

var DefaultPreferredMethods = []string{"brew", "cask", "apt", "prebuiltArchive", "script", "npm", "go", "gem", "vscode"}

var DefaultPrebuiltArchiveDestinationCandidates = []string{"~/.local/bin", "~/bin"}

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
