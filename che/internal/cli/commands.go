package cli

// [>] 🤖🤖

import (
	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/internal/host"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

// step is one op of the full load: its subcommand shape (use/parent/short)
// plus the repoUnit op and a predicate over the local repo's resolved selection
// gating it under `all`.
type step struct {
	name     string
	parent   string
	short    string
	op       func(*CheApp, repoUnit) error
	selected func(spec.Resolved) bool
}

// displayName is the step's `all` log name: parent-qualified for subcommands.
func (s step) displayName() string {
	if s.parent != "" {
		return s.parent + " " + s.name
	}
	return s.name
}

// steps lists every op in load order: the one source for `all` and for
// subcommand construction.
func steps() []step {
	dirs := func(r spec.Resolved) bool { return len(r.Dirs) > 0 }
	svc := func(r spec.Resolved) bool { return len(r.Services) > 0 }
	return []step{
		{
			name: "prune-links", short: "delete broken symlinks",
			op:       func(_ *CheApp, u repoUnit) error { return u.host.PruneBrokenLinks(u.res.Dirs) },
			selected: dirs,
		},
		{
			name: "mk-dirs", short: "create repo-tree dirs + extra-dirs",
			op:       func(_ *CheApp, u repoUnit) error { return u.host.MkDirs(u.res.Dirs, u.res.ExtraDirs) },
			selected: func(r spec.Resolved) bool { return len(r.Dirs)+len(r.ExtraDirs) > 0 },
		},
		{
			name: "link", short: "symlink op (configs into system root)",
			op:       func(_ *CheApp, u repoUnit) error { return u.host.MkLinks(u.res.Links, u.res.Dirs) },
			selected: func(r spec.Resolved) bool { return len(r.Links) > 0 },
		},
		{
			name: "copy", short: "*.ontoHost.cp copy op",
			op:       func(_ *CheApp, u repoUnit) error { return u.host.MkCopies(u.res.Copies, u.res.Dirs) },
			selected: func(r spec.Resolved) bool { return len(r.Copies) > 0 },
		},
		{
			name: "render-templates", short: "render *.tpl sources; each dest path decides target (relative -> repo, ~/ or absolute -> host)",
			op: func(app *CheApp, u repoUnit) error {
				return u.host.RenderTemplates(u.res.Templates,
					boolOrEnv(app.renderSkipSecrets, "CHE_RENDER_TEMPLATES_SKIP_SECRETS"))
			},
			selected: func(r spec.Resolved) bool { return len(r.Templates) > 0 },
		},
		{
			name: "run-scripts", short: "run the profile's scripts, optionally filtered by name substring",
			op: func(_ *CheApp, u repoUnit) error {
				_, err := runScripts(u, nil)
				return err
			},
			selected: func(r spec.Resolved) bool { return len(r.Scripts) > 0 },
		},
		{
			name: "bootout", parent: "services", short: "unload each service (bootout if loaded, wait until gone)",
			op: serviceOp(host.Host.Bootout), selected: svc,
		},
		{
			name: "bootin", parent: "services", short: "load each service (bootstrap from plist)",
			op: serviceOp(host.Host.Bootin), selected: svc,
		},
		{
			name: "ensure", parent: "services", short: "settle then verify each long-running service has a live pid",
			op: serviceOp(host.Host.Ensure), selected: svc,
		},
	}
}

// stepCmd builds a step's subcommand: RunE runs its op over every repoUnit.
// run-scripts and render-templates layer their arg filter / flag on top.
func (app *CheApp) stepCmd(s step) *cobra.Command {
	cmd := &cobra.Command{
		Use:   s.name,
		Short: s.short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.forEachRepoUnit(cmd.Name(), func(u repoUnit) error { return s.op(app, u) })
		},
	}
	switch s.name {
	case "render-templates":
		cmd.Flags().BoolVar(&app.renderSkipSecrets, "skip-secrets", false,
			"skip sources carrying op:// secret refs (logged, dests untouched); env: CHE_RENDER_TEMPLATES_SKIP_SECRETS")
	case "run-scripts":
		cmd.Use = "run-scripts [name...]"
		cmd.RunE = app.runScriptsRunE
	}
	return cmd
}

// [<] 🤖🤖
