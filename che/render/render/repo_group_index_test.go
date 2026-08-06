package render

// [>] 🤖🤖

import (
	"maps"
	"path"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// mkRepo creates a fake repo under root/rel with a .git marker and, if purpose
// is non-empty, an assets/docs-agents/purpose.md carrying it. [why] .git
// markers are stamped at runtime: nested .git dirs cannot be version-controlled.
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

func TestRepoGroupIndexDir(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/repo_group_index_dir.test.spec.yml", func(t *testing.T, c testyml.Case[testyml.Matchers]) {
		root := buildWorkspace(t)
		got, err := RepoGroupIndexDir(filepath.Join(root, c.Input.Args.String(t, 0)))
		require.NoError(t, err)
		for _, m := range c.Expected.Output {
			testyml.MustMatch(t, got, m)
		}
		for _, m := range c.NotExpected.Output {
			testyml.MustNotMatch(t, got, m)
		}
	})
}

// groupIndexWant is repo_group_index's expected.output: subgroup keys the walk
// must (not) yield, per-key content matchers, per-key expected-file indexes.
type groupIndexWant struct {
	Keys        []string          `yaml:"keys"`
	MissingKeys []string          `yaml:"missingKeys"`
	Contains    map[string]string `yaml:"contains"`
	Indexes     map[string]string `yaml:"indexes"`
}

func TestRepoGroupIndex(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/repo_group_index.test.spec.yml", func(t *testing.T, c testyml.Case[groupIndexWant]) {
		a := c.Input.Args
		dir := buildWorkspace(t)
		if fixture := a.String(t, 0); fixture != "" {
			dir = filepath.Join(t.TempDir(), "test-group")
			for _, repo := range a.Strings(t, 1) {
				mkRepo(t, dir, repo, testyml.ReadFile(t, td, path.Join(fixture, repo, purposeRelPath)))
			}
		}
		idx, err := RepoGroupIndex(dir)
		require.NoError(t, err)
		want := c.Expected.Output
		for _, k := range want.Keys {
			assert.Containsf(t, idx, filepath.FromSlash(k), "walk missing subgroup %q; got keys %v", k, slices.Collect(maps.Keys(idx)))
		}
		for _, k := range want.MissingKeys {
			assert.NotContains(t, idx, filepath.FromSlash(k), "repos are not subgroups")
		}
		for k, m := range want.Contains {
			testyml.MustMatch(t, idx[filepath.FromSlash(k)], m)
		}
		for k, expected := range want.Indexes {
			testyml.EqualExpected(t, td, expected, idx[k])
		}
	})
}

// [<] 🤖🤖
