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

func ResolveDir(home, url, ref string) string {
	name := slug(url)
	if ref != "" {
		name += "-" + slug(ref)
	}
	return filepath.Join(fsutil.ResolveCacheHome(home), "remote-sources", name)
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

func refSuffix(ref string) string {
	if ref == "" {
		return ""
	}
	return "@" + ref
}

var checkouts = map[string]string{}

func ResetCache() { checkouts = map[string]string{} }

func EnsureCheckout(home, url, ref string) (string, error) {
	key := url + "?" + ref
	if dir, ok := checkouts[key]; ok {
		return dir, nil
	}
	dir, err := cloneOrUpdate(home, url, ref)
	if err == nil {
		checkouts[key] = dir
	}
	return dir, err
}

// CloneURL turns a schemeless host/path source into an https clone URL; scheme and scp forms pass through.
//
// [why] render remotes already default to https: an include url written the same way must clone the same way
func CloneURL(url string) string {
	if strings.Contains(url, "://") {
		return url
	}
	host, _, _ := strings.Cut(url, "/")
	if strings.Contains(host, ":") || strings.Contains(host, "@") {
		return url
	}
	return "https://" + url
}

func cloneOrUpdate(home, url, ref string) (string, error) {
	dir := ResolveDir(home, url, ref)
	msg := "remote " + url + refSuffix(ref) + " into " + fsutil.AbbreviateHome(dir, home)
	emitAction := func(action string) {
		log.Emit(log.Event{
			Level: log.Levels.Info, Scope: "init-remote-sources", Action: action,
			Msg: msg, Attrs: map[string]string{"url": url, "ref": ref, "checkout": dir},
		})
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		argv := []string{"clone", "--quiet", "--depth", "1", "--single-branch"}
		if ref != "" {
			argv = append(argv, "--branch", ref)
		}
		if err := git(append(argv, CloneURL(url), dir)...); err != nil {
			return "", fmt.Errorf("source clone %s%s: %w", url, refSuffix(ref), err)
		}
		emitAction("cloned")
		return dir, nil
	}
	headBefore, _ := gitOutput("-C", dir, "rev-parse", "HEAD")
	fetch := []string{"-C", dir, "fetch", "--quiet", "--depth", "1"}
	if ref != "" {
		fetch = append(fetch, "origin", ref)
	}
	// [why] a shallow --ff-only pull fails once the fetched history is truncated
	if err := git(fetch...); err != nil {
		// [why] a pin names one immutable commit: serving a different cached one is a silent lie
		if ref != "" {
			return "", fmt.Errorf("source fetch %s%s: %w", url, refSuffix(ref), err)
		}
		return dir, nil
	}
	if err := git("-C", dir, "reset", "--hard", "--quiet", "FETCH_HEAD"); err != nil {
		if ref != "" {
			return "", fmt.Errorf("source checkout %s%s: %w", url, refSuffix(ref), err)
		}
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
