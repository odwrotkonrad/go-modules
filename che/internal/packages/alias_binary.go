package packages

// [>] 🤖🤖

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) aliasBinaries(pkg string, it Item) error {
	if len(it.AliasBinary) == 0 {
		return nil
	}
	binDir := in.userBinDir()
	for _, from := range slices.Sorted(maps.Keys(it.AliasBinary)) {
		to := it.AliasBinary[from]
		src, err := in.Host.LookPath(from)
		if err != nil {
			continue
		}
		dest := filepath.Join(binDir, to)
		if resolved, err := os.Readlink(dest); err == nil && resolved == src {
			continue
		}
		if _, err := in.Host.LookPath(to); err == nil && !fileExists(dest) {
			in.emitSkip(log.Levels.Debug, pkg, to+" already resolves without an alias")
			continue
		}
		if in.Opts.DryRun {
			in.emitDryRun("alias", pkg+": "+to+" -> "+src)
			continue
		}
		if err := in.exec([]string{"mkdir", "-p", binDir}); err != nil {
			return err
		}
		if err := in.exec([]string{"ln", "-sf", src, dest}); err != nil {
			return err
		}
		in.emit(log.Levels.Info, "aliased", pkg+": "+dest+" -> "+src)
	}
	return nil
}

// [<] 🤖🤖

func (in *Installer) runEntryPostInstall(pkg string, e Entry) error {
	if e.PostInstall == nil {
		return nil
	}
	if in.Opts.DryRun {
		in.emitDryRun("postInstall", pkg)
		return nil
	}
	argv, cleanup, err := in.scriptArgv(pkg, e.PostInstall)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	c := execx.Cmd{Argv: argv, Env: in.scriptEnv(pkg, e.PostInstall), Stdout: os.Stdout, Stderr: os.Stderr}
	if err := execx.Default.Exec(c); err != nil {
		return fmt.Errorf("%s: postInstall: %w", pkg, err)
	}
	in.emit(log.Levels.Info, "post-installed", pkg)
	return nil
}
