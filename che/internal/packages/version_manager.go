package packages

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

type vmTool struct {
	installed func(in *Installer, v string) bool
	global    func(in *Installer) string
	install   func(in *Installer, v string) error
	setGlobal func(in *Installer, v string) error
}

var vmTools = map[string]vmTool{
	"pyenv": {
		installed: func(in *Installer, v string) bool {
			out, ok := in.output([]string{"pyenv", "versions", "--bare"})
			return ok && slices.Contains(strings.Fields(out), v)
		},
		global: func(in *Installer) string {
			out, _ := in.output([]string{"pyenv", "global"})
			return strings.TrimSpace(out)
		},
		install: func(in *Installer, v string) error {
			return in.exec([]string{"pyenv", "install", "--skip-existing", v})
		},
		setGlobal: func(in *Installer, v string) error {
			return in.exec([]string{"pyenv", "global", v})
		},
	},
	"nvm": {
		installed: func(in *Installer, v string) bool {
			return fileExists(filepath.Join(in.Host.nvmDir(), "versions", "node", "v"+v))
		},
		global: func(in *Installer) string {
			b, err := os.ReadFile(filepath.Join(in.Host.nvmDir(), "alias", "default"))
			if err != nil {
				return ""
			}
			return strings.TrimPrefix(strings.TrimSpace(string(b)), "v")
		},
		install: func(in *Installer, v string) error {
			return in.execNvm(fmt.Sprintf("nvm install %s", v))
		},
		setGlobal: func(in *Installer, v string) error {
			return in.execNvm(fmt.Sprintf("nvm alias default %s", v))
		},
	},
}

func (in *Installer) execNvm(cmd string) error {
	sh := filepath.Join(in.Host.nvmDir(), "nvm.sh")
	return in.exec([]string{"bash", "-c", fmt.Sprintf(". %q && %s", sh, cmd)})
}

func (in *Installer) installVersionManager(pkg string, s *VersionManagerSpec) error {
	tool := vmTools[s.Tool]
	versions, global := s.Versions, s.Global
	if r, ok := in.requested[pkg]; ok && len(r.Versions) > 0 {
		versions, global = r.Versions, r.globalVersion()
	}
	if global == "" {
		global = versions[0]
	}
	var missing []string
	for _, v := range versions {
		if !tool.installed(in, v) {
			missing = append(missing, v)
		}
	}
	globalOK := tool.global(in) == global
	if len(missing) == 0 && globalOK {
		in.emitSkip(log.Levels.Debug, in.pkgLabel(pkg), "versions present via "+s.Tool)
		return nil
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", fmt.Sprintf("%s via %s (versions: %s, global: %s)", in.pkgLabel(pkg), s.Tool, strings.Join(versions, " "), global))
		return nil
	}
	for _, v := range missing {
		if err := tool.install(in, v); err != nil {
			return err
		}
	}
	if err := tool.setGlobal(in, global); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "installed", fmt.Sprintf("%s via %s (versions: %s, global: %s)", in.pkgLabel(pkg), s.Tool, strings.Join(versions, " "), global))
	return nil
}

// [<] 🤖🤖
