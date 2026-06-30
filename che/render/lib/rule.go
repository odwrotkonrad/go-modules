package lib

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// [>] 🤖🤖🤖
func ruleTarget(node *sitter.Node, src []byte, what string) (target, bool) {
	var name string
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child.Kind() == "targets" {
			name = strings.TrimSpace(child.Utf8Text(src))
			break
		}
	}
	if name == "" || strings.HasPrefix(name, ".") {
		return target{}, false
	}
	var chain []string
	if normal := node.ChildByFieldName("normal"); normal != nil {
		chain = words(normal.Utf8Text(src))
	}
	return target{name: name, what: what, chain: chain}, true
}

// paramTarget builds a parameter entry from a variable_assignment node: name is
// the assigned word, what/vals from the preceding #[what]/#[vals] comments.
func paramTarget(node *sitter.Node, src []byte, what, vals string) (target, bool) {
	word := firstWord(node)
	if word == nil {
		return target{}, false
	}
	name := strings.TrimSpace(word.Utf8Text(src))
	if name == "" {
		return target{}, false
	}
	return target{name: name, what: what, vals: vals}, true
}

// firstWord returns the first descendant "word" node, depth-first. The name sits
// directly under variable_assignment but nests under a "list" in export_directive.
func firstWord(node *sitter.Node) *sitter.Node {
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child.Kind() == "word" {
			return child
		}
		if w := firstWord(child); w != nil {
			return w
		}
	}
	return nil
}

func words(s string) []string {
	return strings.Fields(strings.TrimSpace(s))
}

//[<] 🤖🤖🤖
