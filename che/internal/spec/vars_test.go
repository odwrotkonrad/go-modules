package spec

// [>] 🤖🤖

import (
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

type resolveVarsWant struct {
	Values              map[string]string `yaml:"values"`
	Sources             map[string]string `yaml:"sources"`
	Scopes              map[string]string `yaml:"scopes"`
	ForProfiles         []string          `yaml:"forProfiles"`
	ForEmbedded         []string          `yaml:"forEmbedded"`
	ForEmbeddedProfiles []string          `yaml:"forEmbeddedProfiles"`
}

type resolveVarsDefs struct {
	All      VarDefaults        `yaml:"all"`
	Spec     VarDefs            `yaml:"spec"`
	Profiles map[string]VarDefs `yaml:"profiles"`
}

type resolveVarsFiles struct {
	Defaults map[string]varsFileEntry `yaml:"defaults"`
	Shared   map[string]varsFileEntry `yaml:"shared"`
	Local    map[string]varsFileEntry `yaml:"local"`
}

func varSetOf(entries map[string]varsFileEntry) VarSet {
	if entries == nil {
		return nil
	}
	out := VarSet{}
	for name, e := range entries {
		out[name] = VarValue{Value: e.Value, Scope: e.Scope}
	}
	return out
}

func TestResolveVars(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_vars.test.spec.yml", func(t *testing.T, c testyml.Case[resolveVarsWant]) (resolveVarsWant, error) {
		a := c.Input.Args
		var defs resolveVarsDefs
		var files resolveVarsFiles
		var env, passed map[string]string
		var inherited map[string]varsFileEntry
		a.To(t, 0, &defs)
		a.To(t, 1, &files)
		a.To(t, 2, &env)
		a.To(t, 3, &passed)
		a.To(t, 4, &inherited)
		repo := RepoFiles{Root: "/repo", Defaults: varSetOf(files.Defaults), Shared: varSetOf(files.Shared), Local: varSetOf(files.Local)}
		set, err := ResolveVars(SpecVarDefs(defs), repo, env, passed, varSetOf(inherited))
		if err != nil {
			return resolveVarsWant{}, err
		}
		got := resolveVarsWant{Values: set.Values(), Sources: map[string]string{}, Scopes: map[string]string{}}
		for name, v := range set {
			got.Sources[name] = v.Source
			got.Scopes[name] = string(v.Scope)
		}
		if c.Expected.Output.ForProfiles != nil {
			got.ForProfiles = VarSet(nil).namesOf(set.ForProfiles())
			got.ForEmbedded = set.ForEmbedded().Names()
			got.ForEmbeddedProfiles = set.ForEmbeddedProfiles().Names()
		}
		return got, nil
	})
}

func (VarSet) namesOf(values map[string]string) []string {
	set := VarSet{}
	for name := range values {
		set[name] = VarValue{}
	}
	return set.Names()
}

// [<] 🤖🤖
