package lib

// [>] 🤖🤖

import "strings"

func RenderSuffixAliases(terminal string, rules TerminalRules, extsByType map[string][]string) string {
	var order []string
	aliases := map[string]string{}
	order = applyRules(order, aliases, rules["any"], extsByType)
	order = applyRules(order, aliases, rules[terminal], extsByType)
	lines := make([]string, 0, len(order))
	for _, ext := range order {
		lines = append(lines, ext+"="+aliases[ext])
	}
	return strings.Join(lines, "\n")
}

func applyRules(order []string, aliases map[string]string, rules []OpenerRule, extsByType map[string][]string) []string {
	for _, rule := range rules {
		if rule.Opener == "" {
			continue
		}
		for _, langType := range rule.Types {
			for _, ext := range extsByType[langType] {
				if _, seen := aliases[ext]; !seen {
					order = append(order, ext)
				}
				aliases[ext] = rule.Opener
			}
		}
	}
	return order
}

//[<] 🤖🤖
