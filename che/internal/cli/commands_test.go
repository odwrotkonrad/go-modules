package cli

// [>] 🤖🤖

import (
	"embed"
	"io/fs"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func findCmd(t *testing.T, root *cobra.Command, args []string) (*cobra.Command, []string) {
	t.Helper()
	cmd, rest, err := root.Find(args)
	if err != nil || cmd == root || cmd.RunE == nil {
		t.Fatalf("command %v not found: %v", args, err)
	}
	return cmd, rest
}

// splitProfileArg strips a "--profile <name>" pair from args, returning the
// remaining args plus the profile (default testutil.CheProfile). The harness
// invokes RunE directly (no cobra flag parsing), so the flag maps onto
// profileForce via setupMock.
func splitProfileArg(args []string) ([]string, string) {
	for i, a := range args {
		if a == "--profile" && i+1 < len(args) {
			out := append(append([]string{}, args[:i]...), args[i+2:]...)
			return out, args[i+1]
		}
	}
	return args, testutil.CheProfile
}

// TestCommands: dry-run off, safe-double set, log lines assert the behavior.
func TestCommands(t *testing.T) {
	specs, err := fs.Glob(td, "testdata/spec/cmds/che-*.test.spec.yml")
	require.NoError(t, err)
	require.NotEmpty(t, specs)
	run := func(t *testing.T, c testyml.Case[struct{}]) {
		args, profile := splitProfileArg(c.Context.CommandArgs())
		a, root, home := setupMock(t, c.Context.Pwd, profile, c.Context.MockedInterfaces)
		vars := map[string]string{
			"HOME": home,
			"REPO": a.units[0].host.RepoRoot,
			"ROOT": a.units[0].host.Root,
		}
		cmd, rest := findCmd(t, root, args)
		out, runErr := testutil.CaptureStdout(t, func() error { return cmd.RunE(cmd, rest) })
		c.Expected.Check(t, runErr)
		stripped := testutil.StripStamps(testutil.StripANSI(out))
		for _, f := range c.Expected.StdOut {
			testyml.MustMatch(t, stripped, testyml.Expand(f, vars))
		}
		for _, f := range c.NotExpected.StdOut {
			testyml.MustNotMatch(t, stripped, testyml.Expand(f, vars))
		}
	}
	for _, spec := range specs {
		testyml.Run(t, td, spec, run)
	}
}

// [<] 🤖🤖
