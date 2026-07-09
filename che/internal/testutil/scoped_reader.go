package testutil

// [>] 🤖🤖

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ScopedReader reads the live filesystem only within Roots (the test fixture
// repo + HOME); every path outside reports as absent. Live host state (/etc,
// /Library, ...) never leaks into test results, so op output is identical on
// every platform.
type ScopedReader struct {
	Roots []string
}

func (r ScopedReader) in(path string) bool {
	return slices.ContainsFunc(r.Roots, func(root string) bool {
		return path == root || strings.HasPrefix(path, root+"/")
	})
}

func (r ScopedReader) Stat(path string) (os.FileInfo, error) {
	if !r.in(path) {
		return nil, fs.ErrNotExist
	}
	return os.Stat(path)
}

func (r ScopedReader) Lstat(path string) (os.FileInfo, error) {
	if !r.in(path) {
		return nil, fs.ErrNotExist
	}
	return os.Lstat(path)
}

func (r ScopedReader) ReadDir(path string) ([]os.DirEntry, error) {
	if !r.in(path) {
		return nil, fs.ErrNotExist
	}
	return os.ReadDir(path)
}

func (r ScopedReader) ReadFile(path string) ([]byte, error) {
	if !r.in(path) {
		return nil, fs.ErrNotExist
	}
	return os.ReadFile(path)
}

func (r ScopedReader) Readlink(path string) (string, error) {
	if !r.in(path) {
		return "", fs.ErrNotExist
	}
	return os.Readlink(path)
}

func (r ScopedReader) EvalSymlinks(path string) (string, error) {
	if !r.in(path) {
		return "", fs.ErrNotExist
	}
	return filepath.EvalSymlinks(path)
}

// [<] 🤖🤖
