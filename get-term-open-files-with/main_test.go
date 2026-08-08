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
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func languagesFixture(t *testing.T) string {
	t.Helper()
	return testyml.ReadFile(t, td, "testdata/fixture/common/languages.yml")
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
		out, err := run(c.Context.CommandArgs(), testyml.ConfigDir(t, configName, raw), url)
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

func TestHelpVersion(t *testing.T) {
	testyml.RunHelpVersionSpec(t, td, "testdata/spec/funcs/help_version.test.spec.yml", usage, "get-term-open-files-with", version)
}

//[<] 🤖🤖
