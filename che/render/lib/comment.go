package lib

import (
	"strings"
)

// [>] 🤖🤖🤖
func cutHashes(text string) (count int, rest string, ok bool) {
	i := 0
	for i < len(text) && text[i] == '#' {
		i++
	}
	if i < 2 {
		return 0, "", false
	}
	return i, text[i:], true
}

func parseSectionOpen(text string) (label string, depth int, ok bool) {
	hashes, rest, ok := cutHashes(text)
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
	return label, hashes - 2, true
}

func parseSectionClose(text string) (depth int, ok bool) {
	hashes, rest, ok := cutHashes(text)
	if !ok || !strings.HasPrefix(rest, "[<]") {
		return 0, false
	}
	return hashes - 2, true
}

func parseTagComment(text, tag string) (string, bool) {
	p := "#[" + tag + "]"
	if !strings.HasPrefix(text, p) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(text, p)), true
}

//[<] 🤖🤖🤖
