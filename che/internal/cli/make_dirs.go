package cli

// [>] 🤖🤖

import "github.com/spf13/cobra"

var DirsCmd = &cobra.Command{
	Use:   "mk-dirs",
	Short: "create repo-tree dirs + extra-dirs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return theHost.MkDirs(resolved.Dirs, resolved.ExtraDirs)
	},
}

// [<] 🤖🤖
