package spec

// [>] 🤖🤖

import (
	"fmt"
	"maps"
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec/envinterp"
)

type Interp struct {
	Process map[string]string
	DotEnv  map[string]string
	Ref     map[string]string
	Policy  envinterp.Policy
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

var envSources = struct{ Ref, Spec, DotEnv, Process string }{"ref env", "spec env", ".env", "process env"}

type interpolator struct {
	in       Interp
	envBlock map[string]string
	lookup   map[string]string
	unset    map[string][]EnvUnset
	refs     []EnvRef
}

func newInterpolator(in Interp, root *yaml.Node) *interpolator {
	it := &interpolator{in: in, unset: map[string][]EnvUnset{}}
	launch := mergeEnv(in.DotEnv, in.Process)
	blockLookup := mergeEnv(launch, in.Ref)
	it.envBlock = it.expandEnvBlock(root, blockLookup)
	it.lookup = mergeEnv(mergeEnv(launch, it.envBlock), in.Ref)
	return it
}

func mergeEnv(base, overlay map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(overlay))
	maps.Copy(out, base)
	maps.Copy(out, overlay)
	return out
}

func (it *interpolator) expandEnvBlock(root *yaml.Node, lookup map[string]string) map[string]string {
	block := mapValue(root, "env")
	if block == nil || block.Kind != yaml.MappingNode {
		return nil
	}
	out := map[string]string{}
	for i := 0; i+1 < len(block.Content); i += 2 {
		key, node := block.Content[i], block.Content[i+1]
		it.expandScalar(node, topLevelProfile, "/env/"+key.Value, lookup)
		out[key.Value] = node.Value
	}
	return out
}

func (it *interpolator) walkDoc(root *yaml.Node) {
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, node := root.Content[i].Value, root.Content[i+1]
		if key == "env" {
			continue
		}
		profile := key
		if key == "options" || key == "include" {
			profile = topLevelProfile
		}
		it.walk(node, profile, "/"+key)
	}
}

func (it *interpolator) walk(n *yaml.Node, profile, path string) {
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			it.walk(n.Content[i+1], profile, path+"/"+n.Content[i].Value)
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			it.walk(c, profile, fmt.Sprintf("%s/%d", path, i))
		}
	case yaml.ScalarNode:
		it.expandScalar(n, profile, path, it.lookup)
	}
}

func (it *interpolator) expandScalar(n *yaml.Node, profile, path string, lookup map[string]string) {
	refs := envinterp.Refs(n.Value)
	if len(refs) == 0 {
		return
	}
	for _, ref := range refs {
		set, source := it.provenance(ref.Name, lookup)
		it.refs = append(it.refs, EnvRef{Ref: ref, Profile: profile, Path: path, Set: set, Source: source})
	}
	value, unset := envinterp.Expand(n.Value, func(k string) string { return lookup[k] })
	for _, name := range unset {
		it.unset[profile] = append(it.unset[profile], EnvUnset{Name: name, Path: path})
	}
	n.Value, n.Tag, n.Style = value, "!!str", 0
}

func (it *interpolator) provenance(name string, lookup map[string]string) (bool, string) {
	if lookup[name] == "" {
		return false, ""
	}
	switch {
	case it.in.Ref[name] != "":
		return true, envSources.Ref
	case it.envBlock[name] != "" && lookup[name] == it.envBlock[name]:
		return true, envSources.Spec
	case it.in.Process[name] != "":
		return true, envSources.Process
	default:
		return true, envSources.DotEnv
	}
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

// [<] 🤖🤖
