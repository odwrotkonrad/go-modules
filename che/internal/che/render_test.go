package che

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func readDest(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	require.NoErrorf(t, err, "read %s", rel)
	return string(b)
}

// renderWant: dest paths relative to the fixture root (home lives at home/).
// files asserts exact content, rerun re-renders and re-asserts files
// ([why] merge-upsert must be stable), absent dests must not exist.
type renderWant struct {
	Files       map[string]string   `yaml:"files"`
	Contains    map[string][]string `yaml:"contains"`
	NotContains map[string][]string `yaml:"notContains"`
	Absent      []string            `yaml:"absent"`
	Rerun       bool                `yaml:"rerun"`
}

// TestRenderTemplates: tree/dirs seed the fixture root, items decode straight
// into spec.FileItem, fetch (when named) swaps in the RemoteMockFetcher.
func TestRenderTemplates(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/render_templates.test.spec.yml", func(t *testing.T, c testyml.Case[renderWant]) {
		testutil.RequireRegistered(t, c.Context.MockedInterfaces)
		var tree, fetch map[string]string
		var dirs []string
		var items []spec.FileItem
		var skipSecrets, remote bool
		a := c.Input.Args
		for i := range a {
			switch name := a.Name(i); name {
			case "tree":
				a.To(t, i, &tree)
			case "dirs":
				a.To(t, i, &dirs)
			case "items":
				a.To(t, i, &items)
			case "skipSecrets":
				skipSecrets = a.Bool(t, i)
			case "fetch":
				a.To(t, i, &fetch)
				remote = true
			default:
				t.Fatalf("unknown arg %q", name)
			}
		}
		root := testutil.Tree(t, tree)
		for _, d := range dirs {
			require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0o755))
		}
		p := newProfile(root, filepath.Join(root, "home"), options.Options{}).withDir(root)
		if remote {
			p.Fetcher = testutil.RemoteMockFetcher(fetch)
		}
		renderOnce := func() (string, error) {
			return testutil.CaptureStdout(t, func() error { return p.renderTemplates(items, skipSecrets) })
		}

		out, err := renderOnce()
		c.Expected.Check(t, err)
		stripped := testutil.StripANSI(out)
		vars := map[string]string{"ROOT": root}
		for _, f := range c.Expected.StdOut {
			testyml.MustMatch(t, stripped, testyml.Expand(f, vars))
		}

		w := c.Expected.Output
		checkFiles := func() {
			for rel, want := range w.Files {
				assert.Equal(t, want, readDest(t, root, rel), "%s content", rel)
			}
		}
		checkFiles()
		if w.Rerun {
			_, err := renderOnce()
			require.NoError(t, err)
			checkFiles()
		}
		for rel, frags := range w.Contains {
			body := readDest(t, root, rel)
			for _, f := range frags {
				assert.Contains(t, body, f, "%s must contain %q", rel, f)
			}
		}
		for rel, frags := range w.NotContains {
			body := readDest(t, root, rel)
			for _, f := range frags {
				assert.NotContains(t, body, f, "%s must not contain %q", rel, f)
			}
		}
		for _, rel := range w.Absent {
			_, serr := os.Stat(filepath.Join(root, rel))
			assert.Truef(t, os.IsNotExist(serr), "%s must not exist", rel)
		}
	})
}

// [<] 🤖🤖
