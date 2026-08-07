package fsutil

// [>] 🤖🤖

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func IsGlobMatch(pattern, path string) bool {
	if base, ok := strings.CutSuffix(pattern, "/**"); ok {
		return IsUnder(path, base)
	}
	ok, err := doublestar.Match(pattern, path)
	return err == nil && ok
}

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
		return []string{pattern}
	}
	pre, body, post := pattern[:open], pattern[open+1:closeAt], pattern[closeAt+1:]
	var out []string
	for _, alt := range splitTopLevel(body) {
		out = append(out, ExpandBraces(pre+alt+post)...)
	}
	return out
}

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

func ExpandAll(xs []string) []string {
	var out []string
	for _, x := range xs {
		out = append(out, ExpandBraces(x)...)
	}
	return out
}

// [<] 🤖🤖
