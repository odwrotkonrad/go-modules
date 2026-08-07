package fsutil

// [>] 🤖🤖

import (
	"cmp"
	"os"
	"path/filepath"
)

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
