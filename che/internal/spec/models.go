package spec

// [>] 🤖🤖

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/invopop/jsonschema"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/execx"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

// [<] 🤖🤖

// [>] 🤖🤖 sources

type SourceType string

var SourceTypes = struct{ Remote, Filesystem SourceType }{"remote", "filesystem"}

type SourceRecipe struct {
	URI           string `yaml:"-"`
	SpecFile      string `yaml:"-"`
	DirectoryPath string `yaml:"-"`
}

type SourceReady struct {
	DefinitionURI string
	DirectoryPath string
}

type SpecSourceRecipe struct {
	SourceRecipe `yaml:"-"`
}

type SpecSourceReady struct {
	SourceReady
}

type ProfileSourceRecipe struct {
	SourceRecipe `yaml:"-"`
	ProfileName  string            `yaml:"-"`
	Src          string            `yaml:"source"`
	Options      ProfileOptions    `yaml:"options"`
	Env          map[string]string `yaml:"env"`
	Ctx          map[string]string `yaml:"ctx"`
}

type ProfileSourceReady struct {
	SourceReady
	ProfileName string
}

// [<] 🤖🤖

// [>] 🤖🤖 doc + profiles

type Doc struct {
	Options        Options
	Env            map[string]string
	Include        []SpecSourceRecipe
	ProfileRecipes []ProfileRecipe
}

type Options struct {
	RunIf                   []string        `yaml:"runIf" jsonschema_description:"spec-level predicates: gate every profile of this spec (ANDed with each profile's own); spec-only"`
	AutoDiscover            *bool           `yaml:"autoDiscover" jsonschema_description:"spec block: default for profiles that don't set it; user config: auto-discovery master switch (default true, false: only --profiles and include.profiles refs run); overridden by CHE_AUTO_DISCOVER"`
	LogLevel                string          `yaml:"logLevel" jsonschema:"enum=error,enum=warn,enum=info,enum=debug,enum=trace" jsonschema_description:"human-log level default for profiles that don't set it; overridden by --log-level and CHE_LOG_LEVEL"`
	ProfileWorkingDirectory string          `yaml:"profileWorkingDirectory" jsonschema_description:"the load-ops source tree (absolute, relative to the checkout, ~/, $VAR, env vars expanded); default the checkout itself; makeLinks/makeCopies/renderTemplates host sources resolve against it; home targeting is explicit via a $HOME dest rewrite; spec-only"`
	ValidateSpec            string          `yaml:"validateSpec" jsonschema:"enum=warn,enum=error" jsonschema_description:"how this spec's schema violations report (per-spec: each included spec honors its own); overridden by the flag and env var"`
	DryRun                  Scalar          `yaml:"dryRun" jsonschema:"enum=delta,enum=all,enum=true,enum=false" jsonschema_description:"default dry-run mode: delta (changed dests) | all (every dest) | true (alias for delta) | false (off, the default); overridden by the flag and env var"`
	Profiles                []string        `yaml:"profiles" jsonschema_description:"profiles to run (autoDiscover skipped, runIf still enforced); overridden by --profiles and CHE_PROFILE"`
	SkipRemoteRefs          *bool           `yaml:"skipRemoteRefs" jsonschema_description:"skip sourced include.profiles refs; overridden by the flag and env var"`
	Run                     Run             `yaml:"run" jsonschema_description:"run-command options"`
	Backup                  Backup          `yaml:"backup" jsonschema_description:"backup op defaults"`
	RenderTemplates         RenderTemplates `yaml:"renderTemplates" jsonschema_description:"renderTemplates op defaults"`
	Packages                Packages        `yaml:"packages" jsonschema_description:"installPackages op defaults"`
	Otel                    Otel            `yaml:"otel" jsonschema_description:"OTLP telemetry (metrics + logs) to a local collector; overridden by CHE_OTEL_* env"`
}

type Run struct {
	SkipOps []string `yaml:"skipOps" jsonschema:"enum=prune-broken-links,enum=make-dirs,enum=make-links,enum=make-copies,enum=render-templates,enum=install-packages,enum=run-scripts" jsonschema_description:"ops skipped in the run sequence only (direct op subcommands unaffected); overridden by che run --skip-ops and CHE_RUN_SKIP_OPS"`
}

type Backup struct {
	AutoCreate BackupAutoCreate `yaml:"autoCreate" jsonschema_description:"archive every would-change dest before it is mutated"`
}

type BackupAutoCreate struct {
	Enabled *bool `yaml:"enabled" jsonschema_description:"archive would-change dests before mutating them, both as the run sequence's backup stage and before a directly invoked op; false disables both and leaves che backup create untouched; default true; overridden by --backup-auto-create and CHE_BACKUP_AUTO_CREATE"`
}

type Packages struct {
	File                         string                       `yaml:"file" jsonschema_description:"packages.yml path; default $XDG_CONFIG_HOME/packages/packages.yml; overridden by --packages-file and CHE_PACKAGES_FILE"`
	PreferredInstallationMethods []string                     `yaml:"preferredInstallationMethods" jsonschema:"enum=brew,enum=cask,enum=apt,enum=npm,enum=go,enum=gem,enum=binariesRemoteArchive,enum=script,enum=buildFromSource,enum=pyenv,enum=nvm,enum=nix" jsonschema_description:"manager preference order: listed managers are tried first (in this order) within each package entry, unlisted ones follow in entry order; cascades profile > spec > user config; overridden by CHE_PACKAGES_PREFERRED_METHODS"`
	OnlyInstallationMethods      []string                     `yaml:"onlyInstallationMethods" jsonschema:"enum=brew,enum=cask,enum=apt,enum=npm,enum=go,enum=gem,enum=binariesRemoteArchive,enum=script,enum=buildFromSource,enum=pyenv,enum=nvm,enum=nix" jsonschema_description:"restrict installs to the listed managers: items using any other manager are skipped with no fallthrough, packages pulled in as requires dependencies are exempt; cascades profile > spec > user config; overridden by CHE_PACKAGES_ONLY_METHODS"`
	BinariesRemoteArchive        BinariesRemoteArchiveInstall `yaml:"binariesRemoteArchive" jsonschema_description:"binariesRemoteArchive installation method options"`
	Completions                  CompletionsInstall           `yaml:"completions" jsonschema_description:"zsh completions installation options"`
	Manpages                     ManpagesInstall              `yaml:"manpages" jsonschema_description:"manpages installation options"`
	Source                       PackagesSource               `yaml:"source" jsonschema_description:"where package definitions are read from"`
	AutoUpdate                   AutoUpdate                   `yaml:"autoUpdate" jsonschema_description:"fetch newer published package definitions before installs"`
}

type PackagesSource struct {
	URL string `yaml:"url" jsonschema_description:"definitions source: a generic package registry base or a git repository URL, recognised by shape; default the konradodwrot/che-packages registry; cascades profile > spec > user config; overridden by CHE_PACKAGES_SOURCE_URL"`
	Ref string `yaml:"ref" jsonschema_description:"exact version to fetch: a published version for a registry source, a branch, tag or commit for a git one; empty resolves the newest published version; cascades profile > spec > user config; overridden by CHE_PACKAGES_REF"`
}

type AutoUpdate struct {
	Enabled *bool        `yaml:"enabled" jsonschema_description:"before installs, fetch definitions from packages.source into the cache (failures warn and installs fall through to the cached or builtin set); runs once per che execution; default true; overridden by CHE_PACKAGES_AUTO_UPDATE"`
	If      AutoUpdateIf `yaml:"if" jsonschema_description:"conditions narrowing when autoUpdate behaviour applies"`
}

type AutoUpdateIf struct {
	RefIsLatest  AutoUpdateIfRefIsLatest  `yaml:"refIsLatest" jsonschema_description:"applies while packages.source.ref is empty, i.e. the newest version is resolved on each check"`
	DryRunIsTrue AutoUpdateIfDryRunIsTrue `yaml:"dryRunIsTrue" jsonschema_description:"applies while the run is a dry run"`
}

type AutoUpdateIfRefIsLatest struct {
	Cooldown string `yaml:"cooldown" jsonschema_description:"minimum interval between update checks (Go duration, e.g. 15m, 1h); ignored when packages.source.ref pins a version; default 15m; overridden by CHE_PACKAGES_AUTO_UPDATE_COOLDOWN"`
}

type AutoUpdateIfDryRunIsTrue struct {
	Enabled *bool `yaml:"enabled" jsonschema_description:"check for updates during a dry run; false plans against the cached or builtin set with no fetch; default true; overridden by CHE_PACKAGES_AUTO_UPDATE_IF_DRY_RUN"`
}

type CompletionsInstall struct {
	Zsh ShellCompletionsInstall `yaml:"zsh" jsonschema_description:"zsh completions installation options"`
}

type ShellCompletionsInstall struct {
	Enabled                      *bool        `yaml:"enabled" jsonschema_description:"install zsh completions for packages carrying a completions def; default false; cascades profile > spec > user config; overridden by CHE_PACKAGES_COMPLETIONS_ZSH_ENABLED"`
	InstallDestinationCandidates StringOrList `yaml:"installDestinationCandidates" jsonschema_description:"where zsh completion files install: one path or a candidate list (~/ and $VARs expand); with checkPresentOnFpath the first candidate on fpath wins, else the first entry; default [~/.local/share/zsh/site-functions, ~/.zfunc]; cascades profile > spec > user config; overridden by CHE_PACKAGES_COMPLETIONS_ZSH_INSTALL_DESTINATION_CANDIDATES (comma-separated)"`
	CheckPresentOnFpath          *bool        `yaml:"checkPresentOnFpath" jsonschema_description:"pick the first candidate destination found on fpath ($FPATH, else zsh -c fpath) and warn when none is; default true; overridden by CHE_PACKAGES_COMPLETIONS_ZSH_CHECK_PRESENT_ON_FPATH"`
}

type ManpagesInstall struct {
	InstallDestinationCandidates StringOrList `yaml:"installDestinationCandidates" jsonschema_description:"where binary-method manpages install: one path or a candidate list (~/ and $VARs expand); with checkPresentOnManpath the first candidate on the man search path wins, else the first entry; default ~/.local/share/man; cascades profile > spec > user config; overridden by CHE_PACKAGES_MANPAGES_INSTALL_DESTINATION_CANDIDATES (comma-separated)"`
	CheckPresentOnManpath        *bool        `yaml:"checkPresentOnManpath" jsonschema_description:"pick the first candidate destination found on the man search path (manpath, else $MANPATH) and warn when none is; default true; overridden by CHE_PACKAGES_MANPAGES_CHECK_PRESENT_ON_MANPATH"`
}

type BinariesRemoteArchiveInstall struct {
	InstallDestinationCandidates StringOrList `yaml:"installDestinationCandidates" jsonschema_description:"where binary-method binaries install: one path or a candidate list (~/ and $VARs expand); with checkPresentOnPath the first candidate on PATH wins, else the first entry; default ~/.local/bin; cascades profile > spec > user config; overridden by CHE_PACKAGES_BINARIES_REMOTE_ARCHIVE_INSTALL_DESTINATION_CANDIDATES (comma-separated)"`
	CheckPresentOnPath           *bool        `yaml:"checkPresentOnPath" jsonschema_description:"pick the first candidate destination found on PATH and warn when none is; default true; overridden by CHE_PACKAGES_BINARIES_REMOTE_ARCHIVE_CHECK_PRESENT_ON_PATH"`
}

type PackageRef struct {
	Name          string       `yaml:"name" jsonschema_description:"canonical package name in the packages file"`
	Versions      StringOrList `yaml:"versions,omitempty" jsonschema_description:"version(s) to install: one version or a list (exact 24.16.0 or wildcard 24.*), overriding the packages-file entry's default; multiple versions require a version-manager installation method"`
	GlobalVersion string       `yaml:"globalVersion,omitempty" jsonschema_description:"which installed version becomes the default (version-manager methods only); defaults to the first version listed"`
	Checksum      string       `yaml:"checksum,omitempty" jsonschema_description:"sha256:<hex> of the artifact the pinned version downloads, overriding the packages-file item's checksum"`
}

func (p *PackageRef) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		p.Name = node.Value
		return nil
	}
	var obj struct {
		Name          string       `yaml:"name"`
		Versions      StringOrList `yaml:"versions"`
		GlobalVersion string       `yaml:"globalVersion"`
		Checksum      string       `yaml:"checksum"`
	}
	if err := node.Decode(&obj); err != nil {
		return err
	}
	if obj.Name == "" {
		return fmt.Errorf("installPackages object form requires name")
	}
	if obj.GlobalVersion != "" && !slices.Contains(obj.Versions, obj.GlobalVersion) {
		return fmt.Errorf("installPackages %s: globalVersion %s is not among versions %v", obj.Name, obj.GlobalVersion, []string(obj.Versions))
	}
	if obj.Checksum != "" && !strings.HasPrefix(obj.Checksum, "sha256:") {
		return fmt.Errorf("installPackages %s: checksum must be sha256:<hex>: %q", obj.Name, obj.Checksum)
	}
	p.Name, p.Versions, p.GlobalVersion, p.Checksum = obj.Name, obj.Versions, obj.GlobalVersion, obj.Checksum
	return nil
}

func (p PackageRef) JSONSchema() *jsonschema.Schema {
	obj := &jsonschema.Schema{
		Type:                 "object",
		Properties:           jsonschema.NewProperties(),
		Required:             []string{"name"},
		AdditionalProperties: jsonschema.FalseSchema,
	}
	strOrList := &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		{Type: "string"},
		{Type: "array", Items: &jsonschema.Schema{Type: "string"}},
	}}
	obj.Properties.Set("name", &jsonschema.Schema{Type: "string", Description: "canonical package name in the packages file"})
	obj.Properties.Set("versions", strOrList)
	obj.Properties.Set("globalVersion", &jsonschema.Schema{Type: "string", Description: "which installed version becomes the default (version-manager methods only); defaults to the first listed"})
	obj.Properties.Set("checksum", &jsonschema.Schema{Type: "string", Pattern: "^sha256:.+", Description: "sha256:<hex> of the artifact the pinned version downloads, overriding the packages-file item's checksum"})
	return &jsonschema.Schema{OneOf: []*jsonschema.Schema{{Type: "string"}, obj}}
}

type ToolPackageRef struct {
	Name    string `yaml:"name" jsonschema_description:"package name in the tool's toolPackages map"`
	Version string `yaml:"version,omitempty" jsonschema_description:"version pin overriding the toolPackages entry's value"`
}

func (p *ToolPackageRef) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		p.Name = node.Value
		return nil
	}
	var obj struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	}
	if err := node.Decode(&obj); err != nil {
		return err
	}
	if obj.Name == "" {
		return fmt.Errorf("installToolPackages object form requires name")
	}
	p.Name, p.Version = obj.Name, obj.Version
	return nil
}

func (p ToolPackageRef) JSONSchema() *jsonschema.Schema {
	obj := &jsonschema.Schema{
		Type:                 "object",
		Properties:           jsonschema.NewProperties(),
		Required:             []string{"name"},
		AdditionalProperties: jsonschema.FalseSchema,
	}
	obj.Properties.Set("name", &jsonschema.Schema{Type: "string", Description: "package name in the tool's toolPackages map"})
	obj.Properties.Set("version", &jsonschema.Schema{Type: "string", Description: "version pin overriding the toolPackages entry's value"})
	return &jsonschema.Schema{OneOf: []*jsonschema.Schema{{Type: "string"}, obj}}
}

type StringOrList []string

type Scalar string

func (StringOrList) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{Type: "string"},
			{Type: "array", Items: &jsonschema.Schema{Type: "string"}},
		},
	}
}

type RenderTemplates struct {
	SkipSecrets *bool `yaml:"skipSecrets" jsonschema_description:"skip op:// (1Password) and gcp:// (GCP Secret Manager) secret resolution, render placeholders; overridden by the flag and env var"`
}

type Otel struct {
	Enabled  *bool  `yaml:"enabled" jsonschema_description:"emit OTLP telemetry to the collector; default off; overridden by CHE_OTEL_ENABLED"`
	Endpoint string `yaml:"endpoint" jsonschema_description:"OTLP collector endpoint (host:port); default localhost:4317 (grpc) / localhost:4318 (http); overridden by CHE_OTEL_ENDPOINT"`
	Protocol string `yaml:"protocol" jsonschema:"enum=grpc,enum=http" jsonschema_description:"OTLP transport: grpc | http; default grpc; overridden by CHE_OTEL_PROTOCOL"`
	Metrics  *bool  `yaml:"metrics" jsonschema_description:"export metrics; default on when enabled; overridden by CHE_OTEL_METRICS"`
	Logs     *bool  `yaml:"logs" jsonschema_description:"export che log lines as OTel logs; default on when enabled; overridden by CHE_OTEL_LOGS"`
	Traces   *bool  `yaml:"traces" jsonschema_description:"export spans for command/spec/profile/operation runs and external calls; default on when enabled; overridden by CHE_OTEL_TRACES"`
}

type ProfileRecipe struct {
	Source  ProfileSourceRecipe `yaml:"-" jsonschema:"-"`
	Options ProfileOptions      `yaml:"options" jsonschema_description:"when the profile runs: autoDiscover opts in to bare-che runs, runIf predicates must ALL pass; logLevel/profileWorkingDirectory cascade (most nested wins)"`
	Include includeSet          `yaml:"include"`
	Exclude excludeSet          `yaml:"exclude"`
}

type ProfileOptions struct {
	RunIf                   []string `yaml:"runIf" jsonschema_description:"predicates that must ALL pass for the profile to run"`
	AutoDiscover            *bool    `yaml:"autoDiscover" jsonschema_description:"run on bare che (nil: inherit spec options, then false: runs only via --profiles or include.profiles)"`
	LogLevel                string   `yaml:"logLevel" jsonschema:"enum=error,enum=warn,enum=info,enum=debug,enum=trace" jsonschema_description:"log level around this profile's ops (empty: inherit spec options, then che level)"`
	ProfileWorkingDirectory string   `yaml:"profileWorkingDirectory" jsonschema_description:"the profile's load-ops source tree (empty: inherit spec options, then che level, then the checkout)"`
	Packages                Packages `yaml:"packages" jsonschema_description:"installPackages options for this profile (empty fields: inherit spec options, then user config)"`
}

type includeSet struct {
	Profiles            []ProfileSourceRecipe       `yaml:"profiles" jsonschema_description:"profile refs composed depth-first before this profile's own payload: local profile name scalar, or {source, options, env} where source is <source>/<spec-file>.yml::<profile> locating a profile in another spec (its own checkout anchor)"`
	MakeLinks           []linkEntry                 `yaml:"makeLinks" jsonschema_description:"symlink-op entries, workingDirectory-relative: glob string (dest derived 1:1), {source, dest: [paths]} explicit per-file dests, or {source, dest} dest rewrite (sed-style s:^_home:$HOME: or prefix-swap sugar {source: _home/**, dest: $HOME/**} to target home)"`
	MakeCopies          []entry                     `yaml:"makeCopies" jsonschema_description:"*.ontoHost.cp copy-op perm-groups, workingDirectory-relative sources; a glob source may carry a {source, dest} dest rewrite (sed-style s:^_home:$HOME: or prefix-swap sugar _home/** -> $HOME/**)"`
	RenderTemplates     []templateNode              `yaml:"renderTemplates" jsonschema_description:"*.tpl render-op tree: a leaf renders one source to its dests, a node carrying nested <<< is a group whose source prefixes, perms, ctx and options cascade onto its descendants (innermost wins, dest options win last); local host sources workingDirectory-relative, repo-doc sources (repo dest) workingDirectory-relative, or remote (@<repo>//<path>[?ref=<ref>], explicit dest required, a group prefix carrying the ref recombines with each leaf path); glob and derived-dest forms are host sources; a glob source may carry a {source, dest} dest rewrite (sed-style s:^_home:$HOME: or prefix-swap sugar _home/** -> $HOME/**); dests expand env vars incl ${invokingSpecGitRoot}, the top-level spec's checkout"`
	MakeDirs            []dirGroup                  `yaml:"makeDirs" jsonschema_description:"extra-dir perm-groups; each item one dir path (brace-expanded)"`
	InstallPackages     []PackageRef                `yaml:"installPackages" jsonschema_description:"packages installed from the packages file (default $XDG_CONFIG_HOME/packages/packages.yml): canonical name scalar, or {name, version} to install a specific version (exact or wildcard), overriding the entry's default version; runs before runScripts"`
	InstallToolPackages map[string][]ToolPackageRef `yaml:"installToolPackages" jsonschema_description:"tool-scoped packages installed from the packages file's toolPackages section, per host tool: name scalar, or {name, version} overriding the entry's pin; runs with installPackages"`
	Scripts             []string                    `yaml:"runScripts" jsonschema_description:"script paths or globs, repo-relative, run in spec order"`
}

type excludeSet struct {
	MakeLinks           []string            `yaml:"makeLinks" jsonschema_description:"drop matching link items"`
	MakeCopies          []string            `yaml:"makeCopies" jsonschema_description:"drop matching copy items (source or dest)"`
	RenderTemplates     []string            `yaml:"renderTemplates" jsonschema_description:"drop matching template items (source or dest)"`
	MakeDirs            []string            `yaml:"makeDirs" jsonschema_description:"drop matching dirs"`
	InstallPackages     []string            `yaml:"installPackages" jsonschema_description:"drop matching canonical package names"`
	InstallToolPackages map[string][]string `yaml:"installToolPackages" jsonschema_description:"drop matching package names per host tool"`
	Scripts             []string            `yaml:"runScripts" jsonschema_description:"drop matching scripts (resolved file paths)"`
}

type linkEntry struct {
	glob     string
	DestRule string
	Source   string     `yaml:"source"`
	Dest     []DestSpec `yaml:"dest"`
}

type entry struct {
	Perms `yaml:",inline"`
	Files []fileSpec `yaml:"files" jsonschema:"required" jsonschema_description:"the group's items, each inheriting the group's perms"`
}

type templateNode struct {
	Perms    `yaml:",inline"`
	glob     string
	DestRule string
	Source   string            `yaml:"source"`
	Dest     []DestSpec        `yaml:"dest"`
	Ctx      map[string]string `yaml:"ctx" jsonschema_description:"template context, merged under each descendant's ctx (innermost keys win)"`
	Options  optionsSpec       `yaml:"options" jsonschema_description:"render options, merged under each descendant's options and each explicit dest's options (innermost set field wins, an explicit false overriding an inherited true)"`
	Children []templateNode    `yaml:"<<<" jsonschema_description:"nested nodes, inheriting this node's source and dest prefixes, perms, ctx and options"`
}

type dirGroup struct {
	Perms `yaml:",inline"`
	Files []dirSpec `yaml:"directories" jsonschema:"required" jsonschema_description:"the group's items, each inheriting the group's perms"`
}

type fileSpec struct {
	glob     string
	DestRule string
	Source   string     `yaml:"source"`
	Dest     []DestSpec `yaml:"dest"`
}

type dirSpec struct {
	glob string
	Dest []DestSpec `yaml:"dest"`
}

type DestSpec struct {
	Path    string `yaml:"path"`
	Options render.Options
	opts    optionsSpec
}

type optionsSpec struct {
	WriteType               *string `yaml:"writeType"`
	SkipAutoGeneratedHeader *bool   `yaml:"skipAutoGeneratedHeader"`
	RenderReferencedFiles   *bool   `yaml:"renderReferencedFiles"`
}

type Perms struct {
	Owner      string `yaml:"owner" jsonschema_description:"dest owner user; empty: code default"`
	OwnerGroup string `yaml:"ownerGroup" jsonschema_description:"dest owner group; empty: code default"`
	Chmod      string `yaml:"chmod" jsonschema:"pattern=^[0-7]{3\\,4}$" jsonschema_description:"dest mode, octal string"`
}

type FileItem struct {
	Rel     string
	Dests   []DestSpec
	Ctx     map[string]string
	Derived bool
	Perms
}

// [<] 🤖🤖

// [>] 🤖🤖 operation recipes

type OperationOptions struct {
	SkipSecrets bool
}

type OperationRecipe struct {
	Options OperationOptions
}

type (
	PruneLinksOperationRecipe struct {
		OperationRecipe
		Dirs []string
	}
	MakeDirsOperationRecipe struct {
		OperationRecipe
		Dirs []FileItem
	}
	MakeLinksOperationRecipe struct {
		OperationRecipe
		Links []FileItem
		Dirs  []string
	}
	MakeCopiesOperationRecipe struct {
		OperationRecipe
		Copies []FileItem
		Dirs   []string
	}
	RenderTemplatesOperationRecipe struct {
		OperationRecipe
		Templates []FileItem
	}
	InstallPackagesOperationRecipe struct {
		OperationRecipe
		Packages     []PackageRef
		ToolPackages map[string][]ToolPackageRef
	}
	RunScriptsOperationRecipe struct {
		OperationRecipe
		Scripts []string
	}
)

type OperationRecipes struct {
	PruneLinks      PruneLinksOperationRecipe
	MakeDirs        MakeDirsOperationRecipe
	MakeLinks       MakeLinksOperationRecipe
	MakeCopies      MakeCopiesOperationRecipe
	RenderTemplates RenderTemplatesOperationRecipe
	InstallPackages InstallPackagesOperationRecipe
	RunScripts      RunScriptsOperationRecipe
}

// [<] 🤖🤖

// [>] 🤖🤖 internals

type resolved struct {
	Links        []FileItem
	Copies       []FileItem
	Templates    []FileItem
	Dirs         []string
	ExtraDirs    []FileItem
	Packages     []PackageRef
	ToolPackages map[string][]ToolPackageRef
	Scripts      []string
}

type destRule struct {
	re     *regexp.Regexp
	repl   string
	global bool
}

type Evaluator struct {
	builtins  map[string]func() string
	lookupEnv func(string) string
	exec      execx.CmdExecutor
}

type templateInherited struct {
	prefix     string
	destPrefix string
	perms      Perms
	ctx        map[string]string
	options    optionsSpec
}

type globSet []globPerm

type globPerm struct {
	glob  string
	perms Perms
	rule  *destRule
}

type mergedInclude struct {
	linkGlobs         globSet
	copyGlobs         globSet
	templateGlobs     globSet
	explicitLinks     []FileItem
	explicitCopies    []FileItem
	explicitTemplates []FileItem
	dirs              []FileItem
	packages          []PackageRef
	toolPackages      map[string][]ToolPackageRef
	scripts           []string
	refs              []ProfileSourceRecipe
	exclude           excludeSet
}

// [<] 🤖🤖
