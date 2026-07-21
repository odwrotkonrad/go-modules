package database

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// openTemp opens a fresh ledger DB under a temp dir.
func openTemp(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "ops.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// seedRun starts one spec + profile row for a case.
func seedRun(t *testing.T, db *DB) *ProfileDone {
	t.Helper()
	spec, err := db.StartSpec("20240101T000000", "che.yml", "all")
	require.NoError(t, err)
	prof, err := db.StartProfile(spec, "cli", "cli", "che.yml", "/repo")
	require.NoError(t, err)
	return prof
}

func TestNilDBIsNoop(t *testing.T) {
	var db *DB
	spec, err := db.StartSpec("r", "u", "c")
	require.NoError(t, err)
	require.Nil(t, spec)
	require.NoError(t, db.RecordOperation(nil, OperationDone{}))
	got, err := db.Installed()
	require.NoError(t, err)
	require.Nil(t, got)
	require.NoError(t, db.Close())
}

func TestInstalledLatestPerDest(t *testing.T) {
	db := openTemp(t)
	prof := seedRun(t, db)
	require.NoError(t, db.RecordOperation(prof, OperationDone{OpType: "create", Kind: "link", Dest: "/a"}))
	require.NoError(t, db.RecordOperation(prof, OperationDone{OpType: "noop", Kind: "link", Dest: "/a"}))
	require.NoError(t, db.RecordOperation(prof, OperationDone{OpType: "create", Kind: "copy", Dest: "/b"}))

	got, err := db.Installed()
	require.NoError(t, err)
	require.Len(t, got, 2) // one per dest, latest wins
	// newest-first
	require.Equal(t, "/b", got[0].Dest)
	require.Equal(t, "/a", got[1].Dest)
	require.Equal(t, "noop", got[1].OpType)
}

func TestInstalledExcludesRemoved(t *testing.T) {
	db := openTemp(t)
	prof := seedRun(t, db)
	require.NoError(t, db.RecordOperation(prof, OperationDone{OpType: "create", Kind: "link", Dest: "/a"}))
	require.NoError(t, db.RecordOperation(prof, OperationDone{OpType: "remove", Kind: "link", Dest: "/a"}))

	got, err := db.Installed()
	require.NoError(t, err)
	require.Empty(t, got) // latest op for /a is a remove
}

func TestInstalledForProfileScopes(t *testing.T) {
	db := openTemp(t)
	spec, err := db.StartSpec("20240101T000000", "che.yml", "all")
	require.NoError(t, err)
	a, err := db.StartProfile(spec, "profA", "profA", "che.yml", "/repo")
	require.NoError(t, err)
	b, err := db.StartProfile(spec, "profB", "profB", "che.yml", "/repo")
	require.NoError(t, err)
	require.NoError(t, db.RecordOperation(a, OperationDone{OpType: "create", Kind: "link", Dest: "/a"}))
	require.NoError(t, db.RecordOperation(b, OperationDone{OpType: "create", Kind: "link", Dest: "/b"}))

	got, err := db.InstalledForProfile("profA")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "/a", got[0].Dest)
}

func TestEnsureBackupDedupsByPath(t *testing.T) {
	db := openTemp(t)
	spec, err := db.StartSpec("20240101T000000", "che.yml", "all")
	require.NoError(t, err)
	b1, err := db.EnsureBackup(spec, "/state/backups/che-make-links-ts.tar.bz2", "make-links")
	require.NoError(t, err)
	b2, err := db.EnsureBackup(spec, "/state/backups/che-make-links-ts.tar.bz2", "make-links")
	require.NoError(t, err)
	require.Equal(t, b1.ID, b2.ID) // one row per archive path
	require.Equal(t, spec.ID, b1.SpecDoneID)
}

func TestRecordOperationRoundTripsKindColumns(t *testing.T) {
	db := openTemp(t)
	prof := seedRun(t, db)
	require.NoError(t, db.RecordOperation(prof, OperationDone{
		OpType: "create", Kind: "render", Dest: "/d", SrcRel: "x.tpl", Mode: "0644",
		Prev: Object{Kind: "absent"},
		Next: Object{Kind: "file", Present: true, Mode: "0644"},
	}))
	got, err := db.Installed()
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "render", got[0].Kind)
	require.Equal(t, "x.tpl", got[0].SrcRel)
	require.Equal(t, "0644", got[0].Mode)
	require.Equal(t, "absent", got[0].Prev.Kind)
	require.True(t, got[0].Next.Present)
}

func TestBackupPreloadedOnInstalled(t *testing.T) {
	db := openTemp(t)
	prof := seedRun(t, db)
	spec := &SpecDone{ID: prof.SpecDoneID}
	backup, err := db.EnsureBackup(spec, "/state/backups/a.tar.bz2", "make-links")
	require.NoError(t, err)
	require.NoError(t, db.RecordOperation(prof, OperationDone{
		OpType: "create", Kind: "link", Dest: "/a", BackupID: &backup.ID,
	}))
	got, err := db.Installed()
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotNil(t, got[0].Backup)
	require.Equal(t, "/state/backups/a.tar.bz2", got[0].Backup.Path)
}

func TestBackupsProjectsRunIDNewestFirst(t *testing.T) {
	db := openTemp(t)
	specA, err := db.StartSpec("runaaaaaaaaa", "che.yml", "run")
	require.NoError(t, err)
	specB, err := db.StartSpec("runbbbbbbbbb", "che.yml", "run")
	require.NoError(t, err)
	_, err = db.EnsureBackup(specA, "/b/cli/backup/t1-ida.tar.bz2", "backup")
	require.NoError(t, err)
	_, err = db.EnsureBackup(specB, "/b/cli/backup/t2-idb.tar.bz2", "backup")
	require.NoError(t, err)

	got, err := db.Backups()
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "/b/cli/backup/t2-idb.tar.bz2", got[0].Path) // newest first
	require.Equal(t, "runbbbbbbbbb", got[0].RunID)
	require.Equal(t, "runaaaaaaaaa", got[1].RunID)
}

func TestLatestOpsIncludesRemovesAndProjectsProfileRef(t *testing.T) {
	db := openTemp(t)
	prof := seedRun(t, db)
	require.NoError(t, db.RecordOperation(prof, OperationDone{OpType: "create", Kind: "copy", Dest: "/a"}))
	require.NoError(t, db.RecordOperation(prof, OperationDone{OpType: "remove", Kind: "copy", Dest: "/a", Next: Object{Kind: "absent"}}))
	require.NoError(t, db.RecordOperation(prof, OperationDone{OpType: "create", Kind: "copy", Dest: "/b"}))

	got, err := db.LatestOps()
	require.NoError(t, err)
	require.Len(t, got, 2) // one per dest, remove kept as /a's latest
	require.Equal(t, "/b", got[0].Dest)
	require.Equal(t, "cli", got[0].ProfileRef)
	require.Equal(t, "/a", got[1].Dest)
	require.Equal(t, "remove", got[1].OpType)
}

// [<] 🤖🤖
