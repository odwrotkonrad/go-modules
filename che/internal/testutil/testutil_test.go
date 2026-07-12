package testutil

// [>] 🤖

import (
	"embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func TestRepo(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/repo.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		var files map[string]string
		c.Input.Args.To(t, 0, &files)
		dir := Repo(t, files)
		fi, err := os.Stat(filepath.Join(dir, ".git"))
		require.NoError(t, err, "Repo did not produce a git repo (.git missing)")
		assert.True(t, fi.IsDir())
		b, err := os.ReadFile(filepath.Join(dir, c.Input.Args.String(t, 1)))
		require.NoError(t, err)
		return string(b), nil
	})
}

func TestCaptureStdout(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/capture_stdout.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		return CaptureStdout(t, func() error {
			os.Stdout.WriteString(c.Input.Args.String(t, 0))
			return nil
		})
	})
}

// [<] 🤖
