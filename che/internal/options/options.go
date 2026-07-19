// Package options models che's runtime options: flag values finalized by Resolve.
package options

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// LookupEnv is the env-lookup seam Resolve reads instead of the process env:
// key -> value ("" if unset), fed from the captured launch env by the caller.
type LookupEnv func(string) string

// Setting is one resolved option: its key, final value, and the layer that
// decided it (cliFlag | env | config-file | specFile | default). The config log renders
// these (spec/che/LogBehavior.md).
type Setting struct {
	Key    string
	Value  string
	Source string
}

// SettingsDelta lists the settings a non-default layer decided.
func (c Options) SettingsDelta() []Setting {
	var out []Setting
	for _, s := range c.Settings {
		if s.Source != "default" {
			out = append(out, s)
		}
	}
	return out
}

// FormatSettings renders settings as "[key=value(source),...]".
func FormatSettings(settings []Setting) string {
	parts := make([]string, len(settings))
	for i, s := range settings {
		parts[i] = fmt.Sprintf("%s=%s(%s)", s.Key, s.Value, s.Source)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// cand pairs a candidate value with its source layer, in precedence order.
type cand struct {
	val string
	set bool
	src string
}

func flagStr(v string) cand    { return cand{v, v != "", "cliFlag"} }
func envStr(v string) cand     { return cand{v, v != "", "env"} }
func layer(v, src string) cand { return cand{v, v != "", src} }

func layerList(v []string, src string) cand {
	return cand{strings.Join(v, ","), len(v) > 0, src}
}

// boolLayer is one *bool layer candidate.
type boolLayer struct {
	val *bool
	src string
}

func parseBoolWord(s string) bool {
	switch strings.ToLower(s) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

func (c *Options) record(key, value, source string) {
	c.Settings = append(c.Settings, Setting{Key: key, Value: value, Source: source})
}

// resolveStr picks the first set candidate (else def) and records it.
func (c *Options) resolveStr(key, def string, candidates ...cand) string {
	for _, cd := range candidates {
		if cd.set {
			c.record(key, cd.val, cd.src)
			return cd.val
		}
	}
	c.record(key, def, "default")
	return def
}

// resolveList picks the first set candidate (env comma-split) and records it
// (an empty list displays as []).
func (c *Options) resolveList(key string, candidates ...cand) []string {
	v := c.resolveStr(key, "", candidates...)
	if v == "" {
		c.fillDefault(key, "[]")
		return nil
	}
	return strings.Split(v, ",")
}

// setValue rewrites the recorded display value for key (alias normalization).
func (c *Options) setValue(key, value string) {
	for i := range c.Settings {
		if c.Settings[i].Key == key {
			c.Settings[i].Value = value
		}
	}
}

// fillDefault sets the recorded default display value for key (display only,
// the resolved option itself is untouched).
func (c *Options) fillDefault(key, value string) {
	for i := range c.Settings {
		if c.Settings[i].Key == key && c.Settings[i].Source == "default" && c.Settings[i].Value == "" {
			c.Settings[i].Value = value
		}
	}
}

// FillDefaultSetting is fillDefault for callers outside Resolve (the actual
// run directory, known only at the CLI boundary).
func (c *Options) FillDefaultSetting(key, value string) { c.fillDefault(key, value) }

// resolveBool: a set flag wins, else the env var parses (0/false/off/no ->
// false), else the first set layer pointer, else def; recorded either way.
func (c *Options) resolveBool(key string, flagVal bool, envVal string, def bool, layers ...boolLayer) bool {
	v, src := def, "default"
	switch {
	case flagVal:
		v, src = true, "cliFlag"
	case envVal != "":
		v, src = parseBoolWord(envVal), "env"
	default:
		for _, l := range layers {
			if l.val != nil {
				v, src = *l.val, l.src
				break
			}
		}
	}
	c.record(key, strconv.FormatBool(v), src)
	return v
}

// Resolve finalizes the options in place, per field most-specific wins: flags
// > env vars > the user-config file (config-file) > the local spec's options: block
// (spec) > defaults; mode values validated. env supplies the env-var layer.
// Every resolution records a Setting (key, final value, deciding source) for
// the config log.
func (c *Options) Resolve(env LookupEnv, user, spec Layer) error {
	c.Settings = nil
	c.Settings = nil
	c.DryRun = DryRunMode(c.resolveStr("dryRun", "",
		flagStr(string(c.DryRun)), envStr(env("CHE_DRY_RUN")), layer(user.DryRun, "config-file"), layer(spec.DryRun, "specFile")))
	c.fillDefault("dryRun", "false")
	if c.DryRun == "true" {
		c.DryRun = DryRun.Delta
		c.setValue("dryRun", string(c.DryRun))
	}
	switch c.DryRun {
	case DryRun.Off, DryRun.Delta, DryRun.All:
	default:
		return fmt.Errorf("invalid --dry-run mode %q: want delta, all, or true (alias for delta)", c.DryRun)
	}
	// [why] ValidateSpecCLI is the flag/env/config-file override (empty if none),
	// overriding each spec's own options.validateSpec per-spec; ValidateSpec
	// adds the local spec's own layer, then the warn default. Recorded once,
	// as the effective value.
	cliVal, cliSrc := "", ""
	for _, cd := range []cand{flagStr(string(c.ValidateSpec)), envStr(env("CHE_VALIDATE_SPEC")), layer(user.ValidateSpec, "config-file")} {
		if cd.set {
			cliVal, cliSrc = cd.val, cd.src
			break
		}
	}
	c.ValidateSpecCLI = ValidateSpecMode(cliVal)
	c.ValidateSpec = ValidateSpecMode(cmp.Or(cliVal, spec.ValidateSpec, string(ValidateSpec.Warn)))
	switch {
	case cliVal != "":
		c.record("validateSpec", string(c.ValidateSpec), cliSrc)
	case spec.ValidateSpec != "":
		c.record("validateSpec", string(c.ValidateSpec), "specFile")
	default:
		c.record("validateSpec", string(c.ValidateSpec), "default")
	}
	switch c.ValidateSpec {
	case ValidateSpec.Warn, ValidateSpec.Error:
	default:
		return fmt.Errorf("invalid --validate-spec mode %q: want warn or error", c.ValidateSpec)
	}
	c.CheWorkingDirectory = c.resolveStr("cheWorkingDirectory", "",
		flagStr(c.CheWorkingDirectory), envStr(env("CHE_WORKING_DIRECTORY")))
	c.ProfileWorkingDirectory = c.resolveStr("profileWorkingDirectory", "",
		flagStr(c.ProfileWorkingDirectory), envStr(env("CHE_PROFILE_WORKING_DIRECTORY")), layer(spec.ProfileWorkingDirectory, "specFile"))
	c.Profiles = c.resolveList("profiles",
		layerList(c.Profiles, "cliFlag"), envStr(env("CHE_PROFILE")), layerList(user.Profiles, "config-file"), layerList(spec.Profiles, "specFile"))
	c.SkipOps = c.resolveList("skipOps",
		layerList(c.SkipOps, "cliFlag"), envStr(env("CHE_SKIP_OPS")), layerList(user.SkipOps, "config-file"), layerList(spec.SkipOps, "specFile"))
	c.RunSkipOps = c.resolveList("run.skipOps",
		layerList(c.RunSkipOps, "cliFlag"), envStr(env("CHE_RUN_SKIP_OPS")), layerList(user.Run.SkipOps, "config-file"), layerList(spec.Run.SkipOps, "specFile"))
	for _, name := range slices.Concat(c.SkipOps, c.RunSkipOps) {
		if !slices.Contains(OpNames, name) {
			return fmt.Errorf("invalid skip-ops op %q: want one of %s", name, strings.Join(OpNames, ", "))
		}
	}
	c.SkipRunIf = c.resolveBool("skipRunIf", c.SkipRunIf, env("CHE_SKIP_RUN_IF"), false)
	c.SkipRemoteRefs = c.resolveBool("skipRemoteRefs", c.SkipRemoteRefs, env("CHE_SKIP_REMOTE_REFS"), false,
		boolLayer{user.SkipRemoteRefs, "config-file"}, boolLayer{spec.SkipRemoteRefs, "specFile"})
	c.Debug = c.resolveBool("debug", c.Debug, env("CHE_DEBUG"), false,
		boolLayer{user.Debug, "config-file"}, boolLayer{spec.Debug, "specFile"})
	c.RenderSkipSecrets = c.resolveBool("renderTemplates.skipSecrets", c.RenderSkipSecrets, env("CHE_RENDER_TEMPLATES_SKIP_SECRETS"), false,
		boolLayer{user.RenderTemplates.SkipSecrets, "config-file"}, boolLayer{spec.RenderTemplates.SkipSecrets, "specFile"})
	c.AutoDiscover = c.resolveBool("autoDiscover", false, env("CHE_AUTO_DISCOVER"), true,
		boolLayer{user.AutoDiscover, "config-file"})
	return c.resolveOtel(env, user, spec)
}

// resolveOtel finalizes the OTLP telemetry group: env > config-file > specFile >
// defaults. enabled off (default) -> the provider is a no-op regardless of the
// rest; metrics/logs/traces default on when enabled. protocol validated (grpc
// default).
func (c *Options) resolveOtel(env LookupEnv, user, spec Layer) error {
	c.Otel.Enabled = c.resolveBool("otel.enabled", false, env("CHE_OTEL_ENABLED"), false,
		boolLayer{user.Otel.Enabled, "config-file"}, boolLayer{spec.Otel.Enabled, "specFile"})
	c.Otel.Protocol = c.resolveStr("otel.protocol", "grpc",
		envStr(env("CHE_OTEL_PROTOCOL")), layer(user.Otel.Protocol, "config-file"), layer(spec.Otel.Protocol, "specFile"))
	switch c.Otel.Protocol {
	case "grpc", "http":
	default:
		return fmt.Errorf("invalid otel.protocol %q: want grpc or http", c.Otel.Protocol)
	}
	c.Otel.Endpoint = c.resolveStr("otel.endpoint", defaultOtelEndpoint(c.Otel.Protocol),
		envStr(env("CHE_OTEL_ENDPOINT")), layer(user.Otel.Endpoint, "config-file"), layer(spec.Otel.Endpoint, "specFile"))
	c.Otel.Metrics = c.resolveBool("otel.metrics", false, env("CHE_OTEL_METRICS"), true,
		boolLayer{user.Otel.Metrics, "config-file"}, boolLayer{spec.Otel.Metrics, "specFile"})
	c.Otel.Logs = c.resolveBool("otel.logs", false, env("CHE_OTEL_LOGS"), true,
		boolLayer{user.Otel.Logs, "config-file"}, boolLayer{spec.Otel.Logs, "specFile"})
	c.Otel.Traces = c.resolveBool("otel.traces", false, env("CHE_OTEL_TRACES"), true,
		boolLayer{user.Otel.Traces, "config-file"}, boolLayer{spec.Otel.Traces, "specFile"})
	return nil
}

// defaultOtelEndpoint is the local collector's default OTLP endpoint per transport.
func defaultOtelEndpoint(protocol string) string {
	if protocol == "http" {
		return "localhost:4318"
	}
	return "localhost:4317"
}

// [<] 🤖🤖
