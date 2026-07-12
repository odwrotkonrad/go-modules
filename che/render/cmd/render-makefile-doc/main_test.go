package main

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/che/render/lib"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

func TestGenerateGolden(t *testing.T) {
	got, err := lib.Generate("testdata/Makefile")
	require.NoError(t, err)
	want, err := os.ReadFile("testdata/expected.md")
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

func TestGenerateMissing(t *testing.T) {
	_, err := lib.Generate("testdata/nope.mk")
	assert.Error(t, err, "missing makefile")
}

// TestCheck drives --check against a match, a stale doc, and a missing file;
// the drift diff runs through the mock executor, nothing spawns.
func TestCheck(t *testing.T) {
	execx.Swap(t, testutil.NewCmdMockExecutor())
	mkSrc, err := os.ReadFile("testdata/Makefile")
	require.NoError(t, err)
	doc, err := lib.Generate("testdata/Makefile")
	require.NoError(t, err)
	dir := testutil.Tree(t, map[string]string{
		"Makefile": string(mkSrc),
		"good.md":  doc,
		"stale.md": "stale\n",
	})

	t.Chdir(dir)

	cases := map[string]struct {
		path string
		want int
	}{
		"match":  {filepath.Join(dir, "good.md"), 0},
		"differ": {filepath.Join(dir, "stale.md"), 22},
		"absent": {filepath.Join(dir, "absent.md"), 13},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tool.Run([]string{"--check", c.path})
			assert.Equal(t, c.want, yamlcfg.Code(err), "Run(--check %s)", name)
		})
	}
}

//[<] 🤖🤖
