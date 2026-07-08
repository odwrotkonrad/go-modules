package lib

import (
	"strings"
)

// [>] 🤖🤖🤖
func leaderDepth(text string) (leaders int, rest string, ok bool) {
	i := 0
	for i < len(text) && text[i] == '#' {
		i++
	}
	if i < 2 {
		return 0, "", false
	}
	return i, text[i:], true
}

func sectionOpen(text string) (label string, depth int, ok bool) {
	leaders, rest, ok := leaderDepth(text)
	if !ok || !strings.HasPrefix(rest, "[>]") {
		return "", 0, false
	}
	label = strings.TrimPrefix(rest, "[>]")
	if i := strings.Index(label, includeTag); i >= 0 {
		label = label[:i]
	}
	label = strings.TrimRight(label, " 🤖")
	label = strings.TrimSpace(label)
	if label == "" {
		return "", 0, false
	}
	return label, leaders - 2, true
}

func sectionClose(text string) (depth int, ok bool) {
	leaders, rest, ok := leaderDepth(text)
	if !ok || !strings.HasPrefix(rest, "[<]") {
		return 0, false
	}
	return leaders - 2, true
}

func whatComment(text string) (string, bool) {
	const p = "#[what]"
	if !strings.HasPrefix(text, p) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(text, p)), true
}

// valsComment parses a #[vals] line: a parameter's accepted-values hint,
// rendered as <name>=<vals>.
func valsComment(text string) (string, bool) {
	const p = "#[vals]"
	if !strings.HasPrefix(text, p) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(text, p)), true
}

//[<] 🤖🤖🤖
