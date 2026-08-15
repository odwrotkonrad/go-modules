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
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

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

type restoreRun struct {
	RunID   string `yaml:"runId"`
	Ts      string `yaml:"ts"`
	Content string `yaml:"content"`
}

type restoreSelectorSpec struct {
	RunID            string `yaml:"runId"`
	BackupID         string `yaml:"backupId"`
	Timestamp        string `yaml:"timestamp"`
	UseCurrentRunID  bool   `yaml:"useCurrentRunId"`
	UseFirstBackupID bool   `yaml:"useFirstBackupId"`
}

func TestRestore(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/restore.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		p := realProfile(t)
		dest := filepath.Join(p.home, "f.txt")
		if initial := c.Input.Args.String(t, 0); initial != "" {
			require.NoError(t, os.WriteFile(dest, []byte(initial), 0o644))
		}
		var runs []restoreRun
		c.Input.Args.To(t, 1, &runs)
		if c.Input.Args.Bool(t, 2) {
			mutateCopy(t, p, dest, "installed")
		}
		for _, r := range runs {
			mutateCopy(t, nextRun(t, p, r.RunID, r.Ts), dest, r.Content)
		}
		var sel restoreSelectorSpec
		c.Input.Args.To(t, 3, &sel)
		selector := RestoreSelector{RunID: sel.RunID, BackupID: sel.BackupID, Timestamp: sel.Timestamp}
		if sel.UseCurrentRunID {
			selector.RunID = p.runID
		}
		if sel.UseFirstBackupID {
			backups, err := p.Ledger.Backups()
			require.NoError(t, err)
			require.Len(t, backups, 1)
			_, backupID := fsutil.ParseBackupArchiveName(backups[0].Path)
			require.NotEmpty(t, backupID)
			selector.BackupID = backupID
		}
		if err := restorerOver(t, p, options.Options{}).Restore(selector); err != nil {
			return "", err
		}
		return readFile(t, dest), nil
	})
}

func TestRestoreSkipsDriftedDest(t *testing.T) {
	p := realProfile(t)
	dest := filepath.Join(p.home, "f.txt")
	require.NoError(t, os.WriteFile(dest, []byte("original"), 0o644))
	mutateCopy(t, p, dest, "installed")
	recorded, err := os.Stat(dest)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(dest, recorded.Mode().Perm()^0o100))

	require.NoError(t, restorerOver(t, p, options.Options{}).Restore(RestoreSelector{RunID: testRunID}))
	require.Equal(t, "installed", readFile(t, dest))
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
	require.Contains(t, stripped, "~/")
}

func TestListBackupsEmptyPrintsNothing(t *testing.T) {
	p := realProfile(t)
	ctx := Context{Env: map[string]string{"HOME": p.home}}
	out, err := testutil.CaptureStdout(t, func() error { return ListBackups(ctx) })
	require.NoError(t, err)
	require.Empty(t, testutil.StripANSI(out))
}

// [<] 🤖🤖
