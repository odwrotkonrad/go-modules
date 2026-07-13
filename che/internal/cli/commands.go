package cli

// [>] 🤖🤖

import (
	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/internal/che"
)

// opCmd is one per-op subcommand shape: its cobra wiring plus the prepared
// operation name it runs over every profile.
type opCmd struct {
	name   string
	parent string
	short  string
}

// displayName is the op's prepared-operation and `all` log name:
// parent-qualified for subcommands.
func (o opCmd) displayName() string {
	if o.parent != "" {
		return o.parent + " " + o.name
	}
	return o.name
}

// ops lists every op subcommand in run order: the one source for subcommand
// construction (run order itself lives in che.prepareOperations).
func ops() []opCmd {
	return []opCmd{
		{name: "prune-links", short: "delete broken symlinks"},
		{name: "mk-dirs", short: "create repo-tree dirs + extra-dirs"},
		{name: "link", short: "symlink op (configs into system root)"},
		{name: "copy", short: "*.ontoHost.cp copy op"},
		{name: "render-templates", short: "render *.tpl sources; each dest path decides target (relative -> repo, ~/ or absolute -> host)"},
		{name: "run-scripts", short: "run the profile's scripts, optionally filtered by name substring"},
		{name: "bootout", parent: "services", short: "unload each service (bootout if loaded, wait until gone)"},
		{name: "bootin", parent: "services", short: "load each service (bootstrap from plist)"},
		{name: "ensure", parent: "services", short: "settle then verify each long-running service has a live pid"},
	}
}

// opCmd builds an op's subcommand: RunE runs its prepared operation over every
// profile. run-scripts and render-templates layer their arg filter / flag on top.
func (a *app) opCmd(o opCmd) *cobra.Command {
	name := o.displayName()
	cmd := &cobra.Command{
		Use:   o.name,
		Short: o.short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.root.ExecEach(o.name, func(p *che.ProfileReady) error {
				return p.ExecOperationNamed(name)
			})
		},
	}
	switch o.name {
	case "render-templates":
		cmd.Flags().BoolVar(&a.config.RenderSkipSecrets, "skip-secrets", false,
			"skip sources carrying op:// secret refs (logged, dests untouched); env: CHE_RENDER_TEMPLATES_SKIP_SECRETS")
	case "run-scripts":
		cmd.Use = "run-scripts [name...]"
		cmd.RunE = a.runScriptsRunE
	}
	return cmd
}

// [<] 🤖🤖
