package options

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

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

func (o Options) SettingsDelta() []Setting {
	changed, _ := o.partitionSettings()
	return changed
}

func (o Options) SettingsSorted() []Setting {
	changed, unchanged := o.partitionSettings()
	return append(changed, unchanged...)
}

func (o Options) partitionSettings() (changed, unchanged []Setting) {
	for _, s := range o.Settings {
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

func firstSet(candidates ...cand) (val, src string) {
	for _, cd := range candidates {
		if cd.set {
			return cd.val, cd.src
		}
	}
	return "", ""
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

func (o *Options) record(key, value, source string) {
	o.Settings = append(o.Settings, Setting{Key: key, Value: value, Source: source})
}

func (o *Options) resolveStr(key, def string, candidates ...cand) string {
	if val, src := firstSet(candidates...); src != "" {
		o.record(key, val, src)
		return val
	}
	o.record(key, def, "default")
	return def
}

func (o *Options) resolveList(key string, candidates ...cand) []string {
	v := o.resolveStr(key, "", candidates...)
	o.setKind(key, "list")
	if v == "" {
		o.fillDefault(key, "[]")
		return nil
	}
	return strings.Split(v, ",")
}

func (o *Options) updateSetting(key string, fn func(*Setting)) {
	for i := range o.Settings {
		if o.Settings[i].Key == key {
			fn(&o.Settings[i])
		}
	}
}

func (o *Options) setKind(key, kind string) {
	o.updateSetting(key, func(s *Setting) { s.Kind = kind })
}

func (o *Options) setValue(key, value string) {
	o.updateSetting(key, func(s *Setting) { s.Value = value })
}

func (o *Options) fillDefault(key, value string) {
	o.updateSetting(key, func(s *Setting) {
		if s.Source == "default" && s.Value == "" {
			s.Value = value
		}
	})
}

func (o *Options) FillDefaultSetting(key, value string) { o.fillDefault(key, value) }

func (o *Options) resolveBool(key string, flagVal bool, envVal string, def bool, layers ...boolLayer) bool {
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
	o.record(key, strconv.FormatBool(v), src)
	o.setKind(key, "bool")
	return v
}

func (o *Options) Resolve(env LookupEnv, user, spec Layer) error {
	o.Settings = nil
	o.DryRun = DryRunMode(o.resolveStr("dryRun", "",
		flagStr(string(o.DryRun)), envStr(env("CHE_DRY_RUN")), layer(string(user.DryRun), "config-file"), layer(string(spec.DryRun), "specFile")))
	o.fillDefault("dryRun", "false")
	o.setKind("dryRun", "dryRun")
	if o.DryRun == "true" {
		o.DryRun = DryRun.Delta
		o.setValue("dryRun", string(o.DryRun))
	}
	if o.DryRun == "false" || o.DryRun == "off" {
		o.DryRun = DryRun.Off
		o.setValue("dryRun", "false")
	}
	switch o.DryRun {
	case DryRun.Off, DryRun.Delta, DryRun.All:
	default:
		return fmt.Errorf("invalid --dry-run mode %q: want delta, all, or true (alias for delta)", o.DryRun)
	}
	override, overrideSrc := firstSet(flagStr(string(o.ValidateSpec)), envStr(env("CHE_VALIDATE_SPEC")), layer(user.ValidateSpec, "config-file"))
	o.ValidateSpecCLI = ValidateSpecMode(override)
	o.ValidateSpec = ValidateSpecMode(cmp.Or(override, spec.ValidateSpec, string(ValidateSpec.Warn)))
	switch {
	case override != "":
		o.record("validateSpec", string(o.ValidateSpec), overrideSrc)
	case spec.ValidateSpec != "":
		o.record("validateSpec", string(o.ValidateSpec), "specFile")
	default:
		o.record("validateSpec", string(o.ValidateSpec), "default")
	}
	switch o.ValidateSpec {
	case ValidateSpec.Warn, ValidateSpec.Error:
	default:
		return fmt.Errorf("invalid --validate-spec mode %q: want warn or error", o.ValidateSpec)
	}
	o.CheWorkingDirectory = o.resolveStr("cheWorkingDirectory", "",
		flagStr(o.CheWorkingDirectory), envStr(env("CHE_WORKING_DIRECTORY")))
	o.ProfileWorkingDirectory = o.resolveStr("profileWorkingDirectory", "",
		flagStr(o.ProfileWorkingDirectory), envStr(env("CHE_PROFILE_WORKING_DIRECTORY")), layer(spec.ProfileWorkingDirectory, "specFile"))
	o.Profiles = o.resolveList("profiles",
		layerList(o.Profiles, "cliFlag"), envStr(env("CHE_PROFILE")), layerList(user.Profiles, "config-file"), layerList(spec.Profiles, "specFile"))
	o.SkipOps = o.resolveList("skipOps",
		layerList(o.SkipOps, "cliFlag"), envStr(env("CHE_SKIP_OPS")), layerList(user.SkipOps, "config-file"), layerList(spec.SkipOps, "specFile"))
	o.RunSkipOps = o.resolveList("run.skipOps",
		layerList(o.RunSkipOps, "cliFlag"), envStr(env("CHE_RUN_SKIP_OPS")), layerList(user.Run.SkipOps, "config-file"), layerList(spec.Run.SkipOps, "specFile"))
	for _, name := range slices.Concat(o.SkipOps, o.RunSkipOps) {
		if !slices.Contains(OpNames, name) {
			return fmt.Errorf("invalid skip-ops op %q: want one of %s", name, strings.Join(OpNames, ", "))
		}
	}
	o.SkipRunIf = o.resolveBool("skipRunIf", o.SkipRunIf, env("CHE_SKIP_RUN_IF"), false)
	o.Errexit = o.resolveBool("errexit", o.Errexit, env("CHE_ERREXIT"), false)
	o.SkipRemoteRefs = o.resolveBool("skipRemoteRefs", o.SkipRemoteRefs, env("CHE_SKIP_REMOTE_REFS"), false,
		boolLayer{user.SkipRemoteRefs, "config-file"}, boolLayer{spec.SkipRemoteRefs, "specFile"})
	o.LogLevel = o.resolveStr("logLevel", "info",
		flagStr(o.LogLevel), envStr(env("CHE_LOG_LEVEL")), layer(user.LogLevel, "config-file"), layer(spec.LogLevel, "specFile"))
	if _, err := log.ParseLevel(o.LogLevel); err != nil {
		return fmt.Errorf("--log-level: %w", err)
	}
	silenceDefault := "true"
	if lvl, _ := log.ParseLevel(o.LogLevel); lvl >= log.Levels.Debug {
		silenceDefault = "false"
	}
	o.PackagesSilenceInstallStdout = o.resolveStr("packages.silenceInstallStdout", silenceDefault,
		flagStr(o.PackagesSilenceInstallStdout), envStr(env("CHE_PACKAGES_SILENCE_INSTALL_STDOUT")))
	o.setKind("packages.silenceInstallStdout", "bool")
	switch o.PackagesSilenceInstallStdout {
	case "true", "false":
	default:
		return fmt.Errorf("invalid --silence-install-stdout %q: want true or false", o.PackagesSilenceInstallStdout)
	}
	o.RenderSkipSecrets = o.resolveBool("renderTemplates.skipSecrets", o.RenderSkipSecrets, env("CHE_RENDER_TEMPLATES_SKIP_SECRETS"), false,
		boolLayer{user.RenderTemplates.SkipSecrets, "config-file"}, boolLayer{spec.RenderTemplates.SkipSecrets, "specFile"})
	o.PackagesFile = o.resolveStr("packages.file", "",
		flagStr(o.PackagesFile), envStr(env("CHE_PACKAGES_FILE")), layer(user.Packages.File, "config-file"), layer(spec.Packages.File, "specFile"))
	o.fillDefault("packages.file", packages.BuiltinSentinel)
	o.PackagesOverride = o.resolveStr("packages.override", "",
		flagStr(o.PackagesOverride), envStr(env("CHE_PACKAGES_OVERRIDE")))
	o.PackagesPreferredMethods = o.resolveList("packages.preferredInstallationMethods",
		layerList(o.PackagesPreferredMethods, "cliFlag"), envStr(env("CHE_PACKAGES_PREFERRED_METHODS")),
		layerList(user.Packages.PreferredInstallationMethods, "config-file"), layerList(spec.Packages.PreferredInstallationMethods, "specFile"))
	if err := packages.ValidateManagers(o.PackagesPreferredMethods); err != nil {
		return err
	}
	if len(o.PackagesPreferredMethods) == 0 {
		o.PackagesPreferredMethods = packages.DefaultPreferredMethods
		o.setValue("packages.preferredInstallationMethods", "["+strings.Join(packages.DefaultPreferredMethods, ", ")+"]")
	}
	o.PackagesOnlyMethods = o.resolveList("packages.onlyInstallationMethods",
		layerList(o.PackagesOnlyMethods, "cliFlag"), envStr(env("CHE_PACKAGES_ONLY_METHODS")))
	if err := packages.ValidateManagers(o.PackagesOnlyMethods); err != nil {
		return err
	}
	o.PackagesUpdateCheckEnabled = o.resolveBool("packages.updateCheck.enabled", false, env("CHE_PACKAGES_UPDATE_CHECK"), false,
		boolLayer{user.Packages.UpdateCheck.Enabled, "config-file"}, boolLayer{spec.Packages.UpdateCheck.Enabled, "specFile"})
	o.PackagesUpdateCheckCooldown = o.resolveStr("packages.updateCheck.cooldown", "15m",
		envStr(env("CHE_PACKAGES_UPDATE_CHECK_COOLDOWN")), layer(user.Packages.UpdateCheck.Cooldown, "config-file"), layer(spec.Packages.UpdateCheck.Cooldown, "specFile"))
	if _, err := time.ParseDuration(o.PackagesUpdateCheckCooldown); err != nil {
		return fmt.Errorf("invalid packages.updateCheck.cooldown %q: want a Go duration (15m, 1h)", o.PackagesUpdateCheckCooldown)
	}
	o.PackagesDownloadCacheDir = o.resolveStr("packages.downloadCacheDir", "",
		flagStr(o.PackagesDownloadCacheDir), envStr(env("CHE_PACKAGES_DOWNLOAD_CACHE_DIR")))
	o.PackagesBinariesRemoteArchiveDestinationCandidates = o.resolveList("packages.binariesRemoteArchive.installDestinationCandidates",
		layerList(o.PackagesBinariesRemoteArchiveDestinationCandidates, "cliFlag"), envStr(env("CHE_PACKAGES_BINARIES_REMOTE_ARCHIVE_INSTALL_DESTINATION_CANDIDATES")),
		layerList(user.Packages.BinariesRemoteArchive.InstallDestinationCandidates, "config-file"), layerList(spec.Packages.BinariesRemoteArchive.InstallDestinationCandidates, "specFile"))
	if len(o.PackagesBinariesRemoteArchiveDestinationCandidates) == 0 {
		o.PackagesBinariesRemoteArchiveDestinationCandidates = packages.DefaultBinariesRemoteArchiveDestinationCandidates
		o.setValue("packages.binariesRemoteArchive.installDestinationCandidates", "["+strings.Join(packages.DefaultBinariesRemoteArchiveDestinationCandidates, ", ")+"]")
	}
	o.PackagesBinariesRemoteArchiveCheckPresentOnPath = o.resolveBool("packages.binariesRemoteArchive.checkPresentOnPath", false, env("CHE_PACKAGES_BINARIES_REMOTE_ARCHIVE_CHECK_PRESENT_ON_PATH"), true,
		boolLayer{user.Packages.BinariesRemoteArchive.CheckPresentOnPath, "config-file"}, boolLayer{spec.Packages.BinariesRemoteArchive.CheckPresentOnPath, "specFile"})
	o.PackagesCompletionsEnabled = o.resolveBool("packages.completions.zsh.enabled", false, env("CHE_PACKAGES_COMPLETIONS_ZSH_ENABLED"), false,
		boolLayer{user.Packages.Completions.Zsh.Enabled, "config-file"}, boolLayer{spec.Packages.Completions.Zsh.Enabled, "specFile"})
	o.PackagesCompletionsDestinationCandidates = o.resolveList("packages.completions.zsh.installDestinationCandidates",
		envStr(env("CHE_PACKAGES_COMPLETIONS_ZSH_INSTALL_DESTINATION_CANDIDATES")),
		layerList(user.Packages.Completions.Zsh.InstallDestinationCandidates, "config-file"),
		layerList(spec.Packages.Completions.Zsh.InstallDestinationCandidates, "specFile"))
	if len(o.PackagesCompletionsDestinationCandidates) == 0 {
		o.PackagesCompletionsDestinationCandidates = packages.DefaultCompletionsDestinationCandidates
		o.setValue("packages.completions.zsh.installDestinationCandidates", "["+strings.Join(packages.DefaultCompletionsDestinationCandidates, ", ")+"]")
	}
	o.PackagesCompletionsCheckPresentOnFpath = o.resolveBool("packages.completions.zsh.checkPresentOnFpath", false, env("CHE_PACKAGES_COMPLETIONS_ZSH_CHECK_PRESENT_ON_FPATH"), true,
		boolLayer{user.Packages.Completions.Zsh.CheckPresentOnFpath, "config-file"}, boolLayer{spec.Packages.Completions.Zsh.CheckPresentOnFpath, "specFile"})
	o.AutoDiscover = o.resolveBool("autoDiscover", false, env("CHE_AUTO_DISCOVER"), true,
		boolLayer{user.AutoDiscover, "config-file"})
	return o.resolveOtel(env, user, spec)
}

func (o *Options) resolveOtel(env LookupEnv, user, spec Layer) error {
	o.Otel.Enabled = o.resolveBool("otel.enabled", false, env("CHE_OTEL_ENABLED"), false,
		boolLayer{user.Otel.Enabled, "config-file"}, boolLayer{spec.Otel.Enabled, "specFile"})
	o.Otel.Protocol = o.resolveStr("otel.protocol", "grpc",
		envStr(env("CHE_OTEL_PROTOCOL")), layer(user.Otel.Protocol, "config-file"), layer(spec.Otel.Protocol, "specFile"))
	switch o.Otel.Protocol {
	case "grpc", "http":
	default:
		return fmt.Errorf("invalid otel.protocol %q: want grpc or http", o.Otel.Protocol)
	}
	o.Otel.Endpoint = o.resolveStr("otel.endpoint", defaultOtelEndpoint(o.Otel.Protocol),
		envStr(env("CHE_OTEL_ENDPOINT")), layer(user.Otel.Endpoint, "config-file"), layer(spec.Otel.Endpoint, "specFile"))
	o.Otel.Metrics = o.resolveBool("otel.metrics", false, env("CHE_OTEL_METRICS"), true,
		boolLayer{user.Otel.Metrics, "config-file"}, boolLayer{spec.Otel.Metrics, "specFile"})
	o.Otel.Logs = o.resolveBool("otel.logs", false, env("CHE_OTEL_LOGS"), true,
		boolLayer{user.Otel.Logs, "config-file"}, boolLayer{spec.Otel.Logs, "specFile"})
	o.Otel.Traces = o.resolveBool("otel.traces", false, env("CHE_OTEL_TRACES"), true,
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
