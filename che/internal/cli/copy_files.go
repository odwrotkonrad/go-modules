package cli

// [>] 🤖🤖

import "github.com/spf13/cobra"

var CopyCmd = &cobra.Command{
	Use:   "copy",
	Short: "*.ontoHost.cp copy op",
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, u := range units {
			if err := u.withEnv(func() error { return u.host.MkCopies(u.res.Copies, u.res.Dirs) }); err != nil {
				return err
			}
		}
		return nil
	},
}

// [<] 🤖🤖
