// Package cli builds che's cobra command tree and orchestrates ops across the local repo and plugin checkouts.
package cli

// [>] 🤖🤖

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/internal/config"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/host"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/plugin"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

// repoUnit is one loaded repo: units[0] the local repo, the rest plugin checkouts
// (ref carries their @url::profile form, env the entry's exported envs).
type repoUnit struct {
	host host.Host
	res  spec.Resolved
	ref  string
	env  map[string]string
}

type pluginConfig struct {
	repoRoot string
	home     string
	cfg      config.Config
	eval     func(string) (bool, error)
}

// CheApp owns the CLI's flag destinations and the state built in
// PersistentPreRunE, read by each RunE. Plugin units build lazily
// (ensurePlugin), after the local repoUnit's ops ran.
type CheApp struct {
	dirFlag           string
	dryRunMode        string
	validateSpecMode  string
	profileForce      string
	skipExecIf        bool
	skipPlugins       bool
	debugFlag         bool
	renderSkipSecrets bool
	units             []repoUnit
	pluginRefs        []spec.PluginRef
	pluginUnits       map[string]*repoUnit
	pluginCfg         pluginConfig
	// newHost builds each repoUnit's Host; tests override it to inject a mock fs.
	newHost func(repoRoot, home, profile string, cfg config.Config) host.Host
}

func New() *CheApp {
	return &CheApp{newHost: host.New}
}

// version is injected at build time via -ldflags -X.
var version = "dev"

var dryRunModes = map[string]config.DryRunMode{
	"":      config.DryRun.Off,
	"delta": config.DryRun.Delta,
	"all":   config.DryRun.All,
}

// Root builds che's root command with every subcommand attached: the single
// command-tree source for main and docgen. Resolves the eligible profiles
// (build) before any subcommand runs.
func (app *CheApp) Root() *cobra.Command {
	root := &cobra.Command{
		Use:     "che",
		Version: version,
		Short:   "Spec-driven config loader",
		Long: `che resolves every eligible profile in che.yml (execIf predicates), then
loads the union of files/dirs/installs/services those profiles select.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return app.initApp()
		},
	}
	pf := root.PersistentFlags()
	pf.StringVarP(&app.dirFlag, "directory", "C", "",
		"change into this directory before resolving the repo; env: CHE_DIR")
	pf.StringVar(&app.dryRunMode, "dry-run", "",
		"print mutating actions instead of executing them; values: delta (changed dests, bare-flag default) | all (every dest); default: off; env: CHE_DRY_RUN")
	pf.Lookup("dry-run").NoOptDefVal = "delta"
	pf.StringVar(&app.validateSpecMode, "validate-spec", "",
		"validate each loaded che.yml spec against the JSON Schema; values: warn (log violations) | error (abort on violations); default: warn; env: CHE_VALIDATE_SPEC")
	pf.StringVar(&app.profileForce, "profile", "",
		"run only this profile (autoExec skipped, execIf still enforced); env: CHE_PROFILE")
	pf.BoolVar(&app.skipExecIf, "skip-exec-if", false,
		"treat every execIf predicate as passing; env: CHE_SKIP_EXEC_IF")
	pf.BoolVar(&app.skipPlugins, "skip-plugins", false,
		"skip plugins entries, load only the local repo; env: CHE_SKIP_PLUGINS")
	pf.BoolVar(&app.debugFlag, "debug", false,
		"print debug-level lines (plugin announce, clone/pull attempts); env: CHE_DEBUG")

	services := &cobra.Command{
		Use:   "services",
		Short: "load/unload/verify the profile's launchd services",
	}
	root.AddCommand(app.allCmd(), app.detectCmd(), services)
	for _, s := range steps() {
		cmd := app.stepCmd(s)
		if s.parent == "services" {
			services.AddCommand(cmd)
		} else {
			root.AddCommand(cmd)
		}
	}
	return root
}

// forEachRepoUnit runs op over every repoUnit: the local repo first, then each plugin
// grouped (announce, pull, execIf, resolve, op), skipped ones dropped. A
// failing repoUnit does not stop the rest: failures collect (ref-wrapped, "local"
// for the local repo), report as "<name>(report): fail <ref>: <err>" lines
// after all units, and join into the returned error.
func (app *CheApp) forEachRepoUnit(name string, op func(repoUnit) error) error {
	var fails []error
	run := func(ref string, u repoUnit) {
		if err := u.runWithEnv(func() error { return op(u) }); err != nil {
			fails = append(fails, fmt.Errorf("%s: %w", ref, err))
		}
	}
	for _, u := range app.units {
		run("local", u)
	}
	for _, p := range app.pluginRefs {
		u, ok, err := app.ensurePlugin(p)
		switch {
		case err != nil:
			fails = append(fails, fmt.Errorf("%s: %w", p, err))
		case ok:
			run(p.String(), u)
		}
	}
	for _, err := range fails {
		log.Msg(name+"(report)", "fail "+err.Error(), log.Off)
	}
	return errors.Join(fails...)
}

// ensurePlugin returns p's repoUnit, building it on first use (announced) and
// caching the outcome (ok=false: skipped by execIf).
func (app *CheApp) ensurePlugin(p spec.PluginRef) (repoUnit, bool, error) {
	if u, seen := app.pluginUnits[p.String()]; seen {
		if u == nil {
			return repoUnit{}, false, nil
		}
		return *u, true, nil
	}
	log.Debug("plugin("+p.Profile+")", fmt.Sprintf("run %s", p), log.Off)
	u, ok, err := app.buildPlugin(p)
	if err != nil {
		return repoUnit{}, false, err
	}
	if !ok {
		app.pluginUnits[p.String()] = nil
		return repoUnit{}, false, nil
	}
	app.pluginUnits[p.String()] = &u
	return u, true, nil
}

// runWithEnv runs fn with u.env exported (host values shadowed), restoring after.
func (u repoUnit) runWithEnv(fn func() error) error {
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

// initApp loads the spec, resolves the eligible-profile union, wires the host.
func (app *CheApp) initApp() error {
	dir := cmp.Or(app.dirFlag, os.Getenv("CHE_DIR"))
	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			return fmt.Errorf("-C: %w", err)
		}
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	home, err := invokingHome()
	if err != nil {
		return err
	}
	app.dryRunMode = cmp.Or(app.dryRunMode, os.Getenv("CHE_DRY_RUN"))
	mode, ok := dryRunModes[app.dryRunMode]
	if !ok {
		return fmt.Errorf("invalid --dry-run mode %q: want delta or all", app.dryRunMode)
	}
	app.validateSpecMode = cmp.Or(app.validateSpecMode, os.Getenv("CHE_VALIDATE_SPEC"), "warn")
	if app.validateSpecMode != "warn" && app.validateSpecMode != "error" {
		return fmt.Errorf("invalid --validate-spec mode %q: want warn or error", app.validateSpecMode)
	}
	sp, err := loadSpecValidated(filepath.Join(repoRoot, "che.yml"), app.validateSpecMode)
	if err != nil {
		return err
	}
	cfg := config.Config{
		Dir:          dir,
		DryRun:       mode,
		Profile:      cmp.Or(app.profileForce, os.Getenv("CHE_PROFILE")),
		SkipExecIf:   boolOrEnv(app.skipExecIf, "CHE_SKIP_EXEC_IF"),
		SkipPlugins:  boolOrEnv(app.skipPlugins, "CHE_SKIP_PLUGINS"),
		Debug:        boolOrEnv(app.debugFlag, "CHE_DEBUG"),
		ValidateSpec: app.validateSpecMode,
	}
	log.SetDebug(cfg.Debug)
	eval := spec.NewEvaluator().EvalExecIf
	profiles, err := sp.EligibleProfiles(cfg.Profile, cfg.SkipExecIf, eval)
	if err != nil {
		return err
	}
	h := app.newHost(repoRoot, home, strings.Join(profiles, ","), cfg)
	res, err := sp.Resolve(profiles, h.Root)
	if err != nil {
		return err
	}
	app.units = []repoUnit{{host: h, res: res}}
	app.pluginRefs = res.Plugins
	if cfg.SkipPlugins {
		app.pluginRefs = nil
	}
	app.pluginUnits = map[string]*repoUnit{}
	app.pluginCfg = pluginConfig{repoRoot: repoRoot, home: home, cfg: cfg, eval: eval}
	return nil
}

func boolOrEnv(flag bool, key string) bool {
	return flag || os.Getenv(key) != ""
}

func loadSpecValidated(path, mode string) (*spec.Raw, error) {
	if b, err := os.ReadFile(path); err == nil {
		if finds := spec.ValidateSchema(b); len(finds) > 0 {
			if mode == "error" {
				return nil, fmt.Errorf("schema violations in %s:\n%s", path, strings.Join(finds, "\n"))
			}
			for _, f := range finds {
				log.Msg("validate(che.yml)", f, log.Off)
			}
		}
	}
	return spec.Load(path)
}

// buildPlugin ensures the checkout (git ref: cache clone/pull, dir ref: local
// dir), loads its spec, gates on execIf inside the entry's env overlay,
// resolves anchored at the checkout. Nested plugin refs are ignored (v1).
func (app *CheApp) buildPlugin(p spec.PluginRef) (repoUnit, bool, error) {
	checkout, err := pluginCheckout(p, app.pluginCfg.repoRoot, app.pluginCfg.home)
	if err != nil {
		return repoUnit{}, false, err
	}
	psp, err := loadSpecValidated(filepath.Join(checkout, "che.yml"), app.pluginCfg.cfg.ValidateSpec)
	if err != nil {
		return repoUnit{}, false, err
	}
	h := app.newHost(checkout, app.pluginCfg.home, p.Profile, app.pluginCfg.cfg).WithLogSub("profile=" + p.Profile)
	u := repoUnit{host: h, ref: p.String(), env: p.Env}
	var pass bool
	err = u.runWithEnv(func() error {
		var err error
		if pass, err = psp.ExecIfPass(p.Profile, app.pluginCfg.cfg.SkipExecIf, app.pluginCfg.eval); err != nil || !pass {
			return err
		}
		u.res, err = psp.Resolve([]string{p.Profile}, u.host.Root)
		return err
	})
	if err != nil {
		return repoUnit{}, false, fmt.Errorf("plugin %s: %w", p, err)
	}
	if !pass {
		log.Msg("plugin("+p.Profile+")", fmt.Sprintf("skip %s (execIf failed)", p), log.Off)
		return repoUnit{}, false, nil
	}
	if len(u.res.Plugins) > 0 {
		log.Msg("plugin("+p.Profile+")", fmt.Sprintf("%s: nested plugin refs ignored: %v", p, u.res.Plugins), log.Off)
	}
	return u, true, nil
}

func pluginCheckout(p spec.PluginRef, repoRoot, home string) (string, error) {
	if !p.IsPath {
		return plugin.EnsureCheckout(home, p.URL, p.Profile)
	}
	return resolvePluginDir(p.URL, repoRoot, home)
}

func resolvePluginDir(ref, repoRoot, home string) (string, error) {
	dir := fsutil.ExpandHome(os.ExpandEnv(ref), home)
	if dir == "~" {
		dir = home
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repoRoot, dir)
	}
	if !fsutil.IsDir(dir) {
		return "", fmt.Errorf("plugin dir not found: %s (from ref %s)", dir, ref)
	}
	return dir, nil
}

// findRepoRoot: git toplevel of cwd, che.yml must live there (che's defining marker).
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
