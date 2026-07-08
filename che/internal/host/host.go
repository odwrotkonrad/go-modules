package host

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/konradodwrot/go/che/internal/fsutil"
	"gitlab.com/konradodwrot/go/che/internal/log"
)

// DryRunMode selects how a dry run reports: off (real run), delta (only dests
// that would change), all (every dest, as if nothing existed at the destination).
type DryRunMode int

const (
	DryRunOff DryRunMode = iota
	DryRunDelta
	DryRunAll
)

// Host is the live system the load ops act on: repo source tree, invoking
// identity, detected profile, mutating filesystem.
type Host struct {
	RepoRoot string // <configs> dir (contains che.yml, ci/, templates/)
	Root     string // <configs>/root, the load ops' source subtree
	Home     string
	Profile  string // "<space>/<os>-<arch>"
	mode     DryRunMode
	fs       fsutil.FS
}

func New(repoRoot, home, profile string, mode DryRunMode) Host {
	return Host{
		RepoRoot: repoRoot,
		Root:     filepath.Join(repoRoot, "root"),
		Home:     home,
		Profile:  profile,
		mode:     mode,
		fs:       fsutil.FS{Home: home, Mode: mode.log()},
	}
}

// log maps a DryRunMode to the log-layer dry-run mode (subtype rendering).
func (m DryRunMode) log() log.DryRun {
	switch m {
	case DryRunDelta:
		return log.Delta
	case DryRunAll:
		return log.All
	default:
		return log.Off
	}
}

// DryRun reports whether this is any dry run (delta or all).
func (h Host) DryRun() bool { return h.mode != DryRunOff }

// DryRunAll reports the full-insert-set dry run, bypassing "already settled" skips.
func (h Host) DryRunAll() bool { return h.mode == DryRunAll }

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

// ToDest maps a repo-relative path (under root/) to its live dest:
// HOME or HOME/<rest> -> $HOME[/<rest>], else /<rest>.
func (h Host) ToDest(rel string) string {
	if rel == "HOME" {
		return h.Home
	}
	if rest, ok := strings.CutPrefix(rel, "HOME/"); ok {
		return filepath.Join(h.Home, rest)
	}
	if strings.HasPrefix(rel, "/") {
		return rel // already a live absolute path (make-extra-dirs entries)
	}
	return "/" + rel
}

// UnderHome reports whether dest is the user-owned $HOME tree (no sudo needed).
func (h Host) UnderHome(dest string) bool { return h.fs.UnderHome(dest) }

// [<] 🤖🤖
