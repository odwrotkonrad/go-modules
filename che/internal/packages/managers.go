package packages

// [>] 🤖🤖🤖

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

const Scope = "install-packages"

type Options struct {
	Update                           bool
	IfMissing                        bool
	DryRun                           bool
	PreferredMethods                 []string
	BinaryDestinationCandidates      []string
	BinaryCheckInPath                bool
	CompletionsEnabled               bool
	CompletionsPackages              []string
	CompletionsDestinationCandidates []string
	CompletionsCheckInFpath          bool
}

type Installer struct {
	File       *File
	FilePath   string
	Host       Host
	Opts       Options
	Emit       func(level log.Level, action, msg string)
	EmitSkip   func(level log.Level, action, msg string, reasons ...string)
	EmitDryRun func(action, msg string)

	aptUpdated bool
	codeExts   map[string]bool
	binDir     string
	compDir    string
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

func (in *Installer) sudo(argv ...string) []string {
	if in.Host.OS == "linux" && in.Host.Euid != 0 {
		return append([]string{"sudo"}, argv...)
	}
	return argv
}

func (in *Installer) Install(pkgs []string) error {
	pending := pkgs
	for len(pending) > 0 {
		var still []string
		progress := false
		for _, pkg := range pending {
			entry, err := in.File.Find(pkg, in.FilePath)
			if err != nil {
				return err
			}
			it, ok, err := in.Host.pickPreferred(pkg, entry, in.Opts.PreferredMethods)
			if err != nil {
				return err
			}
			if !ok {
				still = append(still, pkg)
				continue
			}
			progress = true
			if err := in.installVia(pkg, it); err != nil {
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
		in.emitSkip(log.Levels.Info, pkg, "no applicable manager")
	}
	return nil
}

func (in *Installer) installVia(pkg string, it Item) error {
	if in.Opts.IfMissing && in.Host.HasCmd(pkg) {
		in.emitSkip(log.Levels.Info, pkg, "command present (--if-missing)")
		return nil
	}
	if it.Mgr == "binary" {
		return in.installBinary(pkg, it.Binary)
	}
	if it.Mgr == "script" {
		return in.installScript(pkg, it.Script)
	}
	if it.Mgr == "pkg" {
		return in.installPkg(pkg, it.Pkg)
	}
	name := it.Name
	if name == "" {
		name = pkg
	}
	base, pin := splitPin(it.Mgr, name)
	installed := in.isInstalled(pkg, it.Mgr, base)
	switch {
	case installed && pin != "" && in.installedVersion(it.Mgr, base) != pin:
		in.emit(log.Levels.Info, "reinstall", pkg+": -> "+pin)
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
	if err := in.managerInstall(it.Mgr, name); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "installed", pkg+" via "+it.Mgr)
	return nil
}

func splitPin(mgr, name string) (base, pin string) {
	switch mgr {
	case "npm":
		if i := strings.LastIndex(name, "@"); i > 0 {
			return name[:i], name[i+1:]
		}
	case "apt":
		if base, pin, ok := strings.Cut(name, "="); ok {
			return base, pin
		}
	}
	return name, ""
}

func (in *Installer) isInstalled(pkg, mgr, base string) bool {
	switch mgr {
	case "brew":
		_, ok := in.output([]string{"brew", "list", tail(base)})
		return ok
	case "cask":
		_, ok := in.output([]string{"brew", "list", "--cask", tail(base)})
		return ok
	case "apt":
		_, ok := in.output([]string{"dpkg", "-s", base})
		return ok
	case "npm":
		if in.Host.HasCmd(pkg) {
			return true
		}
		_, ok := in.output([]string{"npm", "ls", "--global", "--depth=0", base})
		return ok
	case "code":
		return in.codeExtensions()[strings.ToLower(base)]
	default:
		return in.Host.HasCmd(pkg)
	}
}

func (in *Installer) codeExtensions() map[string]bool {
	if in.codeExts != nil {
		return in.codeExts
	}
	in.codeExts = map[string]bool{}
	if out, ok := in.output([]string{"code", "--list-extensions"}); ok {
		for line := range strings.Lines(out) {
			if ext := strings.ToLower(strings.TrimSpace(line)); ext != "" {
				in.codeExts[ext] = true
			}
		}
	}
	return in.codeExts
}

func (in *Installer) installedVersion(mgr, base string) string {
	switch mgr {
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
	case "code":
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

func (in *Installer) managerInstall(mgr, name string) error {
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
	case "code":
		if err := in.exec([]string{"code", "--install-extension", name}); err != nil {
			return err
		}
		in.codeExtensions()[strings.ToLower(name)] = true
		return nil
	case "gem":
		return in.exec(in.sudo("gem", "install", name))
	default:
		return in.goInstall(name, name)
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
	if in.Host.HasCmd(pkg) {
		if s.Version == "" || in.versionOutputHasPin(pkg, s.Version) {
			in.emitSkip(log.Levels.Debug, pkg, "already installed via script")
			return nil
		}
		in.emit(log.Levels.Info, "reinstall", pkg+": -> "+s.Version)
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", pkg+" via script")
		return nil
	}
	argv, err := in.scriptArgv(pkg, s)
	if err != nil {
		return err
	}
	c := execx.Cmd{Argv: argv, Env: in.scriptEnv(pkg, s), Stdout: os.Stdout, Stderr: os.Stderr}
	if err := execx.Default.Exec(c); err != nil {
		return fmt.Errorf("%s: install script: %w", pkg, err)
	}
	in.emit(log.Levels.Info, "installed", pkg+" via script")
	return nil
}

func (in *Installer) scriptEnv(pkg string, s *ScriptSpec) []string {
	return append(os.Environ(),
		"CHE_PKG_NAME="+pkg,
		"CHE_PKG_VERSION="+s.Version,
		"CHE_PKG_SHA256="+s.Sha256[in.Host.ShaKey()],
		"CHE_PKG_OS="+in.Host.OS,
		"CHE_PKG_ARCH="+in.Host.Arch,
		"CHE_PKG_ARCH_X="+in.Host.ArchX,
		"CHE_PKG_ARCH_G="+in.Host.ArchG,
	)
}

func (in *Installer) scriptArgv(pkg string, s *ScriptSpec) ([]string, error) {
	if s.Run != "" {
		return []string{"/bin/sh", "-ec", s.Run}, nil
	}
	if s.URL != "" {
		content, ok := in.output([]string{"curl", "-fsSL", "--connect-timeout", "30", "--retry", "10", "--retry-delay", "30", "--retry-all-errors", s.URL})
		if !ok || content == "" {
			return nil, fmt.Errorf("%s: install script fetch failed: %s", pkg, s.URL)
		}
		return []string{"/bin/sh", "-ec", content}, nil
	}
	p := s.Path
	if !filepath.IsAbs(p) && in.FilePath != BuiltinPath {
		p = filepath.Join(filepath.Dir(in.FilePath), p)
	}
	if in.FilePath != BuiltinPath || filepath.IsAbs(p) {
		if _, err := os.Stat(p); err == nil {
			return []string{"/bin/sh", "-e", p}, nil
		}
	}
	if b, err := builtinScripts.ReadFile("scripts/" + path.Base(s.Path)); err == nil {
		return []string{"/bin/sh", "-ec", string(b)}, nil
	}
	return nil, fmt.Errorf("%s: install script not found: %s", pkg, s.Path)
}

func (in *Installer) goInstall(pkg, module string) error {
	c := execx.Cmd{
		Argv:   []string{"go", "install", module},
		Env:    append(os.Environ(), "PATH=/usr/local/go/bin:"+in.Host.Getenv("PATH")),
		Stdout: os.Stdout, Stderr: os.Stderr,
	}
	if err := execx.Default.Exec(c); err != nil {
		return fmt.Errorf("go install %s: %w", module, err)
	}
	in.emit(log.Levels.Info, "installed", pkg+" via go")
	return nil
}

func tail(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// [<] 🤖🤖🤖
