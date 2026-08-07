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
    installPackages: [jq, tmux, kind]
  exclude:
    installPackages: [kind]
`)
	rec, err := FindRecipe(recipes, "main")
	require.NoError(t, err)
	ops, _, err := rec.MakeProfile(recipes, t.TempDir())
	require.NoError(t, err)
	require.Equal(t, []string{"git", "jq", "tmux"}, ops.InstallPackages.Packages)
}

// [<] 🤖🤖
