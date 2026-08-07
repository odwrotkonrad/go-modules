package testutil

// [>] 🤖🤖

import (
	"embed"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func StripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

//go:embed specs/*.yml
var specsFS embed.FS

//go:embed all:trees
var treesFS embed.FS

func Spec(t *testing.T, name string) string {
	t.Helper()
	b, err := specsFS.ReadFile("specs/" + name + ".yml")
	require.NoErrorf(t, err, "read spec fixture %q", name)
	return string(b)
}

func WriteFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func WriteTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		WriteFile(t, filepath.Join(dir, rel), body)
	}
}

func Tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	WriteTree(t, dir, files)
	return dir
}

func GitRepo(t *testing.T, dir string) {
	t.Helper()
	repo, err := git.PlainInit(dir, false)
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		repo, err = git.PlainOpen(dir)
	}
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, wt.AddWithOptions(&git.AddOptions{All: true}))
	_, err = wt.Commit("x", &git.CommitOptions{
		Author:            &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
		AllowEmptyCommits: true,
	})
	require.NoError(t, err)
}

func Repo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := Tree(t, files)
	GitRepo(t, dir)
	return dir
}

const CheProfile = "cli/macos"

func CheRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	testyml.CopyDir(t, treesFS, "trees/tree-che-repo", dir)
	WriteTree(t, dir, map[string]string{"che.yml": Spec(t, "che")})
	GitRepo(t, dir)
	home := filepath.Join(dir, "home")
	require.NoError(t, os.MkdirAll(home, 0o755))
	return dir, home
}

func WantLines(t *testing.T, out string, fragments ...string) {
	t.Helper()
	out = StripANSI(out)
	for _, f := range fragments {
		if !strings.Contains(out, f) {
			t.Errorf("output missing %q\n--- got ---\n%s", f, out)
		}
	}
}

func NotLine(t *testing.T, out, fragment string) {
	t.Helper()
	out = StripANSI(out)
	if strings.Contains(out, fragment) {
		t.Errorf("output unexpectedly contains %q\n--- got ---\n%s", fragment, out)
	}
}

func CaptureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	runErr := fn()
	w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	return string(out), runErr
}

// [<] 🤖🤖
