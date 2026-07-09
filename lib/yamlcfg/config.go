// [>] 🤖🤖
package yamlcfg

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	CodeArgs         = 11
	CodeConfig       = 12
	CodeFileNotFound = 13
	CodeNetwork      = 14
)

type CodedError struct {
	Code int
	Msg  string
}

func (e *CodedError) Error() string { return e.Msg }

var SystemDir = "/etc/custom"

func customPaths(name, customDir string) []string {
	system := filepath.Join(SystemDir, name)
	if customDir != "" {
		return []string{system, filepath.Join(customDir, name)}
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return []string{system, filepath.Join(xdg, "custom", name)}
}

func unwrap(n *yaml.Node) *yaml.Node {
	if n != nil && n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	return n
}

func mergeNodes(base, over *yaml.Node) *yaml.Node {
	if base == nil {
		return over
	}
	if base.Kind != yaml.MappingNode || over.Kind != yaml.MappingNode {
		return over
	}
	for i := 0; i+1 < len(over.Content); i += 2 {
		key := over.Content[i]
		val := over.Content[i+1]
		found := false
		for j := 0; j+1 < len(base.Content); j += 2 {
			if base.Content[j].Value == key.Value {
				base.Content[j+1] = mergeNodes(base.Content[j+1], val)
				found = true
				break
			}
		}
		if !found {
			base.Content = append(base.Content, key, val)
		}
	}
	return base
}

func LoadConfigNode(name, customDir string) (*yaml.Node, error) {
	paths := customPaths(name, customDir)
	var existing []string
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			existing = append(existing, p)
		}
	}
	if len(existing) == 0 {
		return nil, &CodedError{CodeFileNotFound, "file not found: " + paths[0]}
	}
	var merged *yaml.Node
	for _, p := range existing {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, &CodedError{CodeConfig, "invalid config: " + p + ": " + err.Error()}
		}
		var node yaml.Node
		if err := yaml.Unmarshal(data, &node); err != nil {
			return nil, &CodedError{CodeConfig, "invalid config: " + p + ": " + err.Error()}
		}
		if len(node.Content) == 0 {
			continue
		}
		merged = mergeNodes(merged, unwrap(&node))
	}
	return merged, nil
}

func LoadConfig(name, customDir string, out any) error {
	node, err := LoadConfigNode(name, customDir)
	if err != nil {
		return err
	}
	if node == nil {
		return nil
	}
	paths := customPaths(name, customDir)
	if err := node.Decode(out); err != nil {
		return &CodedError{CodeConfig, "invalid config: " + paths[0] + ": " + err.Error()}
	}
	return nil
}

//[<] 🤖🤖
