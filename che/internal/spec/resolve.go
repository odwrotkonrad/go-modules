package spec

// [>] 🤖🤖

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path"
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
		if rec.Options.AutoDiscover != nil && !*rec.Options.AutoDiscover {
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
		Templates:    merged.explicitTemplates,
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
			for _, hit := range hits {
				rel, err := filepath.Rel(repoRoot, hit)
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
			dup := slices.ContainsFunc(merged.refs, func(seen ProfileSourceRecipe) bool {
				return seen.URI == ref.URI && seen.ProfileName == ref.ProfileName && seen.Ref == ref.Ref
			})
			if !dup {
				merged.refs = append(merged.refs, ref)
			}
			continue
		}
		included, ok := findRecipe(recipes, ref.ProfileName)
		if !ok {
			return fmt.Errorf("include.profiles names undefined profile %q (from %v)", ref.ProfileName, child)
		}
		if err := mergeRecipe(recipes, merged, included, child); err != nil {
			return err
		}
	}
	include := rec.Include
	for _, entry := range include.MakeLinks {
		switch {
		case entry.glob != "":
			merged.linkGlobs.add(entry.glob, Perms{})
		case len(entry.Sources) > 0:
			for _, glob := range entry.Sources {
				merged.linkGlobs.add(glob, Perms{})
			}
		case entry.Source == "":
			return fmt.Errorf("profile %q: link entry missing source", name)
		case entry.DestRule != "":
			rule, err := ruleFromDest(entry.Source, entry.DestRule)
			if err != nil {
				return fmt.Errorf("profile %q: %w", name, err)
			}
			merged.linkGlobs.addRule(entry.Source, Perms{}, rule)
		case len(entry.Dest) == 0:
			return fmt.Errorf("profile %q: link entry %q missing dest", name, entry.Source)
		default:
			merged.explicitLinks = append(merged.explicitLinks, FileItem{Rel: entry.Source, Dests: entry.Dest})
		}
	}
	if err := splitCopies(include.MakeCopies, &merged.copyGlobs, &merged.explicitCopies); err != nil {
		return fmt.Errorf("profile %q: %w", name, err)
	}
	if err := splitTemplates(include.RenderTemplates, &merged.templateGlobs, &merged.explicitTemplates); err != nil {
		return fmt.Errorf("profile %q: %w", name, err)
	}
	for _, entry := range include.MakeDirs {
		merged.dirs = append(merged.dirs, expandDirGroup(entry)...)
	}
	merged.packages = append(merged.packages, include.InstallPackages...)
	for tool, refs := range include.InstallToolPackages {
		if merged.toolPackages == nil {
			merged.toolPackages = map[string][]ToolPackageRef{}
		}
		merged.toolPackages[tool] = append(merged.toolPackages[tool], refs...)
	}
	merged.scripts = append(merged.scripts, include.Scripts...)
	merged.exclude.append(rec.Exclude)
	return nil
}

func (ex *excludeSet) append(other excludeSet) {
	ex.MakeLinks = append(ex.MakeLinks, other.MakeLinks...)
	ex.MakeCopies = append(ex.MakeCopies, other.MakeCopies...)
	ex.RenderTemplates = append(ex.RenderTemplates, other.RenderTemplates...)
	ex.MakeDirs = append(ex.MakeDirs, other.MakeDirs...)
	ex.InstallPackages = append(ex.InstallPackages, other.InstallPackages...)
	for tool, globs := range other.InstallToolPackages {
		if ex.InstallToolPackages == nil {
			ex.InstallToolPackages = map[string][]string{}
		}
		ex.InstallToolPackages[tool] = append(ex.InstallToolPackages[tool], globs...)
	}
	ex.Scripts = append(ex.Scripts, other.Scripts...)
}

// [why] one walker for both trees: a copy node is a template node with no variables and no options
func splitCopies(nodes []copyNode, globs *globSet, explicit *[]FileItem) error {
	return splitTemplateNodes(copyTreeAsTemplates(nodes), templateInherited{op: "makeCopies"}, globs, explicit)
}

func copyTreeAsTemplates(nodes []copyNode) []templateNode {
	out := make([]templateNode, len(nodes))
	for i, node := range nodes {
		out[i] = templateNode{
			Perms:    node.Perms,
			glob:     node.glob,
			DestRule: node.DestRule,
			Sources:  node.Sources,
			Source:   node.Source,
			Dest:     node.Dest,
			Children: copyTreeAsTemplates(node.Children),
		}
	}
	return out
}

func splitTemplates(nodes []templateNode, globs *globSet, explicit *[]FileItem) error {
	return splitTemplateNodes(nodes, templateInherited{op: "renderTemplates"}, globs, explicit)
}

func splitTemplateNodes(nodes []templateNode, up templateInherited, globs *globSet, explicit *[]FileItem) error {
	for _, node := range nodes {
		if len(node.Sources) > 0 {
			if err := splitTemplateNodes(sourceListLeaves(node, IsRemoteSrc(up.prefix)), up, globs, explicit); err != nil {
				return err
			}
			continue
		}
		down := templateInherited{
			op:         up.op,
			destPrefix: path.Join(up.destPrefix, groupDestPrefix(node)),
			perms:      overlayPerms(node.Perms, up.perms),
			vars:       fsutil.MergeMap(up.vars, node.Variables),
			options:    overlayRenderOptions(node.Options, up.options),
		}
		var err error
		if down.prefix, err = joinTemplateSource(up.op, up.prefix, node.Source); err != nil {
			return err
		}
		if len(node.Children) > 0 {
			if err := checkTemplateGroup(up.op, node); err != nil {
				return err
			}
			if err := splitTemplateNodes(node.Children, down, globs, explicit); err != nil {
				return err
			}
			continue
		}
		if err := splitTemplateLeaf(node, down, globs, explicit); err != nil {
			return err
		}
	}
	return nil
}

func sourceListLeaves(node templateNode, remotePrefix bool) []templateNode {
	out := make([]templateNode, len(node.Sources))
	for i, src := range node.Sources {
		leaf := node
		leaf.Source, leaf.Sources = src, nil
		if remotePrefix {
			leaf.Dest = []DestSpec{{Path: TrimTemplateExt(strings.TrimLeft(src, "/"))}}
		}
		out[i] = leaf
	}
	return out
}

func splitTemplateLeaf(node templateNode, down templateInherited, globs *globSet, explicit *[]FileItem) error {
	leaf := node
	leaf.Source = down.prefix
	if err := checkTemplateSpec(down.op, leaf); err != nil {
		return err
	}
	switch {
	case leaf.glob != "":
		globs.add(leaf.glob, down.perms)
	case leaf.DestRule != "":
		rule, err := ruleFromDest(leaf.Source, leaf.DestRule)
		if err != nil {
			return err
		}
		globs.addRule(leaf.Source, down.perms, rule)
	default:
		*explicit = append(*explicit, FileItem{
			Rel:   leaf.Source,
			Dests: mergeDestOptions(down.options, prefixDests(down.destPrefix, node.Source, leaf.Dest)),
			Vars:  down.vars,
			Perms: down.perms,
		})
	}
	return nil
}

// [why] host dests ($HOME, ~, absolute) anchor themselves: only repo-relative dests take the prefix
func prefixDests(prefix, childSource string, dests []DestSpec) []DestSpec {
	if prefix == "" {
		return dests
	}
	if len(dests) == 0 {
		return []DestSpec{{Path: path.Join(prefix, TrimTemplateExt(childSource))}}
	}
	out := slices.Clone(dests)
	for i := range out {
		if p := out[i].Path; !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "~") && !strings.HasPrefix(p, "$") {
			out[i].Path = path.Join(prefix, p)
		}
	}
	return out
}

// [why] a group's dest is a prefix, not a target: one path, no per-dest options
func groupDestPrefix(node templateNode) string {
	if len(node.Children) == 0 || len(node.Dest) != 1 {
		return ""
	}
	return node.Dest[0].Path
}

func checkTemplateGroup(op string, node templateNode) error {
	switch {
	case node.DestRule != "" || node.glob != "":
		return fmt.Errorf("%s group %q carries nested <<< and a dest rewrite or glob: it is one or the other", op, node.Source)
	case node.Source == "" && len(node.Dest) == 0:
		return fmt.Errorf("%s group carries nested <<< without a source or dest prefix: <<< assembles paths, put a leaf at this level instead", op)
	case len(node.Dest) > 1:
		return fmt.Errorf("%s group %q dest is a prefix: want one path, got %d", op, node.Source, len(node.Dest))
	case len(node.Dest) == 1 && node.Dest[0].opts != (optionsSpec{}):
		return fmt.Errorf("%s group %q dest is a prefix: set options on the group instead", op, node.Source)
	}
	return nil
}

// [why] the ref rides the repo, so a remote prefix and a leaf path plainly concatenate
func joinTemplateSource(op, prefix, src string) (string, error) {
	if prefix == "" || src == "" {
		return cmp.Or(src, prefix), nil
	}
	if !IsRemoteSrc(prefix) {
		return path.Join(prefix, src), nil
	}
	if IsRemoteSrc(src) {
		return "", fmt.Errorf("%s group %q: nested source %q names its own repo, one ref per group", op, prefix, src)
	}
	sep := "/"
	if repoSubdir(RemoteSrcRef(prefix)) == "" {
		sep = "//"
	}
	joined := strings.TrimSuffix(prefix, "/") + sep + strings.TrimLeft(src, "/")
	if !render.IsRemoteRef(joined) {
		return "", fmt.Errorf("%s group source %q joins to a malformed remote ref, want %s<repo>[@<ref>]//<path>: %q", op, prefix, RemoteSrcPrefix, joined)
	}
	return joined, nil
}

func overlayRenderOptions(override, base optionsSpec) optionsSpec {
	if override.WriteType == nil {
		override.WriteType = base.WriteType
	}
	if override.CommentPrefix == nil {
		override.CommentPrefix = base.CommentPrefix
	}
	if override.SkipAutoGeneratedHeader == nil {
		override.SkipAutoGeneratedHeader = base.SkipAutoGeneratedHeader
	}
	if override.MergeUpdate == nil {
		override.MergeUpdate = base.MergeUpdate
	}
	if override.RenderReferencedFiles == nil {
		override.RenderReferencedFiles = base.RenderReferencedFiles
	}
	return override
}

func overlayPerms(override, base Perms) Perms {
	if override.Owner == "" {
		override.Owner = base.Owner
	}
	if override.OwnerGroup == "" {
		override.OwnerGroup = base.OwnerGroup
	}
	if override.Chmod == "" {
		override.Chmod = base.Chmod
	}
	return override
}

func checkTemplateSpec(op string, file templateNode) error {
	switch {
	case file.Source == "" && file.glob == "":
		return fmt.Errorf("%s node needs a source, a glob or nested <<<", op)
	case file.glob != "":
		if IsRemoteSrc(file.glob) {
			return fmt.Errorf("%s glob cannot be remote: %q", op, file.glob)
		}
	case file.DestRule != "":
		if IsRemoteSrc(file.Source) {
			return fmt.Errorf("%s dest rewrite cannot be remote: %q", op, file.Source)
		}
	case IsRemoteSrc(file.Source):
		if !render.IsRemoteRef(file.Source) {
			return fmt.Errorf("%s remote source malformed, want %s<repo>[@<ref>]//<path>: %q", op, RemoteSrcPrefix, file.Source)
		}
		if len(file.Dest) == 0 {
			return fmt.Errorf("%s remote source requires explicit dest: %q", op, file.Source)
		}
	}
	return nil
}

func mergeDestOptions(inherited optionsSpec, dests []DestSpec) []DestSpec {
	if inherited == (optionsSpec{}) {
		return dests
	}
	out := slices.Clone(dests)
	for i := range out {
		out[i].Options = overlayRenderOptions(out[i].opts, inherited).over(render.Options{})
	}
	return out
}

func expandDirGroup(group dirGroup) []FileItem {
	var out []FileItem
	for _, file := range group.Files {
		dests := file.Dest
		if file.glob != "" {
			dests = []DestSpec{{Path: file.glob}}
		}
		for _, dest := range dests {
			for _, path := range fsutil.ExpandBraces(dest.Path) {
				out = append(out, FileItem{Dests: []DestSpec{{Path: path}}, Perms: group.Perms})
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
		case IsTemplateSrc(rel) && appendMatch(merged.templateGlobs, rel, TrimTemplateExt, &res.Templates):
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
	matched, ok := gs.match(rel)
	if !ok {
		return false
	}
	item := FileItem{Rel: rel, Perms: matched.perms}
	if matched.rule != nil {
		base := destBase(rel)
		if dest := matched.rule.apply(base); dest != base {
			item.Dests = []DestSpec{{Path: dest}}
			item.Derived = true
		}
	}
	*items = append(*items, item)
	return true
}

func collectExplicitRels(merged mergedInclude) map[string]bool {
	rels := map[string]bool{}
	for _, item := range merged.explicitLinks {
		rels[item.Rel] = true
	}
	for _, item := range merged.explicitCopies {
		rels[item.Rel] = true
	}
	for _, item := range merged.explicitTemplates {
		rels[item.Rel] = true
	}
	return rels
}

func (gs *globSet) add(glob string, perms Perms) { gs.addRule(glob, perms, nil) }

func (gs *globSet) addRule(glob string, perms Perms, rule *destRule) {
	for _, expanded := range fsutil.ExpandBraces(glob) {
		*gs = append(*gs, globPerm{glob: expanded, perms: perms, rule: rule})
	}
}

func (gs globSet) match(rel string) (globPerm, bool) {
	for _, candidate := range slices.Backward(gs) {
		if isGlobMatch(candidate.glob, rel) {
			return candidate, true
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
		for dir := filepath.Dir(rel); dir != "." && dir != "/" && !dirSeen[dir]; dir = filepath.Dir(dir) {
			dirSeen[dir] = true
			res.Dirs = append(res.Dirs, dir)
		}
	}
	for _, item := range res.Links {
		addRel(DestRel(item))
	}
	for _, item := range res.Copies {
		addRel(DestRel(item))
	}
	for _, item := range res.Templates {
		if len(item.Dests) == 0 {
			addRel(item.Rel)
		} else if item.Derived {
			addRel(item.Dests[0].Path)
		}
	}
	slices.SortFunc(res.Links, byRel)
	slices.SortFunc(res.Copies, byRel)
	slices.SortFunc(res.Templates, byRel)
	slices.Sort(res.Dirs)
}

func byRel(a, b FileItem) int { return cmp.Compare(a.Rel, b.Rel) }

// [why] later entries win, so a profile ref can re-pin a package version
func dedupePackages(refs []PackageRef) []PackageRef {
	indexByName := map[string]int{}
	var out []PackageRef
	for _, ref := range refs {
		if i, ok := indexByName[ref.Name]; ok {
			if len(ref.Versions) > 0 {
				out[i].Versions = ref.Versions
			}
			if ref.GlobalVersion != "" {
				out[i].GlobalVersion = ref.GlobalVersion
			}
			continue
		}
		indexByName[ref.Name] = len(out)
		out = append(out, ref)
	}
	return out
}

// [why] later entries win, so a profile ref can re-pin a tool package version
func dedupeToolPackages(byTool map[string][]ToolPackageRef) map[string][]ToolPackageRef {
	if len(byTool) == 0 {
		return nil
	}
	out := make(map[string][]ToolPackageRef, len(byTool))
	for tool, refs := range byTool {
		indexByName := map[string]int{}
		var kept []ToolPackageRef
		for _, ref := range refs {
			if i, ok := indexByName[ref.Name]; ok {
				if ref.Version != "" {
					kept[i].Version = ref.Version
				}
				continue
			}
			indexByName[ref.Name] = len(kept)
			kept = append(kept, ref)
		}
		out[tool] = kept
	}
	return out
}

func applyExcludes(ex excludeSet, res *resolved) {
	linkGlobs := fsutil.ExpandAll(ex.MakeLinks)
	copyGlobs := fsutil.ExpandAll(ex.MakeCopies)
	templateGlobs := fsutil.ExpandAll(ex.RenderTemplates)
	dirGlobs := fsutil.ExpandAll(ex.MakeDirs)
	pkgGlobs := fsutil.ExpandAll(ex.InstallPackages)
	scriptGlobs := fsutil.ExpandAll(ex.Scripts)

	res.Links = dropFiles(res.Links, linkGlobs)
	res.Copies = dropFiles(res.Copies, copyGlobs)
	res.Templates = dropFiles(res.Templates, templateGlobs)
	res.ExtraDirs = dropFiles(res.ExtraDirs, dirGlobs)
	res.Packages = dropPackages(res.Packages, pkgGlobs)
	for tool, globs := range ex.InstallToolPackages {
		if res.ToolPackages[tool] == nil {
			continue
		}
		expanded := fsutil.ExpandAll(globs)
		res.ToolPackages[tool] = slices.DeleteFunc(res.ToolPackages[tool], func(ref ToolPackageRef) bool {
			return isAnyGlobMatch(expanded, ref.Name)
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
	return slices.DeleteFunc(items, func(item FileItem) bool {
		if isAnyGlobMatch(globs, item.Rel) {
			return true
		}
		for _, dest := range item.Dests {
			if isAnyGlobMatch(globs, dest.Path) {
				return true
			}
		}
		return false
	})
}

func dropPackages(refs []PackageRef, globs []string) []PackageRef {
	if len(globs) == 0 {
		return refs
	}
	return slices.DeleteFunc(refs, func(ref PackageRef) bool { return isAnyGlobMatch(globs, ref.Name) })
}

func dropStrings(values, globs []string) []string {
	if len(globs) == 0 {
		return values
	}
	return slices.DeleteFunc(values, func(value string) bool { return isAnyGlobMatch(globs, value) })
}

func isAnyGlobMatch(globs []string, rel string) bool {
	return slices.ContainsFunc(globs, func(glob string) bool { return isGlobMatch(glob, rel) })
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

func overlayProfileOptions(override, base ProfileOptions) ProfileOptions {
	if override.AutoDiscover == nil {
		override.AutoDiscover = base.AutoDiscover
	}
	if override.LogLevel == "" {
		override.LogLevel = base.LogLevel
	}
	if override.ProfileWorkingDirectory == "" {
		override.ProfileWorkingDirectory = base.ProfileWorkingDirectory
	}
	override.Packages = overlayPackages(override.Packages, base.Packages)
	return override
}

func overlayPackages(override, base Packages) Packages {
	if override.File == "" {
		override.File = base.File
	}
	if len(override.PreferredInstallationMethods) == 0 {
		override.PreferredInstallationMethods = base.PreferredInstallationMethods
	}
	if len(override.OnlyInstallationMethods) == 0 {
		override.OnlyInstallationMethods = base.OnlyInstallationMethods
	}
	if len(override.BinariesRemoteArchive.InstallDestinationCandidates) == 0 {
		override.BinariesRemoteArchive.InstallDestinationCandidates = base.BinariesRemoteArchive.InstallDestinationCandidates
	}
	if override.BinariesRemoteArchive.CheckPresentOnPath == nil {
		override.BinariesRemoteArchive.CheckPresentOnPath = base.BinariesRemoteArchive.CheckPresentOnPath
	}
	if override.Completions.Zsh.Enabled == nil {
		override.Completions.Zsh.Enabled = base.Completions.Zsh.Enabled
	}
	if len(override.Completions.Zsh.InstallDestinationCandidates) == 0 {
		override.Completions.Zsh.InstallDestinationCandidates = base.Completions.Zsh.InstallDestinationCandidates
	}
	if override.Completions.Zsh.CheckPresentOnFpath == nil {
		override.Completions.Zsh.CheckPresentOnFpath = base.Completions.Zsh.CheckPresentOnFpath
	}
	if len(override.Manpages.InstallDestinationCandidates) == 0 {
		override.Manpages.InstallDestinationCandidates = base.Manpages.InstallDestinationCandidates
	}
	if override.Manpages.CheckPresentOnManpath == nil {
		override.Manpages.CheckPresentOnManpath = base.Manpages.CheckPresentOnManpath
	}
	if override.Source.URL == "" {
		override.Source.URL = base.Source.URL
	}
	if override.Source.Ref == "" {
		override.Source.Ref = base.Source.Ref
	}
	if override.AutoUpdate.Enabled == nil {
		override.AutoUpdate.Enabled = base.AutoUpdate.Enabled
	}
	if override.AutoUpdate.If.RefIsLatest.Cooldown == "" {
		override.AutoUpdate.If.RefIsLatest.Cooldown = base.AutoUpdate.If.RefIsLatest.Cooldown
	}
	if override.AutoUpdate.If.DryRunIsTrue.Enabled == nil {
		override.AutoUpdate.If.DryRunIsTrue.Enabled = base.AutoUpdate.If.DryRunIsTrue.Enabled
	}
	return override
}

// [<] 🤖🤖
