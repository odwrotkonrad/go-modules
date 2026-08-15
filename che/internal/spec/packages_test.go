package spec

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func loadDoc(t *testing.T, src string) []ProfileRecipe {
	t.Helper()
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(src), &doc))
	d := &Doc{}
	m := doc.Content[0]
	for i := 0; i+1 < len(m.Content); i += 2 {
		require.NoError(t, d.decodeKey(m.Content[i].Value, m.Content[i+1]))
	}
	return d.ProfileRecipes
}

type packageRefGot struct {
	Name          string   `yaml:"name"`
	Versions      []string `yaml:"versions,omitempty"`
	GlobalVersion string   `yaml:"globalVersion,omitempty"`
}

func summarizeRef(r PackageRef) packageRefGot {
	return packageRefGot{Name: r.Name, Versions: r.Versions, GlobalVersion: r.GlobalVersion}
}

func TestPackageRefUnmarshal(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/package_ref.test.spec.yml", func(t *testing.T, c testyml.Case[packageRefGot]) (packageRefGot, error) {
		var ref PackageRef
		if err := yaml.Unmarshal([]byte(c.Input.Args.String(t, 0)), &ref); err != nil {
			return packageRefGot{}, err
		}
		return summarizeRef(ref), nil
	})
}

func TestMakeProfileInstallPackages(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/make_profile_packages.test.spec.yml", func(t *testing.T, c testyml.Case[[]packageRefGot]) ([]packageRefGot, error) {
		recipes := loadDoc(t, c.Input.Args.String(t, 0))
		rec, err := FindRecipe(recipes, c.Input.Args.String(t, 1))
		require.NoError(t, err)
		ops, _, err := rec.MakeProfile(recipes, t.TempDir())
		if err != nil {
			return nil, err
		}
		got := make([]packageRefGot, 0, len(ops.InstallPackages.Packages))
		for _, r := range ops.InstallPackages.Packages {
			got = append(got, summarizeRef(r))
		}
		return got, nil
	})
}

type toolPackageRefGot struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version,omitempty"`
}

func TestMakeProfileInstallToolPackages(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/make_profile_tool_packages.test.spec.yml", func(t *testing.T, c testyml.Case[map[string][]toolPackageRefGot]) (map[string][]toolPackageRefGot, error) {
		recipes := loadDoc(t, c.Input.Args.String(t, 0))
		rec, err := FindRecipe(recipes, c.Input.Args.String(t, 1))
		require.NoError(t, err)
		ops, _, err := rec.MakeProfile(recipes, t.TempDir())
		if err != nil {
			return nil, err
		}
		got := map[string][]toolPackageRefGot{}
		for tool, refs := range ops.InstallPackages.ToolPackages {
			for _, r := range refs {
				got[tool] = append(got[tool], toolPackageRefGot(r))
			}
		}
		return got, nil
	})
}

// [<] 🤖🤖
