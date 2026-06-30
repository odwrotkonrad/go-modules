// [>] 🤖🤖
package lib

import "strings"

type OpenerRule struct {
	Opener string   `yaml:"opener"`
	Types  []string `yaml:"types"`
}

type Sections map[string][]OpenerRule

func sectionAliases(rules []OpenerRule, byType map[string][]string, order *[]string, aliases map[string]string) {
	for _, rule := range rules {
		if rule.Opener == "" {
			continue
		}
		for _, kind := range rule.Types {
			for _, ext := range byType[kind] {
				if _, seen := aliases[ext]; !seen {
					*order = append(*order, ext)
				}
				aliases[ext] = rule.Opener
			}
		}
	}
}

func Render(terminal string, sections Sections, byType map[string][]string) string {
	order := []string{}
	aliases := map[string]string{}
	sectionAliases(sections["any"], byType, &order, aliases)
	sectionAliases(sections[terminal], byType, &order, aliases)
	lines := make([]string, 0, len(order))
	for _, ext := range order {
		lines = append(lines, ext+"="+aliases[ext])
	}
	return strings.Join(lines, "\n")
}

//[<] 🤖🤖
