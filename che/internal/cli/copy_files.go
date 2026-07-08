package cli

// [>] 🤖🤖

import "github.com/spf13/cobra"

var CopyCmd = &cobra.Command{
	Use:   "copy",
	Short: "*.ontoHost.cp copy op",
	RunE: func(cmd *cobra.Command, args []string) error {
		return theHost.MkCopies(resolved.Copies, resolved.Dirs)
	},
}

// [<] 🤖🤖
