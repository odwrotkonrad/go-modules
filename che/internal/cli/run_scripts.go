package cli

// [>] 🤖🤖

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// runScriptsRunE is the run-scripts RunE: the step op plus the name-substring
// arg filter and a no-match check across all repos.
func (ld *loader) runScriptsRunE(cmd *cobra.Command, args []string) error {
	total := 0
	err := ld.forEachLoad(cmd.Name(), func(l load) error {
		n, err := runScripts(l, args)
		total += n
		return err
	})
	if err != nil {
		return err
	}
	if len(args) > 0 && total == 0 {
		return fmt.Errorf("no script matches: %v", args)
	}
	return nil
}

// runScripts resolves and runs l's scripts filtered by names, returning how
// many matched.
func runScripts(l load, names []string) (int, error) {
	scripts, err := l.host.ResolveScripts(l.selection.Scripts)
	if err != nil {
		return 0, err
	}
	scripts = filterScripts(scripts, names)
	return len(scripts), l.host.RunScripts(scripts)
}

func filterScripts(scripts, names []string) []string {
	if len(names) == 0 {
		return scripts
	}
	var out []string
	for _, s := range scripts {
		if slices.ContainsFunc(names, func(n string) bool { return strings.Contains(s, n) }) {
			out = append(out, s)
		}
	}
	return out
}

// [<] 🤖🤖
