package che

// [>] 🤖🤖🤖

import (
	"cmp"
	"os"
	"path/filepath"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
)

func resolvePackagesFile(env map[string]string, home string, opts options.Options) string {
	if opts.PackagesFile != "" {
		return opts.PackagesFile
	}
	base := cmp.Or(env["XDG_CONFIG_HOME"], filepath.Join(home, ".config"))
	return filepath.Join(base, "packages", "packages.yml")
}

func resolvePackagesOverride(env map[string]string, home string, opts options.Options) string {
	if opts.PackagesOverride != "" {
		return opts.PackagesOverride
	}
	base := cmp.Or(env["XDG_CONFIG_HOME"], filepath.Join(home, ".config"))
	path := filepath.Join(base, "che", "packages-override.yml")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

var NewPackagesHost = packages.NewHost

func loadPackagesFile(env map[string]string, home string, opts options.Options) (*packages.File, string, error) {
	path := resolvePackagesFile(env, home, opts)
	if opts.PackagesFile == "" {
		if _, err := os.Stat(path); err != nil {
			f, err := packages.LoadBuiltin()
			return f, packages.BuiltinPath, err
		}
	}
	f, err := packages.Load(path)
	return f, path, err
}

func NewPackagesInstaller(env map[string]string, home string, opts options.Options) (*packages.Installer, error) {
	f, path, err := loadPackagesFile(env, home, opts)
	if err != nil {
		return nil, err
	}
	if override := resolvePackagesOverride(env, home, opts); override != "" {
		o, err := packages.Load(override)
		if err != nil {
			return nil, err
		}
		f.Merge(o)
	}
	return &packages.Installer{
		File: f, FilePath: path, Host: NewPackagesHost(),
		Opts: packages.Options{
			Update:                           opts.PackagesUpdate,
			IfMissing:                        opts.PackagesIfMissing,
			DryRun:                           opts.DryRun != options.DryRun.Off,
			PreferredMethods:                 opts.PackagesPreferredMethods,
			BinaryDestinationCandidates:      opts.PackagesBinaryDestinationCandidates,
			BinaryCheckInPath:                opts.PackagesBinaryCheckInPath,
			CompletionsEnabled:               opts.PackagesCompletionsEnabled,
			CompletionsPackages:              opts.PackagesCompletionsPackages,
			CompletionsDestinationCandidates: opts.PackagesCompletionsDestinationCandidates,
			CompletionsCheckInFpath:          opts.PackagesCompletionsCheckInFpath,
		},
	}, nil
}

func NewPackagesInstallerFromContext(ctx Context, opts options.Options) (*packages.Installer, error) {
	home, err := resolveInvokingHome(ctx)
	if err != nil {
		return nil, err
	}
	return NewPackagesInstaller(ctx.Env, home, opts)
}

func (p *ProfileReady) newInstaller() (*packages.Installer, error) {
	opts := p.opts
	if f := p.Options.Packages.File; f != "" {
		opts.PackagesFile = f
	}
	if m := p.Options.Packages.PreferredInstallationMethods; len(m) > 0 {
		if err := packages.ValidateManagers(m); err != nil {
			return nil, err
		}
		opts.PackagesPreferredMethods = m
	}
	if d := p.Options.Packages.Binary.InstallDestinationCandidates; len(d) > 0 {
		opts.PackagesBinaryDestinationCandidates = d
	}
	if c := p.Options.Packages.Binary.CheckInPath; c != nil {
		opts.PackagesBinaryCheckInPath = *c
	}
	if e := p.Options.Packages.Completions.Enabled; e != nil {
		opts.PackagesCompletionsEnabled = *e
	}
	if l := p.Options.Packages.Completions.Packages; len(l) > 0 {
		opts.PackagesCompletionsPackages = l
	}
	if d := p.Options.Packages.Completions.InstallDestinationCandidates; len(d) > 0 {
		opts.PackagesCompletionsDestinationCandidates = d
	}
	if c := p.Options.Packages.Completions.CheckInFpath; c != nil {
		opts.PackagesCompletionsCheckInFpath = *c
	}
	in, err := NewPackagesInstaller(p.env, p.home, opts)
	if err != nil {
		return nil, err
	}
	in.Emit = func(level log.Level, action, msg string) { p.emit(level, packages.Scope, action, msg) }
	in.EmitSkip = func(level log.Level, action, msg string, reasons ...string) {
		p.emitSkip(level, packages.Scope, action, msg, reasons...)
	}
	in.EmitDryRun = func(action, msg string) { p.emitDryRun(packages.Scope, action, msg) }
	return in, nil
}

func (p *ProfileReady) installPackages(pkgs []string) error {
	in, err := p.newInstaller()
	if err != nil {
		return err
	}
	if err := in.Install(pkgs); err != nil {
		return err
	}
	if !p.isDryRun() {
		in.CheckPresent(pkgs)
	}
	return nil
}

// [<] 🤖🤖🤖
