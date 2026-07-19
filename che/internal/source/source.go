// Package source manages the cache checkouts of remote source repos, kept at
// ~/.cache/che/remote-sources.
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
// <ResolveCacheHome>/remote-sources/<slug> (default ~/.cache/che/remote-sources).
func Dir(home, url string) string {
	return filepath.Join(fsutil.ResolveCacheHome(home), "remote-sources", slug(url))
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
// path. Shells out to system git so the user's ssh
// config and credential helpers apply.
// checkouts caches ensured URLs for the run: the init stage clones/pulls,
// later resolutions reuse the checkout silently (one fetch per URL per run).
var checkouts = map[string]string{}

// ResetCache clears the per-run checkout cache (tests simulating fresh runs).
func ResetCache() { checkouts = map[string]string{} }

func EnsureCheckout(home, url string) (string, error) {
	if dir, ok := checkouts[url]; ok {
		return dir, nil
	}
	dir, err := ensureCheckout(home, url)
	if err == nil {
		checkouts[url] = dir
	}
	return dir, err
}

// ensureCheckout logs one "<git-url> -> <os-path>" line per source
// (spec/che/InitCmdBehavior.md): cloneRemote, updateRemote and
// skipDueRemoteUpToDate (up to date) all at info.
func ensureCheckout(home, url string) (string, error) {
	dir := Dir(home, url)
	line := url + " -> " + dir
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if err := git("clone", "--quiet", "--depth", "1", "--single-branch", url, dir); err != nil {
			return "", fmt.Errorf("source clone %s: %w", url, err)
		}
		log.Msg("init-remote-sources(cloneRemote)", line, log.Off)
		return dir, nil
	}
	before, _ := gitOut("-C", dir, "rev-parse", "HEAD")
	// [why] a failing update with a cached checkout is survivable: warn and use
	// the cache; only a missing checkout is fatal (spec/che/InitCmdBehavior.md).
	if err := git("-C", dir, "fetch", "--quiet", "--depth", "1"); err != nil {
		log.Msg("init-remote-sources(warning)", fmt.Sprintf("fetch failed, using cached checkout %s: %v", dir, err), log.Off)
		return dir, nil
	}
	if err := git("-C", dir, "reset", "--hard", "--quiet", "FETCH_HEAD"); err != nil {
		log.Msg("init-remote-sources(warning)", fmt.Sprintf("update failed, using cached checkout %s: %v", dir, err), log.Off)
		return dir, nil
	}
	if after, _ := gitOut("-C", dir, "rev-parse", "HEAD"); after != before {
		log.Msg("init-remote-sources(updateRemote)", line, log.Off)
	} else {
		log.Msg("init-remote-sources(update, skippedDue[RemoteUpToDate])", line, log.Off)
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
