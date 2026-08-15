package che

// [>] 🤖🤖

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
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

func writeCachedDefinitions(t *testing.T, cacheHome, version, packagesYML string) {
	t.Helper()
	dir := filepath.Join(cacheHome, "packages", version)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "packages.yml"), []byte(packagesYML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(cacheHome, "packages", "current"), []byte(version+"\n"), 0o644))
}

func TestLoadPackagesFilePrefersCachedDefinitionsOverBuiltin(t *testing.T) {
	home, cacheHome := t.TempDir(), t.TempDir()
	env := map[string]string{"XDG_CONFIG_HOME": filepath.Join(home, ".config"), "CHE_CACHE_HOME": cacheHome}
	writeCachedDefinitions(t, cacheHome, "0.1.0", "packages:\n  cached-only: [brew]\n")

	f, path, err := loadPackagesFile(env, home, options.Options{})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cacheHome, "packages", "0.1.0", "packages.yml"), path)
	require.Contains(t, f.Packages, "cached-only")
}

func TestLoadPackagesFileUserFileBeatsCachedDefinitions(t *testing.T) {
	home, cacheHome := t.TempDir(), t.TempDir()
	cfg := filepath.Join(home, ".config")
	env := map[string]string{"XDG_CONFIG_HOME": cfg, "CHE_CACHE_HOME": cacheHome}
	writeCachedDefinitions(t, cacheHome, "0.1.0", "packages:\n  cached-only: [brew]\n")
	require.NoError(t, os.MkdirAll(filepath.Join(cfg, "packages"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cfg, "packages", "packages.yml"), []byte("packages:\n  user-only: [brew]\n"), 0o644))

	f, path, err := loadPackagesFile(env, home, options.Options{})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cfg, "packages", "packages.yml"), path)
	require.Contains(t, f.Packages, "user-only")
}

func TestLoadPackagesFileFallsBackToBuiltinWithoutCache(t *testing.T) {
	home, cacheHome := t.TempDir(), t.TempDir()
	env := map[string]string{"XDG_CONFIG_HOME": filepath.Join(home, ".config"), "CHE_CACHE_HOME": cacheHome}

	_, path, err := loadPackagesFile(env, home, options.Options{})
	require.NoError(t, err)
	require.Equal(t, packages.BuiltinPath, path)
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
