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

var SystemConfigDir = "/etc/custom"

func LoadConfig(name, userDir string, out any) error {
	root, err := LoadConfigNode(name, userDir)
	if err != nil {
		return err
	}
	if root == nil {
		return nil
	}
	if err := root.Decode(out); err != nil {
		return newConfigError(ResolveConfigPaths(name, userDir)[0], err)
	}
	return nil
}

func LoadConfigNode(name, userDir string) (*yaml.Node, error) {
	paths := ResolveConfigPaths(name, userDir)
	var found []string
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			found = append(found, path)
		}
	}
	if len(found) == 0 {
		return nil, &CodedError{CodeFileNotFound, "file not found: " + paths[0]}
	}
	var merged *yaml.Node
	for _, path := range found {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, newConfigError(path, err)
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, newConfigError(path, err)
		}
		if len(doc.Content) == 0 {
			continue
		}
		merged = MergeNodes(merged, DocumentRoot(&doc))
	}
	return merged, nil
}

func ResolveConfigPaths(name, userDir string) []string {
	system := filepath.Join(SystemConfigDir, name)
	if userDir != "" {
		return []string{system, filepath.Join(userDir, name)}
	}
	xdg := cmp.Or(os.Getenv("XDG_CONFIG_HOME"), filepath.Join(os.Getenv("HOME"), ".config"))
	return []string{system, filepath.Join(xdg, "custom", name)}
}

func DocumentRoot(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
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
		for i := 0; i+1 < len(base.Content); i += 2 {
			if base.Content[i].Value == key.Value {
				base.Content[i+1] = MergeNodes(base.Content[i+1], val)
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

func MapPairs(node *yaml.Node) iter.Seq2[*yaml.Node, *yaml.Node] {
	return func(yield func(*yaml.Node, *yaml.Node) bool) {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if !yield(node.Content[i], node.Content[i+1]) {
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
	if coded, ok := errors.AsType[*CodedError](err); ok {
		return coded.Code
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
