package cli

// [>] 🤖🤖

import (
	"embed"
	"io/fs"
	"testing"

	"github.com/spf13/cobra"

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

// TestCommands runs each subcommand with dry-run off against a record-only
// MockFS and a mock command executor: log lines assert the behavior, nothing
// touches the host.
func TestCommands(t *testing.T) {
	type context struct {
		Directory string
	}
	type in struct {
		Args []string
	}
	type c struct {
		Name    string
		Context context
		In      in
		Want    testyml.Want
		NotWant testyml.Want `yaml:"notWant"`
	}
	specs, err := fs.Glob(td, "testdata/spec/*.spec.yml")
	if err != nil || len(specs) == 0 {
		t.Fatalf("glob spec files: %v (%d found)", err, len(specs))
	}
	run := func(t *testing.T, c c) {
		args, profile := splitProfileArg(c.In.Args)
		a, root, home := setupMock(t, c.Context.Directory, profile)
		vars := map[string]string{
			"HOME": home,
			"REPO": a.units[0].host.RepoRoot,
			"ROOT": a.units[0].host.Root,
		}
		cmd, rest := findCmd(t, root, args)
		out, err := testutil.CaptureStdout(t, func() error { return cmd.RunE(cmd, rest) })
		if c.Want.IsErrorWanted() {
			c.Want.CheckErr(t, err)
		} else if err != nil {
			t.Fatalf("%v errored: %v\n%s", c.In.Args, err, out)
		}
		stripped := testutil.StripStamps(testutil.StripANSI(out))
		for _, f := range c.Want.StdOut {
			testyml.MustMatch(t, stripped, testyml.Expand(f, vars))
		}
		for _, f := range c.NotWant.StdOut {
			testyml.MustNotMatch(t, stripped, testyml.Expand(f, vars))
		}
	}
	for _, spec := range specs {
		testyml.Run(t, td, spec, run)
	}
}

// [<] 🤖🤖
