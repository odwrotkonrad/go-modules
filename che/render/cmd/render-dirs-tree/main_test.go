package main

import (
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"

	"gitlab.com/konradodwrot/go-modules/che/render/render"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

// [>] 🤖🤖
func initRepo(t *testing.T, files []string) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	for _, f := range files {
		abs := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(f); err != nil {
			t.Fatalf("add %s: %v", f, err)
		}
	}
	return dir
}

func TestRunCheck(t *testing.T) {
	dir := initRepo(t, []string{"top", ".hidden/file", "docs/data/x", "src/lib/y"})
	tree, err := render.DirsTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(dir, "good.tree")
	stale := filepath.Join(dir, "stale.tree")
	os.WriteFile(good, []byte(tree), 0o644)
	os.WriteFile(stale, []byte("stale\n"), 0o644)

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

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
			if code := yamlcfg.Code(err); code != c.want {
				t.Errorf("Run(--check %s) = %d, want %d", name, code, c.want)
			}
		})
	}
}

//[<] 🤖🤖
