package render

// [>] 🤖🤖

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

// mkRepo creates a fake repo under root/rel with a .git marker and, if purpose
// is non-empty, an assets/docs-agents/purpose.md carrying it.
func mkRepo(t *testing.T, root, rel, purpose string) {
	t.Helper()
	testutil.WriteFile(t, filepath.Join(root, rel, ".git", "HEAD"), "ref: refs/heads/main\n")
	if purpose != "" {
		testutil.WriteFile(t, filepath.Join(root, rel, purposeRelPath), purpose)
	}
}

// workspace tree:
//
//	root/
//	  leaf/            (leaf subgroup: two repos)
//	    alpha (repo, purpose)
//	    beta  (repo, no purpose)
//	  parent/          (parent subgroup: one direct repo + one child subgroup)
//	    direct (repo, purpose)
//	    child/         (child subgroup: one repo)
//	      gamma (repo, purpose)
func buildWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mkRepo(t, root, "leaf/alpha", "# Purpose\n\nAlpha does things.\n")
	mkRepo(t, root, "leaf/beta", "")
	mkRepo(t, root, "parent/direct", "# Purpose\n\nDirect repo.\n")
	mkRepo(t, root, "parent/child/gamma", "# Purpose\n\nGamma repo.\n")
	return root
}

func TestRepoGroupIndexLeaf(t *testing.T) {
	root := buildWorkspace(t)
	got, err := RepoGroupIndexDir(filepath.Join(root, "leaf"))
	require.NoError(t, err)
	assert.Contains(t, got, "## Repo: ./alpha")
	assert.Contains(t, got, "Alpha does things.")
	assert.Contains(t, got, "## Repo: ./beta")
	assert.Contains(t, got, noPurposePlaceholder, "beta needs the placeholder")
	assert.NotContains(t, got, "Subgroup:", "leaf must have no subgroup headings")
	assert.Contains(t, got, `Repo index for "leaf". Directory tree:`)
	assert.Less(t, strings.Index(got, "## Repo: ./alpha"), strings.Index(got, "## Repo: ./beta"), "repos must sort deterministically")
}

func TestRepoGroupIndexParent(t *testing.T) {
	root := buildWorkspace(t)
	got, err := RepoGroupIndexDir(filepath.Join(root, "parent"))
	require.NoError(t, err)
	assert.Contains(t, got, "## Repo: ./direct")
	assert.Contains(t, got, "Direct repo.")
	// child subgroup inlined without its own section headings or tree
	assert.Contains(t, got, "## Subgroup: ./child")
	assert.NotContains(t, got, "### Repositories", "child inline must carry no Repositories heading")
	assert.Contains(t, got, "### Repo: ./child/gamma")
	assert.Contains(t, got, "Gamma repo.")
	assert.Equal(t, 1, strings.Count(got, "Directory tree"), "child inline must carry no tree")
}

func TestRepoGroupIndexWalk(t *testing.T) {
	root := buildWorkspace(t)
	idx, err := RepoGroupIndex(root)
	require.NoError(t, err)
	for _, want := range []string{"leaf", "parent", filepath.Join("parent", "child")} {
		assert.Containsf(t, idx, want, "walk missing subgroup %q; got keys %v", want, slices.Collect(maps.Keys(idx)))
	}
	assert.NotContains(t, idx, filepath.Join("leaf", "alpha"), "repos are not subgroups")
	assert.Contains(t, idx[filepath.Join("parent", "child")], "## Repo: ./gamma", "child index must list gamma directly")
}

// TestRepoGroupIndexExpected: the presentation fixture (testdata/fixture/repo-group-index/group:
// one repo plus a child group of two repos, each with purpose.md) renders to the
// checked-in expected indexes. .git markers are stamped at runtime ([why] nested
// .git dirs cannot be version-controlled).
func TestRepoGroupIndexExpected(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-group")
	for _, repo := range []string{"che", "tools/render-files", "tools/configs"} {
		purpose, err := os.ReadFile(filepath.Join("testdata/fixture/repo-group-index/group", repo, purposeRelPath))
		require.NoError(t, err)
		mkRepo(t, dir, repo, string(purpose))
	}
	idx, err := RepoGroupIndex(dir)
	require.NoError(t, err)
	for rel, expected := range map[string]string{
		".":     "testdata/fixture/repo-group-index/index.expected.md",
		"tools": "testdata/fixture/repo-group-index/child-group-index.expected.md",
	} {
		want, err := os.ReadFile(expected)
		require.NoError(t, err)
		assert.Equal(t, string(want), idx[rel], "%s index", rel)
	}
}

// [<] 🤖🤖
