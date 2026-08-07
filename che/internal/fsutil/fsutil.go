package fsutil

// [>] 🤖🤖

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-git/go-git/v5"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
)

type FileSystemWriter interface {
	MakeDir(dest string, mode os.FileMode, parents bool) error
	ChangeMode(chmodArg, dest string) error
	MakeSymlink(target, dest string) error
	CopyFile(src, dest string, mode os.FileMode) error
	RemoveFile(dest string) error
	RemoveDir(dest string) error
	ChangeOwner(owner, dest string) error
	InstallFile(dest string, body []byte, mode os.FileMode, owner string) error
	ArchiveDestinations(archivePath string, dests []string) error
}

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

func IsUnder(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+"/")
}

func (f FS) IsUnderHome(dest string) bool {
	return IsUnder(dest, f.Home)
}

func (f FS) escalate(dest string, argv []string) []string {
	if !f.IsUnderHome(dest) && os.Geteuid() != 0 {
		return append([]string{"sudo"}, argv...)
	}
	return argv
}

func (f FS) MakeDir(dest string, mode os.FileMode, parents bool) error {
	return run(f.BuildMkdirArgv(dest, mode, parents))
}

func (f FS) BuildMkdirArgv(dest string, mode os.FileMode, parents bool) []string {
	argv := []string{"mkdir"}
	if parents {
		argv = append(argv, "-p")
	}
	argv = append(argv, buildModeFlag(mode)...)
	argv = append(argv, dest)
	return f.escalate(dest, argv)
}

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

func (f FS) RemoveDir(dest string) error {
	return f.RunPrivileged(dest, "rmdir", dest)
}

func (f FS) ChangeOwner(owner, dest string) error {
	return f.RunPrivileged(dest, "chown", owner, dest)
}

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

func buildInstallArgv(mode os.FileMode, owner, src, dest string) []string {
	argv := append([]string{"install"}, buildModeFlag(mode)...)
	if owner != "" {
		o, g, _ := strings.Cut(owner, ":")
		argv = append(argv, "-o", o, "-g", g)
	}
	return append(argv, src, dest)
}

func (f FS) RunPrivileged(dest string, argv ...string) error {
	return run(f.escalate(dest, argv))
}

func run(argv []string) error {
	return execx.Default.Exec(execx.Cmd{Argv: argv, Stdout: os.Stdout, Stderr: os.Stderr})
}

func FormatModeArg(m os.FileMode) string { return fmt.Sprintf("%04o", m) }

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

func ExpandHome(p, home string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return p
}

func MergeMap[K comparable, V any](base, overlay map[K]V) map[K]V {
	if len(base) == 0 {
		return overlay
	}
	out := maps.Clone(base)
	maps.Copy(out, overlay)
	return out
}

func IsNoRepo(err error) bool { return errors.Is(err, git.ErrRepositoryNotExists) }

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

func ResolveRepoRoot(dir string) (string, error) {
	_, root, err := openRepo(dir)
	return root, err
}

func ListTrackedFiles(root string) ([]string, error) {
	repo, repoRoot, err := openRepo(root)
	if err != nil {
		if IsNoRepo(err) {
			return listFilesWalk(root)
		}
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
			continue
		}
		files = append(files, rel)
	}
	return files, nil
}

func listFilesWalk(root string) ([]string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if abs, err = filepath.EvalSymlinks(abs); err != nil {
		return nil, err
	}
	var files []string
	err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == git.GitDirName {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() && d.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		rel, err := filepath.Rel(abs, p)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list files under %s: %w", root, err)
	}
	return files, nil
}

// [<] 🤖🤖
