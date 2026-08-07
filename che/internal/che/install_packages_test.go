package che

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/options"
)

func TestOperationsOrderInstallPackagesBeforeRunScripts(t *testing.T) {
	p, recipes, _ := setupProfile(t, options.Options{})
	var err error
	p.OperationsReady, err = p.prepareOperations(recipes)
	require.NoError(t, err)
	var names []string
	for _, op := range p.OperationsReady {
		names = append(names, op.Name())
	}
	require.Equal(t, []string{
		"prune-broken-links", "make-dirs", "make-links", "make-copies",
		"render-templates", "install-packages", "run-scripts",
	}, names)
}

func TestResolvePackagesFilePrecedence(t *testing.T) {
	env := map[string]string{"XDG_CONFIG_HOME": "/xdg"}
	require.Equal(t, "/xdg/packages/packages.yml", resolvePackagesFile(env, "/home/u", options.Options{}))
	require.Equal(t, "/home/u/.config/packages/packages.yml", resolvePackagesFile(map[string]string{}, "/home/u", options.Options{}))
	require.Equal(t, "/x.yml", resolvePackagesFile(env, "/home/u", options.Options{PackagesFile: "/x.yml"}))
}

// [<] 🤖🤖
