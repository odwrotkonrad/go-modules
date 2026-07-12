package testutil

// [>] 🤖🤖🤖

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
)

// mockRegistry maps each mockable interface (or swap-point var) to its one
// generic mock type, as declared in specs' context.mockedInterfaces.
var mockRegistry = map[string]string{
	"execx.CmdExecutor":       "testutil.CmdMockExecutor",
	"fsutil.FileSystemWriter": "testutil.FileSystemMockWriter",
	"fsutil.FileSystemReader": "testutil.FileSystemMockReader",
	"fsutil.UserLookup":       "testutil.UserMockLookup",
	"fsutil.GroupLookup":      "testutil.GroupMockLookup",
	"host.RemoteFetcher":      "testutil.RemoteMockFetcher",
	"host.Sleep":              "testutil.SleepMock",
}

// RequireRegistered fails on any context.mockedInterfaces pair the registry
// does not carry.
func RequireRegistered(t *testing.T, decl map[string]string) {
	t.Helper()
	for iface, mock := range decl {
		want, ok := mockRegistry[iface]
		if !ok {
			t.Fatalf("mockedInterfaces: unknown interface %q", iface)
		}
		if mock != want {
			t.Fatalf("mockedInterfaces: %s: unknown mock %q, registry has %q", iface, mock, want)
		}
	}
}

// MockSet is the full safe-double set for a command harness: the executor is
// already swapped in, the host-scoped doubles wire via WithFS and co.
type MockSet struct {
	Exec   *CmdMockExecutor
	FS     *FileSystemMockWriter
	Reader *FileSystemMockReader
}

// ApplyMocks validates decl against the registry and returns the safe-double
// set with execx.Default swapped to the mock executor. Undeclared seams still
// get the default double, never a real implementation.
func ApplyMocks(t *testing.T, decl map[string]string) *MockSet {
	t.Helper()
	RequireRegistered(t, decl)
	set := &MockSet{
		Exec:   NewCmdMockExecutor(),
		FS:     &FileSystemMockWriter{},
		Reader: &FileSystemMockReader{},
	}
	execx.Swap(t, set.Exec)
	return set
}

// SleepMock is the host.Sleep / render opSleep test double: no pacing.
func SleepMock(time.Duration) {}

// CmdMockExecutor is the execx.CmdExecutor test double: records every call,
// nothing spawns. It models the external commands che shells out to (launchd
// state across bootout/bootstrap/print, the plugin git CLI via worktree
// copies) and serves per-case scenario data fed by the harness.
type CmdMockExecutor struct {
	execx.Mock
	Fail           bool     // every call fails
	FailCmds       []string // substring-matched commands that fail
	Out            string   // canned output body
	NotLoaded      bool     // launchd initial state: service not loaded
	NoPid          bool     // launchd print reports a pid-less service
	StubbornPrints int      // prints still reporting present after bootout
	Bodies         []string // captured install file bodies

	loaded *bool             // launchd state, lazily seeded from NotLoaded
	clones map[string]string // plugin clone dir -> source url
}

// NewCmdMockExecutor returns the executor double with its command model wired.
func NewCmdMockExecutor() *CmdMockExecutor {
	m := &CmdMockExecutor{clones: map[string]string{}}
	m.Stub = m.model
	return m
}

func (m *CmdMockExecutor) model(argv []string) ([]byte, error) {
	cmd := strings.Join(argv, " ")
	for _, f := range m.FailCmds {
		if strings.Contains(cmd, f) {
			return nil, errors.New("stub: fail " + f)
		}
	}
	if m.Fail {
		return nil, errors.New("stub: exec fail")
	}
	m.captureInstallBody(argv)
	switch {
	case strings.Contains(cmd, "launchctl"):
		return m.launchctl(cmd)
	case argv[0] == "git":
		return m.git(argv[1:])
	}
	return []byte(m.Out), nil
}

// captureInstallBody snapshots the install source file: the executor caller
// deletes the temp file right after the call.
func (m *CmdMockExecutor) captureInstallBody(argv []string) {
	cmd := argv
	if len(cmd) > 0 && cmd[0] == "sudo" {
		cmd = cmd[1:]
	}
	if len(cmd) < 3 || cmd[0] != "install" {
		return
	}
	if b, err := os.ReadFile(cmd[len(cmd)-2]); err == nil {
		m.Bodies = append(m.Bodies, string(b))
	}
}

// launchctl models launchd state across the executor calls of one case:
// bootout unloads, bootstrap loads, print reports per loaded/NoPid.
func (m *CmdMockExecutor) launchctl(cmd string) ([]byte, error) {
	if m.loaded == nil {
		v := !m.NotLoaded
		m.loaded = &v
	}
	switch {
	case strings.Contains(cmd, "launchctl bootout"):
		*m.loaded = false
	case strings.Contains(cmd, "launchctl bootstrap"):
		*m.loaded = true
	case strings.Contains(cmd, "launchctl print"):
		if !*m.loaded {
			if m.StubbornPrints > 0 {
				m.StubbornPrints--
				return []byte("state = running\n"), nil
			}
			return nil, errors.New("stub: not loaded")
		}
		if m.NoPid {
			return []byte("state = running\n"), nil
		}
		return []byte("\tpid = 4242\n"), nil
	}
	return nil, nil
}

// git models the plugin CLI calls (clone / rev-parse / fetch / reset) without
// spawning git: a clone copies the file:// source worktree plus .git, a reset
// re-copies it (the cache dir contract: hard reset to the remote tip).
func (m *CmdMockExecutor) git(args []string) ([]byte, error) {
	switch {
	case args[0] == "clone":
		src, ok := strings.CutPrefix(args[len(args)-2], "file://")
		if !ok {
			return nil, fmt.Errorf("CmdMockExecutor: git clone models file:// only, got %v", args)
		}
		dir := args[len(args)-1]
		m.clones[dir] = src
		return nil, copyDirAll(src, dir)
	case args[0] == "-C" && len(args) >= 3:
		dir := args[1]
		switch args[2] {
		case "rev-parse":
			return gitHead(dir)
		case "fetch":
			return nil, nil
		case "reset":
			src, ok := m.clones[dir]
			if !ok {
				return nil, fmt.Errorf("CmdMockExecutor: git reset in unknown clone %q", dir)
			}
			if err := os.RemoveAll(dir); err != nil {
				return nil, err
			}
			return nil, copyDirAll(src, dir)
		}
	}
	return nil, fmt.Errorf("CmdMockExecutor: unmodeled git argv %v", args)
}

func gitHead(dir string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(dir, ".git/HEAD"))
	if err != nil {
		return nil, err
	}
	head := strings.TrimSpace(string(b))
	if ref, ok := strings.CutPrefix(head, "ref: "); ok {
		b, err = os.ReadFile(filepath.Join(dir, ".git", ref))
		if err != nil {
			return nil, err
		}
		head = strings.TrimSpace(string(b))
	}
	return []byte(head + "\n"), nil
}

func copyDirAll(src, dest string) error {
	return filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

// FileSystemMockWriter is a record-only fsutil.FileSystemWriter: every call
// appends one formatted line, nothing touches the filesystem, nothing prints.
type FileSystemMockWriter struct {
	calls []string
}

// Calls returns the recorded op lines.
func (m *FileSystemMockWriter) Calls() []string { return m.calls }

func (m *FileSystemMockWriter) record(parts ...string) error {
	m.calls = append(m.calls, strings.Join(parts, " "))
	return nil
}

func mode(mode os.FileMode) string { return fmt.Sprintf("%04o", mode) }

// Mkdir records the op.
func (m *FileSystemMockWriter) Mkdir(dest string, md os.FileMode, parents bool) error {
	parts := []string{"mkdir"}
	if parents {
		parts = append(parts, "-p")
	}
	if md != 0 {
		parts = append(parts, "-m", mode(md))
	}
	return m.record(append(parts, dest)...)
}

// Chmod records the op.
func (m *FileSystemMockWriter) Chmod(chmodArg, dest string) error {
	return m.record("chmod", chmodArg, dest)
}

// Symlink records the op.
func (m *FileSystemMockWriter) Symlink(target, dest string) error {
	return m.record("symlink", target, dest)
}

// Copy records the op.
func (m *FileSystemMockWriter) Copy(src, dest string, md os.FileMode) error {
	return m.record("copy", src, dest, mode(md))
}

// Remove records the op.
func (m *FileSystemMockWriter) Remove(dest string) error {
	return m.record("remove", dest)
}

// Chown records the op.
func (m *FileSystemMockWriter) Chown(owner, dest string) error {
	return m.record("chown", owner, dest)
}

// Install records the op.
func (m *FileSystemMockWriter) Install(dest string, body []byte, md os.FileMode, owner string) error {
	parts := []string{"install", dest, mode(md)}
	if owner != "" {
		parts = append(parts, owner)
	}
	return m.record(parts...)
}

// ArchiveDests records the op.
func (m *FileSystemMockWriter) ArchiveDests(archivePath string, dests []string) error {
	return m.record("archive", archivePath)
}

// FileSystemMockReader is the fsutil.FileSystemReader test double: reads pass
// through to the live filesystem only under Roots (the test fixture repo +
// HOME), Files serves Stat/ReadFile from a path->content map, every other
// path reports absent. The zero value denies all reads, so live host state
// (/etc, /Library, ...) never leaks into test results.
type FileSystemMockReader struct {
	Roots []string
	Files map[string]string
}

func (r *FileSystemMockReader) in(path string) bool {
	return slices.ContainsFunc(r.Roots, func(root string) bool {
		return path == root || strings.HasPrefix(path, root+"/")
	})
}

// Stat scopes os.Stat to Roots, reporting mapped Files as present.
func (r *FileSystemMockReader) Stat(path string) (os.FileInfo, error) {
	if r.in(path) {
		return os.Stat(path)
	}
	if _, ok := r.Files[path]; ok {
		return nil, nil
	}
	return nil, fs.ErrNotExist
}

// Lstat scopes os.Lstat to Roots.
func (r *FileSystemMockReader) Lstat(path string) (os.FileInfo, error) {
	if !r.in(path) {
		return nil, fs.ErrNotExist
	}
	return os.Lstat(path)
}

// ReadDir scopes os.ReadDir to Roots.
func (r *FileSystemMockReader) ReadDir(path string) ([]os.DirEntry, error) {
	if !r.in(path) {
		return nil, fs.ErrNotExist
	}
	return os.ReadDir(path)
}

// ReadFile scopes os.ReadFile to Roots, serving mapped Files content.
func (r *FileSystemMockReader) ReadFile(path string) ([]byte, error) {
	if r.in(path) {
		return os.ReadFile(path)
	}
	if b, ok := r.Files[path]; ok {
		return []byte(b), nil
	}
	return nil, fs.ErrNotExist
}

// Readlink scopes os.Readlink to Roots.
func (r *FileSystemMockReader) Readlink(path string) (string, error) {
	if !r.in(path) {
		return "", fs.ErrNotExist
	}
	return os.Readlink(path)
}

// EvalSymlinks scopes filepath.EvalSymlinks to Roots.
func (r *FileSystemMockReader) EvalSymlinks(path string) (string, error) {
	if !r.in(path) {
		return "", fs.ErrNotExist
	}
	return filepath.EvalSymlinks(path)
}

// UserMockLookup is the fsutil.UserLookup test double: users served from a
// map, unknown names error. The zero value knows no one.
type UserMockLookup map[string]user.User

// Lookup serves the mapped user record.
func (m UserMockLookup) Lookup(name string) (*user.User, error) {
	u, ok := m[name]
	if !ok {
		return nil, user.UnknownUserError(name)
	}
	return &u, nil
}

// GroupMockLookup is the fsutil.GroupLookup test double: groups served from a
// map, unknown names error. The zero value knows no group.
type GroupMockLookup map[string]user.Group

// Lookup serves the mapped group record.
func (m GroupMockLookup) Lookup(name string) (*user.Group, error) {
	g, ok := m[name]
	if !ok {
		return nil, user.UnknownGroupError(name)
	}
	return &g, nil
}

// RemoteMockFetcher is the host.RemoteFetcher test double: ref -> content,
// no git, unknown refs error.
type RemoteMockFetcher map[string]string

// Fetch serves the mapped ref content.
func (m RemoteMockFetcher) Fetch(ref string) (string, error) {
	content, ok := m[ref]
	if !ok {
		return "", fmt.Errorf("RemoteMockFetcher: no fixture for %q", ref)
	}
	return content, nil
}

// [<] 🤖🤖🤖
