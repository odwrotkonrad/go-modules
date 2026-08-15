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
	slug := url
	for _, scheme := range []string{"ssh://", "https://", "http://", "git://", "file://"} {
		slug = strings.TrimPrefix(slug, scheme)
	}
	slug = strings.TrimPrefix(slug, "git@")
	slug = strings.TrimSuffix(slug, ".git")
	slug = strings.NewReplacer("/", "-", ":", "-").Replace(slug)
	return strings.Trim(slug, "-")
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
	msg := "remote " + url + " into " + fsutil.AbbreviateHome(dir, home)
	emitAction := func(action string) {
		log.Emit(log.Event{
			Level: log.Levels.Info, Scope: "init-remote-sources", Action: action,
			Msg: msg, Attrs: map[string]string{"url": url, "checkout": dir},
		})
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if err := git("clone", "--quiet", "--depth", "1", "--single-branch", url, dir); err != nil {
			return "", fmt.Errorf("source clone %s: %w", url, err)
		}
		emitAction("cloned")
		return dir, nil
	}
	headBefore, _ := gitOutput("-C", dir, "rev-parse", "HEAD")
	// [why] a shallow --ff-only pull fails once the fetched history is truncated
	if err := git("-C", dir, "fetch", "--quiet", "--depth", "1"); err != nil {
		log.EmitWarn("init-remote-sources", "warning", fmt.Sprintf("fetch failed, using cached checkout %s: %v", dir, err))
		return dir, nil
	}
	if err := git("-C", dir, "reset", "--hard", "--quiet", "FETCH_HEAD"); err != nil {
		log.EmitWarn("init-remote-sources", "warning", fmt.Sprintf("update failed, using cached checkout %s: %v", dir, err))
		return dir, nil
	}
	if headAfter, _ := gitOutput("-C", dir, "rev-parse", "HEAD"); headAfter != headBefore {
		emitAction("updated")
	} else {
		emitAction("up-to-date")
	}
	return dir, nil
}

func git(args ...string) error {
	return execx.Default.Exec(execx.Cmd{Argv: append([]string{"git"}, args...), Stderr: os.Stderr})
}

func gitOutput(args ...string) (string, error) {
	out, err := execx.Default.Output(execx.Cmd{Argv: append([]string{"git"}, args...), Stderr: os.Stderr})
	return strings.TrimSpace(string(out)), err
}

// [<] 🤖🤖
