package cli

// [>] 🤖🤖

import (
	"embed"
	"strings"
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

// joinLines renders recorded calls one per line, newline-terminated like
// stdout, so block matchers assert order across both.
func joinLines(calls []string) string {
	if len(calls) == 0 {
		return ""
	}
	return strings.Join(calls, "\n") + "\n"
}

// TestCommands runs each subcommand with dry-run off against a record-only
// MockFS and a mock command executor: fs mutations, executed commands and log
// lines assert without touching the host.
func TestCommands(t *testing.T) {
	type context struct {
		Directory string
	}
	type in struct {
		Args    []string
		Profile string
	}
	type want struct {
		testyml.Want `yaml:",inline"`
		FsCalls      testyml.Matchers `yaml:"fsCalls"`
		Cmds         testyml.Matchers `yaml:"cmds"`
	}
	type c struct {
		Name    string
		Context context
		In      in
		Want    want
		NotWant want `yaml:"notWant"`
	}
	testyml.Run(t, td, "testdata/spec/commands.spec.yml", func(t *testing.T, c c) {
		profile := c.In.Profile
		if profile == "" {
			profile = testutil.CheProfile
		}
		home, mock, exe := setupMock(t, c.Context.Directory, profile)
		vars := map[string]string{
			"HOME":    home,
			"REPO":    units[0].host.RepoRoot,
			"ROOT":    units[0].host.Root,
			"PROFILE": profile,
		}
		cmd, rest := findCmd(t, c.In.Args)
		out, err := testutil.CaptureStdout(t, func() error { return cmd.RunE(cmd, rest) })
		if c.Want.IsErrorWanted() {
			c.Want.CheckErr(t, err)
		} else if err != nil {
			t.Fatalf("%v errored: %v\n%s", c.In.Args, err, out)
		}
		got := map[string]string{
			"stdOut":  testutil.StripStamps(testutil.StripANSI(out)),
			"fsCalls": joinLines(mock.Calls()),
			"cmds":    joinLines(exe.Calls()),
		}
		for name, ms := range map[string][2]testyml.Matchers{
			"stdOut":  {c.Want.StdOut, c.NotWant.StdOut},
			"fsCalls": {c.Want.FsCalls, c.NotWant.FsCalls},
			"cmds":    {c.Want.Cmds, c.NotWant.Cmds},
		} {
			for _, f := range ms[0] {
				testyml.MustMatch(t, got[name], testyml.Expand(f, vars))
			}
			for _, f := range ms[1] {
				testyml.MustNotMatch(t, got[name], testyml.Expand(f, vars))
			}
		}
	})
}

// [<] 🤖🤖
