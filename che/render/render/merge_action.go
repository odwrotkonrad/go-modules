package render

// [>] 🤖🤖

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
)

const (
	MergeActionAlwaysUpdate   = "alwaysUpdate"
	MergeActionKeepIfExisting = "keepIfExisting"
	MergeActionDependency     = "dependency"
	MergeActionShell          = "shell"
)

const (
	MergeUpdateNone         = "none"
	MergeUpdateDependencies = "dependencies"
	MergeUpdateShell        = "shell"
	MergeUpdateAll          = "all"
)

var MergeUpdateModes = []string{MergeUpdateNone, MergeUpdateDependencies, MergeUpdateShell, MergeUpdateAll}

const (
	mergeActionOpen  = "\x1e"
	mergeActionClose = "\x1f"
)

var mergeActionPattern = regexp.MustCompile(mergeActionOpen +
	"(" + strings.Join([]string{MergeActionAlwaysUpdate, MergeActionKeepIfExisting, MergeActionDependency, MergeActionShell}, "|") + ")" +
	"(?::([A-Za-z0-9+/=]*))?" + mergeActionClose)

type mergeMark struct {
	action   string
	shellCmd string
}

func (m mergeMark) isShell() bool { return m.action == MergeActionShell || m.shellCmd != "" }

func (m mergeMark) String() string {
	if m.action == "" {
		return ""
	}
	if m.shellCmd == "" {
		return mergeActionOpen + m.action + mergeActionClose
	}
	return mergeActionOpen + m.action + ":" + base64.StdEncoding.EncodeToString([]byte(m.shellCmd)) + mergeActionClose
}

// StripMergeActions removes every merge-action marker from a rendered body.
func StripMergeActions(body []byte) []byte {
	return mergeActionPattern.ReplaceAll(body, nil)
}

func markMergeAction(action string, value any) string {
	prev, bare := splitMergeAction(fmt.Sprint(value))
	return mergeMark{action: action, shellCmd: prev.shellCmd}.String() + bare
}

func splitMergeAction(value string) (mark mergeMark, bare string) {
	loc := mergeActionPattern.FindStringSubmatchIndex(value)
	if loc == nil || loc[0] != 0 {
		return mergeMark{}, value
	}
	mark.action = value[loc[2]:loc[3]]
	if loc[4] >= 0 {
		cmd, err := base64.StdEncoding.DecodeString(value[loc[4]:loc[5]])
		if err == nil {
			mark.shellCmd = string(cmd)
		}
	}
	return mark, value[loc[1]:]
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

func deferredShellFunc(command string) (string, error) {
	return mergeMark{action: MergeActionShell, shellCmd: command}.String(), nil
}

func mergeActionFunc(action string) func(any) string {
	return func(value any) string { return markMergeActionBlock(action, value) }
}

func markMergeActionBlock(action string, value any) string {
	text := fmt.Sprint(value)
	if !strings.Contains(text, "\n") {
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
