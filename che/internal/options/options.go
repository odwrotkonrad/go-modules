// Package options models che's runtime options: flag values finalized by Resolve.
package options

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"os"
)

// Resolve finalizes the options in place: flags win over env vars, env vars
// over the local spec's options: layer, then defaults; mode values validated.
func (c *Options) Resolve(l SpecLayer) error {
	c.DryRun = DryRunMode(cmp.Or(string(c.DryRun), os.Getenv("CHE_DRY_RUN")))
	switch c.DryRun {
	case DryRun.Off, DryRun.Delta, DryRun.All:
	default:
		return fmt.Errorf("invalid --dry-run mode %q: want delta or all", c.DryRun)
	}
	c.ValidateSpecCLI = ValidateSpecMode(cmp.Or(string(c.ValidateSpec), os.Getenv("CHE_VALIDATE_SPEC")))
	c.ValidateSpec = ValidateSpecMode(cmp.Or(string(c.ValidateSpecCLI), l.ValidateSpec, string(ValidateSpec.Warn)))
	switch c.ValidateSpec {
	case ValidateSpec.Warn, ValidateSpec.Error:
	default:
		return fmt.Errorf("invalid --validate-spec mode %q: want warn or error", c.ValidateSpec)
	}
	c.Dir = cmp.Or(c.Dir, os.Getenv("CHE_DIR"))
	c.WorkingDirectory = cmp.Or(c.WorkingDirectory, os.Getenv("CHE_WORKING_DIRECTORY"))
	c.Profile = cmp.Or(c.Profile, os.Getenv("CHE_PROFILE"))
	c.SkipExecIf = boolOrEnv(c.SkipExecIf, "CHE_SKIP_EXEC_IF")
	c.SkipRemoteRefs = boolOrEnv(c.SkipRemoteRefs, "CHE_SKIP_REMOTE_REFS")
	c.Debug = boolOrEnv(c.Debug, "CHE_DEBUG") || (l.Debug != nil && *l.Debug)
	c.RenderSkipSecrets = boolOrEnv(c.RenderSkipSecrets, "CHE_RENDER_TEMPLATES_SKIP_SECRETS")
	return nil
}

func boolOrEnv(flag bool, key string) bool {
	return flag || os.Getenv(key) != ""
}

// [<] 🤖🤖
