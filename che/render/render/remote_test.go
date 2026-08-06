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
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// remoteRefWant is parse_remote_ref's expected.output: the parsed remoteRef fields.
type remoteRefWant struct {
	RepoURL string `yaml:"repoURL"`
	SSHURL  string `yaml:"sshURL"`
	Path    string `yaml:"path"`
	GitRef  string `yaml:"gitRef"`
}

func TestParseRemoteRef(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/parse_remote_ref.test.spec.yml", func(t *testing.T, c testyml.Case[remoteRefWant]) (remoteRefWant, error) {
		got, err := parseRemoteRef(c.Input.Args.String(t, 0))
		return remoteRefWant{RepoURL: got.repoURL, SSHURL: got.sshURL, Path: got.path, GitRef: got.gitRef}, err
	})
}

func TestIsRemoteRef(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/is_remote_ref.test.spec.yml", func(t *testing.T, c testyml.Case[bool]) (bool, error) {
		return IsRemoteRef(c.Input.Args.String(t, 0)), nil
	})
}

func TestRemoteFile(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/remote_file.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		url := "file://" + initRemoteFixture(t)
		return NewRemoteFetcher()(url + c.Input.Args.String(t, 0))
	})
}

func TestNewRemoteFetcher(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/new_remote_fetcher.test.spec.yml", func(t *testing.T, c testyml.Case[string]) {
		url := "file://" + initRemoteFixture(t)
		fetch := NewRemoteFetcher()
		// [why] every fetch past the first must hit the shared clone cache
		for range c.Input.Args.Int(t, 1) {
			got, err := fetch(url + c.Input.Args.String(t, 0))
			require.NoError(t, err)
			assert.Equal(t, c.Expected.Output, got)
		}
	})
}

func TestExecRemoteFile(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/exec_remote_file.test.spec.yml", func(t *testing.T, c testyml.Case[string]) {
		url := "file://" + initRemoteFixture(t)
		repoRoot := testutil.Repo(t, map[string]string{"x": "x"})
		body := testyml.Expand(c.Input.Args.String(t, 0), map[string]string{"URL": url})
		got, err := Exec("t.tpl", []byte(body), repoRoot)
		if c.Expected.Check(t, err) {
			return
		}
		testyml.MustMatch(t, string(got), c.Expected.Output)
	})
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
