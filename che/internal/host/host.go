package host

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

// TrackedFiles lists git-tracked files under root/, repo-relative to root.
func (h Host) TrackedFiles() ([]string, error) { return fsutil.TrackedFiles(h.Root) }

// ResolveScripts expands the scripts list IN SPEC ORDER (no sort). Each entry must
// resolve to >=1 script ([why] catches typos/renames). Globs expand in place.
func (h Host) ResolveScripts(scripts []string) ([]string, error) {
	var out []string
	for _, entry := range scripts {
		abs := filepath.Join(h.RepoRoot, entry)
		if strings.ContainsAny(entry, "*?[") {
			hits, err := filepath.Glob(abs)
			if err != nil {
				return nil, err
			}
			if len(hits) == 0 {
				return nil, fmt.Errorf("run-scripts entry matched no script: %s", entry)
			}
			slices.Sort(hits)
			out = append(out, hits...)
			continue
		}
		if _, err := os.Stat(abs); err != nil {
			return nil, fmt.Errorf("run-scripts script not found: %s", entry)
		}
		out = append(out, abs)
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

// InvokingUser: root runs the ops; HOME-tree dirs belong to the invoking user.
func (h Host) InvokingUser() string {
	if u := os.Getenv("SUDO_USER"); u != "" {
		return u
	}
	if out, err := exec.Command("stat", "-f", "%Su", h.Home).Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// [<] 🤖🤖
