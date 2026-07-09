package spec

// [>] 🤖🤖

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// PluginRef is one parsed plugins entry: a profile defined in another repo,
// loaded and anchored at its own checkout, optionally with envs exported
// around everything done for its unit. IsPath marks a local dir ref (no `@`
// prefix): URL then holds the dir path (absolute, relative, ~/, $VAR).
type PluginRef struct {
	URL     string
	Profile string
	Env     map[string]string
	IsPath  bool
}

// String renders the canonical entry form (env not rendered):
// `@<giturl>::<profile>` remote, `<dir>::<profile>` local.
func (p PluginRef) String() string {
	if p.IsPath {
		return p.URL + "::" + p.Profile
	}
	return "@" + p.URL + "::" + p.Profile
}

// pluginEntry is one plugins list item: a bare `@<giturl>::<profile>` /
// `<dir>::<profile>` string, or a {ref, env} object.
type pluginEntry struct {
	Ref string            `yaml:"ref"`
	Env map[string]string `yaml:"env"`
}

func (p *pluginEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		p.Ref = value.Value
		return nil
	}
	type alias pluginEntry
	return value.Decode((*alias)(p))
}

// parsePluginRef parses one plugins ref, the `@` prefix deciding the kind:
// `@<giturl>::<profile>` remote, `<dir>::<profile>` local dir (absolute
// /path, relative ./path or path, ~/, $VAR). Last `::` splits, both parts
// required.
func parsePluginRef(entry string) (PluginRef, error) {
	raw, isURL := strings.CutPrefix(entry, "@")
	i := strings.LastIndex(raw, "::")
	if i <= 0 || i+2 >= len(raw) {
		return PluginRef{}, fmt.Errorf("plugins entry %q: want @<giturl>::<profile> or <dir>::<profile>", entry)
	}
	return PluginRef{URL: raw[:i], Profile: raw[i+2:], IsPath: !isURL}, nil
}

// [<] 🤖🤖
