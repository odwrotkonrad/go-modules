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

func findCmd(t *testing.T, name string) *cobra.Command {
	t.Helper()
	fields := strings.Fields(name)
	if fields[0] == "services" {
		sub, _, err := ServicesCmd.Find(fields[1:])
		if err != nil || sub == ServicesCmd {
			t.Fatalf("services subcommand %q not found: %v", name, err)
		}
		return sub
	}
	byName := map[string]*cobra.Command{
		"link":             LinkCmd,
		"copy":             CopyCmd,
		"render-templates": RenderCmd,
		"mk-dirs":          DirsCmd,
		"prune-links":      PruneCmd,
		"run-scripts":      RunScriptsCmd,
		"detect":           DetectCmd,
	}
	cmd, ok := byName[fields[0]]
	if !ok {
		t.Fatalf("unknown command %q", name)
	}
	return cmd
}

func TestCommands(t *testing.T) {
	type in struct {
		Cmd     string
		Args    []string
		Profile string
	}
	type want struct {
		testyml.Want `yaml:",inline"`
		DryRunLines  *bool `yaml:"dryRunLines"`
	}
	type c struct {
		Name    string
		In      in
		Want    want
		NotWant testyml.Want `yaml:"notWant"`
	}
	testyml.Run(t, td, "testdata/spec/commands.spec.yml", func(t *testing.T, c c) {
		profile := c.In.Profile
		if profile == "" {
			profile = testutil.CheProfile
		}
		home := setupDryRun(t, profile)
		vars := map[string]string{
			"HOME":    home,
			"REPO":    units[0].host.RepoRoot,
			"ROOT":    units[0].host.Root,
			"PROFILE": profile,
		}
		cmd := findCmd(t, c.In.Cmd)
		out, err := testutil.CaptureStdout(t, func() error { return cmd.RunE(cmd, c.In.Args) })
		if c.Want.WantsError() {
			c.Want.CheckErr(t, err)
			return
		}
		if err != nil {
			t.Fatalf("%s errored: %v", c.In.Cmd, err)
		}
		if c.Want.DryRunLines == nil || *c.Want.DryRunLines {
			for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
				if line != "" && !strings.Contains(line, "dry-run=delta") {
					t.Errorf("non-dry-run line: %q\n--- got ---\n%s", line, out)
				}
			}
		}
		stripped := testutil.StripANSI(out)
		for _, f := range c.Want.StdOut {
			testyml.MustMatch(t, stripped, testyml.Expand(f, vars))
		}
		for _, f := range c.NotWant.StdOut {
			testyml.MustNotMatch(t, stripped, testyml.Expand(f, vars))
		}
	})
}

// [<] 🤖🤖
