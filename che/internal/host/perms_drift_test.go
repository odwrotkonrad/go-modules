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

// setgid (and other >0777 special bits) must round-trip: a dir already at the
// spec mode reports no drift, and a plain dir is fixed up to carry the bit.
// Regression: os.FileMode encodes setgid/sticky outside 0o7000, so comparing a
// parsed octal spec against a raw fi.Mode() always mismatched -> perpetual chmod.
func TestPermsDriftSpecialBits(t *testing.T) {
	cases := []struct {
		name    string
		chmod   string
		mkMode  os.FileMode // mode the dir is created with
		setBits os.FileMode // extra bits to chmod on after mkdir (mkdir mode drops them)
		perm    os.FileMode // expected perm bits after a wet fix
		special os.FileMode // expected special bits after a wet fix
	}{
		{"setgid-settled", "2775", 0o775, os.ModeSetgid, 0o775, os.ModeSetgid},
		{"sticky-settled", "1777", 0o777, os.ModeSticky, 0o777, os.ModeSticky},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, home := testutil.CheRepo(t)
			dest := filepath.Join(home, "special-dir")
			if err := os.Mkdir(dest, c.mkMode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dest, c.mkMode|c.setBits); err != nil {
				t.Fatal(err)
			}
			item := extraDir(dest, c.chmod)

			// already at spec mode -> delta must report nothing (no drift).
			delta := New(home, home, testutil.CheProfile, DryRunDelta)
			out, err := testutil.CaptureStdout(t, func() error { return delta.MkDirs(nil, []spec.FileItem{item}) })
			if err != nil {
				t.Fatal(err)
			}
			if s := strings.TrimSpace(testutil.StripANSI(out)); s != "" {
				t.Errorf("settled %s dest reported drift:\n%s", c.name, s)
			}
		})
	}
}

// a plain dir missing the spec's special bit drifts, and a wet run adds it.
func TestPermsDriftAddsSetgid(t *testing.T) {
	_, home := testutil.CheRepo(t)
	dest := filepath.Join(home, "setgid-dir")
	if err := os.Mkdir(dest, 0o775); err != nil { // no setgid vs spec 2775
		t.Fatal(err)
	}
	item := extraDir(dest, "2775")

	delta := New(home, home, testutil.CheProfile, DryRunDelta)
	out, err := testutil.CaptureStdout(t, func() error { return delta.MkDirs(nil, []spec.FileItem{item}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(testutil.StripANSI(out), "mkdir(chmod,dry-run=delta): 2775 "+dest) {
		t.Errorf("delta missing setgid drift line for %s:\n%s", dest, testutil.StripANSI(out))
	}

	wet := New(home, home, testutil.CheProfile, DryRunOff)
	if err := wet.MkDirs(nil, []spec.FileItem{item}); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(dest)
	if fi.Mode().Perm() != 0o775 || fi.Mode()&os.ModeSetgid == 0 {
		t.Errorf("wet run left mode %v, want 0775 with setgid", fi.Mode())
	}

	settled, err := testutil.CaptureStdout(t, func() error { return delta.MkDirs(nil, []spec.FileItem{item}) })
	if err != nil {
		t.Fatal(err)
	}
	if s := strings.TrimSpace(testutil.StripANSI(settled)); s != "" {
		t.Errorf("settled dest printed after fix:\n%s", s)
	}
}

// [<] 🤖🤖
