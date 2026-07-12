package plugin

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

// slug: deterministic dir name per url form.
func TestSlug(t *testing.T) {
	cases := map[string]string{
		"git@gitlab.com:konradodwrot/workspace.git": "gitlab.com-konradodwrot-workspace",
		"https://gitlab.com/a/b.git":                "gitlab.com-a-b",
		"ssh://git@gitlab.com/a/b.git":              "gitlab.com-a-b",
		"file:///tmp/x":                             "tmp-x",
	}
	for url, want := range cases {
		assert.Equal(t, want, slug(url), "slug(%q)", url)
	}
}

// Ensure: first call clones into the cache dir (logged as cloned), second
// hard-resets to new upstream commits (logged as pulled); a no-change pull
// stays silent (attempt lines are debug-gated). The git CLI runs through the
// mock executor's go-git model: nothing spawns.
func TestEnsureCloneThenPull(t *testing.T) {
	execx.Swap(t, testutil.NewCmdMockExecutor())
	up := testutil.Repo(t, map[string]string{"che.yml": "p: {}\n"})
	home := t.TempDir()
	url := "file://" + up

	var dir string
	out, err := testutil.CaptureStdout(t, func() error {
		var e error
		dir, e = Ensure(home, url, "p")
		return e
	})
	require.NoError(t, err, "Ensure (clone)")
	assert.Equal(t, Dir(home, url), dir)
	_, err = os.Stat(filepath.Join(dir, "che.yml"))
	require.NoError(t, err, "clone missing che.yml")
	testutil.WantLines(t, out, "plugin(p): cloned "+url+" -> "+dir)
	testutil.NotLine(t, out, "clone "+url)

	out, err = testutil.CaptureStdout(t, func() error {
		_, e := Ensure(home, url, "p")
		return e
	})
	require.NoError(t, err, "Ensure (no-change pull)")
	assert.Empty(t, strings.TrimSpace(out), "no-change pull must stay silent")

	testutil.WriteTree(t, up, map[string]string{"extra.txt": "x\n"})
	testutil.GitRepo(t, up)
	out, err = testutil.CaptureStdout(t, func() error {
		_, e := Ensure(home, url, "p")
		return e
	})
	require.NoError(t, err, "Ensure (pull)")
	_, err = os.Stat(filepath.Join(dir, "extra.txt"))
	assert.NoError(t, err, "pull must fetch extra.txt")
	pulledRe := regexp.MustCompile(`plugin\(p\): pulled [0-9a-f]{7}\.\.[0-9a-f]{7} ` + regexp.QuoteMeta(dir))
	assert.True(t, pulledRe.MatchString(testutil.StripANSI(out)), "output %q missing pulled <old7>..<new7> line", out)
	testutil.NotLine(t, out, "pull "+dir)
}

// [<] 🤖🤖
