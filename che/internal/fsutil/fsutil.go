// Package fsutil bundles filesystem helpers for ops: repo root and home resolution, path expansion, archiving.
package fsutil

// [>] 🤖🤖

import (
	"fmt"
	"maps"
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
	MakeDir(dest string, mode os.FileMode, parents bool) error
	ChangeMode(chmodArg, dest string) error
	MakeSymlink(target, dest string) error
	CopyFile(src, dest string, mode os.FileMode) error
	RemoveFile(dest string) error
	ChangeOwner(owner, dest string) error
	InstallFile(dest string, body []byte, mode os.FileMode, owner string) error
	ArchiveDestinations(archivePath string, dests []string) error
}

// FileSystemReader is the read surface host ops consult (settled checks,
// prune scans, content diffs). OSReader is the real implementation, tests
// swap in a fixture-scoped mock so live host state never leaks into results.
type FileSystemReader interface {
	StatPath(path string) (os.FileInfo, error)
	LstatPath(path string) (os.FileInfo, error)
	ReadDirectory(path string) ([]os.DirEntry, error)
	ReadFileBytes(path string) ([]byte, error)
	ReadLink(path string) (string, error)
	EvaluateSymlinks(path string) (string, error)
}

func (OSReader) StatPath(path string) (os.FileInfo, error)        { return os.Stat(path) }
func (OSReader) LstatPath(path string) (os.FileInfo, error)       { return os.Lstat(path) }
func (OSReader) ReadDirectory(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }
func (OSReader) ReadFileBytes(path string) ([]byte, error)        { return os.ReadFile(path) }
func (OSReader) ReadLink(path string) (string, error)             { return os.Readlink(path) }
func (OSReader) EvaluateSymlinks(path string) (string, error)     { return filepath.EvalSymlinks(path) }

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

// MakeDir runs its own priv-escalated argv (MkdirArgv), not through Priv.
func (f FS) MakeDir(dest string, mode os.FileMode, parents bool) error {
	return run(f.BuildMkdirArgv(dest, mode, parents))
}

// BuildMkdirArgv builds a per-dest-escalated mkdir argv. mode 0 -> no -m (mkdir honors umask).
func (f FS) BuildMkdirArgv(dest string, mode os.FileMode, parents bool) []string {
	argv := []string{"mkdir"}
	if parents {
		argv = append(argv, "-p")
	}
	argv = append(argv, buildModeFlag(mode)...)
	argv = append(argv, dest)
	return f.escalate(dest, argv)
}

// ChangeMode applies explicit mode arg (setgid/sticky bits, not honored by mkdir mode).
func (f FS) ChangeMode(chmodArg, dest string) error {
	return f.RunPrivileged(dest, "chmod", chmodArg, dest)
}

func (f FS) MakeSymlink(target, dest string) error {
	noDeref := "-n"
	if runtime.GOOS == "darwin" {
		noDeref = "-h"
	}
	return f.RunPrivileged(dest, "ln", "-fs", noDeref, target, dest)
}

func (f FS) CopyFile(src, dest string, mode os.FileMode) error {
	return f.RunPrivileged(dest, buildInstallArgv(mode, "", src, dest)...)
}

func (f FS) RemoveFile(dest string) error {
	return f.RunPrivileged(dest, "rm", "-f", dest)
}

func (f FS) ChangeOwner(owner, dest string) error {
	return f.RunPrivileged(dest, "chown", owner, dest)
}

// InstallFile writes body to a temp, installs at dest with mode/owner, sudo iff dest
// outside Home. owner "" -> no -o/-g.
func (f FS) InstallFile(dest string, body []byte, mode os.FileMode, owner string) error {
	tmp, err := os.CreateTemp("", "che-tmpl-*")
	if err != nil {
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := os.WriteFile(tmp.Name(), body, 0o600); err != nil {
		return err
	}

	return f.RunPrivileged(dest, buildInstallArgv(mode, owner, tmp.Name(), dest)...)
}

// buildInstallArgv builds an install argv: mode flag, optional -o/-g from
// owner ("owner:group", "" -> none), then src dest.
func buildInstallArgv(mode os.FileMode, owner, src, dest string) []string {
	argv := append([]string{"install"}, buildModeFlag(mode)...)
	if owner != "" {
		o, g, _ := strings.Cut(owner, ":")
		argv = append(argv, "-o", o, "-g", g)
	}
	return append(argv, src, dest)
}

// RunPrivileged runs argv as root unless dest under Home (user-owned).
func (f FS) RunPrivileged(dest string, argv ...string) error {
	return run(f.escalate(dest, argv))
}

func run(argv []string) error {
	return execx.Default.Exec(execx.Cmd{Argv: argv, Stdout: os.Stdout, Stderr: os.Stderr})
}

// FormatModeArg renders an octal mode for install/mkdir/chmod argv.
func FormatModeArg(m os.FileMode) string { return fmt.Sprintf("%04o", m) }

// buildModeFlag is ["-m", <mode>] for a set mode, nil when unset (0).
func buildModeFlag(m os.FileMode) []string {
	if m == 0 {
		return nil
	}
	return []string{"-m", FormatModeArg(m)}
}

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

// MergeMap merges base under overlay (overlay keys win), reusing overlay when
// base is empty (no clone).
func MergeMap[K comparable, V any](base, overlay map[K]V) map[K]V {
	if len(base) == 0 {
		return overlay
	}
	out := maps.Clone(base)
	maps.Copy(out, overlay)
	return out
}

// openRepo opens the git repo containing dir (walking up for .git).
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

// ResolveRepoRoot returns the git toplevel for dir (working-tree root).
func ResolveRepoRoot(dir string) (string, error) {
	_, root, err := openRepo(dir)
	return root, err
}

// ListTrackedFiles lists git-tracked files under root, relative to root. root may be
// a repo subtree: only entries within it returned, prefix-stripped.
func ListTrackedFiles(root string) ([]string, error) {
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
