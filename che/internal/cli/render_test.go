package cli

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/che/render/lib"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func runRenderCase(t *testing.T, c testyml.Case[struct{}], dir string) (string, error) {
	t.Helper()
	root := New().Root()
	args := c.Context.CommandArgsExpanded(map[string]string{"DIR": dir})
	cmd, rest := findCmd(t, root, args)
	return testutil.CaptureStdout(t, func() error { return cmd.RunE(cmd, rest) })
}

func TestRenderTpl(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/cmds/render-tpl.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		dir := t.TempDir()
		if c.Context.Pwd != "" {
			testyml.CopyDir(t, td, c.Context.Pwd, dir)
		}
		t.Chdir(dir)
		for k, v := range c.Context.Env {
			t.Setenv(k, v)
		}
		_, err := runRenderCase(t, c, dir)
		c.Expected.Check(t, err)
	})
}

func TestRenderDirsTree(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/cmds/render-dirs-tree.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		execx.Swap(t, testutil.NewCmdMockExecutor())
		dir := testutil.Repo(t, map[string]string{"top": "x", ".hidden/file": "x", "docs/data/x": "x", "src/lib/y": "x"})
		tree, err := render.DirsTree(dir)
		require.NoError(t, err)
		testutil.WriteFile(t, filepath.Join(dir, "good.tree"), tree)
		testutil.WriteFile(t, filepath.Join(dir, "stale.tree"), "stale\n")
		t.Chdir(dir)
		_, err = runRenderCase(t, c, dir)
		c.Expected.Check(t, err)
	})
}

func TestRenderMakefileDoc(t *testing.T) {
	golden := "testdata/fixture/render/makefile-doc/Makefile"
	testyml.Run(t, td, "testdata/spec/cmds/render-makefile-doc.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		execx.Swap(t, testutil.NewCmdMockExecutor())
		doc, err := lib.Generate(golden)
		require.NoError(t, err)
		dir := testutil.Tree(t, map[string]string{
			"Makefile": testyml.ReadFile(t, td, golden),
			"good.md":  doc,
			"stale.md": "stale\n",
		})
		t.Chdir(dir)
		out, err := runRenderCase(t, c, dir)
		if c.Expected.Check(t, err) {
			return
		}
		if c.Expected.Files != "" {
			testyml.EqualExpected(t, td, c.Expected.Files, out)
		}
	})
}

// [<] 🤖🤖
