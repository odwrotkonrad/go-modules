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
// var layers beneath and over its variables: block, lowest first.
type Interp struct {
	Below    []EnvLayer
	Above    []EnvLayer
	VarsBase map[string]string
	VarsOver map[string]string
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
	in       Interp
	envBlock map[string]string
	varBlock map[string]string
	layers   []EnvLayer
	lookup   map[string]string
	vars     map[string]string
	unset    map[string][]EnvUnset
	varUnset []EnvUnset
	refs     []EnvRef
}

func newInterpolator(in Interp, root *yaml.Node) (*interpolator, error) {
	it := &interpolator{in: in, unset: map[string][]EnvUnset{}, layers: slices.Concat(in.Below, in.Above)}
	blockEnv := flattenLayers(it.layers)
	it.varBlock, it.varUnset = it.expandBlock(root, "variables", "/variables/", envinterp.MapLookup(blockEnv, nil))
	if err := validateKeys("variables", it.varBlock); err != nil {
		return nil, err
	}
	explicit := mergeEnv(in.VarsBase, it.varBlock)
	it.envBlock, _ = it.expandBlock(root, "env", "/env/", envinterp.MapLookup(blockEnv, mergeEnv(mergeEnv(blockEnv, explicit), in.VarsOver)))
	it.layers = slices.Concat(in.Below, []EnvLayer{{Name: EnvSources.Spec, Env: it.envBlock}}, in.Above)
	it.lookup = flattenLayers(it.layers)
	// [why] env sets a variable implicitly, only while nothing declares it: a consumer's che.env or a CI
	// job variable reaches every included spec with nothing on the refs, any explicit declaration wins
	it.vars = mergeEnv(mergeEnv(it.lookup, explicit), in.VarsOver)
	return it, nil
}

func mergeEnv(base, overlay map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(overlay))
	maps.Copy(out, base)
	maps.Copy(out, overlay)
	return out
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

func (it *interpolator) walkDoc(root *yaml.Node) {
	lookup := envinterp.MapLookup(it.lookup, it.vars)
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, node := root.Content[i].Value, root.Content[i+1]
		if key == "env" || key == "variables" {
			continue
		}
		profile := key
		if key == "options" || key == "include" {
			profile = topLevelProfile
		}
		it.walk(node, profile, "/"+key, lookup)
	}
}

func (it *interpolator) walk(n *yaml.Node, profile, path string, lookup envinterp.Lookup) {
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			it.walk(n.Content[i+1], profile, path+"/"+n.Content[i].Value, lookup)
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
		if ref.Namespace != envinterp.Namespaces.Env {
			continue
		}
		set, source := it.provenance(ref, lookup)
		it.refs = append(it.refs, EnvRef{Ref: ref, Profile: profile, Path: path, Set: set, Source: source})
	}
	value, unset := envinterp.Expand(n.Value, lookup)
	var varUnset []EnvUnset
	for _, ref := range unset {
		if ref.Namespace == envinterp.Namespaces.Var {
			varUnset = append(varUnset, EnvUnset{Name: ref.Name, Path: path})
			continue
		}
		it.unset[profile] = append(it.unset[profile], EnvUnset{Name: ref.Name, Path: path})
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

// VarUnsetError formats unset var refs of one spec as a single actionable error.
func VarUnsetError(specPath string, unset []EnvUnset) error {
	lines := make([]string, 0, len(unset))
	for _, u := range unset {
		lines = append(lines, fmt.Sprintf("  %s at %s", u.Name, u.Path))
	}
	return fmt.Errorf("%s: unset vars:\n%s\n(declare them in che.variables.yml, the spec's variables: block, the ref's variables:, or add a || default)",
		specPath, strings.Join(lines, "\n"))
}

// [<] 🤖🤖
