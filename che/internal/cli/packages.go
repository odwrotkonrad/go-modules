package cli

// [>] 🤖🤖🤖

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/internal/che"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
)

func (a *app) packagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packages",
		Short: "install packages from packages.yml and check their state",
	}
	pf := cmd.PersistentFlags()
	pf.StringVar(&a.flags.PackagesFile, "packages-file", "",
		"packages.yml path; default: $XDG_CONFIG_HOME/packages/packages.yml; env: CHE_PACKAGES_FILE")
	pf.StringVar(&a.flags.PackagesOverride, "packages-override", "",
		"override packages file (same-name entries replace, new names append); default: $XDG_CONFIG_HOME/che/packages-override.yml if present; env: CHE_PACKAGES_OVERRIDE")
	pf.StringSliceVar(&a.flags.PackagesPreferredMethods, "preferred-methods", nil,
		"installation-method preference order (comma-separated or repeated): listed managers try first within each package entry, unlisted follow in entry order; values: brew | cask | apt | npm | go | gem | binary | script | pkg | code; env: CHE_PACKAGES_PREFERRED_METHODS")

	install := &cobra.Command{
		Use:   "install [pkg...]",
		Short: "install packages by canonical name (no args: every resolved profile's include.installPackages)",
		RunE:  a.packagesInstallRunE,
	}
	install.Flags().BoolVar(&a.flags.PackagesUpdate, "update", false,
		"refresh installed unpinned packages via their manager; pinned ones converge on the pin regardless")
	install.Flags().BoolVar(&a.flags.PackagesIfMissing, "if-missing", false,
		"skip packages whose canonical command exists anywhere on PATH, regardless of manager")

	cmd.AddCommand(install,
		a.packagesCheckCmd("check-present", "check the canonical commands resolve on PATH (errors on any missing)", true,
			func(in *packages.Installer, pkgs []string) error {
				if missing := in.CheckPresent(pkgs); len(missing) > 0 {
					return fmt.Errorf("missing commands: %s", strings.Join(missing, ", "))
				}
				return nil
			}),
		a.packagesCheckCmd("check-upgradable", "warn on manager-reported outdated packages and binary pins drifted from --version output", false,
			func(in *packages.Installer, pkgs []string) error { return in.CheckUpgradable(pkgs) }),
		a.packagesCheckCmd("check-not-shadowed", "warn when a package's manager-expected binary is not the first PATH hit", false,
			func(in *packages.Installer, pkgs []string) error { return in.CheckNotShadowed(pkgs) }),
		a.packagesCheckCmd("check-single-present", "warn when a canonical command resolves in more than one PATH dir, listing every location", false,
			func(in *packages.Installer, pkgs []string) error { return in.CheckSinglePresent(pkgs) }))
	return cmd
}

func (a *app) profilePackages() []string {
	var out []string
	for _, p := range a.root.AllProfiles() {
		for _, op := range p.OperationsReady {
			ip, ok := op.(*che.InstallPackagesOperationReady)
			if !ok {
				continue
			}
			for _, pkg := range ip.Packages {
				if !slices.Contains(out, pkg) {
					out = append(out, pkg)
				}
			}
		}
	}
	return out
}

func (a *app) packagesInstallRunE(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		in, err := che.NewPackagesInstallerFromContext(a.ctx, a.opts)
		if err != nil {
			return err
		}
		if err := in.Install(args); err != nil {
			return err
		}
		if a.opts.DryRun == options.DryRun.Off {
			in.CheckPresent(args)
		}
		return nil
	}
	return a.root.ExecEach(a.runCtx, "install-packages", func(ctx context.Context, p *che.ProfileReady) error {
		return p.ExecOperationNamed(ctx, "install-packages")
	})
}

func (a *app) packagesCheckCmd(name, short string, takesArgs bool, run func(*packages.Installer, []string) error) *cobra.Command {
	use := name
	if takesArgs {
		use = name + " [pkg...]"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := che.NewPackagesInstallerFromContext(a.ctx, a.opts)
			if err != nil {
				return err
			}
			pkgs := args
			if len(pkgs) == 0 {
				pkgs = a.profilePackages()
			}
			if len(pkgs) == 0 {
				pkgs = slices.Sorted(maps.Keys(in.File.Packages))
			}
			return run(in, pkgs)
		},
	}
}

// [<] 🤖🤖🤖
