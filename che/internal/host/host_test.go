package host

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/config"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func TestToDest(t *testing.T) {
	h := Host{Home: "/Users/x"}
	cases := map[string]string{
		"HOME":                                "/Users/x",
		"HOME/.config/zsh":                    "/Users/x/.config/zsh",
		"etc/zshrc":                           "/etc/zshrc",
		"/var/custom/cache/dir_size_exporter": "/var/custom/cache/dir_size_exporter",
		"/var/log/otelcol":                    "/var/log/otelcol",
	}
	for in, want := range cases {
		if got := h.ToDest(in); got != want {
			t.Errorf("ToDest(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrepend(t *testing.T) {
	// existing: value prepended with ':'
	env := prepend([]string{"PATH=foo", "OTHER=x"}, "PATH", "bar")
	if !slices.Contains(env, "PATH=bar:foo") {
		t.Errorf("prepend existing: got %v, want PATH=bar:foo", env)
	}
	// absent: appended bare
	env = prepend([]string{"OTHER=x"}, "PATH", "bar")
	if !slices.Contains(env, "PATH=bar") {
		t.Errorf("prepend absent: got %v, want PATH=bar", env)
	}
}

func TestSrc(t *testing.T) {
	h := New("/repo", "/Users/x", "cli/macos", config.Config{})
	if got := h.Src("etc/zshrc"); got != "/repo/root/etc/zshrc" {
		t.Errorf("Src = %q, want /repo/root/etc/zshrc", got)
	}
}

// TestResolveScripts: spec order kept, rels mapped to abs, missing errors.
func TestResolveScripts(t *testing.T) {
	dir := t.TempDir()
	scripts := []string{
		"ci/zsh/scripts/installs/90-kitty.zsh",
		"ci/zsh/scripts/installs/00-ci-deps.zsh",
	}
	for _, rel := range scripts {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	h := New(dir, "/Users/x", "cli/macos", config.Config{})

	got, err := h.ResolveScripts(scripts)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(dir, scripts[0]),
		filepath.Join(dir, scripts[1]),
	}
	if !slices.Equal(got, want) {
		t.Errorf("ResolveScripts = %v, want %v", got, want)
	}

	if _, err := h.ResolveScripts([]string{"ci/zsh/scripts/installs/99-absent.zsh"}); err == nil {
		t.Error("ResolveScripts must error on a missing script")
	}
}

func TestHostIsUnderHome(t *testing.T) {
	type in struct {
		Args []string
	}
	type c struct {
		Name string
		In   in
		Want bool
	}
	h := Host{Home: "/Users/x"}
	testyml.Run(t, td, "testdata/spec/unit/is_under_home.spec.yml", func(t *testing.T, c c) {
		if got := h.IsUnderHome(c.In.Args[0]); got != c.Want {
			t.Errorf("IsUnderHome(%q) = %v, want %v", c.In.Args[0], got, c.Want)
		}
	})
}

// [<] 🤖🤖
