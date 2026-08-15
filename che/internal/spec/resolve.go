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

var ErrNoneEligible = errors.New("no eligible profile")

type Rejection struct {
	Ref  string
	Cond string
}

func EligibleRecipes(recipes []ProfileRecipe, forced []string, forceAll bool, eval func(expr string) (bool, error)) (eligible []string, rejected []Rejection, err error) {
	if len(forced) > 0 {
		out := make([]string, 0, len(forced))
		for _, name := range forced {
			rec, ok := findRecipe(recipes, name)
			if !ok {
				return nil, nil, undefinedProfile(recipes, fmt.Sprintf("--profiles %q", name))
			}
			pass, _, err := AllPass(name, rec.Options.RunIf, forceAll, eval)
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
	for _, rec := range recipes {
		if rec.Options.AutoDiscover == nil || !*rec.Options.AutoDiscover {
			continue
		}
		name := rec.Source.GetProfileName()
		ok, failed, err := AllPass(name, rec.Options.RunIf, forceAll, eval)
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
			listProfileNames(recipes, func(rec ProfileRecipe) bool { return rec.Options.AutoDiscover != nil && *rec.Options.AutoDiscover }))
	}
	return out, rejected, nil
}

func FindRecipe(recipes []ProfileRecipe, name string) (ProfileRecipe, error) {
	rec, ok := findRecipe(recipes, name)
	if !ok {
		return ProfileRecipe{}, undefinedProfile(recipes, fmt.Sprintf("profile %q", name))
	}
	return rec, nil
}

func findRecipe(recipes []ProfileRecipe, name string) (ProfileRecipe, bool) {
	for _, rec := range recipes {
		if rec.Source.GetProfileName() == name {
			return rec, true
		}
	}
	return ProfileRecipe{}, false
}

func undefinedProfile(recipes []ProfileRecipe, ref string) error {
	return fmt.Errorf("%s is not defined in che.yml (defined: %v)", ref, listProfileNames(recipes, func(ProfileRecipe) bool { return true }))
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

func listProfileNames(recipes []ProfileRecipe, keep func(ProfileRecipe) bool) []string {
	var out []string
	for _, rec := range recipes {
		if keep(rec) {
			out = append(out, rec.Source.GetProfileName())
		}
	}
	return slices.Sorted(slices.Values(out))
}

func (r ProfileRecipe) MakeProfile(recipes []ProfileRecipe, workingDir string) (OperationRecipes, []ProfileSourceRecipe, error) {
	var merged mergedInclude
	if err := mergeRecipe(recipes, &merged, r, nil); err != nil {
		return OperationRecipes{}, nil, err
	}
	scripts, err := expandScripts(r.Source.DirectoryPath, fsutil.ExpandAll(merged.scripts))
	if err != nil {
		return OperationRecipes{}, nil, err
	}
	res := resolved{
		ExtraDirs:    merged.dirs,
		Packages:     dedupePackages(merged.packages),
		ToolPackages: dedupeToolPackages(merged.toolPackages),
		Scripts:      scripts,
		Links:        merged.explicitLinks,
		Copies:       merged.explicitCopies,
		Templates:    merged.explicitTmpls,
	}
	if err := classify(workingDir, merged, &res); err != nil {
		return OperationRecipes{}, nil, err
	}
	applyExcludes(merged.exclude, &res)
	return res.operationRecipes(), merged.refs, nil
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
		InstallPackages: InstallPackagesOperationRecipe{Packages: res.Packages, ToolPackages: res.ToolPackages},
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

func mergeRecipe(recipes []ProfileRecipe, merged *mergedInclude, rec ProfileRecipe, seen []string) error {
	name := rec.Source.GetProfileName()
	if slices.Contains(seen, name) {
		return fmt.Errorf("include.profiles cycle: %v -> %s", seen, name)
	}
	child := append(slices.Clone(seen), name)
	for _, ref := range rec.Include.Profiles {
		if ref.URI != "" {
			dup := slices.ContainsFunc(merged.refs, func(q ProfileSourceRecipe) bool {
				return q.URI == ref.URI && q.ProfileName == ref.ProfileName
			})
			if !dup {
				merged.refs = append(merged.refs, ref)
			}
			continue
		}
		m, ok := findRecipe(recipes, ref.ProfileName)
		if !ok {
			return fmt.Errorf("include.profiles names undefined profile %q (from %v)", ref.ProfileName, child)
		}
		if err := mergeRecipe(recipes, merged, m, child); err != nil {
			return err
		}
	}
	in := rec.Include
	for _, e := range in.MakeLinks {
		switch {
		case e.glob != "":
			merged.linkGlobs.add(e.glob, Perms{})
		case e.Source == "":
			return fmt.Errorf("profile %q: link entry missing source", name)
		case e.DestRule != "":
			rule, err := ruleFromDest(e.Source, e.DestRule)
			if err != nil {
				return fmt.Errorf("profile %q: %w", name, err)
			}
			merged.linkGlobs.addRule(e.Source, Perms{}, rule)
		case len(e.Dest) == 0:
			return fmt.Errorf("profile %q: link entry %q missing dest", name, e.Source)
		default:
			merged.explicitLinks = append(merged.explicitLinks, FileItem{Rel: e.Source, Dests: e.Dest})
		}
	}
	if err := splitEntries(in.MakeCopies, &merged.copyGlobs, &merged.explicitCopies); err != nil {
		return fmt.Errorf("profile %q: %w", name, err)
	}
	if err := splitTemplates(in.RenderTemplates, &merged.tmplGlobs, &merged.explicitTmpls); err != nil {
		return fmt.Errorf("profile %q: %w", name, err)
	}
	for _, e := range in.MakeDirs {
		merged.dirs = append(merged.dirs, expandDirGroup(e)...)
	}
	merged.packages = append(merged.packages, in.InstallPackages...)
	for tool, refs := range in.InstallToolPackages {
		if merged.toolPackages == nil {
			merged.toolPackages = map[string][]ToolPackageRef{}
		}
		merged.toolPackages[tool] = append(merged.toolPackages[tool], refs...)
	}
	merged.scripts = append(merged.scripts, in.Scripts...)
	merged.exclude.append(rec.Exclude)
	return nil
}

func (ex *excludeSet) append(o excludeSet) {
	ex.MakeLinks = append(ex.MakeLinks, o.MakeLinks...)
	ex.MakeCopies = append(ex.MakeCopies, o.MakeCopies...)
	ex.RenderTemplates = append(ex.RenderTemplates, o.RenderTemplates...)
	ex.MakeDirs = append(ex.MakeDirs, o.MakeDirs...)
	ex.InstallPackages = append(ex.InstallPackages, o.InstallPackages...)
	for tool, globs := range o.InstallToolPackages {
		if ex.InstallToolPackages == nil {
			ex.InstallToolPackages = map[string][]string{}
		}
		ex.InstallToolPackages[tool] = append(ex.InstallToolPackages[tool], globs...)
	}
	ex.Scripts = append(ex.Scripts, o.Scripts...)
}

func splitEntries(entries []entry, globs *globSet, explicit *[]FileItem) error {
	for _, e := range entries {
		if err := splitGroupFiles(e.Files, e.Perms, globs, explicit, nil, func(f fileSpec) FileItem {
			return FileItem{Rel: f.Source, Dests: f.Dest, Perms: e.Perms}
		}); err != nil {
			return err
		}
	}
	return nil
}

func splitTemplates(entries []templateGroup, globs *globSet, explicit *[]FileItem) error {
	for _, e := range entries {
		if err := splitGroupFiles(e.Files, e.Perms, globs, explicit, checkTemplateSpec, func(f fileSpec) FileItem {
			return FileItem{Rel: f.Source, Dests: mergeDestOptions(e.Options, f.Dest), Ctx: fsutil.MergeMap(e.Ctx, f.Ctx), Perms: e.Perms}
		}); err != nil {
			return err
		}
	}
	return nil
}

func splitGroupFiles(files []fileSpec, perms Perms, globs *globSet, explicit *[]FileItem, check func(fileSpec) error, makeItem func(fileSpec) FileItem) error {
	for _, f := range files {
		if check != nil {
			if err := check(f); err != nil {
				return err
			}
		}
		switch {
		case f.glob != "":
			globs.add(f.glob, perms)
		case f.DestRule != "":
			rule, err := ruleFromDest(f.Source, f.DestRule)
			if err != nil {
				return err
			}
			globs.addRule(f.Source, perms, rule)
		default:
			*explicit = append(*explicit, makeItem(f))
		}
	}
	return nil
}

func checkTemplateSpec(f fileSpec) error {
	switch {
	case f.glob != "":
		if IsRemoteSrc(f.glob) {
			return fmt.Errorf("renderTemplates glob cannot be remote: %q", f.glob)
		}
	case f.DestRule != "":
		if IsRemoteSrc(f.Source) {
			return fmt.Errorf("renderTemplates dest rewrite cannot be remote: %q", f.Source)
		}
	case IsRemoteSrc(f.Source):
		if !render.IsRemoteRef(RemoteSrcRef(f.Source)) {
			return fmt.Errorf("renderTemplates remote source malformed, want @<repo>//<path>[?ref=<ref>]: %q", f.Source)
		}
		if len(f.Dest) == 0 {
			return fmt.Errorf("renderTemplates remote source requires explicit dest: %q", f.Source)
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

func expandDirGroup(g dirGroup) []FileItem {
	var out []FileItem
	for _, f := range g.Files {
		paths := f.Dest
		if f.glob != "" {
			paths = []DestSpec{{Path: f.glob}}
		}
		for _, d := range paths {
			for _, p := range fsutil.ExpandBraces(d.Path) {
				out = append(out, FileItem{Dests: []DestSpec{{Path: p}}, Perms: g.Perms})
			}
		}
	}
	return out
}

func classify(workingDir string, merged mergedInclude, res *resolved) error {
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		return nil
	}
	tracked, err := fsutil.ListTrackedFiles(workingDir)
	if err != nil {
		return err
	}
	explicit := collectExplicitRels(merged)
	for _, rel := range tracked {
		if explicit[rel] {
			continue
		}
		switch {
		case IsTmplSrc(rel) && appendMatch(merged.tmplGlobs, rel, TrimTmplExt, &res.Templates):
		case strings.HasSuffix(rel, CpExt) && appendMatch(merged.copyGlobs, rel, trimCpExt, &res.Copies):
		case filepath.Base(rel) == ".gitkeep":
		case appendMatch(merged.linkGlobs, rel, identity, &res.Links):
		}
	}
	collectDirs(res)
	return nil
}

func identity(rel string) string  { return rel }
func trimCpExt(rel string) string { return strings.TrimSuffix(rel, CpExt) }

func appendMatch(gs globSet, rel string, destBase func(string) string, items *[]FileItem) bool {
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

func collectExplicitRels(merged mergedInclude) map[string]bool {
	m := map[string]bool{}
	for _, it := range merged.explicitLinks {
		m[it.Rel] = true
	}
	for _, it := range merged.explicitCopies {
		m[it.Rel] = true
	}
	for _, it := range merged.explicitTmpls {
		m[it.Rel] = true
	}
	return m
}

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

// [why] later entries win, so a profile ref can re-pin a package version
func dedupePackages(xs []PackageRef) []PackageRef {
	at := map[string]int{}
	var out []PackageRef
	for _, x := range xs {
		if i, ok := at[x.Name]; ok {
			if len(x.Versions) > 0 {
				out[i].Versions = x.Versions
			}
			if x.GlobalVersion != "" {
				out[i].GlobalVersion = x.GlobalVersion
			}
			continue
		}
		at[x.Name] = len(out)
		out = append(out, x)
	}
	return out
}

// [why] later entries win, so a profile ref can re-pin a tool package version
func dedupeToolPackages(m map[string][]ToolPackageRef) map[string][]ToolPackageRef {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string][]ToolPackageRef, len(m))
	for tool, refs := range m {
		at := map[string]int{}
		var kept []ToolPackageRef
		for _, r := range refs {
			if i, ok := at[r.Name]; ok {
				if r.Version != "" {
					kept[i].Version = r.Version
				}
				continue
			}
			at[r.Name] = len(kept)
			kept = append(kept, r)
		}
		out[tool] = kept
	}
	return out
}

func applyExcludes(ex excludeSet, res *resolved) {
	linkGlobs := fsutil.ExpandAll(ex.MakeLinks)
	copyGlobs := fsutil.ExpandAll(ex.MakeCopies)
	tmplGlobs := fsutil.ExpandAll(ex.RenderTemplates)
	dirGlobs := fsutil.ExpandAll(ex.MakeDirs)
	pkgGlobs := fsutil.ExpandAll(ex.InstallPackages)
	scriptGlobs := fsutil.ExpandAll(ex.Scripts)

	res.Links = dropFiles(res.Links, linkGlobs)
	res.Copies = dropFiles(res.Copies, copyGlobs)
	res.Templates = dropFiles(res.Templates, tmplGlobs)
	res.ExtraDirs = dropFiles(res.ExtraDirs, dirGlobs)
	res.Packages = dropPackages(res.Packages, pkgGlobs)
	for tool, globs := range ex.InstallToolPackages {
		if res.ToolPackages[tool] == nil {
			continue
		}
		expanded := fsutil.ExpandAll(globs)
		res.ToolPackages[tool] = slices.DeleteFunc(res.ToolPackages[tool], func(r ToolPackageRef) bool {
			return isAnyGlobMatch(expanded, r.Name)
		})
	}
	res.Scripts = dropStrings(res.Scripts, scriptGlobs)

	res.Dirs = nil
	collectDirs(res)
	res.Dirs = dropStrings(res.Dirs, dirGlobs)
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

func dropPackages(xs []PackageRef, globs []string) []PackageRef {
	if len(globs) == 0 {
		return xs
	}
	return slices.DeleteFunc(xs, func(x PackageRef) bool { return isAnyGlobMatch(globs, x.Name) })
}

func dropStrings(xs, globs []string) []string {
	if len(globs) == 0 {
		return xs
	}
	return slices.DeleteFunc(xs, func(x string) bool { return isAnyGlobMatch(globs, x) })
}

func isAnyGlobMatch(globs []string, rel string) bool {
	return slices.ContainsFunc(globs, func(g string) bool { return isGlobMatch(g, rel) })
}

func (o ProfileOptions) Over(spec Options) ProfileOptions {
	return overlayProfileOptions(o, ProfileOptions{
		AutoDiscover:            spec.AutoDiscover,
		LogLevel:                spec.LogLevel,
		ProfileWorkingDirectory: spec.ProfileWorkingDirectory,
		Packages:                spec.Packages,
	})
}

func (o ProfileOptions) OverRef(entry ProfileOptions) ProfileOptions {
	if entry.RunIf == nil {
		entry.RunIf = o.RunIf
	}
	return overlayProfileOptions(entry, o)
}

func overlayProfileOptions(hi, lo ProfileOptions) ProfileOptions {
	if hi.AutoDiscover == nil {
		hi.AutoDiscover = lo.AutoDiscover
	}
	if hi.LogLevel == "" {
		hi.LogLevel = lo.LogLevel
	}
	if hi.ProfileWorkingDirectory == "" {
		hi.ProfileWorkingDirectory = lo.ProfileWorkingDirectory
	}
	hi.Packages = overlayPackages(hi.Packages, lo.Packages)
	return hi
}

func overlayPackages(hi, lo Packages) Packages {
	if hi.File == "" {
		hi.File = lo.File
	}
	if len(hi.PreferredInstallationMethods) == 0 {
		hi.PreferredInstallationMethods = lo.PreferredInstallationMethods
	}
	if len(hi.BinariesRemoteArchive.InstallDestinationCandidates) == 0 {
		hi.BinariesRemoteArchive.InstallDestinationCandidates = lo.BinariesRemoteArchive.InstallDestinationCandidates
	}
	if hi.BinariesRemoteArchive.CheckPresentOnPath == nil {
		hi.BinariesRemoteArchive.CheckPresentOnPath = lo.BinariesRemoteArchive.CheckPresentOnPath
	}
	if hi.Completions.Zsh.Enabled == nil {
		hi.Completions.Zsh.Enabled = lo.Completions.Zsh.Enabled
	}
	if len(hi.Completions.Zsh.InstallDestinationCandidates) == 0 {
		hi.Completions.Zsh.InstallDestinationCandidates = lo.Completions.Zsh.InstallDestinationCandidates
	}
	if hi.Completions.Zsh.CheckPresentOnFpath == nil {
		hi.Completions.Zsh.CheckPresentOnFpath = lo.Completions.Zsh.CheckPresentOnFpath
	}
	return hi
}

// [<] 🤖🤖
