package spec

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go/che/internal/fsutil"
)

const (
	TmplExt = ".host.tpl"
	CpExt   = ".host.cp"
)

// Raw mirrors che.yml. Profiles is the enum tree of declared leaves; other
// top-level keys are defined blocks: leaf profiles and mixin profiles.
type Raw struct {
	Profiles map[string]any         `yaml:"profiles"`
	profiles map[string]profileSpec // every defined block, keyed by name
}

// profileSpec is one block: mixin-profiles composed in order, then include
// (additive) and exclude (subtractive glob filter, applied last, wins).
type profileSpec struct {
	MixinProfiles []string   `yaml:"mixin-profiles"`
	Include       includeSet `yaml:"include"`
	Exclude       excludeSet `yaml:"exclude"`
}

// includeSet is the additive payload: link globs, copy/template/mkdirs entries
// (glob-string OR rich object), script globs, service names.
type includeSet struct {
	Link     []string `yaml:"link"`
	Copy     []entry  `yaml:"copy"`
	Template []entry  `yaml:"template"`
	Mkdirs   []entry  `yaml:"mkdirs"`
	Scripts  []string `yaml:"run-scripts"`
	Services []string `yaml:"services"`
}

// excludeSet is the subtractive payload: every key a flat glob-string list, a
// match drops the item.
type excludeSet struct {
	Link     []string `yaml:"link"`
	Copy     []string `yaml:"copy"`
	Template []string `yaml:"template"`
	Mkdirs   []string `yaml:"mkdirs"`
	Scripts  []string `yaml:"run-scripts"`
	Services []string `yaml:"services"`
}

// DestSpec is one dest path plus per-dest template option: a scalar path, or a
// mapping carrying the render option.
type DestSpec struct {
	Path                  string `yaml:"path"`
	RenderReferencedFiles bool   `yaml:"render-referenced-files"`
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
	Owner      string `yaml:"owner"`
	OwnerGroup string `yaml:"owner-group"`
	Chmod      string `yaml:"chmod"`
}

// entry is a copy/template/mkdirs perm-group: optional perms cascading to every
// item in Files (globs included).
type entry struct {
	Perms `yaml:",inline"`
	Files []fileSpec `yaml:"files"`
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

// FileItem is one resolved file: repo-relative source (under root/), explicit
// dests (nil -> derived in host), optional perms.
type FileItem struct {
	Rel   string
	Dests []DestSpec
	Perms
}

// Resolved is the classified, repo-relative selection the ops consume.
type Resolved struct {
	Links     []FileItem // link op: regular files minus templates/copies/.gitkeep
	Copies    []FileItem // copy op: *.host.cp
	Templates []FileItem // render op: *.host.tpl
	Dirs      []string   // every ancestor dir of links+copies+templates, plus mkdirs
	ExtraDirs []FileItem // mkdirs only (live dest entries), one per path, carrying perms
	Services  []string   // service names
	Scripts   []string   // script entries in spec order
}

// globSet is an ordered list of op globs, each carrying its group's perms
// (zero Perms if none). Globs are brace-expanded on add.
type globSet []globPerm

type globPerm struct {
	glob  string
	perms Perms
}

func (gs *globSet) add(glob string, perms Perms) {
	for _, g := range fsutil.ExpandBraces(glob) {
		*gs = append(*gs, globPerm{glob: g, perms: perms})
	}
}

// match returns the perms of the last glob matching rel, and whether any did.
func (gs globSet) match(rel string) (perms Perms, hit bool) {
	for _, g := range gs {
		if globMatch(g.glob, rel) {
			perms, hit = g.perms, true
		}
	}
	return perms, hit
}

// globMatch matches rel against an op glob, ignoring a trailing slash.
func globMatch(glob, rel string) bool {
	return fsutil.MatchGlob(strings.TrimSuffix(glob, "/"), rel)
}

// effective is the composed additive selection before classification + exclude.
// Each op's globs carry their group's perms; classify stamps matched files
// with them (last match wins).
type effective struct {
	linkGlobs globSet    // link-op globs (repo-relative under root/)
	copyGlobs globSet    // copy-op globs
	tmplGlobs globSet    // template-op globs
	richCopy  []FileItem // rich-form copy entries
	richTmpl  []FileItem // rich-form template entries
	dirs      []FileItem // mkdirs: glob forms expanded to one item per path, rich carry perms
	scripts   []string   // script paths (order = run order)
	services  []string   // service names
	exclude   excludeSet // accumulated exclude globs (applied last, wins)
}

// Load parses che.yml: the `profiles:` enum plus every other top-level key as a
// defined block.
func Load(path string) (*Raw, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("spec not found: %s", path)
	}
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	s := &Raw{profiles: map[string]profileSpec{}}
	for key, node := range raw {
		if key == "profiles" {
			if err := node.Decode(&s.Profiles); err != nil {
				return nil, fmt.Errorf("parse profiles enum: %w", err)
			}
			continue
		}
		var ps profileSpec
		if err := node.Decode(&ps); err != nil {
			return nil, fmt.Errorf("parse profile %q: %w", key, err)
		}
		s.profiles[key] = ps
	}
	return s, nil
}

// Resolve validates the profile is defined, composes its mixin-profiles and
// includes, classifies git-tracked files, then applies excludes as a final glob
// filter. Output is repo-relative.
func (r *Raw) Resolve(profile, root string) (Resolved, error) {
	if !r.isDetectable(profile) {
		return Resolved{}, fmt.Errorf(
			"detected profile %q is not defined in che.yml (defined: %v)",
			profile, r.detectableLeaves())
	}
	var eff effective
	if err := r.mergeInto(&eff, profile, nil); err != nil {
		return Resolved{}, err
	}
	scripts, err := expandScripts(filepath.Dir(root), fsutil.ExpandAll(eff.scripts))
	if err != nil {
		return Resolved{}, err
	}
	res := Resolved{
		ExtraDirs: eff.dirs,
		Scripts:   scripts,
		Services:  fsutil.ExpandAll(eff.services),
		Copies:    eff.richCopy,
		Templates: eff.richTmpl,
	}
	if err := classify(root, eff, &res); err != nil {
		return Resolved{}, err
	}
	applyExcludes(eff.exclude, &res)
	return res, nil
}

// expandScripts resolves each repo-relative script entry to repo-relative file
// paths, IN SPEC ORDER (globs expand sorted in place). Resolving here, before
// applyExcludes, lets run-scripts excludes match real files (not glob strings).
// Each entry must resolve to >=1 file ([why] catches typos/renames).
func expandScripts(repoRoot string, entries []string) ([]string, error) {
	var out []string
	for _, entry := range entries {
		if strings.ContainsAny(entry, "*?[") {
			hits, err := filepath.Glob(filepath.Join(repoRoot, entry))
			if err != nil {
				return nil, err
			}
			if len(hits) == 0 {
				return nil, fmt.Errorf("run-scripts entry matched no script: %s", entry)
			}
			slices.Sort(hits)
			for _, h := range hits {
				rel, err := filepath.Rel(repoRoot, h)
				if err != nil {
					return nil, err
				}
				out = append(out, rel)
			}
			continue
		}
		if _, err := os.Stat(filepath.Join(repoRoot, entry)); err != nil {
			return nil, fmt.Errorf("run-scripts script not found: %s", entry)
		}
		out = append(out, entry)
	}
	return out, nil
}

// classify applies the glob-form ops to git-tracked files under root, bucketing
// them into Links/Copies/Templates plus ancestor Dirs. Glob copy/template files
// inherit the matching glob's perms.
func classify(root string, eff effective, res *Resolved) error {
	tracked, err := fsutil.TrackedFiles(root)
	if err != nil {
		return err
	}
	rich := richRels(eff) // rich entries win: skip their glob twins
	for _, rel := range tracked {
		if rich[rel] {
			continue
		}
		switch {
		case strings.HasSuffix(rel, TmplExt) && hit(eff.tmplGlobs, rel, &res.Templates):
		case strings.HasSuffix(rel, CpExt) && hit(eff.copyGlobs, rel, &res.Copies):
		case filepath.Base(rel) == ".gitkeep":
			// excluded from every op
		case hit(eff.linkGlobs, rel, &res.Links):
		}
	}
	collectDirs(res)
	return nil
}

// hit appends rel (with its matched perms) to items if any glob in gs matches,
// reporting whether it did.
func hit(gs globSet, rel string, items *[]FileItem) bool {
	perms, ok := gs.match(rel)
	if ok {
		*items = append(*items, FileItem{Rel: rel, Perms: perms})
	}
	return ok
}

// richRels is the set of source rels claimed by rich copy/template entries.
func richRels(eff effective) map[string]bool {
	m := map[string]bool{}
	for _, it := range eff.richCopy {
		m[it.Rel] = true
	}
	for _, it := range eff.richTmpl {
		m[it.Rel] = true
	}
	return m
}

// collectDirs derives every ancestor dir of the file items into res.Dirs.
func collectDirs(res *Resolved) {
	dirSeen := map[string]bool{}
	add := func(items []FileItem) {
		for _, it := range items {
			for d := filepath.Dir(it.Rel); d != "." && !dirSeen[d]; d = filepath.Dir(d) {
				dirSeen[d] = true
				res.Dirs = append(res.Dirs, d)
			}
		}
	}
	add(res.Links)
	add(res.Copies)
	add(res.Templates)
	slices.SortFunc(res.Links, byRel)
	slices.SortFunc(res.Copies, byRel)
	slices.SortFunc(res.Templates, byRel)
	slices.Sort(res.Dirs) // lexical, parents before children
}

func byRel(a, b FileItem) int { return cmp.Compare(a.Rel, b.Rel) }

func matchAny(globs []string, rel string) bool {
	return slices.ContainsFunc(globs, func(g string) bool { return globMatch(g, rel) })
}

// applyExcludes drops items matching any exclude glob across all keys. Excludes
// win over everything, including rich include entries.
func applyExcludes(ex excludeSet, res *Resolved) {
	link := fsutil.ExpandAll(ex.Link)
	copyG := fsutil.ExpandAll(ex.Copy)
	tmplG := fsutil.ExpandAll(ex.Template)
	dirG := fsutil.ExpandAll(ex.Mkdirs)
	instG := fsutil.ExpandAll(ex.Scripts)
	svcG := fsutil.ExpandAll(ex.Services)

	res.Links = dropFiles(res.Links, link)
	res.Copies = dropFiles(res.Copies, copyG)
	res.Templates = dropFiles(res.Templates, tmplG)
	res.ExtraDirs = dropFiles(res.ExtraDirs, dirG)
	res.Scripts = dropStrings(res.Scripts, instG)
	res.Services = dropStrings(res.Services, svcG)

	res.Dirs = nil
	collectDirs(res)
	res.Dirs = dropStrings(res.Dirs, dirG)
}

// dropFiles drops any FileItem whose rel or any dest matches an exclude glob.
func dropFiles(items []FileItem, globs []string) []FileItem {
	if len(globs) == 0 {
		return items
	}
	return slices.DeleteFunc(items, func(it FileItem) bool {
		if matchAny(globs, it.Rel) {
			return true
		}
		for _, d := range it.Dests {
			if matchAny(globs, d.Path) {
				return true
			}
		}
		return false
	})
}

// dropStrings drops any entry matching an exclude glob.
func dropStrings(xs, globs []string) []string {
	if len(globs) == 0 {
		return xs
	}
	return slices.DeleteFunc(xs, func(x string) bool { return matchAny(globs, x) })
}

// mergeInto composes name into eff: mixin-profiles depth-first, then this
// profile's include sections (additive). Excludes are handled separately
// (applyExcludes). seen catches cycles.
func (r *Raw) mergeInto(eff *effective, name string, seen []string) error {
	if slices.Contains(seen, name) {
		return fmt.Errorf("mixin-profiles cycle: %v -> %s", seen, name)
	}
	ps, ok := r.profiles[name]
	if !ok {
		return fmt.Errorf("mixin-profiles names undefined profile %q (from %v)", name, seen)
	}
	child := append(slices.Clone(seen), name)
	for _, m := range ps.MixinProfiles {
		if err := r.mergeInto(eff, m, child); err != nil {
			return err
		}
	}
	in := ps.Include
	for _, g := range in.Link {
		eff.linkGlobs.add(g, Perms{})
	}
	splitEntries(in.Copy, &eff.copyGlobs, &eff.richCopy)
	splitEntries(in.Template, &eff.tmplGlobs, &eff.richTmpl)
	for _, e := range in.Mkdirs {
		eff.dirs = append(eff.dirs, dirItems(e)...)
	}
	eff.scripts = append(eff.scripts, in.Scripts...)
	eff.services = append(eff.services, in.Services...)
	eff.exclude.append(ps.Exclude)
	return nil
}

// append accumulates another profile's exclude globs (composition order).
func (ex *excludeSet) append(o excludeSet) {
	ex.Link = append(ex.Link, o.Link...)
	ex.Copy = append(ex.Copy, o.Copy...)
	ex.Template = append(ex.Template, o.Template...)
	ex.Mkdirs = append(ex.Mkdirs, o.Mkdirs...)
	ex.Scripts = append(ex.Scripts, o.Scripts...)
	ex.Services = append(ex.Services, o.Services...)
}

// splitEntries walks each perm-group's Files: glob items go to globs carrying
// the group's perms, {source,dest} items become rich FileItems carrying them.
func splitEntries(entries []entry, globs *globSet, rich *[]FileItem) {
	for _, e := range entries {
		for _, f := range e.Files {
			if f.glob != "" {
				globs.add(f.glob, e.Perms)
				continue
			}
			*rich = append(*rich, FileItem{Rel: f.Source, Dests: f.Dest, Perms: e.Perms})
		}
	}
}

// dirItems expands each mkdirs perm-group item into one FileItem per
// brace-expanded dest path, carrying the group's perms (path in Dests[0]).
func dirItems(e entry) []FileItem {
	var out []FileItem
	for _, f := range e.Files {
		paths := f.Dest
		if f.glob != "" {
			paths = []DestSpec{{Path: f.glob}}
		}
		for _, d := range paths {
			for _, p := range fsutil.ExpandBraces(d.Path) {
				out = append(out, FileItem{Dests: []DestSpec{{Path: p}}, Perms: e.Perms})
			}
		}
	}
	return out
}

// isDetectable reports whether profile is both declared in the enum and defined.
func (r *Raw) isDetectable(profile string) bool {
	_, defined := r.profiles[profile]
	return defined && r.declared(profile)
}

// declared walks the profiles enum tree for the "<space>/<os>" leaf.
func (r *Raw) declared(profile string) bool {
	space, leaf, ok := strings.Cut(profile, "/")
	if !ok {
		return false
	}
	m, ok := r.Profiles[space].(map[string]any)
	if !ok {
		return false
	}
	_, ok = m[leaf]
	return ok
}

// detectableLeaves lists the enum leaves that are also defined.
func (r *Raw) detectableLeaves() []string {
	var out []string
	for space, leaves := range r.Profiles {
		m, ok := leaves.(map[string]any)
		if !ok {
			continue
		}
		for leaf := range m {
			name := space + "/" + leaf
			if _, defined := r.profiles[name]; defined {
				out = append(out, name)
			}
		}
	}
	return slices.Sorted(slices.Values(out))
}

// [<] 🤖🤖
