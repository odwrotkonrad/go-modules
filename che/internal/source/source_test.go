package source

// [>] 🤖🤖

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func TestSlug(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/slug.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		return slug(c.Input.Args.String(t, 0)), nil
	})
}

func TestResolveDir(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_dir.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		return ResolveDir(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1), c.Input.Args.String(t, 2)), nil
	})
}

type ensureWant struct {
	Silent bool     `yaml:"silent"`
	Files  []string `yaml:"files"`
}

func TestCloneURLs(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/clone_url.test.spec.yml", func(t *testing.T, c testyml.Case[[]string]) ([]string, error) {
		return CloneURLs(c.Input.Args.String(t, 0)), nil
	})
}

func TestEnsure(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/ensure_checkout.test.spec.yml", func(t *testing.T, c testyml.Case[ensureWant]) {
		execx.Swap(t, testutil.NewCmdMockExecutor())
		ResetCache()
		t.Cleanup(ResetCache)
		level := log.Levels.Info
		if c.Input.Args.Bool(t, 2) {
			level = log.Levels.Debug
		}
		t.Cleanup(log.SwapLevel(level))
		up := testutil.Repo(t, map[string]string{"che.yml": "p: {}\n"})
		home := t.TempDir()
		url := "file://" + up
		a := c.Input.Args
		gitRef := a.String(t, 4)
		if gitRef != "" {
			testutil.GitTag(t, up, gitRef)
		}
		for range a.Int(t, 0) {
			_, err := testutil.CaptureStdout(t, func() error {
				_, e := EnsureCheckout(home, url, gitRef)
				return e
			})
			require.NoError(t, err, "prior EnsureCheckout")
			ResetCache()
		}
		if a.Bool(t, 1) {
			testutil.WriteTree(t, up, map[string]string{"extra.txt": "x\n"})
			testutil.GitRepo(t, up)
		}
		if a.Bool(t, 3) {
			require.NoError(t, os.RemoveAll(up))
		}
		var dir string
		out, err := testutil.CaptureStdout(t, func() error {
			var e error
			dir, e = EnsureCheckout(home, url, gitRef)
			return e
		})
		if c.Expected.Check(t, err) {
			return
		}
		assert.Equal(t, ResolveDir(home, url, gitRef), dir)
		out = testutil.StripANSI(out)
		vars := map[string]string{"URL": url, "DIR": dir, "ABBRDIR": fsutil.AbbreviateHome(dir, home)}
		for _, m := range c.Expected.StdOut {
			testyml.MustMatch(t, out, testyml.Expand(m, vars))
		}
		for _, m := range c.NotExpected.StdOut {
			testyml.MustNotMatch(t, out, testyml.Expand(m, vars))
		}
		if c.Expected.Output.Silent {
			assert.Empty(t, strings.TrimSpace(out), "no-change pull must stay silent")
		}
		for _, f := range c.Expected.Output.Files {
			_, err := os.Stat(filepath.Join(dir, f))
			assert.NoErrorf(t, err, "checkout missing %s", f)
		}
	})
}

// [<] 🤖🤖
