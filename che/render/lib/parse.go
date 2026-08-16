package lib

import (
	"fmt"
	"os"
	"strings"
)

// [>] 🤖🤖🤖
const includeTag = "[genai-include]"

func Generate(path string) (string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", path)
	}
	return render(parse(src)), nil
}

func parse(src []byte) []section {
	var out []section
	var stack []frame

	emit := func(f frame) {
		if f.kept && len(f.targets) > 0 {
			out = append(out, section{heading: f.heading, level: f.level, targets: f.targets})
		}
	}

	cur := func() *frame {
		if len(stack) == 0 {
			return nil
		}
		return &stack[len(stack)-1]
	}

	var pending pendingComment

	for _, node := range scan(src) {
		text := node.text
		prev := pending
		pending = pendingComment{}

		switch node.kind {
		case nodeComment:
			if label, depth, ok := parseSectionOpen(text); ok {
				if c := cur(); c != nil {
					emit(*c)
					c.targets = nil
				}
				kept := strings.Contains(text, includeTag) || (cur() != nil && cur().kept)
				stack = append(stack, frame{
					heading: label,
					depth:   depth,
					level:   3 + depth,
					kept:    kept,
				})
			} else if depth, ok := parseSectionClose(text); ok {
				for len(stack) > 0 && stack[len(stack)-1].depth >= depth {
					emit(stack[len(stack)-1])
					stack = stack[:len(stack)-1]
				}
			} else if what, ok := parseTagComment(text, "what"); ok {
				pending = pendingComment{what: what, vals: prev.vals}
			} else if vals, ok := parseTagComment(text, "vals"); ok {
				pending = pendingComment{what: prev.what, vals: vals}
			}
		case nodeRule:
			if c := cur(); c != nil && c.kept {
				if t, ok := parseRuleTarget(text, prev.what); ok {
					c.targets = append(c.targets, t)
				}
			}
		case nodeAssignment:
			if c := cur(); c != nil && c.kept && prev.what != "" {
				if t, ok := parseParamTarget(text, prev.what, prev.vals); ok {
					c.targets = append(c.targets, t)
				}
			}
		}
	}
	for len(stack) > 0 {
		emit(stack[len(stack)-1])
		stack = stack[:len(stack)-1]
	}
	return out
}

//[<] 🤖🤖🤖
