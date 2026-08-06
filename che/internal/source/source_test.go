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

// TestDir: the cache checkout lives under the resolved CACHE base — default
// ~/.cache/che/remote-sources, CHE_CACHE_HOME (che's base directly), or
// XDG_CACHE_HOME/che.
func TestDir(t *testing.T) {
	const url = "https://example.com/x.git"
	for _, tc := range []struct {
		name, che, xdg, want string
	}{
		{"default", "", "", "/h/.cache/che/remote-sources/example.com-x"},
		{"cheOverride", "/o", "/x", "/o/remote-sources/example.com-x"},
		{"xdgBase", "", "/x", "/x/che/remote-sources/example.com-x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CHE_CACHE_HOME", tc.che)
			t.Setenv("XDG_CACHE_HOME", tc.xdg)
			assert.Equal(t, tc.want, Dir("/h", url))
		})
	}
}

// ensureWant is ensure's expected.output: silence plus files the checkout
// must carry.
type ensureWant struct {
	Silent bool     `yaml:"silent"`
	Files  []string `yaml:"files"`
}

// TestEnsure: a first call clones into the cache dir (logged as cloneRemote), a
// later one hard-resets to new upstream commits (logged as pulled); a
// no-change pull stays silent (attempt lines are debug-gated). The git CLI
// runs through the mock executor's go-git model: nothing spawns.
func TestEnsure(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/ensure_checkout.test.spec.yml", func(t *testing.T, c testyml.Case[ensureWant]) {
		execx.Swap(t, testutil.NewCmdMockExecutor())
		ResetCache()
		t.Cleanup(ResetCache)
		level := log.Levels.Info
		if c.Input.Args.Bool(t, 2) {
			level = log.Levels.Debug
		}
		prev := log.GetLevel()
		log.SetLevel(level)
		t.Cleanup(func() { log.SetLevel(prev) })
		up := testutil.Repo(t, map[string]string{"che.yml": "p: {}\n"})
		home := t.TempDir()
		url := "file://" + up
		a := c.Input.Args
		for range a.Int(t, 0) {
			_, err := testutil.CaptureStdout(t, func() error {
				_, e := EnsureCheckout(home, url)
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
			dir, e = EnsureCheckout(home, url)
			return e
		})
		require.NoError(t, err)
		assert.Equal(t, Dir(home, url), dir)
		out = testutil.StripANSI(out)
		vars := map[string]string{"URL": url, "DIR": dir, "ABBRDIR": abbreviateHome(dir, home)}
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
