package cli

// [>] 🤖🤖

import "github.com/spf13/cobra"

var RenderCmd = &cobra.Command{
	Use:   "render-templates",
	Short: "render *.tpl sources; each dest path decides target (relative -> repo, ~/ or absolute -> host)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return theHost.RenderTemplates(resolved.Templates)
	},
}

// [<] 🤖🤖
