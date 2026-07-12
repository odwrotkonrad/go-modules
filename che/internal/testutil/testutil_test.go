package testutil

// [>] 🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoWritesAndCommits(t *testing.T) {
	dir := Repo(t, map[string]string{"a.txt": "hi\n", "sub/b.txt": "yo\n"})

	b, err := os.ReadFile(filepath.Join(dir, "sub/b.txt"))
	require.NoError(t, err)
	assert.Equal(t, "yo\n", string(b))
	fi, err := os.Stat(filepath.Join(dir, ".git"))
	require.NoError(t, err, "Repo did not produce a git repo (.git missing)")
	assert.True(t, fi.IsDir())
}

func TestCaptureStdout(t *testing.T) {
	out, err := CaptureStdout(t, func() error {
		os.Stdout.WriteString("hello")
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "hello", out)
}

// [<] 🤖
