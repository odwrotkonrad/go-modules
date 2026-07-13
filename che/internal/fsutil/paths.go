package fsutil

// [>] 🤖🤖

import (
	"cmp"
	"os"
	"path/filepath"
)

// ResolveDataHome resolves che's XDG data base dir:
// CHE_DATA_HOME (che's base directly), else XDG_DATA_HOME/che, else ~/.local/share/che.
// Reserved: honored by the resolver but no che path uses it today.
func ResolveDataHome(home string) string {
	return resolveBaseDir("CHE_DATA_HOME", "XDG_DATA_HOME", home, ".local/share")
}

// ResolveCacheHome resolves che's XDG cache base dir:
// CHE_CACHE_HOME (che's base directly), else XDG_CACHE_HOME/che, else ~/.cache/che.
func ResolveCacheHome(home string) string {
	return resolveBaseDir("CHE_CACHE_HOME", "XDG_CACHE_HOME", home, ".cache")
}

// ResolveStateHome resolves che's XDG state base dir:
// CHE_STATE_HOME (che's base directly), else XDG_STATE_HOME/che, else ~/.local/state/che.
func ResolveStateHome(home string) string {
	return resolveBaseDir("CHE_STATE_HOME", "XDG_STATE_HOME", home, ".local/state")
}

// ResolveConfigHome resolves che's XDG config base dir:
// CHE_CONFIG_HOME (che's base directly), else XDG_CONFIG_HOME/che, else ~/.config/che.
func ResolveConfigHome(home string) string {
	return resolveBaseDir("CHE_CONFIG_HOME", "XDG_CONFIG_HOME", home, ".config")
}

// ResolveUserConfigPath is the user-config file: <ResolveConfigHome>/config.yml
// (default ~/.config/che/config.yml).
func ResolveUserConfigPath(home string) string {
	return filepath.Join(ResolveConfigHome(home), "config.yml")
}

// resolveBaseDir applies the per-base precedence: CHE override (che's base
// directly, no /che suffix), else XDG base + /che, else ~/<defaultRel>/che.
func resolveBaseDir(cheEnv, xdgEnv, home, defaultRel string) string {
	if che := os.Getenv(cheEnv); che != "" {
		return che
	}
	base := cmp.Or(os.Getenv(xdgEnv), filepath.Join(home, defaultRel))
	return filepath.Join(base, "che")
}

// [<] 🤖🤖
