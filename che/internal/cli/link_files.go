package cli

// [>] 🤖🤖

import "github.com/spf13/cobra"

var LinkCmd = &cobra.Command{
	Use:   "link",
	Short: "symlink op (configs into system root)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return forEachUnit(cmd.Name(), func(u unit) error { return u.host.MkLinks(u.res.Links, u.res.Dirs) })
	},
}

// [<] 🤖🤖
