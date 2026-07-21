package fsutil

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveBackupArchivePathLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CHE_STATE_HOME", filepath.Join(home, "state"))
	got := ResolveBackupArchivePath(home, "cli-macos", "make-copies", "20260721T114639", "a1b2c3d4e5f6")
	want := filepath.Join(home, "state", "backups", "cli-macos", "make-copies", "20260721T114639-a1b2c3d4e5f6.tar.bz2")
	require.Equal(t, want, got)
}

func TestSlugRef(t *testing.T) {
	require.Equal(t, "cli-macos", SlugRef("cli/macos"))
	require.Equal(t, "remote-remote-ops-remote", SlugRef("remote:remote:ops-remote"))
	require.Equal(t, "plain", SlugRef("plain"))
}

func TestParseBackupArchiveName(t *testing.T) {
	ts, id := ParseBackupArchiveName("/x/backups/cli-macos/backup/20260721T114639-a1b2c3d4e5f6.tar.bz2")
	require.Equal(t, "20260721T114639", ts)
	require.Equal(t, "a1b2c3d4e5f6", id)

	ts, id = ParseBackupArchiveName("/x/backups/legacy.tar.bz2")
	require.Equal(t, "legacy", ts)
	require.Empty(t, id)
}

// [<] 🤖🤖
