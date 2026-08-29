package spec

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/source"
	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

func (r SourceRecipe) GetSourceType() SourceType {
	if IsRemoteSrc(r.URI) {
		return SourceTypes.Remote
	}
	return SourceTypes.Filesystem
}

func (r SourceRecipe) IsValid() error {
	if r.GetSourceType() != SourceTypes.Remote {
		if r.Ref != "" {
			return fmt.Errorf("source %q: ref %s needs a remote %q source", r.URI, r.Ref, RemoteSrcPrefix)
		}
		if IsSpecFile(r.URI) {
			return fmt.Errorf("source %q names a spec file: name the dir, put a file path under the spec key", r.URI)
		}
		return nil
	}
	rest := RemoteSrcRef(r.URI)
	if rest == "" {
		return fmt.Errorf("source %q: empty git url after %q", r.URI, RemoteSrcPrefix)
	}
	if _, sub := splitRepoSubdir(rest); sub != "" {
		return fmt.Errorf("source %q: //<path> is gone, put the dir or spec file path under the spec key", r.URI)
	}
	return nil
}

// [why] a source resolves to a dir and the ordered spec candidates it offers: an explicit spec file
// is the one candidate, a dir (the source itself, or its spec: subdir) offers che.export.yml first
func (r SourceRecipe) prepare(repoRoot, home string, invoked bool) (SourceReady, error) {
	if err := r.IsValid(); err != nil {
		return SourceReady{}, err
	}
	root, err := r.resolveDir(repoRoot, home)
	if err != nil {
		return SourceReady{}, err
	}
	if invoked {
		def, ok := CheFile(root, SpecFileName)
		if !ok {
			return SourceReady{}, fmt.Errorf("spec not found: want %s", CheFileCandidates(root, SpecFileName))
		}
		return SourceReady{DefinitionURI: def, Candidates: []string{def}, DirectoryPath: root}, nil
	}
	if IsSpecFile(r.Spec) {
		def := filepath.Join(root, r.Spec)
		if _, err := os.Stat(def); err != nil {
			return SourceReady{}, fmt.Errorf("spec %s not found at %s (source %q)", r.Spec, root, r.URI)
		}
		return SourceReady{DefinitionURI: def, Candidates: []string{def}, DirectoryPath: filepath.Dir(def)}, nil
	}
	dir := filepath.Join(root, r.Spec)
	if !fsutil.IsDir(dir) {
		return SourceReady{}, fmt.Errorf("spec dir %s not found at %s (source %q)", r.Spec, root, r.URI)
	}
	candidates, err := SpecCandidates(dir, r.GetSourceType() == SourceTypes.Remote)
	if err != nil {
		return SourceReady{}, fmt.Errorf("source %q: %w", r.URI, err)
	}
	return SourceReady{DefinitionURI: candidates[0], Candidates: candidates, DirectoryPath: dir}, nil
}

func (r SourceRecipe) resolveDir(repoRoot, home string) (string, error) {
	if r.GetSourceType() == SourceTypes.Remote {
		return source.EnsureCheckout(home, RemoteSrcRef(r.URI), r.Ref)
	}
	if r.URI == "" {
		if r.DirectoryPath != "" {
			return expandDir(r.DirectoryPath, repoRoot, home), nil
		}
		return repoRoot, nil
	}
	dir := expandDir(r.URI, repoRoot, home)
	if !fsutil.IsDir(dir) {
		return "", fmt.Errorf("source dir not found: %s (from %q)", dir, r.URI)
	}
	return dir, nil
}

func splitRepoSubdir(ref string) (string, string) {
	start := 0
	if i := strings.Index(ref, "://"); i >= 0 {
		start = i + 3
	}
	if j := strings.Index(ref[start:], "//"); j >= 0 {
		return ref[:start+j], ref[start+j+2:]
	}
	return ref, ""
}

func expandDir(ref, repoRoot, home string) string {
	dir := fsutil.ExpandHome(os.ExpandEnv(ref), home)
	if dir == "~" {
		dir = home
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repoRoot, dir)
	}
	return dir
}

// PrepareSource resolves an included or invoked spec source. invoked: the spec che runs, che.yml only.
func (r SpecSourceRecipe) PrepareSource(repoRoot, home string, invoked bool) (SpecSourceReady, error) {
	ready, err := r.prepare(repoRoot, home, invoked)
	return SpecSourceReady{ready}, err
}

func (r ProfileSourceRecipe) PrepareSource(repoRoot, home string) (ProfileSourceReady, error) {
	ready, err := r.prepare(repoRoot, home, false)
	return ProfileSourceReady{SourceReady: ready, ProfileName: r.ProfileName}, err
}

func (r ProfileSourceRecipe) GetProfileName() string { return r.ProfileName }

func (r ProfileSourceReady) GetProfileName() string { return r.ProfileName }

// AsSpecSource turns a profile ref into the spec source it targets.
func (r ProfileSourceRecipe) AsSpecSource() SpecSourceRecipe {
	return SpecSourceRecipe{SourceRecipe: r.SourceRecipe, Env: r.Env, Variables: r.Variables}
}

func (r ProfileSourceRecipe) String() string {
	if r.URI == "" {
		return r.ProfileName
	}
	out := r.URI
	if r.Ref != "" {
		out += "@" + r.Ref
	}
	if r.Spec != "" {
		out += " spec " + r.Spec
	}
	return out + " profile " + r.ProfileName
}

func (r ProfileSourceRecipe) DisplayRef() string {
	if r.URI == "" {
		return r.ProfileName
	}
	if IsRemoteSrc(r.URI) {
		at := ""
		if r.Ref != "" {
			at = "@" + r.Ref
		}
		return "remote:" + repoName(RemoteSrcRef(r.URI)) + at + ":" + r.ProfileName
	}
	return strings.TrimSuffix(r.URI+"/"+r.Spec, "/") + "::" + r.ProfileName
}

func repoName(url string) string {
	s := strings.TrimSuffix(strings.TrimSuffix(url, "/"), ".git")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

func (i *profileListItem) UnmarshalYAML(value *yaml.Node) error {
	type alias profileListItem
	if err := decodeScalarOr(value, &i.Profile, (*alias)(i)); err != nil {
		return err
	}
	if i.Profile == "" {
		return fmt.Errorf("include.profiles entry profiles item: missing profile")
	}
	return nil
}

// Expand turns a multi-profile entry into one ref per listed profile, source, ref, options, env
// and variables shared; a single-profile entry returns itself.
func (r ProfileSourceRecipe) Expand() []ProfileSourceRecipe {
	if len(r.Profiles) == 0 {
		return []ProfileSourceRecipe{r}
	}
	out := make([]ProfileSourceRecipe, 0, len(r.Profiles))
	for _, item := range r.Profiles {
		one := r
		one.Profiles = nil
		one.ProfileName = item.Profile
		one.Spec = cmp.Or(item.Spec, r.Spec)
		out = append(out, one)
	}
	return out
}

func (r *SpecSourceRecipe) UnmarshalYAML(value *yaml.Node) error {
	var scalar string
	type alias SpecSourceRecipe
	if err := decodeScalarOr(value, &scalar, (*alias)(r)); err != nil {
		return err
	}
	src := cmp.Or(scalar, r.Src)
	if src == "" {
		return fmt.Errorf("include.sources entry missing source")
	}
	uri, gitRef, err := cutSourceRef(src)
	if err != nil {
		return err
	}
	r.URI, r.Ref, r.Spec, r.Src, r.SpecPath = uri, gitRef, r.SpecPath, "", ""
	if err := validateKeys("include.sources "+src+" variables", r.Variables); err != nil {
		return err
	}
	return r.IsValid()
}

func (r *ProfileSourceRecipe) UnmarshalYAML(value *yaml.Node) error {
	var scalar string
	type alias ProfileSourceRecipe
	if err := decodeScalarOr(value, &scalar, (*alias)(r)); err != nil {
		return err
	}
	if legacy := ctxAlias(value, "include.profiles entry"); legacy != nil {
		r.Variables = mergeEnv(legacy, r.Variables)
	}
	if scalar != "" && !isOldRefForm(scalar) {
		r.ProfileName = scalar
		return nil
	}
	src := cmp.Or(scalar, r.Src)
	if src == "" {
		return fmt.Errorf("include.profiles entry missing source")
	}
	if isOldRefForm(src) {
		// [why] pinned upstream tags still carry the URL form: refusing it refuses every consumer until each
		// upstream re-tags, so it decodes to the keyed form and warns until every pin has moved
		uri, specPath, profile, err := splitOldRefForm(src)
		if err != nil {
			return err
		}
		log.EmitWarn("load-spec", "deprecated", fmt.Sprintf("include.profiles entry %q: <source>/<spec-file>::<profile> is gone, write {source: %q, spec: %q, profile: %q}", src, uri, specPath, profile))
		r.Src, r.SpecPath, r.Profile = uri, specPath, profile
	}
	if r.Profile == "" && len(r.Profiles) == 0 {
		return fmt.Errorf("include.profiles entry %q: missing profile (or profiles)", r.Src)
	}
	if r.Profile != "" && len(r.Profiles) > 0 {
		return fmt.Errorf("include.profiles entry %q: profile and profiles are exclusive", r.Src)
	}
	uri, gitRef, err := cutSourceRef(r.Src)
	if err != nil {
		return err
	}
	r.URI, r.Ref, r.Spec, r.ProfileName, r.Src, r.SpecPath, r.Profile = uri, gitRef, r.SpecPath, r.Profile, "", "", ""
	if err := r.IsValid(); err != nil {
		return err
	}
	return validateKeys("include.profiles "+r.String()+" variables", r.Variables)
}

func isOldRefForm(src string) bool {
	return strings.Contains(strings.TrimPrefix(src, RemoteSrcPrefix), "::")
}

func splitOldRefForm(src string) (uri, specPath, profile string, err error) {
	ref, profile, _ := strings.Cut(strings.TrimPrefix(src, RemoteSrcPrefix), "::")
	if strings.HasPrefix(src, RemoteSrcPrefix) {
		ref = RemoteSrcPrefix + ref
	}
	if profile == "" {
		return "", "", "", fmt.Errorf("include.profiles entry %q: missing profile name", src)
	}
	slash := strings.LastIndex(ref, "/")
	if slash <= 0 || !IsSpecFile(ref[slash+1:]) {
		return "", "", "", fmt.Errorf("include.profiles entry %q: needs {source, spec, profile}", src)
	}
	dir := strings.TrimSuffix(ref[:slash], "/")
	if strings.HasPrefix(dir, RemoteSrcPrefix) {
		repo, sub := splitRepoSubdir(RemoteSrcRef(dir))
		return RemoteSrcPrefix + repo, sub, profile, nil
	}
	return dir, "", profile, nil
}

// [why] pinned remote specs still carry ctx: a hard error would refuse every consumer until each
// upstream re-tags, so the old key decodes as variables and warns
func ctxAlias(value *yaml.Node, where string) map[string]string {
	if value.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		if value.Content[i].Value != "ctx" {
			continue
		}
		var legacy map[string]string
		if err := value.Content[i+1].Decode(&legacy); err != nil {
			return nil
		}
		log.EmitWarn("load-spec", "deprecated", where+": ctx is renamed to variables, decoded as variables")
		return legacy
	}
	return nil
}

func cutSourceRef(src string) (rest, gitRef string, err error) {
	if strings.Contains(src, "?ref=") {
		return "", "", fmt.Errorf("source %q: ?ref= is gone, pin with %s<repo>@<ref>", src, RemoteSrcPrefix)
	}
	marker, ok := render.CutGitMarker(src)
	if !ok {
		return src, "", nil
	}
	rest, gitRef, err = render.CutRefSuffix(marker)
	if err != nil {
		return "", "", err
	}
	return RemoteSrcPrefix + rest, gitRef, nil
}

// [<] 🤖🤖
