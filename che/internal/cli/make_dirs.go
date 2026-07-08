package cli

// [>] 🤖🤖

import "github.com/spf13/cobra"

var DirsCmd = &cobra.Command{
	Use:   "mk-dirs",
	Short: "create repo-tree dirs + extra-dirs",
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, u := range units {
			if err := u.withEnv(func() error { return u.host.MkDirs(u.res.Dirs, u.res.ExtraDirs) }); err != nil {
				return err
			}
		}
		return nil
	},
}

// [<] 🤖🤖
