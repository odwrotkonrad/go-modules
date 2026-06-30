package fsutil

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-git/go-git/v5"

	"gitlab.com/konradodwrot/go/che/internal/log"
)

// FS runs mutating fs ops, honoring DryRun, escalating priv per-dest (sudo iff
// dest outside invoking user's Home).
type FS struct {
	Home   string
	DryRun bool
}

// Log emits a 'title: msg' line through the dry-run gate. Title is "type" or
// "type(subtype)" (see log.Msg).
func (f FS) Log(title, msg string) { log.Msg(title, msg, f.DryRun) }

// UnderHome reports dest in user-owned Home tree (no sudo).
func (f FS) UnderHome(dest string) bool {
	return dest == f.Home || strings.HasPrefix(dest, f.Home+"/")
}

// mutate logs (verb: logArg), then unless dry-run runs argv with per-dest priv.
// One dry-run+log gate for every mutating op.
func (f FS) mutate(verb, logArg, dest string, argv ...string) error {
	if !f.DryRun {
		if err := f.Priv(dest, argv...); err != nil {
			return err
		}
	}
	f.Log(verb, logArg)
	return nil
}

// Mkdir makes one dir with mode. asUser (set, under root): owned by that user.
// parents adds -p. mkdir builds its own priv-escalated argv, so it runs the
// command directly rather than through Priv.
func (f FS) Mkdir(dest, asUser string, mode os.FileMode, parents bool) error {
	if f.DryRun {
		f.Log("mkdir", dest)
		return nil
	}
	argv := f.MkdirArgv(dest, asUser, mode, parents)
	if err := run(exec.Command(argv[0], argv[1:]...)); err != nil {
		return err
	}
	f.Log("mkdir", dest)
	return nil
}

// MkdirArgv builds a mkdir argv, escalating per dest/asUser: asUser -> sudo -u
// <user>, root-tree -> sudo unless root, HOME-tree -> direct. parents adds -p.
// mode 0 -> no -m (mkdir honors umask).
func (f FS) MkdirArgv(dest, asUser string, mode os.FileMode, parents bool) []string {
	base := []string{"mkdir"}
	if parents {
		base = append(base, "-p")
	}
	base = append(base, modeFlag(mode)...)
	base = append(base, dest)
	switch {
	case asUser != "" && os.Geteuid() == 0:
		return append([]string{"sudo", "-u", asUser}, base...)
	case !f.UnderHome(dest) && os.Geteuid() != 0:
		return append([]string{"sudo"}, base...)
	default:
		return base
	}
}

// Chmod applies explicit mode arg (setgid/sticky bits, not honored by mkdir mode).
func (f FS) Chmod(chmodArg, dest string) error {
	return f.mutate("chmod", chmodArg+" "+dest, dest, "chmod", chmodArg, dest)
}

func (f FS) Symlink(target, dest string) error {
	noDeref := "-n"
	if runtime.GOOS == "darwin" {
		noDeref = "-h"
	}
	return f.mutate("ln", dest, dest, "ln", "-fs", noDeref, target, dest)
}

func (f FS) Copy(src, dest string, mode os.FileMode) error {
	argv := append([]string{"install"}, modeFlag(mode)...)
	argv = append(argv, src, dest)
	return f.mutate("cp", dest, dest, argv...)
}

func (f FS) Remove(dest string) error {
	return f.mutate("rm", dest, dest, "rm", "-f", dest)
}

func (f FS) Chown(owner, dest string) error {
	return f.mutate("chown", owner+" "+dest, dest, "chown", owner, dest)
}

// Install writes body to a temp, installs at dest with mode/owner, sudo iff dest
// outside Home. owner "" -> no -o/-g. Honors dry-run.
func (f FS) Install(dest string, body []byte, mode os.FileMode, owner string) error {
	if f.DryRun {
		f.Log("render", dest)
		return nil
	}
	tmp, err := os.CreateTemp("", "che-tmpl-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		return err
	}
	tmp.Close()

	argv := append([]string{"install"}, modeFlag(mode)...)
	if owner != "" {
		o, g, _ := strings.Cut(owner, ":")
		argv = append(argv, "-o", o, "-g", g)
	}
	argv = append(argv, tmp.Name(), dest)
	return f.mutate("render", dest, dest, argv...)
}

// Priv runs argv as root unless dest under Home (user-owned).
func (f FS) Priv(dest string, argv ...string) error {
	if !f.UnderHome(dest) && os.Geteuid() != 0 {
		argv = append([]string{"sudo"}, argv...)
	}
	return run(exec.Command(argv[0], argv[1:]...))
}

func run(c *exec.Cmd) error {
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
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
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
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
