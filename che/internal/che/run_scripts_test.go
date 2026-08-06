package che

// [>] 🤖🤖

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

func setupScripts(t *testing.T, cfg options.Options) (*ProfileReady, []string) {
	t.Helper()
	p, _, dir := setupProfile(t, cfg)
	scripts := make([]string, 2)
	for i, name := range []string{"first", "second"} {
		s := filepath.Join(dir, "install", name)
		require.NoError(t, os.WriteFile(s, []byte("#!/bin/sh\n"), 0o755))
		scripts[i] = s
	}
	m := testutil.NewCmdMockExecutor()
	m.FailCmds = []string{"install/first"}
	execx.Swap(t, m)
	return p, scripts
}

func TestRunScriptsErrexitStopsAtFirstFailure(t *testing.T) {
	p, scripts := setupScripts(t, options.Options{Errexit: true})
	out, err := testutil.CaptureStdout(t, func() error { return p.runScripts(scripts) })
	require.ErrorIs(t, err, ErrErrexit)
	require.ErrorContains(t, err, "install/first")
	testutil.WantLines(t, out, "failed "+scripts[0])
	testutil.NotLine(t, out, scripts[1])
}

func TestRunScriptsDefaultContinuesAfterFailure(t *testing.T) {
	p, scripts := setupScripts(t, options.Options{})
	out, err := testutil.CaptureStdout(t, func() error { return p.runScripts(scripts) })
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrErrexit))
	testutil.WantLines(t, out, "failed "+scripts[0], "ran "+scripts[1])
}

func TestExecEachErrexitStopsRemainingProfiles(t *testing.T) {
	repo := testutil.Repo(t, map[string]string{
		"che.yml": "p:\n  options: {autoDiscover: true}\n  include:\n    runScripts: [scripts/fail]\n" +
			"q:\n  options: {autoDiscover: true}\n  include:\n    runScripts: [scripts/ok]\n",
		"scripts/fail": "#!/bin/sh\n",
		"scripts/ok":   "#!/bin/sh\n",
	})
	_, baseEnv := prepEnv(t)
	root, err := PrepareSpecs(newContext(baseEnv, repo),
		options.Options{SkipRunIf: true, AutoDiscover: true, Errexit: true}, spec.SpecSourceRecipe{})
	require.NoError(t, err)
	m := testutil.NewCmdMockExecutor()
	m.FailCmds = []string{"scripts/fail"}
	execx.Swap(t, m)
	out, err := testutil.CaptureStdout(t, func() error {
		return root.ExecEach(context.Background(), "run-scripts", func(ctx context.Context, p *ProfileReady) error {
			_, e := p.ExecRunScripts(ctx, nil)
			return e
		})
	})
	require.ErrorIs(t, err, ErrErrexit)
	testutil.WantLines(t, out, "scripts/fail")
	testutil.NotLine(t, out, "ran "+filepath.Join(repo, "scripts/ok"))
}

// [<] 🤖🤖
