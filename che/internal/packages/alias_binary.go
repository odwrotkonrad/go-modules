package packages

// [>] 🤖🤖

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) aliasBinaries(pkg string, it Item) error {
	if len(it.AliasBinary) == 0 {
		return nil
	}
	binDir := in.resolveBinDir()
	for _, from := range slices.Sorted(maps.Keys(it.AliasBinary)) {
		to := it.AliasBinary[from]
		src, err := in.Host.LookPath(from)
		if err != nil {
			continue
		}
		pathDest := strings.ContainsRune(to, '/')
		dest := filepath.Join(binDir, to)
		if pathDest {
			dest = in.expandPath(to)
		}
		if resolved, err := os.Readlink(dest); err == nil && resolved == src {
			continue
		}
		if !pathDest {
			if _, err := in.Host.LookPath(to); err == nil && !fileExists(dest) {
				in.emitPresent(log.Levels.Debug, pkg, to+" already resolves without an alias")
				continue
			}
		}
		if in.Opts.DryRun {
			in.emitDryRun("alias", pkg+": "+to+" -> "+src)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := makeSymlink(src, dest); err != nil {
			return err
		}
		in.emit(log.Levels.Info, "aliased", pkg+": "+dest+" -> "+src)
	}
	return nil
}

func (in *Installer) runEntryPostInstall(pkg string, e Entry) error {
	if e.PostInstall == nil {
		return nil
	}
	if in.Opts.DryRun {
		in.emitDryRun("postInstall", pkg)
		return nil
	}
	if err := in.runScript(pkg, e.PostInstall, "postInstall"); err != nil {
		return err
	}
	in.emit(log.Levels.Info, "post-installed", pkg)
	return nil
}

// [<] 🤖🤖
