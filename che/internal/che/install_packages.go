package che

// [>] 🤖🤖🤖

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

func resolveConfigBase(env map[string]string, home string) string {
	return cmp.Or(env["XDG_CONFIG_HOME"], filepath.Join(home, ".config"))
}

func resolvePackagesFile(env map[string]string, home string, opts options.Options) string {
	if opts.PackagesFile != "" {
		return opts.PackagesFile
	}
	return filepath.Join(resolveConfigBase(env, home), "packages", "packages.yml")
}

func resolvePackagesOverride(env map[string]string, home string, opts options.Options) string {
	if opts.PackagesOverride != "" {
		return opts.PackagesOverride
	}
	path := filepath.Join(resolveConfigBase(env, home), "che", "packages-override.yml")
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
		if abs, err := filepath.Abs(override); err == nil {
			override = abs
		}
		o, err := packages.Load(override)
		if err != nil {
			return nil, err
		}
		o.ResolveScriptPaths(filepath.Dir(override))
		f.Merge(o)
	}
	if err := f.ValidatePlatforms(); err != nil {
		return nil, err
	}
	return &packages.Installer{
		File: f, FilePath: path, Host: NewPackagesHost(),
		Opts: packages.Options{
			Update:               opts.PackagesUpdate,
			IfMissing:            opts.PackagesIfMissing,
			MissingMethodWarn:    opts.PackagesMissingMethodWarn,
			DryRun:               opts.DryRun != options.DryRun.Off,
			PreferredMethods:     opts.PackagesPreferredMethods,
			OnlyMethods:          opts.PackagesOnlyMethods,
			DownloadCacheDir:     opts.PackagesDownloadCacheDir,
			SilenceInstallStdout: opts.PackagesSilenceInstallStdout == "true",
			BinariesRemoteArchiveDestinationCandidates: opts.PackagesBinariesRemoteArchiveDestinationCandidates,
			BinariesRemoteArchiveCheckPresentOnPath:    opts.PackagesBinariesRemoteArchiveCheckPresentOnPath,
			CompletionsEnabled:                         opts.PackagesCompletionsEnabled,
			CompletionsDestinationCandidates:           opts.PackagesCompletionsDestinationCandidates,
			CompletionsCheckPresentOnFpath:             opts.PackagesCompletionsCheckPresentOnFpath,
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
	if d := p.Options.Packages.BinariesRemoteArchive.InstallDestinationCandidates; len(d) > 0 {
		opts.PackagesBinariesRemoteArchiveDestinationCandidates = d
	}
	if c := p.Options.Packages.BinariesRemoteArchive.CheckPresentOnPath; c != nil {
		opts.PackagesBinariesRemoteArchiveCheckPresentOnPath = *c
	}
	if e := p.Options.Packages.Completions.Zsh.Enabled; e != nil {
		opts.PackagesCompletionsEnabled = *e
	}
	if d := p.Options.Packages.Completions.Zsh.InstallDestinationCandidates; len(d) > 0 {
		opts.PackagesCompletionsDestinationCandidates = d
	}
	if c := p.Options.Packages.Completions.Zsh.CheckPresentOnFpath; c != nil {
		opts.PackagesCompletionsCheckPresentOnFpath = *c
	}
	in, err := NewPackagesInstaller(p.env, p.home, opts)
	if err != nil {
		return nil, err
	}
	in.Emit = func(level log.Level, action, msg string) { p.emit(level, packages.Scope, action, msg) }
	in.EmitSkip = func(level log.Level, action, msg string, reasons ...string) {
		p.emit(level, packages.Scope, action, msg, reasons...)
	}
	in.EmitDryRun = func(action, msg string) { p.emitDryRun(packages.Scope, action, msg) }
	return in, nil
}

func InstallPackageNames(profiles []*ProfileReady) []string {
	var out []string
	for _, p := range profiles {
		for _, op := range p.OperationsReady {
			ip, ok := op.(*InstallPackagesOperationReady)
			if !ok {
				continue
			}
			for _, pkg := range ip.Packages {
				if !slices.Contains(out, pkg.Name) {
					out = append(out, pkg.Name)
				}
			}
		}
	}
	return out
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
