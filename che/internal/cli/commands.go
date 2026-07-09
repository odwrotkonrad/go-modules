package cli

// [>] 🤖🤖

import "github.com/spf13/cobra"

var (
	LinkCmd = unitCmd("link", "symlink op (configs into system root)",
		func(u unit) error { return u.host.MkLinks(u.res.Links, u.res.Dirs) })
	CopyCmd = unitCmd("copy", "*.ontoHost.cp copy op",
		func(u unit) error { return u.host.MkCopies(u.res.Copies, u.res.Dirs) })
	RenderCmd = unitCmd("render-templates", "render *.tpl sources; each dest path decides target (relative -> repo, ~/ or absolute -> host)",
		func(u unit) error { return u.host.RenderTemplates(u.res.Templates) })
	DirsCmd = unitCmd("mk-dirs", "create repo-tree dirs + extra-dirs",
		func(u unit) error { return u.host.MkDirs(u.res.Dirs, u.res.ExtraDirs) })
	PruneCmd = unitCmd("prune-links", "delete broken symlinks",
		func(u unit) error { return u.host.PruneBrokenLinks(u.res.Dirs) })
)

func unitCmd(use, short string, op func(unit) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return forEachUnit(cmd.Name(), op)
		},
	}
}

// [<] 🤖🤖
