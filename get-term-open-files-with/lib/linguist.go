package lib

// [>] 🤖🤖

import (
	"cmp"
	"errors"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

const LanguagesURL = "https://raw.githubusercontent.com/github-linguist/linguist/master/lib/linguist/languages.yml"

func LoadTypeExtensions(url string) (map[string][]string, error) {
	data, err := fetchLanguages(url)
	if err != nil {
		return nil, err
	}
	var langs map[string]language
	if err := yaml.Unmarshal(data, &langs); err != nil {
		return nil, &yamlcfg.CodedError{Code: yamlcfg.CodeConfig, Msg: "invalid languages data: " + err.Error()}
	}
	uniqueExts := map[string]map[string]bool{}
	for _, lang := range langs {
		if lang.Type == "" {
			continue
		}
		exts := uniqueExts[lang.Type]
		if exts == nil {
			exts = map[string]bool{}
			uniqueExts[lang.Type] = exts
		}
		for _, ext := range lang.Extensions {
			exts[normalizeExt(ext)] = true
		}
	}
	extsByType := map[string][]string{}
	for langType, exts := range uniqueExts {
		extsByType[langType] = slices.Sorted(maps.Keys(exts))
	}
	return extsByType, nil
}

func fetchLanguages(url string) ([]byte, error) {
	cachePath := filepath.Join(ResolveCacheDir(), "languages.yml")
	if info, err := os.Stat(cachePath); err == nil && !info.IsDir() {
		return os.ReadFile(cachePath)
	}
	body, err := download(url)
	if err == nil {
		err = writeCache(cachePath, body)
	}
	if err != nil {
		return nil, newNetworkErr(url)
	}
	return body, nil
}

func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}
	return io.ReadAll(res.Body)
}

func writeCache(cachePath string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(cachePath, body, 0o644)
}

func ResolveCacheDir() string {
	if dir := os.Getenv("LINGUIST_CACHE_DIR"); dir != "" {
		return dir
	}
	xdg := cmp.Or(os.Getenv("XDG_CACHE_HOME"), filepath.Join(os.Getenv("HOME"), ".cache"))
	return filepath.Join(xdg, "get-term-open-files-with")
}

func normalizeExt(ext string) string {
	return strings.TrimLeft(strings.ToLower(ext), ".")
}

func newNetworkErr(url string) *yamlcfg.CodedError {
	return &yamlcfg.CodedError{Code: yamlcfg.CodeNetwork, Msg: "network fetch failed: " + url}
}

//[<] 🤖🤖
