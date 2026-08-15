package lib

import (
	"fmt"
	"os"
	"strings"

	tsmake "github.com/tree-sitter-grammars/tree-sitter-make/bindings/go"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// [>] 🤖🤖🤖
const includeTag = "[genai-include]"

func Generate(path string) (string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", path)
	}
	sections, err := parse(src)
	if err != nil {
		return "", err
	}
	return render(sections), nil
}

func parse(src []byte) ([]section, error) {
	parser := sitter.NewParser()
	if err := parser.SetLanguage(sitter.NewLanguage(tsmake.Language())); err != nil {
		return nil, err
	}
	tree := parser.Parse(src, nil)
	root := tree.RootNode()

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

	for i := range root.NamedChildCount() {
		node := root.NamedChild(i)
		text := strings.TrimSpace(node.Utf8Text(src))
		prev := pending
		pending = pendingComment{}

		switch node.Kind() {
		case "comment":
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
		case "rule":
			if c := cur(); c != nil && c.kept {
				if t, ok := parseRuleTarget(node, src, prev.what); ok {
					c.targets = append(c.targets, t)
				}
			}
		case "variable_assignment", "export_directive":
			if c := cur(); c != nil && c.kept && prev.what != "" {
				if t, ok := parseParamTarget(node, src, prev.what, prev.vals); ok {
					c.targets = append(c.targets, t)
				}
			}
		}
	}
	for len(stack) > 0 {
		emit(stack[len(stack)-1])
		stack = stack[:len(stack)-1]
	}
	return out, nil
}

//[<] 🤖🤖🤖
