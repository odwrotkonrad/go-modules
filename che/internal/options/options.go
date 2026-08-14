package options

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/packages"
)

type LookupEnv func(string) string

type Setting struct {
	Key    string
	Value  string
	Source string
	Kind   string
}

func (c Options) SettingsDelta() []Setting {
	changed, _ := c.partitionSettings()
	return changed
}

func (c Options) SettingsSorted() []Setting {
	changed, unchanged := c.partitionSettings()
	return append(changed, unchanged...)
}

func (c Options) partitionSettings() (changed, unchanged []Setting) {
	for _, s := range c.Settings {
		if s.IsChanged() {
			changed = append(changed, s)
		} else {
			unchanged = append(unchanged, s)
		}
	}
	return changed, unchanged
}

func (s Setting) IsChanged() bool { return s.Source != "default" }

func (s Setting) DisplaySource() string {
	if s.Source == "default" {
		return "unset"
	}
	return s.Source
}

func FormatSettings(settings []Setting) string {
	parts := make([]string, len(settings))
	for i, s := range settings {
		parts[i] = fmt.Sprintf("%s=%s (%s)", s.Key, s.Value, s.Source)
	}
	return strings.Join(parts, ", ")
}

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

func (c *Options) resolveList(key string, candidates ...cand) []string {
	v := c.resolveStr(key, "", candidates...)
	c.setKind(key, "list")
	if v == "" {
		c.fillDefault(key, "[]")
		return nil
	}
	return strings.Split(v, ",")
}

func (c *Options) updateSetting(key string, fn func(*Setting)) {
	for i := range c.Settings {
		if c.Settings[i].Key == key {
			fn(&c.Settings[i])
		}
	}
}

func (c *Options) setKind(key, kind string) {
	c.updateSetting(key, func(s *Setting) { s.Kind = kind })
}

func (c *Options) setValue(key, value string) {
	c.updateSetting(key, func(s *Setting) { s.Value = value })
}

func (c *Options) fillDefault(key, value string) {
	c.updateSetting(key, func(s *Setting) {
		if s.Source == "default" && s.Value == "" {
			s.Value = value
		}
	})
}

func (c *Options) FillDefaultSetting(key, value string) { c.fillDefault(key, value) }

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
	c.setKind(key, "bool")
	return v
}

func (c *Options) Resolve(env LookupEnv, user, spec Layer) error {
	c.Settings = nil
	c.DryRun = DryRunMode(c.resolveStr("dryRun", "",
		flagStr(string(c.DryRun)), envStr(env("CHE_DRY_RUN")), layer(string(user.DryRun), "config-file"), layer(string(spec.DryRun), "specFile")))
	c.fillDefault("dryRun", "false")
	c.setKind("dryRun", "dryRun")
	if c.DryRun == "true" {
		c.DryRun = DryRun.Delta
		c.setValue("dryRun", string(c.DryRun))
	}
	if c.DryRun == "false" || c.DryRun == "off" {
		c.DryRun = DryRun.Off
		c.setValue("dryRun", "false")
	}
	switch c.DryRun {
	case DryRun.Off, DryRun.Delta, DryRun.All:
	default:
		return fmt.Errorf("invalid --dry-run mode %q: want delta, all, or true (alias for delta)", c.DryRun)
	}
	// [why] ValidateSpecCLI is the flag/env/config-file override (empty if none),
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
	c.Errexit = c.resolveBool("errexit", c.Errexit, env("CHE_ERREXIT"), false)
	c.SkipRemoteRefs = c.resolveBool("skipRemoteRefs", c.SkipRemoteRefs, env("CHE_SKIP_REMOTE_REFS"), false,
		boolLayer{user.SkipRemoteRefs, "config-file"}, boolLayer{spec.SkipRemoteRefs, "specFile"})
	c.LogLevel = c.resolveStr("logLevel", "info",
		flagStr(c.LogLevel), envStr(env("CHE_LOG_LEVEL")), layer(user.LogLevel, "config-file"), layer(spec.LogLevel, "specFile"))
	if _, err := log.ParseLevel(c.LogLevel); err != nil {
		return fmt.Errorf("--log-level: %w", err)
	}
	c.RenderSkipSecrets = c.resolveBool("renderTemplates.skipSecrets", c.RenderSkipSecrets, env("CHE_RENDER_TEMPLATES_SKIP_SECRETS"), false,
		boolLayer{user.RenderTemplates.SkipSecrets, "config-file"}, boolLayer{spec.RenderTemplates.SkipSecrets, "specFile"})
	c.PackagesFile = c.resolveStr("packages.file", "",
		flagStr(c.PackagesFile), envStr(env("CHE_PACKAGES_FILE")), layer(user.Packages.File, "config-file"), layer(spec.Packages.File, "specFile"))
	c.fillDefault("packages.file", packages.BuiltinSentinel)
	c.PackagesOverride = c.resolveStr("packages.override", "",
		flagStr(c.PackagesOverride), envStr(env("CHE_PACKAGES_OVERRIDE")))
	c.PackagesPreferredMethods = c.resolveList("packages.preferredInstallationMethods",
		layerList(c.PackagesPreferredMethods, "cliFlag"), envStr(env("CHE_PACKAGES_PREFERRED_METHODS")),
		layerList(user.Packages.PreferredInstallationMethods, "config-file"), layerList(spec.Packages.PreferredInstallationMethods, "specFile"))
	if err := packages.ValidateManagers(c.PackagesPreferredMethods); err != nil {
		return err
	}
	if len(c.PackagesPreferredMethods) == 0 {
		c.PackagesPreferredMethods = packages.DefaultPreferredMethods
		c.setValue("packages.preferredInstallationMethods", "["+strings.Join(packages.DefaultPreferredMethods, ", ")+"]")
	}
	c.PackagesOnlyMethods = c.resolveList("packages.onlyInstallationMethods",
		layerList(c.PackagesOnlyMethods, "cliFlag"), envStr(env("CHE_PACKAGES_ONLY_METHODS")))
	if err := packages.ValidateManagers(c.PackagesOnlyMethods); err != nil {
		return err
	}
	c.PackagesDownloadCacheDir = c.resolveStr("packages.downloadCacheDir", "",
		flagStr(c.PackagesDownloadCacheDir), envStr(env("CHE_PACKAGES_DOWNLOAD_CACHE_DIR")))
	c.PackagesBinariesRemoteArchiveDestinationCandidates = c.resolveList("packages.binariesRemoteArchive.installDestinationCandidates",
		layerList(c.PackagesBinariesRemoteArchiveDestinationCandidates, "cliFlag"), envStr(env("CHE_PACKAGES_BINARIES_REMOTE_ARCHIVE_INSTALL_DESTINATION_CANDIDATES")),
		layerList(user.Packages.BinariesRemoteArchive.InstallDestinationCandidates, "config-file"), layerList(spec.Packages.BinariesRemoteArchive.InstallDestinationCandidates, "specFile"))
	if len(c.PackagesBinariesRemoteArchiveDestinationCandidates) == 0 {
		c.PackagesBinariesRemoteArchiveDestinationCandidates = packages.DefaultBinariesRemoteArchiveDestinationCandidates
		c.setValue("packages.binariesRemoteArchive.installDestinationCandidates", "["+strings.Join(packages.DefaultBinariesRemoteArchiveDestinationCandidates, ", ")+"]")
	}
	c.PackagesBinariesRemoteArchiveCheckPresentOnPath = c.resolveBool("packages.binariesRemoteArchive.checkPresentOnPath", false, env("CHE_PACKAGES_BINARIES_REMOTE_ARCHIVE_CHECK_PRESENT_ON_PATH"), true,
		boolLayer{user.Packages.BinariesRemoteArchive.CheckPresentOnPath, "config-file"}, boolLayer{spec.Packages.BinariesRemoteArchive.CheckPresentOnPath, "specFile"})
	c.PackagesCompletionsEnabled = c.resolveBool("packages.completions.zsh.enabled", false, env("CHE_PACKAGES_COMPLETIONS_ZSH_ENABLED"), false,
		boolLayer{user.Packages.Completions.Zsh.Enabled, "config-file"}, boolLayer{spec.Packages.Completions.Zsh.Enabled, "specFile"})
	c.PackagesCompletionsDestinationCandidates = c.resolveList("packages.completions.zsh.installDestinationCandidates",
		envStr(env("CHE_PACKAGES_COMPLETIONS_ZSH_INSTALL_DESTINATION_CANDIDATES")),
		layerList(user.Packages.Completions.Zsh.InstallDestinationCandidates, "config-file"),
		layerList(spec.Packages.Completions.Zsh.InstallDestinationCandidates, "specFile"))
	if len(c.PackagesCompletionsDestinationCandidates) == 0 {
		c.PackagesCompletionsDestinationCandidates = packages.DefaultCompletionsDestinationCandidates
		c.setValue("packages.completions.zsh.installDestinationCandidates", "["+strings.Join(packages.DefaultCompletionsDestinationCandidates, ", ")+"]")
	}
	c.PackagesCompletionsCheckPresentOnFpath = c.resolveBool("packages.completions.zsh.checkPresentOnFpath", false, env("CHE_PACKAGES_COMPLETIONS_ZSH_CHECK_PRESENT_ON_FPATH"), true,
		boolLayer{user.Packages.Completions.Zsh.CheckPresentOnFpath, "config-file"}, boolLayer{spec.Packages.Completions.Zsh.CheckPresentOnFpath, "specFile"})
	c.AutoDiscover = c.resolveBool("autoDiscover", false, env("CHE_AUTO_DISCOVER"), true,
		boolLayer{user.AutoDiscover, "config-file"})
	return c.resolveOtel(env, user, spec)
}

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

func defaultOtelEndpoint(protocol string) string {
	if protocol == "http" {
		return "localhost:4318"
	}
	return "localhost:4317"
}

// [<] 🤖🤖
