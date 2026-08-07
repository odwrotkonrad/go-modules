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
	ArchX     string
	ArchG     string
	Euid      int
	LookPath  func(string) (string, error)
	PathDirs  func() []string
	FpathDirs func() []string
	Getenv    func(string) string
}

func NewHost() Host {
	archX, archG := "x86_64", "x86_64"
	if runtime.GOARCH == "arm64" {
		archX, archG = "aarch64", "arm64"
	}
	return Host{
		OS: runtime.GOOS, Arch: runtime.GOARCH, ArchX: archX, ArchG: archG,
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

func (h Host) ShaKey() string { return h.OS + "-" + h.Arch }

func (h Host) HasCmd(name string) bool {
	_, err := h.LookPath(name)
	return err == nil
}

func (h Host) expand(s, version string) string {
	r := strings.NewReplacer(
		"{version}", version, "{os}", h.OS, "{arch}", h.Arch,
		"{arch_x}", h.ArchX, "{arch_g}", h.ArchG)
	return r.Replace(s)
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
	case "code":
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
		_, ok := it.PrebuiltArchive.Sha256[h.ShaKey()]
		return ok, nil
	case "pkg":
		if it.Pkg == nil || it.Pkg.URL == "" {
			return false, fmt.Errorf("package %s: pkg item missing url", pkg)
		}
		return h.OS == "darwin", nil
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
	return h.pickPreferred(pkg, entry, nil)
}

func (h Host) pickPreferred(pkg string, entry Entry, preferred []string) (Item, bool, error) {
	for _, it := range orderByPreference(entry.Items, preferred) {
		ok, err := h.applicable(pkg, it)
		if err != nil {
			return Item{}, false, err
		}
		if ok {
			return it, true, nil
		}
	}
	return Item{}, false, nil
}

var KnownManagers = []string{"brew", "cask", "apt", "npm", "go", "gem", "prebuiltArchive", "script", "pkg", "code"}

var DefaultPreferredMethods = []string{"brew", "cask", "apt", "pkg", "prebuiltArchive", "script", "npm", "go", "gem", "code"}

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
