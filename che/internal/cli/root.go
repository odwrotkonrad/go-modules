package cli

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go/che/internal/fsutil"
	"gitlab.com/konradodwrot/go/che/internal/host"
	"gitlab.com/konradodwrot/go/che/internal/spec"
)

// Built once in PersistentPreRunE, read by each RunE.
var (
	dryRunMode string
	theHost    host.Host
	resolved   spec.Resolved
)

// version is injected at build time via -ldflags -X.
var version = "dev"

// dryRunModes maps the --dry-run flag value to a host.DryRunMode.
var dryRunModes = map[string]host.DryRunMode{
	"":      host.DryRunOff,
	"delta": host.DryRunDelta,
	"all":   host.DryRunAll,
}

// RootCmd is che's root command. Resolves the profile (build) before any
// subcommand runs. Subcommands attached by the command package.
var RootCmd = &cobra.Command{
	Use:     "che",
	Version: version,
	Short:   "Spec-driven config loader",
	Long: `che detects OS+arch+virt -> profile in che.yml, then loads only the
files/dirs/installs/services that profile selects.`,
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
}

// build detects -> loads spec -> resolves -> wires the host. Run in
// PersistentPreRunE before any subcommand RunE.
func build() error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	home, err := invokingHome()
	if err != nil {
		return err
	}
	// CHE_FORCE_PROFILE overrides detection (test/VM hook).
	profile := os.Getenv("CHE_FORCE_PROFILE")
	if profile == "" {
		profile = fsutil.DetectProfile()
	}
	if dryRunMode == "" {
		dryRunMode = os.Getenv("CHE_DRY_RUN")
	}
	mode, ok := dryRunModes[dryRunMode]
	if !ok {
		return fmt.Errorf("invalid --dry-run mode %q: want delta or all", dryRunMode)
	}
	h := host.New(repoRoot, home, profile, mode)
	sp, err := spec.Load(filepath.Join(repoRoot, "che.yml"))
	if err != nil {
		return err
	}
	res, err := sp.Resolve(profile, h.Root)
	if err != nil {
		return err
	}
	theHost = h
	resolved = res
	return nil
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
