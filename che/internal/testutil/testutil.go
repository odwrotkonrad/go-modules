// Package testutil holds shared che test fixtures: file tree, committed git repo, stdout capture, mock registry.
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

// StripANSI removes SGR escape sequences so assertions stay style-agnostic.
func StripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

var stampRe = regexp.MustCompile(`(?m)^\d{2}:\d{2}:\d{2}\.\d{3}: `)

// StripStamps removes per-line log timestamps so adjacent lines assert as one block.
func StripStamps(s string) string { return stampRe.ReplaceAllString(s, "") }

//go:embed specs/*.yml
var specsFS embed.FS

//go:embed all:trees
var treesFS embed.FS

// Spec returns the named che.yml fixture (testutil/specs/<name>.yml).
func Spec(t *testing.T, name string) string {
	t.Helper()
	b, err := specsFS.ReadFile("specs/" + name + ".yml")
	require.NoErrorf(t, err, "read spec fixture %q", name)
	return string(b)
}

// WriteFile writes content at path, creating parent dirs.
func WriteFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// WriteTree writes each rel->content file under dir, creating parent dirs.
func WriteTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		WriteFile(t, filepath.Join(dir, rel), body)
	}
}

// Tree returns a temp dir holding the rel->content files.
func Tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	WriteTree(t, dir, files)
	return dir
}

// GitRepo inits dir as a git repo (go-git, idempotent) and commits everything in it.
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

// Repo returns a temp dir of files, committed as a git repo.
func Repo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := Tree(t, files)
	GitRepo(t, dir)
	return dir
}

// CheProfile is the profile specs/che.yml resolves under.
const CheProfile = "cli/macos"

// CheRepo materializes the mock che repo (specs/che.yml + trees/tree-che-repo
// covering every op) plus an on-disk HOME. Returns (repoDir, homeDir).
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

// WantLines asserts every fragment appears in out (order-independent, style-agnostic).
func WantLines(t *testing.T, out string, fragments ...string) {
	t.Helper()
	out = StripANSI(out)
	for _, f := range fragments {
		if !strings.Contains(out, f) {
			t.Errorf("output missing %q\n--- got ---\n%s", f, out)
		}
	}
}

// NotLine asserts the fragment does not appear in out (style-agnostic).
func NotLine(t *testing.T, out, fragment string) {
	t.Helper()
	out = StripANSI(out)
	if strings.Contains(out, fragment) {
		t.Errorf("output unexpectedly contains %q\n--- got ---\n%s", fragment, out)
	}
}

// CaptureStdout runs fn with os.Stdout piped. Returns printed output plus fn's error.
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
