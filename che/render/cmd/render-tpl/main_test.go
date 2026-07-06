package main

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunRenders(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "t.tmpl.md")
	os.WriteFile(tpl, []byte("hi {{ env.Getenv \"WHO\" }}\n"), 0o644)
	t.Setenv("WHO", "there")
	if code := run([]string{"-f", tpl}); code != 0 {
		t.Fatalf("run = %d, want 0", code)
	}
}

func TestRunReadBody(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "d.md"), []byte("---\nk: v\n---\nBODY\n"), 0o644)
	tpl := filepath.Join(dir, "t.tmpl.md")
	os.WriteFile(tpl, []byte("{{ readBody \"d.md\" }}"), 0o644)
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	if code := run([]string{"-f", tpl}); code != 0 {
		t.Fatalf("run = %d, want 0", code)
	}
}

func TestRunBadArgs(t *testing.T) {
	if code := run([]string{"nope"}); code != 11 {
		t.Errorf("bad args = %d, want 11", code)
	}
	if code := run([]string{"-f", "/no/such/file"}); code != 13 {
		t.Errorf("missing file = %d, want 13", code)
	}
}

//[<] 🤖🤖
