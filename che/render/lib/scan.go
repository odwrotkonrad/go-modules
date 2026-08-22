package lib

import "strings"

// [>] 🤖🤖🤖
type nodeKind int

const (
	nodeComment nodeKind = iota
	nodeRule
	nodeAssignment
	nodeInclude
)

type node struct {
	kind nodeKind
	text string
}

var assignOps = []string{":::=", "::=", ":=", "!=", "?=", "+=", "="}

func scan(src []byte) []node {
	var out []node
	lines := strings.Split(string(src), "\n")
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		if strings.HasPrefix(raw, "\t") {
			continue
		}
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			out = append(out, node{kind: nodeComment, text: line})
			continue
		}
		for continues(line) && i+1 < len(lines) {
			i++
			line = strings.TrimSuffix(line, "\\") + " " + strings.TrimSpace(lines[i])
			line = strings.TrimSpace(line)
		}
		if kind, ok := classify(line); ok {
			out = append(out, node{kind: kind, text: line})
		}
	}
	return out
}

func continues(line string) bool {
	trailing := len(line) - len(strings.TrimRight(line, "\\"))
	return trailing%2 == 1
}

func classify(line string) (nodeKind, bool) {
	body, exported := cutDirective(line)

	// [why] an include names the file whose targets belong in this doc, and it
	//   carries no colon of its own: classify it before the rule and assign tests
	if _, ok := cutIncludeDirective(body); ok {
		return nodeInclude, true
	}

	assignAt := findAssign(body)
	colonAt := findRuleColon(body)

	if assignAt >= 0 && (colonAt < 0 || assignAt <= colonAt) {
		if strings.TrimSpace(body[:assignAt]) == "" {
			return 0, false
		}
		return nodeAssignment, true
	}
	if colonAt >= 0 {
		return nodeRule, true
	}
	if exported && body != "" && !strings.ContainsAny(body, " \t") {
		return nodeAssignment, true
	}
	return 0, false
}

// cutIncludeDirective returns the paths an include line names, and whether the
// line was an include at all. A leading - marks the include optional to make.
func cutIncludeDirective(body string) ([]string, bool) {
	rest, ok := strings.CutPrefix(body, "include ")
	if !ok {
		if rest, ok = strings.CutPrefix(body, "-include "); !ok {
			rest, ok = strings.CutPrefix(body, "sinclude ")
		}
	}
	if !ok {
		return nil, false
	}
	paths := strings.Fields(rest)
	if len(paths) == 0 {
		return nil, false
	}
	return paths, true
}

func cutDirective(line string) (body string, exported bool) {
	body = line
	for _, d := range []string{"override ", "export ", "unexport "} {
		if rest, ok := strings.CutPrefix(body, d); ok {
			body = strings.TrimSpace(rest)
			exported = exported || d == "export "
		}
	}
	return body, exported
}

func findAssign(body string) int {
	best, bestOp := -1, ""
	for _, op := range assignOps {
		i := indexOutsideVar(body, op)
		if i < 0 {
			continue
		}
		if best < 0 || i < best || (i == best && len(op) > len(bestOp)) {
			best, bestOp = i, op
		}
	}
	return best
}

func findRuleColon(body string) int {
	i := indexOutsideVar(body, ":")
	if i < 0 {
		return -1
	}
	if strings.TrimSpace(body[:i]) == "" {
		return -1
	}
	return i
}

func indexOutsideVar(s, sub string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch {
		case (s[i] == '$') && i+1 < len(s) && (s[i+1] == '(' || s[i+1] == '{'):
			depth++
			i++
			continue
		case depth > 0 && (s[i] == ')' || s[i] == '}'):
			depth--
			continue
		}
		if depth == 0 && strings.HasPrefix(s[i:], sub) {
			return i
		}
	}
	return -1
}

//[<] 🤖🤖🤖
