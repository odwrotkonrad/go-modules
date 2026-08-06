package che

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

// restorerOver runs the restore algorithm over p's ledger, reusing p's real
// seams (its own run row keeps restore ops distinct).
func restorerOver(t *testing.T, p *ProfileReady, opts options.Options) *Restorer {
	t.Helper()
	spec, err := p.Ledger.StartSpec(p.runID+"-re", "", "backup-restore")
	require.NoError(t, err)
	prof, err := p.Ledger.StartProfile(spec, "restore", "restore", "", p.home)
	require.NoError(t, err)
	rp := *p
	rp.ref, rp.specDone, rp.profileDone, rp.opts = "restore", spec, prof, opts
	rp.logDepth = 1
	return &Restorer{p: &rp, dryRun: opts.DryRun != options.DryRun.Off}
}

// nextRun clones p as a fresh run: its own spec/profile rows, run id, run ts.
func nextRun(t *testing.T, p *ProfileReady, runID, runTs string) *ProfileReady {
	t.Helper()
	spec, err := p.Ledger.StartSpec(runID, "", "test")
	require.NoError(t, err)
	prof, err := p.Ledger.StartProfile(spec, "cli", "cli", "", p.home)
	require.NoError(t, err)
	np := *p
	np.runID, np.runTs, np.specDone, np.profileDone = runID, runTs, spec, prof
	np.backedUp, np.currentArchive, np.currentSub = false, "", ""
	return &np
}

// mutateCopy runs one recorded make-copies mutation writing content onto dest,
// its pre-state archived.
func mutateCopy(t *testing.T, p *ProfileReady, dest, content string) {
	t.Helper()
	require.NoError(t, p.archiveBefore("make-copies", []string{dest}))
	require.NoError(t, p.mutate("make-copies", "create", dest, dest, opInfo{kind: "copy"}, func() error {
		return os.WriteFile(dest, []byte(content), 0o644)
	}))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(body)
}

func TestRestoreByRunIDRestoresThatRunsArchives(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "f.txt")
	require.NoError(t, os.WriteFile(dest, []byte("original"), 0o644))
	mutateCopy(t, p, dest, "installed")

	require.NoError(t, restorerOver(t, p, options.Options{}).Restore(RestoreSelector{RunID: testRunID}))
	require.Equal(t, "original", readFile(t, dest))
}

func TestRestoreByBackupIDRestoresOneArchive(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "f.txt")
	require.NoError(t, os.WriteFile(dest, []byte("original"), 0o644))
	mutateCopy(t, p, dest, "installed")
	backups, err := p.Ledger.Backups()
	require.NoError(t, err)
	require.Len(t, backups, 1)
	_, backupID := fsutil.ParseBackupArchiveName(backups[0].Path)
	require.NotEmpty(t, backupID)

	require.NoError(t, restorerOver(t, p, options.Options{}).Restore(RestoreSelector{BackupID: backupID}))
	require.Equal(t, "original", readFile(t, dest))
}

func TestRestoreByTimestampPicksLatestAtOrBefore(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "f.txt")
	require.NoError(t, os.WriteFile(dest, []byte("v0"), 0o644))
	run1 := nextRun(t, p, "run1aaaaaaaa", "20240101T000000")
	mutateCopy(t, run1, dest, "v1")
	run2 := nextRun(t, p, "run2bbbbbbbb", "20240201T000000")
	mutateCopy(t, run2, dest, "v2")

	// [why] latest backup <= T: between the runs only run1's archive qualifies,
	// holding the pre-run1 state.
	require.NoError(t, restorerOver(t, p, options.Options{}).Restore(RestoreSelector{Timestamp: "20240115T000000"}))
	require.Equal(t, "v0", readFile(t, dest))

	mutateCopy(t, nextRun(t, p, "run3cccccccc", "20240301T000000"), dest, "v3")
	require.NoError(t, restorerOver(t, p, options.Options{}).Restore(RestoreSelector{Timestamp: "20240215T000000"}))
	require.Equal(t, "v1", readFile(t, dest)) // run2's archive captured v1
}

func TestRestoreByTimestampBeforeEveryBackupErrors(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "f.txt")
	mutateCopy(t, nextRun(t, p, "run1aaaaaaaa", "20240101T000000"), dest, "v1")

	err := restorerOver(t, p, options.Options{}).Restore(RestoreSelector{Timestamp: "20230101T000000"})
	require.ErrorContains(t, err, "no backup matches")
}

func TestRestoreSkipsDriftedDest(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "f.txt")
	require.NoError(t, os.WriteFile(dest, []byte("original"), 0o644))
	mutateCopy(t, p, dest, "installed")
	recorded, err := os.Stat(dest)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(dest, recorded.Mode().Perm()^0o100)) // user drift

	require.NoError(t, restorerOver(t, p, options.Options{}).Restore(RestoreSelector{RunID: testRunID}))
	require.Equal(t, "installed", readFile(t, dest)) // drifted dest left untouched
}

func TestRestoreDryRunWritesNothing(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "f.txt")
	require.NoError(t, os.WriteFile(dest, []byte("original"), 0o644))
	mutateCopy(t, p, dest, "installed")
	r := restorerOver(t, p, options.Options{DryRun: options.DryRun.Delta})

	out, err := testutil.CaptureStdout(t, func() error { return r.Restore(RestoreSelector{RunID: testRunID}) })
	require.NoError(t, err)
	require.Contains(t, testutil.StripANSI(out), "restore "+dest+" (dry run)")
	require.Equal(t, "installed", readFile(t, dest))
}

func TestRestoreSelectorErrors(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "f.txt")
	mutateCopy(t, p, dest, "installed")
	r := restorerOver(t, p, options.Options{})

	require.ErrorContains(t, r.Restore(RestoreSelector{}), "--run-id")
	require.ErrorContains(t, r.Restore(RestoreSelector{RunID: "nope"}), "no backup matches")
	require.ErrorContains(t, r.Restore(RestoreSelector{BackupID: "nope"}), "no backup matches")
	require.ErrorContains(t, r.Restore(RestoreSelector{Timestamp: "bogus"}), "invalid --timestamp")
}

func TestRestoreUnreadableArchiveErrors(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "f.txt")
	require.NoError(t, os.WriteFile(dest, []byte("original"), 0o644))
	mutateCopy(t, p, dest, "installed")
	backups, err := p.Ledger.Backups()
	require.NoError(t, err)
	require.Len(t, backups, 1)
	require.NoError(t, os.Remove(backups[0].Path))

	err = restorerOver(t, p, options.Options{}).Restore(RestoreSelector{RunID: testRunID})
	require.ErrorContains(t, err, "read archive")
}

func TestListBackupsPrintsBackupPoints(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "f.txt")
	require.NoError(t, os.WriteFile(dest, []byte("original"), 0o644))
	mutateCopy(t, p, dest, "installed")
	backups, err := p.Ledger.Backups()
	require.NoError(t, err)
	require.Len(t, backups, 1)
	_, backupID := fsutil.ParseBackupArchiveName(backups[0].Path)

	ctx := Context{Env: map[string]string{"HOME": p.home}}
	out, err := testutil.CaptureStdout(t, func() error { return ListBackups(ctx) })
	require.NoError(t, err)
	stripped := testutil.StripANSI(out)
	require.Contains(t, stripped, "# backups")
	require.Contains(t, stripped, "run "+testRunID+", backup "+backupID+", "+testRunID)
	require.Contains(t, stripped, "~/") // abbreviated path
}

func TestListBackupsEmptyPrintsNothing(t *testing.T) {
	p := realProfile(t)
	ctx := Context{Env: map[string]string{"HOME": p.home}}
	out, err := testutil.CaptureStdout(t, func() error { return ListBackups(ctx) })
	require.NoError(t, err)
	require.Empty(t, testutil.StripANSI(out))
}

// [<] 🤖🤖
