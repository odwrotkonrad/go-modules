package main

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

// TestRunCheck drives --check against a match, a stale doc, and a missing
// file; the drift diff runs through the mock executor, nothing spawns.
func TestRunCheck(t *testing.T) {
	execx.Swap(t, testutil.NewCmdMockExecutor())
	dir := testutil.Repo(t, map[string]string{"top": "x", ".hidden/file": "x", "docs/data/x": "x", "src/lib/y": "x"})
	tree, err := render.DirsTree(dir)
	require.NoError(t, err)
	good := filepath.Join(dir, "good.tree")
	stale := filepath.Join(dir, "stale.tree")
	testutil.WriteFile(t, good, tree)
	testutil.WriteFile(t, stale, "stale\n")

	t.Chdir(dir)

	cases := map[string]struct {
		path string
		want int
	}{
		"match":  {good, 0},
		"differ": {stale, 22},
		"absent": {filepath.Join(dir, "absent.tree"), 13},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tool.Run([]string{"--check", c.path})
			assert.Equal(t, c.want, yamlcfg.Code(err), "Run(--check %s)", name)
		})
	}
}

//[<] 🤖🤖
