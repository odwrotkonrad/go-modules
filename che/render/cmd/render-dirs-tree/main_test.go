package main

// [>] 🤖🤖

import (
	"embed"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

// TestRunCheck drives --check; the drift diff runs through the mock
// executor, nothing spawns.
func TestRunCheck(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/cmds/render-dirs-tree.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		execx.Swap(t, testutil.NewCmdMockExecutor())
		dir := testutil.Repo(t, map[string]string{"top": "x", ".hidden/file": "x", "docs/data/x": "x", "src/lib/y": "x"})
		tree, err := render.DirsTree(dir)
		require.NoError(t, err)
		testutil.WriteFile(t, filepath.Join(dir, "good.tree"), tree)
		testutil.WriteFile(t, filepath.Join(dir, "stale.tree"), "stale\n")
		t.Chdir(dir)
		args := c.Context.CommandArgs()
		for i, a := range args {
			args[i] = testyml.Expand(a, map[string]string{"DIR": dir})
		}
		_, err = tool.Run(args)
		c.Expected.Check(t, err)
	})
}

//[<] 🤖🤖
