package spec

// [>] 🤖🤖

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

func (gs *globSet) add(glob string, perms Perms) { gs.addRule(glob, perms, nil) }

func (gs *globSet) addRule(glob string, perms Perms, rule *destRule) {
	for _, g := range fsutil.ExpandBraces(glob) {
		*gs = append(*gs, globPerm{glob: g, perms: perms, rule: rule})
	}
}

func (gs globSet) match(rel string) (globPerm, bool) {
	for _, g := range slices.Backward(gs) {
		if isGlobMatch(g.glob, rel) {
			return g, true
		}
	}
	return globPerm{}, false
}

func isGlobMatch(glob, rel string) bool {
	return fsutil.IsGlobMatch(strings.TrimSuffix(glob, "/"), rel)
}

var ErrNoneEligible = errors.New("no eligible profile")

type Rejection struct {
	Ref  string
	Cond string
}

func EligibleRecipes(recipes []ProfileRecipe, forced []string, forceAll bool, eval func(expr string) (bool, error)) (eligible []string, rejected []Rejection, err error) {
	if len(forced) > 0 {
		out := make([]string, 0, len(forced))
		for _, name := range forced {
			ps, ok := findRecipe(recipes, name)
			if !ok {
				return nil, nil, undefinedProfile(recipes, fmt.Sprintf("--profiles %q", name))
			}
			pass, _, err := AllPass(name, ps.Options.RunIf, forceAll, eval)
			if err != nil {
				return nil, nil, err
			}
			if !pass {
				return nil, nil, fmt.Errorf("--profiles %q failed its runIf predicates (pass --skip-run-if to run it regardless)", name)
			}
			out = append(out, name)
		}
		return out, nil, nil
	}
	var out []string
	for _, ps := range recipes {
		if ps.Options.AutoDiscover == nil || !*ps.Options.AutoDiscover {
			continue
		}
		name := ps.Source.GetProfileName()
		ok, failed, err := AllPass(name, ps.Options.RunIf, forceAll, eval)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			out = append(out, name)
		} else {
			rejected = append(rejected, Rejection{Ref: name, Cond: failed})
		}
	}
	if len(out) == 0 {
		return nil, rejected, fmt.Errorf("%w: no autoDiscover profile passed its runIf (candidates: %v; use --profiles or CHE_SKIP_RUN_IF)",
			ErrNoneEligible,
			names(recipes, func(ps ProfileRecipe) bool { return ps.Options.AutoDiscover != nil && *ps.Options.AutoDiscover }))
	}
	return out, rejected, nil
}

func FindRecipe(recipes []ProfileRecipe, name string) (ProfileRecipe, error) {
	ps, ok := findRecipe(recipes, name)
	if !ok {
		return ProfileRecipe{}, undefinedProfile(recipes, fmt.Sprintf("profile %q", name))
	}
	return ps, nil
}

func findRecipe(recipes []ProfileRecipe, name string) (ProfileRecipe, bool) {
	for _, ps := range recipes {
		if ps.Source.GetProfileName() == name {
			return ps, true
		}
	}
	return ProfileRecipe{}, false
}

func undefinedProfile(recipes []ProfileRecipe, ref string) error {
	return fmt.Errorf("%s is not defined in che.yml (defined: %v)", ref, names(recipes, func(ProfileRecipe) bool { return true }))
}

func AllPass(name string, exprs []string, forceAll bool, eval func(expr string) (bool, error)) (pass bool, failed string, err error) {
	if forceAll {
		return true, "", nil
	}
	for _, expr := range exprs {
		ok, err := eval(expr)
		if err != nil {
			return false, expr, fmt.Errorf("profile %q runIf %q: %w", name, expr, err)
		}
		verdict := "passed"
		if !ok {
			verdict = "failed"
		}
		log.Emit(log.Event{
			Level: log.Levels.Trace, Scope: "discover-profiles", Action: "evaluated-run-if",
			Msg:   fmt.Sprintf("%s: %q %s", name, expr, verdict),
			Attrs: map[string]string{"profile": name, "condition": expr, "verdict": verdict},
		})
		if !ok {
			return false, expr, nil
		}
	}
	return true, "", nil
}

func names(recipes []ProfileRecipe, keep func(ProfileRecipe) bool) []string {
	var out []string
	for _, ps := range recipes {
		if keep(ps) {
			out = append(out, ps.Source.GetProfileName())
		}
	}
	return slices.Sorted(slices.Values(out))
}

func (r ProfileRecipe) MakeProfile(recipes []ProfileRecipe, workingDir string) (OperationRecipes, []ProfileSourceRecipe, error) {
	var eff effective
	if err := mergeRecipe(recipes, &eff, r, nil); err != nil {
		return OperationRecipes{}, nil, err
	}
	repoRoot := r.Source.DirectoryPath
	root := workingDir
	scripts, err := expandScripts(repoRoot, fsutil.ExpandAll(eff.scripts))
	if err != nil {
		return OperationRecipes{}, nil, err
	}
	res := resolved{
		ExtraDirs: eff.dirs,
		Packages:  dedupe(eff.packages),
		Scripts:   scripts,
		Links:     eff.richLink,
		Copies:    eff.richCopy,
		Templates: eff.richTmpl,
	}
	if err := classify(root, eff, &res); err != nil {
		return OperationRecipes{}, nil, err
	}
	applyExcludes(eff.exclude, &res)
	return res.operationRecipes(), eff.refs, nil
}

func (res resolved) operationRecipes() OperationRecipes {
	dirs := make([]FileItem, 0, len(res.Dirs)+len(res.ExtraDirs))
	for _, d := range res.Dirs {
		dirs = append(dirs, FileItem{Rel: d})
	}
	dirs = append(dirs, res.ExtraDirs...)
	return OperationRecipes{
		PruneLinks:      PruneLinksOperationRecipe{Dirs: res.Dirs},
		MakeDirs:        MakeDirsOperationRecipe{Dirs: dirs},
		MakeLinks:       MakeLinksOperationRecipe{Links: res.Links, Dirs: res.Dirs},
		MakeCopies:      MakeCopiesOperationRecipe{Copies: res.Copies, Dirs: res.Dirs},
		RenderTemplates: RenderTemplatesOperationRecipe{Templates: res.Templates},
		InstallPackages: InstallPackagesOperationRecipe{Packages: res.Packages},
		RunScripts:      RunScriptsOperationRecipe{Scripts: res.Scripts},
	}
}

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

func classify(root string, eff effective, res *resolved) error {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}
	tracked, err := fsutil.ListTrackedFiles(root)
	if err != nil {
		return err
	}
	rich := richRels(eff)
	for _, rel := range tracked {
		if rich[rel] {
			continue
		}
		switch {
		case IsTmplSrc(rel) && hit(eff.tmplGlobs, rel, TrimTmplExt, &res.Templates):
		case strings.HasSuffix(rel, CpExt) && hit(eff.copyGlobs, rel, trimCpExt, &res.Copies):
		case filepath.Base(rel) == ".gitkeep":
		case hit(eff.linkGlobs, rel, identity, &res.Links):
		}
	}
	collectDirs(res)
	return nil
}

func identity(rel string) string  { return rel }
func trimCpExt(rel string) string { return strings.TrimSuffix(rel, CpExt) }

func hit(gs globSet, rel string, destBase func(string) string, items *[]FileItem) bool {
	gp, ok := gs.match(rel)
	if !ok {
		return false
	}
	it := FileItem{Rel: rel, Perms: gp.perms}
	if gp.rule != nil {
		base := destBase(rel)
		if dest := gp.rule.apply(base); dest != base {
			it.Dests = []DestSpec{{Path: dest}}
			it.Derived = true
		}
	}
	*items = append(*items, it)
	return true
}

func richRels(eff effective) map[string]bool {
	m := map[string]bool{}
	for _, it := range eff.richLink {
		m[it.Rel] = true
	}
	for _, it := range eff.richCopy {
		m[it.Rel] = true
	}
	for _, it := range eff.richTmpl {
		m[it.Rel] = true
	}
	return m
}

func collectDirs(res *resolved) {
	dirSeen := map[string]bool{}
	addRel := func(rel string) {
		for d := filepath.Dir(rel); d != "." && d != "/" && !dirSeen[d]; d = filepath.Dir(d) {
			dirSeen[d] = true
			res.Dirs = append(res.Dirs, d)
		}
	}
	for _, it := range res.Links {
		addRel(DestRel(it))
	}
	for _, it := range res.Copies {
		addRel(DestRel(it))
	}
	for _, it := range res.Templates {
		if len(it.Dests) == 0 {
			addRel(it.Rel)
		} else if it.Derived {
			addRel(it.Dests[0].Path)
		}
	}
	slices.SortFunc(res.Links, byRel)
	slices.SortFunc(res.Copies, byRel)
	slices.SortFunc(res.Templates, byRel)
	slices.Sort(res.Dirs)
}

func byRel(a, b FileItem) int { return cmp.Compare(a.Rel, b.Rel) }

func dedupe(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func isAnyGlobMatch(globs []string, rel string) bool {
	return slices.ContainsFunc(globs, func(g string) bool { return isGlobMatch(g, rel) })
}

func applyExcludes(ex excludeSet, res *resolved) {
	link := fsutil.ExpandAll(ex.MakeLinks)
	copyG := fsutil.ExpandAll(ex.MakeCopies)
	tmplG := fsutil.ExpandAll(ex.RenderTemplates)
	dirG := fsutil.ExpandAll(ex.MakeDirs)
	pkgG := fsutil.ExpandAll(ex.InstallPackages)
	instG := fsutil.ExpandAll(ex.Scripts)

	res.Links = dropFiles(res.Links, link)
	res.Copies = dropFiles(res.Copies, copyG)
	res.Templates = dropFiles(res.Templates, tmplG)
	res.ExtraDirs = dropFiles(res.ExtraDirs, dirG)
	res.Packages = dropStrings(res.Packages, pkgG)
	res.Scripts = dropStrings(res.Scripts, instG)

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

func mergeRecipe(recipes []ProfileRecipe, eff *effective, ps ProfileRecipe, seen []string) error {
	name := ps.Source.GetProfileName()
	if slices.Contains(seen, name) {
		return fmt.Errorf("include.profiles cycle: %v -> %s", seen, name)
	}
	child := append(slices.Clone(seen), name)
	for _, ref := range ps.Include.Profiles {
		if ref.URI != "" {
			dup := slices.ContainsFunc(eff.refs, func(q ProfileSourceRecipe) bool {
				return q.URI == ref.URI && q.ProfileName == ref.ProfileName
			})
			if !dup {
				eff.refs = append(eff.refs, ref)
			}
			continue
		}
		m, ok := findRecipe(recipes, ref.ProfileName)
		if !ok {
			return fmt.Errorf("include.profiles names undefined profile %q (from %v)", ref.ProfileName, child)
		}
		if err := mergeRecipe(recipes, eff, m, child); err != nil {
			return err
		}
	}
	in := ps.Include
	for _, e := range in.MakeLinks {
		switch {
		case e.glob != "":
			eff.linkGlobs.add(e.glob, Perms{})
		case e.Source == "":
			return fmt.Errorf("profile %q: link entry missing source", name)
		case e.DestRule != "":
			rule, err := ruleFromDest(e.Source, e.DestRule)
			if err != nil {
				return fmt.Errorf("profile %q: %w", name, err)
			}
			eff.linkGlobs.addRule(e.Source, Perms{}, rule)
		case len(e.Dest) == 0:
			return fmt.Errorf("profile %q: link entry %q missing dest", name, e.Source)
		default:
			eff.richLink = append(eff.richLink, FileItem{Rel: e.Source, Dests: e.Dest})
		}
	}
	if err := splitEntries(in.MakeCopies, &eff.copyGlobs, &eff.richCopy); err != nil {
		return fmt.Errorf("profile %q: %w", name, err)
	}
	if err := splitTemplates(in.RenderTemplates, &eff.tmplGlobs, &eff.richTmpl); err != nil {
		return fmt.Errorf("profile %q: %w", name, err)
	}
	for _, e := range in.MakeDirs {
		eff.dirs = append(eff.dirs, dirItems(e)...)
	}
	eff.packages = append(eff.packages, in.InstallPackages...)
	eff.scripts = append(eff.scripts, in.Scripts...)
	eff.exclude.append(ps.Exclude)
	return nil
}

func (ex *excludeSet) append(o excludeSet) {
	ex.MakeLinks = append(ex.MakeLinks, o.MakeLinks...)
	ex.MakeCopies = append(ex.MakeCopies, o.MakeCopies...)
	ex.RenderTemplates = append(ex.RenderTemplates, o.RenderTemplates...)
	ex.MakeDirs = append(ex.MakeDirs, o.MakeDirs...)
	ex.InstallPackages = append(ex.InstallPackages, o.InstallPackages...)
	ex.Scripts = append(ex.Scripts, o.Scripts...)
}

func splitEntries(entries []entry, globs *globSet, rich *[]FileItem) error {
	for _, e := range entries {
		for _, f := range e.Files {
			if f.glob != "" {
				globs.add(f.glob, e.Perms)
				continue
			}
			if f.DestRule != "" {
				rule, err := ruleFromDest(f.Source, f.DestRule)
				if err != nil {
					return err
				}
				globs.addRule(f.Source, e.Perms, rule)
				continue
			}
			*rich = append(*rich, FileItem{Rel: f.Source, Dests: f.Dest, Perms: e.Perms})
		}
	}
	return nil
}

func splitTemplates(entries []templateGroup, globs *globSet, rich *[]FileItem) error {
	for _, e := range entries {
		for _, f := range e.Files {
			if f.glob != "" {
				if IsRemoteSrc(f.glob) {
					return fmt.Errorf("renderTemplates glob cannot be remote: %q", f.glob)
				}
				globs.add(f.glob, e.Perms)
				continue
			}
			if f.DestRule != "" {
				if IsRemoteSrc(f.Source) {
					return fmt.Errorf("renderTemplates dest rewrite cannot be remote: %q", f.Source)
				}
				rule, err := ruleFromDest(f.Source, f.DestRule)
				if err != nil {
					return err
				}
				globs.addRule(f.Source, e.Perms, rule)
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
			*rich = append(*rich, FileItem{Rel: f.Source, Dests: mergeDestOptions(e.Options, f.Dest), Ctx: fsutil.MergeMap(e.Ctx, f.Ctx), Perms: e.Perms})
		}
	}
	return nil
}

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

func (o ProfileOptions) Over(spec Options) ProfileOptions {
	if o.AutoDiscover == nil {
		o.AutoDiscover = spec.AutoDiscover
	}
	if o.LogLevel == "" {
		o.LogLevel = spec.LogLevel
	}
	if o.ProfileWorkingDirectory == "" {
		o.ProfileWorkingDirectory = spec.ProfileWorkingDirectory
	}
	if o.Packages.File == "" {
		o.Packages.File = spec.Packages.File
	}
	if len(o.Packages.PreferredInstallationMethods) == 0 {
		o.Packages.PreferredInstallationMethods = spec.Packages.PreferredInstallationMethods
	}
	if len(o.Packages.PrebuiltArchive.InstallDestinationCandidates) == 0 {
		o.Packages.PrebuiltArchive.InstallDestinationCandidates = spec.Packages.PrebuiltArchive.InstallDestinationCandidates
	}
	if o.Packages.Completions.Zsh.Enabled == nil {
		o.Packages.Completions.Zsh.Enabled = spec.Packages.Completions.Zsh.Enabled
	}
	if len(o.Packages.Completions.Zsh.InstallDestinationCandidates) == 0 {
		o.Packages.Completions.Zsh.InstallDestinationCandidates = spec.Packages.Completions.Zsh.InstallDestinationCandidates
	}
	if o.Packages.Completions.Zsh.CheckInFpath == nil {
		o.Packages.Completions.Zsh.CheckInFpath = spec.Packages.Completions.Zsh.CheckInFpath
	}
	if o.Packages.PrebuiltArchive.CheckInPath == nil {
		o.Packages.PrebuiltArchive.CheckInPath = spec.Packages.PrebuiltArchive.CheckInPath
	}
	return o
}

func (o ProfileOptions) OverRef(entry ProfileOptions) ProfileOptions {
	if entry.RunIf != nil {
		o.RunIf = entry.RunIf
	}
	if entry.AutoDiscover != nil {
		o.AutoDiscover = entry.AutoDiscover
	}
	if entry.LogLevel != "" {
		o.LogLevel = entry.LogLevel
	}
	if entry.ProfileWorkingDirectory != "" {
		o.ProfileWorkingDirectory = entry.ProfileWorkingDirectory
	}
	if entry.Packages.File != "" {
		o.Packages.File = entry.Packages.File
	}
	if len(entry.Packages.PreferredInstallationMethods) > 0 {
		o.Packages.PreferredInstallationMethods = entry.Packages.PreferredInstallationMethods
	}
	if len(entry.Packages.PrebuiltArchive.InstallDestinationCandidates) > 0 {
		o.Packages.PrebuiltArchive.InstallDestinationCandidates = entry.Packages.PrebuiltArchive.InstallDestinationCandidates
	}
	if entry.Packages.Completions.Zsh.Enabled != nil {
		o.Packages.Completions.Zsh.Enabled = entry.Packages.Completions.Zsh.Enabled
	}
	if len(entry.Packages.Completions.Zsh.InstallDestinationCandidates) > 0 {
		o.Packages.Completions.Zsh.InstallDestinationCandidates = entry.Packages.Completions.Zsh.InstallDestinationCandidates
	}
	if entry.Packages.Completions.Zsh.CheckInFpath != nil {
		o.Packages.Completions.Zsh.CheckInFpath = entry.Packages.Completions.Zsh.CheckInFpath
	}
	if entry.Packages.PrebuiltArchive.CheckInPath != nil {
		o.Packages.PrebuiltArchive.CheckInPath = entry.Packages.PrebuiltArchive.CheckInPath
	}
	return o
}

// [<] 🤖🤖
