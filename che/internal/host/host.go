// Package host applies resolved ops onto a target host: links, copies, renders, dirs, services, anchored at a repo root and home.
package host

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/options"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

func (g gitFetcher) Fetch(ref string) (string, error) { return g.fetch(ref) }

// New builds a Host anchored at repoRoot (che.yml + repo-relative
// scripts/templates), with root the resolved load-ops source tree (the
// options.workingDirectory value; the RootPrefix logical token and the HOME/
// folder map onto it).
func New(repoRoot, root, home, profile string, cfg options.Options) Host {
	return Host{
		RepoRoot: repoRoot,
		Root:     root,
		Home:     home,
		Profile:  profile,
		cfg:      cfg,
		fs:       fsutil.FS{Home: home},
		reader:   fsutil.OSReader{},
		fetcher:  gitFetcher{fetch: render.NewRemoteFetcher()},
	}
}

// WithFetcher: copy fetching remote template sources through f (test injection).
func (h Host) WithFetcher(f RemoteFetcher) Host {
	h.fetcher = f
	return h
}

// WithLogSub: copy logging s as a trailing subtype word (e.g. "profile=<name>").
func (h Host) WithLogSub(s string) Host {
	h.logSub = s
	return h
}

// WithFS: copy running mutating fs ops through fs (test injection).
func (h Host) WithFS(fs fsutil.FileSystemWriter) Host {
	h.fs = fs
	return h
}

// WithFSReader: copy running dest-facing reads through r (test injection).
func (h Host) WithFSReader(r fsutil.FileSystemReader) Host {
	h.reader = r
	return h
}

func (h Host) log(title, msg string) { log.MsgSub(title, msg, logMode(h.cfg.DryRun), h.logSub) }

// mutate is the one dry-run+log gate for every mutating op: dry run logs only
// (fs untouched); real run executes fn, then logs.
func (h Host) mutate(title, msg string, fn func() error) error {
	if !h.IsDryRun() {
		if err := fn(); err != nil {
			return err
		}
	}
	h.log(title, msg)
	return nil
}

func logMode(m options.DryRunMode) log.DryRun {
	switch m {
	case options.DryRun.Delta:
		return log.Delta
	case options.DryRun.All:
		return log.All
	default:
		return log.Off
	}
}

// IsDryRun reports whether this is any dry run (delta or all).
func (h Host) IsDryRun() bool { return h.cfg.DryRun != options.DryRun.Off }

// Src maps a repo-relative path (under root/) to its absolute source path.
func (h Host) Src(rel string) string { return filepath.Join(h.Root, rel) }

// ResolveScripts maps spec-resolved script rels (globs already expanded by
// spec.Resolve) to absolute paths, IN SPEC ORDER, verifying each exists.
func (h Host) ResolveScripts(rels []string) ([]string, error) {
	out := make([]string, len(rels))
	for i, rel := range rels {
		abs := filepath.Join(h.RepoRoot, rel)
		if _, err := os.Stat(abs); err != nil {
			return nil, fmt.Errorf("run-scripts script not found: %s", rel)
		}
		out[i] = abs
	}
	return out, nil
}

// ToDest maps a working-tree rel path to its live dest. Env vars expand first
// (so specs can write $HOME/... dests), $HOME resolving to the invoking user's
// home (h.Home, correct under sudo where the process $HOME differs). Then: an
// already-absolute path stays (make-extra-dirs entries, $HOME-rooted dests),
// the HOME tree folder maps onto h.Home, everything else is a system-root path.
func (h Host) ToDest(rel string) string {
	rel = h.expandEnv(rel)
	if rel == "HOME" {
		return h.Home
	}
	if rest, ok := strings.CutPrefix(rel, "HOME/"); ok {
		return filepath.Join(h.Home, rest)
	}
	if strings.HasPrefix(rel, "/") {
		return rel
	}
	return "/" + rel
}

// expandEnv expands env vars in p, with $HOME/${HOME} bound to h.Home ([why]
// the invoking user's home, not the process env, which diverges under sudo).
func (h Host) expandEnv(p string) string {
	return os.Expand(p, func(k string) string {
		if k == "HOME" {
			return h.Home
		}
		return os.Getenv(k)
	})
}

// [<] 🤖🤖
