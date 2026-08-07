package options

// [>] 🤖🤖

import (
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var yamlSkippedKeys = []string{"cheWorkingDirectory", "skipRunIf", "errexit", "packages.override"}

func DefaultSettings() ([]Setting, error) {
	o := &Options{}
	if err := o.Resolve(func(string) string { return "" }, Layer{}, Layer{}); err != nil {
		return nil, err
	}
	return o.Settings, nil
}

func SettingsYAML(settings []Setting) (string, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, s := range settings {
		if slices.Contains(yamlSkippedKeys, s.Key) {
			continue
		}
		insertYAML(root, strings.Split(s.Key, "."), settingNode(s))
	}
	b, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func settingNode(s Setting) *yaml.Node {
	switch s.Kind {
	case "bool":
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: s.Value}
	case "list":
		seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		value := strings.TrimSuffix(strings.TrimPrefix(s.Value, "["), "]")
		if value != "" {
			for _, v := range strings.Split(value, ",") {
				seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: strings.TrimSpace(v)})
			}
		}
		return seq
	default:
		v := s.Value
		if s.Key == "dryRun" && v == "false" {
			v = ""
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Value: v}
	}
}

func insertYAML(m *yaml.Node, path []string, value *yaml.Node) {
	key := path[0]
	var target *yaml.Node
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			target = m.Content[i+1]
			break
		}
	}
	if len(path) == 1 {
		if target == nil {
			m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, value)
		}
		return
	}
	if target == nil {
		target = &yaml.Node{Kind: yaml.MappingNode}
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, target)
	}
	insertYAML(target, path[1:], value)
}

// [<] 🤖🤖
