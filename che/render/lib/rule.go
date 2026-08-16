package lib

import (
	"strings"
)

// [>] 🤖🤖🤖
func parseRuleTarget(line, what string) (target, bool) {
	colon := findRuleColon(line)
	if colon < 0 {
		return target{}, false
	}
	name := strings.TrimSpace(line[:colon])
	if name == "" || strings.HasPrefix(name, ".") {
		return target{}, false
	}
	rest := strings.TrimPrefix(line[colon:], ":")
	rest = strings.TrimPrefix(rest, ":")
	if i := strings.Index(rest, ";"); i >= 0 {
		rest = rest[:i]
	}
	if i := indexOutsideVar(rest, "|"); i >= 0 {
		rest = rest[:i]
	}
	return target{name: name, what: what, deps: strings.Fields(rest)}, true
}

func parseParamTarget(line, what, vals string) (target, bool) {
	body, _ := cutDirective(line)

	name := body
	if assignAt := findAssign(body); assignAt >= 0 {
		name = body[:assignAt]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return target{}, false
	}
	return target{name: name, what: what, vals: vals}, true
}

//[<] 🤖🤖🤖
