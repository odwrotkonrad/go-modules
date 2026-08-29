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
	SpecFileName      = "che.yml"
	ExportFileName    = "che.export.yml"
	VariablesFileName = "che.variables.yml"
	CheEnvFileName    = "che.env"
	CheDir            = ".che"
)

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

// RepoFiles is what one repo declares beside its spec: che.env (implicit variables and env for its
// specs and every spec they include) and che.variables.yml (explicit variables), both expanded.
type RepoFiles struct {
	Root      string
	Env       map[string]string
	Variables map[string]string
}

// LoadRepoFiles reads che.variables.yml and che.env under root, values interpolating ${{ env.X }} from process.
func LoadRepoFiles(root string, process map[string]string) (RepoFiles, error) {
	vars, err := loadVariablesFile(root, process)
	if err != nil {
		return RepoFiles{}, err
	}
	env, err := loadEnvFile(root, process)
	if err != nil {
		return RepoFiles{}, err
	}
	return RepoFiles{Root: root, Env: env, Variables: vars}, nil
}

func loadVariablesFile(root string, process map[string]string) (map[string]string, error) {
	path, ok := CheFile(root, VariablesFileName)
	if !ok {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var raw map[string]string
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: want a flat KEY: value map: %w", path, err)
	}
	if err := validateKeys(path, raw); err != nil {
		return nil, err
	}
	return expandValues(path, raw, envinterp.MapLookup(process, nil))
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
	return expandValues(path, raw, envinterp.MapLookup(process, nil))
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
