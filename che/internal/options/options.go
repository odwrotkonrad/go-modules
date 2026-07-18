// Package options models che's runtime options: flag values finalized by Resolve.
package options

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"slices"
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
	if c.DryRun == "true" {
		c.DryRun = DryRun.All
	}
	switch c.DryRun {
	case DryRun.Off, DryRun.Delta, DryRun.All:
	default:
		return fmt.Errorf("invalid --dry-run mode %q: want delta, all, or true (alias for all)", c.DryRun)
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
	c.SkipOps = listOr(env, c.SkipOps, "CHE_SKIP_OPS", user.SkipOps, spec.SkipOps)
	c.AllSkipOps = listOr(env, c.AllSkipOps, "CHE_ALL_SKIP_OPS", user.All.SkipOps, spec.All.SkipOps)
	for _, name := range slices.Concat(c.SkipOps, c.AllSkipOps) {
		if !slices.Contains(OpNames, name) {
			return fmt.Errorf("invalid skip-ops op %q: want one of %s", name, strings.Join(OpNames, ", "))
		}
	}
	c.SkipExecIf = boolOr(env, c.SkipExecIf, "CHE_SKIP_EXEC_IF")
	c.SkipRemoteRefs = boolOr(env, c.SkipRemoteRefs, "CHE_SKIP_REMOTE_REFS", user.SkipRemoteRefs, spec.SkipRemoteRefs)
	c.Debug = boolOr(env, c.Debug, "CHE_DEBUG", user.Debug, spec.Debug)
	c.RenderSkipSecrets = boolOr(env, c.RenderSkipSecrets, "CHE_RENDER_TEMPLATES_SKIP_SECRETS",
		user.RenderTemplates.SkipSecrets, spec.RenderTemplates.SkipSecrets)
	c.AutoDiscover = user.AutoDiscover
	return c.resolveOtel(env, user, spec)
}

// resolveOtel finalizes the OTLP telemetry group: env > user-config > spec >
// defaults. enabled off (default) -> the provider is a no-op regardless of the
// rest; metrics/logs/traces default on when enabled. protocol validated (grpc
// default).
func (c *Options) resolveOtel(env LookupEnv, user, spec Layer) error {
	c.Otel.Enabled = boolOr(env, false, "CHE_OTEL_ENABLED", user.Otel.Enabled, spec.Otel.Enabled)
	c.Otel.Protocol = cmp.Or(env("CHE_OTEL_PROTOCOL"), user.Otel.Protocol, spec.Otel.Protocol, "grpc")
	switch c.Otel.Protocol {
	case "grpc", "http":
	default:
		return fmt.Errorf("invalid otel.protocol %q: want grpc or http", c.Otel.Protocol)
	}
	c.Otel.Endpoint = cmp.Or(env("CHE_OTEL_ENDPOINT"), user.Otel.Endpoint, spec.Otel.Endpoint, defaultOtelEndpoint(c.Otel.Protocol))
	c.Otel.Metrics = boolDefaultTrue(env, "CHE_OTEL_METRICS", user.Otel.Metrics, spec.Otel.Metrics)
	c.Otel.Logs = boolDefaultTrue(env, "CHE_OTEL_LOGS", user.Otel.Logs, spec.Otel.Logs)
	c.Otel.Traces = boolDefaultTrue(env, "CHE_OTEL_TRACES", user.Otel.Traces, spec.Otel.Traces)
	return nil
}

// defaultOtelEndpoint is the local collector's default OTLP endpoint per transport.
func defaultOtelEndpoint(protocol string) string {
	if protocol == "http" {
		return "localhost:4318"
	}
	return "localhost:4317"
}

// boolDefaultTrue resolves an on-by-default bool: CHE_<key> ("0"/"false"/"off"/""
// -> false, else true) wins, else the first set layer pointer, else true.
func boolDefaultTrue(env LookupEnv, envKey string, layers ...*bool) bool {
	if e := env(envKey); e != "" {
		switch strings.ToLower(e) {
		case "0", "false", "off", "no":
			return false
		default:
			return true
		}
	}
	for _, l := range layers {
		if l != nil {
			return *l
		}
	}
	return true
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

// boolOr is true when the flag is set, else parses the env var (0/false/off/no
// -> false, any other non-empty -> true), else the first set layer pointer,
// else false.
func boolOr(env LookupEnv, flag bool, envKey string, layers ...*bool) bool {
	if flag {
		return true
	}
	if e := env(envKey); e != "" {
		switch strings.ToLower(e) {
		case "0", "false", "off", "no":
			return false
		default:
			return true
		}
	}
	for _, l := range layers {
		if l != nil {
			return *l
		}
	}
	return false
}

// [<] 🤖🤖
