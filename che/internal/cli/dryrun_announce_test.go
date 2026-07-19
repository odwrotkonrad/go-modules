package cli

// [>] 🤖🤖

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/che"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// TestDryRunAnnounce: dry run announces itself once, as the whole output's
// first line, mode-specific desc, no other line carrying a dry-run marker;
// --dry-run=true aliases delta (spec/che/ExecutionBehavior.md). Cases in
// dry_run_announce.test.spec.yml: expected.output pins the resolved mode,
// stdOut the announce line, stdOutCounts its once-ness.
func TestDryRunAnnounce(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/cmds/dry_run_announce.test.spec.yml",
		func(t *testing.T, c testyml.Case[string]) {
			a, root, _ := repoEnv(t, c.Context.Pwd)
			for k, v := range c.Context.Env {
				t.Setenv(k, v)
			}
			a.flags.Profiles = []string{testutil.CheProfile}
			m := testutil.ApplyMocks(t, c.Context.MockedInterfaces)
			realSeams := che.NewSeams
			testyml.Swap(t, &che.NewSeams, func(home string) che.Seams {
				s := realSeams(home)
				s.FS = m.FS
				s.Reader = &testutil.FileSystemMockReader{Roots: []string{a.flags.CheWorkingDirectory, home}}
				s.Ledger = nil
				return s
			})
			out, err := testutil.CaptureStdout(t, func() error {
				if err := a.init("run"); err != nil {
					return err
				}
				cmd, rest := findCmd(t, root, []string{"run"})
				return cmd.RunE(cmd, rest)
			})
			require.NoError(t, err)
			assert.Equal(t, options.DryRunMode(c.Expected.Output), a.opts.DryRun, "resolved mode")
			stripped := testutil.StripANSI(out)
			lines := strings.Split(stripped, "\n")
			require.NotEmpty(t, lines)
			assert.Equal(t, strings.TrimSuffix(testyml.Expand(c.Expected.StdOut[0], nil), "\n"), lines[0],
				"announce opens the whole output")
			for sub, n := range c.Expected.StdOutCounts {
				testyml.MustCount(t, stripped, sub, n)
			}
			for _, sub := range c.NotExpected.StdOut {
				testyml.MustNotMatch(t, stripped, sub)
			}
		})
}

// [<] 🤖🤖
