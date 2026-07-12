package main

// [>] 🤖🤖

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/get-term-open-files-with/lib"
	"gitlab.com/konradodwrot/go-modules/lib/climain"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

//go:embed all:testdata
var td embed.FS

func languagesFixture(t *testing.T) string {
	t.Helper()
	return testyml.ReadFile(t, td, "testdata/fixture/common/languages.yml")
}

func seedCache(t *testing.T) {
	t.Helper()
	cache := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cache, "languages.yml"), []byte(languagesFixture(t)), 0o644))
	t.Setenv("LINGUIST_CACHE_DIR", cache)
}

// configDir writes raw as the user config (system dir absent), returns the dir.
func configDir(t *testing.T, raw string) string {
	t.Helper()
	testyml.Swap(t, &yamlcfg.SystemDir, filepath.Join(t.TempDir(), "no-system"))
	dir := t.TempDir()
	if raw != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, configName), []byte(raw), 0o644))
	}
	return dir
}

func TestRun(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/get-term-open-files-with.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		seedCache(t)
		raw := ""
		if cfg := c.Input.Args.String(t, 0); cfg != "" {
			raw = testyml.ReadFile(t, td, cfg)
		}
		out, err := run(c.Context.CommandArgs(), configDir(t, raw), lib.LanguagesURL)
		if c.Expected.Check(t, err) {
			return
		}
		testyml.EqualExpected(t, td, c.Expected.Files, out)
	})
}

func termFixture(t *testing.T) string {
	t.Helper()
	return testyml.ReadFile(t, td, "testdata/fixture/run/term.yml")
}

func TestFetchWritesCache(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("LINGUIST_CACHE_DIR", cache)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(languagesFixture(t)))
	}))
	defer srv.Close()
	dir := configDir(t, termFixture(t))
	out, err := run([]string{"any"}, dir, srv.URL)
	require.NoError(t, err)
	assert.NotEmpty(t, out)
	_, err = os.Stat(filepath.Join(cache, "languages.yml"))
	assert.NoError(t, err, "cache not written")
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
	assert.Equal(t, yamlcfg.CodeNetwork, yamlcfg.Code(err), "err: %v", err)
}

func TestHelp(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		out, done := climain.HelpVersion([]string{flag}, usage, "get-term-open-files-with", version)
		require.Truef(t, done, "%s: not handled", flag)
		assert.Equal(t, usage, out, flag)
	}
}

//[<] 🤖🤖
