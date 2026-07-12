package spec

// [>] 🤖🤖

import (
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

// Domain model:
//
//	CheSpec  che.yml: every top-level key one Profile
//	  Profile
//	    ProfileOptions  eligibility: autoDiscover, execIf predicates
//	    mixinProfiles   local profile names composed depth-first
//	    plugins         pluginEntry -> PluginRef: a profile from another repo
//	    includeSet      additive payload per op
//	      linkEntry / entry / dirGroup   perm-groups (Perms cascade to items)
//	        fileSpec / dirSpec           scalar-or-object union items
//	          DestSpec                   dest path + per-dest render options
//	    excludeSet      subtractive glob filter, applied last, wins
//	Resolved  Resolve output, the selection the ops consume
//	  FileItem  one resolved file: source rel, dests, perms, template ctx

// CheSpec mirrors che.yml: every top-level key is a defined profile block.
type CheSpec struct {
	profiles []Profile // in declaration order
}

type Profile struct {
	Name          string         `yaml:"-" jsonschema:"-"`
	Options       ProfileOptions `yaml:"options" jsonschema_description:"when the profile runs: autoDiscover opts in to bare-che runs, execIf predicates must ALL pass"`
	MixinProfiles []string       `yaml:"mixinProfiles" jsonschema_description:"local profile names composed depth-first, in order"`
	Plugins       []pluginEntry  `yaml:"plugins"`
	Include       includeSet     `yaml:"include"`
	Exclude       excludeSet     `yaml:"exclude"`
}

// ProfileOptions self-describes when a profile runs.
type ProfileOptions struct {
	ExecIf       []string `yaml:"execIf"`
	AutoDiscover bool     `yaml:"autoDiscover" jsonschema_description:"run on bare che (default false: runs only via --profile or mixinProfiles)"`
}

// PluginRef is one parsed plugins entry: a profile defined in another repo,
// loaded and anchored at its own checkout, optionally with envs exported
// around its load. IsPath marks a local dir ref (no `@` prefix): URL then
// holds the dir path (absolute, relative, ~/, $VAR).
type PluginRef struct {
	URL     string
	Profile string
	Env     map[string]string
	IsPath  bool
}

// pluginEntry is one plugins list item: a bare `@<giturl>::<profile>` /
// `<dir>::<profile>` string, or a {ref, env} object.
type pluginEntry struct {
	Ref string            `yaml:"ref"`
	Env map[string]string `yaml:"env"`
}

// includeSet is the additive payload.
type includeSet struct {
	Link            []linkEntry `yaml:"link" jsonschema_description:"symlink-op entries, repo-relative under root/: glob string (dest derived 1:1) or {source, dest} sed-style rewrite"`
	Copy            []entry     `yaml:"copy" jsonschema_description:"*.ontoHost.cp copy-op perm-groups"`
	RenderTemplates []entry     `yaml:"renderTemplates" jsonschema_description:"*.tpl render-op perm-groups; sources repo-root-relative or remote (@<repo>//<path>[?ref=<ref>], explicit dest required), glob and derived-dest forms must be root/-prefixed"`
	Mkdirs          []dirGroup  `yaml:"mkdirs" jsonschema_description:"extra-dir perm-groups; each item one dir path (brace-expanded)"`
	Scripts         []string    `yaml:"runScripts" jsonschema_description:"script paths or globs, repo-relative, run in spec order"`
	Services        []string    `yaml:"services" jsonschema_description:"launchd service names"`
}

// excludeSet is the subtractive payload: flat glob-string lists.
type excludeSet struct {
	Link            []string `yaml:"link" jsonschema_description:"drop matching link items"`
	Copy            []string `yaml:"copy" jsonschema_description:"drop matching copy items (source or dest)"`
	RenderTemplates []string `yaml:"renderTemplates" jsonschema_description:"drop matching template items (source or dest)"`
	Mkdirs          []string `yaml:"mkdirs" jsonschema_description:"drop matching dirs"`
	Scripts         []string `yaml:"runScripts" jsonschema_description:"drop matching scripts (resolved file paths)"`
	Services        []string `yaml:"services" jsonschema_description:"drop matching services"`
}

// linkEntry: bare glob string or {source, dest}. glob set iff the glob form.
type linkEntry struct {
	glob   string
	Source string `yaml:"source"`
	Dest   string `yaml:"dest"`
}

// entry is a copy/template perm-group.
type entry struct {
	Perms   `yaml:",inline"`
	Ctx     map[string]string `yaml:"ctx" jsonschema_description:"renderTemplates only: group-level template context, merged under each item's ctx (item keys win)"`
	Options render.Options    `yaml:"options" jsonschema_description:"renderTemplates only: group-level render options, merged under each explicit dest's options (dest-set fields win)"`
	Files   []fileSpec        `yaml:"files" jsonschema:"required" jsonschema_description:"the group's items, each inheriting the group's perms"`
}

// dirGroup is a mkdirs perm-group.
type dirGroup struct {
	Perms `yaml:",inline"`
	Files []dirSpec `yaml:"files" jsonschema:"required" jsonschema_description:"the group's items, each inheriting the group's perms"`
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
// (nil -> derived in host), optional perms and template context.
type FileItem struct {
	Rel   string
	Dests []DestSpec
	Ctx   map[string]string
	Perms
}

// Resolved is the classified, repo-relative selection the ops consume.
type Resolved struct {
	Links     []FileItem  // link op: regular files minus templates/copies/.gitkeep
	Copies    []FileItem  // copy op: *.ontoHost.cp
	Templates []FileItem  // render op: *.tpl, dest path decides host vs repo
	Dirs      []string    // every ancestor dir of links+copies+derived-dest templates, plus mkdirs
	ExtraDirs []FileItem  // mkdirs only (live dest entries), one per path, carrying perms
	Services  []string    // service names
	Scripts   []string    // script entries in spec order
	Plugins   []PluginRef // profile-level plugins entries, composition order
}

// [<] 🤖🤖
