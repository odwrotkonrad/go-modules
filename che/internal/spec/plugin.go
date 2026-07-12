package spec

// [>] 🤖🤖

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// String renders the canonical entry form (env not rendered):
// `@<giturl>::<profile>` remote, `<dir>::<profile>` local.
func (p PluginRef) String() string {
	if p.IsPath {
		return p.URL + "::" + p.Profile
	}
	return "@" + p.URL + "::" + p.Profile
}

func (p *pluginEntry) UnmarshalYAML(value *yaml.Node) error {
	type alias pluginEntry
	return decodeScalarOr(value, &p.Ref, (*alias)(p))
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
