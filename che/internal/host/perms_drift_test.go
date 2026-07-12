package host

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/config"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

// extraDir builds a mkdirs FileItem for an absolute dest carrying chmod.
func extraDir(dest, chmod string) spec.FileItem {
	return spec.FileItem{Dests: []spec.DestSpec{{Path: dest}}, Perms: spec.Perms{Chmod: chmod}}
}

// mode drift on an existing extra-dir: delta reports mkdir(chmod), dry-run off
// fixes it, a settled dest prints nothing, and isDirSettled no longer swallows drift.
func TestPermsDriftChmod(t *testing.T) {
	_, home := testutil.CheRepo(t)
	dest := filepath.Join(home, "drift-dir")
	require.NoError(t, os.Mkdir(dest, 0o700)) // wrong mode vs spec 0755
	item := extraDir(dest, "0755")

	delta := New(home, home, testutil.CheProfile, config.Config{DryRun: config.DryRun.Delta})
	deltaOut, err := testutil.CaptureStdout(t, func() error { return delta.MkDirs(nil, []spec.FileItem{item}) })
	require.NoError(t, err)
	assert.Contains(t, testutil.StripANSI(deltaOut), "mkdir(chmod,dry-run=delta): 0755 "+dest)
	fi, _ := os.Stat(dest)
	assert.Equal(t, os.FileMode(0o700), fi.Mode().Perm(), "delta must not mutate (dry run)")

	wet := New(home, home, testutil.CheProfile, config.Config{})
	require.NoError(t, wet.MkDirs(nil, []spec.FileItem{item}))
	fi, _ = os.Stat(dest)
	assert.Equal(t, os.FileMode(0o755), fi.Mode().Perm(), "wet run must fix the mode")

	settledOut, err := testutil.CaptureStdout(t, func() error { return delta.MkDirs(nil, []spec.FileItem{item}) })
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(testutil.StripANSI(settledOut)), "settled dest must print nothing")
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
	}{
		{"setgid-settled", "2775", 0o775, os.ModeSetgid},
		{"sticky-settled", "1777", 0o777, os.ModeSticky},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, home := testutil.CheRepo(t)
			dest := filepath.Join(home, "special-dir")
			require.NoError(t, os.Mkdir(dest, c.mkMode))
			require.NoError(t, os.Chmod(dest, c.mkMode|c.setBits))
			item := extraDir(dest, c.chmod)

			// already at spec mode -> delta must report nothing (no drift).
			delta := New(home, home, testutil.CheProfile, config.Config{DryRun: config.DryRun.Delta})
			out, err := testutil.CaptureStdout(t, func() error { return delta.MkDirs(nil, []spec.FileItem{item}) })
			require.NoError(t, err)
			assert.Empty(t, strings.TrimSpace(testutil.StripANSI(out)), "settled %s dest must report no drift", c.name)
		})
	}
}

// a plain dir missing the spec's special bit drifts, and a wet run adds it.
func TestPermsDriftAddsSetgid(t *testing.T) {
	_, home := testutil.CheRepo(t)
	dest := filepath.Join(home, "setgid-dir")
	require.NoError(t, os.Mkdir(dest, 0o775)) // no setgid vs spec 2775
	item := extraDir(dest, "2775")

	delta := New(home, home, testutil.CheProfile, config.Config{DryRun: config.DryRun.Delta})
	out, err := testutil.CaptureStdout(t, func() error { return delta.MkDirs(nil, []spec.FileItem{item}) })
	require.NoError(t, err)
	assert.Contains(t, testutil.StripANSI(out), "mkdir(chmod,dry-run=delta): 2775 "+dest)

	wet := New(home, home, testutil.CheProfile, config.Config{})
	require.NoError(t, wet.MkDirs(nil, []spec.FileItem{item}))
	fi, _ := os.Stat(dest)
	assert.Equal(t, os.FileMode(0o775), fi.Mode().Perm())
	assert.NotZero(t, fi.Mode()&os.ModeSetgid, "wet run must add setgid")

	settled, err := testutil.CaptureStdout(t, func() error { return delta.MkDirs(nil, []spec.FileItem{item}) })
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(testutil.StripANSI(settled)), "settled dest must print nothing after fix")
}

// [<] 🤖🤖
