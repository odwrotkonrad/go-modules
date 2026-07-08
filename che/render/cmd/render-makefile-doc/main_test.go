package main

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/render/lib"
)

// [>] 🤖🤖
func TestGenerateGolden(t *testing.T) {
	got, err := lib.Generate("testdata/Makefile")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	wantBytes, err := os.ReadFile("testdata/expected.md")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(wantBytes) {
		t.Errorf("output mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, wantBytes)
	}
}

func TestGenerateMissing(t *testing.T) {
	if _, err := lib.Generate("testdata/nope.mk"); err == nil {
		t.Fatal("expected error for missing makefile")
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	mkSrc, _ := os.ReadFile("testdata/Makefile")
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), mkSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := lib.Generate("testdata/Makefile")
	if err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(dir, "good.md")
	stale := filepath.Join(dir, "stale.md")
	os.WriteFile(good, []byte(doc), 0o644)
	os.WriteFile(stale, []byte("stale\n"), 0o644)

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

	cases := map[string]struct {
		path string
		want int
	}{
		"match":  {good, 0},
		"differ": {stale, 22},
		"absent": {filepath.Join(dir, "absent.md"), 13},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if code := tool.Run([]string{"--check", c.path}); code != c.want {
				t.Errorf("Run(--check %s) = %d, want %d", name, code, c.want)
			}
		})
	}
}

//[<] 🤖🤖
