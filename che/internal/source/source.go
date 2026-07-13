// Package source manages the cache checkouts of remote source repos, kept at
// ~/.cache/che/sources.
package source

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

// Dir is the managed cache checkout path for url:
// <ResolveCacheHome>/sources/<slug> (default ~/.cache/che/sources).
func Dir(home, url string) string {
	return filepath.Join(fsutil.ResolveCacheHome(home), "sources", slug(url))
}

// slug sanitizes url into a deterministic dir name: scheme/git@ stripped,
// .git trimmed, path separators flattened.
func slug(url string) string {
	s := url
	for _, p := range []string{"ssh://", "https://", "http://", "git://", "file://"} {
		s = strings.TrimPrefix(s, p)
	}
	s = strings.TrimPrefix(s, "git@")
	s = strings.TrimSuffix(s, ".git")
	s = strings.NewReplacer("/", "-", ":", "-").Replace(s)
	return strings.Trim(s, "-")
}

// EnsureCheckout clones url into its cache dir (first run: shallow, remote default
// branch only) or updates it to the remote tip (shallow fetch + hard reset:
// [why] a shallow --ff-only pull fails once the fetched history is truncated;
// the dir is a managed cache, never edited in place), returning the checkout
// path. name subtypes the logs. Shells out to system git so the user's ssh
// config and credential helpers apply.
func EnsureCheckout(home, url, name string) (string, error) {
	dir := Dir(home, url)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		log.Debug("source("+name+")", fmt.Sprintf("clone %s -> %s", url, dir), log.Off)
		if err := git("clone", "--quiet", "--depth", "1", "--single-branch", url, dir); err != nil {
			return "", fmt.Errorf("source clone %s: %w", url, err)
		}
		log.Msg("source("+name+")", fmt.Sprintf("cloned %s -> %s", url, dir), log.Off)
		return dir, nil
	}
	log.Debug("source("+name+")", fmt.Sprintf("pull %s", dir), log.Off)
	before, _ := gitOut("-C", dir, "rev-parse", "HEAD")
	if err := git("-C", dir, "fetch", "--quiet", "--depth", "1"); err != nil {
		return "", fmt.Errorf("source fetch %s: %w", dir, err)
	}
	if err := git("-C", dir, "reset", "--hard", "--quiet", "FETCH_HEAD"); err != nil {
		return "", fmt.Errorf("source reset %s: %w", dir, err)
	}
	if after, _ := gitOut("-C", dir, "rev-parse", "HEAD"); after != before {
		log.Msg("source("+name+")", fmt.Sprintf("pulled %.7s..%.7s %s", before, after, dir), log.Off)
	}
	return dir, nil
}

func git(args ...string) error {
	return execx.Default.Exec(execx.Cmd{Argv: append([]string{"git"}, args...), Stderr: os.Stderr})
}

// gitOut runs git, returning its trimmed stdout.
func gitOut(args ...string) (string, error) {
	out, err := execx.Default.Output(execx.Cmd{Argv: append([]string{"git"}, args...), Stderr: os.Stderr})
	return strings.TrimSpace(string(out)), err
}

// [<] 🤖🤖
