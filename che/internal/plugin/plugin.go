// Package plugin manages the cache checkouts of remote plugin repos.
package plugin

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

// Dir is the managed cache checkout path for url:
// <home>/.local/share/che/plugins/<slug>.
func Dir(home, url string) string {
	return filepath.Join(home, ".local/share/che/plugins", slug(url))
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

// Ensure clones url into its cache dir (first run: shallow, remote default
// branch only) or updates it to the remote tip (shallow fetch + hard reset:
// [why] a shallow --ff-only pull fails once the fetched history is truncated;
// the dir is a managed cache, never edited in place), returning the checkout
// path. name subtypes the logs. Shells out to system git so the user's ssh
// config and credential helpers apply.
func Ensure(home, url, name string) (string, error) {
	dir := Dir(home, url)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		log.Debug("plugin("+name+")", fmt.Sprintf("clone %s -> %s", url, dir), log.Off)
		if err := git("clone", "--quiet", "--depth", "1", "--single-branch", url, dir); err != nil {
			return "", fmt.Errorf("plugin clone %s: %w", url, err)
		}
		log.Msg("plugin("+name+")", fmt.Sprintf("cloned %s -> %s", url, dir), log.Off)
		return dir, nil
	}
	log.Debug("plugin("+name+")", fmt.Sprintf("pull %s", dir), log.Off)
	before, _ := gitOut("-C", dir, "rev-parse", "HEAD")
	if err := git("-C", dir, "fetch", "--quiet", "--depth", "1"); err != nil {
		return "", fmt.Errorf("plugin fetch %s: %w", dir, err)
	}
	if err := git("-C", dir, "reset", "--hard", "--quiet", "FETCH_HEAD"); err != nil {
		return "", fmt.Errorf("plugin reset %s: %w", dir, err)
	}
	if after, _ := gitOut("-C", dir, "rev-parse", "HEAD"); after != before {
		log.Msg("plugin("+name+")", fmt.Sprintf("pulled %.7s..%.7s %s", before, after, dir), log.Off)
	}
	return dir, nil
}

func git(args ...string) error {
	c := exec.Command("git", args...)
	c.Stderr = os.Stderr
	return c.Run()
}

// gitOut runs git, returning its trimmed stdout.
func gitOut(args ...string) (string, error) {
	c := exec.Command("git", args...)
	c.Stderr = os.Stderr
	out, err := c.Output()
	return strings.TrimSpace(string(out)), err
}

// [<] 🤖🤖
