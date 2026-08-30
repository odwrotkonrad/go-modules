package spec

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec/envinterp"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

type loadInterpWant struct {
	Env       map[string]string   `yaml:"env"`
	Lookup    map[string]string   `yaml:"lookup"`
	Include   []string            `yaml:"include"`
	Workdir   map[string]string   `yaml:"workdir"`
	Templates map[string][]string `yaml:"templates"`
	Unset     map[string][]string `yaml:"unset"`
	Sources   map[string]string   `yaml:"sources"`
}

type loadInterpLayers struct {
	Below    []EnvLayer          `yaml:"below"`
	Above    []EnvLayer          `yaml:"above"`
	Vars     map[string]VarValue `yaml:"vars"`
	Builtins map[string]string   `yaml:"builtins"`
}

func TestLoadInterpolates(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/load_interp.test.spec.yml", func(t *testing.T, c testyml.Case[loadInterpWant]) (loadInterpWant, error) {
		a := c.Input.Args
		var layers loadInterpLayers
		a.To(t, 1, &layers)
		in := Interp{Below: layers.Below, Above: layers.Above, Vars: layers.Vars, Builtins: layers.Builtins, Policy: envinterp.Policy(a.String(t, 2))}
		dir := testutil.Tree(t, map[string]string{"che.yml": a.String(t, 0)})
		d, err := Load(filepath.Join(dir, "che.yml"), in)
		if err != nil {
			return loadInterpWant{}, err
		}
		got := loadInterpWant{Env: d.Env, Unset: map[string][]string{}, Sources: map[string]string{}}
		if c.Expected.Output.Lookup != nil {
			got.Lookup = map[string]string{}
			for k := range c.Expected.Output.Lookup {
				got.Lookup[k] = d.Lookup[k]
			}
		}
		for _, inc := range d.SpecsInclude {
			got.Include = append(got.Include, inc.URI)
		}
		for _, rec := range d.ProfileRecipes {
			if wd := rec.Options.ProfileWorkingDirectory; wd != "" {
				if got.Workdir == nil {
					got.Workdir = map[string]string{}
				}
				got.Workdir[rec.Source.ProfileName] = wd
			}
			for _, node := range rec.Include.RenderTemplates {
				if got.Templates == nil {
					got.Templates = map[string][]string{}
				}
				got.Templates[rec.Source.ProfileName] = append(got.Templates[rec.Source.ProfileName], node.Source)
			}
		}
		for profile, unset := range d.EnvUnset {
			for _, u := range unset {
				got.Unset[profile] = append(got.Unset[profile], u.Name+"@"+u.Path)
			}
		}
		for _, ref := range d.EnvRefs {
			if ref.Set {
				got.Sources[ref.Name] = ref.Source
			}
		}
		return got, nil
	})
}

// [<] 🤖🤖
