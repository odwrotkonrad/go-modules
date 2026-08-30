package spec

// [>] 🤖🤖

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// [<] 🤖🤖

// [>] 🤖🤖 definitions

func (d *VarDef) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode && node.Tag == "!!null" {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("want a mapping of required | scope | description | type | enum, or nothing")
	}
	allowed := []string{"required", "scope", "description", "type", "enum"}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		if key == "default" {
			return fmt.Errorf("default is not allowed, put it in %s", VariablesDefaultsFileName)
		}
		if !slices.Contains(allowed, key) {
			return fmt.Errorf("unknown key %q: want %s", key, strings.Join(allowed, " | "))
		}
	}
	type alias VarDef
	if err := node.Decode((*alias)(d)); err != nil {
		return err
	}
	if d.Scope != "" && !slices.Contains(VarScopeNames, string(d.Scope)) {
		return fmt.Errorf("scope %q: want %s", d.Scope, strings.Join(VarScopeNames, " | "))
	}
	if d.Type != "" && !slices.Contains(VarTypeNames, string(d.Type)) {
		return fmt.Errorf("type %q: want %s", d.Type, strings.Join(VarTypeNames, " | "))
	}
	return nil
}

// Declared reports whether any definition, spec or profile, names the variable.
func (d SpecVarDefs) Declared(name string) bool {
	if _, ok := d.Spec[name]; ok {
		return true
	}
	for _, defs := range d.Profiles {
		if _, ok := defs[name]; ok {
			return true
		}
	}
	return false
}

// Def returns the effective definition of a name (the spec's, else the first profile's declaring
// it), scope and required filled from all where the definition sets neither.
func (d SpecVarDefs) Def(name string) (VarDef, bool) {
	def, ok := d.Spec[name]
	if !ok {
		for _, profile := range slices.Sorted(maps.Keys(d.Profiles)) {
			if def, ok = d.Profiles[profile][name]; ok {
				break
			}
		}
	}
	if !ok {
		return VarDef{}, false
	}
	if def.Scope == "" {
		def.Scope = d.All.Scope
	}
	if def.Required == nil {
		def.Required = d.All.Required
	}
	return def, true
}

func (d SpecVarDefs) names() []string {
	seen := map[string]bool{}
	for name := range d.Spec {
		seen[name] = true
	}
	for _, defs := range d.Profiles {
		for name := range defs {
			seen[name] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

// [<] 🤖🤖

// [>] 🤖🤖 resolution

// ResolveVars layers one spec's variable values, lowest first: defaults < env (declared names only)
// < inherited recursive-scope values < shared < local < passed. Every value must be declared,
// every required definition valued, every literal typed.
func ResolveVars(defs SpecVarDefs, files RepoFiles, env map[string]string, passed map[string]string, inherited VarSet) (VarSet, error) {
	out := VarSet{}
	layer := func(source string, values VarSet) {
		for name, v := range values {
			out[name] = VarValue{Value: v.Value, Scope: v.Scope, Source: source}
		}
	}
	layer(VarSources.Defaults, files.Defaults)
	for _, name := range defs.names() {
		if value, ok := env[name]; ok {
			out[name] = VarValue{Value: value, Source: VarSources.Env}
		}
	}
	for name, v := range inherited {
		out[name] = VarValue{Value: v.Value, Scope: v.Scope, Source: VarSources.Inherited, inherited: true}
	}
	layer(VarSources.Shared, files.Shared)
	layer(VarSources.Local, files.Local)
	for name, value := range passed {
		out[name] = VarValue{Value: value, Source: VarSources.Passed}
	}
	for name, v := range out {
		def, _ := defs.Def(name)
		if v.Scope == "" {
			v.Scope = def.Scope
		}
		if v.Scope == "" {
			v.Scope = VarScopes.InvokingSpecDefinedProfiles
		}
		out[name] = v
	}
	return out, validateVars(defs, out, files)
}

func validateVars(defs SpecVarDefs, set VarSet, files RepoFiles) error {
	var errs []string
	for _, name := range slices.Sorted(maps.Keys(set)) {
		v := set[name]
		if v.inherited {
			continue
		}
		def, ok := defs.Def(name)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: set by %s but declared by no variablesDefinitions", name, v.Source))
			continue
		}
		if err := checkLiteral(def, v.Value); err != nil {
			errs = append(errs, fmt.Sprintf("%s (%s): %s", name, v.Source, err))
		}
	}
	var missing []string
	for _, name := range defs.names() {
		def, _ := defs.Def(name)
		if _, ok := set[name]; def.Required != nil && *def.Required && !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		errs = append(errs, fmt.Sprintf("required variables without a value: %s (set them in %s, %s, %s, or pass them at the specsInclude or profile entry)",
			strings.Join(missing, ", "), CheFileCandidates(files.Root, VariablesFileName), VariablesLocalFileName, VariablesDefaultsFileName))
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("variables:\n  %s", strings.Join(errs, "\n  "))
}

func checkLiteral(def VarDef, value string) error {
	switch def.Type {
	case VarTypes.Integer:
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return fmt.Errorf("want an integer, got %q", value)
		}
	case VarTypes.Boolean:
		if value != "true" && value != "false" {
			return fmt.Errorf("want true | false, got %q", value)
		}
	}
	if len(def.Enum) > 0 && !slices.Contains(def.Enum, value) {
		return fmt.Errorf("want one of %s, got %q", strings.Join(def.Enum, " | "), value)
	}
	return nil
}

// [<] 🤖🤖

// [>] 🤖🤖 projections

// Values flattens the set: what the spec's own top level interpolates.
func (s VarSet) Values() map[string]string {
	out := make(map[string]string, len(s))
	for name, v := range s {
		out[name] = v.Value
	}
	return out
}

func (s VarSet) scoped(scopes ...VarScope) VarSet {
	out := VarSet{}
	for name, v := range s {
		if slices.Contains(scopes, v.Scope) {
			out[name] = v
		}
	}
	return out
}

// ForProfiles is what the profiles this spec defines interpolate and render with.
func (s VarSet) ForProfiles() map[string]string {
	return s.scoped(VarScopes.InvokingSpecDefinedProfiles, VarScopes.RecursiveSpecsAndProfiles).Values()
}

// ForEmbedded is what an embedded spec's top level inherits.
func (s VarSet) ForEmbedded() VarSet {
	return s.scoped(VarScopes.RecursiveSpecs, VarScopes.RecursiveSpecsAndProfiles)
}

// ForEmbeddedProfiles is what a profile ref into an embedded spec inherits.
func (s VarSet) ForEmbeddedProfiles() VarSet {
	return s.scoped(VarScopes.RecursiveSpecsAndProfiles)
}

// Names lists the set's variable names, sorted.
func (s VarSet) Names() []string { return slices.Sorted(maps.Keys(s)) }

// [<] 🤖🤖
