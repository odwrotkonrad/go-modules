package render

// [>] 🤖🤖

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// mkRepo creates a fake repo under root/rel with a .git marker and, if purpose
// is non-empty, an assets/docs-agents/purpose.md carrying it.
func mkRepo(t *testing.T, root, rel, purpose string) {
	t.Helper()
	writeFile(t, filepath.Join(root, rel, ".git", "HEAD"), "ref: refs/heads/main\n")
	if purpose != "" {
		writeFile(t, filepath.Join(root, rel, purposeRelPath), purpose)
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
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "## Repo: ./alpha") || !strings.Contains(got, "Alpha does things.") {
		t.Errorf("leaf missing alpha purpose:\n%s", got)
	}
	if !strings.Contains(got, "## Repo: ./beta") || !strings.Contains(got, noPurposePlaceholder) {
		t.Errorf("leaf missing beta placeholder:\n%s", got)
	}
	if strings.Contains(got, "Subgroup:") {
		t.Errorf("leaf should have no subgroup headings:\n%s", got)
	}
	if !strings.Contains(got, `This is "leaf" directory with dirs structure containing repositories:`) {
		t.Errorf("leaf missing pwd line:\n%s", got)
	}
	// deterministic order: alpha before beta
	if strings.Index(got, "## Repo: ./alpha") > strings.Index(got, "## Repo: ./beta") {
		t.Errorf("repos not sorted:\n%s", got)
	}
}

func TestRepoGroupIndexParent(t *testing.T) {
	root := buildWorkspace(t)
	got, err := RepoGroupIndexDir(filepath.Join(root, "parent"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "## Repo: ./direct") || !strings.Contains(got, "Direct repo.") {
		t.Errorf("parent missing direct repo:\n%s", got)
	}
	// child subgroup inlined without its own section headings or tree
	if !strings.Contains(got, "## Subgroup: ./child") || strings.Contains(got, "### Repositories") {
		t.Errorf("child inline should carry a Subgroup heading, no Repositories heading:\n%s", got)
	}
	if !strings.Contains(got, "### Repo: ./child/gamma") || !strings.Contains(got, "Gamma repo.") {
		t.Errorf("parent missing grandchild repo via child inline:\n%s", got)
	}
	if strings.Count(got, "dirs structure") != 1 {
		t.Errorf("child inline should carry no tree:\n%s", got)
	}
}

func TestRepoGroupIndexWalk(t *testing.T) {
	root := buildWorkspace(t)
	idx, err := RepoGroupIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"leaf", "parent", filepath.Join("parent", "child")} {
		if _, ok := idx[want]; !ok {
			t.Errorf("walk missing subgroup %q; got keys %v", want, slices.Collect(maps.Keys(idx)))
		}
	}
	// repos are not subgroups
	if _, ok := idx[filepath.Join("leaf", "alpha")]; ok {
		t.Error("repo should not be indexed as a subgroup")
	}
	// child index lists gamma directly
	if !strings.Contains(idx[filepath.Join("parent", "child")], "## Repo: ./gamma") {
		t.Errorf("child index missing gamma:\n%s", idx[filepath.Join("parent", "child")])
	}
}

// TestRepoGroupIndexExpected: the presentation fixture (testdata/fixture/repo-group-index/group:
// one repo plus a child group of two repos, each with purpose.md) renders to the
// checked-in expected indexes. .git markers are stamped at runtime ([why] nested
// .git dirs cannot be version-controlled).
func TestRepoGroupIndexExpected(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-group")
	for _, repo := range []string{"che", "tools/render-files", "tools/configs"} {
		purpose, err := os.ReadFile(filepath.Join("testdata/fixture/repo-group-index/group", repo, purposeRelPath))
		if err != nil {
			t.Fatal(err)
		}
		mkRepo(t, dir, repo, string(purpose))
	}
	idx, err := RepoGroupIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	for rel, expected := range map[string]string{
		".":     "testdata/fixture/repo-group-index/index.expected.md",
		"tools": "testdata/fixture/repo-group-index/child-group-index.expected.md",
	} {
		want, err := os.ReadFile(expected)
		if err != nil {
			t.Fatal(err)
		}
		if idx[rel] != string(want) {
			t.Errorf("%s index mismatch:\n--- got ---\n%s\n--- want ---\n%s", rel, idx[rel], want)
		}
	}
}

// [<] 🤖🤖
