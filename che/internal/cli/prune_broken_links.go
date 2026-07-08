package cli

// [>] 🤖🤖

import "github.com/spf13/cobra"

var PruneCmd = &cobra.Command{
	Use:   "prune-links",
	Short: "delete broken symlinks",
	RunE: func(cmd *cobra.Command, args []string) error {
		return forEachUnit(func(u unit) error { return u.host.PruneBrokenLinks(u.res.Dirs) })
	},
}

// [<] 🤖🤖
