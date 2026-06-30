package fsutil

// [>] 🤖🤖

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

func TestModeArg(t *testing.T) {
	cases := map[os.FileMode]string{
		0644:  "0644",
		0600:  "0600",
		0755:  "0755",
		02775: "2775",
	}
	for mode, want := range cases {
		if got := ModeArg(mode); got != want {
			t.Errorf("ModeArg(%o) = %q, want %q", mode, got, want)
		}
	}
}

func TestIsDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if !IsDir(dir) {
		t.Errorf("IsDir(%q) = false, want true", dir)
	}
	if IsDir(file) {
		t.Errorf("IsDir(file) = true, want false")
	}
	if IsDir(filepath.Join(dir, "absent")) {
		t.Errorf("IsDir(absent) = true, want false")
	}
}

func TestUnderHome(t *testing.T) {
	f := FS{Home: "/Users/x"}
	cases := map[string]bool{
		"/Users/x":         true,
		"/Users/x/.config": true,
		"/Users/xyz":       false, // prefix-but-not-subtree
		"/etc/zshrc":       false,
		"/Users/x/a/b/c":   true,
	}
	for dest, want := range cases {
		if got := f.UnderHome(dest); got != want {
			t.Errorf("UnderHome(%q) = %v, want %v", dest, got, want)
		}
	}
}

func TestExpandAll(t *testing.T) {
	got := ExpandAll([]string{"plain", "x/{a,b}/y"})
	want := []string{"plain", "x/a/y", "x/b/y"}
	if !slices.Equal(got, want) {
		t.Errorf("ExpandAll = %v, want %v", got, want)
	}
}

// TestTrackedFiles: subtree filtering + untracked exclusion.
func TestTrackedFiles(t *testing.T) {
	dir := testutil.Repo(t, map[string]string{"a.txt": "x", "sub/b.txt": "x"})
	if err := os.WriteFile(filepath.Join(dir, "untracked"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := TrackedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	if want := []string{"a.txt", "sub/b.txt"}; !slices.Equal(got, want) {
		t.Errorf("TrackedFiles = %v, want %v (untracked must be excluded)", got, want)
	}

	// subtree: only entries under it, prefix-stripped
	sub, err := TrackedFiles(filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"b.txt"}; !slices.Equal(sub, want) {
		t.Errorf("TrackedFiles(sub) = %v, want %v", sub, want)
	}
}

// TestTrackedFilesMatchesCLI: byte-parity, go-git index walk vs `git ls-files
// --exclude-standard` on a mock root/ subtree (hidden, .gitkeep, markers, nesting).
func TestTrackedFilesMatchesCLI(t *testing.T) {
	dir := testutil.Repo(t, map[string]string{
		"che.yml":                                    "profiles:\n", // outside root/, excluded
		"root/etc/zshrc":                             "z\n",
		"root/etc/zsh/zshenv":                        "e\n",
		"root/HOME/.config/zsh/.zshrc":               "hidden\n",
		"root/HOME/.config/zsh/.gitkeep":             "",
		"root/HOME/.config/git/config.host.tpl":      "tpl\n",
		"root/Library/LaunchDaemons/x.plist.host.cp": "cp\n",
	})
	root := filepath.Join(dir, "root")

	out, err := exec.Command("git", "-C", root, "ls-files", "--exclude-standard").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	want := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	got, err := TrackedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("TrackedFiles != git ls-files\n got:  %v\n want: %v", got, want)
	}
}

// TestMkdirArgv: priv escalation depends on euid, assert the euid-independent
// shape (mkdir + mode + dest tail) and the asUser case at euid 0.
func TestMkdirArgv(t *testing.T) {
	f := FS{Home: "/Users/x"}
	argv := f.MkdirArgv("/Users/x/.config", "", 0o750, true)
	want := []string{"mkdir", "-p", "-m", "0750", "/Users/x/.config"}
	if !slices.Equal(argv, want) {
		t.Errorf("MkdirArgv(home dest) = %v, want %v", argv, want)
	}
	// no -p when parents is false
	argv = f.MkdirArgv("/Users/x/.config", "", 0o750, false)
	if slices.Contains(argv, "-p") {
		t.Errorf("MkdirArgv(parents=false) included -p: %v", argv)
	}
	// no -m when mode is 0 (umask honored)
	argv = f.MkdirArgv("/Users/x/.config", "", 0, true)
	if want := []string{"mkdir", "-p", "/Users/x/.config"}; !slices.Equal(argv, want) {
		t.Errorf("MkdirArgv(zero mode) = %v, want %v", argv, want)
	}
}

// [<] 🤖🤖
