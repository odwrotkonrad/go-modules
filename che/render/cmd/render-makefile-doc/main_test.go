package main

// [>] 🤖🤖

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/che/render/lib"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func TestRun(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/cmds/render-makefile-doc.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		execx.Swap(t, testutil.NewCmdMockExecutor())
		doc, err := lib.Generate("testdata/Makefile")
		require.NoError(t, err)
		dir := testutil.Tree(t, map[string]string{
			"Makefile": testyml.ReadFile(t, td, "testdata/Makefile"),
			"good.md":  doc,
			"stale.md": "stale\n",
		})
		t.Chdir(dir)
		args := c.Context.CommandArgsExpanded(map[string]string{"DIR": dir})
		out, err := tool.Run(args)
		if c.Expected.Check(t, err) {
			return
		}
		if c.Expected.Files != "" {
			testyml.EqualExpected(t, td, c.Expected.Files, out)
		}
	})
}

//[<] 🤖🤖
