package che

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/database"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
)

func realProfile(t *testing.T) *ProfileReady {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("non-root: FS writer stays under-home to avoid sudo")
	}
	home := t.TempDir()
	t.Setenv("CHE_STATE_HOME", filepath.Join(home, "state"))
	db, err := database.Open(filepath.Join(home, "state", "ops.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	spec, err := db.StartSpec(testRunID, "", "test")
	require.NoError(t, err)
	prof, err := db.StartProfile(spec, "cli", "cli", "", home)
	require.NoError(t, err)
	return &ProfileReady{
		ref: "cli", home: home, runID: testRunID, runTs: testRunID, specDone: spec, profileDone: prof,
		Seams: Seams{FS: fsutil.FS{Home: home}, Reader: fsutil.OSReader{}, Ledger: db},
	}
}

func uninstallerOver(t *testing.T, p *ProfileReady) *Uninstaller {
	t.Helper()
	spec, err := p.Ledger.StartSpec(testRunID+"-un", "", "uninstall")
	require.NoError(t, err)
	prof, err := p.Ledger.StartProfile(spec, "uninstall", "uninstall", "", p.home)
	require.NoError(t, err)
	up := *p
	up.ref, up.specDone, up.profileDone = "uninstall", spec, prof
	return &Uninstaller{p: &up}
}

func TestUninstallRemovesCreatedDest(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "created.txt")
	require.NoError(t, p.archiveBefore("make-copies", []string{dest}))
	require.NoError(t, p.mutate("make-copies", "create", dest, dest, opInfo{kind: "copy"}, func() error {
		return os.WriteFile(dest, []byte("new"), 0o644)
	}))
	require.FileExists(t, dest)

	require.NoError(t, uninstallerOver(t, p).Uninstall())
	require.NoFileExists(t, dest)
}

func TestUninstallRestoresPreExisting(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "pre.txt")
	require.NoError(t, os.WriteFile(dest, []byte("original"), 0o644))
	require.NoError(t, p.archiveBefore("make-copies", []string{dest}))
	require.NoError(t, p.mutate("make-copies", "create", dest, dest, opInfo{kind: "copy"}, func() error {
		return os.WriteFile(dest, []byte("overwritten"), 0o644)
	}))

	require.NoError(t, uninstallerOver(t, p).Uninstall())
	body, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, "original", string(body))
}

func TestUninstallSkipsDrift(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "drift.txt")
	require.NoError(t, p.archiveBefore("make-copies", []string{dest}))
	require.NoError(t, p.mutate("make-copies", "create", dest, dest, opInfo{kind: "copy"}, func() error {
		return os.WriteFile(dest, []byte("che"), 0o644)
	}))
	recorded, err := os.Stat(dest)
	require.NoError(t, err)
	drifted := recorded.Mode().Perm() ^ 0o100
	require.NoError(t, os.Chmod(dest, drifted))

	require.NoError(t, uninstallerOver(t, p).Uninstall())
	require.FileExists(t, dest)
	fi, err := os.Stat(dest)
	require.NoError(t, err)
	require.Equal(t, drifted, fi.Mode().Perm())
}

func TestUninstallInverseExcludesFromInstalled(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "gone.txt")
	require.NoError(t, p.archiveBefore("make-copies", []string{dest}))
	require.NoError(t, p.mutate("make-copies", "create", dest, dest, opInfo{kind: "copy"}, func() error {
		return os.WriteFile(dest, []byte("x"), 0o644)
	}))
	require.NoError(t, uninstallerOver(t, p).Uninstall())

	got, err := p.Ledger.Installed()
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestUninstallDryRunWritesNothing(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "dry.txt")
	require.NoError(t, p.archiveBefore("make-copies", []string{dest}))
	require.NoError(t, p.mutate("make-copies", "create", dest, dest, opInfo{kind: "copy"}, func() error {
		return os.WriteFile(dest, []byte("x"), 0o644)
	}))
	u := uninstallerOver(t, p)
	u.dryRun = true
	u.p.opts = options.Options{DryRun: options.DryRun.Delta}

	require.NoError(t, u.Uninstall())
	require.FileExists(t, dest)
}

// [<] 🤖🤖
