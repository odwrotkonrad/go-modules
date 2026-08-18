package che

// [>] 🤖🤖🤖

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

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
			if dir, _, ok := packages.ResolveCurrentDefinitions(packages.ResolveDefinitionsCacheDir(env, home)); ok {
				cached := filepath.Join(dir, "packages.yml")
				if f, err := packages.Load(cached); err == nil {
					return f, cached, nil
				}
			}
			f, err := packages.LoadBuiltin()
			return f, packages.BuiltinPath, err
		}
	}
	f, err := packages.Load(path)
	return f, path, err
}

func checkDefinitionsUpdate(env map[string]string, home string, opts options.Options) error {
	if !opts.PackagesUpdateCheckEnabled {
		return nil
	}
	cooldown, err := time.ParseDuration(opts.PackagesUpdateCheckCooldown)
	if err != nil {
		cooldown = packages.DefaultUpdateCooldown
	}
	cacheDir := packages.ResolveDefinitionsCacheDir(env, home)
	res, err := packages.UpdateDefinitions(cacheDir, packages.ResolveUpdateBaseURL(env), cooldown, false)
	switch {
	case err != nil && opts.DryRun != options.DryRun.Off:
		return fmt.Errorf("packages definitions update failed: %w", err)
	case err != nil:
		log.Emit(log.Event{Level: log.Levels.Warn, Scope: packages.Scope, Action: "update-check", Msg: "definitions update failed, using current set: " + err.Error()})
	case res.Updated:
		log.Emit(log.Event{Level: log.Levels.Info, Scope: packages.Scope, Action: "update-check", Msg: "definitions updated to " + res.Version})
	}
	return nil
}

func NewPackagesInstaller(env map[string]string, home string, opts options.Options) (*packages.Installer, error) {
	if err := checkDefinitionsUpdate(env, home, opts); err != nil {
		return nil, err
	}
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
			ManpagesDestinationCandidates:              opts.PackagesManpagesDestinationCandidates,
			ManpagesCheckPresentOnManpath:              opts.PackagesManpagesCheckPresentOnManpath,
		},
	}, nil
}

func UpdatePackagesDefinitions(ctx Context, opts options.Options, force bool) (packages.UpdateResult, error) {
	home, err := resolveInvokingHome(ctx)
	if err != nil {
		return packages.UpdateResult{}, err
	}
	cooldown, err := time.ParseDuration(opts.PackagesUpdateCheckCooldown)
	if err != nil {
		cooldown = packages.DefaultUpdateCooldown
	}
	cacheDir := packages.ResolveDefinitionsCacheDir(ctx.Env, home)
	return packages.UpdateDefinitions(cacheDir, packages.ResolveUpdateBaseURL(ctx.Env), cooldown, force)
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
	if m := p.Options.Packages.OnlyInstallationMethods; len(m) > 0 {
		if err := packages.ValidateManagers(m); err != nil {
			return nil, err
		}
		opts.PackagesOnlyMethods = m
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
	if d := p.Options.Packages.Manpages.InstallDestinationCandidates; len(d) > 0 {
		opts.PackagesManpagesDestinationCandidates = d
	}
	if c := p.Options.Packages.Manpages.CheckPresentOnManpath; c != nil {
		opts.PackagesManpagesCheckPresentOnManpath = *c
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
		reqs[i] = packages.Request{Name: r.Name, Versions: r.Versions, Global: r.GlobalVersion, Checksum: r.Checksum}
	}
	if err := in.InstallRequests(reqs); err != nil {
		return err
	}
	if !p.isDryRun() {
		in.CheckPresent(packages.RequestNames(reqs))
	}
	return nil
}

func (p *ProfileReady) installToolPackages(tools map[string][]spec.ToolPackageRef) error {
	if len(tools) == 0 {
		return nil
	}
	in, err := p.newInstaller()
	if err != nil {
		return err
	}
	for _, tool := range slices.Sorted(maps.Keys(tools)) {
		refs := tools[tool]
		if len(refs) == 0 {
			continue
		}
		reqs := make([]packages.Request, len(refs))
		for i, r := range refs {
			reqs[i] = packages.Request{Name: r.Name}
			if r.Version != "" {
				reqs[i].Versions = []string{r.Version}
			}
		}
		if err := in.InstallToolPackages(tool, reqs); err != nil {
			return err
		}
		if !p.isDryRun() {
			in.CheckToolPackagesPresent(tool, packages.RequestNames(reqs))
		}
	}
	return nil
}

func InstallToolPackageNames(profiles []*ProfileReady, tool string) []string {
	var out []string
	for _, p := range profiles {
		for _, op := range p.OperationsReady {
			ip, ok := op.(*InstallPackagesOperationReady)
			if !ok {
				continue
			}
			for _, ref := range ip.ToolPackages[tool] {
				if !slices.Contains(out, ref.Name) {
					out = append(out, ref.Name)
				}
			}
		}
	}
	return out
}

// [<] 🤖🤖🤖
