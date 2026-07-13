// Package cli builds che's cobra command tree over the che package's prepared specs.
package cli

// [>] 🤖🤖

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/internal/che"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

// version is injected at build time via -ldflags -X.
var version = "dev"

// app wires the cobra tree: init (PersistentPreRunE) prepares opts and the
// root SpecReady as separately initialized values, read by each RunE.
type app struct {
	flags options.Options // cobra flag destinations
	opts  options.Options // finalized by che.PrepareApplicationOptions
	root  *che.SpecReady  // prepared by che.PrepareSpecs
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
			return a.init()
		},
	}
	pf := root.PersistentFlags()
	pf.StringVarP(&a.flags.Dir, "directory", "C", "",
		"change into this directory before resolving the repo; env: CHE_DIR")
	pf.StringVar(&a.flags.WorkingDirectory, "working-directory", "",
		"the load-ops source tree (che level; spec/profile options.workingDirectory override); default root; env: CHE_WORKING_DIRECTORY")
	pf.StringVar((*string)(&a.flags.DryRun), "dry-run", "",
		"print mutating actions instead of executing them; values: delta (changed dests, bare-flag default) | all (every dest); default: off; env: CHE_DRY_RUN")
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

	services := &cobra.Command{
		Use:   "services",
		Short: "load/unload/verify the profile's launchd services",
	}
	root.AddCommand(a.allCmd(), a.discoverCmd(), services)
	for _, o := range ops() {
		cmd := a.opCmd(o)
		if o.parent == "services" {
			services.AddCommand(cmd)
		} else {
			root.AddCommand(cmd)
		}
	}
	return root
}

// init prepares the run: build the launch context from the process (the one
// ambient-read site), resolve options (flags > env vars > local che.yml
// options: > defaults), then the whole spec tree.
func (a *app) init() error {
	ctx, err := che.NewContext()
	if err != nil {
		return err
	}
	ctx, a.opts, err = che.PrepareApplicationOptions(ctx, a.flags)
	if err != nil {
		return err
	}
	a.root, err = che.PrepareSpecs(ctx, a.opts, spec.SpecSourceRecipe{})
	return err
}

// allCmd runs every profile's full op sequence, profile by profile. A failing
// profile does not stop the rest: failures collect, report, and join.
func (a *app) allCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "all",
		Short: "run every op each profile selects, profile by profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.root.ExecEach("all", (*che.ProfileReady).ExecOperations)
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
	err := a.root.ExecEach(cmd.Name(), func(p *che.ProfileReady) error {
		n, err := p.ExecRunScripts(args)
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
