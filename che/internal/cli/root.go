package cli

// [>] 🤖🤖

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go/che/internal/fsutil"
	"gitlab.com/konradodwrot/go/che/internal/host"
	"gitlab.com/konradodwrot/go/che/internal/log"
	"gitlab.com/konradodwrot/go/che/internal/plugin"
	"gitlab.com/konradodwrot/go/che/internal/spec"
)

// unit is one loaded repo: units[0] the local repo, the rest plugin checkouts
// (ref carries their @url::profile form, env the entry's exported envs).
type unit struct {
	host host.Host
	res  spec.Resolved
	ref  string
	env  map[string]string
}

// forEachUnit runs op over every unit: the local repo first, then each plugin
// grouped (announce, pull, execIf, resolve, op), skipped ones dropped.
func forEachUnit(op func(unit) error) error {
	for _, u := range units {
		if err := u.withEnv(func() error { return op(u) }); err != nil {
			return err
		}
	}
	for _, p := range pluginRefs {
		u, ok, err := ensurePlugin(p)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := u.withEnv(func() error { return op(u) }); err != nil {
			return err
		}
	}
	return nil
}

// ensurePlugin returns p's unit, building it on first use (announced) and
// caching the outcome (ok=false: skipped by execIf).
func ensurePlugin(p spec.PluginRef) (unit, bool, error) {
	if u, seen := pluginUnits[p.String()]; seen {
		if u == nil {
			return unit{}, false, nil
		}
		return *u, true, nil
	}
	log.Msg("plugin("+p.Profile+")", fmt.Sprintf("run %s", p), log.Off)
	u, ok, err := buildPlugin(p, pluginCfg.home, pluginCfg.mode, pluginCfg.forceAll, pluginCfg.eval)
	if err != nil {
		return unit{}, false, err
	}
	if !ok {
		pluginUnits[p.String()] = nil
		return unit{}, false, nil
	}
	pluginUnits[p.String()] = &u
	return u, true, nil
}

// withEnv runs fn with u.env exported into the process env (host values
// shadowed), restoring the prior state after. No-op when u.env is empty.
func (u unit) withEnv(fn func() error) error {
	if len(u.env) == 0 {
		return fn()
	}
	for _, k := range slices.Sorted(maps.Keys(u.env)) {
		prev, had := os.LookupEnv(k)
		if err := os.Setenv(k, u.env[k]); err != nil {
			return err
		}
		defer func() {
			if had {
				os.Setenv(k, prev)
			} else {
				os.Unsetenv(k)
			}
		}()
	}
	return fn()
}

// Built once in PersistentPreRunE, read by each RunE. Plugin units build
// lazily (ensurePlugin), after the local unit's ops ran.
var (
	dryRunMode   string
	profileForce string
	omitExecIf   bool
	skipPlugins  bool
	units        []unit
	pluginRefs   []spec.PluginRef
	pluginUnits  map[string]*unit
	pluginCfg    struct {
		home     string
		mode     host.DryRunMode
		forceAll bool
		eval     func(string) (bool, error)
	}
)

// version is injected at build time via -ldflags -X.
var version = "dev"

// dryRunModes maps the --dry-run flag value to a host.DryRunMode.
var dryRunModes = map[string]host.DryRunMode{
	"":      host.DryRunOff,
	"delta": host.DryRunDelta,
	"all":   host.DryRunAll,
}

// RootCmd is che's root command. Resolves the eligible profiles (build) before
// any subcommand runs. Subcommands attached by the command package.
var RootCmd = &cobra.Command{
	Use:     "che",
	Version: version,
	Short:   "Spec-driven config loader",
	Long: `che resolves every eligible profile in che.yml (execIf predicates), then
loads the union of files/dirs/installs/services those profiles select.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return build()
	},
}

func init() {
	RootCmd.PersistentFlags().StringVar(&dryRunMode, "dry-run", "",
		"print mutating actions instead of executing them: delta (changed dests) | all (every dest)")
	RootCmd.PersistentFlags().Lookup("dry-run").NoOptDefVal = "delta"
	RootCmd.PersistentFlags().StringVar(&profileForce, "profile", "",
		"run only this profile (autoExec skipped, execIf still enforced); env: CHE_PROFILE")
	RootCmd.PersistentFlags().BoolVar(&omitExecIf, "omit-exec-if", false,
		"treat every execIf predicate as passing; env: CHE_OMIT_EXEC_IF")
	RootCmd.PersistentFlags().BoolVar(&skipPlugins, "skip-plugins", false,
		"skip plugins entries, load only the local repo; env: CHE_SKIP_PLUGINS")
}

// Attach wires every subcommand onto RootCmd and returns it: the single
// command-tree source for main and docgen.
func Attach() *cobra.Command {
	RootCmd.AddCommand(
		LinkCmd,
		CopyCmd,
		RenderCmd,
		DirsCmd,
		PruneCmd,
		RunScriptsCmd,
		DetectCmd,
		ServicesCmd,
	)
	return RootCmd
}

// build loads spec -> lists eligible profiles -> resolves union -> wires the
// host. Run in PersistentPreRunE before any subcommand RunE.
func build() error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	home, err := invokingHome()
	if err != nil {
		return err
	}
	if dryRunMode == "" {
		dryRunMode = os.Getenv("CHE_DRY_RUN")
	}
	mode, ok := dryRunModes[dryRunMode]
	if !ok {
		return fmt.Errorf("invalid --dry-run mode %q: want delta or all", dryRunMode)
	}
	sp, err := spec.Load(filepath.Join(repoRoot, "che.yml"))
	if err != nil {
		return err
	}
	// --profile (env CHE_PROFILE) runs only that profile, autoExec
	// skipped but execIf still enforced; --omit-exec-if (env
	// CHE_OMIT_EXEC_IF, truthy) makes every execIf pass; else the union of
	// every autoExec profile passing execIf. --skip-plugins (env
	// CHE_SKIP_PLUGINS, truthy) drops plugins entries, local repo only.
	// Flags win over envs.
	forceOne := profileForce
	if forceOne == "" {
		forceOne = os.Getenv("CHE_PROFILE")
	}
	forceAll := omitExecIf || os.Getenv("CHE_OMIT_EXEC_IF") != ""
	eval := spec.NewEvaluator().EvalExecIf
	profiles, err := sp.EligibleProfiles(forceOne, forceAll, eval)
	if err != nil {
		return err
	}
	h := host.New(repoRoot, home, strings.Join(profiles, ","), mode)
	res, err := sp.Resolve(profiles, h.Root)
	if err != nil {
		return err
	}
	units = []unit{{host: h, res: res}}
	pluginRefs = res.Plugins
	if skipPlugins || os.Getenv("CHE_SKIP_PLUGINS") != "" {
		pluginRefs = nil
	}
	pluginUnits = map[string]*unit{}
	pluginCfg.home, pluginCfg.mode, pluginCfg.forceAll, pluginCfg.eval = home, mode, forceAll, eval
	return nil
}

// buildPlugin ensures the plugin checkout, loads its spec, then inside the
// entry's env overlay gates on the remote profile's execIf (fail -> skipped,
// logged) and resolves it anchored at the checkout. Nested plugin refs inside
// the remote profile are ignored (v1).
func buildPlugin(p spec.PluginRef, home string, mode host.DryRunMode, forceAll bool, eval func(string) (bool, error)) (unit, bool, error) {
	checkout, err := plugin.Ensure(home, p.URL, p.Profile)
	if err != nil {
		return unit{}, false, err
	}
	psp, err := spec.Load(filepath.Join(checkout, "che.yml"))
	if err != nil {
		return unit{}, false, err
	}
	u := unit{host: host.New(checkout, home, p.Profile, mode), ref: p.String(), env: p.Env}
	var pass bool
	err = u.withEnv(func() error {
		var err error
		if pass, err = psp.ExecIfPass(p.Profile, forceAll, eval); err != nil || !pass {
			return err
		}
		u.res, err = psp.Resolve([]string{p.Profile}, u.host.Root)
		return err
	})
	if err != nil {
		return unit{}, false, fmt.Errorf("plugin %s: %w", p, err)
	}
	if !pass {
		log.Msg("plugin("+p.Profile+")", fmt.Sprintf("skip %s (execIf failed)", p), log.Off)
		return unit{}, false, nil
	}
	if len(u.res.Plugins) > 0 {
		log.Msg("plugin("+p.Profile+")", fmt.Sprintf("%s: nested plugin refs ignored: %v", p, u.res.Plugins), log.Off)
	}
	return u, true, nil
}

// findRepoRoot resolves repo root from git toplevel of cwd, verifies che.yml
// lives there (che's defining marker).
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := fsutil.RepoRoot(dir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(root, "che.yml")); err != nil {
		return "", fmt.Errorf("che.yml not found at repo root %s", root)
	}
	return root, nil
}

// invokingHome resolves the invoking user's home. Under sudo (EUID 0,
// SUDO_USER set), looks up that user's home from passwd so dest paths derive
// from the real user, not /var/root. Otherwise uses $HOME.
func invokingHome() (string, error) {
	if os.Geteuid() == 0 {
		if name := os.Getenv("SUDO_USER"); name != "" {
			h, err := fsutil.UserHome(name)
			if err != nil {
				return "", fmt.Errorf("lookup SUDO_USER %q: %w", name, err)
			}
			return h, nil
		}
	}
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("HOME must be set")
	}
	return home, nil
}

// [<] 🤖🤖
