package spec

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

func TestMakeProfileInstallPackages(t *testing.T) {
	recipes := loadDoc(t, `
base:
  include:
    installPackages: [git, jq]
main:
  include:
    profiles: [base]
    installPackages: [{name: jq, versions: "1.7.*"}, tmux, kind]
  exclude:
    installPackages: [kind]
`)
	rec, err := FindRecipe(recipes, "main")
	require.NoError(t, err)
	ops, _, err := rec.MakeProfile(recipes, t.TempDir())
	require.NoError(t, err)
	require.Equal(t, []PackageRef{{Name: "git"}, {Name: "jq", Versions: StringOrList{"1.7.*"}}, {Name: "tmux"}}, ops.InstallPackages.Packages)
}

func TestPackageRefRequiresName(t *testing.T) {
	var ref PackageRef
	require.ErrorContains(t, yaml.Unmarshal([]byte("{versions: 1.2.3}"), &ref), "requires name")
}

func TestPackageRefVersionListAndGlobal(t *testing.T) {
	var ref PackageRef
	require.NoError(t, yaml.Unmarshal([]byte(`{name: node, versions: [24.16.0, 22.14.0], globalVersion: 22.14.0}`), &ref))
	require.Equal(t, StringOrList{"24.16.0", "22.14.0"}, ref.Versions)
	require.Equal(t, "22.14.0", ref.GlobalVersion)

	var scalar PackageRef
	require.NoError(t, yaml.Unmarshal([]byte(`{name: go, versions: 1.26.4}`), &scalar))
	require.Equal(t, StringOrList{"1.26.4"}, scalar.Versions)
}

func TestPackageRefGlobalVersionMustBeListed(t *testing.T) {
	var ref PackageRef
	err := yaml.Unmarshal([]byte(`{name: node, versions: [24.16.0], globalVersion: 22.14.0}`), &ref)
	require.ErrorContains(t, err, "globalVersion 22.14.0 is not among versions")
}

// [<] 🤖🤖
