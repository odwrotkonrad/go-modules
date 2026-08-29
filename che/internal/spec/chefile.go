package spec

// [>] 🤖🤖

import (
	"fmt"
	"os"
	"path/filepath"

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

// SpecFile resolves the spec of a source dir: an explicit file as given (che.yml still root-first
// then .che/), none given: che.export.yml first, then che.yml root-first then .che/.
func SpecFile(dir, specFile string) (string, bool) {
	switch specFile {
	case "":
		if path, ok := CheFile(dir, ExportFileName); ok {
			return path, true
		}
		return CheFile(dir, SpecFileName)
	case SpecFileName:
		return CheFile(dir, specFile)
	}
	path := filepath.Join(dir, specFile)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

// IsExportSpec reports whether path is a spec designed for reuse, che.export.yml.
func IsExportSpec(path string) bool {
	return filepath.Base(path) == ExportFileName
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
