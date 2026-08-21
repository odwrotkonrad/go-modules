package che

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

type copiesWant struct {
	Files          map[string]string `yaml:"files"`
	Modes          map[string]string `yaml:"modes"`
	Absent         []string          `yaml:"absent"`
	Rerun          bool              `yaml:"rerun"`
	NotRerunStdOut []string          `yaml:"notRerunStdOut"`
}

func TestMakeCopies(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/make_copies.test.spec.yml", func(t *testing.T, c testyml.Case[copiesWant]) {
		testutil.RequireRegistered(t, c.Context.MockedInterfaces)
		var fetch map[string]string
		var items []spec.FileItem
		a := c.Input.Args
		for i := range a {
			switch name := a.Name(i); name {
			case "fetch":
				a.To(t, i, &fetch)
			case "items":
				a.To(t, i, &items)
			default:
				t.Fatalf("unknown arg %q", name)
			}
		}
		root := testutil.Tree(t, map[string]string{})
		home := filepath.Join(root, "home")
		require.NoError(t, os.MkdirAll(filepath.Join(home, "prompts"), 0o755))
		p := newProfile(root, home, options.Options{}).withDir(root)
		p.Fetcher = testutil.RemoteMockFetcher(fetch)
		copyOnce := func() (string, error) {
			return testutil.CaptureStdout(t, func() error { return p.makeCopies(items, nil) })
		}

		t.Cleanup(log.SwapLevel(log.Levels.Debug))
		out, err := copyOnce()
		c.Expected.Check(t, err)
		stripped := testutil.StripANSI(out)
		vars := map[string]string{"HOME": home}
		for _, f := range c.Expected.StdOut {
			testyml.MustMatch(t, stripped, testyml.Expand(f, vars))
		}

		w := c.Expected.Output
		for rel, want := range w.Files {
			assert.Equal(t, want, readDest(t, root, rel), "%s content", rel)
		}
		for rel, want := range w.Modes {
			info, statErr := os.Stat(filepath.Join(root, rel))
			require.NoErrorf(t, statErr, "stat %s", rel)
			assert.Equal(t, want, fmt.Sprintf("%04o", info.Mode().Perm()), "%s mode", rel)
		}
		for _, rel := range w.Absent {
			assert.NoFileExists(t, filepath.Join(root, rel))
		}
		if w.Rerun {
			out2, err := copyOnce()
			require.NoError(t, err)
			for _, f := range w.NotRerunStdOut {
				assert.NotContains(t, testutil.StripANSI(out2), testyml.Expand(f, vars))
			}
		}
	})
}

// [<] 🤖🤖
