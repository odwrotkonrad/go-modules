// [>] 🤖🤖
package lib

import "strings"

type OpenerRule struct {
	Opener string   `yaml:"opener"`
	Types  []string `yaml:"types"`
}

type Sections map[string][]OpenerRule

// appendAliases folds one section's rules into aliases (last rule wins),
// returning order extended with each first-seen extension.
func appendAliases(order []string, aliases map[string]string, rules []OpenerRule, byType map[string][]string) []string {
	for _, rule := range rules {
		if rule.Opener == "" {
			continue
		}
		for _, kind := range rule.Types {
			for _, ext := range byType[kind] {
				if _, seen := aliases[ext]; !seen {
					order = append(order, ext)
				}
				aliases[ext] = rule.Opener
			}
		}
	}
	return order
}

func Render(terminal string, sections Sections, byType map[string][]string) string {
	var order []string
	aliases := map[string]string{}
	order = appendAliases(order, aliases, sections["any"], byType)
	order = appendAliases(order, aliases, sections[terminal], byType)
	lines := make([]string, 0, len(order))
	for _, ext := range order {
		lines = append(lines, ext+"="+aliases[ext])
	}
	return strings.Join(lines, "\n")
}

//[<] 🤖🤖
