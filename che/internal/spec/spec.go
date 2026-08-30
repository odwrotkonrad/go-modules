package spec

// [>] 🤖🤖

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/spec/envinterp"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

const (
	TemplateExt       = ".tpl"
	CpExt             = ".ontoHost.cp"
	DefaultWorkingDir = "."
)

var templateExts = []string{".ontoHost.tpl", ".ontoRepo.tpl", ".tpl"}

func IsTemplateSrc(rel string) bool { return strings.HasSuffix(rel, TemplateExt) }

const RemoteSrcPrefix = render.GitMarker

func IsRemoteSrc(source string) bool {
	_, ok := render.CutGitMarker(source)
	return ok
}

func (r ProfileRecipe) IncludedProfileRefs() []string {
	var out []string
	for _, ref := range r.Include.Profiles {
		out = append(out, ref.DisplayRef())
	}
	return out
}

func (r ProfileRecipe) SourcedRefs() []ProfileSourceRecipe {
	var out []ProfileSourceRecipe
	for _, ref := range r.Include.Profiles {
		if ref.URI != "" {
			out = append(out, ref)
		}
	}
	return out
}

// ProfilePathSep joins the profile names of a ref's path: a::b names b as included by top-level a.
const ProfilePathSep = "::"

func expandProfileRefs(refs []ProfileSourceRecipe) []ProfileSourceRecipe {
	var out []ProfileSourceRecipe
	for _, ref := range refs {
		out = append(out, ref.Expand()...)
	}
	return out
}

func RemoteSrcRef(source string) string {
	rest, _ := render.CutGitMarker(source)
	return rest
}

func TrimTemplateExt(rel string) string {
	for _, ext := range templateExts {
		if trimmed, ok := strings.CutSuffix(rel, ext); ok {
			return trimmed
		}
	}
	return rel
}

func (s *SourceSpec) UnmarshalYAML(value *yaml.Node) error {
	type alias SourceSpec
	if err := decodeScalarOr(value, &s.raw, (*alias)(s)); err != nil {
		return err
	}
	if s.raw == "" && s.URL == "" {
		return fmt.Errorf("source object requires url")
	}
	return nil
}

// String renders the source in the loader's string form: the scalar verbatim, an object as git::<url>[@<ref>][//<filepath>].
func (s SourceSpec) String() string {
	if s.raw != "" || s.URL == "" {
		return s.raw
	}
	out := RemoteSrcPrefix + s.URL
	if s.Ref != "" {
		out += "@" + s.Ref
	}
	if s.FilePath != "" {
		out += "//" + strings.TrimPrefix(s.FilePath, "/")
	}
	return out
}

// [why] every reader of a node's source speaks the string form: an object source decodes once,
// here, into the scalar the rest of the loader already understands
func flattenObjectSource(value *yaml.Node) (*yaml.Node, error) {
	if value.Kind != yaml.MappingNode {
		return value, nil
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		if value.Content[i].Value != "source" || value.Content[i+1].Kind != yaml.MappingNode {
			continue
		}
		var src SourceSpec
		if err := value.Content[i+1].Decode(&src); err != nil {
			return nil, err
		}
		rest := *value
		rest.Content = slices.Clone(value.Content)
		rest.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: src.String()}
		return &rest, nil
	}
	return value, nil
}

func (l *linkEntry) UnmarshalYAML(value *yaml.Node) error {
	node, err := flattenObjectSource(value)
	if err != nil {
		return err
	}
	if rule, rest, ok := takeScalarDest(node); ok {
		l.DestRule = rule
		node = rest
	}
	sources, node, err := takeSourceList(node)
	if err != nil {
		return err
	}
	type alias linkEntry
	if err := decodeScalarOr(node, &l.glob, (*alias)(l)); err != nil {
		return err
	}
	l.Sources = sources
	return checkSourceList(sources, l.DestRule != "" || len(l.Dest) > 0, false)
}

func takeSourceList(value *yaml.Node) ([]string, *yaml.Node, error) {
	if value.Kind != yaml.MappingNode {
		return nil, value, nil
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		if value.Content[i].Value != "source" || value.Content[i+1].Kind != yaml.SequenceNode {
			continue
		}
		sources := []string{}
		if err := value.Content[i+1].Decode(&sources); err != nil {
			return nil, nil, fmt.Errorf("source list: %w", err)
		}
		rest := *value
		rest.Content = slices.Concat(value.Content[:i], value.Content[i+2:])
		return sources, &rest, nil
	}
	return nil, value, nil
}

func checkSourceList(sources []string, hasDest, hasChildren bool) error {
	if sources == nil {
		return nil
	}
	switch {
	case len(sources) == 0:
		return errors.New("source list is empty")
	case hasDest:
		return errors.New("source list takes no dest: each dest is the source path under the enclosing dest prefix")
	case hasChildren:
		return errors.New("source list takes no nested <<<: a list is a set of leaves, not a group prefix")
	}
	for _, src := range sources {
		if IsRemoteSrc(src) {
			return fmt.Errorf("source list item cannot be remote: %q", src)
		}
	}
	return nil
}

func decodeScalarOr[T any](value *yaml.Node, scalar *string, obj *T) error {
	if value.Kind == yaml.ScalarNode {
		*scalar = value.Value
		return nil
	}
	return value.Decode(obj)
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

func (o optionsSpec) over(base render.Options) render.Options {
	if o.WriteType != nil {
		base.WriteType = *o.WriteType
	}
	if o.CommentPrefix != nil {
		base.CommentPrefix = *o.CommentPrefix
	}
	if o.SkipAutoGeneratedHeader != nil {
		base.SkipAutoGeneratedHeader = *o.SkipAutoGeneratedHeader
	}
	if o.RenderReferencedFiles != nil {
		base.RenderReferencedFiles = *o.RenderReferencedFiles
	}
	if o.MergeUpdate != nil {
		base.MergeUpdate = *o.MergeUpdate
	}
	return base
}

func (s *Scalar) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("want a scalar, got kind %v", value.Kind)
	}
	*s = Scalar(value.Value)
	return nil
}

func (s *StringOrList) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		*s = StringOrList{value.Value}
		return nil
	}
	var list []string
	if err := value.Decode(&list); err != nil {
		return err
	}
	*s = list
	return nil
}

func (d *dirSpec) UnmarshalYAML(value *yaml.Node) error {
	type alias dirSpec
	return decodeScalarOr(value, &d.glob, (*alias)(d))
}

func (c *copyNode) UnmarshalYAML(value *yaml.Node) error {
	type alias copyNode
	if err := decodeTreeNode(value, &c.glob, &c.DestRule, &c.Dest, &c.Sources, (*alias)(c)); err != nil {
		return err
	}
	return checkSourceList(c.Sources, c.DestRule != "" || len(c.Dest) > 0, len(c.Children) > 0)
}

func (t *templateNode) UnmarshalYAML(value *yaml.Node) error {
	type alias templateNode
	if err := decodeTreeNode(value, &t.glob, &t.DestRule, &t.Dest, &t.Sources, (*alias)(t)); err != nil {
		return err
	}
	return checkSourceList(t.Sources, t.DestRule != "" || len(t.Dest) > 0, len(t.Children) > 0)
}

func decodeTreeNode[T any](value *yaml.Node, glob, destRule *string, dest *[]DestSpec, sources *[]string, obj *T) error {
	node, err := flattenObjectSource(value)
	if err != nil {
		return err
	}
	scalarDest, rest, hasScalarDest := takeScalarDest(node)
	if hasScalarDest {
		node = rest
	}
	if *sources, node, err = takeSourceList(node); err != nil {
		return err
	}
	if err := decodeScalarOr(node, glob, obj); err != nil {
		return err
	}
	switch {
	case !hasScalarDest:
	case isDestRule(scalarDest):
		*destRule = scalarDest
	default:
		*dest = []DestSpec{{Path: scalarDest}}
	}
	return nil
}

func isDestRule(dest string) bool {
	if len(dest) >= 2 && dest[0] == 's' && dest[1] != '\\' && !isWord(dest[1]) {
		return true
	}
	return strings.HasSuffix(dest, "/**")
}

func takeScalarDest(value *yaml.Node) (string, *yaml.Node, bool) {
	if value.Kind != yaml.MappingNode {
		return "", nil, false
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		if value.Content[i].Value != "dest" || value.Content[i+1].Kind != yaml.ScalarNode {
			continue
		}
		rule := value.Content[i+1].Value
		rest := *value
		rest.Content = slices.Concat(value.Content[:i], value.Content[i+2:])
		return rule, &rest, true
	}
	return "", nil, false
}

func DestRel(it FileItem) string {
	if len(it.Dests) > 0 {
		return it.Dests[0].Path
	}
	return it.Rel
}

// Load reads one spec file under in.
func Load(path string, in Interp) (*Doc, error) { return LoadMerged([]string{path}, in) }

// LoadMerged reads several spec files as one spec (the invoked che.yml and .che/che.yml): top-level
// mappings merge key by key, profilesDefinitions and specsInclude concatenate, a key set twice errors.
func LoadMerged(paths []string, in Interp) (*Doc, error) {
	m, err := mergedSpecNode(paths)
	if err != nil {
		return nil, err
	}
	d := &Doc{}
	if m == nil {
		return d, nil
	}
	label := strings.Join(paths, " + ")
	if err := checkTopLevelKeys(m); err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	it := newInterpolator(in, m)
	it.walkDoc(m)
	d.EnvUnset, d.EnvRefs, d.Lookup, d.Vars = it.unset, it.refs, it.lookup, in.Vars
	if len(it.builtinUnknown) > 0 {
		return nil, BuiltinUnknownError(label, it.builtinUnknown)
	}
	if len(it.varUnset) > 0 {
		return nil, VarUnsetError(label, it.varUnset)
	}
	if in.Policy != envinterp.Policies.Empty && len(d.EnvUnset[topLevelProfile]) > 0 {
		return nil, EnvUnsetError(label, d.EnvUnset[topLevelProfile])
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		key, node := m.Content[i].Value, m.Content[i+1]
		if err := d.decodeKey(key, node); err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
	}
	if d.VarDefs, err = specVarDefsOf(m, d.ProfileRecipes); err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	return d, nil
}

func readSpecNode(path string) (*yaml.Node, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("spec not found: %s", path)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	m := doc.Content[0]
	if m.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("parse %s: want a mapping at the top level", path)
	}
	return m, nil
}

func mergedSpecNode(paths []string) (*yaml.Node, error) {
	var merged *yaml.Node
	for _, path := range paths {
		m, err := readSpecNode(path)
		if err != nil {
			return nil, err
		}
		switch {
		case m == nil:
		case merged == nil:
			merged = m
		default:
			if err := mergeMappings(merged, m, ""); err != nil {
				return nil, fmt.Errorf("%s + %s: %w", paths[0], path, err)
			}
		}
	}
	return merged, nil
}

// [why] two files, one spec: mappings merge down to the leaves, sequences concatenate, a leaf or a
// profile set by both files is a conflict the user resolves, never a silent override
func mergeMappings(dst, src *yaml.Node, path string) error {
	for i := 0; i+1 < len(src.Content); i += 2 {
		key, value := src.Content[i], src.Content[i+1]
		existing := mapValue(dst, key.Value)
		keyPath := path + "/" + key.Value
		switch {
		case existing == nil:
			dst.Content = append(dst.Content, key, value)
		case path == "/"+keyProfiles:
			return fmt.Errorf("profile %q defined in both files", key.Value)
		case existing.Kind == yaml.MappingNode && value.Kind == yaml.MappingNode:
			if err := mergeMappings(existing, value, keyPath); err != nil {
				return err
			}
		case existing.Kind == yaml.SequenceNode && value.Kind == yaml.SequenceNode:
			existing.Content = append(existing.Content, value.Content...)
		default:
			return fmt.Errorf("%s set in both files", keyPath)
		}
	}
	return nil
}

func checkTopLevelKeys(m *yaml.Node) error {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if key := m.Content[i].Value; !slices.Contains(topLevelKeys, key) {
			return fmt.Errorf("unknown top-level key %q: want %s", key, strings.Join(topLevelKeys, " | "))
		}
	}
	return nil
}

func (d *Doc) decodeKey(key string, node *yaml.Node) error {
	switch key {
	case keyOptions:
		return node.Decode(&d.Options)
	case keyEnv:
		return node.Decode(&d.Env)
	case keyVarDefs:
		return nil
	case keySpecsInclude:
		if err := node.Decode(&d.SpecsInclude); err != nil {
			return fmt.Errorf("parse %s: %w", keySpecsInclude, err)
		}
		return nil
	case keyProfiles:
		if node.Kind != yaml.MappingNode {
			return fmt.Errorf("parse %s: want a mapping keyed by profile name", keyProfiles)
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			if err := d.decodeProfile(node.Content[i].Value, node.Content[i+1]); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown top-level key %q: want %s", key, strings.Join(topLevelKeys, " | "))
	}
}

func (d *Doc) decodeProfile(name string, node *yaml.Node) error {
	if _, err := FindRecipe(d.ProfileRecipes, name); err == nil {
		return fmt.Errorf("parse profile %q: defined twice", name)
	}
	var ps ProfileRecipe
	if err := node.Decode(&ps); err != nil {
		return fmt.Errorf("parse profile %q: %w", name, err)
	}
	if err := checkProfileType(name, ps.Type); err != nil {
		return err
	}
	ps.Include.Profiles = expandProfileRefs(ps.Include.Profiles)
	ps.Source.ProfileName = name
	d.ProfileRecipes = append(d.ProfileRecipes, ps)
	return nil
}

// PeekVarDefs decodes only the variable definitions of a spec (the top-level variablesDefinitions
// and each profile's own), uninterpolated: values resolve before the doc walks.
func PeekVarDefs(paths []string) (SpecVarDefs, error) {
	m, err := mergedSpecNode(paths)
	if err != nil || m == nil {
		return SpecVarDefs{}, err
	}
	var profiles []ProfileRecipe
	if block := mapValue(m, keyProfiles); block != nil && block.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(block.Content); i += 2 {
			var own struct {
				VarDefs VarDefs `yaml:"variablesDefinitions"`
			}
			if err := block.Content[i+1].Decode(&own); err != nil {
				return SpecVarDefs{}, fmt.Errorf("%s: parse profile %q: %w", strings.Join(paths, " + "), block.Content[i].Value, err)
			}
			profiles = append(profiles, ProfileRecipe{Source: ProfileSourceRecipe{ProfileName: block.Content[i].Value}, VarDefs: own.VarDefs})
		}
	}
	defs, err := specVarDefsOf(m, profiles)
	if err != nil {
		return SpecVarDefs{}, fmt.Errorf("%s: %w", strings.Join(paths, " + "), err)
	}
	return defs, nil
}

func specVarDefsOf(m *yaml.Node, profiles []ProfileRecipe) (SpecVarDefs, error) {
	var defs SpecVarDefs
	if node := mapValue(m, keyVarDefs); node != nil {
		if err := node.Decode(&defs); err != nil {
			return defs, fmt.Errorf("parse %s: %w", keyVarDefs, err)
		}
	}
	for _, rec := range profiles {
		name := rec.Source.ProfileName
		if len(rec.VarDefs) == 0 {
			continue
		}
		if _, both := defs.Profiles[name]; both {
			return defs, fmt.Errorf("profile %q: variablesDefinitions given both in %s.profilesVariablesDefinitions and in the profile", name, keyVarDefs)
		}
		if defs.Profiles == nil {
			defs.Profiles = map[string]VarDefs{}
		}
		defs.Profiles[name] = rec.VarDefs
	}
	return defs, nil
}

// [why] a missing type warns, not errors: consumers pull upstream specs at pinned tags, so the
// requirement bites only once every upstream has re-tagged with a type on each profile
func checkProfileType(name string, t ProfileType) error {
	if t == "" {
		return fmt.Errorf("profile %q: type is required: %s", name, strings.Join(ProfileTypeNames, " | "))
	}
	if err := ValidateProfileType(string(t)); err != nil {
		return fmt.Errorf("profile %q: %w", name, err)
	}
	return nil
}

// [<] 🤖🤖
