// [>] 🤖🤖
package lib

import (
	"strings"

	"gopkg.in/yaml.v3"
)

func Render(cfg *yaml.Node) (string, error) {
	if cfg == nil {
		return "", nil
	}
	root := cfg
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return "", nil
		}
		root = root.Content[0]
	}
	if root.Kind == yaml.ScalarNode && root.Tag == "!!null" {
		return "", nil
	}
	if root.Kind != yaml.MappingNode {
		return "", &CodedError{CodeConfig, "invalid config: top level must be a mapping"}
	}
	var lines []string
	for i := 0; i+1 < len(root.Content); i += 2 {
		bundle := root.Content[i].Value
		roles := root.Content[i+1]
		if roles.Kind != yaml.MappingNode {
			return "", &CodedError{CodeConfig, "invalid config: " + bundle + " must be a mapping"}
		}
		for j := 0; j+1 < len(roles.Content); j += 2 {
			role := roles.Content[j].Value
			utis := roles.Content[j+1]
			if utis.Kind != yaml.SequenceNode {
				return "", &CodedError{CodeConfig, "invalid config: " + bundle + "." + role + " must be a list"}
			}
			for _, uti := range utis.Content {
				lines = append(lines, bundle+" "+uti.Value+" "+role)
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}

//[<] 🤖🤖
