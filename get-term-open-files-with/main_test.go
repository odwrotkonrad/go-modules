// [>] 🤖🤖
package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/konradodwrot/go/cruft/get-term-open-files-with/lib"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func seedCache(t *testing.T) {
	t.Helper()
	cache := t.TempDir()
	if err := os.WriteFile(filepath.Join(cache, "languages.yml"), readTestdata(t, "languages.yml"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LINGUIST_CACHE_DIR", cache)
}

func configDir(t *testing.T, raw []byte) string {
	t.Helper()
	lib.SystemDir = filepath.Join(t.TempDir(), "no-system")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, configName), raw, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func codeOf(err error) int {
	var ce *lib.CodedError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return -1
}

func TestPositive(t *testing.T) {
	term := readTestdata(t, "term.yml")
	cases := []struct {
		name string
		cfg  []byte
		arg  string
		want string
	}{
		{
			"any_uses_base_opener", term, "any",
			"go=vim\npy=vim\nrb=vim\ncss=vim\nhtml=vim\njson=vim\nyaml=vim\nyml=vim\nrst=vim\ntxt=vim",
		},
		{
			"vscode_overrides_any_keeps_prose", term, "vscode",
			"go=code -r\npy=code -r\nrb=code -r\ncss=code -r\nhtml=code -r\njson=code -r\nyaml=code -r\nyml=code -r\nrst=vim\ntxt=vim",
		},
		{
			"kitty_partial_override", term, "kitty",
			"go=vim\npy=vim\nrb=vim\ncss=vim\nhtml=vim\njson=bat\nyaml=bat\nyml=bat\nrst=nvim\ntxt=nvim",
		},
		{
			"last_opener_wins",
			[]byte("vscode:\n  - opener: code -r\n    types: [data]\n  - opener: code -w\n    types: [data]\n"),
			"vscode",
			"json=code -w\nyaml=code -w\nyml=code -w",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seedCache(t)
			dir := configDir(t, c.cfg)
			out, err := run([]string{c.arg}, dir, lib.LanguagesURL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != c.want {
				t.Errorf("got:\n%q\nwant:\n%q", out, c.want)
			}
		})
	}
}

func TestArgErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no_args", []string{}},
		{"two_args", []string{"any", "vscode"}},
		{"three_args", []string{"any", "vscode", "kitty"}},
		{"unknown_terminal", []string{"alacritty"}},
		{"case_sensitive", []string{"Any"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seedCache(t)
			dir := configDir(t, readTestdata(t, "term.yml"))
			_, err := run(c.args, dir, lib.LanguagesURL)
			if codeOf(err) != 11 {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestConfigErrors(t *testing.T) {
	t.Run("missing_config", func(t *testing.T) {
		seedCache(t)
		lib.SystemDir = filepath.Join(t.TempDir(), "no-system")
		dir := t.TempDir()
		_, err := run([]string{"any"}, dir, lib.LanguagesURL)
		if codeOf(err) != 13 {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("invalid_config", func(t *testing.T) {
		seedCache(t)
		dir := configDir(t, []byte("any: [unclosed\n"))
		_, err := run([]string{"any"}, dir, lib.LanguagesURL)
		if codeOf(err) != 12 {
			t.Fatalf("got %v", err)
		}
	})
}

func TestFetchWritesCache(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("LINGUIST_CACHE_DIR", cache)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(readTestdata(t, "languages.yml"))
	}))
	defer srv.Close()
	dir := configDir(t, readTestdata(t, "term.yml"))
	out, err := run([]string{"any"}, dir, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatal("empty output")
	}
	if _, err := os.Stat(filepath.Join(cache, "languages.yml")); err != nil {
		t.Fatalf("cache not written: %v", err)
	}
}

func TestFetchFailureExit14(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("LINGUIST_CACHE_DIR", cache)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	dir := configDir(t, readTestdata(t, "term.yml"))
	_, err := run([]string{"any"}, dir, srv.URL)
	if codeOf(err) != 14 {
		t.Fatalf("got %v", err)
	}
}

func TestHelp(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		out, err := run([]string{flag}, "", lib.LanguagesURL)
		if err != nil {
			t.Fatalf("%s: %v", flag, err)
		}
		if out != usage {
			t.Errorf("%s: usage mismatch", flag)
		}
	}
}

//[<] 🤖🤖
