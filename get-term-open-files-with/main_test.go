package main

// [>] 🤖🤖

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/konradodwrot/go-modules/get-term-open-files-with/lib"
	"gitlab.com/konradodwrot/go-modules/lib/climain"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

//go:embed all:testdata
var td embed.FS

func languagesFixture(t *testing.T) []byte {
	t.Helper()
	return []byte(testyml.ReadFile(t, td, "testdata/fixture/common/languages.yml"))
}

func seedCache(t *testing.T) {
	t.Helper()
	cache := t.TempDir()
	if err := os.WriteFile(filepath.Join(cache, "languages.yml"), languagesFixture(t), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LINGUIST_CACHE_DIR", cache)
}

func configDir(t *testing.T, raw []byte) string {
	t.Helper()
	yamlcfg.SystemDir = filepath.Join(t.TempDir(), "no-system")
	dir := t.TempDir()
	if raw != nil {
		if err := os.WriteFile(filepath.Join(dir, configName), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func checkCoded(t *testing.T, w testyml.Want, err error) bool {
	t.Helper()
	if !w.IsErrorWanted() {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return false
	}
	w.CheckErr(t, err)
	if got := yamlcfg.Code(err); w.ExitCode != 0 && got != w.ExitCode {
		t.Fatalf("Code = %d (%v), want %d", got, err, w.ExitCode)
	}
	return true
}

func TestRun(t *testing.T) {
	type in struct {
		Config string
		Args   []string
	}
	type c struct {
		Name string
		In   in
		Want testyml.Want
	}
	testyml.Run(t, td, "testdata/spec/run.spec.yml", func(t *testing.T, c c) {
		seedCache(t)
		var raw []byte
		if c.In.Config != "" {
			raw = []byte(testyml.ReadFile(t, td, c.In.Config))
		}
		out, err := run(c.In.Args, configDir(t, raw), lib.LanguagesURL)
		if checkCoded(t, c.Want, err) {
			return
		}
		testyml.EqualExpected(t, td, c.Want.FilesOut, out)
	})
}

func termFixture(t *testing.T) []byte {
	t.Helper()
	return []byte(testyml.ReadFile(t, td, "testdata/fixture/run/term.yml"))
}

func TestFetchWritesCache(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("LINGUIST_CACHE_DIR", cache)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(languagesFixture(t))
	}))
	defer srv.Close()
	dir := configDir(t, termFixture(t))
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
	dir := configDir(t, termFixture(t))
	_, err := run([]string{"any"}, dir, srv.URL)
	if yamlcfg.Code(err) != 14 {
		t.Fatalf("got %v", err)
	}
}

func TestHelp(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		out, done := climain.HelpVersion([]string{flag}, usage, "get-term-open-files-with", version)
		if !done {
			t.Fatalf("%s: not handled", flag)
		}
		if out != usage {
			t.Errorf("%s: usage mismatch", flag)
		}
	}
}

//[<] 🤖🤖
