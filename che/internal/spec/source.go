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
		return nil
	}
	if RemoteSrcRef(r.URI) == "" {
		return fmt.Errorf("source %q: empty git url after %q", r.URI, RemoteSrcPrefix)
	}
	return nil
}

func (r SourceRecipe) prepare(repoRoot, home string) (SourceReady, error) {
	if err := r.IsValid(); err != nil {
		return SourceReady{}, err
	}
	dir, err := r.resolveDir(repoRoot, home)
	if err != nil {
		return SourceReady{}, err
	}
	specFile := cmp.Or(r.SpecFile, "che.yml")
	def := filepath.Join(dir, specFile)
	if _, err := os.Stat(def); err != nil {
		return SourceReady{}, fmt.Errorf("%s not found at %s (source %q)", specFile, dir, r.URI)
	}
	return SourceReady{DefinitionURI: def, DirectoryPath: dir}, nil
}

func (r SourceRecipe) resolveDir(repoRoot, home string) (string, error) {
	if r.GetSourceType() == SourceTypes.Remote {
		// [why] @<repo>//<subdir> targets a spec nested in the remote checkout
		repo, sub := splitRepoSubdir(RemoteSrcRef(r.URI))
		dir, err := source.EnsureCheckout(home, repo, r.Ref)
		if err != nil || sub == "" {
			return dir, err
		}
		return filepath.Join(dir, sub), nil
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

func (r SpecSourceRecipe) PrepareSource(repoRoot, home string) (SpecSourceReady, error) {
	ready, err := r.prepare(repoRoot, home)
	return SpecSourceReady{ready}, err
}

func (r ProfileSourceRecipe) PrepareSource(repoRoot, home string) (ProfileSourceReady, error) {
	ready, err := r.prepare(repoRoot, home)
	return ProfileSourceReady{SourceReady: ready, ProfileName: r.ProfileName}, err
}

func (r ProfileSourceRecipe) GetProfileName() string { return r.ProfileName }

func (r ProfileSourceReady) GetProfileName() string { return r.ProfileName }

func (r ProfileSourceRecipe) String() string {
	if r.URI == "" {
		return r.ProfileName
	}
	return withRefSuffix(r.URI, r.refSuffix()) + "/" + cmp.Or(r.SpecFile, "che.yml") + "::" + r.ProfileName
}

// [why] the ref sits in the repo position: before the //, after any scp user
func withRefSuffix(uri, suffix string) string {
	if suffix == "" {
		return uri
	}
	repo, sub := splitRepoSubdir(uri)
	if sub == "" {
		return repo + suffix
	}
	return repo + suffix + "//" + sub
}

func (r ProfileSourceRecipe) refSuffix() string {
	if r.Ref == "" {
		return ""
	}
	return "@" + r.Ref
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
	return r.String()
}

func repoName(url string) string {
	s := strings.TrimSuffix(strings.TrimSuffix(url, "/"), ".git")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

func (r *ProfileSourceRecipe) UnmarshalYAML(value *yaml.Node) error {
	var scalar string
	type alias ProfileSourceRecipe
	if err := decodeScalarOr(value, &scalar, (*alias)(r)); err != nil {
		return err
	}
	src := cmp.Or(scalar, r.Src)
	if src == "" {
		return fmt.Errorf("include.profiles entry missing source")
	}
	uri, specFile, profile, gitRef, err := splitSourceRef(src)
	if err != nil {
		return err
	}
	r.URI, r.SpecFile, r.ProfileName, r.Ref = uri, specFile, profile, gitRef
	r.Src = ""
	if err := r.IsValid(); err != nil {
		return err
	}
	if r.URI == "" && len(r.Env) > 0 {
		return fmt.Errorf("include.profiles entry %q: env requires a source", r.ProfileName)
	}
	return nil
}

func splitSourceRef(src string) (uri, specFile, profile, gitRef string, err error) {
	// [why] the ref rides the repo, never the profile name: cut it before the :: split
	src, gitRef, err = cutSourceRef(src)
	if err != nil {
		return "", "", "", "", err
	}
	i := strings.LastIndex(src, "::")
	if i < 0 {
		return "", "", src, gitRef, nil
	}
	ref, profile := src[:i], src[i+2:]
	if profile == "" {
		return "", "", "", "", fmt.Errorf("include.profiles source %q: missing profile name", src)
	}
	// [why] split on the last '/' by hand: path.Dir collapses the // in remote refs
	slash := strings.LastIndex(ref, "/")
	if slash <= 0 {
		return "", "", "", "", fmt.Errorf("include.profiles source %q: needs a <source>/<spec-file>.yml::<profile> path", src)
	}
	// [why] a spec file directly under the repo leaves the '//' separator's second slash on the dir
	dir, file := strings.TrimSuffix(ref[:slash], "/"), ref[slash+1:]
	if !strings.HasSuffix(file, ".yml") {
		return "", "", "", "", fmt.Errorf("include.profiles source %q: needs a <source>/<spec-file>.yml::<profile> path", src)
	}
	return dir, file, profile, gitRef, nil
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
