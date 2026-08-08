package che

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/database"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func countOf(t *testing.T, attrs map[string]string, op string) string {
	t.Helper()
	delta, ok := attrs[op+".delta"]
	require.Truef(t, ok, "op %s missing in %v", op, attrs)
	return "delta=" + delta + ",all=" + attrs[op+".all"]
}

func recordHealthyLink(t *testing.T, p *ProfileReady) {
	t.Helper()
	require.NoError(t, p.Ledger.RecordOperation(p.profileDone, database.OperationDone{
		OpType: "create", Kind: "link", Dest: filepath.Join(p.home, ".config/zsh/alive"),
		Target: p.resolveSrc("_home/.config/zsh/.zshrc"),
		Prev:   database.Object{Kind: "absent"},
		Next:   database.Object{Kind: "link", Present: true},
	}))
}

func seedRenderedDest(t *testing.T, p *ProfileReady, res spec.OperationRecipes) {
	t.Helper()
	item := res.RenderTemplates.Templates[0]
	body, err := os.ReadFile(p.templateSrcPath(item))
	require.NoError(t, err)
	dest := p.resolveTemplateDests(item)[0].path
	require.NoError(t, os.MkdirAll(filepath.Dir(dest), 0o755))
	require.NoError(t, os.WriteFile(dest, body, 0o644))
}

func TestDiscoverSummary(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/discover_summary.test.spec.yml", func(t *testing.T, c testyml.Case[map[string]string]) {
		p, res, dir := setupProfile(t, options.Options{})
		ops, err := p.prepareOperations(res)
		require.NoError(t, err)
		p.OperationsReady = ops
		a := c.Input.Args
		for i := range a {
			if !a.Bool(t, i) {
				continue
			}
			switch name := a.Name(i); name {
			case "healthyLink":
				recordHealthyLink(t, p)
			case "brokenLink":
				seedBrokenLink(t, p)
			case "settleLink":
				settlers["make-links"](t, p, res)
			case "modifyTemplate":
				tpl := filepath.Join(dir, "root/_home/.config/zsh/t.ontoHost.tpl")
				require.NoError(t, os.WriteFile(tpl, []byte("changed\n"), 0o644))
			case "realRender":
				// [why] record-only writer keeps OS perms commands out; the landed
				m := testutil.ApplyMocks(t, map[string]string{
					"execx.CmdExecutor":       "testutil.CmdMockExecutor",
					"fsutil.FileSystemWriter": "testutil.FileSystemMockWriter",
					"fsutil.FileSystemReader": "testutil.FileSystemMockReader",
				})
				fs, reader := p.FS, p.Reader
				p.FS, p.Reader = m.FS, m.Reader
				_, err := testutil.CaptureStdout(t, func() error {
					return p.renderTemplates(res.RenderTemplates.Templates, false)
				})
				require.NoError(t, err)
				p.FS, p.Reader = fs, reader
				seedRenderedDest(t, p, res)
			case "seedRenderedDest":
				seedRenderedDest(t, p, res)
			default:
				t.Fatalf("unknown step %q", name)
			}
		}
		var events []log.Event
		log.SetSink(func(e log.Event) { events = append(events, e) })
		t.Cleanup(func() { log.SetSink(nil) })
		_, err = testutil.CaptureStdout(t, func() error { p.LogDiscovered(); return nil })
		require.NoError(t, err)
		require.NotEmpty(t, events, "a discovered heading event")
		attrs := events[0].Attrs
		for op, want := range c.Expected.Output {
			assert.Equalf(t, want, countOf(t, attrs, op), "op %s in %v", op, attrs)
		}
	})
}

func TestMakeDirsCountsPermsDrift(t *testing.T) {
	p, _, _ := setupProfile(t, options.Options{})
	dest := filepath.Join(p.home, "drift-dir")
	require.NoError(t, os.MkdirAll(dest, 0o755))
	require.NoError(t, os.Chmod(dest, 0o775))
	op := &MakeDirsOperationReady{Dirs: []spec.FileItem{{
		Dests: []spec.DestSpec{{Path: dest}},
		Perms: spec.Perms{Chmod: "0755"},
	}}}
	_, delta := op.counts(p)
	assert.Equal(t, 1, delta)
	require.NoError(t, os.Chmod(dest, 0o755))
	_, delta = op.counts(p)
	assert.Equal(t, 0, delta)
}

// [<] 🤖🤖
