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

func ResolveDir(home, url string) string {
	return filepath.Join(fsutil.ResolveCacheHome(home), "remote-sources", slug(url))
}

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

var checkouts = map[string]string{}

func ResetCache() { checkouts = map[string]string{} }

func EnsureCheckout(home, url string) (string, error) {
	if dir, ok := checkouts[url]; ok {
		return dir, nil
	}
	dir, err := cloneOrUpdate(home, url)
	if err == nil {
		checkouts[url] = dir
	}
	return dir, err
}

func cloneOrUpdate(home, url string) (string, error) {
	dir := ResolveDir(home, url)
	line := "remote " + url + " into " + fsutil.AbbreviateHome(dir, home)
	emit := func(action string) {
		log.Emit(log.Event{
			Level: log.Levels.Info, Scope: "init-remote-sources", Action: action,
			Msg: line, Attrs: map[string]string{"url": url, "checkout": dir},
		})
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if err := git("clone", "--quiet", "--depth", "1", "--single-branch", url, dir); err != nil {
			return "", fmt.Errorf("source clone %s: %w", url, err)
		}
		emit("cloned")
		return dir, nil
	}
	before, _ := gitOut("-C", dir, "rev-parse", "HEAD")
	// [why] a shallow --ff-only pull fails once the fetched history is truncated
	if err := git("-C", dir, "fetch", "--quiet", "--depth", "1"); err != nil {
		log.EmitWarn("init-remote-sources", "warning", fmt.Sprintf("fetch failed, using cached checkout %s: %v", dir, err))
		return dir, nil
	}
	if err := git("-C", dir, "reset", "--hard", "--quiet", "FETCH_HEAD"); err != nil {
		log.EmitWarn("init-remote-sources", "warning", fmt.Sprintf("update failed, using cached checkout %s: %v", dir, err))
		return dir, nil
	}
	if after, _ := gitOut("-C", dir, "rev-parse", "HEAD"); after != before {
		emit("updated")
	} else {
		emit("up-to-date")
	}
	return dir, nil
}

func git(args ...string) error {
	return execx.Default.Exec(execx.Cmd{Argv: append([]string{"git"}, args...), Stderr: os.Stderr})
}

func gitOut(args ...string) (string, error) {
	out, err := execx.Default.Output(execx.Cmd{Argv: append([]string{"git"}, args...), Stderr: os.Stderr})
	return strings.TrimSpace(string(out)), err
}

// [<] 🤖🤖
