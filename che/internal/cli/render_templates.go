package cli

// [>] 🤖🤖

import "github.com/spf13/cobra"

var RenderCmd = &cobra.Command{
	Use:   "render-templates",
	Short: "render *.tpl sources; each dest path decides target (relative -> repo, ~/ or absolute -> host)",
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, u := range units {
			if err := u.withEnv(func() error { return u.host.RenderTemplates(u.res.Templates) }); err != nil {
				return err
			}
		}
		return nil
	},
}

// [<] 🤖🤖
