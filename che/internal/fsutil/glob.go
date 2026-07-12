package fsutil

// [>] 🤖🤖

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// IsGlobMatch matches path against pattern. ** spans separators, * one segment, prefix may end in * (suffix glob).
func IsGlobMatch(pattern, path string) bool {
	if base, ok := strings.CutSuffix(pattern, "/**"); ok {
		return IsUnder(path, base)
	}
	ok, err := doublestar.Match(pattern, path)
	return err == nil && ok
}

// ExpandBraces expands {a,b,c} groups into the cartesian product (zsh-style), e.g. "x/{a,b}/y" -> ["x/a/y","x/b/y"].
// No braces returns the pattern unchanged.
func ExpandBraces(pattern string) []string {
	open := strings.IndexByte(pattern, '{')
	if open < 0 {
		return []string{pattern}
	}
	depth, closeAt := 0, -1
	for i := open; i < len(pattern) && closeAt < 0; i++ {
		switch pattern[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				closeAt = i
			}
		}
	}
	if closeAt < 0 {
		return []string{pattern} // unbalanced, leave as-is
	}
	pre, body, post := pattern[:open], pattern[open+1:closeAt], pattern[closeAt+1:]
	var out []string
	for _, alt := range splitTopLevel(body) {
		out = append(out, ExpandBraces(pre+alt+post)...)
	}
	return out
}

// splitTopLevel splits a brace body on top-level commas.
func splitTopLevel(body string) []string {
	var parts []string
	depth, start := 0, 0
	for i := range len(body) {
		switch body[i] {
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, body[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, body[start:])
}

// ExpandAll brace-expands every pattern in xs, flattened.
func ExpandAll(xs []string) []string {
	var out []string
	for _, x := range xs {
		out = append(out, ExpandBraces(x)...)
	}
	return out
}

// [<] 🤖🤖
