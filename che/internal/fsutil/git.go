package fsutil

// [>] 🤖🤖

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
)

func IsNoRepo(err error) bool { return errors.Is(err, git.ErrRepositoryNotExists) }

func ResolveRepoRoot(dir string) (string, error) {
	_, root, err := openRepo(dir)
	return root, err
}

func openRepo(dir string) (*git.Repository, string, error) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, "", fmt.Errorf("open git repo from %s: %w", dir, err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, "", fmt.Errorf("worktree from %s: %w", dir, err)
	}
	root, err := filepath.EvalSymlinks(worktree.Filesystem.Root())
	if err != nil {
		return nil, "", err
	}
	return repo, root, nil
}

func ListTrackedFiles(root string) ([]string, error) {
	repo, repoRoot, err := openRepo(root)
	if err != nil {
		if IsNoRepo(err) {
			return walkFiles(root)
		}
		return nil, err
	}
	index, err := repo.Storer.Index()
	if err != nil {
		return nil, fmt.Errorf("read git index under %s: %w", root, err)
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if base, err = filepath.EvalSymlinks(base); err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range index.Entries {
		abs := filepath.Join(repoRoot, entry.Name)
		rel, err := filepath.Rel(base, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			continue
		}
		files = append(files, rel)
	}
	return files, nil
}

func walkFiles(root string) ([]string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if abs, err = filepath.EvalSymlinks(abs); err != nil {
		return nil, err
	}
	var files []string
	err = filepath.WalkDir(abs, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == git.GitDirName {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() && entry.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		rel, err := filepath.Rel(abs, path)
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
