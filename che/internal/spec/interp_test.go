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
	Env     map[string]string   `yaml:"env"`
	Include []string            `yaml:"include"`
	Workdir map[string]string   `yaml:"workdir"`
	Unset   map[string][]string `yaml:"unset"`
	Sources map[string]string   `yaml:"sources"`
}

func TestLoadInterpolates(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/load_interp.test.spec.yml", func(t *testing.T, c testyml.Case[loadInterpWant]) (loadInterpWant, error) {
		a := c.Input.Args
		var in Interp
		a.To(t, 1, &in.Process)
		a.To(t, 2, &in.DotEnv)
		a.To(t, 3, &in.Ref)
		in.Policy = envinterp.Policy(a.String(t, 4))
		dir := testutil.Tree(t, map[string]string{"che.yml": a.String(t, 0)})
		d, err := Load(filepath.Join(dir, "che.yml"), in)
		if err != nil {
			return loadInterpWant{}, err
		}
		got := loadInterpWant{Env: d.Env, Unset: map[string][]string{}, Sources: map[string]string{}}
		for _, inc := range d.Include {
			got.Include = append(got.Include, inc.URI)
		}
		for _, rec := range d.ProfileRecipes {
			if wd := rec.Options.ProfileWorkingDirectory; wd != "" {
				if got.Workdir == nil {
					got.Workdir = map[string]string{}
				}
				got.Workdir[rec.Source.ProfileName] = wd
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
