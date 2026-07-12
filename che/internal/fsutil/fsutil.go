package fsutil

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-git/go-git/v5"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
)

// FileSystemWriter is the mutating fs surface host ops drive; FS is the real
// implementation, record-only mocks stand in for tests.
type FileSystemWriter interface {
	Mkdir(dest string, mode os.FileMode, parents bool) error
	Chmod(chmodArg, dest string) error
	Symlink(target, dest string) error
	Copy(src, dest string, mode os.FileMode) error
	Remove(dest string) error
	Chown(owner, dest string) error
	Install(dest string, body []byte, mode os.FileMode, owner string) error
	ArchiveDests(archivePath string, dests []string) error
}

// FileSystemReader is the read surface host ops consult (settled checks,
// prune scans, content diffs); OSReader is the real implementation, tests
// swap in a fixture-scoped mock so live host state never leaks into results.
type FileSystemReader interface {
	Stat(path string) (os.FileInfo, error)
	Lstat(path string) (os.FileInfo, error)
	ReadDir(path string) ([]os.DirEntry, error)
	ReadFile(path string) ([]byte, error)
	Readlink(path string) (string, error)
	EvalSymlinks(path string) (string, error)
}

// OSReader reads the live filesystem.
type OSReader struct{}

func (OSReader) Stat(path string) (os.FileInfo, error)      { return os.Stat(path) }
func (OSReader) Lstat(path string) (os.FileInfo, error)     { return os.Lstat(path) }
func (OSReader) ReadDir(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }
func (OSReader) ReadFile(path string) ([]byte, error)       { return os.ReadFile(path) }
func (OSReader) Readlink(path string) (string, error)       { return os.Readlink(path) }
func (OSReader) EvalSymlinks(path string) (string, error)   { return filepath.EvalSymlinks(path) }

// FS runs mutating fs ops, escalating priv per-dest (sudo iff dest outside
// invoking user's Home). Pure execution: no logging, no dry-run gate.
type FS struct {
	Home string
}

// IsUnder reports path inside the root tree (root itself included).
func IsUnder(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+"/")
}

// IsUnderHome reports dest in user-owned Home tree (no sudo).
func (f FS) IsUnderHome(dest string) bool {
	return IsUnder(dest, f.Home)
}

// escalate prefixes sudo unless dest is user-owned (under Home) or already root.
func (f FS) escalate(dest string, argv []string) []string {
	if !f.IsUnderHome(dest) && os.Geteuid() != 0 {
		return append([]string{"sudo"}, argv...)
	}
	return argv
}

// Mkdir makes one dir with mode. parents adds -p. mkdir builds its own
// priv-escalated argv, so it runs the command directly rather than through Priv.
func (f FS) Mkdir(dest string, mode os.FileMode, parents bool) error {
	return run(f.MkdirArgv(dest, mode, parents))
}

// MkdirArgv builds a mkdir argv, escalating per dest: root-tree -> sudo unless
// root, HOME-tree -> direct. parents adds -p. mode 0 -> no -m (mkdir honors umask).
func (f FS) MkdirArgv(dest string, mode os.FileMode, parents bool) []string {
	argv := []string{"mkdir"}
	if parents {
		argv = append(argv, "-p")
	}
	argv = append(argv, modeFlag(mode)...)
	argv = append(argv, dest)
	return f.escalate(dest, argv)
}

// Chmod applies explicit mode arg (setgid/sticky bits, not honored by mkdir mode).
func (f FS) Chmod(chmodArg, dest string) error {
	return f.Priv(dest, "chmod", chmodArg, dest)
}

func (f FS) Symlink(target, dest string) error {
	noDeref := "-n"
	if runtime.GOOS == "darwin" {
		noDeref = "-h"
	}
	return f.Priv(dest, "ln", "-fs", noDeref, target, dest)
}

func (f FS) Copy(src, dest string, mode os.FileMode) error {
	argv := append([]string{"install"}, modeFlag(mode)...)
	argv = append(argv, src, dest)
	return f.Priv(dest, argv...)
}

func (f FS) Remove(dest string) error {
	return f.Priv(dest, "rm", "-f", dest)
}

// Chown applies owner[:group].
func (f FS) Chown(owner, dest string) error {
	return f.Priv(dest, "chown", owner, dest)
}

// Install writes body to a temp, installs at dest with mode/owner, sudo iff dest
// outside Home. owner "" -> no -o/-g.
func (f FS) Install(dest string, body []byte, mode os.FileMode, owner string) error {
	tmp, err := os.CreateTemp("", "che-tmpl-*")
	if err != nil {
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := os.WriteFile(tmp.Name(), body, 0o600); err != nil {
		return err
	}

	argv := append([]string{"install"}, modeFlag(mode)...)
	if owner != "" {
		o, g, _ := strings.Cut(owner, ":")
		argv = append(argv, "-o", o, "-g", g)
	}
	argv = append(argv, tmp.Name(), dest)
	return f.Priv(dest, argv...)
}

// Priv runs argv as root unless dest under Home (user-owned).
func (f FS) Priv(dest string, argv ...string) error {
	return run(f.escalate(dest, argv))
}

func run(argv []string) error {
	return execx.Default.Exec(execx.Cmd{Argv: argv, Stdout: os.Stdout, Stderr: os.Stderr})
}

// ModeArg renders an octal mode for install/mkdir/chmod argv.
func ModeArg(m os.FileMode) string { return fmt.Sprintf("%04o", m) }

// modeFlag is ["-m", <mode>] for a set mode, nil when unset (0).
func modeFlag(m os.FileMode) []string {
	if m == 0 {
		return nil
	}
	return []string{"-m", ModeArg(m)}
}

// IsDir reports whether p is an existing directory.
func IsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// ExpandHome resolves a leading ~/ in p to home.
func ExpandHome(p, home string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return p
}

// openRepo opens the git repo containing dir (walking up for .git), returns it
// plus worktree root.
func openRepo(dir string) (*git.Repository, string, error) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, "", fmt.Errorf("open git repo from %s: %w", dir, err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, "", fmt.Errorf("worktree from %s: %w", dir, err)
	}
	root, err := filepath.EvalSymlinks(wt.Filesystem.Root())
	if err != nil {
		return nil, "", err
	}
	return repo, root, nil
}

// RepoRoot returns the git toplevel for dir (working-tree root).
func RepoRoot(dir string) (string, error) {
	_, root, err := openRepo(dir)
	return root, err
}

// TrackedFiles lists git-tracked files under root, relative to root. root may be
// a repo subtree: only entries within it returned, prefix-stripped.
func TrackedFiles(root string) ([]string, error) {
	repo, repoRoot, err := openRepo(root)
	if err != nil {
		return nil, err
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		return nil, fmt.Errorf("read git index under %s: %w", root, err)
	}
	if root, err = filepath.Abs(root); err != nil {
		return nil, err
	}
	if root, err = filepath.EvalSymlinks(root); err != nil {
		return nil, err
	}
	var files []string
	for _, e := range idx.Entries {
		abs := filepath.Join(repoRoot, e.Name)
		rel, err := filepath.Rel(root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			continue // outside the requested subtree
		}
		files = append(files, rel)
	}
	return files, nil
}

// [<] 🤖🤖
