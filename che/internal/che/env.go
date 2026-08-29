package che

// [>] 🤖🤖

import (
	"os"
	"path/filepath"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec/envinterp"
)

func (c Context) withRepoFiles(root string) (Context, error) {
	files, err := spec.LoadRepoFiles(root, c.Env)
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

type overlay struct {
	inherited map[string]string
	env       map[string]string
	vars      map[string]string
}

type repoFilesCache struct {
	process map[string]string
	byRoot  map[string]spec.RepoFiles
}

func newRepoFilesCache(ctx Context) *repoFilesCache {
	cache := &repoFilesCache{process: ctx.processEnv(), byRoot: map[string]spec.RepoFiles{}}
	if ctx.RepoFiles.Root != "" {
		cache.byRoot[ctx.RepoFiles.Root] = ctx.RepoFiles
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

func (c *repoFilesCache) forDir(dir string) (spec.RepoFiles, error) {
	root, err := c.rootOf(dir)
	if err != nil {
		return spec.RepoFiles{}, err
	}
	if files, ok := c.byRoot[root]; ok {
		return files, nil
	}
	files, err := spec.LoadRepoFiles(root, c.process)
	if err != nil {
		return spec.RepoFiles{}, err
	}
	c.byRoot[root] = files
	return files, nil
}

func (c *repoFilesCache) interp(dir string, root bool, over overlay, policy envinterp.Policy) (spec.Interp, error) {
	files, err := c.forDir(dir)
	if err != nil {
		return spec.Interp{}, err
	}
	in := spec.Interp{Policy: policy, VarsBase: files.Variables, VarsOver: over.vars}
	if root {
		in.Below = []spec.EnvLayer{{Name: spec.EnvSources.CheEnv, Env: files.Env}, {Name: spec.EnvSources.Process, Env: c.process}}
		in.Above = []spec.EnvLayer{{Name: spec.EnvSources.Ref, Env: over.env}}
		return in, nil
	}
	in.Below = []spec.EnvLayer{{Name: spec.EnvSources.IncludedCheEnv, Env: files.Env}}
	in.Above = []spec.EnvLayer{{Name: spec.EnvSources.Inherited, Env: over.inherited}, {Name: spec.EnvSources.Ref, Env: over.env}}
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
	path, ok := spec.CheFile(root, spec.SpecFileName)
	if !ok {
		return filepath.Join(root, spec.SpecFileName)
	}
	return path
}

// [<] 🤖🤖
