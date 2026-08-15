package che

// [>] 🤖🤖

import (
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
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

func TestResolvePackagesFile(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/resolve_packages_file.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		var env map[string]string
		c.Input.Args.To(t, 0, &env)
		opts := options.Options{PackagesFile: c.Input.Args.String(t, 2)}
		return resolvePackagesFile(env, c.Input.Args.String(t, 1), opts), nil
	})
}

// [<] 🤖🤖
