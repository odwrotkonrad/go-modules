package lib

// [>] 🤖🤖

import (
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

func RenderDutiLines(cfg *yaml.Node) (string, error) {
	if cfg == nil {
		return "", nil
	}
	root := yamlcfg.DocumentRoot(cfg)
	if root.Kind == yaml.DocumentNode && len(root.Content) == 0 {
		return "", nil
	}
	if root.Kind == yaml.ScalarNode && root.Tag == "!!null" {
		return "", nil
	}
	if root.Kind != yaml.MappingNode {
		return "", newConfigErr("top level must be a mapping")
	}
	var lines []string
	for bundleKey, roles := range yamlcfg.MapPairs(root) {
		bundle := bundleKey.Value
		if roles.Kind != yaml.MappingNode {
			return "", newConfigErr(bundle + " must be a mapping")
		}
		for roleKey, utis := range yamlcfg.MapPairs(roles) {
			role := roleKey.Value
			if utis.Kind != yaml.SequenceNode {
				return "", newConfigErr(bundle + "." + role + " must be a list")
			}
			for _, uti := range utis.Content {
				lines = append(lines, bundle+" "+uti.Value+" "+role)
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}

func newConfigErr(msg string) *yamlcfg.CodedError {
	return &yamlcfg.CodedError{Code: yamlcfg.CodeConfig, Msg: "invalid config: " + msg}
}

//[<] 🤖🤖
