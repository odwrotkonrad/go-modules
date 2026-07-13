package spec

// [>] 🤖🤖

import (
	"regexp"

	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

// Domain model:
//
//	Doc  che.yml: reserved top-level keys options/env/include, every other
//	     top-level key one ProfileRecipe
//	  Options  top-level options: block: spec-wide defaults + che knobs
//	  include  sources: other specs composed in (SpecSourceRecipe list)
//	  ProfileRecipe  raw declared profile
//	    Source          ProfileSourceRecipe stamped at parse
//	    ProfileOptions  eligibility + cascade: autoDiscover, execIf, debug, workingDirectory
//	    includeSet      additive payload per op, plus profiles: profile refs
//	      linkEntry / entry / dirGroup   perm-groups (Perms cascade to items)
//	        fileSpec / dirSpec           scalar-or-object union items
//	          DestSpec                   dest path + per-dest render options
//	    excludeSet      subtractive glob filter, applied last, wins
//	Sources  Recipe/Ready pairs locating a spec (SpecSource*) or one profile
//	         within a spec (ProfileSource*), sharing embedded parents
//	OperationRecipes  MakeProfile output: per-kind operation recipes with the
//	                  profile's UNRESOLVED subjects, in run order
//	  FileItem  one resolved file: source rel, dests, perms, template ctx

// [<] 🤖🤖

// [>] 🤖🤖 sources

// SourceType classifies a source URI: remote (@ prefix) | filesystem.
type SourceType string

// SourceTypes namespaces the SourceType values.
var SourceTypes = struct{ Remote, Filesystem SourceType }{"remote", "filesystem"}

// SourceRecipe is the shared declared-source parent.
type SourceRecipe struct {
	// URI locates the source: <dir> (absolute, relative, ~/, $VAR) or
	// @<giturl>; "" -> the local repo.
	URI string `yaml:"-"`
	// SpecFile is the spec-file path within the source (relative to the
	// resolved dir); "" -> che.yml.
	SpecFile string `yaml:"-"`
	// DirectoryPath is where the source lives locally; defaults: remote ->
	// the XDG cache dir it clones into, filesystem -> the expanded URI dir,
	// "" local -> the repo root. The directory option (cascade) lands here.
	DirectoryPath string `yaml:"-"`
}

// SourceReady is the shared resolved-source parent. DirectoryPath is the
// checkout anchor (che.yml + repo-relative scripts/templates). The load-ops
// working dir and the $HOME-mapping folder are option-level (Che/Spec/Profile),
// resolved and threaded separately, not source properties.
type SourceReady struct {
	DefinitionURI string
	DirectoryPath string
}

// SpecSourceRecipe locates a whole spec (a che.yml): top-level include.sources
// entries, the spec's own Source.
type SpecSourceRecipe struct {
	SourceRecipe `yaml:"-"`
}

// SpecSourceReady is a resolved SpecSourceRecipe.
type SpecSourceReady struct {
	SourceReady
}

// ProfileSourceRecipe locates one profile within a spec: include.profiles
// entries (scalar = local profile name; rich {source, options, env} where
// source is <uri>/<spec-file>.yml::<profile>), and each ProfileRecipe's parse-stamped Source.
type ProfileSourceRecipe struct {
	SourceRecipe `yaml:"-"`
	ProfileName  string `yaml:"-"`
	// Src is the entry's source URI, <source>/<spec-file>.yml::<profile> (source: @<giturl> or
	// <dir>; bare <profile> -> local). Split at decode into URI + SpecFile + ProfileName.
	Src string `yaml:"source"`
	// Options are the entry-set overrides of the referenced profile's own
	// (one more cascade level, most nested wins).
	Options ProfileOptions `yaml:"options"`
	// Env is the overlay exported around the referenced profile (sourced refs only).
	Env map[string]string `yaml:"env"`
}

// ProfileSourceReady is a resolved ProfileSourceRecipe.
type ProfileSourceReady struct {
	SourceReady
	ProfileName string
}

// [<] 🤖🤖

// [>] 🤖🤖 doc + profiles

// Doc mirrors che.yml: reserved keys options/env/include, every other
// top-level key a defined profile block.
type Doc struct {
	Options        Options
	Env            map[string]string
	Include        []SpecSourceRecipe
	ProfileRecipes []ProfileRecipe
}

// Options is the top-level options: block: spec-wide defaults + che knobs.
type Options struct {
	ExecIf           []string `yaml:"execIf" jsonschema_description:"spec-level predicates: gate every profile of this spec (ANDed with each profile's own)"`
	AutoDiscover     *bool    `yaml:"autoDiscover" jsonschema_description:"default for profiles that don't set it"`
	Debug            *bool    `yaml:"debug" jsonschema_description:"default for profiles that don't set it"`
	WorkingDirectory string   `yaml:"workingDirectory" jsonschema_description:"the load-ops source tree (absolute, relative to the checkout, ~/, $VAR, env vars expanded); default root; the RootPrefix logical token maps onto it, the HOME/ folder under it maps onto $HOME"`
	ValidateSpec     string   `yaml:"validateSpec" jsonschema:"enum=warn,enum=error" jsonschema_description:"how this spec's schema violations report (per-spec: each included spec honors its own); overridden by the flag and env var"`
}

// ProfileRecipe is one raw declared profile.
type ProfileRecipe struct {
	Source  ProfileSourceRecipe `yaml:"-" jsonschema:"-"`
	Options ProfileOptions      `yaml:"options" jsonschema_description:"when the profile runs: autoDiscover opts in to bare-che runs, execIf predicates must ALL pass; debug/workingDirectory cascade (most nested wins)"`
	Include includeSet          `yaml:"include"`
	Exclude excludeSet          `yaml:"exclude"`
}

// ProfileOptions self-describes when and where a profile runs. Pointer/zero
// fields inherit the level above (profile > spec > che, most nested wins).
type ProfileOptions struct {
	ExecIf           []string `yaml:"execIf"`
	AutoDiscover     *bool    `yaml:"autoDiscover" jsonschema_description:"run on bare che (nil: inherit spec options, then false: runs only via --profile or include.profiles)"`
	Debug            *bool    `yaml:"debug" jsonschema_description:"print debug-level lines around this profile (nil: inherit spec options, then che level)"`
	WorkingDirectory string   `yaml:"workingDirectory" jsonschema_description:"the profile's load-ops source tree (empty: inherit spec options, then che level, then the checkout)"`
}

// includeSet is the additive payload.
type includeSet struct {
	Profiles        []ProfileSourceRecipe `yaml:"profiles" jsonschema_description:"profile refs composed depth-first before this profile's own payload: local profile name scalar, or {source, options, env} where source is <source>/<spec-file>.yml::<profile> locating a profile in another spec (its own checkout anchor)"`
	MakeLinks       []linkEntry           `yaml:"makeLinks" jsonschema_description:"symlink-op entries, repo-relative under root/: glob string (dest derived 1:1) or {source, dest} sed-style rewrite"`
	MakeCopies      []entry               `yaml:"makeCopies" jsonschema_description:"*.ontoHost.cp copy-op perm-groups"`
	RenderTemplates []templateGroup       `yaml:"renderTemplates" jsonschema_description:"*.tpl render-op perm-groups; sources repo-root-relative or remote (@<repo>//<path>[?ref=<ref>], explicit dest required), glob and derived-dest forms must be root/-prefixed"`
	MakeDirs        []dirGroup            `yaml:"makeDirs" jsonschema_description:"extra-dir perm-groups; each item one dir path (brace-expanded)"`
	Scripts         []string              `yaml:"runScripts" jsonschema_description:"script paths or globs, repo-relative, run in spec order"`
	Services        []string              `yaml:"runServices" jsonschema_description:"launchd service names"`
}

// excludeSet is the subtractive payload: flat glob-string lists.
type excludeSet struct {
	MakeLinks       []string `yaml:"makeLinks" jsonschema_description:"drop matching link items"`
	MakeCopies      []string `yaml:"makeCopies" jsonschema_description:"drop matching copy items (source or dest)"`
	RenderTemplates []string `yaml:"renderTemplates" jsonschema_description:"drop matching template items (source or dest)"`
	MakeDirs        []string `yaml:"makeDirs" jsonschema_description:"drop matching dirs"`
	Scripts         []string `yaml:"runScripts" jsonschema_description:"drop matching scripts (resolved file paths)"`
	Services        []string `yaml:"runServices" jsonschema_description:"drop matching services"`
}

// linkEntry: bare glob string or {source, dest}. glob set iff the glob form.
type linkEntry struct {
	glob   string
	Source string `yaml:"source"`
	Dest   string `yaml:"dest"`
}

// entry is a makeCopies perm-group.
type entry struct {
	Perms `yaml:",inline"`
	Files []fileSpec `yaml:"files" jsonschema:"required" jsonschema_description:"the group's items, each inheriting the group's perms"`
}

// templateGroup is a renderTemplates perm-group.
type templateGroup struct {
	Perms   `yaml:",inline"`
	Ctx     map[string]string `yaml:"ctx" jsonschema_description:"group-level template context, merged under each item's ctx (item keys win)"`
	Options render.Options    `yaml:"options" jsonschema_description:"group-level render options, merged under each explicit dest's options (dest-set fields win)"`
	Files   []fileSpec        `yaml:"templates" jsonschema:"required" jsonschema_description:"the group's items, each inheriting the group's perms"`
}

// dirGroup is a makeDirs perm-group.
type dirGroup struct {
	Perms `yaml:",inline"`
	Files []dirSpec `yaml:"directories" jsonschema:"required" jsonschema_description:"the group's items, each inheriting the group's perms"`
}

// fileSpec: bare glob string or {source, dest}. glob set iff the glob form.
type fileSpec struct {
	glob   string
	Source string            `yaml:"source"`
	Dest   []DestSpec        `yaml:"dest"`
	Ctx    map[string]string `yaml:"ctx" jsonschema_description:"renderTemplates only: values exposed as the template's root context (.key)"`
}

// dirSpec: bare dir path string or {dest}. glob set iff the path form.
type dirSpec struct {
	glob string
	Dest []DestSpec `yaml:"dest"`
}

// DestSpec is one dest path plus its per-dest render options. opts keeps the
// presence-aware raw form so group-level options merge per field.
type DestSpec struct {
	Path    string `yaml:"path"`
	Options render.Options
	opts    optionsSpec
}

// optionsSpec mirrors render.Options with pointer fields: nil = field not set
// in yaml, letting a dest override or inherit group-level options per field.
type optionsSpec struct {
	WriteType               *string `yaml:"writeType"`
	SkipAutoGeneratedHeader *bool   `yaml:"skipAutoGeneratedHeader"`
	RenderReferencedFiles   *bool   `yaml:"renderReferencedFiles"`
}

// Perms is shared ownership/mode: empty fields mean "use the code default".
type Perms struct {
	Owner      string `yaml:"owner" jsonschema_description:"dest owner user; empty: code default"`
	OwnerGroup string `yaml:"ownerGroup" jsonschema_description:"dest owner group; empty: code default"`
	Chmod      string `yaml:"chmod" jsonschema:"pattern=^[0-7]{3\\,4}$" jsonschema_description:"dest mode, octal string"`
}

// FileItem is one resolved file: source (templates: repo-root-relative or
// @-prefixed remote ref, links/copies: under root/), explicit dests
// (nil -> derived in host), optional perms and template context. MakeDirs items
// without Dests are ancestor dirs (path in Rel, zero perms).
type FileItem struct {
	Rel   string
	Dests []DestSpec
	Ctx   map[string]string
	Perms
}

// [<] 🤖🤖

// [>] 🤖🤖 operation recipes

// OperationOptions is the shared per-op recipe knob set.
type OperationOptions struct {
	SkipSecrets bool // render: skip sources carrying op:// secret refs
}

// OperationRecipe is the shared operation-recipe parent: pure data, no Host.
type OperationRecipe struct {
	Options OperationOptions
}

// Per-kind operation recipes: one per subcommand, subjects UNRESOLVED
// (spec domain, emitted by MakeProfile).
type (
	PruneLinksOperationRecipe struct {
		OperationRecipe
		Dirs []string
	}
	MakeDirsOperationRecipe struct {
		OperationRecipe
		Dirs []FileItem // ancestor dirs (Rel, zero perms) + makeDirs entries (Dests + perms), one list
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
	RunScriptsOperationRecipe struct {
		OperationRecipe
		Scripts []string // repo-relative, run order
	}
	RunServicesOperationRecipe struct {
		OperationRecipe
		Services []string // service names
	}
)

// OperationRecipes is the ordered per-kind recipe set one profile selects:
// MakeProfile output, field order = run order.
type OperationRecipes struct {
	PruneLinks      PruneLinksOperationRecipe
	MakeDirs        MakeDirsOperationRecipe
	MakeLinks       MakeLinksOperationRecipe
	MakeCopies      MakeCopiesOperationRecipe
	RenderTemplates RenderTemplatesOperationRecipe
	RunScripts      RunScriptsOperationRecipe
	RunServices     RunServicesOperationRecipe
}

// [<] 🤖🤖

// [>] 🤖🤖 internals

// resolved is the classified, repo-relative selection MakeProfile builds
// before emitting OperationRecipes.
type resolved struct {
	Links     []FileItem // link op: regular files minus templates/copies/.gitkeep
	Copies    []FileItem // makeCopies op: *.ontoHost.cp
	Templates []FileItem // render op: *.tpl, dest path decides host vs repo
	Dirs      []string   // every ancestor dir of links+copies+derived-dest templates
	ExtraDirs []FileItem // makeDirs only (live dest entries), one per path, carrying perms
	Services  []string   // service names
	Scripts   []string   // script entries in spec order
}

// destRule is a parsed sed-style dest rewrite: pattern, replacement ($1
// backrefs), global flag (absent -> first match only).
type destRule struct {
	re     *regexp.Regexp
	repl   string
	global bool
}

// Evaluator resolves execIf predicate expressions. Builtins are lazy (resolved
// only when referenced) and cached per run ([why] IsVirtualized shells out).
type Evaluator struct {
	builtins map[string]func() string
}

// globSet is an ordered list of op globs, each carrying its group's perms
// (zero Perms if none) and an optional link dest rule. Globs are
// brace-expanded on add.
type globSet []globPerm

type globPerm struct {
	glob  string
	perms Perms
	rule  *destRule
}

// effective is the composed additive selection before classification + exclude.
// Each op's globs carry their group's perms; classify stamps matched files
// with them (last match wins).
type effective struct {
	linkGlobs globSet               // link-op globs (repo-relative under root/)
	copyGlobs globSet               // copy-op globs
	tmplGlobs globSet               // render-templates globs (repo-root-relative, root/-prefixed)
	richCopy  []FileItem            // rich-form copy entries
	richTmpl  []FileItem            // rich-form render-templates entries (repo-root-relative)
	dirs      []FileItem            // makeDirs: glob forms expanded to one item per path, rich carry perms
	scripts   []string              // script paths (order = run order)
	services  []string              // service names
	refs      []ProfileSourceRecipe // sourced include.profiles refs (composition order)
	exclude   excludeSet            // accumulated exclude globs (applied last, wins)
}

// [<] 🤖🤖
