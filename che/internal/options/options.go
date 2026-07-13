// Package options models che's runtime options: flag values finalized by Resolve.
package options

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"strings"
)

// LookupEnv is the env-lookup seam Resolve reads instead of the process env:
// key -> value ("" if unset), fed from the captured launch env by the caller.
type LookupEnv func(string) string

// Resolve finalizes the options in place, per field most-specific wins: flags
// > env vars > the user-config file (user) > the local spec's options: block
// (spec) > defaults; mode values validated. env supplies the env-var layer.
func (c *Options) Resolve(env LookupEnv, user, spec Layer) error {
	c.DryRun = DryRunMode(strOr(env, string(c.DryRun), "CHE_DRY_RUN", user.DryRun, spec.DryRun))
	switch c.DryRun {
	case DryRun.Off, DryRun.Delta, DryRun.All:
	default:
		return fmt.Errorf("invalid --dry-run mode %q: want delta or all", c.DryRun)
	}
	// [why] ValidateSpecCLI is the flag/env/user override (empty if none),
	// overriding each spec's own options.validateSpec per-spec; ValidateSpec
	// adds the local spec's own layer, then the warn default.
	c.ValidateSpecCLI = ValidateSpecMode(strOr(env, string(c.ValidateSpec), "CHE_VALIDATE_SPEC", user.ValidateSpec))
	c.ValidateSpec = ValidateSpecMode(cmp.Or(string(c.ValidateSpecCLI), spec.ValidateSpec, string(ValidateSpec.Warn)))
	switch c.ValidateSpec {
	case ValidateSpec.Warn, ValidateSpec.Error:
	default:
		return fmt.Errorf("invalid --validate-spec mode %q: want warn or error", c.ValidateSpec)
	}
	c.Dir = cmp.Or(c.Dir, env("CHE_DIR"))
	c.WorkingDirectory = cmp.Or(c.WorkingDirectory, env("CHE_WORKING_DIRECTORY"), spec.WorkingDirectory)
	c.Profiles = listOr(env, c.Profiles, "CHE_PROFILE", user.Profiles, spec.Profiles)
	c.SkipExecIf = boolOr(env, c.SkipExecIf, "CHE_SKIP_EXEC_IF")
	c.SkipRemoteRefs = boolOr(env, c.SkipRemoteRefs, "CHE_SKIP_REMOTE_REFS", user.SkipRemoteRefs, spec.SkipRemoteRefs)
	c.Debug = boolOr(env, c.Debug, "CHE_DEBUG", user.Debug, spec.Debug)
	c.RenderSkipSecrets = boolOr(env, c.RenderSkipSecrets, "CHE_RENDER_TEMPLATES_SKIP_SECRETS",
		user.RenderTemplates.SkipSecrets, spec.RenderTemplates.SkipSecrets)
	c.AutoDiscover = user.AutoDiscover
	return nil
}

// strOr resolves a string option: flag, else env, else each layer in order.
func strOr(env LookupEnv, flag, envKey string, layers ...string) string {
	return cmp.Or(append([]string{flag, env(envKey)}, layers...)...)
}

// listOr resolves a []string option: flag if set, else CHE_<key> (comma-split),
// else the first non-empty layer.
func listOr(env LookupEnv, flag []string, envKey string, layers ...[]string) []string {
	if len(flag) > 0 {
		return flag
	}
	if e := env(envKey); e != "" {
		return strings.Split(e, ",")
	}
	for _, l := range layers {
		if len(l) > 0 {
			return l
		}
	}
	return nil
}

// boolOr is true when the flag is set, the env var is non-empty, or any layer
// pointer is set true.
func boolOr(env LookupEnv, flag bool, envKey string, layers ...*bool) bool {
	if flag || env(envKey) != "" {
		return true
	}
	for _, l := range layers {
		if l != nil && *l {
			return true
		}
	}
	return false
}

// [<] 🤖🤖
