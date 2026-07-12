package render

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

func TestParseRemoteRef(t *testing.T) {
	t.Run("bare host path", func(t *testing.T) {
		got, err := parseRemoteRef("gitlab.com/konradodwrot/conventions//conventions/comments/convention.md")
		require.NoError(t, err)
		assert.Equal(t, "https://gitlab.com/konradodwrot/conventions.git", got.repoURL)
		assert.Equal(t, "ssh://git@gitlab.com/konradodwrot/conventions.git", got.sshURL)
		assert.Equal(t, "conventions/comments/convention.md", got.path)
		assert.Empty(t, got.gitRef)
	})
	t.Run("ref query", func(t *testing.T) {
		got, err := parseRemoteRef("github.com/foo/bar//docs/x.md?ref=v1.2.3")
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3", got.gitRef)
		assert.Equal(t, "docs/x.md", got.path)
	})
	t.Run("explicit scheme kept verbatim", func(t *testing.T) {
		got, err := parseRemoteRef("file:///tmp/repo//a/b.md?ref=main")
		require.NoError(t, err)
		assert.Equal(t, "file:///tmp/repo", got.repoURL)
		assert.Empty(t, got.sshURL)
		assert.Equal(t, "a/b.md", got.path)
		assert.Equal(t, "main", got.gitRef)
	})
	t.Run("errors", func(t *testing.T) {
		for _, ref := range []string{"gitlab.com/x/y", "gitlab.com/x/y//", "//a.md", "gitlab.com/x/y//a.md?tag=v1"} {
			_, err := parseRemoteRef(ref)
			assert.Errorf(t, err, "parseRemoteRef(%q)", ref)
		}
	})
}

func TestRemoteFile(t *testing.T) {
	url := "file://" + initRemoteFixture(t)

	resolve := remoteFileResolver()

	t.Run("default branch", func(t *testing.T) {
		got, err := resolve(url + "//docs/note.md")
		require.NoError(t, err)
		assert.Equal(t, "main content\n", got)
	})
	t.Run("nested path", func(t *testing.T) {
		got, err := resolve(url + "//docs/deep/inner.md")
		require.NoError(t, err)
		assert.Equal(t, "inner\n", got)
	})
	t.Run("branch ref", func(t *testing.T) {
		got, err := resolve(url + "//docs/note.md?ref=feature")
		require.NoError(t, err)
		assert.Equal(t, "feature content\n", got)
	})
	t.Run("tag ref", func(t *testing.T) {
		got, err := resolve(url + "//docs/note.md?ref=v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "main content\n", got)
	})
	t.Run("missing file", func(t *testing.T) {
		_, err := resolve(url + "//docs/absent.md")
		assert.Error(t, err)
	})
	t.Run("missing ref", func(t *testing.T) {
		_, err := resolve(url + "//docs/note.md?ref=nope")
		assert.Error(t, err)
	})
}

func TestIsRemoteRef(t *testing.T) {
	for ref, want := range map[string]bool{
		"gitlab.com/acme/tools//snippets/agent.md":            true,
		"file:///tmp/repo//a/b.md?ref=main":                   true,
		"templates/agents/ro.md.ontoRepo.tpl":                 false,
		"root/HOME/.config/claude/settings.json.ontoHost.tpl": false,
		"gitlab.com/acme/tools":                               false,
	} {
		assert.Equal(t, want, IsRemoteRef(ref), "IsRemoteRef(%q)", ref)
	}
}

func TestNewRemoteFetcher(t *testing.T) {
	url := "file://" + initRemoteFixture(t)
	fetch := NewRemoteFetcher()
	for range 2 { // second fetch hits the clone cache
		got, err := fetch(url + "//docs/note.md")
		require.NoError(t, err)
		assert.Equal(t, "main content\n", got)
	}
}

func TestExecRemoteFile(t *testing.T) {
	url := "file://" + initRemoteFixture(t)
	repoRoot := testutil.Repo(t, map[string]string{"x": "x"})
	body := "{{ remoteFile \"" + url + "//docs/note.md\" }}"
	got, err := Exec("t.tpl", []byte(body), repoRoot)
	require.NoError(t, err)
	assert.Contains(t, string(got), "main content")
}

func initRemoteFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)
	sig := &object.Signature{Name: "t", Email: "t@t", When: time.Now()}
	commit := func(path, content string) {
		testutil.WriteFile(t, filepath.Join(dir, path), content)
		_, err := wt.Add(path)
		require.NoError(t, err)
		_, err = wt.Commit("c "+path, &git.CommitOptions{Author: sig})
		require.NoError(t, err)
	}
	commit("docs/note.md", "main content\n")
	commit("docs/deep/inner.md", "inner\n")
	head, err := repo.Head()
	require.NoError(t, err)
	_, err = repo.CreateTag("v1.0.0", head.Hash(), nil)
	require.NoError(t, err)
	require.NoError(t, wt.Checkout(&git.CheckoutOptions{Branch: "refs/heads/feature", Create: true}))
	commit("docs/note.md", "feature content\n")
	require.NoError(t, wt.Checkout(&git.CheckoutOptions{Branch: head.Name()}))
	return dir
}

// [<] 🤖🤖
