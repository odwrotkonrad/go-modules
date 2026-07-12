// Package cli builds che's cobra command tree and orchestrates ops across the local repo and plugin checkouts.
package cli

// [>] 🤖🤖

import (
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
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/plugin"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

// version is injected at build time via -ldflags -X.
var version = "dev"

// Root builds che's root command with every subcommand attached: the single
// command-tree source for main and docgen.
func (ld *loader) Root() *cobra.Command {
	root := &cobra.Command{
		Use:     "che",
		Version: version,
		Short:   "Spec-driven config loader",
		Long: `che resolves every eligible profile in che.yml (execIf predicates), then
loads the union of files/dirs/installs/services those profiles select.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return ld.init()
		},
	}
	pf := root.PersistentFlags()
	pf.StringVarP(&ld.config.Dir, "directory", "C", "",
		"change into this directory before resolving the repo; env: CHE_DIR")
	pf.StringVar((*string)(&ld.config.DryRun), "dry-run", "",
		"print mutating actions instead of executing them; values: delta (changed dests, bare-flag default) | all (every dest); default: off; env: CHE_DRY_RUN")
	pf.Lookup("dry-run").NoOptDefVal = "delta"
	pf.StringVar((*string)(&ld.config.ValidateSpec), "validate-spec", "",
		"validate each loaded che.yml spec against the JSON Schema; values: warn (log violations) | error (abort on violations); default: warn; env: CHE_VALIDATE_SPEC")
	pf.StringVar(&ld.config.Profile, "profile", "",
		"run only this profile (autoDiscover skipped, execIf still enforced); env: CHE_PROFILE")
	pf.BoolVar(&ld.config.SkipExecIf, "skip-exec-if", false,
		"treat every execIf predicate as passing; env: CHE_SKIP_EXEC_IF")
	pf.BoolVar(&ld.config.SkipPlugins, "skip-plugins", false,
		"skip plugins entries, load only the local repo; env: CHE_SKIP_PLUGINS")
	pf.BoolVar(&ld.config.Debug, "debug", false,
		"print debug-level lines (plugin announce, clone/pull attempts); env: CHE_DEBUG")

	services := &cobra.Command{
		Use:   "services",
		Short: "load/unload/verify the profile's launchd services",
	}
	root.AddCommand(ld.allCmd(), ld.detectCmd(), services)
	for _, s := range steps() {
		cmd := ld.stepCmd(s)
		if s.parent == "services" {
			services.AddCommand(cmd)
		} else {
			root.AddCommand(cmd)
		}
	}
	return root
}

// init finalizes config, loads the local repo (spec, eligible
// profiles, host, selection), and constructs the plugin loader.
func (ld *loader) init() error {
	if err := ld.config.Resolve(); err != nil {
		return err
	}
	cfg := ld.config
	if cfg.Dir != "" {
		if err := os.Chdir(cfg.Dir); err != nil {
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
	sp, err := loadSpecValidated(filepath.Join(repoRoot, "che.yml"), cfg.ValidateSpec)
	if err != nil {
		return err
	}
	log.SetDebug(cfg.Debug)
	eval := spec.NewEvaluator().EvalExecIf
	profiles, err := sp.EligibleProfiles(cfg.Profile, cfg.SkipExecIf, eval)
	if err != nil {
		return err
	}
	h := ld.newHost(repoRoot, home, strings.Join(profiles, ","), cfg)
	selection, err := sp.Resolve(profiles, h.Root)
	if err != nil {
		return err
	}
	ld.local = load{host: h, selection: selection}
	refs := selection.Plugins
	if cfg.SkipPlugins {
		refs = nil
	}
	ld.plugins = pluginLoader{
		refs:     refs,
		built:    map[string]*load{},
		repoRoot: repoRoot,
		home:     home,
		cfg:      cfg,
		eval:     eval,
		newHost:  ld.newHost,
	}
	return nil
}

// forEachLoad runs op over the local load, then each plugin load (built on
// first use, execIf-skipped ones dropped). A failing load does not stop the
// rest: failures collect (ref-wrapped, "local" for the local repo), report as
// "<name>(report): fail <ref>: <err>" lines after all loads, and join into
// the returned error.
func (ld *loader) forEachLoad(name string, op func(load) error) error {
	var fails []error
	run := func(ref string, l load) {
		if err := l.runWithPluginEnv(func() error { return op(l) }); err != nil {
			fails = append(fails, fmt.Errorf("%s: %w", ref, err))
		}
	}
	run("local", ld.local)
	for _, p := range ld.plugins.refs {
		l, ok, err := ld.plugins.ensure(p)
		switch {
		case err != nil:
			fails = append(fails, fmt.Errorf("%s: %w", p, err))
		case ok:
			run(p.String(), l)
		}
	}
	for _, err := range fails {
		log.Msg(name+"(report)", "fail "+err.Error(), log.Off)
	}
	return errors.Join(fails...)
}

// runWithPluginEnv runs fn with l.pluginEnv exported (host values shadowed),
// restoring after.
func (l load) runWithPluginEnv(fn func() error) error {
	if len(l.pluginEnv) == 0 {
		return fn()
	}
	for _, k := range slices.Sorted(maps.Keys(l.pluginEnv)) {
		prev, had := os.LookupEnv(k)
		if err := os.Setenv(k, l.pluginEnv[k]); err != nil {
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

// ensure returns p's load, building it on first use (announced) and caching
// the outcome (ok=false: skipped by execIf).
func (pl *pluginLoader) ensure(p spec.PluginRef) (load, bool, error) {
	if l, seen := pl.built[p.String()]; seen {
		if l == nil {
			return load{}, false, nil
		}
		return *l, true, nil
	}
	log.Debug("plugin("+p.Profile+")", fmt.Sprintf("run %s", p), log.Off)
	l, ok, err := pl.build(p)
	if err != nil {
		return load{}, false, err
	}
	if !ok {
		pl.built[p.String()] = nil
		return load{}, false, nil
	}
	pl.built[p.String()] = &l
	return l, true, nil
}

// build ensures the checkout (git ref: cache clone/pull, dir ref: local dir),
// loads its spec, gates on execIf inside the entry's env overlay, resolves
// anchored at the checkout. Nested plugin refs are ignored (v1).
func (pl *pluginLoader) build(p spec.PluginRef) (load, bool, error) {
	checkout, err := pluginCheckout(p, pl.repoRoot, pl.home)
	if err != nil {
		return load{}, false, err
	}
	psp, err := loadSpecValidated(filepath.Join(checkout, "che.yml"), pl.cfg.ValidateSpec)
	if err != nil {
		return load{}, false, err
	}
	h := pl.newHost(checkout, pl.home, p.Profile, pl.cfg).WithLogSub("profile=" + p.Profile)
	l := load{host: h, pluginRef: p.String(), pluginEnv: p.Env}
	var pass bool
	err = l.runWithPluginEnv(func() error {
		var err error
		if pass, err = psp.ExecIfPass(p.Profile, pl.cfg.SkipExecIf, pl.eval); err != nil || !pass {
			return err
		}
		l.selection, err = psp.Resolve([]string{p.Profile}, l.host.Root)
		return err
	})
	if err != nil {
		return load{}, false, fmt.Errorf("plugin %s: %w", p, err)
	}
	if !pass {
		log.Msg("plugin("+p.Profile+")", fmt.Sprintf("skip %s (execIf failed)", p), log.Off)
		return load{}, false, nil
	}
	if len(l.selection.Plugins) > 0 {
		log.Msg("plugin("+p.Profile+")", fmt.Sprintf("%s: nested plugin refs ignored: %v", p, l.selection.Plugins), log.Off)
	}
	return l, true, nil
}

func loadSpecValidated(path string, mode config.ValidateSpecMode) (*spec.CheSpec, error) {
	if b, err := os.ReadFile(path); err == nil {
		if finds := spec.ValidateSchema(b); len(finds) > 0 {
			if mode == config.ValidateSpec.Error {
				return nil, fmt.Errorf("schema violations in %s:\n%s", path, strings.Join(finds, "\n"))
			}
			for _, f := range finds {
				log.Msg("validate(che.yml)", f, log.Off)
			}
		}
	}
	return spec.Load(path)
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
