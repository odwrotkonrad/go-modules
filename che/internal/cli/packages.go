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
		"packages.yml path, fully superseding the builtin packages.yml shipped in che (a set file must exist; the builtin serves only when no file exists at the default path); default: $XDG_CONFIG_HOME/packages/packages.yml; env: CHE_PACKAGES_FILE")
	pf.StringVar(&a.flags.PackagesOverride, "packages-override", "",
		"override packages file merged over the effective base (the packages file, or the builtin when none exists): same-name entries replace, new names append; default: $XDG_CONFIG_HOME/che/packages-override.yml if present; env: CHE_PACKAGES_OVERRIDE")
	pf.StringSliceVar(&a.flags.PackagesPreferredMethods, "preferred-methods", nil,
		"installation-method preference order (comma-separated or repeated): listed managers try first within each package entry, unlisted follow in entry order; values: brew | cask | apt | npm | go | gem | prebuiltBinariesArchive | script | vscode | versionManager; env: CHE_PACKAGES_PREFERRED_METHODS")

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
		a.packagesCheckCmd("check-present", "check the canonical commands resolve on PATH (errors on any missing)",
			func(in *packages.Installer, pkgs []string) error {
				if missing := in.CheckPresent(pkgs); len(missing) > 0 {
					return fmt.Errorf("missing commands: %s", strings.Join(missing, ", "))
				}
				return nil
			}),
		a.packagesCheckCmd("check-upgradable", "warn on manager-reported outdated packages and binary pins drifted from --version output",
			func(in *packages.Installer, pkgs []string) error { return in.CheckUpgradable(pkgs) }),
		a.packagesCheckCmd("check-not-shadowed", "warn when a package's manager-expected binary is not the first PATH hit",
			func(in *packages.Installer, pkgs []string) error { return in.CheckNotShadowed(pkgs) }),
		a.packagesCheckCmd("check-single-present", "warn when a canonical command resolves in more than one PATH dir, listing every location",
			func(in *packages.Installer, pkgs []string) error { return in.CheckSinglePresent(pkgs) }),
		a.packagesConfigCmd())
	return cmd
}

func (a *app) packagesConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "inspect the resolved packages database",
	}
	var delta, all, defaults bool
	var output string
	show := &cobra.Command{
		Use:   "show",
		Short: "print the packages database (--all default: the effective merged set; --delta: entries differing from the builtin; --defaults: the builtin only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			builtin, err := packages.LoadBuiltin()
			if err != nil {
				return err
			}
			file := builtin
			if !defaults {
				in, err := che.NewPackagesInstallerFromContext(a.ctx, a.opts)
				if err != nil {
					return err
				}
				file = in.File
				if delta {
					file = file.Delta(builtin)
				}
			}
			switch output {
			case "", "text":
				for _, name := range slices.Sorted(maps.Keys(file.Packages)) {
					var mgrs []string
					for _, it := range file.Packages[name].Items {
						mgrs = append(mgrs, it.Mgr)
					}
					fmt.Printf("%s = %s\n", name, strings.Join(mgrs, ", "))
				}
				return nil
			case "yaml":
				out, err := file.YAML()
				if err != nil {
					return err
				}
				fmt.Print(out)
				return nil
			default:
				return fmt.Errorf("invalid --output %q: want text or yaml", output)
			}
		},
	}
	show.Flags().BoolVar(&delta, "delta", false, "print only the entries differing from the builtin packages.yml")
	show.Flags().BoolVar(&all, "all", false, "print the effective merged set (packages file or builtin, plus override; default mode)")
	show.Flags().BoolVar(&defaults, "defaults", false, "print the builtin packages.yml only")
	show.Flags().StringVar(&output, "output", "text",
		"output format; values: text (name = methods lines) | yaml (packages.yml shape)")
	show.MarkFlagsMutuallyExclusive("delta", "all", "defaults")
	cmd.AddCommand(show)
	return cmd
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

func (a *app) packagesCheckCmd(name, short string, run func(*packages.Installer, []string) error) *cobra.Command {
	use := name + " [pkg...]"
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
				pkgs = che.InstallPackageNames(a.root.AllProfiles())
			}
			if len(pkgs) == 0 {
				pkgs = slices.Sorted(maps.Keys(in.File.Packages))
			}
			return run(in, pkgs)
		},
	}
}

// [<] 🤖🤖🤖
