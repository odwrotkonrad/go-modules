package cli

// [>] 🤖🤖

import "github.com/spf13/cobra"

var PruneCmd = &cobra.Command{
	Use:   "prune-links",
	Short: "delete broken symlinks",
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, u := range units {
			if err := u.withEnv(func() error { return u.host.PruneBrokenLinks(u.res.Dirs) }); err != nil {
				return err
			}
		}
		return nil
	},
}

// [<] 🤖🤖
