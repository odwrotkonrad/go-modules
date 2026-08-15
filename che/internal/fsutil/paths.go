package fsutil

// [>] 🤖🤖

import (
	"cmp"
	"os"
	"path/filepath"
	"strings"
)

func IsUnder(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+"/")
}

func ExpandHome(p, home string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return p
}

func AbbreviateHome(path, home string) string {
	if home != "" && strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func ResolveCacheHome(home string) string {
	return resolveBaseDir("CHE_CACHE_HOME", "XDG_CACHE_HOME", home, ".cache")
}

func ResolveStateHome(home string) string {
	return resolveBaseDir("CHE_STATE_HOME", "XDG_STATE_HOME", home, ".local/state")
}

func ResolveConfigHome(home string) string {
	return resolveBaseDir("CHE_CONFIG_HOME", "XDG_CONFIG_HOME", home, ".config")
}

func ResolveUserConfigPath(home string) string {
	return filepath.Join(ResolveConfigHome(home), "config.yml")
}

func resolveBaseDir(cheEnv, xdgEnv, home, defaultRel string) string {
	if che := os.Getenv(cheEnv); che != "" {
		return che
	}
	base := cmp.Or(os.Getenv(xdgEnv), filepath.Join(home, defaultRel))
	return filepath.Join(base, "che")
}

// [<] 🤖🤖
