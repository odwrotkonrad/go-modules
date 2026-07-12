package lib

// [>] 🤖🤖

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

func languagesServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("Go:\n  type: programming\n  extensions: [\".go\"]\n"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCacheDirEnvFallbacks(t *testing.T) {
	t.Setenv("LINGUIST_CACHE_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "/xc")
	assert.Equal(t, filepath.Join("/xc", "get-term-open-files-with"), CacheDir())
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "/h")
	assert.Equal(t, filepath.Join("/h", ".cache", "get-term-open-files-with"), CacheDir())
}

func TestFetchConnectionRefusedExit14(t *testing.T) {
	t.Setenv("LINGUIST_CACHE_DIR", t.TempDir())
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	_, err := fetchLanguages(url)
	assert.Equal(t, yamlcfg.CodeNetwork, yamlcfg.Code(err), "err: %v", err)
}

func TestFetchCacheMkdirFailureExit14(t *testing.T) {
	file := filepath.Join(t.TempDir(), "plain")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))
	t.Setenv("LINGUIST_CACHE_DIR", filepath.Join(file, "sub"))
	_, err := fetchLanguages(languagesServer(t).URL)
	assert.Equal(t, yamlcfg.CodeNetwork, yamlcfg.Code(err), "err: %v", err)
}

func TestFetchCacheWriteFailureExit14(t *testing.T) {
	cache := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(cache, "languages.yml"), 0o755))
	t.Setenv("LINGUIST_CACHE_DIR", cache)
	_, err := fetchLanguages(languagesServer(t).URL)
	assert.Equal(t, yamlcfg.CodeNetwork, yamlcfg.Code(err), "err: %v", err)
}

func TestTypeExtensionsInvalidYamlExit12(t *testing.T) {
	cache := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cache, "languages.yml"), []byte("a: [unclosed"), 0o644))
	t.Setenv("LINGUIST_CACHE_DIR", cache)
	_, err := TypeExtensions("http://unused.invalid")
	assert.Equal(t, yamlcfg.CodeConfig, yamlcfg.Code(err), "err: %v", err)
}

//[<] 🤖🤖
