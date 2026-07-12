package lib

// [>] 🤖🤖

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func languagesServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("Go:\n  type: programming\n  extensions: [\".go\"]\n"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCacheDir(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/cache_dir.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		return CacheDir(), nil
	})
}

// failingFetch materializes the named failure precondition, returns the fetch URL.
func failingFetch(t *testing.T, precondition string) string {
	t.Helper()
	switch precondition {
	case "connectionRefused":
		t.Setenv("LINGUIST_CACHE_DIR", t.TempDir())
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close()
		return url
	case "mkdirBlockedByFile":
		file := filepath.Join(t.TempDir(), "plain")
		require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))
		t.Setenv("LINGUIST_CACHE_DIR", filepath.Join(file, "sub"))
	case "writeBlockedByDir":
		cache := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(cache, "languages.yml"), 0o755))
		t.Setenv("LINGUIST_CACHE_DIR", cache)
	}
	return languagesServer(t).URL
}

func TestFetchLanguages(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/fetch_languages.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		url := failingFetch(t, c.Input.Args.String(t, 0))
		_, err := fetchLanguages(url)
		c.Expected.Check(t, err)
	})
}

func TestTypeExtensions(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/type_extensions.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		cache := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(cache, "languages.yml"), []byte(c.Input.Args.String(t, 0)), 0o644))
		t.Setenv("LINGUIST_CACHE_DIR", cache)
		_, err := TypeExtensions("http://unused.invalid")
		c.Expected.Check(t, err)
	})
}

//[<] 🤖🤖
