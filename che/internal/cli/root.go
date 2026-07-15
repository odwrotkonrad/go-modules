// Package cli builds che's cobra command tree over the che package's prepared specs.
package cli

// [>] 🤖🤖

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"gitlab.com/konradodwrot/go-modules/che/internal/che"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/telemetry"
)

// version is injected at build time via -ldflags -X.
var version = "dev"

// app wires the cobra tree: init (PersistentPreRunE) prepares opts and the
// root SpecReady as separately initialized values, read by each RunE.
type app struct {
	flags   options.Options      // cobra flag destinations
	opts    options.Options      // finalized by che.PrepareApplicationOptions
	ctx     che.Context          // captured launch world (env/cwd/runID/command), for spec-less commands (uninstall)
	root    *che.SpecReady       // prepared by che.PrepareSpecs
	tel     *telemetry.Telemetry // OTLP telemetry, started in init, flushed in PersistentPostRunE (nil = off)
	runCtx  context.Context      // run root span ctx, opened in init, parent of every command span
	runSpan trace.Span           // the run root span, ended in shutdownTelemetry
}

func New() *app { return &app{} }

// Root builds che's root command with every subcommand attached: the single
// command-tree source for main and docgen.
func (a *app) Root() *cobra.Command {
	root := &cobra.Command{
		Use:     "che",
		Version: version,
		Short:   "Spec-driven config loader",
		Long: `che resolves every eligible profile in che.yml (execIf predicates), then
runs each profile's full op sequence, profile by profile (composed specs and
sourced profile refs included).`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return a.init(cmd.Name())
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			a.shutdownTelemetry()
			return nil
		},
	}
	pf := root.PersistentFlags()
	pf.StringVarP(&a.flags.Dir, "directory", "C", "",
		"change into this directory before resolving the repo; env: CHE_DIR")
	pf.StringVar(&a.flags.WorkingDirectory, "working-directory", "",
		"the load-ops source tree (che level; spec/profile options.workingDirectory override); default root; env: CHE_WORKING_DIRECTORY")
	pf.StringVar((*string)(&a.flags.DryRun), "dry-run", "",
		"print mutating actions instead of executing them; values: delta (changed dests, bare-flag default) | all (every dest) | true (alias for all); default: off; env: CHE_DRY_RUN")
	pf.Lookup("dry-run").NoOptDefVal = "delta"
	pf.StringVar((*string)(&a.flags.ValidateSpec), "validate-spec", "",
		"validate each loaded che.yml spec against the JSON Schema; values: warn (log violations) | error (abort on violations); default: warn; env: CHE_VALIDATE_SPEC")
	pf.StringSliceVar(&a.flags.Profiles, "profiles", nil,
		"run only these profiles (comma-separated or repeated; autoDiscover skipped, execIf still enforced); env: CHE_PROFILE (comma-separated)")
	pf.BoolVar(&a.flags.SkipExecIf, "skip-exec-if", false,
		"treat every execIf predicate as passing; env: CHE_SKIP_EXEC_IF")
	pf.BoolVar(&a.flags.SkipRemoteRefs, "skip-remote-refs", false,
		"skip sourced include.profiles refs, load only the local repo's specs; env: CHE_SKIP_REMOTE_REFS")
	pf.BoolVar(&a.flags.Debug, "debug", false,
		"print debug-level lines (source announce, clone/pull attempts); env: CHE_DEBUG")

	root.AddCommand(a.allCmd(), a.discoverCmd(), a.uninstallCmd())
	for _, o := range ops() {
		root.AddCommand(a.opCmd(o))
	}
	return root
}

// init prepares the run: build the launch context from the process (the one
// ambient-read site), resolve options (flags > env vars > local che.yml
// options: > defaults), then the whole spec tree. command names the invoked
// subcommand, carried to the ledger run (SpecDone.Command). uninstall reads the
// ledger only, so it skips spec preparation (no che.yml needed).
func (a *app) init(command string) error {
	ctx, err := che.NewContext()
	if err != nil {
		return err
	}
	ctx.Command = command
	ctx, a.opts, err = che.PrepareApplicationOptions(ctx, a.flags)
	if err != nil {
		return err
	}
	a.startTelemetry(ctx)
	ctx.Tel = a.tel
	a.runCtx, a.runSpan = a.tel.Span(context.Background(), "che run",
		attribute.String("che.command", command), attribute.String("che.run_id", ctx.RunID))
	ctx.RunCtx = a.runCtx
	a.ctx = ctx
	a.tel.CountCommand(a.runCtx, command)
	if command == "uninstall" {
		return nil
	}
	a.root, err = che.PrepareSpecs(ctx, a.opts, spec.SpecSourceRecipe{})
	if err != nil {
		return err
	}
	a.tel.CountSpec(a.runCtx)
	return nil
}

// startTelemetry starts the OTLP providers from the resolved otel config and
// registers the log-mirror sink. A start error (unreachable collector is not one:
// the dial is lazy) degrades to telemetry off, never failing the run.
func (a *app) startTelemetry(ctx che.Context) {
	cfg := telemetry.Config(a.opts.Otel)
	tel, err := telemetry.Start(context.Background(), cfg, ctx.RunID, ctx.Command)
	if err != nil {
		log.Debug("otel", "start failed: "+err.Error(), log.Off)
		return
	}
	a.tel = tel
	if tel != nil {
		log.SetSink(tel.LogRecord)
	}
}

// shutdownTelemetry flushes and closes the providers (bounded), clears the sink.
// A flush error (unreachable collector) is logged at debug, never surfaced.
func (a *app) shutdownTelemetry() {
	if a.runSpan != nil {
		a.runSpan.End()
	}
	log.SetSink(nil)
	if err := a.tel.Shutdown(context.Background()); err != nil {
		log.Debug("otel", "shutdown: "+err.Error(), log.Off)
	}
}

// allCmd runs every profile's full op sequence, profile by profile. A failing
// profile does not stop the rest: failures collect, report, and join.
func (a *app) allCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "all",
		Short: "run every op each profile selects, profile by profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.root.ExecEach(a.runCtx, "all", func(ctx context.Context, p *che.ProfileReady) error {
				return p.ExecOperations(ctx)
			})
		},
	}
}

// uninstallCmd backs out everything the ledger marks installed onto this host,
// across every run: restore each dest's pre-install backup or remove it
// (snapshotting first so uninstall is reversible). Reads the ledger, not the
// spec, so it does not iterate profiles.
func (a *app) uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "back out everything che installed (ledger-driven), restoring pre-install backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := che.NewUninstaller(a.ctx, a.opts)
			if err != nil {
				return err
			}
			defer func() { _ = u.Close() }()
			return u.Uninstall()
		},
	}
}

// discoverCmd prints each prepared profile's ref and exits.
func (a *app) discoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "print the prepared profiles (one per line) and exit",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, p := range a.root.AllProfiles() {
				fmt.Println(p.Ref())
			}
			return nil
		},
	}
}

// runScriptsRunE is the run-scripts RunE: the op plus the name-substring arg
// filter and a no-match check across all profiles.
func (a *app) runScriptsRunE(cmd *cobra.Command, args []string) error {
	total := 0
	err := a.root.ExecEach(a.runCtx, cmd.Name(), func(ctx context.Context, p *che.ProfileReady) error {
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
