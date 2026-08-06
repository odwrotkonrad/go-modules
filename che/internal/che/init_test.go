package che

// [>] 🤖🤖

import (
	"maps"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// initFlags decodes the case's flags arg.
type initFlags struct {
	SkipRemoteRefs bool `yaml:"skipRemoteRefs"`
	// PrepareAfter runs PrepareSpecs after init and asserts it fetches nothing
	// (discovery reuses init's checkouts).
	PrepareAfter bool `yaml:"prepareAfter"`
}

// TestInitSources: the init stage prefetches every reachable remote spec
// source — conditions unevaluated, ref fetches survive failures, skipRemoteRefs
// honored, discovery reusing the checkouts.
func TestInitSources(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/init_sources.test.spec.yml",
		func(t *testing.T, c testyml.Case[struct{}]) {
			var refFiles, hostFiles map[string]string
			var flags initFlags
			c.Input.Args.To(t, 0, &refFiles)
			c.Input.Args.To(t, 1, &hostFiles)
			c.Input.Args.To(t, 2, &flags)

			vars := map[string]string{}
			if len(refFiles) > 0 {
				vars["REF_DIR"] = testutil.Repo(t, refFiles)
			}
			hostTree := map[string]string{}
			for rel, body := range hostFiles {
				hostTree[rel] = testyml.Expand(body, vars)
			}
			hostRepo := testutil.Repo(t, hostTree)
			home, baseEnv := prepEnv(t)
			vars["HOME"] = home

			env := map[string]string{}
			maps.Copy(env, baseEnv)
			for k, v := range c.Context.Env {
				env[k] = v
			}
			t.Chdir(hostRepo)
			for k, v := range env {
				t.Setenv(k, v)
			}
			t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
			ctx := newContext(env, hostRepo)

			prev := log.GetLevel()
			log.SetLevel(log.Levels.Trace)
			t.Cleanup(func() { log.SetLevel(prev) })
			out, err := testutil.CaptureStdout(t, func() error {
				return InitSources(ctx, options.Options{SkipRemoteRefs: flags.SkipRemoteRefs})
			})
			if c.Expected.Check(t, err) {
				return
			}
			stripped := testutil.StripANSI(out)
			for _, m := range c.Expected.StdOut {
				testyml.MustMatch(t, stripped, testyml.Expand(m, vars))
			}
			for _, m := range c.NotExpected.StdOut {
				testyml.MustNotMatch(t, stripped, testyml.Expand(m, vars))
			}
			if flags.PrepareAfter {
				prepOut, err := testutil.CaptureStdout(t, func() error {
					_, e := PrepareSpecs(ctx, options.Options{AutoDiscover: true}, spec.SpecSourceRecipe{})
					return e
				})
				require.NoError(t, err)
				prepOut = testutil.StripANSI(prepOut)
				testyml.MustNotMatch(t, prepOut, ": clone ")
				testyml.MustNotMatch(t, prepOut, ": pull ")
			}
		})
}

// [<] 🤖🤖
