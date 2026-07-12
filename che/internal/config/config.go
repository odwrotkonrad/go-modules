// Package config models che's runtime options: flag values finalized by Resolve.
package config

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"os"
)

// Resolve applies env fallbacks in place (flags win) and validates the mode values.
func (c *Config) Resolve() error {
	c.DryRun = DryRunMode(cmp.Or(string(c.DryRun), os.Getenv("CHE_DRY_RUN")))
	switch c.DryRun {
	case DryRun.Off, DryRun.Delta, DryRun.All:
	default:
		return fmt.Errorf("invalid --dry-run mode %q: want delta or all", c.DryRun)
	}
	c.ValidateSpec = ValidateSpecMode(cmp.Or(string(c.ValidateSpec), os.Getenv("CHE_VALIDATE_SPEC"), string(ValidateSpec.Warn)))
	switch c.ValidateSpec {
	case ValidateSpec.Warn, ValidateSpec.Error:
	default:
		return fmt.Errorf("invalid --validate-spec mode %q: want warn or error", c.ValidateSpec)
	}
	c.Dir = cmp.Or(c.Dir, os.Getenv("CHE_DIR"))
	c.Profile = cmp.Or(c.Profile, os.Getenv("CHE_PROFILE"))
	c.SkipExecIf = boolOrEnv(c.SkipExecIf, "CHE_SKIP_EXEC_IF")
	c.SkipPlugins = boolOrEnv(c.SkipPlugins, "CHE_SKIP_PLUGINS")
	c.Debug = boolOrEnv(c.Debug, "CHE_DEBUG")
	c.RenderSkipSecrets = boolOrEnv(c.RenderSkipSecrets, "CHE_RENDER_TEMPLATES_SKIP_SECRETS")
	return nil
}

func boolOrEnv(flag bool, key string) bool {
	return flag || os.Getenv(key) != ""
}

// [<] 🤖🤖
