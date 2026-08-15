package lib

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// [>] 🤖🤖🤖
func parseRuleTarget(node *sitter.Node, src []byte, what string) (target, bool) {
	var name string
	for i := range node.NamedChildCount() {
		child := node.NamedChild(i)
		if child.Kind() == "targets" {
			name = strings.TrimSpace(child.Utf8Text(src))
			break
		}
	}
	if name == "" || strings.HasPrefix(name, ".") {
		return target{}, false
	}
	var deps []string
	if normal := node.ChildByFieldName("normal"); normal != nil {
		deps = strings.Fields(normal.Utf8Text(src))
	}
	return target{name: name, what: what, deps: deps}, true
}

func parseParamTarget(node *sitter.Node, src []byte, what, vals string) (target, bool) {
	word := findWord(node)
	if word == nil {
		return target{}, false
	}
	name := strings.TrimSpace(word.Utf8Text(src))
	if name == "" {
		return target{}, false
	}
	return target{name: name, what: what, vals: vals}, true
}

func findWord(node *sitter.Node) *sitter.Node {
	for i := range node.NamedChildCount() {
		child := node.NamedChild(i)
		if child.Kind() == "word" {
			return child
		}
		if w := findWord(child); w != nil {
			return w
		}
	}
	return nil
}

//[<] 🤖🤖🤖
