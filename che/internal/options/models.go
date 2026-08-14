package options

// [>] 🤖🤖

import "gitlab.com/konradodwrot/go-modules/che/internal/spec"

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
	Profiles                                           []string
	SkipOps                                            []string
	RunSkipOps                                         []string
	SkipRunIf                                          bool
	SkipRemoteRefs                                     bool
	Errexit                                            bool
	LogLevel                                           string
	RenderSkipSecrets                                  bool
	PackagesFile                                       string
	PackagesOverride                                   string
	PackagesUpdate                                     bool
	PackagesIfMissing                                  bool
	PackagesPreferredMethods                           []string
	PackagesOnlyMethods                                []string
	PackagesDownloadCacheDir                           string
	PackagesBinariesRemoteArchiveDestinationCandidates []string
	PackagesBinariesRemoteArchiveCheckPresentOnPath    bool
	PackagesCompletionsEnabled                         bool
	PackagesCompletionsDestinationCandidates           []string
	PackagesCompletionsCheckPresentOnFpath             bool
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
