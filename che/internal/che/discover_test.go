package che

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/database"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// countOf extracts one "op(delta=…,all=…)" segment from a discovered line.
func countOf(t *testing.T, summary, op string) string {
	t.Helper()
	_, rest, found := strings.Cut(summary, op+"(")
	require.Truef(t, found, "op %s missing in %q", op, summary)
	seg, _, _ := strings.Cut(rest, ")")
	return seg
}

// recordHealthyLink records a ledger link op whose source still exists.
func recordHealthyLink(t *testing.T, p *ProfileReady) {
	t.Helper()
	require.NoError(t, p.Ledger.RecordOperation(p.profileDone, database.OperationDone{
		OpType: "create", Kind: "link", Dest: filepath.Join(p.home, ".config/zsh/alive"),
		Target: p.resolveSrc("_home/.config/zsh/.zshrc"),
		Prev:   database.Object{Kind: "absent"},
		Next:   database.Object{Kind: "link", Present: true},
	}))
}

// TestDiscoverSummary: per-op all/delta counts over a tree mixing settled and
// unsettled dests, the ledger prune scan, and the render-delta mock-render
// cache — each case applies its named setup steps IN ORDER, then asserts
// discoverSummary's op segments.
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
				m := testutil.ApplyMocks(t, map[string]string{
					"fsutil.FileSystemWriter": "testutil.FileSystemMockWriter",
					"fsutil.FileSystemReader": "testutil.FileSystemMockReader",
				})
				p.FS, p.Reader = m.FS, m.Reader
				_, err := testutil.CaptureStdout(t, func() error {
					return p.renderTemplates(res.RenderTemplates.Templates, false)
				})
				require.NoError(t, err)
			default:
				t.Fatalf("unknown step %q", name)
			}
		}
		got := p.discoverSummary()
		for op, want := range c.Expected.Output {
			assert.Equalf(t, want, countOf(t, got, op), "op %s in %q", op, got)
		}
	})
}

// [<] 🤖🤖
