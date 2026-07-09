package testutil

// [>] 🤖🤖🤖

import (
	"fmt"
	"os"
	"strings"
)

// MockFS is a record-only fsutil.FileSystemWriter: every call appends one
// formatted line, nothing touches the filesystem, nothing prints.
type MockFS struct {
	calls []string
}

func (m *MockFS) Calls() []string { return m.calls }

func (m *MockFS) record(parts ...string) error {
	m.calls = append(m.calls, strings.Join(parts, " "))
	return nil
}

func mode(mode os.FileMode) string { return fmt.Sprintf("%04o", mode) }

func (m *MockFS) Mkdir(dest string, md os.FileMode, parents bool) error {
	parts := []string{"mkdir"}
	if parents {
		parts = append(parts, "-p")
	}
	if md != 0 {
		parts = append(parts, "-m", mode(md))
	}
	return m.record(append(parts, dest)...)
}

func (m *MockFS) Chmod(chmodArg, dest string) error {
	return m.record("chmod", chmodArg, dest)
}

func (m *MockFS) Symlink(target, dest string) error {
	return m.record("symlink", target, dest)
}

func (m *MockFS) Copy(src, dest string, md os.FileMode) error {
	return m.record("copy", src, dest, mode(md))
}

func (m *MockFS) Remove(dest string) error {
	return m.record("remove", dest)
}

func (m *MockFS) Chown(owner, dest string) error {
	return m.record("chown", owner, dest)
}

func (m *MockFS) Install(dest string, body []byte, md os.FileMode, owner string) error {
	parts := []string{"install", dest, mode(md)}
	if owner != "" {
		parts = append(parts, owner)
	}
	return m.record(parts...)
}

func (m *MockFS) ArchiveDests(archivePath string, dests []string) error {
	return m.record("archive", archivePath)
}

// [<] 🤖🤖🤖
