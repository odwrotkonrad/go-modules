package packages

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

func (in *Installer) resolveManDir() string {
	return in.resolveDestDir(&in.manDir, in.Opts.ManpagesDestinationCandidates, DefaultManpagesDestinationCandidates,
		in.Opts.ManpagesCheckPresentOnManpath, in.manpathDirs,
		"packages.manpages.installDestinationCandidates", "MANPATH")
}

func (in *Installer) manpathDirs() []string {
	if in.Host.ManpathDirs == nil {
		return nil
	}
	return in.Host.ManpathDirs()
}

func (in *Installer) findManpages(pkg string, entry Entry) []string {
	if it, picked, err := in.pickItem(pkg, entry); err == nil && picked && it.Manpages != nil {
		return it.Manpages
	}
	return entry.Manpages
}

func (in *Installer) installManpageMembers(pkg, opt, version, arch string, b *BinariesRemoteArchiveSpec) error {
	if len(b.ExtractManpages) == 0 {
		return nil
	}
	manDir := in.resolveManDir()
	for _, m := range b.ExtractManpages {
		m = in.Host.expandTokens(m, version, arch)
		src := filepath.Join(opt, m)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("%s: manpage member not in archive: %s", pkg, m)
		}
		_, section, err := ParseManpage(path.Base(m))
		if err != nil {
			return fmt.Errorf("%s: %w", pkg, err)
		}
		dest := filepath.Join(manDir, "man"+section, path.Base(m))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := makeSymlink(src, dest); err != nil {
			return err
		}
	}
	return nil
}

func (in *Installer) warnManDirOffManpath(pkg, dir string) {
	if slices.Contains(in.manpathDirs(), dir) {
		return
	}
	in.emit(log.Levels.Warn, "not-on-manpath", pkg+": "+dir+" is not on the man search path")
}

func (in *Installer) CheckManpages(pkgs []string) error {
	for _, pkg := range pkgs {
		entry, err := in.findEntry(pkg)
		if err != nil {
			return err
		}
		for _, page := range in.findManpages(pkg, entry) {
			base, section, err := ParseManpage(page)
			if err != nil {
				return fmt.Errorf("%s: %w", pkg, err)
			}
			hits := in.findManpageHits(base, section)
			switch {
			case len(hits) == 0:
				in.emit(log.Levels.Warn, "missing-manpage", pkg+": "+page+" resolves nowhere on the man search path")
			case len(hits) > 1:
				in.emit(log.Levels.Warn, "multiple-present", pkg+": manpage "+page+": "+strings.Join(hits, ", "))
			}
		}
	}
	return nil
}

func (in *Installer) findManpageHits(base, section string) []string {
	var hits []string
	for _, dir := range in.manpathDirs() {
		matches, err := filepath.Glob(filepath.Join(dir, "man"+section, base+"."+section+"*"))
		if err != nil || len(matches) == 0 {
			continue
		}
		hits = append(hits, matches[0])
	}
	return hits
}

// [<] 🤖🤖
