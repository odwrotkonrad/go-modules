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

func (a *app) makePackagesCmd() *cobra.Command {
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
		"installation-method preference order (comma-separated or repeated): listed managers try first within each package entry, unlisted follow in entry order; values: brew | cask | apt | npm | go | gem | binariesRemoteArchive | script | pyenv | nvm; env: CHE_PACKAGES_PREFERRED_METHODS")
	pf.StringSliceVar(&a.flags.PackagesOnlyMethods, "only-methods", nil,
		"restrict installs to the listed managers (comma-separated or repeated): items using any other manager are skipped, a package with no listed manager applicable is not installed; values: brew | cask | apt | npm | go | gem | binariesRemoteArchive | script | pyenv | nvm; env: CHE_PACKAGES_ONLY_METHODS")
	pf.StringVar(&a.flags.PackagesDownloadCacheDir, "download-cache-dir", "",
		"binariesRemoteArchive download cache directory: assets download to <dir>/<sha256(url)>-<basename> and later installs reuse the file, a checksum mismatch evicts it; empty disables caching; env: CHE_PACKAGES_DOWNLOAD_CACHE_DIR")

	install := &cobra.Command{
		Use:   "install [pkg...]",
		Short: "install packages by canonical name (no args: every resolved profile's include.installPackages)",
		RunE:  a.installPackagesRunE,
	}
	install.Flags().BoolVar(&a.flags.PackagesUpdate, "update", false,
		"refresh installed unpinned packages via their manager; pinned ones converge on the pin regardless")
	install.Flags().BoolVar(&a.flags.PackagesIfMissing, "if-missing", false,
		"skip packages whose canonical command exists anywhere on PATH, regardless of manager")
	install.Flags().BoolVar(&a.flags.PackagesMissingMethodWarn, "missing-method-warn", false,
		"warn instead of erroring when a requested package has no applicable installation method on this host")
	install.Flags().StringVar(&a.flags.PackagesSilenceInstallStdout, "silence-install-stdout", "",
		"silence the installation method's stdout/stderr (a failing method's captured output always prints); values: true (bare-flag default) | false (stream as it runs); default: true at error/warn/info log level, false at debug/trace; env: CHE_PACKAGES_SILENCE_INSTALL_STDOUT")
	install.Flags().Lookup("silence-install-stdout").NoOptDefVal = "true"
	addPackagesKindFlag(install, &a.packagesKind)

	cmd.AddCommand(install,
		a.makeCheckPresentCmd(),
		a.makeCheckCmd("check-upgradable", "warn on manager-reported outdated packages and binary pins drifted from --version output",
			func(in *packages.Installer, pkgs []string) error { return in.CheckUpgradable(pkgs) }),
		a.makeCheckCmd("check-not-shadowed", "warn when a package's manager-expected binary is not the first PATH hit",
			func(in *packages.Installer, pkgs []string) error { return in.CheckNotShadowed(pkgs) }),
		a.makeCheckCmd("check-single-present", "warn when a canonical command resolves in more than one PATH dir, listing every location",
			func(in *packages.Installer, pkgs []string) error { return in.CheckSinglePresent(pkgs) }),
		a.makePackagesConfigCmd())
	return cmd
}

func (a *app) installPackagesRunE(cmd *cobra.Command, args []string) error {
	if err := validatePackagesKind(a.packagesKind); err != nil {
		return err
	}
	if len(args) > 0 {
		in, err := che.NewPackagesInstallerFromContext(a.ctx, a.opts)
		if err != nil {
			return err
		}
		if a.packagesKind != "packages" {
			if err := in.InstallToolPackages(a.packagesKind, packages.Requests(args)); err != nil {
				return err
			}
			if a.opts.DryRun == options.DryRun.Off {
				in.CheckToolPackagesPresent(a.packagesKind, args)
			}
			return nil
		}
		if err := in.Install(args); err != nil {
			return err
		}
		if a.opts.DryRun == options.DryRun.Off {
			in.CheckPresent(args)
		}
		return nil
	}
	return a.specs.ExecEach(a.runCtx, "install-packages", func(ctx context.Context, p *che.ProfileReady) error {
		return p.ExecOperationNamed(ctx, "install-packages")
	})
}

func (a *app) makeCheckPresentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-present [pkg...]",
		Short: "check the canonical commands resolve on PATH (errors on any missing); --kind=<tool> checks the tool's installed packages instead",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePackagesKind(a.packagesKind); err != nil {
				return err
			}
			in, err := che.NewPackagesInstallerFromContext(a.ctx, a.opts)
			if err != nil {
				return err
			}
			if a.packagesKind != "packages" {
				names := a.resolveToolPkgs(in, a.packagesKind, args)
				if missing := in.CheckToolPackagesPresent(a.packagesKind, names); len(missing) > 0 {
					return fmt.Errorf("missing %s packages: %s", a.packagesKind, strings.Join(missing, ", "))
				}
				return nil
			}
			if missing := in.CheckPresent(a.resolvePkgs(in, args)); len(missing) > 0 {
				return fmt.Errorf("missing commands: %s", strings.Join(missing, ", "))
			}
			return nil
		},
	}
	addPackagesKindFlag(cmd, &a.packagesKind)
	return cmd
}

func (a *app) makeCheckCmd(name, short string, run func(*packages.Installer, []string) error) *cobra.Command {
	return &cobra.Command{
		Use:   name + " [pkg...]",
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := che.NewPackagesInstallerFromContext(a.ctx, a.opts)
			if err != nil {
				return err
			}
			return run(in, a.resolvePkgs(in, args))
		},
	}
}

func (a *app) makePackagesConfigCmd() *cobra.Command {
	return makeConfigShowCmd(
		"inspect the resolved packages database",
		"print the packages database (--all default: the effective merged set; --delta: entries differing from the builtin; --defaults: the builtin only)",
		showHelp{
			delta:    "print only the entries differing from the builtin packages.yml",
			all:      "print the effective merged set (packages file or builtin, plus override; default mode)",
			defaults: "print the builtin packages.yml only",
			output:   "output format; values: text (name = methods lines) | yaml (packages.yml shape)",
		},
		"all",
		func(mode, output string) error {
			builtin, err := packages.LoadBuiltin()
			if err != nil {
				return err
			}
			file := builtin
			if mode != "defaults" {
				in, err := che.NewPackagesInstallerFromContext(a.ctx, a.opts)
				if err != nil {
					return err
				}
				file = in.File
				if mode == "delta" {
					file = file.Delta(builtin)
				}
			}
			return emitShowOutput(output, func() {
				for _, name := range slices.Sorted(maps.Keys(file.Packages)) {
					var mgrs []string
					for _, it := range file.Packages[name].Items {
						mgrs = append(mgrs, it.Mgr)
					}
					fmt.Printf("%s = %s\n", name, strings.Join(mgrs, ", "))
				}
				for _, tool := range slices.Sorted(maps.Keys(file.ToolPackages)) {
					set := file.ToolPackages[tool]
					for _, name := range slices.Sorted(maps.Keys(set)) {
						version := set[name]
						if version == "" {
							version = "rolling"
						}
						fmt.Printf("%s/%s = %s\n", tool, name, version)
					}
				}
			}, file.YAML)
		})
}

func (a *app) resolvePkgs(in *packages.Installer, args []string) []string {
	if len(args) > 0 {
		return args
	}
	if pkgs := che.InstallPackageNames(a.specs.AllProfiles()); len(pkgs) > 0 {
		return pkgs
	}
	return slices.Sorted(maps.Keys(in.File.Packages))
}

func (a *app) resolveToolPkgs(in *packages.Installer, tool string, args []string) []string {
	if len(args) > 0 {
		return args
	}
	if names := che.InstallToolPackageNames(a.specs.AllProfiles(), tool); len(names) > 0 {
		return names
	}
	return slices.Sorted(maps.Keys(in.File.ToolPackages[tool]))
}

func addPackagesKindFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVar(dest, "kind", "packages",
		"package kind: packages (canonical entries) or a tool name whose toolPackages entries install via that tool; values: packages | "+strings.Join(packages.KnownTools(), " | "))
}

func validatePackagesKind(kind string) error {
	if kind == "packages" || slices.Contains(packages.KnownTools(), kind) {
		return nil
	}
	return fmt.Errorf("unknown kind %q: want packages or one of %s", kind, strings.Join(packages.KnownTools(), ", "))
}

// [<] 🤖🤖🤖
