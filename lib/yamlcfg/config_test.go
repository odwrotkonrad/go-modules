// [>] 🤖🤖
package yamlcfg

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, raw string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
}

func noSystem(t *testing.T) {
	t.Helper()
	prev := SystemDir
	SystemDir = filepath.Join(t.TempDir(), "no-system")
	t.Cleanup(func() { SystemDir = prev })
}

func codeOf(err error) int {
	var ce *CodedError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return -1
}

func TestLoadConfigMissing(t *testing.T) {
	noSystem(t)
	var out map[string]string
	err := LoadConfig("nope.yml", t.TempDir(), &out)
	if codeOf(err) != CodeFileNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	noSystem(t)
	dir := t.TempDir()
	writeFile(t, dir, "cfg.yml", "a: [unclosed\n")
	var out map[string]string
	err := LoadConfig("cfg.yml", dir, &out)
	if codeOf(err) != CodeConfig {
		t.Fatalf("got %v", err)
	}
}

func TestLoadConfigUserOnly(t *testing.T) {
	noSystem(t)
	dir := t.TempDir()
	writeFile(t, dir, "cfg.yml", "a: 1\nb: 2\n")
	var out map[string]int
	if err := LoadConfig("cfg.yml", dir, &out); err != nil {
		t.Fatal(err)
	}
	if out["a"] != 1 || out["b"] != 2 {
		t.Fatalf("got %v", out)
	}
}

func TestLoadConfigDeepMerge(t *testing.T) {
	prev := SystemDir
	t.Cleanup(func() { SystemDir = prev })
	SystemDir = t.TempDir()
	writeFile(t, SystemDir, "cfg.yml", "top:\n  keep: sys\n  over: sys\nonly: sys\n")
	dir := t.TempDir()
	writeFile(t, dir, "cfg.yml", "top:\n  over: user\n  add: user\n")
	var out struct {
		Top  map[string]string `yaml:"top"`
		Only string            `yaml:"only"`
	}
	if err := LoadConfig("cfg.yml", dir, &out); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"keep": "sys", "over": "user", "add": "user"}
	for k, v := range want {
		if out.Top[k] != v {
			t.Errorf("top.%s: got %q want %q", k, out.Top[k], v)
		}
	}
	if out.Only != "sys" {
		t.Errorf("only: got %q", out.Only)
	}
}

func TestLoadConfigNodeEmptyFiles(t *testing.T) {
	noSystem(t)
	dir := t.TempDir()
	writeFile(t, dir, "cfg.yml", "")
	node, err := LoadConfigNode("cfg.yml", dir)
	if err != nil || node != nil {
		t.Fatalf("got node=%v err=%v", node, err)
	}
}

//[<] 🤖🤖
