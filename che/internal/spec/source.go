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
)

// GetSourceType classifies the URI: @-prefixed -> remote, else filesystem
// (local "" included).
func (r SourceRecipe) GetSourceType() SourceType {
	if IsRemoteSrc(r.URI) {
		return SourceTypes.Remote
	}
	return SourceTypes.Filesystem
}

// IsValid rejects malformed URIs: remote needs a non-empty git URL after the
// marker; "" (local) and path shapes pass.
func (r SourceRecipe) IsValid() error {
	if r.GetSourceType() == SourceTypes.Remote && RemoteSrcRef(r.URI) == "" {
		return fmt.Errorf("source %q: empty git url after %q", r.URI, RemoteSrcPrefix)
	}
	return nil
}

// prepare resolves the source into its local directory (remote: clone/pull
// the cache checkout; filesystem: expand ~/, $VAR, relative -> repoRoot;
// "" local -> DirectoryPath or repoRoot) and locates its che.yml.
func (r SourceRecipe) prepare(repoRoot, home, name string) (SourceReady, error) {
	if err := r.IsValid(); err != nil {
		return SourceReady{}, err
	}
	dir, err := r.resolveDir(repoRoot, home, name)
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

func (r SourceRecipe) resolveDir(repoRoot, home, name string) (string, error) {
	if r.GetSourceType() == SourceTypes.Remote {
		return source.EnsureCheckout(home, RemoteSrcRef(r.URI), name)
	}
	if r.URI == "" {
		if r.DirectoryPath != "" {
			return expandDir(r.DirectoryPath, repoRoot, home)
		}
		return repoRoot, nil
	}
	dir, err := expandDir(r.URI, repoRoot, home)
	if err != nil {
		return "", err
	}
	if !fsutil.IsDir(dir) {
		return "", fmt.Errorf("source dir not found: %s (from %q)", dir, r.URI)
	}
	return dir, nil
}

// expandDir expands ~/, $VAR and anchors relative paths at repoRoot.
func expandDir(ref, repoRoot, home string) (string, error) {
	dir := fsutil.ExpandHome(os.ExpandEnv(ref), home)
	if dir == "~" {
		dir = home
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repoRoot, dir)
	}
	return dir, nil
}

// PrepareSource resolves the spec source: isValid, then resolve into
// DirectoryPath; the target must hold a valid che.yml.
func (r SpecSourceRecipe) PrepareSource(repoRoot, home string) (SpecSourceReady, error) {
	ready, err := r.prepare(repoRoot, home, "spec")
	return SpecSourceReady{ready}, err
}

// PrepareSource resolves the containing spec of the referenced profile.
func (r ProfileSourceRecipe) PrepareSource(repoRoot, home string) (ProfileSourceReady, error) {
	ready, err := r.prepare(repoRoot, home, r.ProfileName)
	return ProfileSourceReady{SourceReady: ready, ProfileName: r.ProfileName}, err
}

// GetProfileName is the one accessor for a profile's name.
func (r ProfileSourceRecipe) GetProfileName() string { return r.ProfileName }

// GetProfileName is the one accessor for a profile's name.
func (r ProfileSourceReady) GetProfileName() string { return r.ProfileName }

// String renders the canonical ref form: bare name local, else
// <source>/<spec-file>::<name> (spec file defaults to che.yml).
func (r ProfileSourceRecipe) String() string {
	if r.URI == "" {
		return r.ProfileName
	}
	return r.URI + "/" + cmp.Or(r.SpecFile, "che.yml") + "::" + r.ProfileName
}

// UnmarshalYAML decodes an include.profiles entry: a scalar is a local profile
// name; an object's `source` is `<source>/<spec-file>::<profile>` (bare
// `<profile>` -> local) with `options`/`env` alongside. Splits source into URI
// + SpecFile + ProfileName; env is allowed on sourced entries only.
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
	uri, specFile, profile, err := splitSourceRef(src)
	if err != nil {
		return err
	}
	r.URI, r.SpecFile, r.ProfileName = uri, specFile, profile
	r.Src = ""
	if r.URI == "" && len(r.Env) > 0 {
		return fmt.Errorf("include.profiles entry %q: env requires a source", r.ProfileName)
	}
	return nil
}

// splitSourceRef splits `<source>/<spec-file>::<profile>` into its parts. No
// `::` -> a bare local profile name (empty URI + spec file). Otherwise the last
// `/`-segment before `::` is the spec file (must end in .yml, distinguishing it
// from a `.git` source suffix), and the rest is the source.
func splitSourceRef(src string) (uri, specFile, profile string, err error) {
	i := strings.LastIndex(src, "::")
	if i < 0 {
		return "", "", src, nil
	}
	ref, profile := src[:i], src[i+2:]
	if profile == "" {
		return "", "", "", fmt.Errorf("include.profiles source %q: missing profile name", src)
	}
	// [why] split on the last '/' by hand: path.Dir collapses the // in
	// file:// / https:// URLs.
	slash := strings.LastIndex(ref, "/")
	if slash <= 0 {
		return "", "", "", fmt.Errorf("include.profiles source %q: needs a <source>/<spec-file>.yml::<profile> path", src)
	}
	dir, file := ref[:slash], ref[slash+1:]
	if !strings.HasSuffix(file, ".yml") {
		return "", "", "", fmt.Errorf("include.profiles source %q: needs a <source>/<spec-file>.yml::<profile> path", src)
	}
	return dir, file, profile, nil
}

// [<] 🤖🤖
