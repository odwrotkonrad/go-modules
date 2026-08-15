package database

// [>] 🤖🤖

import (
	"embed"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func openTemp(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "ops.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
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

type seedBackup struct {
	Path string `yaml:"path"`
	Op   string `yaml:"op"`
}

type seedOp struct {
	Profile     string `yaml:"profile"`
	OpType      string `yaml:"opType"`
	Kind        string `yaml:"kind"`
	Dest        string `yaml:"dest"`
	SrcRel      string `yaml:"srcRel"`
	Mode        string `yaml:"mode"`
	PrevKind    string `yaml:"prevKind"`
	NextKind    string `yaml:"nextKind"`
	NextPresent bool   `yaml:"nextPresent"`
	NextMode    string `yaml:"nextMode"`
	BackupPath  string `yaml:"backupPath"`
}

type seedSpec struct {
	RunID    string       `yaml:"runId"`
	Command  string       `yaml:"command"`
	Profiles []string     `yaml:"profiles"`
	Backups  []seedBackup `yaml:"backups"`
	Ops      []seedOp     `yaml:"ops"`
}

type opGot struct {
	Dest        string `yaml:"dest"`
	OpType      string `yaml:"opType,omitempty"`
	Kind        string `yaml:"kind,omitempty"`
	SrcRel      string `yaml:"srcRel,omitempty"`
	Mode        string `yaml:"mode,omitempty"`
	PrevKind    string `yaml:"prevKind,omitempty"`
	NextPresent bool   `yaml:"nextPresent,omitempty"`
	ProfileRef  string `yaml:"profileRef,omitempty"`
	BackupPath  string `yaml:"backupPath,omitempty"`
}

type backupGot struct {
	Path  string `yaml:"path"`
	RunID string `yaml:"runId"`
}

type dedupGot struct {
	Same       bool `yaml:"same"`
	SpecLinked bool `yaml:"specLinked"`
}

type queriesGot struct {
	Ops     []opGot     `yaml:"ops,omitempty"`
	Backups []backupGot `yaml:"backups,omitempty"`
	Dedup   *dedupGot   `yaml:"dedup,omitempty"`
}

func summarizeOps(rows []OperationDone) []opGot {
	got := make([]opGot, 0, len(rows))
	for _, r := range rows {
		g := opGot{
			Dest: r.Dest, OpType: r.OpType, Kind: r.Kind, SrcRel: r.SrcRel, Mode: r.Mode,
			PrevKind: r.Prev.Kind, NextPresent: r.Next.Present, ProfileRef: r.ProfileRef,
		}
		if r.Backup != nil {
			g.BackupPath = r.Backup.Path
		}
		got = append(got, g)
	}
	return got
}

func seedDB(t *testing.T, db *DB, specs []seedSpec) map[string]*ProfileDone {
	t.Helper()
	profiles := map[string]*ProfileDone{}
	for _, s := range specs {
		spec, err := db.StartSpec(s.RunID, "che.yml", s.Command)
		require.NoError(t, err)
		for _, ref := range s.Profiles {
			prof, err := db.StartProfile(spec, ref, ref, "che.yml", "/repo")
			require.NoError(t, err)
			profiles[ref] = prof
		}
		backupsByPath := map[string]*Backup{}
		for _, b := range s.Backups {
			backup, err := db.EnsureBackup(spec, b.Path, b.Op)
			require.NoError(t, err)
			backupsByPath[b.Path] = backup
		}
		for _, o := range s.Ops {
			done := OperationDone{
				OpType: o.OpType, Kind: o.Kind, Dest: o.Dest, SrcRel: o.SrcRel, Mode: o.Mode,
				Prev: Object{Kind: o.PrevKind},
				Next: Object{Kind: o.NextKind, Present: o.NextPresent, Mode: o.NextMode},
			}
			if o.BackupPath != "" {
				require.NotNilf(t, backupsByPath[o.BackupPath], "op backupPath %q not seeded", o.BackupPath)
				done.BackupID = &backupsByPath[o.BackupPath].ID
			}
			require.NoError(t, db.RecordOperation(profiles[o.Profile], done))
		}
	}
	return profiles
}

func TestQueries(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/queries.test.spec.yml", func(t *testing.T, c testyml.Case[queriesGot]) (queriesGot, error) {
		db := openTemp(t)
		var specs []seedSpec
		c.Input.Args.To(t, 0, &specs)
		seedDB(t, db, specs)

		query, arg, _ := strings.Cut(c.Input.Args.String(t, 1), " ")
		switch query {
		case "installed":
			rows, err := db.Installed()
			if err != nil {
				return queriesGot{}, err
			}
			return queriesGot{Ops: summarizeOps(rows)}, nil
		case "installedForProfile":
			rows, err := db.InstalledForProfile(arg)
			if err != nil {
				return queriesGot{}, err
			}
			return queriesGot{Ops: summarizeOps(rows)}, nil
		case "latestOps":
			rows, err := db.LatestOps()
			if err != nil {
				return queriesGot{}, err
			}
			return queriesGot{Ops: summarizeOps(rows)}, nil
		case "backups":
			rows, err := db.Backups()
			if err != nil {
				return queriesGot{}, err
			}
			got := make([]backupGot, 0, len(rows))
			for _, b := range rows {
				got = append(got, backupGot{Path: b.Path, RunID: b.RunID})
			}
			return queriesGot{Backups: got}, nil
		case "ensureBackupDedup":
			path, op, _ := strings.Cut(arg, " ")
			spec, err := db.StartSpec("dedupspecrun", "che.yml", "all")
			require.NoError(t, err)
			b1, err := db.EnsureBackup(spec, path, op)
			require.NoError(t, err)
			b2, err := db.EnsureBackup(spec, path, op)
			require.NoError(t, err)
			return queriesGot{Dedup: &dedupGot{Same: b1.ID == b2.ID, SpecLinked: b1.SpecDoneID == spec.ID}}, nil
		default:
			t.Fatalf("unknown query %q", query)
			return queriesGot{}, nil
		}
	})
}

// [<] 🤖🤖
