package testutil

// [>] 🤖

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoWritesAndCommits(t *testing.T) {
	dir := Repo(t, map[string]string{"a.txt": "hi\n", "sub/b.txt": "yo\n"})

	if b, err := os.ReadFile(filepath.Join(dir, "sub/b.txt")); err != nil || string(b) != "yo\n" {
		t.Errorf("WriteTree: sub/b.txt = %q, %v", b, err)
	}
	if fi, err := os.Stat(filepath.Join(dir, ".git")); err != nil || !fi.IsDir() {
		t.Error("Repo did not produce a git repo (.git missing)")
	}
}

func TestCaptureStdout(t *testing.T) {
	out, err := CaptureStdout(t, func() error {
		os.Stdout.WriteString("hello")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Errorf("CaptureStdout = %q, want hello", out)
	}
}

// [<] 🤖
