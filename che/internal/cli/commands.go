package cli

// [>] 🤖🤖

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/internal/che"
)

type op struct {
	name  string
	short string
}

func makeOps() []op {
	return []op{
		{name: "prune-broken-links", short: "delete broken symlinks"},
		{name: "make-dirs", short: "create repo-tree dirs + extra-dirs"},
		{name: "make-links", short: "symlink op (configs into system root)"},
		{name: "make-copies", short: "*.ontoHost.cp copy op"},
		{name: "render-templates", short: "render *.tpl sources; each dest path decides target (relative -> repo, ~/ or absolute -> host)"},
		{name: "run-scripts", short: "run the profile's scripts, optionally filtered by name substring"},
	}
}

func (a *app) makeOpCmd(o op) *cobra.Command {
	cmd := &cobra.Command{Use: o.name, Short: o.short}
	switch o.name {
	case "run-scripts":
		cmd.Use = "run-scripts [name...]"
		cmd.RunE = a.runScriptsRunE
	default:
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			return a.specs.ExecEach(a.runCtx, o.name, func(ctx context.Context, p *che.ProfileReady) error {
				return p.ExecOperationNamed(ctx, o.name)
			})
		}
	}
	if o.name == "render-templates" {
		cmd.Flags().BoolVar(&a.flags.RenderSkipSecrets, "skip-secrets", false,
			"skip sources carrying op:// or gcp:// secret refs (logged, dests untouched); env: CHE_RENDER_TEMPLATES_SKIP_SECRETS")
	}
	return cmd
}

func (a *app) runScriptsRunE(cmd *cobra.Command, args []string) error {
	total := 0
	err := a.specs.ExecEach(a.runCtx, cmd.Name(), func(ctx context.Context, p *che.ProfileReady) error {
		n, err := p.ExecRunScripts(ctx, args)
		total += n
		return err
	})
	if err != nil {
		return err
	}
	if len(args) > 0 && total == 0 {
		return fmt.Errorf("no script matches: %v", args)
	}
	return nil
}

// [<] 🤖🤖
