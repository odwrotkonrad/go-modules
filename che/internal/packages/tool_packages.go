package packages

// [>] 🤖🤖

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

type toolRoutine struct {
	baseGroup string
	command   string
	list      func(in *Installer) map[string]string
	install   func(in *Installer, name, pin string) error
	update    func(in *Installer, name string) error
}

var toolRoutines = map[string]toolRoutine{
	"vscode": {
		baseGroup: "vscode",
		command:   "code",
		list:      func(in *Installer) map[string]string { return in.codeExtensions() },
		install: func(in *Installer, name, pin string) error {
			target := name
			if pin != "" {
				target += "@" + pin
			}
			if err := in.exec(in.codeArgv("--install-extension", target)); err != nil {
				return err
			}
			in.codeExtensions()[strings.ToLower(name)] = pin
			return nil
		},
		update: func(in *Installer, name string) error {
			return in.exec(in.codeArgv("--install-extension", name, "--force"))
		},
	},
}

func KnownTools() []string { return slices.Sorted(maps.Keys(toolRoutines)) }

func findToolRoutine(tool string) (toolRoutine, error) {
	r, ok := toolRoutines[tool]
	if !ok {
		return toolRoutine{}, fmt.Errorf("unknown tool %q: want one of %s", tool, strings.Join(KnownTools(), ", "))
	}
	return r, nil
}

func lookupToolPackage(installed map[string]string, name string) (string, bool) {
	if v, ok := installed[name]; ok {
		return v, true
	}
	v, ok := installed[strings.ToLower(name)]
	return v, ok
}

func (in *Installer) codeArgv(args ...string) []string {
	argv := append([]string{"code"}, args...)
	if in.Host.Euid == 0 {
		argv = append(argv, "--no-sandbox", "--user-data-dir", filepath.Join(in.Host.Getenv("HOME"), ".vscode-root"))
	}
	return argv
}

func (in *Installer) codeExtensions() map[string]string {
	if in.codeExts != nil {
		return in.codeExts
	}
	in.codeExts = map[string]string{}
	if out, ok := in.output(in.codeArgv("--list-extensions", "--show-versions")); ok {
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

func (in *Installer) InstallToolPackages(tool string, reqs []Request) error {
	r, err := findToolRoutine(tool)
	if err != nil {
		return err
	}
	pins := make(map[string]string, len(reqs))
	for _, req := range reqs {
		pin, ok := in.File.ToolPackages[tool][req.Name]
		if !ok {
			return fmt.Errorf("unknown %s package: %s (required entry in toolPackages.%s of %s)", tool, req.Name, tool, in.FilePath)
		}
		if v := req.globalVersion(); v != "" {
			pin = v
		}
		pins[req.Name] = pin
	}
	if err := in.ensureBasePackages(tool, tool); err != nil {
		return err
	}
	if !in.Opts.DryRun && !in.Host.HasCmd(r.command) {
		in.emitSkip(log.Levels.Warn, tool+" packages", r.command+" command absent")
		return nil
	}
	installed := r.list(in)
	for _, req := range reqs {
		pin := pins[req.Name]
		label := tool + "/" + req.Name
		current, present := lookupToolPackage(installed, req.Name)
		switch {
		case present && pin != "" && PinMatches(current, pin):
			in.emitPresent(log.Levels.Info, label, "already installed via "+tool)
			continue
		case present && pin == "" && !in.Opts.Update:
			in.emitPresent(log.Levels.Info, label, "already installed via "+tool)
			continue
		case present && pin == "":
			if in.Opts.DryRun {
				in.emitDryRun("update", label+" via "+tool)
				continue
			}
			if err := r.update(in, req.Name); err != nil {
				return err
			}
			in.emit(log.Levels.Info, "updated", label+" via "+tool)
			continue
		case present:
			in.emit(log.Levels.Info, "reinstall", reinstallMsg(label, current, pin))
		}
		if in.Opts.DryRun {
			in.emitDryRun("install", labelWithVersion(label, pin)+" via "+tool)
			continue
		}
		if err := r.install(in, req.Name, pin); err != nil {
			return err
		}
		in.emit(log.Levels.Info, "installed", labelWithVersion(label, pin)+" via "+tool)
	}
	return nil
}

func (in *Installer) CheckToolPackagesPresent(tool string, names []string) []string {
	r, err := findToolRoutine(tool)
	if err != nil {
		return names
	}
	installed := r.list(in)
	var missing []string
	for _, name := range names {
		if _, ok := lookupToolPackage(installed, name); ok {
			in.emit(log.Levels.Debug, "present", tool+"/"+name)
			continue
		}
		in.emit(log.Levels.Warn, "missing", tool+"/"+name)
		missing = append(missing, name)
	}
	return missing
}

// [<] 🤖🤖
