package cli

// [>] 🤖🤖

import (
	"context"

	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/internal/che"
)

// opCmd is one per-op subcommand shape: its cobra wiring plus the prepared
// operation name it runs over every profile.
type opCmd struct {
	name  string
	short string
}

// ops lists every op subcommand in run order: the one source for subcommand
// construction (run order itself lives in che.prepareOperations).
func ops() []opCmd {
	return []opCmd{
		{name: "prune-broken-links", short: "delete broken symlinks"},
		{name: "make-dirs", short: "create repo-tree dirs + extra-dirs"},
		{name: "make-links", short: "symlink op (configs into system root)"},
		{name: "make-copies", short: "*.ontoHost.cp copy op"},
		{name: "render-templates", short: "render *.tpl sources; each dest path decides target (relative -> repo, ~/ or absolute -> host)"},
		{name: "run-scripts", short: "run the profile's scripts, optionally filtered by name substring"},
	}
}

// opCmd builds an op's subcommand: RunE runs its prepared operation over every
// profile. run-scripts and render-templates layer their arg filter / flag on top.
func (a *app) opCmd(o opCmd) *cobra.Command {
	name := o.name
	cmd := &cobra.Command{Use: o.name, Short: o.short}
	switch o.name {
	case "run-scripts":
		cmd.Use = "run-scripts [name...]"
		cmd.RunE = a.runScriptsRunE
	default:
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			return a.root.ExecEach(a.runCtx, o.name, func(ctx context.Context, p *che.ProfileReady) error {
				return p.ExecOperationNamed(ctx, name)
			})
		}
	}
	if o.name == "render-templates" {
		cmd.Flags().BoolVar(&a.flags.RenderSkipSecrets, "skip-secrets", false,
			"skip sources carrying op:// or gcp:// secret refs (logged, dests untouched); env: CHE_RENDER_TEMPLATES_SKIP_SECRETS")
	}
	return cmd
}

// [<] 🤖🤖
