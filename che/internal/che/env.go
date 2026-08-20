package che

// [>] 🤖🤖

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/joho/godotenv"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec/envinterp"
)

func (c Context) withDotEnv(path string) (Context, error) {
	dotEnv, err := godotenv.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("read %s: %w", path, err)
	}
	c.ProcessEnv = c.Env
	c.DotEnv = dotEnv
	c.Env = overlayEnv(dotEnv, c.Env)
	return c, nil
}

func (c Context) interp(refEnv map[string]string, policy envinterp.Policy) spec.Interp {
	process := c.ProcessEnv
	if process == nil {
		process = c.Env
	}
	return spec.Interp{Process: process, DotEnv: c.DotEnv, Ref: refEnv, Policy: policy}
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

// [<] 🤖🤖
