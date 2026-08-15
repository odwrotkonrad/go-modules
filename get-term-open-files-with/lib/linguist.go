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
	extSets := map[string]map[string]bool{}
	for _, lang := range langs {
		if lang.Type == "" {
			continue
		}
		set := extSets[lang.Type]
		if set == nil {
			set = map[string]bool{}
			extSets[lang.Type] = set
		}
		for _, ext := range lang.Extensions {
			set[normExt(ext)] = true
		}
	}
	extsByType := map[string][]string{}
	for kind, set := range extSets {
		extsByType[kind] = slices.Sorted(maps.Keys(set))
	}
	return extsByType, nil
}

func fetchLanguages(url string) ([]byte, error) {
	cachePath := filepath.Join(ResolveCacheDir(), "languages.yml")
	if info, err := os.Stat(cachePath); err == nil && !info.IsDir() {
		return os.ReadFile(cachePath)
	}
	body, err := download(url, cachePath)
	if err != nil {
		return nil, newNetworkErr(url)
	}
	return body, nil
}

func download(url, cachePath string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return nil, err
	}
	return body, os.WriteFile(cachePath, body, 0o644)
}

func ResolveCacheDir() string {
	if d := os.Getenv("LINGUIST_CACHE_DIR"); d != "" {
		return d
	}
	xdg := cmp.Or(os.Getenv("XDG_CACHE_HOME"), filepath.Join(os.Getenv("HOME"), ".cache"))
	return filepath.Join(xdg, "get-term-open-files-with")
}

func normExt(ext string) string {
	return strings.TrimLeft(strings.ToLower(ext), ".")
}

func newNetworkErr(url string) *yamlcfg.CodedError {
	return &yamlcfg.CodedError{Code: yamlcfg.CodeNetwork, Msg: "network fetch failed: " + url}
}

//[<] 🤖🤖
