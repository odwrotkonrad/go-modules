// Package cli builds che's cobra command tree over the che package's prepared specs.
package cli

// [>] 🤖🤖

import (
	"context"
	"fmt"
	"slices"

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
		Long: `che resolves every eligible profile in che.yml (runIf predicates), then
runs each profile's full op sequence, profile by profile (composed specs and
sourced profile refs included).`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			name := cmd.Name()
			if p := cmd.Parent(); p != nil {
				switch p.Name() {
				case "config":
					name = "config"
				case "backup":
					// [why] backup create stays on the spec path under the
					// command name "backup"; ls/restore are ledger-only.
					if name == "create" {
						name = "backup"
					} else {
						name = "backup-" + name
					}
				}
			}
			return a.init(name)
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			a.shutdownTelemetry()
			return nil
		},
	}
	pf := root.PersistentFlags()
	pf.StringVarP(&a.flags.CheWorkingDirectory, "che-working-directory", "C", "",
		"change into this directory before resolving the repo; env: CHE_WORKING_DIRECTORY")
	pf.StringVar(&a.flags.ProfileWorkingDirectory, "profile-working-directory", "",
		"the load-ops source tree (che level; spec/profile options.profileWorkingDirectory override); default root; env: CHE_PROFILE_WORKING_DIRECTORY")
	pf.StringVar((*string)(&a.flags.DryRun), "dry-run", "",
		"print mutating actions instead of executing them; values: delta (changed dests, bare-flag default) | all (every dest) | true (alias for delta); default: off; env: CHE_DRY_RUN")
	pf.Lookup("dry-run").NoOptDefVal = "delta"
	pf.StringVar((*string)(&a.flags.ValidateSpec), "validate-spec", "",
		"validate each loaded che.yml spec against the JSON Schema; values: warn (log violations) | error (abort on violations); default: warn; env: CHE_VALIDATE_SPEC")
	pf.StringSliceVar(&a.flags.Profiles, "profiles", nil,
		"run only these profiles (comma-separated or repeated; autoDiscover skipped, runIf still enforced); env: CHE_PROFILE (comma-separated)")
	pf.StringSliceVar(&a.flags.SkipOps, "skip-ops", nil,
		"skip these ops everywhere (comma-separated or repeated; dropped from the run sequence, direct op subcommands become logged no-ops); values: prune-broken-links | make-dirs | make-links | make-copies | render-templates | run-scripts; env: CHE_SKIP_OPS")
	pf.BoolVar(&a.flags.SkipRunIf, "skip-run-if", false,
		"treat every runIf predicate as passing; env: CHE_SKIP_RUN_IF")
	pf.BoolVar(&a.flags.SkipRemoteRefs, "skip-remote-refs", false,
		"skip sourced include.profiles refs, load only the local repo's specs; env: CHE_SKIP_REMOTE_REFS")
	pf.StringVar(&a.flags.LogLevel, "log-level", "",
		"human-log level; values: error (failures only) | warn | info (what happened) | debug (adds intentions and won't-happen with reasons) | trace (adds details); default: info; env: CHE_LOG_LEVEL")

	root.AddCommand(a.runCmd(), a.initCmd(), a.backupCmd(), a.discoverCmd(), a.uninstallCmd(), a.configCmd())
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
	// [why] the dry-run announce opens the whole output, then the config
	// delta at debug (spec/che/log.md). `che config show` owns
	// its own config output, so it skips the startup delta line; completion
	// commands print to stdout for shell eval, so they stay silent.
	if command != "completion" && command != "__complete" {
		if a.opts.DryRun != options.DryRun.Off {
			desc := "no actual operations will be performed, reporting only dests that would change"
			if a.opts.DryRun == options.DryRun.All {
				desc = "no actual operations will be performed, reporting every dest's state"
			}
			log.EmitInfo("config", "dry-run", "("+string(a.opts.DryRun)+") "+desc)
		}
		if command != "config" {
			log.EmitDebug("config", "config-delta", options.FormatSettings(a.opts.SettingsDelta()))
		}
	}
	a.startTelemetry(ctx)
	ctx.Tel = a.tel
	a.runCtx, a.runSpan = a.tel.Span(context.Background(), "che run",
		attribute.String("che.command", command), attribute.String("che.run_id", ctx.RunID))
	ctx.RunCtx = a.runCtx
	a.ctx = ctx
	a.tel.CountCommand(a.runCtx, command)
	if command == "uninstall" || command == "config" || command == "backup-ls" || command == "backup-restore" {
		return nil
	}
	// [why] the init stage prefetches every remote spec source before
	// discovery; run announces both stages as sub-headings like its ops.
	if command == "run" {
		log.EmitHeading(log.Levels.Info, 1, "run", "running", "init-remote-sources")
	}
	if err := che.InitSources(ctx, a.opts); err != nil {
		return err
	}
	if command == "init-remote-sources" {
		return nil
	}
	if command == "run" {
		log.EmitHeading(log.Levels.Info, 1, "run", "running", "discover-profiles")
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
		log.EmitTrace("otel", "error", "start failed: "+err.Error())
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
		log.EmitTrace("otel", "error", "shutdown: "+err.Error())
	}
}

// runCmd runs every profile's full op sequence, profile by profile. A failing
// profile does not stop the rest: failures collect, report, and join.
func (a *app) runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "run every op each profile selects, profile by profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.root.ExecEach(a.runCtx, "run", func(ctx context.Context, p *che.ProfileReady) error {
				// [why] the backup stage archives every op dest once, before the
				// other ops; they skip their own archives.
				if err := p.ExecBackupStage(); err != nil {
					return err
				}
				return p.ExecOperations(ctx)
			})
		},
	}
	cmd.Flags().StringSliceVar(&a.flags.RunSkipOps, "skip-ops", nil,
		"skip these ops in the run sequence only (comma-separated or repeated); values: prune-broken-links | make-dirs | make-links | make-copies | render-templates | run-scripts; env: CHE_RUN_SKIP_OPS")
	return cmd
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

// initCmd runs only the init stage: prefetch every remote spec source
// (clone/pull the run cache) and exit; the fetch itself happens in init().
func (a *app) initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init-remote-sources",
		Short: "fetch the remote spec sources (clone/pull the cache checkouts) and exit",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
}

// backupCmd is the backup command group: `backup create` archives every
// would-change op dest, `backup ls` lists the recorded backup points, `backup
// restore` restores state by --run-id, --backup-id, or --timestamp. Bare
// `backup` prints usage.
func (a *app) backupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "manage backup archives: create, ls, restore",
	}
	create := &cobra.Command{
		Use:   "create",
		Short: "archive every op dest (links, copies, host renders) into the per-run backup archive and exit",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.root.ExecEach(a.runCtx, "backup", func(ctx context.Context, p *che.ProfileReady) error {
				return p.ExecBackup()
			})
		},
	}
	ls := &cobra.Command{
		Use:   "ls",
		Short: "list the backup points (run id, backup id, timestamp, size, path), newest first",
		RunE: func(cmd *cobra.Command, args []string) error {
			return che.ListBackups(a.ctx)
		},
	}
	var sel che.RestoreSelector
	restore := &cobra.Command{
		Use:   "restore",
		Short: "restore state from backup archives: --run-id (that run's archives), --backup-id (one archive), --timestamp (point-in-time)",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := che.NewRestorer(a.ctx, a.opts)
			if err != nil {
				return err
			}
			defer func() { _ = r.Close() }()
			return r.Restore(sel)
		},
	}
	restore.Flags().StringVar(&sel.RunID, "run-id", "", "restore every archive of this run id")
	restore.Flags().StringVar(&sel.BackupID, "backup-id", "", "restore the single archive carrying this backup id")
	restore.Flags().StringVar(&sel.Timestamp, "timestamp", "", "point-in-time restore: per dest, the newest backup at or before this timestamp (20060102T150405)")
	restore.MarkFlagsMutuallyExclusive("run-id", "backup-id", "timestamp")
	cmd.AddCommand(create, ls, restore)
	return cmd
}

// discoverCmd logs each prepared profile's discovered entry (per-op change
// counts) and exits.
func (a *app) discoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover-profiles",
		Short: "log the discovered profiles with their per-op changes and exit",
		RunE: func(cmd *cobra.Command, args []string) error {
			a.root.LogDiscovered()
			return nil
		},
	}
}

// configCmd is the config command group: `config show` prints the resolved
// options, one "key = value  (source)" line each, the non-default delta by
// default (--delta), the full set with --all.
func (a *app) configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "inspect che's resolved configuration",
	}
	var delta, all bool
	show := &cobra.Command{
		Use:   "show",
		Short: "print the resolved options with their deciding sources (--delta default, --all for every option)",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings := a.opts.SettingsDelta()
			if all {
				settings = a.opts.SettingsSorted()
			}
			for _, s := range settings {
				fmt.Printf("%s = %s  (%s)\n", s.Key, s.Value, s.DisplaySource())
			}
			return nil
		},
	}
	show.Flags().BoolVar(&delta, "delta", false, "print only the options differing from defaults (default mode)")
	show.Flags().BoolVar(&all, "all", false, "print every option with its value and source")
	show.MarkFlagsMutuallyExclusive("delta", "all")
	cmd.AddCommand(show)
	return cmd
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
	if len(args) > 0 && total == 0 && !slices.Contains(a.opts.SkipOps, "run-scripts") {
		return fmt.Errorf("no script matches: %v", args)
	}
	return nil
}

// [<] 🤖🤖
