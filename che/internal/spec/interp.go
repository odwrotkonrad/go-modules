package spec

// [>] 🤖🤖

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec/envinterp"
)

// EnvLayer is one named env source, provenance reported by name.
type EnvLayer struct {
	Name string
	Env  map[string]string
}

// Interp is the lookup context one spec file loads under: env layers beneath and over its env: block,
// lowest first, and its resolved variables.
type Interp struct {
	Below    []EnvLayer
	Above    []EnvLayer
	Vars     VarSet
	Builtins map[string]string
	Policy   envinterp.Policy
}

type EnvUnset struct {
	Name string
	Path string
}

type EnvRef struct {
	envinterp.Ref
	Profile string
	Path    string
	Set     bool
	Source  string
}

const topLevelProfile = ""

// EnvSources names the env layers che builds, for provenance reports.
var EnvSources = struct{ Ref, Spec, CheEnv, Process, Inherited, IncludedCheEnv string }{
	"ref env", "spec env", "che.env", "process env", "inherited env", "included che.env",
}

type interpolator struct {
	in             Interp
	envBlock       map[string]string
	layers         []EnvLayer
	lookup         map[string]string
	vars           map[string]string
	profileVars    map[string]string
	unset          map[string][]EnvUnset
	varUnset       []EnvUnset
	builtinUnknown []EnvUnset
	refs           []EnvRef
}

func newInterpolator(in Interp, root *yaml.Node) *interpolator {
	it := &interpolator{in: in, unset: map[string][]EnvUnset{}, layers: slices.Concat(in.Below, in.Above)}
	blockEnv := flattenLayers(it.layers)
	it.vars, it.profileVars = in.Vars.Values(), in.Vars.ForProfiles()
	it.envBlock, it.varUnset = it.expandBlock(root, keyEnv, "/"+keyEnv+"/", envinterp.MapLookup(blockEnv, it.vars, in.Builtins))
	it.layers = slices.Concat(in.Below, []EnvLayer{{Name: EnvSources.Spec, Env: it.envBlock}}, in.Above)
	it.lookup = flattenLayers(it.layers)
	return it
}

func flattenLayers(layers []EnvLayer) map[string]string {
	out := map[string]string{}
	for _, layer := range layers {
		maps.Copy(out, layer.Env)
	}
	return out
}

func validateKeys(block string, values map[string]string) error {
	for _, key := range slices.Sorted(maps.Keys(values)) {
		if !envinterp.KeyPattern.MatchString(key) {
			return fmt.Errorf("%s: key %q must match %s", block, key, envinterp.KeyPattern)
		}
	}
	return nil
}

func (it *interpolator) expandBlock(root *yaml.Node, key, pathPrefix string, lookup envinterp.Lookup) (map[string]string, []EnvUnset) {
	block := mapValue(root, key)
	if block == nil || block.Kind != yaml.MappingNode {
		return nil, nil
	}
	out := map[string]string{}
	var varUnset []EnvUnset
	for i := 0; i+1 < len(block.Content); i += 2 {
		k, node := block.Content[i], block.Content[i+1]
		varUnset = append(varUnset, it.expandScalar(node, topLevelProfile, pathPrefix+k.Value, lookup)...)
		out[k.Value] = node.Value
	}
	return out, varUnset
}

const (
	keyOptions      = "options"
	keyEnv          = "env"
	keyVarDefs      = "variablesDefinitions"
	keySpecsInclude = "specsInclude"
	keyProfiles     = "profilesDefinitions"
)

var topLevelKeys = []string{keyOptions, keyEnv, keyVarDefs, keySpecsInclude, keyProfiles}

func (it *interpolator) walkDoc(root *yaml.Node) {
	specLookup := envinterp.MapLookup(it.lookup, it.vars, it.in.Builtins)
	profileLookup := envinterp.MapLookup(it.lookup, it.profileVars, it.in.Builtins)
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, node := root.Content[i].Value, root.Content[i+1]
		switch key {
		case keyEnv, keyVarDefs:
		case keyProfiles:
			if node.Kind != yaml.MappingNode {
				continue
			}
			for j := 0; j+1 < len(node.Content); j += 2 {
				name := node.Content[j].Value
				it.walk(node.Content[j+1], name, "/"+keyProfiles+"/"+name, profileLookup)
			}
		default:
			it.walk(node, topLevelProfile, "/"+key, specLookup)
		}
	}
}

func (it *interpolator) walk(n *yaml.Node, profile, path string, lookup envinterp.Lookup) {
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i].Value
			if key == keyVarDefs && profile != topLevelProfile && path == "/"+keyProfiles+"/"+profile {
				continue
			}
			it.walk(n.Content[i+1], profile, path+"/"+key, lookup)
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			it.walk(c, profile, fmt.Sprintf("%s/%d", path, i), lookup)
		}
	case yaml.ScalarNode:
		it.varUnset = append(it.varUnset, it.expandScalar(n, profile, path, lookup)...)
	}
}

func (it *interpolator) expandScalar(n *yaml.Node, profile, path string, lookup envinterp.Lookup) []EnvUnset {
	refs := envinterp.Refs(n.Value)
	if len(refs) == 0 {
		return nil
	}
	for _, ref := range refs {
		switch ref.Namespace {
		case envinterp.Namespaces.Env:
			set, source := it.provenance(ref, lookup)
			it.refs = append(it.refs, EnvRef{Ref: ref, Profile: profile, Path: path, Set: set, Source: source})
		case envinterp.Namespaces.Builtin:
			if !slices.Contains(envinterp.BuiltinNames, ref.Name) {
				it.builtinUnknown = append(it.builtinUnknown, EnvUnset{Name: ref.Name, Path: path})
			}
		}
	}
	value, unset := envinterp.Expand(n.Value, lookup)
	var varUnset []EnvUnset
	for _, ref := range unset {
		switch ref.Namespace {
		case envinterp.Namespaces.Var:
			varUnset = append(varUnset, EnvUnset{Name: ref.Name, Path: path})
		case envinterp.Namespaces.Env:
			it.unset[profile] = append(it.unset[profile], EnvUnset{Name: ref.Name, Path: path})
		}
	}
	n.Value, n.Tag, n.Style = value, "!!str", 0
	return varUnset
}

func (it *interpolator) provenance(ref envinterp.Ref, lookup envinterp.Lookup) (bool, string) {
	value := lookup(ref)
	if value == "" {
		return false, ""
	}
	for _, layer := range slices.Backward(it.layers) {
		if layer.Env[ref.Name] == value {
			return true, layer.Name
		}
	}
	return true, ""
}

func mapValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// EnvUnsetError formats unset bare refs of one spec as a single actionable error.
func EnvUnsetError(specPath string, unset []EnvUnset) error {
	lines := make([]string, 0, len(unset))
	for _, u := range unset {
		lines = append(lines, fmt.Sprintf("  %s at %s", u.Name, u.Path))
	}
	return fmt.Errorf("%s: unset env vars:\n%s\n(export them, add a || default, or set options.envUnset: empty)",
		specPath, strings.Join(lines, "\n"))
}

// BuiltinUnknownError formats bare refs naming no che built-in as a single actionable error.
func BuiltinUnknownError(specPath string, unknown []EnvUnset) error {
	lines := make([]string, 0, len(unknown))
	for _, u := range unknown {
		lines = append(lines, fmt.Sprintf("  unknown built-in ${{ %s }} at %s", u.Name, u.Path))
	}
	return fmt.Errorf("%s:\n%s\n(defined: %s. env vars are ${{ env.NAME }}, variables ${{ var.NAME }})",
		specPath, strings.Join(lines, "\n"), strings.Join(envinterp.BuiltinNames, ", "))
}

// VarUnsetError formats unset var refs of one spec as a single actionable error.
func VarUnsetError(specPath string, unset []EnvUnset) error {
	lines := make([]string, 0, len(unset))
	for _, u := range unset {
		lines = append(lines, fmt.Sprintf("  %s at %s", u.Name, u.Path))
	}
	return fmt.Errorf("%s: unset vars:\n%s\n(declare them in variablesDefinitions, set them in %s or %s, pass them at the specsInclude or profile entry, or add a || default)",
		specPath, strings.Join(lines, "\n"), VariablesFileName, VariablesLocalFileName)
}

// [<] 🤖🤖
