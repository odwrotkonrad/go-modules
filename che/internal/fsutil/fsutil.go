package fsutil

// [>] 🤖🤖

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
	return f.runEscalated(dest, "chmod", chmodArg, dest)
}

func (f FS) MakeSymlink(target, dest string) error {
	noDeref := "-n"
	if runtime.GOOS == "darwin" {
		noDeref = "-h"
	}
	return f.runEscalated(dest, "ln", "-fs", noDeref, target, dest)
}

func (f FS) CopyFile(src, dest string, mode os.FileMode) error {
	return f.runEscalated(dest, buildInstallArgv(mode, "", src, dest)...)
}

func (f FS) RemoveFile(dest string) error {
	return f.runEscalated(dest, "rm", "-f", dest)
}

func (f FS) RemoveDir(dest string) error {
	return f.runEscalated(dest, "rmdir", dest)
}

func (f FS) ChangeOwner(owner, dest string) error {
	return f.runEscalated(dest, "chown", owner, dest)
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

	return f.runEscalated(dest, buildInstallArgv(mode, owner, tmp.Name(), dest)...)
}

func buildInstallArgv(mode os.FileMode, owner, src, dest string) []string {
	argv := append([]string{"install"}, buildModeFlag(mode)...)
	if owner != "" {
		user, group, _ := strings.Cut(owner, ":")
		argv = append(argv, "-o", user, "-g", group)
	}
	return append(argv, src, dest)
}

func (f FS) runEscalated(dest string, argv ...string) error {
	return run(f.escalate(dest, argv))
}

func run(argv []string) error {
	return execx.Default.Exec(execx.Cmd{Argv: argv, Stdout: os.Stdout, Stderr: os.Stderr})
}

func FormatModeArg(mode os.FileMode) string { return fmt.Sprintf("%04o", mode) }

func buildModeFlag(mode os.FileMode) []string {
	if mode == 0 {
		return nil
	}
	return []string{"-m", FormatModeArg(mode)}
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func MergeMap[K comparable, V any](base, overlay map[K]V) map[K]V {
	if len(base) == 0 {
		return overlay
	}
	out := maps.Clone(base)
	maps.Copy(out, overlay)
	return out
}

// [<] 🤖🤖
