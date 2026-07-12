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

// unit is one loaded repo: units[0] the local repo, the rest plugin checkouts
// (ref carries their @url::profile form, env the entry's exported envs).
type unit struct {
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
// (ensurePlugin), after the local unit's ops ran.
type CheApp struct {
	dirFlag            string
	dryRunMode         string
	validateSchemaMode string
	profileForce       string
	skipExecIf         bool
	skipPlugins        bool
	debugFlag          bool
	renderSkipSecrets  bool
	units              []unit
	pluginRefs         []spec.PluginRef
	pluginUnits        map[string]*unit
	pluginCfg          pluginConfig
	// newHost builds each unit's Host; tests override it to inject a mock fs.
	newHost func(repoRoot, home, profile string, cfg config.Config) host.Host
}

func New() *CheApp {
	return &CheApp{newHost: host.New}
}

// version is injected at build time via -ldflags -X.
var version = "dev"

// dryRunModes maps the --dry-run flag value to a config.DryRunMode.
var dryRunModes = map[string]config.DryRunMode{
	"":      config.DryRunOff,
	"delta": config.DryRunDelta,
	"all":   config.DryRunAll,
}

// Root builds che's root command with every subcommand attached: the single
// command-tree source for main and docgen. Resolves the eligible profiles
// (build) before any subcommand runs.
func (c *CheApp) Root() *cobra.Command {
	root := &cobra.Command{
		Use:     "che",
		Version: version,
		Short:   "Spec-driven config loader",
		Long: `che resolves every eligible profile in che.yml (execIf predicates), then
loads the union of files/dirs/installs/services those profiles select.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return c.build()
		},
	}
	pf := root.PersistentFlags()
	pf.StringVarP(&c.dirFlag, "directory", "C", "",
		"change into this directory before resolving the repo; env: CHE_DIR")
	pf.StringVar(&c.dryRunMode, "dry-run", "",
		"print mutating actions instead of executing them; values: delta (changed dests, bare-flag default) | all (every dest); default: off; env: CHE_DRY_RUN")
	pf.Lookup("dry-run").NoOptDefVal = "delta"
	pf.StringVar(&c.validateSchemaMode, "validate-schema", "",
		"validate each loaded che.yml against its JSON Schema; values: warn (log violations) | error (abort on violations); default: warn; env: CHE_VALIDATE_SCHEMA")
	pf.StringVar(&c.profileForce, "profile", "",
		"run only this profile (autoExec skipped, execIf still enforced); env: CHE_PROFILE")
	pf.BoolVar(&c.skipExecIf, "skip-exec-if", false,
		"treat every execIf predicate as passing; env: CHE_SKIP_EXEC_IF")
	pf.BoolVar(&c.skipPlugins, "skip-plugins", false,
		"skip plugins entries, load only the local repo; env: CHE_SKIP_PLUGINS")
	pf.BoolVar(&c.debugFlag, "debug", false,
		"print debug-level lines (plugin announce, clone/pull attempts); env: CHE_DEBUG")

	services := &cobra.Command{
		Use:   "services",
		Short: "load/unload/verify the profile's launchd services",
	}
	root.AddCommand(c.allCmd(), c.detectCmd(), services)
	for _, s := range steps() {
		cmd := c.stepCmd(s)
		if s.parent == "services" {
			services.AddCommand(cmd)
		} else {
			root.AddCommand(cmd)
		}
	}
	return root
}

// forEachUnit runs op over every unit: the local repo first, then each plugin
// grouped (announce, pull, execIf, resolve, op), skipped ones dropped. A
// failing unit does not stop the rest: failures collect (ref-wrapped, "local"
// for the local repo), report as "<name>(report): fail <ref>: <err>" lines
// after all units, and join into the returned error.
func (c *CheApp) forEachUnit(name string, op func(unit) error) error {
	var fails []error
	run := func(ref string, u unit) {
		if err := u.withEnv(func() error { return op(u) }); err != nil {
			fails = append(fails, fmt.Errorf("%s: %w", ref, err))
		}
	}
	for _, u := range c.units {
		run("local", u)
	}
	for _, p := range c.pluginRefs {
		u, ok, err := c.ensurePlugin(p)
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

// ensurePlugin returns p's unit, building it on first use (announced) and
// caching the outcome (ok=false: skipped by execIf).
func (c *CheApp) ensurePlugin(p spec.PluginRef) (unit, bool, error) {
	if u, seen := c.pluginUnits[p.String()]; seen {
		if u == nil {
			return unit{}, false, nil
		}
		return *u, true, nil
	}
	log.Debug("plugin("+p.Profile+")", fmt.Sprintf("run %s", p), log.Off)
	u, ok, err := c.buildPlugin(p)
	if err != nil {
		return unit{}, false, err
	}
	if !ok {
		c.pluginUnits[p.String()] = nil
		return unit{}, false, nil
	}
	c.pluginUnits[p.String()] = &u
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

// build loads spec -> lists eligible profiles -> resolves union -> wires the
// host. Run in PersistentPreRunE before any subcommand RunE.
func (c *CheApp) build() error {
	dir := cmp.Or(c.dirFlag, os.Getenv("CHE_DIR"))
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
	c.dryRunMode = cmp.Or(c.dryRunMode, os.Getenv("CHE_DRY_RUN"))
	mode, ok := dryRunModes[c.dryRunMode]
	if !ok {
		return fmt.Errorf("invalid --dry-run mode %q: want delta or all", c.dryRunMode)
	}
	c.validateSchemaMode = cmp.Or(c.validateSchemaMode, os.Getenv("CHE_VALIDATE_SCHEMA"), "warn")
	if c.validateSchemaMode != "warn" && c.validateSchemaMode != "error" {
		return fmt.Errorf("invalid --validate-schema mode %q: want warn or error", c.validateSchemaMode)
	}
	sp, err := loadSpecValidated(filepath.Join(repoRoot, "che.yml"), c.validateSchemaMode)
	if err != nil {
		return err
	}
	// --profile (env CHE_PROFILE) runs only that profile, autoExec
	// skipped but execIf still enforced; --skip-exec-if (env
	// CHE_SKIP_EXEC_IF, truthy) makes every execIf pass; else the union of
	// every autoExec profile passing execIf. --skip-plugins (env
	// CHE_SKIP_PLUGINS, truthy) drops plugins entries, local repo only.
	// --debug (env CHE_DEBUG, truthy) prints debug-level lines.
	// Flags win over envs.
	cfg := config.Config{
		Dir:            dir,
		DryRun:         mode,
		Profile:        cmp.Or(c.profileForce, os.Getenv("CHE_PROFILE")),
		SkipExecIf:     boolOrEnv(c.skipExecIf, "CHE_SKIP_EXEC_IF"),
		SkipPlugins:    boolOrEnv(c.skipPlugins, "CHE_SKIP_PLUGINS"),
		Debug:          boolOrEnv(c.debugFlag, "CHE_DEBUG"),
		ValidateSchema: c.validateSchemaMode,
	}
	log.SetDebug(cfg.Debug)
	eval := spec.NewEvaluator().EvalExecIf
	profiles, err := sp.EligibleProfiles(cfg.Profile, cfg.SkipExecIf, eval)
	if err != nil {
		return err
	}
	h := c.newHost(repoRoot, home, strings.Join(profiles, ","), cfg)
	res, err := sp.Resolve(profiles, h.Root)
	if err != nil {
		return err
	}
	c.units = []unit{{host: h, res: res}}
	c.pluginRefs = res.Plugins
	if cfg.IsOptionEqualTo(config.OptionSkipPlugins, true) {
		c.pluginRefs = nil
	}
	c.pluginUnits = map[string]*unit{}
	c.pluginCfg = pluginConfig{repoRoot: repoRoot, home: home, cfg: cfg, eval: eval}
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

// buildPlugin ensures the plugin checkout (git ref: cache clone/pull; dir ref:
// resolved local dir), loads its spec, then inside the entry's env overlay
// gates on the remote profile's execIf (fail -> skipped, logged) and resolves
// it anchored at the checkout, all against pluginCfg. Nested plugin refs
// inside the remote profile are ignored (v1).
func (c *CheApp) buildPlugin(p spec.PluginRef) (unit, bool, error) {
	checkout, err := pluginCheckout(p, c.pluginCfg.repoRoot, c.pluginCfg.home)
	if err != nil {
		return unit{}, false, err
	}
	psp, err := loadSpecValidated(filepath.Join(checkout, "che.yml"), c.pluginCfg.cfg.ValidateSchema)
	if err != nil {
		return unit{}, false, err
	}
	h := c.newHost(checkout, c.pluginCfg.home, p.Profile, c.pluginCfg.cfg).WithLogSub("profile=" + p.Profile)
	u := unit{host: h, ref: p.String(), env: p.Env}
	var pass bool
	err = u.withEnv(func() error {
		var err error
		if pass, err = psp.ExecIfPass(p.Profile, c.pluginCfg.cfg.SkipExecIf, c.pluginCfg.eval); err != nil || !pass {
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

// pluginCheckout resolves p's repo dir: git URL -> managed cache clone/pull,
// dir path -> resolved local dir (no ensure logs, no git).
func pluginCheckout(p spec.PluginRef, repoRoot, home string) (string, error) {
	if !p.IsPath {
		return plugin.Ensure(home, p.URL, p.Profile)
	}
	return resolvePluginDir(p.URL, repoRoot, home)
}

// resolvePluginDir expands a dir-path plugin ref: $VAR, then ~ -> home, then
// relative -> joined onto the local repo root (the che.yml dir). The dir must exist.
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
