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

// TrimTmplExt strips the template suffix from rel (longest match first),
// yielding the derived dest path.
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

// profileSpec is one block: options self-describe eligibility (autoExec,
// execIf), mixinProfiles composed in order (local names only), plugins
// collected as remote-profile refs, then include (additive) and exclude
// (subtractive glob filter, applied last, wins).
type profileSpec struct {
	Options       ProfileOptions `yaml:"options" jsonschema_description:"when the profile runs: autoExec opts in to bare-che runs, execIf predicates must ALL pass"`
	MixinProfiles []string       `yaml:"mixinProfiles" jsonschema_description:"local profile names composed depth-first, in order"`
	Plugins       []pluginEntry  `yaml:"plugins"`
	Include       includeSet     `yaml:"include"`
	Exclude       excludeSet     `yaml:"exclude"`
}

// ProfileOptions self-describes when a profile runs. AutoExec (default
// false): opt in to bare-che runs; without it a profile runs only when named
// via --profile or composed via mixinProfiles. ExecIf: predicate expressions
// (`<source>` or `<source> == <literal>`, sources builtin:*/env:*),
// autoExec-eligible iff ALL pass; empty -> always.
type ProfileOptions struct {
	ExecIf   []string `yaml:"execIf"`
	AutoExec bool     `yaml:"autoExec" jsonschema_description:"run on bare che (default false: runs only via --profile or mixinProfiles)"`
}

// includeSet is the additive payload: link entries (glob-string OR
// {source, dest} rewrite), copy/template entries (glob-string OR rich
// object), mkdirs entries (path-string OR {dest}), script globs, service
// names.
type includeSet struct {
	Link            []linkEntry `yaml:"link" jsonschema_description:"symlink-op entries, repo-relative under root/: glob string (dest derived 1:1) or {source, dest} sed-style rewrite"`
	Copy            []entry     `yaml:"copy" jsonschema_description:"*.ontoHost.cp copy-op perm-groups"`
	RenderTemplates []entry     `yaml:"renderTemplates" jsonschema_description:"*.tpl render-op perm-groups; sources repo-root-relative, glob and derived-dest forms must be root/-prefixed"`
	Mkdirs          []dirGroup  `yaml:"mkdirs" jsonschema_description:"extra-dir perm-groups; each item one dir path (brace-expanded)"`
	Scripts         []string    `yaml:"runScripts" jsonschema_description:"script paths or globs, repo-relative, run in spec order"`
	Services        []string    `yaml:"services" jsonschema_description:"launchd service names"`
}

// linkEntry is one link item: a bare glob string (dest derived 1:1), or a
// {source, dest} object where dest is a sed-style rewrite rule. glob is set
// iff the glob form.
type linkEntry struct {
	glob   string
	Source string `yaml:"source"`
	Dest   string `yaml:"dest"`
}

func (l *linkEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		l.glob = value.Value
		return nil
	}
	type alias linkEntry
	return value.Decode((*alias)(l))
}

// excludeSet is the subtractive payload: every key a flat glob-string list, a
// match drops the item.
type excludeSet struct {
	Link            []string `yaml:"link" jsonschema_description:"drop matching link items"`
	Copy            []string `yaml:"copy" jsonschema_description:"drop matching copy items (source or dest)"`
	RenderTemplates []string `yaml:"renderTemplates" jsonschema_description:"drop matching template items (source or dest)"`
	Mkdirs          []string `yaml:"mkdirs" jsonschema_description:"drop matching dirs"`
	Scripts         []string `yaml:"runScripts" jsonschema_description:"drop matching scripts (resolved file paths)"`
	Services        []string `yaml:"services" jsonschema_description:"drop matching services"`
}

// DestSpec is one dest path plus its per-dest render options (render-files'
// canonical type): a scalar path, or a mapping carrying `options`.
type DestSpec struct {
	Path    string         `yaml:"path"`
	Options render.Options `yaml:"options"`
}

func (d *DestSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		d.Path = value.Value
		return nil
	}
	type alias DestSpec
	return value.Decode((*alias)(d))
}

// Perms is shared ownership/mode: empty fields mean "use the code default".
type Perms struct {
	Owner      string `yaml:"owner" jsonschema_description:"dest owner user; empty: code default"`
	OwnerGroup string `yaml:"ownerGroup" jsonschema_description:"dest owner group; empty: code default"`
	Chmod      string `yaml:"chmod" jsonschema:"pattern=^[0-7]{3\\,4}$" jsonschema_description:"dest mode, octal string"`
}

// entry is a copy/template perm-group: optional perms cascading to every
// item in Files (globs included).
type entry struct {
	Perms `yaml:",inline"`
	Files []fileSpec `yaml:"files" jsonschema:"required" jsonschema_description:"the group's items, each inheriting the group's perms"`
}

// dirGroup is a mkdirs perm-group: optional perms cascading to every item in
// Files (globs included).
type dirGroup struct {
	Perms `yaml:",inline"`
	Files []dirSpec `yaml:"files" jsonschema:"required" jsonschema_description:"the group's items, each inheriting the group's perms"`
}

// dirSpec is one item in a mkdirs perm-group's Files list: a bare dir path
// string, or a {dest} object. glob is set iff the path form.
type dirSpec struct {
	glob string
	Dest []DestSpec `yaml:"dest"`
}

func (d *dirSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		d.glob = value.Value
		return nil
	}
	type alias dirSpec
	return value.Decode((*alias)(d))
}

// fileSpec is one item in a perm-group's Files list: a bare glob string, or a
// {source, dest} object. glob is set iff the glob form.
type fileSpec struct {
	glob   string
	Source string     `yaml:"source"`
	Dest   []DestSpec `yaml:"dest"`
}

func (f *fileSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		f.glob = value.Value
		return nil
	}
	type alias fileSpec
	return value.Decode((*alias)(f))
}

// FileItem is one resolved file: repo-relative source (templates:
// repo-root-relative; links/copies: under root/), explicit dests (nil ->
// derived in host), optional perms. Per-dest render options live on each
// DestSpec.
type FileItem struct {
	Rel   string
	Dests []DestSpec
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

// Load parses che.yml: every top-level key is a defined profile block.
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
