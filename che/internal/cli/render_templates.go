package cli

// [>] 🤖🤖

import "github.com/spf13/cobra"

var (
	renderHost bool
	renderRepo bool
)

var RenderCmd = &cobra.Command{
	Use:   "render-templates",
	Short: "render *.host.tpl (host) and *.repo.tpl (repo); neither flag => both",
	RunE: func(cmd *cobra.Command, args []string) error {
		host, repo := renderHost, renderRepo
		if !host && !repo { // neither flag => both
			host, repo = true, true
		}
		if host {
			if err := theHost.RenderTemplates(resolved.Templates); err != nil {
				return err
			}
		}
		if repo {
			if err := theHost.RenderRepoTemplates(resolved.RepoTemplates); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	RenderCmd.Flags().BoolVar(&renderHost, "host", false, "render only *.host.tpl onto the host")
	RenderCmd.Flags().BoolVar(&renderRepo, "repo", false, "render only *.repo.tpl onto the repo")
}

// [<] 🤖🤖
