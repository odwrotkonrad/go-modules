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

func serveStatus(t *testing.T, status int, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

type runWant struct {
	CacheWritten bool `yaml:"cacheWritten"`
}

func TestRun(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/cmds/get-term-open-files-with.test.spec.yml", func(t *testing.T, c testyml.Case[runWant]) {
		cache := t.TempDir()
		t.Setenv("LINGUIST_CACHE_DIR", cache)
		url := lib.LanguagesURL
		switch c.Input.Args.String(t, 1) {
		case "languages":
			url = serveStatus(t, http.StatusOK, languagesFixture(t))
		case "error500":
			url = serveStatus(t, http.StatusInternalServerError, "")
		default:
			require.NoError(t, os.WriteFile(filepath.Join(cache, "languages.yml"), []byte(languagesFixture(t)), 0o644))
		}
		raw := ""
		if cfg := c.Input.Args.String(t, 0); cfg != "" {
			raw = testyml.ReadFile(t, td, cfg)
		}
		out, err := run(c.Context.CommandArgs(), configDir(t, raw), url)
		if c.Expected.Check(t, err) {
			return
		}
		testyml.EqualExpected(t, td, c.Expected.Files, out)
		if c.Expected.Output.CacheWritten {
			_, statErr := os.Stat(filepath.Join(cache, "languages.yml"))
			assert.NoError(t, statErr, "cache not written")
		}
	})
}

type helpVersionWant struct {
	Usage bool `yaml:"usage"`
	Done  bool `yaml:"done"`
}

func TestHelpVersion(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/help_version.test.spec.yml", func(t *testing.T, c testyml.Case[helpVersionWant]) (helpVersionWant, error) {
		out, done := climain.HelpVersion(c.Input.Args.Strings(t, 0), usage, "get-term-open-files-with", version)
		return helpVersionWant{Usage: out == usage, Done: done}, nil
	})
}

//[<] 🤖🤖
