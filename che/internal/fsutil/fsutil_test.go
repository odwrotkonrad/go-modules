package fsutil

// [>] 🤖🤖

import (
	"embed"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func octal(t *testing.T, s string) os.FileMode {
	t.Helper()
	n, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		t.Fatalf("mode %q: %v", s, err)
	}
	return os.FileMode(n)
}

func TestModeArg(t *testing.T) {
	type in struct {
		Args []string
	}
	type c struct {
		Name string
		In   in
		Want string
	}
	testyml.Run(t, td, "testdata/spec/mode_arg.spec.yml", func(t *testing.T, c c) {
		if got := ModeArg(octal(t, c.In.Args[0])); got != c.Want {
			t.Errorf("ModeArg(%s) = %q, want %q", c.In.Args[0], got, c.Want)
		}
	})
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
	type in struct {
		Args []string
	}
	type c struct {
		Name string
		In   in
		Want bool
	}
	f := FS{Home: "/Users/x"}
	testyml.Run(t, td, "testdata/spec/under_home.spec.yml", func(t *testing.T, c c) {
		if got := f.UnderHome(c.In.Args[0]); got != c.Want {
			t.Errorf("UnderHome(%q) = %v, want %v", c.In.Args[0], got, c.Want)
		}
	})
}

func TestExpandAll(t *testing.T) {
	type in struct {
		Args []string
	}
	type c struct {
		Name string
		In   in
		Want []string
	}
	testyml.Run(t, td, "testdata/spec/expand_all.spec.yml", func(t *testing.T, c c) {
		if got := ExpandAll(c.In.Args); !slices.Equal(got, c.Want) {
			t.Errorf("ExpandAll = %v, want %v", got, c.Want)
		}
	})
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
		"che.yml":                                        "profiles:\n", // outside root/, excluded
		"root/etc/zshrc":                                 "z\n",
		"root/etc/zsh/zshenv":                            "e\n",
		"root/HOME/.config/zsh/.zshrc":                   "hidden\n",
		"root/HOME/.config/zsh/.gitkeep":                 "",
		"root/HOME/.config/git/config.ontoHost.tpl":      "tpl\n",
		"root/Library/LaunchDaemons/x.plist.ontoHost.cp": "cp\n",
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

func TestMkdirArgv(t *testing.T) {
	type in struct {
		Args    []string
		Parents bool
	}
	type c struct {
		Name string
		In   in
		Want []string
	}
	f := FS{Home: "/Users/x"}
	testyml.Run(t, td, "testdata/spec/mkdir_argv.spec.yml", func(t *testing.T, c c) {
		if got := f.MkdirArgv(c.In.Args[0], octal(t, c.In.Args[1]), c.In.Parents); !slices.Equal(got, c.Want) {
			t.Errorf("MkdirArgv(%v, %v) = %v, want %v", c.In.Args, c.In.Parents, got, c.Want)
		}
	})
}

// [<] 🤖🤖
