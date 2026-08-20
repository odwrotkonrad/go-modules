// Package envinterp substitutes ${{ env.NAME }} and ${{ env.NAME || default }} in che.yml scalars.
package envinterp

// [>] 🤖🤖

import (
	"regexp"
	"strings"
)

// Policy decides what an unset bare ref does: error the load, or read as "".
type Policy string

// Policies enumerates every Policy value.
var Policies = struct{ Error, Empty Policy }{"error", "empty"}

// Ref is one parsed ${{ env.* }} occurrence.
type Ref struct {
	Name       string
	Default    string
	HasDefault bool
}

var refPattern = regexp.MustCompile(`\$\{\{\s*env\.([A-Za-z_][A-Za-z0-9_]*)\s*(?:\|\|(.*?))?\s*\}\}`)

// Expand substitutes every ref in s, returning the names of unset refs that had no default.
func Expand(s string, lookup func(string) string) (string, []string) {
	if !strings.Contains(s, "${{") {
		return s, nil
	}
	var unset []string
	out := refPattern.ReplaceAllStringFunc(s, func(m string) string {
		ref := parseRef(refPattern.FindStringSubmatch(m))
		if v := lookup(ref.Name); v != "" {
			return v
		}
		if ref.HasDefault {
			return ref.Default
		}
		unset = append(unset, ref.Name)
		return ""
	})
	return out, unset
}

// Refs reports every ref in s, in order.
func Refs(s string) []Ref {
	if !strings.Contains(s, "${{") {
		return nil
	}
	var refs []Ref
	for _, m := range refPattern.FindAllStringSubmatch(s, -1) {
		refs = append(refs, parseRef(m))
	}
	return refs
}

// ValidPolicy reports whether p names a known policy.
func ValidPolicy(p Policy) bool {
	return p == Policies.Error || p == Policies.Empty
}

func parseRef(m []string) Ref {
	ref := Ref{Name: m[1]}
	if strings.Contains(m[0], "||") {
		ref.HasDefault = true
		ref.Default = strings.TrimSpace(m[2])
	}
	return ref
}

// [<] 🤖🤖
