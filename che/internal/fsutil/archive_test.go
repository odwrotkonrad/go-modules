package fsutil

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestResolveBackupArchivePathLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CHE_STATE_HOME", filepath.Join(home, "state"))
	got := ResolveBackupArchivePath(home, "cli-macos", "make-copies", "20260721T114639", "a1b2c3d4e5f6")
	want := filepath.Join(home, "state", "backups", "cli-macos", "make-copies", "20260721T114639-a1b2c3d4e5f6.tar.bz2")
	require.Equal(t, want, got)
}

func TestSlugRef(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/slug_ref.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		return SlugRef(c.Input.Args.String(t, 0)), nil
	})
}

type archiveNameGot struct {
	Ts string `yaml:"ts"`
	ID string `yaml:"id,omitempty"`
}

func TestParseBackupArchiveName(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/parse_backup_archive_name.test.spec.yml", func(t *testing.T, c testyml.Case[archiveNameGot]) (archiveNameGot, error) {
		ts, id := ParseBackupArchiveName(c.Input.Args.String(t, 0))
		return archiveNameGot{Ts: ts, ID: id}, nil
	})
}

// [<] 🤖🤖
