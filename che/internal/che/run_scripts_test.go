package che

// [>] 🤖🤖

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

func setupScripts(t *testing.T, cfg options.Options, failCmds []string) (*ProfileReady, []string) {
	t.Helper()
	p, _, dir := setupProfile(t, cfg)
	scripts := make([]string, 2)
	for i, name := range []string{"first", "second"} {
		s := filepath.Join(dir, "install", name)
		require.NoError(t, os.WriteFile(s, []byte("#!/bin/sh\n"), 0o755))
		scripts[i] = s
	}
	m := testutil.NewCmdMockExecutor()
	m.FailCmds = failCmds
	execx.Swap(t, m)
	return p, scripts
}

func TestRunScripts(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/run_scripts.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		opts := options.Options{Errexit: c.Input.Args.Bool(t, 1)}
		failCmds := c.Input.Args.Strings(t, 2)
		vars := map[string]string{}
		var out string
		var err error
		if c.Input.Args.Bool(t, 0) {
			repo := testutil.Repo(t, map[string]string{
				"che.yml": "profilesDefinitions:\n  p:\n    options: {autoDiscover: true}\n    include:\n      runScripts: [scripts/fail]\n" +
					"  q:\n    options: {autoDiscover: true}\n    include:\n      runScripts: [scripts/ok]\n",
				"scripts/fail": "#!/bin/sh\n",
				"scripts/ok":   "#!/bin/sh\n",
			})
			vars["REPO"] = repo
			_, baseEnv := prepEnv(t)
			opts.SkipRunIf, opts.AutoDiscover = true, true
			root, prepErr := PrepareSpecs(newContext(baseEnv, repo), opts, spec.SpecSourceRecipe{})
			require.NoError(t, prepErr)
			m := testutil.NewCmdMockExecutor()
			m.FailCmds = failCmds
			execx.Swap(t, m)
			out, err = capturedExec(t, func() error {
				return root.ExecEach(context.Background(), "run-scripts", func(ctx context.Context, p *ProfileReady) error {
					_, e := p.ExecRunScripts(ctx, nil)
					return e
				})
			})
		} else {
			p, scripts := setupScripts(t, opts, failCmds)
			vars["SCRIPT_FIRST"], vars["SCRIPT_SECOND"] = scripts[0], scripts[1]
			out, err = capturedExec(t, func() error { return p.runScripts(scripts) })
		}
		c.Expected.Check(t, err)
		for _, m := range c.NotExpected.ErrorOutput {
			require.Error(t, err)
			if testyml.IsMatch(err.Error(), m) {
				t.Errorf("error %q unexpectedly matches %q", err.Error(), m)
			}
		}
		stripped := testutil.StripANSI(out)
		for _, m := range c.Expected.StdOut {
			testyml.MustMatch(t, stripped, testyml.Expand(m, vars))
		}
		for _, m := range c.NotExpected.StdOut {
			testyml.MustNotMatch(t, stripped, testyml.Expand(m, vars))
		}
	})
}

func capturedExec(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	var err error
	out, capErr := testutil.CaptureStdout(t, func() error {
		err = fn()
		return nil
	})
	require.NoError(t, capErr)
	return out, err
}

// [<] 🤖🤖
