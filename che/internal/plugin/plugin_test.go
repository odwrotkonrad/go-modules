package plugin

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

// ensureWant is ensure's expected.output: silence plus files the checkout
// must carry.
type ensureWant struct {
	Silent bool     `yaml:"silent"`
	Files  []string `yaml:"files"`
}

// TestEnsure: a first call clones into the cache dir (logged as cloned), a
// later one hard-resets to new upstream commits (logged as pulled); a
// no-change pull stays silent (attempt lines are debug-gated). The git CLI
// runs through the mock executor's go-git model: nothing spawns.
func TestEnsure(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/ensure_checkout.test.spec.yml", func(t *testing.T, c testyml.Case[ensureWant]) {
		execx.Swap(t, testutil.NewCmdMockExecutor())
		up := testutil.Repo(t, map[string]string{"che.yml": "p: {}\n"})
		home := t.TempDir()
		url := "file://" + up
		a := c.Input.Args
		for range a.Int(t, 0) {
			_, err := testutil.CaptureStdout(t, func() error {
				_, e := EnsureCheckout(home, url, "p")
				return e
			})
			require.NoError(t, err, "prior EnsureCheckout")
		}
		if a.Bool(t, 1) {
			testutil.WriteTree(t, up, map[string]string{"extra.txt": "x\n"})
			testutil.GitRepo(t, up)
		}
		var dir string
		out, err := testutil.CaptureStdout(t, func() error {
			var e error
			dir, e = EnsureCheckout(home, url, "p")
			return e
		})
		require.NoError(t, err)
		assert.Equal(t, Dir(home, url), dir)
		out = testutil.StripANSI(out)
		vars := map[string]string{"URL": url, "DIR": dir}
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
