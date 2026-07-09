package spec

// [>] 🤖🤖

import (
	"fmt"
	"regexp"
	"strings"
)

// destRule is a parsed sed-style dest rewrite: pattern, replacement ($1
// backrefs), global flag (absent -> first match only).
type destRule struct {
	re     *regexp.Regexp
	repl   string
	global bool
}

// parseDestRule parses a link dest rewrite "s/<pattern>/<replacement>/[g]"
// (Go regexp flavor, `\/` escapes a literal slash). Anything else errors:
// the 1:1 form is the bare glob string, not a dest value.
func parseDestRule(s string) (*destRule, error) {
	malformed := fmt.Errorf("link dest %q: want s/<pattern>/<replacement>/[g]", s)
	body, ok := strings.CutPrefix(s, "s/")
	if !ok {
		return nil, malformed
	}
	parts := splitRule(body)
	if len(parts) != 3 || parts[0] == "" || (parts[2] != "" && parts[2] != "g") {
		return nil, malformed
	}
	re, err := regexp.Compile(parts[0])
	if err != nil {
		return nil, fmt.Errorf("link dest %q: %w", s, err)
	}
	return &destRule{re: re, repl: parts[1], global: parts[2] == "g"}, nil
}

// splitRule splits a rule body on the `/` delimiter, unescaping `\/`.
func splitRule(s string) []string {
	var parts []string
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && s[i+1] == '/':
			b.WriteByte('/')
			i++
		case s[i] == '/':
			parts = append(parts, b.String())
			b.Reset()
		default:
			b.WriteByte(s[i])
		}
	}
	return append(parts, b.String())
}

// apply rewrites rel: every match with the g flag, else the first only. No
// match -> rel unchanged.
func (r *destRule) apply(rel string) string {
	if r.global {
		return r.re.ReplaceAllString(rel, r.repl)
	}
	loc := r.re.FindStringSubmatchIndex(rel)
	if loc == nil {
		return rel
	}
	return rel[:loc[0]] + string(r.re.ExpandString(nil, r.repl, rel, loc)) + rel[loc[1]:]
}

// [<] 🤖🤖
