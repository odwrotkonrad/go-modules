package host

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/spec"
	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// extraDir builds a mkdirs FileItem for an absolute dest carrying chmod.
func extraDir(dest, chmod string) spec.FileItem {
	return spec.FileItem{Dests: []spec.DestSpec{{Path: dest}}, Perms: spec.Perms{Chmod: chmod}}
}

// mode drift on an existing extra-dir: delta reports mkdir(chmod), DryRunOff
// fixes it, a settled dest prints nothing, and dirSettled no longer swallows drift.
func TestPermsDriftChmod(t *testing.T) {
	_, home := testutil.CheRepo(t)
	dest := filepath.Join(home, "drift-dir")
	if err := os.Mkdir(dest, 0o700); err != nil { // wrong mode vs spec 0755
		t.Fatal(err)
	}
	item := extraDir(dest, "0755")

	delta := New(home, home, testutil.CheProfile, DryRunDelta)
	deltaOut, err := testutil.CaptureStdout(t, func() error { return delta.MkDirs(nil, []spec.FileItem{item}) })
	if err != nil {
		t.Fatal(err)
	}
	deltaOut = testutil.StripANSI(deltaOut)
	if !strings.Contains(deltaOut, "mkdir(chmod,dry-run=delta): 0755 "+dest) {
		t.Errorf("delta missing chmod drift line for %s:\n%s", dest, deltaOut)
	}
	if fi, _ := os.Stat(dest); fi.Mode().Perm() != 0o700 {
		t.Errorf("delta mutated mode to %o, want 0700 (dry run)", fi.Mode().Perm())
	}

	wet := New(home, home, testutil.CheProfile, DryRunOff)
	if err := wet.MkDirs(nil, []spec.FileItem{item}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(dest); fi.Mode().Perm() != 0o755 {
		t.Errorf("wet run left mode %o, want 0755 (fixed)", fi.Mode().Perm())
	}

	settledOut, err := testutil.CaptureStdout(t, func() error { return delta.MkDirs(nil, []spec.FileItem{item}) })
	if err != nil {
		t.Fatal(err)
	}
	if s := strings.TrimSpace(testutil.StripANSI(settledOut)); s != "" {
		t.Errorf("settled dest printed:\n%s", s)
	}
}

// [<] 🤖🤖
