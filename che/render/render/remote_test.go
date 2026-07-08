package render

// [>] 🤖🤖

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestParseRemoteRef(t *testing.T) {
	t.Run("bare host path", func(t *testing.T) {
		got, err := parseRemoteRef("gitlab.com/konradodwrot/conventions//conventions/comments/convention.md")
		if err != nil {
			t.Fatal(err)
		}
		if got.repoURL != "https://gitlab.com/konradodwrot/conventions.git" {
			t.Errorf("repoURL = %q", got.repoURL)
		}
		if got.sshURL != "ssh://git@gitlab.com/konradodwrot/conventions.git" {
			t.Errorf("sshURL = %q", got.sshURL)
		}
		if got.path != "conventions/comments/convention.md" || got.gitRef != "" {
			t.Errorf("path = %q, gitRef = %q", got.path, got.gitRef)
		}
	})
	t.Run("ref query", func(t *testing.T) {
		got, err := parseRemoteRef("github.com/foo/bar//docs/x.md?ref=v1.2.3")
		if err != nil {
			t.Fatal(err)
		}
		if got.gitRef != "v1.2.3" || got.path != "docs/x.md" {
			t.Errorf("path = %q, gitRef = %q", got.path, got.gitRef)
		}
	})
	t.Run("explicit scheme kept verbatim", func(t *testing.T) {
		got, err := parseRemoteRef("file:///tmp/repo//a/b.md?ref=main")
		if err != nil {
			t.Fatal(err)
		}
		if got.repoURL != "file:///tmp/repo" || got.sshURL != "" {
			t.Errorf("repoURL = %q, sshURL = %q", got.repoURL, got.sshURL)
		}
		if got.path != "a/b.md" || got.gitRef != "main" {
			t.Errorf("path = %q, gitRef = %q", got.path, got.gitRef)
		}
	})
	t.Run("errors", func(t *testing.T) {
		for _, ref := range []string{"gitlab.com/x/y", "gitlab.com/x/y//", "//a.md", "gitlab.com/x/y//a.md?tag=v1"} {
			if _, err := parseRemoteRef(ref); err == nil {
				t.Errorf("parseRemoteRef(%q): want error", ref)
			}
		}
	})
}

func TestRemoteFile(t *testing.T) {
	url := "file://" + initRemoteFixture(t)

	resolve := remoteFileResolver()

	t.Run("default branch", func(t *testing.T) {
		got, err := resolve(url + "//docs/note.md")
		if err != nil {
			t.Fatal(err)
		}
		if got != "main content\n" {
			t.Errorf("content = %q", got)
		}
	})
	t.Run("nested path", func(t *testing.T) {
		got, err := resolve(url + "//docs/deep/inner.md")
		if err != nil {
			t.Fatal(err)
		}
		if got != "inner\n" {
			t.Errorf("content = %q", got)
		}
	})
	t.Run("branch ref", func(t *testing.T) {
		got, err := resolve(url + "//docs/note.md?ref=feature")
		if err != nil {
			t.Fatal(err)
		}
		if got != "feature content\n" {
			t.Errorf("content = %q", got)
		}
	})
	t.Run("tag ref", func(t *testing.T) {
		got, err := resolve(url + "//docs/note.md?ref=v1.0.0")
		if err != nil {
			t.Fatal(err)
		}
		if got != "main content\n" {
			t.Errorf("content = %q", got)
		}
	})
	t.Run("missing file", func(t *testing.T) {
		if _, err := resolve(url + "//docs/absent.md"); err == nil {
			t.Error("want error for missing file")
		}
	})
	t.Run("missing ref", func(t *testing.T) {
		if _, err := resolve(url + "//docs/note.md?ref=nope"); err == nil {
			t.Error("want error for missing ref")
		}
	})
}

func TestExecRemoteFile(t *testing.T) {
	url := "file://" + initRemoteFixture(t)
	repoRoot := initRepo(t, []string{"x"})
	body := "{{ remoteFile \"" + url + "//docs/note.md\" }}"
	got, err := Exec("t.tpl", []byte(body), repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "main content") {
		t.Errorf("Exec remoteFile = %q", got)
	}
}

func initRemoteFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: "t", Email: "t@t", When: time.Now()}
	commit := func(path, content string) {
		writeFile(t, filepath.Join(dir, path), content)
		if _, err := wt.Add(path); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Commit("c "+path, &git.CommitOptions{Author: sig}); err != nil {
			t.Fatal(err)
		}
	}
	commit("docs/note.md", "main content\n")
	commit("docs/deep/inner.md", "inner\n")
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateTag("v1.0.0", head.Hash(), nil); err != nil {
		t.Fatal(err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Branch: "refs/heads/feature", Create: true}); err != nil {
		t.Fatal(err)
	}
	commit("docs/note.md", "feature content\n")
	if err := wt.Checkout(&git.CheckoutOptions{Branch: head.Name()}); err != nil {
		t.Fatal(err)
	}
	return dir
}

// [<] 🤖🤖
