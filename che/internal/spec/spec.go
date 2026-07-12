// Package spec parses, schema-validates, and resolves che.yml: profiles, include/exclude sets, plugin refs.
package spec

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

const (
	TmplExt    = ".tpl"
	CpExt      = ".ontoHost.cp"
	RootPrefix = "root/"
)

// tmplExts are the accepted template suffixes, longest first (informational
// only: the dest path decides host vs repo).
var tmplExts = []string{".ontoHost.tpl", ".ontoRepo.tpl", ".tpl"}

// IsTmplSrc reports whether rel is a template source (any accepted suffix).
func IsTmplSrc(rel string) bool { return strings.HasSuffix(rel, TmplExt) }

// RemoteSrcPrefix marks a renderTemplates source as remote:
// @<repo>//<path>[?ref=<ref>], fetched at render time.
const RemoteSrcPrefix = "@"

// IsRemoteSrc reports whether source is a remote template source.
func IsRemoteSrc(source string) bool { return strings.HasPrefix(source, RemoteSrcPrefix) }

// RemoteSrcRef strips the remote marker, yielding the fetchable ref.
func RemoteSrcRef(source string) string { return strings.TrimPrefix(source, RemoteSrcPrefix) }

// TrimTmplExt strips the template suffix (longest first), yielding the derived dest.
func TrimTmplExt(rel string) string {
	for _, ext := range tmplExts {
		if trimmed, ok := strings.CutSuffix(rel, ext); ok {
			return trimmed
		}
	}
	return rel
}

// Raw mirrors che.yml: every top-level key is a defined profile block.
type Raw struct {
	profiles map[string]profileSpec // every defined block, keyed by name
	order    []string               // profile names in declaration order
}

type profileSpec struct {
	Options       ProfileOptions `yaml:"options" jsonschema_description:"when the profile runs: autoExec opts in to bare-che runs, execIf predicates must ALL pass"`
	MixinProfiles []string       `yaml:"mixinProfiles" jsonschema_description:"local profile names composed depth-first, in order"`
	Plugins       []pluginEntry  `yaml:"plugins"`
	Include       includeSet     `yaml:"include"`
	Exclude       excludeSet     `yaml:"exclude"`
}

// ProfileOptions self-describes when a profile runs.
type ProfileOptions struct {
	ExecIf   []string `yaml:"execIf"`
	AutoExec bool     `yaml:"autoExec" jsonschema_description:"run on bare che (default false: runs only via --profile or mixinProfiles)"`
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

// linkEntry: bare glob string or {source, dest}. glob set iff the glob form.
type linkEntry struct {
	glob   string
	Source string `yaml:"source"`
	Dest   string `yaml:"dest"`
}

func (l *linkEntry) UnmarshalYAML(value *yaml.Node) error {
	type alias linkEntry
	return decodeScalarOr(value, &l.glob, (*alias)(l))
}

// decodeScalarOr implements the scalar-or-object union form shared by every
// spec entry type: a scalar node's value lands in scalar, anything else
// decodes into obj (an alias type, sidestepping UnmarshalYAML recursion).
func decodeScalarOr[T any](value *yaml.Node, scalar *string, obj *T) error {
	if value.Kind == yaml.ScalarNode {
		*scalar = value.Value
		return nil
	}
	return value.Decode(obj)
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

// DestSpec is one dest path plus its per-dest render options. opts keeps the
// presence-aware raw form so group-level options merge per field.
type DestSpec struct {
	Path    string `yaml:"path"`
	Options render.Options
	opts    optionsSpec
}

func (d *DestSpec) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Path    string      `yaml:"path"`
		Options optionsSpec `yaml:"options"`
	}
	if err := decodeScalarOr(value, &d.Path, &raw); err != nil {
		return err
	}
	if raw.Path != "" {
		d.Path = raw.Path
	}
	d.opts = raw.Options
	d.Options = raw.Options.over(render.Options{})
	return nil
}

// optionsSpec mirrors render.Options with pointer fields: nil = field not set
// in yaml, letting a dest override or inherit group-level options per field.
type optionsSpec struct {
	WriteType               *string `yaml:"writeType"`
	SkipAutoGeneratedHeader *bool   `yaml:"skipAutoGeneratedHeader"`
	RenderReferencedFiles   *bool   `yaml:"renderReferencedFiles"`
}

func (o optionsSpec) over(base render.Options) render.Options {
	if o.WriteType != nil {
		base.WriteType = *o.WriteType
	}
	if o.SkipAutoGeneratedHeader != nil {
		base.SkipAutoGeneratedHeader = *o.SkipAutoGeneratedHeader
	}
	if o.RenderReferencedFiles != nil {
		base.RenderReferencedFiles = *o.RenderReferencedFiles
	}
	return base
}

// Perms is shared ownership/mode: empty fields mean "use the code default".
type Perms struct {
	Owner      string `yaml:"owner" jsonschema_description:"dest owner user; empty: code default"`
	OwnerGroup string `yaml:"ownerGroup" jsonschema_description:"dest owner group; empty: code default"`
	Chmod      string `yaml:"chmod" jsonschema:"pattern=^[0-7]{3\\,4}$" jsonschema_description:"dest mode, octal string"`
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

// dirSpec: bare dir path string or {dest}. glob set iff the path form.
type dirSpec struct {
	glob string
	Dest []DestSpec `yaml:"dest"`
}

func (d *dirSpec) UnmarshalYAML(value *yaml.Node) error {
	type alias dirSpec
	return decodeScalarOr(value, &d.glob, (*alias)(d))
}

// fileSpec: bare glob string or {source, dest}. glob set iff the glob form.
type fileSpec struct {
	glob   string
	Source string            `yaml:"source"`
	Dest   []DestSpec        `yaml:"dest"`
	Ctx    map[string]string `yaml:"ctx" jsonschema_description:"renderTemplates only: values exposed as the template's root context (.key)"`
}

func (f *fileSpec) UnmarshalYAML(value *yaml.Node) error {
	type alias fileSpec
	return decodeScalarOr(value, &f.glob, (*alias)(f))
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

// LinkDestRel is a link item's repo-relative dest path: the rewritten
// Dests[0] when a dest rule applied, else the source Rel (1:1).
func LinkDestRel(it FileItem) string {
	if len(it.Dests) > 0 {
		return it.Dests[0].Path
	}
	return it.Rel
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

// Load parses che.yml.
func Load(path string) (*Raw, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("spec not found: %s", path)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	s := &Raw{profiles: map[string]profileSpec{}}
	if len(doc.Content) == 0 {
		return s, nil
	}
	m := doc.Content[0]
	for i := 0; i+1 < len(m.Content); i += 2 {
		key := m.Content[i].Value
		var ps profileSpec
		if err := m.Content[i+1].Decode(&ps); err != nil {
			return nil, fmt.Errorf("parse profile %q: %w", key, err)
		}
		s.profiles[key] = ps
		s.order = append(s.order, key)
	}
	return s, nil
}

// [<] 🤖🤖
