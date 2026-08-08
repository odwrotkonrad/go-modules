package packages

// [>] 🤖🤖🤖

import (
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

const Scope = "install-packages"

type Options struct {
	Update                                       bool
	IfMissing                                    bool
	DryRun                                       bool
	PreferredMethods                             []string
	PrebuiltBinariesArchiveDestinationCandidates []string
	PrebuiltBinariesArchiveCheckPresentOnPath    bool
	CompletionsEnabled                           bool
	CompletionsDestinationCandidates             []string
	CompletionsCheckPresentOnFpath               bool
}

type Installer struct {
	File       *File
	FilePath   string
	Host       Host
	Opts       Options
	Emit       func(level log.Level, action, msg string)
	EmitSkip   func(level log.Level, action, msg string, reasons ...string)
	EmitDryRun func(action, msg string)

	aptUpdated     bool
	codeExts       map[string]string
	outdated       map[string]map[string]bool
	binDir         string
	compDir        string
	requested      map[string]Request
	baseDone       map[string]bool
	baseInstalling bool
}

func curlArgv(url string, dest ...string) []string {
	argv := []string{"curl", "-fsSL", "--connect-timeout", "30", "--retry", "10", "--retry-delay", "30", "--retry-all-errors"}
	for _, d := range dest {
		argv = append(argv, "-o", d)
	}
	return append(argv, url)
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

func (in *Installer) emitDryRun(action, msg string) {
	if in.EmitDryRun != nil {
		in.EmitDryRun(action, msg)
		return
	}
	log.Emit(log.Event{Level: log.Levels.Info, Scope: Scope, Action: action, Msg: msg, DryRun: true})
}

func (in *Installer) exec(argv []string) error {
	c := execx.Cmd{Argv: argv, Stdout: os.Stdout, Stderr: os.Stderr}
	if err := execx.Default.Exec(c); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
	}
	return nil
}

func (in *Installer) output(argv []string) (string, bool) {
	out, err := execx.Default.Output(execx.Cmd{Argv: argv})
	return string(out), err == nil
}

func (in *Installer) cmdFor(pkg string) string {
	if e, ok := in.File.Packages[pkg]; ok && e.Command != "" {
		return e.Command
	}
	return pkg
}

func (in *Installer) hasCmd(pkg string) bool { return in.Host.HasCmd(in.cmdFor(pkg)) }

func (in *Installer) findEntry(pkg string) (Entry, error) { return in.File.Find(pkg, in.FilePath) }

func (in *Installer) pickItem(pkg string, entry Entry) (Item, bool, error) {
	return in.Host.pickPreferred(pkg, entry, in.Opts.PreferredMethods, in.File.platformsOrBuiltin()[in.Host.ShaKey()])
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
					in.emitSkip(log.Levels.Info, pkg, "no install manager, command absent")
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
			in.emitSkip(log.Levels.Debug, pkg, "no applicable manager, command present")
			entry, err := in.findEntry(pkg)
			if err != nil {
				return err
			}
			if err := in.installCompletions(pkg, entry); err != nil {
				return err
			}
			continue
		}
		in.emitSkip(log.Levels.Info, pkg, "no applicable manager")
	}
	return nil
}

// [why] requires are expanded depth-first so a package's dependencies install before it
func (in *Installer) plan(reqs []Request) ([]string, error) {
	if in.requested == nil {
		in.requested = map[string]Request{}
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
	return out, nil
}

func (in *Installer) installVia(pkg string, it Item) error {
	if in.Opts.IfMissing && in.hasCmd(pkg) {
		in.emitSkip(log.Levels.Info, pkg, "command present (--if-missing)")
		return nil
	}
	if !in.baseInstalling {
		if err := in.ensureBasePackages(it.Mgr); err != nil {
			return err
		}
	}
	switch {
	case it.Mgr == "apt" && it.Apt != nil:
		return in.installAptSpec(pkg, it.Apt)
	case it.Mgr == "versionManager":
		return in.installVersionManager(pkg, it.VersionManager)
	case it.Mgr == "prebuiltBinariesArchive":
		return in.installPrebuiltBinariesArchive(pkg, it.PrebuiltBinariesArchive)
	case it.Mgr == "script":
		return in.installScript(pkg, it.Script)
	}
	base := it.Name
	if base == "" {
		base = pkg
	}
	pin := in.pinFor(pkg, "")
	if strings.Contains(base, "{version}") {
		if pin == "" {
			return fmt.Errorf("%s: item name %s references {version} but no version is pinned", pkg, it.Name)
		}
		base = strings.ReplaceAll(base, "{version}", pin)
	}
	installed := in.isInstalled(pkg, it.Mgr, base)
	switch {
	case installed && pin != "" && !in.pinSatisfied(pkg, it.Mgr, base, pin):
		in.emit(log.Levels.Info, "reinstall", pkg+": -> "+pin)
		if !pinnable(it.Mgr) {
			return in.update(pkg, it.Mgr, base)
		}
	case installed && in.Opts.Update && pin == "":
		return in.update(pkg, it.Mgr, base)
	case installed:
		in.emitSkip(log.Levels.Debug, pkg, "already installed via "+it.Mgr)
		return nil
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", pkg+" via "+it.Mgr)
		return nil
	}
	if err := in.managerInstall(it.Mgr, pinnedName(it.Mgr, base, pin), pin); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "installed", pkg+" via "+it.Mgr)
	return nil
}

// [why] managers whose install command takes the pinned version; brew is excluded on purpose:
//
//	it ships versions as separate formulae (node@24), so the packages file names the formula directly
func pinnable(mgr string) bool {
	return slices.Contains([]string{"npm", "apt", "gem", "go", "vscode"}, mgr)
}

func pinnedName(mgr, base, pin string) string {
	if pin == "" {
		return base
	}
	switch mgr {
	case "npm", "vscode":
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
	// [why] a pinned brew formula is its own package (node@24): ask about the pinned name
	case "brew":
		_, ok := in.output([]string{"brew", "list", path.Base(base)})
		return ok
	case "cask":
		_, ok := in.output([]string{"brew", "list", "--cask", path.Base(base)})
		return ok
	case "apt":
		_, ok := in.output([]string{"dpkg", "-s", base})
		return ok
	case "npm":
		if in.hasCmd(pkg) {
			return true
		}
		_, ok := in.output([]string{"npm", "ls", "--global", "--depth=0", base})
		return ok
	case "vscode":
		_, ok := in.codeExtensions()[strings.ToLower(base)]
		return ok
	default:
		return in.hasCmd(pkg)
	}
}

// [why] --show-versions prints "publisher.ext@1.2.3", the only way to check an extension pin
func (in *Installer) codeExtensions() map[string]string {
	if in.codeExts != nil {
		return in.codeExts
	}
	in.codeExts = map[string]string{}
	if out, ok := in.output([]string{"code", "--list-extensions", "--show-versions"}); ok {
		for line := range strings.Lines(out) {
			ext := strings.ToLower(strings.TrimSpace(line))
			if ext == "" {
				continue
			}
			name, version, _ := strings.Cut(ext, "@")
			in.codeExts[name] = version
		}
	}
	return in.codeExts
}

func (in *Installer) installedVersion(mgr, base string) string {
	switch mgr {
	case "brew":
		out, ok := in.output([]string{"brew", "list", "--versions", path.Base(base)})
		if !ok {
			return ""
		}
		if fields := strings.Fields(out); len(fields) > 1 {
			return fields[len(fields)-1]
		}
		return ""
	case "vscode":
		return in.codeExtensions()[strings.ToLower(base)]
	case "apt":
		out, ok := in.output([]string{"dpkg-query", "-W", "-f=${Version}", base})
		if !ok {
			return ""
		}
		return strings.TrimSpace(out)
	case "npm":
		out, ok := in.output([]string{"npm", "ls", "--global", "--depth=0", base})
		if !ok {
			return ""
		}
		_, ver, found := strings.Cut(out, base+"@")
		if !found {
			return ""
		}
		return strings.TrimSpace(strings.Fields(ver)[0])
	default:
		return ""
	}
}

func (in *Installer) update(pkg, mgr, base string) error {
	if in.Opts.DryRun {
		in.emitDryRun("update", pkg+" via "+mgr)
		return nil
	}
	var argv []string
	switch mgr {
	case "brew":
		argv = []string{"brew", "upgrade", base}
	case "cask":
		argv = []string{"brew", "upgrade", "--cask", base}
	case "apt":
		if err := in.aptUpdate(); err != nil {
			return err
		}
		argv = in.sudo("apt-get", "install", "--only-upgrade", "--yes", base)
	case "npm":
		argv = in.sudo("npm", "update", "--global", base)
	case "vscode":
		argv = []string{"code", "--install-extension", base, "--force"}
	case "gem":
		argv = in.sudo("gem", "update", base)
	case "go":
		return in.goInstall(pkg, base)
	}
	if err := in.exec(argv); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "updated", pkg+" via "+mgr)
	return nil
}

func (in *Installer) managerInstall(mgr, name, pin string) error {
	switch mgr {
	case "brew", "cask":
		return in.brewInstall(mgr, name)
	case "apt":
		if err := in.aptUpdate(); err != nil {
			return err
		}
		return in.exec(in.sudo("apt-get", "install", "--yes", "--no-install-recommends", name))
	case "npm":
		return in.exec(in.sudo("npm", "install", "--global", name))
	case "vscode":
		if err := in.exec([]string{"code", "--install-extension", name}); err != nil {
			return err
		}
		base, version, _ := strings.Cut(strings.ToLower(name), "@")
		in.codeExtensions()[base] = version
		return nil
	case "gem":
		if pin != "" {
			return in.exec(in.sudo("gem", "install", name, "-v", pin))
		}
		return in.exec(in.sudo("gem", "install", name))
	case "go":
		return in.goInstall(name, name)
	default:
		return fmt.Errorf("no install routine for manager %s", mgr)
	}
}

func (in *Installer) brewInstall(mgr, name string) error {
	if parts := strings.Split(name, "/"); len(parts) >= 3 {
		if err := in.exec([]string{"brew", "tap", parts[0] + "/" + parts[1]}); err != nil {
			return err
		}
		kind := "--formula"
		if mgr == "cask" {
			kind = "--cask"
		}
		if err := in.exec([]string{"brew", "trust", kind, name}); err != nil {
			return err
		}
	}
	argv := []string{"brew", "install", name}
	if mgr == "cask" {
		argv = []string{"brew", "install", "--cask", name}
	}
	return in.exec(argv)
}

func (in *Installer) aptUpdate() error {
	if in.aptUpdated {
		return nil
	}
	in.aptUpdated = true
	return in.exec(in.sudo("apt-get", "update"))
}

func (in *Installer) installScript(pkg string, s *ScriptSpec) error {
	if s.ValidateArtifact != "" {
		if fileExists(in.expandPath(s.ValidateArtifact)) {
			in.emitSkip(log.Levels.Debug, pkg, "already installed via script ("+s.ValidateArtifact+" present)")
			return nil
		}
	} else if in.hasCmd(pkg) {
		if pin := in.pinFor(pkg, s.Version); pin == "" || in.versionOutputHasPin(pkg, pin) {
			in.emitSkip(log.Levels.Debug, pkg, "already installed via script")
			return nil
		}
		in.emit(log.Levels.Info, "reinstall", pkg+": -> "+s.Version)
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", pkg+" via script")
		return nil
	}
	argv, cleanup, err := in.scriptArgv(pkg, s)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	c := execx.Cmd{Argv: argv, Env: in.scriptEnv(pkg, s), Stdout: os.Stdout, Stderr: os.Stderr}
	if err := execx.Default.Exec(c); err != nil {
		return fmt.Errorf("%s: install script: %w", pkg, err)
	}
	in.emit(log.Levels.Info, "installed", pkg+" via script")
	return nil
}

func (in *Installer) scriptEnv(pkg string, s *ScriptSpec) []string {
	env := append(os.Environ(),
		"CHE_PKG_NAME="+pkg,
		"CHE_PKG_VERSION="+s.Version,
		"CHE_PKG_SHA256="+s.Platforms.Sha[in.Host.ShaKey()],
		"CHE_PKG_OS="+in.Host.OS,
		"CHE_PKG_ARCH="+in.Host.Arch,
	)
	names := in.File.archNameConventionsOrBuiltin()
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

// [why] with -c the first trailing word becomes $0, so a placeholder keeps s.Args aligned to $1..
func withScriptArgs(argv []string, s *ScriptSpec) []string {
	if len(s.Args) == 0 {
		return argv
	}
	return append(append(argv, "che-script"), s.Args...)
}

func (in *Installer) scriptArgv(pkg string, s *ScriptSpec) ([]string, func(), error) {
	if s.Run != "" {
		return withScriptArgs([]string{"/bin/sh", "-ec", s.Run}, s), nil, nil
	}
	// [why] the fetched script runs as a file so its shebang picks the interpreter (nvm insists on bash)
	if s.URL != "" {
		content, ok := in.output(curlArgv(s.URL))
		if !ok || content == "" {
			return nil, nil, fmt.Errorf("%s: install script fetch failed: %s", pkg, s.URL)
		}
		f, err := os.CreateTemp("", "che-script-*")
		if err != nil {
			return nil, nil, err
		}
		if _, err := f.WriteString(content); err != nil {
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
	// [why] only builtin entries reach the embedded scripts: a user file's missing script must
	//   error, never silently run a same-named shipped one
	if in.FilePath == BuiltinPath && !filepath.IsAbs(s.Path) {
		if b, err := builtinScripts.ReadFile("scripts/" + path.Base(s.Path)); err == nil {
			return withScriptArgs([]string{"/bin/sh", "-ec", string(b)}, s), nil, nil
		}
	}
	return nil, nil, fmt.Errorf("%s: install script not found: %s", pkg, p)
}

// [why] go install demands a version suffix: an unpinned entry tracks the module's latest
func (in *Installer) goInstall(pkg, module string) error {
	if !strings.Contains(module, "@") {
		module += "@latest"
	}
	c := execx.Cmd{
		Argv:   []string{"go", "install", module},
		Env:    append(os.Environ(), "PATH="+in.userBinDir()+":"+in.Host.Getenv("PATH")),
		Stdout: os.Stdout, Stderr: os.Stderr,
	}
	if err := execx.Default.Exec(c); err != nil {
		return fmt.Errorf("go install %s: %w", module, err)
	}
	in.emit(log.Levels.Info, "installed", pkg+" via go")
	return nil
}

// [<] 🤖🤖🤖
