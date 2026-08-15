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

var version = "dev"

type app struct {
	flags        options.Options
	opts         options.Options
	packagesKind string
	ctx          che.Context
	root         *che.SpecReady
	tel          *telemetry.Telemetry
	runCtx       context.Context
	runSpan      trace.Span
}

func New() *app { return &app{} }

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
				case "packages":
					name = "packages-" + name
				case "backup":
					// [why] backup create stays on the spec path under the
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
		"skip these ops everywhere (comma-separated or repeated; dropped from the run sequence, direct op subcommands become logged no-ops); values: prune-broken-links | make-dirs | make-links | make-copies | render-templates | install-packages | run-scripts; env: CHE_SKIP_OPS")
	pf.BoolVar(&a.flags.SkipRunIf, "skip-run-if", false,
		"treat every runIf predicate as passing; env: CHE_SKIP_RUN_IF")
	pf.BoolVar(&a.flags.Errexit, "errexit", false,
		"stop the run at the first script failure (remaining scripts, ops, profiles skipped); default: continue and report all failures; env: CHE_ERREXIT")
	pf.BoolVar(&a.flags.SkipRemoteRefs, "skip-remote-refs", false,
		"skip sourced include.profiles refs, load only the local repo's specs; env: CHE_SKIP_REMOTE_REFS")
	pf.StringVar(&a.flags.LogLevel, "log-level", "",
		"human-log level; values: error (failures only) | warn | info (what happened) | debug (adds intentions and won't-happen with reasons) | trace (adds details); default: info; env: CHE_LOG_LEVEL")

	root.AddCommand(a.runCmd(), a.initCmd(), a.backupCmd(), a.discoverCmd(), a.uninstallCmd(), a.configCmd(), a.packagesCmd())
	for _, o := range ops() {
		root.AddCommand(a.opCmd(o))
	}
	return root
}

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

func (a *app) shutdownTelemetry() {
	if a.runSpan != nil {
		a.runSpan.End()
	}
	log.SetSink(nil)
	if err := a.tel.Shutdown(context.Background()); err != nil {
		log.EmitTrace("otel", "error", "shutdown: "+err.Error())
	}
}

func (a *app) runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "run every op each profile selects, profile by profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.root.ExecEach(a.runCtx, "run", func(ctx context.Context, p *che.ProfileReady) error {
				// [why] the backup stage archives every op dest once, before the
				if err := p.ExecBackup(); err != nil {
					return err
				}
				return p.ExecOperations(ctx)
			})
		},
	}
	cmd.Flags().StringSliceVar(&a.flags.RunSkipOps, "skip-ops", nil,
		"skip these ops in the run sequence only (comma-separated or repeated); values: prune-broken-links | make-dirs | make-links | make-copies | render-templates | install-packages | run-scripts; env: CHE_RUN_SKIP_OPS")
	return cmd
}

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

func (a *app) initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init-remote-sources",
		Short: "fetch the remote spec sources (clone/pull the cache checkouts) and exit",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
}

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

func (a *app) configCmd() *cobra.Command {
	return makeConfigShowCmd(
		"inspect che's resolved configuration",
		"print the resolved options with their deciding sources (--delta default, --all for every option, --defaults for the code defaults)",
		showHelp{
			delta:    "print only the options differing from defaults (default mode)",
			all:      "print every option with its value and source",
			defaults: "print every option's default value (configured values ignored)",
			output:   "output format; values: text (key = value (source) lines) | yaml (config-file shape, seedable as $XDG_CONFIG_HOME/che/config.yml)",
		},
		"delta",
		func(mode, output string) error {
			settings := a.opts.SettingsDelta()
			switch mode {
			case "defaults":
				var err error
				if settings, err = options.DefaultSettings(); err != nil {
					return err
				}
			case "all":
				settings = a.opts.SettingsSorted()
			}
			return emitShowOutput(output, func() {
				for _, s := range settings {
					fmt.Printf("%s = %s  (%s)\n", s.Key, s.Value, s.DisplaySource())
				}
			}, func() (string, error) { return options.SettingsYAML(settings) })
		})
}

type showHelp struct {
	delta    string
	all      string
	defaults string
	output   string
}

func makeConfigShowCmd(parentShort, short string, help showHelp, defaultMode string, run func(mode, output string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: parentShort,
	}
	var delta, all, defaults bool
	var output string
	show := &cobra.Command{
		Use:   "show",
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := defaultMode
			switch {
			case delta:
				mode = "delta"
			case all:
				mode = "all"
			case defaults:
				mode = "defaults"
			}
			return run(mode, output)
		},
	}
	show.Flags().BoolVar(&delta, "delta", false, help.delta)
	show.Flags().BoolVar(&all, "all", false, help.all)
	show.Flags().BoolVar(&defaults, "defaults", false, help.defaults)
	show.Flags().StringVar(&output, "output", "text", help.output)
	show.MarkFlagsMutuallyExclusive("delta", "all", "defaults")
	cmd.AddCommand(show)
	return cmd
}

func emitShowOutput(output string, text func(), yaml func() (string, error)) error {
	switch output {
	case "", "text":
		text()
		return nil
	case "yaml":
		out, err := yaml()
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	default:
		return fmt.Errorf("invalid --output %q: want text or yaml", output)
	}
}

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
