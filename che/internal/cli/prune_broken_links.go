package cli

// [>] 🤖🤖

import "github.com/spf13/cobra"

var PruneCmd = &cobra.Command{
	Use:   "prune-links",
	Short: "delete broken symlinks",
	RunE: func(cmd *cobra.Command, args []string) error {
		return theHost.PruneBrokenLinks(resolved.Dirs)
	},
}

// [<] 🤖🤖
