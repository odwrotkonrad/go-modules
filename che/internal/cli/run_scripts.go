package cli

// [>] 🤖🤖

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var RunScriptsCmd = &cobra.Command{
	Use:   "run-scripts [name...]",
	Short: "run the profile's scripts, optionally filtered by name substring",
	RunE: func(cmd *cobra.Command, args []string) error {
		scripts, err := theHost.ResolveScripts(resolved.Scripts)
		if err != nil {
			return err
		}
		scripts = filterScripts(scripts, args)
		if len(args) > 0 && len(scripts) == 0 {
			return fmt.Errorf("no script matches: %v", args)
		}
		return theHost.RunScripts(scripts)
	},
}

func filterScripts(scripts, names []string) []string {
	if len(names) == 0 {
		return scripts
	}
	var out []string
	for _, s := range scripts {
		for _, n := range names {
			if strings.Contains(s, n) {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// [<] 🤖🤖
