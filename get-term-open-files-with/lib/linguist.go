package lib

// [>] 🤖🤖

import (
	"cmp"
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

func CacheDir() string {
	if d := os.Getenv("LINGUIST_CACHE_DIR"); d != "" {
		return d
	}
	xdg := cmp.Or(os.Getenv("XDG_CACHE_HOME"), filepath.Join(os.Getenv("HOME"), ".cache"))
	return filepath.Join(xdg, "get-term-open-files-with")
}

// netErr is the CodeNetwork error every fetch/cache failure maps to.
func netErr(url string) *yamlcfg.CodedError {
	return &yamlcfg.CodedError{Code: yamlcfg.CodeNetwork, Msg: "network fetch failed: " + url}
}

func fetchLanguages(url string) ([]byte, error) {
	cached := filepath.Join(CacheDir(), "languages.yml")
	if info, err := os.Stat(cached); err == nil && !info.IsDir() {
		return os.ReadFile(cached)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Get(url)
	if err != nil {
		return nil, netErr(url)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, netErr(url)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, netErr(url)
	}
	if err := os.MkdirAll(CacheDir(), 0o755); err != nil {
		return nil, netErr(url)
	}
	if err := os.WriteFile(cached, body, 0o644); err != nil {
		return nil, netErr(url)
	}
	return body, nil
}

type language struct {
	Type       string   `yaml:"type"`
	Extensions []string `yaml:"extensions"`
}

func TypeExtensions(url string) (map[string][]string, error) {
	data, err := fetchLanguages(url)
	if err != nil {
		return nil, err
	}
	var langs map[string]language
	if err := yaml.Unmarshal(data, &langs); err != nil {
		return nil, &yamlcfg.CodedError{Code: yamlcfg.CodeConfig, Msg: "invalid languages data: " + err.Error()}
	}
	sets := map[string]map[string]bool{}
	for _, lang := range langs {
		if lang.Type == "" {
			continue
		}
		set := sets[lang.Type]
		if set == nil {
			set = map[string]bool{}
			sets[lang.Type] = set
		}
		for _, ext := range lang.Extensions {
			set[normExt(ext)] = true
		}
	}
	out := map[string][]string{}
	for kind, set := range sets {
		out[kind] = slices.Sorted(maps.Keys(set))
	}
	return out, nil
}

func normExt(ext string) string {
	return strings.TrimLeft(strings.ToLower(ext), ".")
}

//[<] 🤖🤖
