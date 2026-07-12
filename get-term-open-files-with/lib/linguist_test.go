package lib

// [>] 🤖🤖

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
	if got := CacheDir(); got != filepath.Join("/xc", "get-term-open-files-with") {
		t.Errorf("xdg cache dir = %q", got)
	}
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "/h")
	if got := CacheDir(); got != filepath.Join("/h", ".cache", "get-term-open-files-with") {
		t.Errorf("home cache dir = %q", got)
	}
}

func TestFetchConnectionRefusedExit14(t *testing.T) {
	t.Setenv("LINGUIST_CACHE_DIR", t.TempDir())
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	_, err := fetchLanguages(url)
	if yamlcfg.Code(err) != yamlcfg.CodeNetwork {
		t.Fatalf("Code = %d (%v), want %d", yamlcfg.Code(err), err, yamlcfg.CodeNetwork)
	}
}

func TestFetchCacheMkdirFailureExit14(t *testing.T) {
	file := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LINGUIST_CACHE_DIR", filepath.Join(file, "sub"))
	_, err := fetchLanguages(languagesServer(t).URL)
	if yamlcfg.Code(err) != yamlcfg.CodeNetwork {
		t.Fatalf("Code = %d (%v), want %d", yamlcfg.Code(err), err, yamlcfg.CodeNetwork)
	}
}

func TestFetchCacheWriteFailureExit14(t *testing.T) {
	cache := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cache, "languages.yml"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LINGUIST_CACHE_DIR", cache)
	_, err := fetchLanguages(languagesServer(t).URL)
	if yamlcfg.Code(err) != yamlcfg.CodeNetwork {
		t.Fatalf("Code = %d (%v), want %d", yamlcfg.Code(err), err, yamlcfg.CodeNetwork)
	}
}

func TestTypeExtensionsInvalidYamlExit12(t *testing.T) {
	cache := t.TempDir()
	if err := os.WriteFile(filepath.Join(cache, "languages.yml"), []byte("a: [unclosed"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LINGUIST_CACHE_DIR", cache)
	_, err := TypeExtensions("http://unused.invalid")
	if yamlcfg.Code(err) != yamlcfg.CodeConfig {
		t.Fatalf("Code = %d (%v), want %d", yamlcfg.Code(err), err, yamlcfg.CodeConfig)
	}
}

//[<] 🤖🤖
