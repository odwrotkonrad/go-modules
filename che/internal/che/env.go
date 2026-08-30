package che

// [>] 🤖🤖

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec/envinterp"
)

func (c Context) withRepoFiles(root string) (Context, error) {
	files, err := spec.LoadRepoFiles(root, root, false, c.Env)
	if err != nil {
		return c, err
	}
	c.ProcessEnv = c.Env
	c.RepoFiles = files
	c.Env = overlayEnv(files.Env, c.Env)
	return c, nil
}

func (c Context) processEnv() map[string]string {
	if c.ProcessEnv != nil {
		return c.ProcessEnv
	}
	return c.Env
}

// overlay is what an invoking spec hands the spec it embeds: its env lookup, the entry's env and
// explicit variable pass, and the variables whose scope reaches the embedded spec.
type overlay struct {
	inherited     map[string]string
	env           map[string]string
	passed        map[string]string
	inheritedVars spec.VarSet
}

type repoFilesCache struct {
	process map[string]string
	byDir   map[string]spec.RepoFiles
}

func newRepoFilesCache(ctx Context) *repoFilesCache {
	cache := &repoFilesCache{process: ctx.processEnv(), byDir: map[string]spec.RepoFiles{}}
	if ctx.RepoFiles.Root != "" {
		cache.byDir[ctx.RepoFiles.Root+"#false"] = ctx.RepoFiles
	}
	return cache
}

func (c *repoFilesCache) rootOf(dir string) (string, error) {
	root, err := fsutil.ResolveRepoRoot(dir)
	if err != nil {
		if !fsutil.IsNoRepo(err) {
			return "", err
		}
		root = dir
	}
	return root, nil
}

func (c *repoFilesCache) forSpec(dir string, export bool) (spec.RepoFiles, error) {
	root, err := c.rootOf(dir)
	if err != nil {
		return spec.RepoFiles{}, err
	}
	key := fmt.Sprintf("%s#%t", dir, export)
	if files, ok := c.byDir[key]; ok {
		return files, nil
	}
	files, err := spec.LoadRepoFiles(dir, root, export, c.process)
	if err != nil {
		return spec.RepoFiles{}, err
	}
	c.byDir[key] = files
	return files, nil
}

// interp builds the lookup context of the spec at paths (one file, or the invoked pair) under dir:
// env layers, then its variables resolved against its own definitions.
func (c *repoFilesCache) interp(paths []string, dir string, root bool, over overlay, policy envinterp.Policy) (spec.Interp, error) {
	files, err := c.forSpec(dir, spec.IsExportSpec(paths[0]))
	if err != nil {
		return spec.Interp{}, err
	}
	repoRoot, err := c.rootOf(dir)
	if err != nil {
		return spec.Interp{}, err
	}
	in := spec.Interp{Policy: policy, Builtins: map[string]string{"repoRoot": repoRoot}}
	if root {
		in.Below = []spec.EnvLayer{{Name: spec.EnvSources.CheEnv, Env: files.Env}, {Name: spec.EnvSources.Process, Env: c.process}}
		in.Above = []spec.EnvLayer{{Name: spec.EnvSources.Ref, Env: over.env}}
	} else {
		in.Below = []spec.EnvLayer{{Name: spec.EnvSources.IncludedCheEnv, Env: files.Env}}
		in.Above = []spec.EnvLayer{{Name: spec.EnvSources.Inherited, Env: over.inherited}, {Name: spec.EnvSources.Ref, Env: over.env}}
	}
	defs, err := spec.PeekVarDefs(paths)
	if err != nil {
		return spec.Interp{}, err
	}
	env := map[string]string{}
	for _, layer := range slices.Concat(in.Below, in.Above) {
		maps.Copy(env, layer.Env)
	}
	if in.Vars, err = spec.ResolveVars(defs, files, env, over.passed, over.inheritedVars); err != nil {
		return spec.Interp{}, fmt.Errorf("%s: %w", strings.Join(paths, " + "), err)
	}
	return in, nil
}

func exportEnv(env map[string]string) func() {
	type prior struct {
		value string
		set   bool
	}
	priors := make(map[string]prior, len(env))
	for k, v := range env {
		old, set := os.LookupEnv(k)
		priors[k] = prior{old, set}
		os.Setenv(k, v)
	}
	return func() {
		for k, p := range priors {
			if p.set {
				os.Setenv(k, p.value)
				continue
			}
			os.Unsetenv(k)
		}
	}
}

func specPathOf(root string) string {
	paths := spec.RootSpecPaths(root)
	if len(paths) == 0 {
		return filepath.Join(root, spec.SpecFileName)
	}
	return strings.Join(paths, " + ")
}

// [<] 🤖🤖
