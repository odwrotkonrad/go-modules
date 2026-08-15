package yamlcfg

// [>] 🤖🤖

import (
	"cmp"
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var SystemDir = "/etc/custom"

func LoadConfig(name, customDir string, out any) error {
	node, err := LoadConfigNode(name, customDir)
	if err != nil {
		return err
	}
	if node == nil {
		return nil
	}
	if err := node.Decode(out); err != nil {
		return newConfigError(resolveCustomPaths(name, customDir)[0], err)
	}
	return nil
}

func LoadConfigNode(name, customDir string) (*yaml.Node, error) {
	paths := resolveCustomPaths(name, customDir)
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
			return nil, newConfigError(p, err)
		}
		var node yaml.Node
		if err := yaml.Unmarshal(data, &node); err != nil {
			return nil, newConfigError(p, err)
		}
		if len(node.Content) == 0 {
			continue
		}
		merged = MergeNodes(merged, Unwrap(&node))
	}
	return merged, nil
}

func resolveCustomPaths(name, customDir string) []string {
	system := filepath.Join(SystemDir, name)
	if customDir != "" {
		return []string{system, filepath.Join(customDir, name)}
	}
	xdg := cmp.Or(os.Getenv("XDG_CONFIG_HOME"), filepath.Join(os.Getenv("HOME"), ".config"))
	return []string{system, filepath.Join(xdg, "custom", name)}
}

func Unwrap(n *yaml.Node) *yaml.Node {
	if n != nil && n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	return n
}

func MergeNodes(base, over *yaml.Node) *yaml.Node {
	if over == nil || over.Kind == 0 {
		return base
	}
	if base == nil || base.Kind != yaml.MappingNode || over.Kind != yaml.MappingNode {
		return over
	}
	for key, val := range MapPairs(over) {
		found := false
		for j := 0; j+1 < len(base.Content); j += 2 {
			if base.Content[j].Value == key.Value {
				base.Content[j+1] = MergeNodes(base.Content[j+1], val)
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

func MapPairs(n *yaml.Node) iter.Seq2[*yaml.Node, *yaml.Node] {
	return func(yield func(*yaml.Node, *yaml.Node) bool) {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if !yield(n.Content[i], n.Content[i+1]) {
				return
			}
		}
	}
}

const (
	CodeArgs         = 11
	CodeConfig       = 12
	CodeFileNotFound = 13
	CodeNetwork      = 14
)

func (e *CodedError) Error() string { return e.Msg }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ce, ok := errors.AsType[*CodedError](err); ok {
		return ce.Code
	}
	return 1
}

func ArgsError(args []string) *CodedError {
	return &CodedError{CodeArgs, "invalid arguments: " + fmt.Sprint(args)}
}

func newConfigError(path string, err error) *CodedError {
	return &CodedError{CodeConfig, "invalid config: " + path + ": " + err.Error()}
}

//[<] 🤖🤖
