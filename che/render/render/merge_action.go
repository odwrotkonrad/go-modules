package render

// [>] 🤖🤖

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	MergeActionAlwaysUpdate   = "alwaysUpdate"
	MergeActionKeepIfExisting = "keepIfExisting"
)

const (
	mergeActionOpen  = "\x1e"
	mergeActionClose = "\x1f"
)

var mergeActionPattern = regexp.MustCompile(mergeActionOpen + "(" + MergeActionAlwaysUpdate + "|" + MergeActionKeepIfExisting + ")" + mergeActionClose)

// StripMergeActions removes every merge-action marker from a rendered body.
func StripMergeActions(body []byte) []byte {
	return mergeActionPattern.ReplaceAll(body, nil)
}

func markMergeAction(action string, value any) string {
	_, bare := splitMergeAction(fmt.Sprint(value))
	return mergeActionOpen + action + mergeActionClose + bare
}

func splitMergeAction(value string) (action, bare string) {
	loc := mergeActionPattern.FindStringSubmatchIndex(value)
	if loc == nil || loc[0] != 0 {
		return "", value
	}
	return value[loc[2]:loc[3]], value[loc[1]:]
}

func withMergeAction(action string, resolve func(string) (string, error)) func(string) (string, error) {
	return func(arg string) (string, error) {
		v, err := resolve(arg)
		if err != nil {
			return "", err
		}
		return markMergeAction(action, v), nil
	}
}

func mergeActionFunc(action string) func(any) string {
	return func(value any) string { return markMergeActionBlock(action, value) }
}

func markMergeActionBlock(action string, value any) string {
	text := fmt.Sprint(value)
	if !strings.Contains(strings.TrimRight(text, "\n"), "\n") {
		return markMergeAction(action, text)
	}
	var out strings.Builder
	for line := range strings.Lines(text) {
		out.WriteString(markMergeActionLine(action, line))
	}
	return out.String()
}

func markMergeActionLine(action, line string) string {
	body := strings.TrimSuffix(line, "\n")
	eol := strings.TrimPrefix(line, body)
	if trimmed := strings.TrimSpace(body); trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return line
	}
	key, val, ok := strings.Cut(body, "=")
	if !ok {
		return line
	}
	return key + "=" + markMergeAction(action, val) + eol
}

// [<] 🤖🤖
