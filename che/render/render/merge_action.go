package render

// [>] 🤖🤖

import (
	"fmt"
	"regexp"
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
	return func(value any) string { return markMergeAction(action, value) }
}

// [<] 🤖🤖
