package spec

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

// globSet is an ordered list of op globs, each carrying its group's perms
// (zero Perms if none) and an optional link dest rule. Globs are
// brace-expanded on add.
type globSet []globPerm

type globPerm struct {
	glob  string
	perms Perms
	rule  *destRule
}

func (gs *globSet) add(glob string, perms Perms) { gs.addRule(glob, perms, nil) }

func (gs *globSet) addRule(glob string, perms Perms, rule *destRule) {
	for _, g := range fsutil.ExpandBraces(glob) {
		*gs = append(*gs, globPerm{glob: g, perms: perms, rule: rule})
	}
}

// match: last match wins.
func (gs globSet) match(rel string) (globPerm, bool) {
	for _, g := range slices.Backward(gs) {
		if isGlobMatch(g.glob, rel) {
			return g, true
		}
	}
	return globPerm{}, false
}

// isGlobMatch matches rel against an op glob, ignoring a trailing slash.
func isGlobMatch(glob, rel string) bool {
	return fsutil.IsGlobMatch(strings.TrimSuffix(glob, "/"), rel)
}

// effective is the composed additive selection before classification + exclude.
// Each op's globs carry their group's perms; classify stamps matched files
// with them (last match wins).
type effective struct {
	linkGlobs globSet     // link-op globs (repo-relative under root/)
	copyGlobs globSet     // copy-op globs
	tmplGlobs globSet     // render-templates globs (repo-root-relative, root/-prefixed)
	richCopy  []FileItem  // rich-form copy entries
	richTmpl  []FileItem  // rich-form render-templates entries (repo-root-relative)
	dirs      []FileItem  // mkdirs: glob forms expanded to one item per path, rich carry perms
	scripts   []string    // script paths (order = run order)
	services  []string    // service names
	plugins   []PluginRef // profile-level plugin refs (composition order)
	exclude   excludeSet  // accumulated exclude globs (applied last, wins)
}

// EligibleProfiles lists the profiles to Resolve, in declaration order:
//  1. forceOne (--profile by name): only that profile, autoDiscover skipped,
//     execIf still enforced (forceAll = --skip-exec-if lifts it).
//  2. else every autoDiscover profile whose execIf expressions ALL pass
//     (forceAll makes every execIf pass, it does not lift autoDiscover).
//  3. zero eligible: error.
func (r *CheSpec) EligibleProfiles(forceOne string, forceAll bool, eval func(expr string) (bool, error)) ([]string, error) {
	if forceOne != "" {
		ps, ok := r.profile(forceOne)
		if !ok {
			return nil, r.undefinedProfile(fmt.Sprintf("--profile %q", forceOne))
		}
		pass, err := allPass(forceOne, ps.Options.ExecIf, forceAll, eval)
		if err != nil {
			return nil, err
		}
		if !pass {
			return nil, fmt.Errorf("--profile %q failed its execIf predicates (pass --skip-exec-if to run it regardless)", forceOne)
		}
		return []string{forceOne}, nil
	}
	var out []string
	for _, ps := range r.profiles {
		if !ps.Options.AutoDiscover {
			continue
		}
		ok, err := allPass(ps.Name, ps.Options.ExecIf, forceAll, eval)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, ps.Name)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no eligible profile: no autoDiscover profile passed its execIf (candidates: %v; use --profile or CHE_SKIP_EXEC_IF)",
			r.names(func(ps Profile) bool { return ps.Options.AutoDiscover }))
	}
	return out, nil
}

// ExecIfPass: the named profile's execIf expressions all pass.
func (r *CheSpec) ExecIfPass(name string, forceAll bool, eval func(expr string) (bool, error)) (bool, error) {
	ps, ok := r.profile(name)
	if !ok {
		return false, r.undefinedProfile(fmt.Sprintf("profile %q", name))
	}
	return allPass(name, ps.Options.ExecIf, forceAll, eval)
}

// profile returns the named Profile.
func (r *CheSpec) profile(name string) (Profile, bool) {
	for _, ps := range r.profiles {
		if ps.Name == name {
			return ps, true
		}
	}
	return Profile{}, false
}

func (r *CheSpec) undefinedProfile(ref string) error {
	return fmt.Errorf("%s is not defined in che.yml (defined: %v)", ref, r.names(func(Profile) bool { return true }))
}

// allPass logs each pass, rejects at debug level only.
func allPass(name string, exprs []string, forceAll bool, eval func(expr string) (bool, error)) (bool, error) {
	if forceAll {
		return true, nil
	}
	for _, expr := range exprs {
		ok, err := eval(expr)
		if err != nil {
			return false, fmt.Errorf("profile %q execIf %q: %w", name, expr, err)
		}
		if !ok {
			log.Debug("execIf(reject)", fmt.Sprintf("profile %s: %s", name, expr), log.Off)
			return false, nil
		}
		log.Msg("execIf(pass)", fmt.Sprintf("profile %s: %s", name, expr), log.Off)
	}
	return true, nil
}

func (r *CheSpec) names(keep func(Profile) bool) []string {
	var out []string
	for _, ps := range r.profiles {
		if keep(ps) {
			out = append(out, ps.Name)
		}
	}
	return slices.Sorted(slices.Values(out))
}

// Resolve validates each profile is defined, composes their mixinProfiles and
// includes into one union (in order), classifies git-tracked files, then
// applies excludes as a final glob filter. Output is repo-relative.
func (r *CheSpec) Resolve(profiles []string, root string) (Resolved, error) {
	var eff effective
	for _, profile := range profiles {
		if _, ok := r.profile(profile); !ok {
			return Resolved{}, r.undefinedProfile(fmt.Sprintf("profile %q", profile))
		}
		if err := r.mergeInto(&eff, profile, nil); err != nil {
			return Resolved{}, err
		}
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
		Plugins:   eff.plugins,
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
// inherit the matching glob's perms. Template globs are repo-root-relative
// (root/-prefixed), so tracked rels match with the root/ prefix restored.
func classify(root string, eff effective, res *Resolved) error {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil // no root/ subtree: repo-only project (rich template selection only)
	}
	tracked, err := fsutil.TrackedFiles(root)
	if err != nil {
		return err
	}
	rich := richRels(eff) // rich entries win: skip their glob twins
	for _, rel := range tracked {
		if rich[rel] || rich[RootPrefix+rel] {
			continue
		}
		switch {
		case IsTmplSrc(rel) && hit(eff.tmplGlobs, RootPrefix+rel, &res.Templates):
		case strings.HasSuffix(rel, CpExt) && hit(eff.copyGlobs, rel, &res.Copies):
		case filepath.Base(rel) == ".gitkeep":
			// excluded from every op
		case hit(eff.linkGlobs, rel, &res.Links):
		}
	}
	collectDirs(res)
	return nil
}

// hit: a matched link dest rule that changes rel lands as the item's explicit dest.
func hit(gs globSet, rel string, items *[]FileItem) bool {
	gp, ok := gs.match(rel)
	if !ok {
		return false
	}
	it := FileItem{Rel: rel, Perms: gp.perms}
	if gp.rule != nil {
		if dest := gp.rule.apply(rel); dest != rel {
			it.Dests = []DestSpec{{Path: dest}}
		}
	}
	*items = append(*items, it)
	return true
}

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
// Links contribute their dest rel ([why] rewritten host dirs must exist).
// Templates contribute only derived-dest (glob-form) items, root/ prefix
// stripped ([why] rich dests need no pre-created host dirs).
func collectDirs(res *Resolved) {
	dirSeen := map[string]bool{}
	addRel := func(rel string) {
		for d := filepath.Dir(rel); d != "." && !dirSeen[d]; d = filepath.Dir(d) {
			dirSeen[d] = true
			res.Dirs = append(res.Dirs, d)
		}
	}
	for _, it := range res.Links {
		addRel(LinkDestRel(it))
	}
	add := func(items []FileItem) {
		for _, it := range items {
			addRel(it.Rel)
		}
	}
	add(res.Copies)
	for _, it := range res.Templates {
		if len(it.Dests) == 0 {
			addRel(strings.TrimPrefix(it.Rel, RootPrefix))
		}
	}
	slices.SortFunc(res.Links, byRel)
	slices.SortFunc(res.Copies, byRel)
	slices.SortFunc(res.Templates, byRel)
	slices.Sort(res.Dirs) // lexical, parents before children
}

func byRel(a, b FileItem) int { return cmp.Compare(a.Rel, b.Rel) }

func isAnyGlobMatch(globs []string, rel string) bool {
	return slices.ContainsFunc(globs, func(g string) bool { return isGlobMatch(g, rel) })
}

// applyExcludes drops items matching any exclude glob across all keys. Excludes
// win over everything, including rich include entries.
func applyExcludes(ex excludeSet, res *Resolved) {
	link := fsutil.ExpandAll(ex.Link)
	copyG := fsutil.ExpandAll(ex.Copy)
	tmplG := fsutil.ExpandAll(ex.RenderTemplates)
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

func dropFiles(items []FileItem, globs []string) []FileItem {
	if len(globs) == 0 {
		return items
	}
	return slices.DeleteFunc(items, func(it FileItem) bool {
		if isAnyGlobMatch(globs, it.Rel) {
			return true
		}
		for _, d := range it.Dests {
			if isAnyGlobMatch(globs, d.Path) {
				return true
			}
		}
		return false
	})
}

func dropStrings(xs, globs []string) []string {
	if len(globs) == 0 {
		return xs
	}
	return slices.DeleteFunc(xs, func(x string) bool { return isAnyGlobMatch(globs, x) })
}

// mergeInto composes name into eff: mixinProfiles depth-first, then this
// profile's include sections (additive). Excludes are handled separately
// (applyExcludes). seen catches cycles.
func (r *CheSpec) mergeInto(eff *effective, name string, seen []string) error {
	if slices.Contains(seen, name) {
		return fmt.Errorf("mixinProfiles cycle: %v -> %s", seen, name)
	}
	ps, ok := r.profile(name)
	if !ok {
		return fmt.Errorf("mixinProfiles names undefined profile %q (from %v)", name, seen)
	}
	child := append(slices.Clone(seen), name)
	for _, m := range ps.MixinProfiles {
		if err := r.mergeInto(eff, m, child); err != nil {
			return err
		}
	}
	for _, pe := range ps.Plugins {
		ref, err := parsePluginRef(pe.Ref)
		if err != nil {
			return fmt.Errorf("profile %q: %w", name, err)
		}
		ref.Env = pe.Env
		dup := slices.ContainsFunc(eff.plugins, func(q PluginRef) bool {
			return q.URL == ref.URL && q.Profile == ref.Profile && q.IsPath == ref.IsPath
		})
		if !dup {
			eff.plugins = append(eff.plugins, ref)
		}
	}
	in := ps.Include
	for _, e := range in.Link {
		if e.glob != "" {
			eff.linkGlobs.add(e.glob, Perms{})
			continue
		}
		if e.Source == "" {
			return fmt.Errorf("profile %q: link entry missing source", name)
		}
		rule, err := parseDestRule(e.Dest)
		if err != nil {
			return fmt.Errorf("profile %q: %w", name, err)
		}
		eff.linkGlobs.addRule(e.Source, Perms{}, rule)
	}
	splitEntries(in.Copy, &eff.copyGlobs, &eff.richCopy)
	if err := splitTemplates(in.RenderTemplates, &eff.tmplGlobs, &eff.richTmpl); err != nil {
		return err
	}
	for _, e := range in.Mkdirs {
		eff.dirs = append(eff.dirs, dirItems(e)...)
	}
	eff.scripts = append(eff.scripts, in.Scripts...)
	eff.services = append(eff.services, in.Services...)
	eff.exclude.append(ps.Exclude)
	return nil
}

func (ex *excludeSet) append(o excludeSet) {
	ex.Link = append(ex.Link, o.Link...)
	ex.Copy = append(ex.Copy, o.Copy...)
	ex.RenderTemplates = append(ex.RenderTemplates, o.RenderTemplates...)
	ex.Mkdirs = append(ex.Mkdirs, o.Mkdirs...)
	ex.Scripts = append(ex.Scripts, o.Scripts...)
	ex.Services = append(ex.Services, o.Services...)
}

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

// splitTemplates: glob items go to globs, {source, dest} items become rich
// FileItems. Remote refs (@<repo>//<path>) require an explicit dest, the
// derived-dest form (glob, or rich without dest) a root/-prefixed source
// ([why] only root/ paths map to host dests).
func splitTemplates(entries []entry, globs *globSet, rich *[]FileItem) error {
	for _, e := range entries {
		for _, f := range e.Files {
			if f.glob != "" {
				if IsRemoteSrc(f.glob) {
					return fmt.Errorf("renderTemplates glob cannot be remote: %q", f.glob)
				}
				if !strings.HasPrefix(f.glob, RootPrefix) {
					return fmt.Errorf("renderTemplates glob must be root/-prefixed (derived host dest): %q", f.glob)
				}
				globs.add(f.glob, e.Perms)
				continue
			}
			if IsRemoteSrc(f.Source) {
				if !render.IsRemoteRef(RemoteSrcRef(f.Source)) {
					return fmt.Errorf("renderTemplates remote source malformed, want @<repo>//<path>[?ref=<ref>]: %q", f.Source)
				}
				if len(f.Dest) == 0 {
					return fmt.Errorf("renderTemplates remote source requires explicit dest: %q", f.Source)
				}
			}
			if len(f.Dest) == 0 && !strings.HasPrefix(f.Source, RootPrefix) {
				return fmt.Errorf("renderTemplates source without dest must be root/-prefixed (derived host dest): %q", f.Source)
			}
			*rich = append(*rich, FileItem{Rel: f.Source, Dests: mergeDestOptions(e.Options, f.Dest), Ctx: mergeCtx(e.Ctx, f.Ctx), Perms: e.Perms})
		}
	}
	return nil
}

// mergeCtx merges a group-level ctx under an item's: item keys win.
func mergeCtx(group, item map[string]string) map[string]string {
	if len(group) == 0 {
		return item
	}
	out := maps.Clone(group)
	maps.Copy(out, item)
	return out
}

// mergeDestOptions merges group-level render options under each dest's own:
// fields the dest sets win, unset fields inherit the group's.
func mergeDestOptions(group render.Options, dests []DestSpec) []DestSpec {
	if group == (render.Options{}) {
		return dests
	}
	out := slices.Clone(dests)
	for i := range out {
		out[i].Options = out[i].opts.over(group)
	}
	return out
}

// dirItems expands each mkdirs perm-group item into one FileItem per
// brace-expanded dest path, carrying the group's perms (path in Dests[0]).
func dirItems(e dirGroup) []FileItem {
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

// [<] 🤖🤖
