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
// --dry-run=true aliases delta (spec/che/ExecutionBehavior.md).
func TestDryRunAnnounce(t *testing.T) {
	cases := []struct {
		mode, wantLine string
		wantMode       options.DryRunMode
	}{
		{"delta", "dry-run(config.dryRun=delta): no actual operations will be performed, reporting only dests that would change", options.DryRun.Delta},
		{"true", "dry-run(config.dryRun=delta): no actual operations will be performed, reporting only dests that would change", options.DryRun.Delta},
		{"all", "dry-run(config.dryRun=all): no actual operations will be performed, reporting every dest's state", options.DryRun.All},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			a, root, _ := repoEnv(t, "testdata/fixture/commands/all/sample-tree-only-links")
			t.Setenv("CHE_DRY_RUN", tc.mode)
			a.flags.Profiles = []string{testutil.CheProfile}
			m := testutil.ApplyMocks(t, map[string]string{
				"execx.CmdExecutor":       "testutil.CmdMockExecutor",
				"fsutil.FileSystemWriter": "testutil.FileSystemMockWriter",
				"fsutil.FileSystemReader": "testutil.FileSystemMockReader",
			})
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
			assert.Equal(t, tc.wantMode, a.opts.DryRun, "resolved mode")
			stripped := testutil.StripANSI(out)
			lines := strings.Split(stripped, "\n")
			require.NotEmpty(t, lines)
			assert.Equal(t, tc.wantLine, lines[0], "announce opens the whole output")
			assert.Equal(t, 1, strings.Count(stripped, "dry-run("), "announced exactly once")
			assert.NotContains(t, stripped, ",dry-run=", "no per-line dry-run marker")
			assert.NotContains(t, stripped, "(dry-run=", "no per-line dry-run marker")
		})
	}
}

// [<] 🤖🤖
