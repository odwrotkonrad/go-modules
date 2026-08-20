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
	"gitlab.com/konradodwrot/go-modules/che/internal/spec/envinterp"
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

type candidate struct {
	value    string
	provided bool
	source   string
}

func fromFlag(value string) candidate { return candidate{value, value != "", "cliFlag"} }
func fromEnv(value string) candidate  { return candidate{value, value != "", "env"} }

func fromLayer(value, source string) candidate {
	return candidate{value, value != "", source}
}

func fromLayerList(values []string, source string) candidate {
	return candidate{strings.Join(values, ","), len(values) > 0, source}
}

func firstProvided(candidates ...candidate) (value, source string) {
	for _, c := range candidates {
		if c.provided {
			return c.value, c.source
		}
	}
	return "", ""
}

type boolCandidate struct {
	value  *bool
	source string
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

func (o *Options) resolveStr(key, def string, candidates ...candidate) string {
	if value, source := firstProvided(candidates...); source != "" {
		o.record(key, value, source)
		return value
	}
	o.record(key, def, "default")
	return def
}

func (o *Options) resolveList(key string, candidates ...candidate) []string {
	joined := o.resolveStr(key, "", candidates...)
	o.setKind(key, "list")
	if joined == "" {
		o.fillDefault(key, "[]")
		return nil
	}
	return strings.Split(joined, ",")
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

// [why] a default-true option needs a flag that can say "false", which a bare bool cannot: an unset
//
//	bool and an explicit --flag=false are both false, so the flag would mask every layer below it
func (o *Options) resolveBoolPtr(key string, flagVal *bool, envVal string, def bool, candidates ...boolCandidate) bool {
	if flagVal != nil {
		return o.recordBool(key, *flagVal, "cliFlag")
	}
	return o.resolveBool(key, false, envVal, def, candidates...)
}

func (o *Options) recordBool(key string, value bool, source string) bool {
	o.record(key, strconv.FormatBool(value), source)
	o.setKind(key, "bool")
	return value
}

func (o *Options) resolveBool(key string, flagVal bool, envVal string, def bool, candidates ...boolCandidate) bool {
	value, source := def, "default"
	switch {
	case flagVal:
		value, source = true, "cliFlag"
	case envVal != "":
		value, source = parseBoolWord(envVal), "env"
	default:
		for _, c := range candidates {
			if c.value != nil {
				value, source = *c.value, c.source
				break
			}
		}
	}
	return o.recordBool(key, value, source)
}

func (o *Options) Resolve(env LookupEnv, user, spec Layer) error {
	o.Settings = nil
	o.DryRun = DryRunMode(o.resolveStr("dryRun", "",
		fromFlag(string(o.DryRun)), fromEnv(env("CHE_DRY_RUN")), fromLayer(string(user.DryRun), "config-file"), fromLayer(string(spec.DryRun), "specFile")))
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
	override, overrideSrc := firstProvided(fromFlag(string(o.ValidateSpec)), fromEnv(env("CHE_VALIDATE_SPEC")), fromLayer(user.ValidateSpec, "config-file"))
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
	o.EnvUnset = envinterp.Policy(o.resolveStr("envUnset", string(envinterp.Policies.Error),
		fromFlag(string(o.EnvUnset)), fromEnv(env("CHE_ENV_UNSET")), fromLayer(user.EnvUnset, "config-file"), fromLayer(spec.EnvUnset, "specFile")))
	if !envinterp.ValidPolicy(o.EnvUnset) {
		return fmt.Errorf("invalid --env-unset policy %q: want error or empty", o.EnvUnset)
	}
	o.CheWorkingDirectory = o.resolveStr("cheWorkingDirectory", "",
		fromFlag(o.CheWorkingDirectory), fromEnv(env("CHE_WORKING_DIRECTORY")))
	o.ProfileWorkingDirectory = o.resolveStr("profileWorkingDirectory", "",
		fromFlag(o.ProfileWorkingDirectory), fromEnv(env("CHE_PROFILE_WORKING_DIRECTORY")), fromLayer(spec.ProfileWorkingDirectory, "specFile"))
	o.Profiles = o.resolveList("profiles",
		fromLayerList(o.Profiles, "cliFlag"), fromEnv(env("CHE_PROFILE")), fromLayerList(user.Profiles, "config-file"), fromLayerList(spec.Profiles, "specFile"))
	o.RunSkipOps = o.resolveList("run.skipOps",
		fromLayerList(o.RunSkipOps, "cliFlag"), fromEnv(env("CHE_RUN_SKIP_OPS")), fromLayerList(user.Run.SkipOps, "config-file"), fromLayerList(spec.Run.SkipOps, "specFile"))
	for _, name := range o.RunSkipOps {
		if !slices.Contains(OpNames, name) {
			return fmt.Errorf("invalid skip-ops op %q: want one of %s", name, strings.Join(OpNames, ", "))
		}
	}
	o.BackupAutoCreateDisabled = !o.resolveBoolPtr("backup.autoCreate.enabled", o.BackupAutoCreateFlag, env("CHE_BACKUP_AUTO_CREATE"), true,
		boolCandidate{user.Backup.AutoCreate.Enabled, "config-file"}, boolCandidate{spec.Backup.AutoCreate.Enabled, "specFile"})
	o.SkipRunIf = o.resolveBool("skipRunIf", o.SkipRunIf, env("CHE_SKIP_RUN_IF"), false)
	o.Errexit = o.resolveBool("errexit", o.Errexit, env("CHE_ERREXIT"), false)
	o.SkipRemoteRefs = o.resolveBool("skipRemoteRefs", o.SkipRemoteRefs, env("CHE_SKIP_REMOTE_REFS"), false,
		boolCandidate{user.SkipRemoteRefs, "config-file"}, boolCandidate{spec.SkipRemoteRefs, "specFile"})
	o.LogLevel = o.resolveStr("logLevel", "info",
		fromFlag(o.LogLevel), fromEnv(env("CHE_LOG_LEVEL")), fromLayer(user.LogLevel, "config-file"), fromLayer(spec.LogLevel, "specFile"))
	if _, err := log.ParseLevel(o.LogLevel); err != nil {
		return fmt.Errorf("--log-level: %w", err)
	}
	silenceDefault := "true"
	if lvl, _ := log.ParseLevel(o.LogLevel); lvl >= log.Levels.Debug {
		silenceDefault = "false"
	}
	o.PackagesSilenceInstallStdout = o.resolveStr("packages.silenceInstallStdout", silenceDefault,
		fromFlag(o.PackagesSilenceInstallStdout), fromEnv(env("CHE_PACKAGES_SILENCE_INSTALL_STDOUT")))
	o.setKind("packages.silenceInstallStdout", "bool")
	switch o.PackagesSilenceInstallStdout {
	case "true", "false":
	default:
		return fmt.Errorf("invalid --silence-install-stdout %q: want true or false", o.PackagesSilenceInstallStdout)
	}
	o.RenderSkipSecrets = o.resolveBool("renderTemplates.skipSecrets", o.RenderSkipSecrets, env("CHE_RENDER_TEMPLATES_SKIP_SECRETS"), false,
		boolCandidate{user.RenderTemplates.SkipSecrets, "config-file"}, boolCandidate{spec.RenderTemplates.SkipSecrets, "specFile"})
	o.RenderSkipVariables = o.resolveBool("renderTemplates.skipVariables", o.RenderSkipVariables, env("CHE_RENDER_TEMPLATES_SKIP_VARIABLES"), false,
		boolCandidate{user.RenderTemplates.SkipVariables, "config-file"}, boolCandidate{spec.RenderTemplates.SkipVariables, "specFile"})
	o.PackagesFile = o.resolveStr("packages.file", "",
		fromFlag(o.PackagesFile), fromEnv(env("CHE_PACKAGES_FILE")), fromLayer(user.Packages.File, "config-file"), fromLayer(spec.Packages.File, "specFile"))
	o.fillDefault("packages.file", packages.BuiltinSentinel)
	o.PackagesOverride = o.resolveStr("packages.override", "",
		fromFlag(o.PackagesOverride), fromEnv(env("CHE_PACKAGES_OVERRIDE")))
	o.PackagesPreferredMethods = o.resolveList("packages.preferredInstallationMethods",
		fromLayerList(o.PackagesPreferredMethods, "cliFlag"), fromEnv(env("CHE_PACKAGES_PREFERRED_METHODS")),
		fromLayerList(user.Packages.PreferredInstallationMethods, "config-file"), fromLayerList(spec.Packages.PreferredInstallationMethods, "specFile"))
	if err := packages.ValidateManagers(o.PackagesPreferredMethods); err != nil {
		return err
	}
	if len(o.PackagesPreferredMethods) == 0 {
		o.PackagesPreferredMethods = packages.DefaultPreferredMethods
		o.setValue("packages.preferredInstallationMethods", "["+strings.Join(packages.DefaultPreferredMethods, ", ")+"]")
	}
	o.PackagesOnlyMethods = o.resolveList("packages.onlyInstallationMethods",
		fromLayerList(o.PackagesOnlyMethods, "cliFlag"), fromEnv(env("CHE_PACKAGES_ONLY_METHODS")),
		fromLayerList(user.Packages.OnlyInstallationMethods, "config-file"), fromLayerList(spec.Packages.OnlyInstallationMethods, "specFile"))
	if err := packages.ValidateManagers(o.PackagesOnlyMethods); err != nil {
		return err
	}
	//[why] the default is assigned, not merely displayed: `url:` written with no value resolves to
	//   an empty string that every layer treats as "set", and an empty base URL fetched
	//   "/latest/version.txt" as a schemeless path, retried 11 times, then failed
	o.PackagesSourceURL = o.resolveStr("packages.source.url", packages.DefaultSourceURL,
		fromFlag(o.PackagesSourceURL), fromEnv(env("CHE_PACKAGES_SOURCE_URL")), fromEnv(env("CHE_PACKAGES_UPDATE_URL")),
		fromLayer(user.Packages.Source.URL, "config-file"), fromLayer(spec.Packages.Source.URL, "specFile"))
	if o.PackagesSourceURL == "" {
		o.PackagesSourceURL = packages.DefaultSourceURL
		o.setValue("packages.source.url", packages.DefaultSourceURL)
	}
	o.PackagesSourceRef = o.resolveStr("packages.source.ref", "",
		fromFlag(o.PackagesSourceRef), fromEnv(env("CHE_PACKAGES_REF")),
		fromLayer(user.Packages.Source.Ref, "config-file"), fromLayer(spec.Packages.Source.Ref, "specFile"))
	o.PackagesAutoUpdateEnabled = o.resolveBool("packages.autoUpdate.enabled", false, env("CHE_PACKAGES_AUTO_UPDATE"), true,
		boolCandidate{user.Packages.AutoUpdate.Enabled, "config-file"}, boolCandidate{spec.Packages.AutoUpdate.Enabled, "specFile"})
	o.PackagesAutoUpdateCooldown = o.resolveStr("packages.autoUpdate.if.refIsLatest.cooldown", "15m",
		fromEnv(env("CHE_PACKAGES_AUTO_UPDATE_COOLDOWN")), fromLayer(user.Packages.AutoUpdate.If.RefIsLatest.Cooldown, "config-file"), fromLayer(spec.Packages.AutoUpdate.If.RefIsLatest.Cooldown, "specFile"))
	if _, err := time.ParseDuration(o.PackagesAutoUpdateCooldown); err != nil {
		return fmt.Errorf("invalid packages.autoUpdate.if.refIsLatest.cooldown %q: want a Go duration (15m, 1h)", o.PackagesAutoUpdateCooldown)
	}
	o.PackagesAutoUpdateDryRunEnabled = o.resolveBool("packages.autoUpdate.if.dryRunIsTrue.enabled", false, env("CHE_PACKAGES_AUTO_UPDATE_IF_DRY_RUN"), true,
		boolCandidate{user.Packages.AutoUpdate.If.DryRunIsTrue.Enabled, "config-file"}, boolCandidate{spec.Packages.AutoUpdate.If.DryRunIsTrue.Enabled, "specFile"})
	o.PackagesDownloadCacheDir = o.resolveStr("packages.downloadCacheDir", "",
		fromFlag(o.PackagesDownloadCacheDir), fromEnv(env("CHE_PACKAGES_DOWNLOAD_CACHE_DIR")))
	o.PackagesBinariesRemoteArchiveDestinationCandidates = o.resolveList("packages.binariesRemoteArchive.installDestinationCandidates",
		fromLayerList(o.PackagesBinariesRemoteArchiveDestinationCandidates, "cliFlag"), fromEnv(env("CHE_PACKAGES_BINARIES_REMOTE_ARCHIVE_INSTALL_DESTINATION_CANDIDATES")),
		fromLayerList(user.Packages.BinariesRemoteArchive.InstallDestinationCandidates, "config-file"), fromLayerList(spec.Packages.BinariesRemoteArchive.InstallDestinationCandidates, "specFile"))
	if len(o.PackagesBinariesRemoteArchiveDestinationCandidates) == 0 {
		o.PackagesBinariesRemoteArchiveDestinationCandidates = packages.DefaultBinariesRemoteArchiveDestinationCandidates
		o.setValue("packages.binariesRemoteArchive.installDestinationCandidates", "["+strings.Join(packages.DefaultBinariesRemoteArchiveDestinationCandidates, ", ")+"]")
	}
	o.PackagesBinariesRemoteArchiveCheckPresentOnPath = o.resolveBool("packages.binariesRemoteArchive.checkPresentOnPath", false, env("CHE_PACKAGES_BINARIES_REMOTE_ARCHIVE_CHECK_PRESENT_ON_PATH"), true,
		boolCandidate{user.Packages.BinariesRemoteArchive.CheckPresentOnPath, "config-file"}, boolCandidate{spec.Packages.BinariesRemoteArchive.CheckPresentOnPath, "specFile"})
	o.PackagesCompletionsEnabled = o.resolveBool("packages.completions.zsh.enabled", false, env("CHE_PACKAGES_COMPLETIONS_ZSH_ENABLED"), false,
		boolCandidate{user.Packages.Completions.Zsh.Enabled, "config-file"}, boolCandidate{spec.Packages.Completions.Zsh.Enabled, "specFile"})
	o.PackagesCompletionsDestinationCandidates = o.resolveList("packages.completions.zsh.installDestinationCandidates",
		fromEnv(env("CHE_PACKAGES_COMPLETIONS_ZSH_INSTALL_DESTINATION_CANDIDATES")),
		fromLayerList(user.Packages.Completions.Zsh.InstallDestinationCandidates, "config-file"),
		fromLayerList(spec.Packages.Completions.Zsh.InstallDestinationCandidates, "specFile"))
	if len(o.PackagesCompletionsDestinationCandidates) == 0 {
		o.PackagesCompletionsDestinationCandidates = packages.DefaultCompletionsDestinationCandidates
		o.setValue("packages.completions.zsh.installDestinationCandidates", "["+strings.Join(packages.DefaultCompletionsDestinationCandidates, ", ")+"]")
	}
	o.PackagesCompletionsCheckPresentOnFpath = o.resolveBool("packages.completions.zsh.checkPresentOnFpath", false, env("CHE_PACKAGES_COMPLETIONS_ZSH_CHECK_PRESENT_ON_FPATH"), true,
		boolCandidate{user.Packages.Completions.Zsh.CheckPresentOnFpath, "config-file"}, boolCandidate{spec.Packages.Completions.Zsh.CheckPresentOnFpath, "specFile"})
	o.PackagesManpagesDestinationCandidates = o.resolveList("packages.manpages.installDestinationCandidates",
		fromEnv(env("CHE_PACKAGES_MANPAGES_INSTALL_DESTINATION_CANDIDATES")),
		fromLayerList(user.Packages.Manpages.InstallDestinationCandidates, "config-file"),
		fromLayerList(spec.Packages.Manpages.InstallDestinationCandidates, "specFile"))
	if len(o.PackagesManpagesDestinationCandidates) == 0 {
		o.PackagesManpagesDestinationCandidates = packages.DefaultManpagesDestinationCandidates
		o.setValue("packages.manpages.installDestinationCandidates", "["+strings.Join(packages.DefaultManpagesDestinationCandidates, ", ")+"]")
	}
	o.PackagesManpagesCheckPresentOnManpath = o.resolveBool("packages.manpages.checkPresentOnManpath", false, env("CHE_PACKAGES_MANPAGES_CHECK_PRESENT_ON_MANPATH"), true,
		boolCandidate{user.Packages.Manpages.CheckPresentOnManpath, "config-file"}, boolCandidate{spec.Packages.Manpages.CheckPresentOnManpath, "specFile"})
	o.AutoDiscover = o.resolveBool("autoDiscover", false, env("CHE_AUTO_DISCOVER"), true,
		boolCandidate{user.AutoDiscover, "config-file"})
	return o.resolveOtel(env, user, spec)
}

func (o *Options) resolveOtel(env LookupEnv, user, spec Layer) error {
	o.Otel.Enabled = o.resolveBool("otel.enabled", false, env("CHE_OTEL_ENABLED"), false,
		boolCandidate{user.Otel.Enabled, "config-file"}, boolCandidate{spec.Otel.Enabled, "specFile"})
	o.Otel.Protocol = o.resolveStr("otel.protocol", "grpc",
		fromEnv(env("CHE_OTEL_PROTOCOL")), fromLayer(user.Otel.Protocol, "config-file"), fromLayer(spec.Otel.Protocol, "specFile"))
	switch o.Otel.Protocol {
	case "grpc", "http":
	default:
		return fmt.Errorf("invalid otel.protocol %q: want grpc or http", o.Otel.Protocol)
	}
	o.Otel.Endpoint = o.resolveStr("otel.endpoint", defaultOtelEndpoint(o.Otel.Protocol),
		fromEnv(env("CHE_OTEL_ENDPOINT")), fromLayer(user.Otel.Endpoint, "config-file"), fromLayer(spec.Otel.Endpoint, "specFile"))
	o.Otel.Metrics = o.resolveBool("otel.metrics", false, env("CHE_OTEL_METRICS"), true,
		boolCandidate{user.Otel.Metrics, "config-file"}, boolCandidate{spec.Otel.Metrics, "specFile"})
	o.Otel.Logs = o.resolveBool("otel.logs", false, env("CHE_OTEL_LOGS"), true,
		boolCandidate{user.Otel.Logs, "config-file"}, boolCandidate{spec.Otel.Logs, "specFile"})
	o.Otel.Traces = o.resolveBool("otel.traces", false, env("CHE_OTEL_TRACES"), true,
		boolCandidate{user.Otel.Traces, "config-file"}, boolCandidate{spec.Otel.Traces, "specFile"})
	return nil
}

func defaultOtelEndpoint(protocol string) string {
	if protocol == "http" {
		return "localhost:4318"
	}
	return "localhost:4317"
}

// [<] 🤖🤖
