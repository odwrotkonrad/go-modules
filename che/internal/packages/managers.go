package packages

// [>] 🤖🤖🤖

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	chepackages "gitlab.com/konradodwrot/go-modules/che-packages"
	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/fetchx"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

const Scope = "install-packages"

type Options struct {
	Update                                     bool
	IfMissing                                  bool
	MissingMethodWarn                          bool
	DryRun                                     bool
	PreferredMethods                           []string
	OnlyMethods                                []string
	DownloadCacheDir                           string
	BinariesRemoteArchiveDestinationCandidates []string
	BinariesRemoteArchiveCheckPresentOnPath    bool
	CompletionsEnabled                         bool
	CompletionsDestinationCandidates           []string
	CompletionsCheckPresentOnFpath             bool
	SilenceInstallStdout                       bool
}

type Installer struct {
	File       *File
	FilePath   string
	Host       Host
	Opts       Options
	Out        io.Writer
	Emit       func(level log.Level, action, msg string)
	EmitSkip   func(level log.Level, action, msg string, reasons ...string)
	EmitDryRun func(action, msg string)

	aptUpdated     bool
	brewUpdated    bool
	codeExts       map[string]string
	outdated       map[string]map[string]bool
	binDir         string
	compDir        string
	requested      map[string]Request
	requiredBy     map[string]string
	baseDone       map[string]bool
	baseInstalling bool
}

func (in *Installer) download(url, dest string) error {
	in.emit(log.Levels.Debug, "download", url)
	return fetchx.Default.Download(url, dest)
}

func (in *Installer) fetch(url string) ([]byte, bool) {
	in.emit(log.Levels.Debug, "download", url)
	body, err := fetchx.Default.Fetch(url)
	return body, err == nil
}

type Request struct {
	Name     string
	Versions []string
	Global   string
}

func Requests(names []string) []Request {
	out := make([]Request, len(names))
	for i, n := range names {
		out[i] = Request{Name: n}
	}
	return out
}

func RequestNames(reqs []Request) []string {
	out := make([]string, len(reqs))
	for i, r := range reqs {
		out[i] = r.Name
	}
	return out
}

func (in *Installer) emit(level log.Level, action, msg string) {
	if in.Emit != nil {
		in.Emit(level, action, msg)
		return
	}
	log.Emit(log.Event{Level: level, Scope: Scope, Action: action, Msg: msg})
}

func (in *Installer) emitSkip(level log.Level, msg string, reasons ...string) {
	if in.EmitSkip != nil {
		in.EmitSkip(level, "install", msg, reasons...)
		return
	}
	log.EmitSkip(level, Scope, "install", msg, reasons...)
}

func (in *Installer) emitPresent(level log.Level, msg, reason string) {
	in.emit(level, "", msg+": ✅ "+reason)
}

func (in *Installer) emitDryRun(action, msg string) {
	if in.EmitDryRun != nil {
		in.EmitDryRun(action, msg)
		return
	}
	log.Emit(log.Event{Level: log.Levels.Info, Scope: Scope, Action: action, Msg: msg, DryRun: true})
}

func (in *Installer) out() io.Writer {
	if in.Out != nil {
		return in.Out
	}
	return os.Stdout
}

func (in *Installer) runCmd(c execx.Cmd) error {
	if !in.Opts.SilenceInstallStdout {
		c.Stdout, c.Stderr = in.out(), in.out()
		return execx.Default.Exec(c)
	}
	var buf bytes.Buffer
	c.Stdout, c.Stderr = &buf, &buf
	if err := execx.Default.Exec(c); err != nil {
		_, _ = in.out().Write(buf.Bytes())
		return err
	}
	return nil
}

func (in *Installer) exec(argv []string) error {
	if err := in.runCmd(execx.Cmd{Argv: argv}); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
	}
	return nil
}

func (in *Installer) output(argv []string) (string, bool) {
	out, err := execx.Default.Output(execx.Cmd{Argv: argv})
	return string(out), err == nil
}

func labelWithVersion(pkg, version string) string {
	if version == "" {
		return pkg
	}
	return pkg + " " + version
}

// [>] 🤖🤖
func (in *Installer) labelPkg(pkg string) string {
	if by, ok := in.requiredBy[pkg]; ok && by != pkg {
		return by + " dependency " + pkg
	}
	return pkg
}

func formatReinstall(label, current, to string) string {
	if current == "" {
		return label + ": -> " + to
	}
	return label + ": " + current + " -> " + to
}

// [<] 🤖🤖

func (in *Installer) resolveCmd(pkg string) string {
	if e, ok := in.File.Packages[pkg]; ok && e.Command != "" {
		return e.Command
	}
	return pkg
}

func (in *Installer) hasCmd(pkg string) bool {
	cmd := in.resolveCmd(pkg)
	if in.Host.HasCmd(cmd) {
		return true
	}
	return cmd == "brew" && in.Host.BrewBin() != ""
}

func (in *Installer) findEntry(pkg string) (Entry, error) { return in.File.Find(pkg, in.FilePath) }

func (in *Installer) pickItem(pkg string, entry Entry) (Item, bool, error) {
	only := in.Opts.OnlyMethods
	if by, ok := in.requiredBy[pkg]; ok && by != pkg {
		only = nil
	}
	return in.Host.pickPreferred(pkg, entry, in.Opts.PreferredMethods, only, in.File.EligibleInstallers(in.Host), in.baseInstalling)
}

func (in *Installer) sudo(argv ...string) []string {
	if in.Host.OS == "linux" && in.Host.Euid != 0 {
		return append([]string{"sudo"}, argv...)
	}
	return argv
}

func (in *Installer) Install(pkgs []string) error {
	return in.InstallRequests(Requests(pkgs))
}

func (in *Installer) InstallRequests(reqs []Request) error {
	pkgs, err := in.plan(reqs)
	if err != nil {
		return err
	}
	pending := pkgs
	for len(pending) > 0 {
		var still []string
		progress := false
		for _, pkg := range pending {
			entry, err := in.findEntry(pkg)
			if err != nil {
				return err
			}
			if len(entry.Items) == 0 {
				progress = true
				if !in.hasCmd(pkg) {
					in.emitSkip(log.Levels.Info, in.labelPkg(pkg), "no install manager, command absent")
					continue
				}
				if err := in.installCompletions(pkg, entry); err != nil {
					return err
				}
				continue
			}
			it, ok, err := in.pickItem(pkg, entry)
			if err != nil {
				return err
			}
			if !ok {
				still = append(still, pkg)
				continue
			}
			progress = true
			had := in.hasCmd(pkg)
			if err := in.installVia(pkg, it); err != nil {
				return err
			}
			if !had {
				if err := in.runEntryPostInstall(pkg, entry); err != nil {
					return err
				}
			}
			if err := in.aliasBinaries(pkg, it); err != nil {
				return err
			}
			if err := in.installCompletions(pkg, entry); err != nil {
				return err
			}
		}
		if !progress {
			break
		}
		pending = still
	}
	for _, pkg := range pending {
		if in.hasCmd(pkg) {
			in.emitSkip(log.Levels.Info, in.labelPkg(pkg), "no applicable installation method, command present")
			entry, err := in.findEntry(pkg)
			if err != nil {
				return err
			}
			if err := in.installCompletions(pkg, entry); err != nil {
				return err
			}
			continue
		}
		if by, dep := in.requiredBy[pkg]; !dep || by == pkg {
			if !in.Opts.MissingMethodWarn {
				return fmt.Errorf("no applicable installation method for %s", pkg)
			}
			in.emitSkip(log.Levels.Warn, in.labelPkg(pkg), "no applicable installation method")
			continue
		}
		in.emitSkip(log.Levels.Info, in.labelPkg(pkg), "no applicable installation method")
	}
	return nil
}

func (in *Installer) plan(reqs []Request) ([]string, error) {
	if in.requested == nil {
		in.requested = map[string]Request{}
	}
	if in.requiredBy == nil {
		in.requiredBy = map[string]string{}
	}
	var out []string
	seen := map[string]bool{}
	var walk func(name string, chain []string) error
	walk = func(name string, chain []string) error {
		if slices.Contains(chain, name) {
			return fmt.Errorf("package %s: requires cycle %s", name, strings.Join(append(chain, name), " -> "))
		}
		if seen[name] {
			return nil
		}
		seen[name] = true
		entry, err := in.findEntry(name)
		if err != nil {
			return err
		}
		for _, dep := range entry.Requires {
			if _, ok := in.requiredBy[dep]; !ok {
				in.requiredBy[dep] = name
			}
			if err := walk(dep, append(chain, name)); err != nil {
				return err
			}
		}
		out = append(out, name)
		return nil
	}
	for _, r := range reqs {
		if len(r.Versions) > 0 {
			in.requested[r.Name] = r
		}
		if err := walk(r.Name, nil); err != nil {
			return nil, err
		}
	}
	if !in.baseInstalling {
		for _, r := range reqs {
			delete(in.requiredBy, r.Name)
		}
	}
	return out, nil
}

func (in *Installer) installVia(pkg string, it Item) error {
	if in.Opts.IfMissing && in.hasCmd(pkg) {
		in.emitSkip(log.Levels.Info, in.labelPkg(pkg), "command present (--if-missing)")
		return nil
	}
	if err := in.ensureBasePackages(it.Mgr, pkg); err != nil {
		return err
	}
	switch {
	case it.Mgr == "apt" && it.Apt != nil:
		return in.installAptSpec(pkg, it.Apt)
	case it.Mgr == "nix":
		return in.installNixSpec(pkg, it.Nix)
	case it.VersionManager != nil:
		return in.installVersionManager(pkg, it.VersionManager)
	case it.Mgr == "binariesRemoteArchive":
		return in.installBinariesRemoteArchive(pkg, it.BinariesRemoteArchive)
	case it.Mgr == "buildFromSource":
		return in.installBuildFromSource(pkg, it.BuildFromSource)
	case it.Mgr == "script":
		return in.installScript(pkg, it.Script)
	}
	pin := in.resolvePin(pkg, it.Version)
	if it.Mgr == "cask" && pin != "" {
		in.emit(log.Levels.Warn, "unpinnable", in.labelPkg(pkg)+": version "+pin+" cannot be enforced via brew/cask, installing the current release")
		pin = ""
	}
	base, err := in.resolvePkgName(pkg, it, pin)
	if err != nil {
		return err
	}
	installed := in.isInstalled(pkg, it.Mgr, base)
	switch {
	case installed && pin != "" && !in.pinSatisfied(pkg, it.Mgr, base, pin):
		if !installAcceptsPin(it.Mgr) && !in.managerHasNewer(it.Mgr, base) {
			in.emitSkip(log.Levels.Warn, in.labelPkg(pkg), fmt.Sprintf("pin %s unsatisfiable via %s: installed %s is the manager's latest", pin, it.Mgr, in.installedVersion(it.Mgr, base)))
			return nil
		}
		in.emit(log.Levels.Info, "reinstall", formatReinstall(in.labelPkg(pkg), in.installedVersion(it.Mgr, base), pin))
		if !installAcceptsPin(it.Mgr) {
			return in.update(pkg, it.Mgr, base)
		}
	case installed && in.Opts.Update && pin == "":
		return in.update(pkg, it.Mgr, base)
	case installed:
		in.emitPresent(log.Levels.Info, in.labelPkg(pkg), "already installed via "+it.Mgr)
		return nil
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", labelWithVersion(in.labelPkg(pkg), pin)+" via "+it.Mgr)
		return nil
	}
	if err := in.managerInstall(it.Mgr, applyPin(it.Mgr, base, pin), pin); err != nil {
		return err
	}
	ver := pin
	if ver == "" && it.Mgr == "apt" {
		ver = in.installedVersion("apt", base)
	}
	in.emit(log.Levels.Info, "installed", labelWithVersion(in.labelPkg(pkg), ver)+" via "+it.Mgr)
	return nil
}

func installAcceptsPin(mgr string) bool {
	return slices.Contains([]string{"npm", "apt", "gem", "go"}, mgr)
}

func (in *Installer) resolvePkgName(pkg string, it Item, pin string) (string, error) {
	base := cmp.Or(it.Name, pkg)
	if it.Mgr == "brew" && pin != "" {
		base += "@" + pin
	}
	if it.Registry != "" {
		if !in.File.hasBrewTap(it.Registry) {
			return "", fmt.Errorf("%s: unknown brew registry %q (installerRegistries.brew)", pkg, it.Registry)
		}
		base = it.Registry + "/" + base
	}
	return base, nil
}

func (in *Installer) resolveBrewBin() string {
	if _, err := in.Host.LookPath("brew"); err == nil {
		return "brew"
	}
	if p := in.Host.BrewBin(); p != "" {
		return p
	}
	return "brew"
}

// [>] 🤖🤖
func (in *Installer) resolveNpmBin() string {
	if in.Host.HasCmd("npm") {
		return "npm"
	}
	if v := versionManagerRoutines["nvm"].global(in); v != "" {
		p := filepath.Join(in.Host.nvmDir(), "versions", "node", "v"+v, "bin", "npm")
		if fileExists(p) {
			return p
		}
	}
	return "npm"
}

// [<] 🤖🤖

func (in *Installer) makeBrewArgv(mgr, sub string, args ...string) []string {
	argv := []string{in.resolveBrewBin(), sub}
	if mgr == "cask" {
		argv = append(argv, "--cask")
	}
	return append(argv, args...)
}

func (in *Installer) managerHasNewer(mgr, base string) bool {
	if mgr != "brew" && mgr != "cask" {
		return true
	}
	out, ok := in.output(in.makeBrewArgv(mgr, "outdated", "--quiet", path.Base(base)))
	return ok && strings.TrimSpace(out) != ""
}

func applyPin(mgr, base, pin string) string {
	if pin == "" {
		return base
	}
	switch mgr {
	case "npm":
		return base + "@" + pin
	case "apt":
		return base + "=" + pin
	case "go":
		module, _, _ := strings.Cut(base, "@")
		return module + "@v" + pin
	}
	return base
}

func (in *Installer) pinSatisfied(pkg, mgr, base, pin string) bool {
	if v := in.installedVersion(mgr, base); v != "" {
		return PinMatches(v, pin)
	}
	return in.versionOutputHasPin(pkg, pin)
}

func splitPin(name string) (base, pin string) {
	if i := strings.LastIndex(name, "@"); i > 0 {
		return name[:i], name[i+1:]
	}
	return name, ""
}

func (in *Installer) isInstalled(pkg, mgr, base string) bool {
	switch mgr {
	case "brew", "cask":
		_, ok := in.output(in.makeBrewArgv(mgr, "list", path.Base(base)))
		return ok
	case "apt":
		_, ok := in.output([]string{"dpkg", "-s", base})
		return ok
	case "npm":
		if in.hasCmd(pkg) {
			return true
		}
		_, ok := in.output([]string{in.resolveNpmBin(), "ls", "--global", "--depth=0", base})
		return ok
	case "nix":
		out, ok := in.listNixProfile()
		if !ok {
			return false
		}
		found, _ := findNixEntry(out, base)
		return found
	default:
		return in.hasCmd(pkg)
	}
}

func (in *Installer) installedVersion(mgr, base string) string {
	switch mgr {
	case "brew":
		out, ok := in.output([]string{in.resolveBrewBin(), "list", "--versions", path.Base(base)})
		if !ok {
			return ""
		}
		if fields := strings.Fields(out); len(fields) > 1 {
			return fields[len(fields)-1]
		}
		return ""
	case "apt":
		out, ok := in.output([]string{"dpkg-query", "-W", "-f=${Version}", base})
		if !ok {
			return ""
		}
		return strings.TrimSpace(out)
	case "npm":
		out, ok := in.output([]string{in.resolveNpmBin(), "ls", "--global", "--depth=0", base})
		if !ok {
			return ""
		}
		_, ver, found := strings.Cut(out, base+"@")
		if !found {
			return ""
		}
		return strings.TrimSpace(strings.Fields(ver)[0])
	case "nix":
		out, ok := in.listNixProfile()
		if !ok {
			return ""
		}
		_, ver := findNixEntry(out, base)
		return ver
	default:
		return ""
	}
}

type managerRoutine struct {
	install func(in *Installer, mgr, name, pin string) error
	update  func(in *Installer, mgr, base string) error
}

var brewRoutine = managerRoutine{
	install: func(in *Installer, mgr, name, _ string) error { return in.brewInstall(mgr, name) },
	update:  func(in *Installer, mgr, base string) error { return in.exec(in.makeBrewArgv(mgr, "upgrade", base)) },
}

var managerRoutines = map[string]managerRoutine{
	"brew": brewRoutine,
	"cask": brewRoutine,
	"apt": {
		install: func(in *Installer, _, name, _ string) error {
			if err := in.aptUpdate(); err != nil {
				return err
			}
			return in.exec(in.sudo("apt-get", "install", "--yes", "--no-install-recommends", name))
		},
		update: func(in *Installer, _, base string) error {
			if err := in.aptUpdate(); err != nil {
				return err
			}
			return in.exec(in.sudo("apt-get", "install", "--only-upgrade", "--yes", base))
		},
	},
	"npm": {
		install: func(in *Installer, _, name, _ string) error {
			if err := in.exec(in.sudo(in.resolveNpmBin(), "install", "--global", name)); err != nil {
				return err
			}
			in.linkNvmNpmBins()
			return nil
		},
		update: func(in *Installer, _, base string) error {
			if err := in.exec(in.sudo(in.resolveNpmBin(), "update", "--global", base)); err != nil {
				return err
			}
			in.linkNvmNpmBins()
			return nil
		},
	},
	"gem": {
		install: func(in *Installer, _, name, pin string) error {
			if pin != "" {
				return in.exec(in.sudo("gem", "install", name, "-v", pin))
			}
			return in.exec(in.sudo("gem", "install", name))
		},
		update: func(in *Installer, _, base string) error { return in.exec(in.sudo("gem", "update", base)) },
	},
	"go": {
		install: func(in *Installer, _, name, _ string) error { return in.goInstall(name, name) },
	},
	"nix": nixInstallRoutine,
}

func (in *Installer) update(pkg, mgr, base string) error {
	if in.Opts.DryRun {
		in.emitDryRun("update", in.labelPkg(pkg)+" via "+mgr)
		return nil
	}
	if mgr == "go" {
		return in.goInstall(pkg, base)
	}
	r, ok := managerRoutines[mgr]
	if !ok || r.update == nil {
		return fmt.Errorf("no update routine for manager %s", mgr)
	}
	if err := r.update(in, mgr, base); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "updated", labelWithVersion(in.labelPkg(pkg), in.installedVersion(mgr, base))+" via "+mgr)
	return nil
}

func (in *Installer) managerInstall(mgr, name, pin string) error {
	r, ok := managerRoutines[mgr]
	if !ok {
		return fmt.Errorf("no install routine for manager %s", mgr)
	}
	return r.install(in, mgr, name, pin)
}

func (in *Installer) brewInstall(mgr, name string) error {
	if err := in.brewUpdate(); err != nil {
		return err
	}
	if parts := strings.Split(name, "/"); len(parts) >= 3 {
		tap := parts[0] + "/" + parts[1]
		if err := in.exec([]string{in.resolveBrewBin(), "tap", tap}); err != nil {
			return err
		}
		if err := in.exec([]string{in.resolveBrewBin(), "trust", tap}); err != nil {
			return err
		}
	}
	return in.exec(in.makeBrewArgv(mgr, "install", name))
}

func (in *Installer) aptUpdate() error {
	if in.aptUpdated {
		return nil
	}
	in.aptUpdated = true
	return in.exec(in.sudo("apt-get", "update"))
}

func (in *Installer) brewUpdate() error {
	if in.brewUpdated {
		return nil
	}
	in.brewUpdated = true
	return in.exec([]string{in.resolveBrewBin(), "update", "--quiet"})
}

func (in *Installer) skipInstalledOrEmitReinstall(pkg, method, pin, to string, checkInstalled, checkPin func() bool, current func() string) bool {
	if !checkInstalled() {
		return false
	}
	if pin == "" || checkPin() {
		in.emitPresent(log.Levels.Info, in.labelPkg(pkg), "already installed via "+method)
		return true
	}
	cur := ""
	if current != nil {
		cur = current()
	}
	in.emit(log.Levels.Info, "reinstall", formatReinstall(in.labelPkg(pkg), cur, to))
	return false
}

func (in *Installer) runScript(pkg string, s *ScriptSpec, errLabel string) error {
	argv, cleanup, err := in.makeScriptArgv(pkg, s)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	if err := in.runCmd(execx.Cmd{Argv: argv, Env: in.makeScriptEnv(pkg, s)}); err != nil {
		return fmt.Errorf("%s: %s: %w", pkg, errLabel, err)
	}
	return nil
}

func (in *Installer) installScript(pkg string, s *ScriptSpec) error {
	pin := in.resolvePin(pkg, s.Version)
	if s.ValidateArtifact != "" {
		if fileExists(in.expandPath(s.ValidateArtifact)) {
			in.emitPresent(log.Levels.Info, in.labelPkg(pkg), "already installed via script ("+s.ValidateArtifact+" present)")
			return nil
		}
	} else if in.skipInstalledOrEmitReinstall(pkg, "script", pin, s.Version,
		func() bool { return in.hasCmd(pkg) },
		func() bool { return in.versionOutputHasPin(pkg, pin) }, nil) {
		return nil
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", labelWithVersion(in.labelPkg(pkg), pin)+" via script")
		return nil
	}
	if err := in.runScript(pkg, s, "install script"); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "installed", labelWithVersion(in.labelPkg(pkg), pin)+" via script")
	return nil
}

func (in *Installer) makeScriptEnv(pkg string, s *ScriptSpec) []string {
	env := append(os.Environ(),
		"CHE_PKG_NAME="+pkg,
		"CHE_PKG_VERSION="+s.Version,
		"CHE_PKG_SHA256="+strings.TrimPrefix(s.PlatformEligibility.Checksums[in.Host.PlatformKey()], "sha256:"),
		"CHE_PKG_OS="+in.Host.OS,
		"CHE_PKG_ARCH="+in.Host.Arch,
	)
	names := in.File.archSchemesOrBuiltin()
	for _, set := range slices.Sorted(maps.Keys(names)) {
		if v, ok := names[set][in.Host.Arch]; ok {
			env = append(env, "CHE_PKG_ARCH_"+strings.ToUpper(set)+"="+v)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(s.Env)) {
		env = append(env, k+"="+in.expandPath(s.Env[k]))
	}
	return env
}

func withScriptArgs(argv []string, s *ScriptSpec) []string {
	if len(s.Args) == 0 {
		return argv
	}
	const shDashCArgv0 = "che-script"
	return append(append(argv, shDashCArgv0), s.Args...)
}

func (in *Installer) makeScriptArgv(pkg string, s *ScriptSpec) ([]string, func(), error) {
	if s.Run != "" {
		return withScriptArgs([]string{"/bin/sh", "-ec", s.Run}, s), nil, nil
	}
	if s.URL != "" {
		content, ok := in.fetch(s.URL)
		if !ok || len(content) == 0 {
			return nil, nil, fmt.Errorf("%s: install script fetch failed: %s", pkg, s.URL)
		}
		f, err := os.CreateTemp("", "che-script-*")
		if err != nil {
			return nil, nil, err
		}
		if _, err := f.Write(content); err != nil {
			return nil, nil, err
		}
		if err := f.Chmod(0o755); err != nil {
			return nil, nil, err
		}
		if err := f.Close(); err != nil {
			return nil, nil, err
		}
		cleanup := func() { _ = os.Remove(f.Name()) }
		return append([]string{f.Name()}, s.Args...), cleanup, nil
	}
	p := s.Path
	if !filepath.IsAbs(p) && in.FilePath != BuiltinPath {
		p = filepath.Join(filepath.Dir(in.FilePath), p)
	}
	if in.FilePath != BuiltinPath || filepath.IsAbs(p) {
		if _, err := os.Stat(p); err == nil {
			return append([]string{"/bin/sh", "-e", p}, s.Args...), nil, nil
		}
	}
	if in.FilePath == BuiltinPath && !filepath.IsAbs(s.Path) {
		if b, err := chepackages.Scripts.ReadFile("scripts/" + path.Base(s.Path)); err == nil {
			return withScriptArgs([]string{"/bin/sh", "-ec", string(b)}, s), nil, nil
		}
	}
	return nil, nil, fmt.Errorf("%s: install script not found: %s", pkg, p)
}

func (in *Installer) goInstall(pkg, module string) error {
	if !strings.Contains(module, "@") {
		module += "@latest"
	}
	c := execx.Cmd{
		Argv: []string{"go", "install", module},
		Env:  append(os.Environ(), "PATH="+in.resolveBinDir()+":"+in.Host.Getenv("PATH")),
	}
	if err := in.runCmd(c); err != nil {
		return fmt.Errorf("go install %s: %w", module, err)
	}
	in.emit(log.Levels.Info, "installed", in.labelPkg(pkg)+" via go")
	return nil
}

// [<] 🤖🤖🤖
