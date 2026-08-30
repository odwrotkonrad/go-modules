package options

// [>] 🤖🤖

import (
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec/envinterp"
)

type DryRunMode string

var DryRun = struct{ Off, Delta, All DryRunMode }{"", "delta", "all"}

type ValidateSpecMode string

var ValidateSpec = struct{ Warn, Error ValidateSpecMode }{"warn", "error"}

var OpNames = []string{"prune-broken-links", "make-dirs", "make-links", "make-copies", "render-templates", "install-packages", "run-scripts"}

type Options struct {
	CheWorkingDirectory                                string
	ProfileWorkingDirectory                            string
	DryRun                                             DryRunMode
	ValidateSpec                                       ValidateSpecMode
	ValidateSpecCLI                                    ValidateSpecMode
	EnvUnset                                           envinterp.Policy
	EnvUnsetReport                                     bool
	Profiles                                           []string
	TargetProfileTypes                                 []string
	RunSkipOps                                         []string
	BackupAutoCreateFlag                               *bool
	BackupAutoCreateDisabled                           bool
	SkipRunIf                                          bool
	SkipRemoteRefs                                     bool
	Errexit                                            bool
	LogLevel                                           string
	RenderSkipVariables                                bool
	RenderMergeUpdate                                  string
	PackagesFile                                       string
	PackagesOverride                                   string
	PackagesUpdate                                     bool
	PackagesIfMissing                                  bool
	PackagesMissingMethodWarn                          bool
	PackagesPreferredMethods                           []string
	PackagesOnlyMethods                                []string
	PackagesDownloadCacheDir                           string
	PackagesBinariesRemoteArchiveDestinationCandidates []string
	PackagesBinariesRemoteArchiveCheckPresentOnPath    bool
	PackagesCompletionsEnabled                         bool
	PackagesCompletionsDestinationCandidates           []string
	PackagesCompletionsCheckPresentOnFpath             bool
	PackagesManpagesDestinationCandidates              []string
	PackagesManpagesCheckPresentOnManpath              bool
	PackagesSilenceInstallStdout                       string
	PackagesSourceURL                                  string
	PackagesSourceRef                                  string
	PackagesAutoUpdateEnabled                          bool
	PackagesAutoUpdateCooldown                         string
	PackagesAutoUpdateDryRunEnabled                    bool
	AutoDiscover                                       bool
	Otel                                               Otel
	Settings                                           []Setting
}

type Otel struct {
	Enabled  bool
	Endpoint string
	Protocol string
	Metrics  bool
	Logs     bool
	Traces   bool
}

type Layer = spec.Options

// [<] 🤖🤖
