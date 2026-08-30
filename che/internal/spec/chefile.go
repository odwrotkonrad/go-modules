package spec

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec/envinterp"
)

const (
	SpecFileName              = "che.yml"
	ExportFileName            = "che.export.yml"
	VariablesFileName         = "cheVariables.yml"
	VariablesDefaultsFileName = "cheVariables.defaults.yml"
	VariablesLocalFileName    = "cheVariables.local.yml"
	legacyVariablesFileName   = "che.variables.yml"
	CheEnvFileName            = "che.env"
	CheDir                    = ".che"
)

// RootSpecPaths lists the invoked spec's files: che.yml and .che/che.yml, both when both exist,
// loaded as one spec.
func RootSpecPaths(dir string) []string {
	var out []string
	for _, path := range []string{filepath.Join(dir, SpecFileName), filepath.Join(dir, CheDir, SpecFileName)} {
		if _, err := os.Stat(path); err == nil {
			out = append(out, path)
		}
	}
	return out
}

// CheFile resolves a che file under dir: <dir>/<name> first, <dir>/.che/<name> second.
func CheFile(dir, name string) (string, bool) {
	atRoot := filepath.Join(dir, name)
	nested := filepath.Join(dir, CheDir, name)
	_, rootErr := os.Stat(atRoot)
	_, nestedErr := os.Stat(nested)
	switch {
	case rootErr == nil && nestedErr == nil:
		log.EmitDebug("discover-profiles", "resolve-che-file", atRoot+" wins over "+nested)
		return atRoot, true
	case rootErr == nil:
		return atRoot, true
	case nestedErr == nil:
		return nested, true
	}
	return "", false
}

// SpecCandidates lists the spec files a source dir offers, search order first: che.export.yml at the
// dir, then .che/che.export.yml (warned: hidden dir), else che.yml at the dir or .che/che.yml.
// Several plain che.yml and no export errors. remote adds the not-for-reuse warning on a plain che.yml.
func SpecCandidates(dir string, remote bool) ([]string, error) {
	exportRoot := filepath.Join(dir, ExportFileName)
	exportNested := filepath.Join(dir, CheDir, ExportFileName)
	var exports []string
	for _, path := range []string{exportRoot, exportNested} {
		if _, err := os.Stat(path); err == nil {
			exports = append(exports, path)
		}
	}
	if len(exports) > 0 {
		if slices.Contains(exports, exportNested) {
			log.EmitWarn("load-spec", "not-recommended", exportNested+": "+ExportFileName+" sits in a hidden dir, keep it at the config root")
		}
		return exports, nil
	}
	plainRoot := filepath.Join(dir, SpecFileName)
	plainNested := filepath.Join(dir, CheDir, SpecFileName)
	var plains []string
	for _, path := range []string{plainRoot, plainNested} {
		if _, err := os.Stat(path); err == nil {
			plains = append(plains, path)
		}
	}
	switch len(plains) {
	case 0:
		return nil, fmt.Errorf("no spec at %s: want %s, or %s", dir, ExportFileName, CheFileCandidates(dir, SpecFileName))
	case 1:
		if remote {
			log.EmitWarn("load-spec", "not-recommended", plains[0]+": a spec not designed for reuse, consume a "+ExportFileName)
		}
		return plains, nil
	}
	return nil, fmt.Errorf("%s holds several %s and no %s: export the reusable one", dir, SpecFileName, ExportFileName)
}

// IsSpecFile reports whether path names a spec file rather than a dir.
func IsSpecFile(path string) bool {
	return strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")
}

// CheFileCandidates lists both lookup paths of a che file, for error text.
func CheFileCandidates(dir, name string) string {
	return filepath.Join(dir, name) + " or " + filepath.Join(dir, CheDir, name)
}

// RepoFiles is what one spec declares beside itself: che.env (env for its specs and every spec they
// include, read at the git root) and the three cheVariables files, read at the spec's own dir (a
// che.export.yml strictly beside itself, a che.yml at its dir or under its .che/), values expanded.
type RepoFiles struct {
	Root     string
	SpecDir  string
	Env      map[string]string
	Defaults VarSet
	Shared   VarSet
	Local    VarSet
}

// IsExportSpec reports whether path names a che.export.yml.
func IsExportSpec(path string) bool { return filepath.Base(path) == ExportFileName }

// LoadRepoFiles reads che.env under root and the cheVariables files at specDir (beside an export,
// else at specDir or under its .che/), values interpolating ${{ env.X }} from process.
func LoadRepoFiles(specDir, root string, export bool, process map[string]string) (RepoFiles, error) {
	files := RepoFiles{Root: root, SpecDir: specDir}
	lookup := func(name string) (string, bool) {
		if !export {
			return CheFile(specDir, name)
		}
		path := filepath.Join(specDir, name)
		_, err := os.Stat(path)
		return path, err == nil
	}
	if path, ok := lookup(legacyVariablesFileName); ok {
		return files, fmt.Errorf("%s is no longer read, rename to %s", path, VariablesFileName)
	}
	var err error
	if files.Defaults, err = loadVarsFileAt(lookup, VariablesDefaultsFileName, process); err != nil {
		return files, err
	}
	if files.Shared, err = loadVarsFileAt(lookup, VariablesFileName, process); err != nil {
		return files, err
	}
	if files.Local, err = loadVarsFileAt(lookup, VariablesLocalFileName, process); err != nil {
		return files, err
	}
	if files.Env, err = loadEnvFile(root, process); err != nil {
		return files, err
	}
	return files, nil
}

func loadVarsFileAt(lookup func(string) (string, bool), name string, process map[string]string) (VarSet, error) {
	if path, ok := lookup(name); ok {
		log.EmitDebug("discover-profiles", "resolve-che-file", path)
		return loadVarsFile(path, process)
	}
	return nil, nil
}

type varsFileEntry struct {
	Value string   `yaml:"value"`
	Scope VarScope `yaml:"scope"`
}

func (e *varsFileEntry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		e.Value = node.Value
		return nil
	}
	type alias varsFileEntry
	if err := node.Decode((*alias)(e)); err != nil {
		return err
	}
	if e.Scope != "" && !slices.Contains(VarScopeNames, string(e.Scope)) {
		return fmt.Errorf("scope %q: want %s", e.Scope, strings.Join(VarScopeNames, " | "))
	}
	return nil
}

// loadVarsFile reads one cheVariables file: KEY: value or KEY: {value, scope}, ${{ env.X }} expanded.
func loadVarsFile(path string, process map[string]string) (VarSet, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var raw map[string]varsFileEntry
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: want KEY: value or KEY: {value, scope}: %w", path, err)
	}
	values := make(map[string]string, len(raw))
	for key, entry := range raw {
		values[key] = entry.Value
	}
	if err := validateKeys(path, values); err != nil {
		return nil, err
	}
	expanded, err := expandValues(path, values, envinterp.MapLookup(process, nil, nil))
	if err != nil {
		return nil, err
	}
	out := make(VarSet, len(raw))
	for key, entry := range raw {
		out[key] = VarValue{Value: expanded[key], Scope: entry.Scope, Source: path}
	}
	return out, nil
}

func loadEnvFile(root string, process map[string]string) (map[string]string, error) {
	path, ok := CheFile(root, CheEnvFileName)
	if !ok {
		return nil, nil
	}
	raw, err := godotenv.Read(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return expandValues(path, raw, envinterp.MapLookup(process, nil, nil))
}

func expandValues(path string, raw map[string]string, lookup envinterp.Lookup) (map[string]string, error) {
	out := make(map[string]string, len(raw))
	var unset []EnvUnset
	for key, value := range raw {
		expanded, missing := envinterp.Expand(value, lookup)
		for _, ref := range missing {
			if ref.Namespace == envinterp.Namespaces.Var {
				unset = append(unset, EnvUnset{Name: ref.Name, Path: key})
			}
		}
		out[key] = expanded
	}
	if len(unset) > 0 {
		return nil, fmt.Errorf("%s: ${{ var.* }} is not available in repo files, values interpolate ${{ env.* }} only: %s", path, unset[0].Name)
	}
	return out, nil
}

// [<] 🤖🤖
