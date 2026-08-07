package packages

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) installCompletions(pkg string, e Entry) error {
	def := e.Completions.Zsh
	if def == nil || !in.Opts.CompletionsEnabled {
		return nil
	}
	dir := in.completionsDir()
	name := def.Name
	if name == "" {
		name = "_" + pkg
	}
	dest := filepath.Join(dir, name)
	if _, err := os.Stat(dest); err == nil {
		in.emitSkip(log.Levels.Debug, pkg+" zsh completions", "already present at "+dest)
		return nil
	}
	if in.Opts.DryRun {
		in.emitDryRun("install", pkg+" zsh completions -> "+dest)
		return nil
	}
	tmp, err := os.MkdirTemp("", "che-completions-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	asset := filepath.Join(tmp, name)
	if def.Cmd != "" {
		out, ok := in.output([]string{"sh", "-ec", def.Cmd})
		if !ok {
			return fmt.Errorf("%s: completions cmd failed: %s", pkg, def.Cmd)
		}
		if err := os.WriteFile(asset, []byte(out), 0o644); err != nil {
			return err
		}
	} else {
		if err := in.exec(curlArgv(def.URL, asset)); err != nil {
			return err
		}
	}
	if def.Sha256 != "" {
		if err := in.verifySha256(pkg, asset, def.Sha256); err != nil {
			return err
		}
	}
	if err := in.exec([]string{"mkdir", "-p", dir}); err != nil {
		return err
	}
	if err := in.exec([]string{"install", "-m", "0644", asset, dest}); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "installed", pkg+" zsh completions -> "+dest)
	return nil
}

func (in *Installer) completionsDir() string {
	return in.resolveDestDir(&in.compDir, in.Opts.CompletionsDestinationCandidates, DefaultCompletionsDestinationCandidates,
		in.Opts.CompletionsCheckPresentOnFpath, in.fpath,
		"packages.completions.zsh.installDestinationCandidates", "fpath")
}

func (in *Installer) fpath() []string {
	if in.Host.FpathDirs == nil {
		return nil
	}
	return in.Host.FpathDirs()
}

// [<] 🤖🤖
