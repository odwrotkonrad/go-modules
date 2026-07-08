package plugin

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

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
		if got := slug(url); got != want {
			t.Errorf("slug(%q) = %q, want %q", url, got, want)
		}
	}
}

// Ensure: first call shallow-clones into the cache dir, second fast-forward
// pulls new upstream commits.
func TestEnsureCloneThenPull(t *testing.T) {
	up := testutil.Repo(t, map[string]string{"che.yml": "p: {}\n"})
	home := t.TempDir()
	url := "file://" + up

	dir, err := Ensure(home, url, "p")
	if err != nil {
		t.Fatalf("Ensure (clone) errored: %v", err)
	}
	if dir != Dir(home, url) {
		t.Errorf("dir = %q, want %q", dir, Dir(home, url))
	}
	if _, err := os.Stat(filepath.Join(dir, "che.yml")); err != nil {
		t.Fatalf("clone missing che.yml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git/shallow")); err != nil {
		t.Errorf("clone not shallow: %v", err)
	}

	testutil.WriteTree(t, up, map[string]string{"extra.txt": "x\n"})
	testutil.GitRepo(t, up)
	if _, err := Ensure(home, url, "p"); err != nil {
		t.Fatalf("Ensure (pull) errored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "extra.txt")); err != nil {
		t.Errorf("pull did not fetch extra.txt: %v", err)
	}
}

// [<] 🤖🤖
