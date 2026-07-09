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

func findCmd(t *testing.T, args []string) (*cobra.Command, []string) {
	t.Helper()
	if args[0] == "services" {
		sub, rest, err := ServicesCmd.Find(args[1:])
		if err != nil || sub == ServicesCmd {
			t.Fatalf("services subcommand %v not found: %v", args, err)
		}
		return sub, rest
	}
	byName := map[string]*cobra.Command{
		"all":              AllCmd,
		"link":             LinkCmd,
		"copy":             CopyCmd,
		"render-templates": RenderCmd,
		"mk-dirs":          DirsCmd,
		"prune-links":      PruneCmd,
		"run-scripts":      RunScriptsCmd,
		"detect":           DetectCmd,
	}
	cmd, ok := byName[args[0]]
	if !ok {
		t.Fatalf("unknown command %v", args)
	}
	return cmd, args[1:]
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
		home, _, _ := setupMock(t, c.Context.Directory, profile)
		vars := map[string]string{
			"HOME": home,
			"REPO": units[0].host.RepoRoot,
			"ROOT": units[0].host.Root,
		}
		cmd, rest := findCmd(t, args)
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
