// Package envinterp substitutes ${{ env.NAME }}, ${{ var.NAME }} and their ${{ ... || default }} forms in che.yml scalars.
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

// Namespace is the ref prefix: env, var, or none for a che built-in.
type Namespace string

// Namespaces enumerates every Namespace value.
var Namespaces = struct{ Env, Var, Builtin Namespace }{"env", "var", ""}

// BuiltinNames lists every ${{ builtin.NAME }} che defines.
var BuiltinNames = []string{"repoRoot"}

// Ref is one parsed ${{ <namespace>.<name> }} occurrence.
type Ref struct {
	Namespace  Namespace
	Name       string
	Default    string
	HasDefault bool
}

// Lookup resolves one ref to its value, "" meaning unset.
type Lookup func(Ref) string

var refPattern = regexp.MustCompile(`\$\{\{\s*(?:(env|var)\.)?([A-Za-z_][A-Za-z0-9_]*)\s*(?:\|\|(.*?))?\s*\}\}`)

// KeyPattern is the shape every var and env name must take.
var KeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Expand substitutes every ref in s, returning the refs that were unset and had no default.
func Expand(s string, lookup Lookup) (string, []Ref) {
	if !strings.Contains(s, "${{") {
		return s, nil
	}
	var unset []Ref
	out := refPattern.ReplaceAllStringFunc(s, func(m string) string {
		ref := parseRef(refPattern.FindStringSubmatch(m))
		if v := lookup(ref); v != "" {
			return v
		}
		if ref.HasDefault {
			return ref.Default
		}
		unset = append(unset, ref)
		return ""
	})
	return out, unset
}

// EnvLookup adapts a name lookup to env refs only, var refs read as unset.
func EnvLookup(lookup func(string) string) Lookup {
	return func(ref Ref) string {
		if ref.Namespace != Namespaces.Env {
			return ""
		}
		return lookup(ref.Name)
	}
}

// MapLookup resolves env refs from env and var refs from vars.
func MapLookup(env, vars, builtins map[string]string) Lookup {
	return func(ref Ref) string {
		switch ref.Namespace {
		case Namespaces.Var:
			return vars[ref.Name]
		case Namespaces.Builtin:
			return builtins[ref.Name]
		}
		return env[ref.Name]
	}
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
	ref := Ref{Namespace: Namespace(m[1]), Name: m[2]}
	if strings.Contains(m[0], "||") {
		ref.HasDefault = true
		ref.Default = strings.TrimSpace(m[3])
	}
	return ref
}

// [<] 🤖🤖
