package che

// [>] 🤖🤖🤖

import (
	"cmp"
	"os"
	"path/filepath"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
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
	if opts.PackagesFile == packages.BuiltinSentinel {
		f, err := packages.LoadBuiltin()
		return f, packages.BuiltinPath, err
	}
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
	if err := f.ValidatePlatforms(); err != nil {
		return nil, err
	}
	return &packages.Installer{
		File: f, FilePath: path, Host: NewPackagesHost(),
		Opts: packages.Options{
			Update:                               opts.PackagesUpdate,
			IfMissing:                            opts.PackagesIfMissing,
			DryRun:                               opts.DryRun != options.DryRun.Off,
			PreferredMethods:                     opts.PackagesPreferredMethods,
			PrebuiltArchiveDestinationCandidates: opts.PackagesPrebuiltArchiveDestinationCandidates,
			PrebuiltArchiveCheckInPath:           opts.PackagesPrebuiltArchiveCheckInPath,
			CompletionsEnabled:                   opts.PackagesCompletionsEnabled,
			CompletionsDestinationCandidates:     opts.PackagesCompletionsDestinationCandidates,
			CompletionsCheckInFpath:              opts.PackagesCompletionsCheckInFpath,
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
	if d := p.Options.Packages.PrebuiltArchive.InstallDestinationCandidates; len(d) > 0 {
		opts.PackagesPrebuiltArchiveDestinationCandidates = d
	}
	if c := p.Options.Packages.PrebuiltArchive.CheckInPath; c != nil {
		opts.PackagesPrebuiltArchiveCheckInPath = *c
	}
	if e := p.Options.Packages.Completions.Zsh.Enabled; e != nil {
		opts.PackagesCompletionsEnabled = *e
	}
	if d := p.Options.Packages.Completions.Zsh.InstallDestinationCandidates; len(d) > 0 {
		opts.PackagesCompletionsDestinationCandidates = d
	}
	if c := p.Options.Packages.Completions.Zsh.CheckInFpath; c != nil {
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

func (p *ProfileReady) installPackages(refs []spec.PackageRef) error {
	in, err := p.newInstaller()
	if err != nil {
		return err
	}
	reqs := make([]packages.Request, len(refs))
	for i, r := range refs {
		reqs[i] = packages.Request{Name: r.Name, Versions: r.Versions, Global: r.GlobalVersion}
	}
	if err := in.InstallRequests(reqs); err != nil {
		return err
	}
	if !p.isDryRun() {
		in.CheckPresent(packages.RequestNames(reqs))
	}
	return nil
}

// [<] 🤖🤖🤖
