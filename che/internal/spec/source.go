package spec

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"

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
	def := filepath.Join(dir, "che.yml")
	if _, err := os.Stat(def); err != nil {
		return SourceReady{}, fmt.Errorf("che.yml not found at %s (source %q)", dir, r.URI)
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

// String renders the canonical ref form: bare name local, <source>::<name> sourced.
func (r ProfileSourceRecipe) String() string {
	if r.URI == "" {
		return r.ProfileName
	}
	return r.URI + "::" + r.ProfileName
}

// UnmarshalYAML decodes an include.profiles entry: scalar -> local profile
// name; {ref, options, env} object -> sourced ref (options.source lifts to
// URI; env allowed on sourced entries only).
func (r *ProfileSourceRecipe) UnmarshalYAML(value *yaml.Node) error {
	type alias ProfileSourceRecipe
	if err := decodeScalarOr(value, &r.ProfileName, (*alias)(r)); err != nil {
		return err
	}
	if r.ProfileName == "" {
		return fmt.Errorf("include.profiles entry missing ref")
	}
	r.URI = r.Options.Source
	r.Options.Source = ""
	if r.URI == "" && len(r.Env) > 0 {
		return fmt.Errorf("include.profiles entry %q: env requires options.source", r.ProfileName)
	}
	return nil
}

// [<] 🤖🤖
