package che

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// snapshotTree: sorted path + content under dir. [why] prove dry-run mutates nothing.
func snapshotTree(t *testing.T, dir string) string {
	t.Helper()
	var lines []string
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		switch {
		case fi.IsDir():
			lines = append(lines, "d "+rel)
		case fi.Mode()&os.ModeSymlink != 0:
			target, _ := os.Readlink(p)
			lines = append(lines, "l "+rel+" -> "+target)
		default:
			b, _ := os.ReadFile(p)
			lines = append(lines, "f "+rel+" = "+string(b))
		}
		return nil
	})
	require.NoError(t, err)
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// TestDryRunDelta: each op under dry-run=delta prints only dry-run lines
// (expected.stdOut pins per-op line format) and mutates nothing; the spec must
// cover every op in ops.
func TestDryRunDelta(t *testing.T) {
	covered := map[string]bool{}
	testyml.Run(t, td, "testdata/spec/cmds/dry_run_delta.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		op := strings.Join(c.Context.CommandArgs(), "-")
		run, ok := ops[op]
		require.Truef(t, ok, "unknown command %q", c.Context.Command)
		covered[op] = true
		p, res, dir := setupProfile(t, options.Options{DryRun: options.DryRun.Delta})
		before := snapshotTree(t, dir)
		out, err := testutil.CaptureStdout(t, func() error { return run(p, res) })
		require.NoErrorf(t, err, "%s dry-run", op)
		stripped := testutil.StripStamps(testutil.StripANSI(out))
		vars := map[string]string{"HOME": p.home, "REPO": dir, "ROOT": p.root()}
		for _, f := range c.Expected.StdOut {
			testyml.MustMatch(t, stripped, testyml.Expand(f, vars))
		}
		for line := range strings.SplitSeq(strings.TrimSpace(stripped), "\n") {
			if line != "" && !strings.Contains(line, "dry-run=delta") {
				t.Errorf("%s printed a non-dry-run line: %q", op, line)
			}
		}
		if after := snapshotTree(t, dir); after != before {
			t.Errorf("%s dry-run mutated the tree:\nBEFORE:\n%s\nAFTER:\n%s", op, before, after)
		}
	})
	for name := range ops {
		if !covered[name] {
			t.Errorf("op %q lacks a dry-run=delta case", name)
		}
	}
}

// settlers put one dest into desired state (link points into repo / copy
// matches), returning it.
var settlers = map[string]func(*testing.T, *ProfileReady, spec.OperationRecipes) string{
	"make-links": func(t *testing.T, p *ProfileReady, r spec.OperationRecipes) string {
		t.Helper()
		item := r.MakeLinks.Links[0]
		dest := p.toDest(item.Rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(dest), 0o755))
		require.NoError(t, os.Symlink(p.src(item.Rel), dest))
		return dest
	},
	"make-copies": func(t *testing.T, p *ProfileReady, r spec.OperationRecipes) string {
		t.Helper()
		item := r.MakeCopies.Copies[0]
		dest := p.copyDests(item)[0]
		src, err := os.ReadFile(p.src(item.Rel))
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(dest), 0o755))
		require.NoError(t, os.WriteFile(dest, src, 0o644))
		return dest
	},
}

// TestDryRunAll: a settled dest reports only under dry-run=all, delta skips it.
// expected.stdOut matches the all run, notExpected.stdOut the delta run, ${DEST}
// expands to the settled dest.
func TestDryRunAll(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/cmds/dry_run_all.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		op := strings.Join(c.Context.CommandArgs(), "-")
		settle, ok := settlers[op]
		require.Truef(t, ok, "unknown command %q", c.Context.Command)
		dir, home := testutil.CheRepo(t)
		res := makeOpRecipes(t, dir, testutil.CheProfile)
		dest := settle(t, newProfile(dir, home, options.Options{}).withDir(dir), res)
		vars := map[string]string{"DEST": dest}

		delta := newProfile(dir, home, options.Options{DryRun: options.DryRun.Delta}).withDir(dir)
		deltaOut, err := testutil.CaptureStdout(t, func() error { return ops[op](delta, res) })
		require.NoError(t, err)
		for _, f := range c.NotExpected.StdOut {
			testyml.MustNotMatch(t, testutil.StripANSI(deltaOut), testyml.Expand(f, vars))
		}

		all := newProfile(dir, home, options.Options{DryRun: options.DryRun.All}).withDir(dir)
		allOut, err := testutil.CaptureStdout(t, func() error { return ops[op](all, res) })
		require.NoError(t, err)
		for _, f := range c.Expected.StdOut {
			testyml.MustMatch(t, testutil.StripANSI(allOut), testyml.Expand(f, vars))
		}
	})
}

// [<] 🤖🤖
