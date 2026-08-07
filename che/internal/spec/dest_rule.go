package spec

// [>] 🤖🤖

import (
	"fmt"
	"regexp"
	"strings"
)

func ruleFromDest(source, dest string) (*destRule, error) {
	if len(dest) >= 2 && dest[0] == 's' && dest[1] != '\\' && !isWord(dest[1]) {
		return parseDestRule(dest)
	}
	const sfx = "/**"
	if strings.HasSuffix(source, sfx) && strings.HasSuffix(dest, sfx) {
		srcPrefix := strings.TrimSuffix(source, sfx)
		dstPrefix := strings.TrimSuffix(dest, sfx)
		re, err := regexp.Compile("^" + regexp.QuoteMeta(srcPrefix))
		if err != nil {
			return nil, fmt.Errorf("dest sugar %q: %w", dest, err)
		}
		return &destRule{re: re, repl: dstPrefix, global: false}, nil
	}
	return nil, fmt.Errorf("dest %q: want s<delim><pattern><delim><replacement><delim>[g] or <prefix>/** with source <prefix>/**", dest)
}

func isWord(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func parseDestRule(s string) (*destRule, error) {
	malformed := fmt.Errorf("dest rule %q: want s<delim><pattern><delim><replacement><delim>[g]", s)
	if len(s) < 2 || s[0] != 's' {
		return nil, malformed
	}
	delim := s[1]
	if delim == '\\' {
		return nil, malformed
	}
	parts := splitRule(s[2:], delim)
	if len(parts) != 3 || parts[0] == "" || (parts[2] != "" && parts[2] != "g") {
		return nil, malformed
	}
	re, err := regexp.Compile(parts[0])
	if err != nil {
		return nil, fmt.Errorf("dest rule %q: %w", s, err)
	}
	return &destRule{re: re, repl: parts[1], global: parts[2] == "g"}, nil
}

func splitRule(s string, delim byte) []string {
	var parts []string
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && s[i+1] == delim:
			b.WriteByte(delim)
			i++
		case s[i] == delim:
			parts = append(parts, b.String())
			b.Reset()
		default:
			b.WriteByte(s[i])
		}
	}
	return append(parts, b.String())
}

func (r *destRule) apply(rel string) string {
	if r.global {
		return r.re.ReplaceAllLiteralString(rel, r.repl)
	}
	loc := r.re.FindStringIndex(rel)
	if loc == nil {
		return rel
	}
	return rel[:loc[0]] + r.repl + rel[loc[1]:]
}

// [<] 🤖🤖
