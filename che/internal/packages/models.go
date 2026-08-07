package packages

// [>] 🤖🤖🤖

import (
	"embed"
	"fmt"
	"maps"
	"os"

	"gopkg.in/yaml.v3"
)

//go:embed packages.yml
var builtinPackages []byte

//go:embed all:scripts
var builtinScripts embed.FS

const BuiltinPath = "builtin packages.yml"

func LoadBuiltin() (*File, error) {
	f := &File{}
	if err := yaml.Unmarshal(builtinPackages, f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", BuiltinPath, err)
	}
	return f, nil
}

type File struct {
	Packages map[string]Entry `yaml:"packages"`
}

type Entry struct {
	Items       []Item
	Command     string
	Completions Completions
}

type Completions struct {
	Zsh *CompletionFile `yaml:"zsh"`
}

type CompletionFile struct {
	Name   string `yaml:"name"`
	Cmd    string `yaml:"cmd"`
	URL    string `yaml:"url"`
	Sha256 string `yaml:"sha256"`
}

func (e *Entry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.SequenceNode {
		return node.Decode(&e.Items)
	}
	var obj struct {
		Managers    []Item      `yaml:"installMethods"`
		Command     string      `yaml:"command"`
		Completions Completions `yaml:"completions"`
	}
	if err := node.Decode(&obj); err != nil {
		return err
	}
	if len(obj.Managers) == 0 && obj.Completions.Zsh == nil {
		return fmt.Errorf("package entry object form requires installMethods or completions")
	}
	if z := obj.Completions.Zsh; z != nil && (z.Cmd == "") == (z.URL == "") {
		return fmt.Errorf("completions.zsh requires exactly one of cmd or url")
	}
	e.Items = obj.Managers
	e.Command = obj.Command
	e.Completions = obj.Completions
	return nil
}

type Item struct {
	Mgr             string
	Name            string
	PrebuiltArchive *PrebuiltArchiveSpec
	Script          *ScriptSpec
	Pkg             *PkgSpec
	Apt             *AptSpec
}

type AptSpec struct {
	Packages []string     `yaml:"packages"`
	Repo     *AptRepoSpec `yaml:"repo"`
}

type AptRepoSpec struct {
	URL        string `yaml:"url"`
	GpgURL     string `yaml:"gpgUrl"`
	Suites     string `yaml:"suites"`
	Components string `yaml:"components"`
}

type PkgSpec struct {
	Version string            `yaml:"version"`
	URL     string            `yaml:"url"`
	Sha256  map[string]string `yaml:"sha256"`
}

type ScriptSpec struct {
	Run     string            `yaml:"run"`
	Path    string            `yaml:"path"`
	URL     string            `yaml:"remoteUrl"`
	OS      string            `yaml:"os"`
	Version string            `yaml:"version"`
	Sha256  map[string]string `yaml:"sha256"`
}

type PrebuiltArchiveSpec struct {
	Version string            `yaml:"version"`
	URL     string            `yaml:"url"`
	Bin     string            `yaml:"bin"`
	Sha256  map[string]string `yaml:"sha256"`
}

func (it *Item) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		it.Mgr = node.Value
		return nil
	}
	if node.Kind != yaml.MappingNode || len(node.Content) != 2 {
		return fmt.Errorf("manager item must be a scalar or a single-key map")
	}
	key, val := node.Content[0], node.Content[1]
	if key.Value == "prebuiltArchive" {
		it.Mgr = "prebuiltArchive"
		it.PrebuiltArchive = &PrebuiltArchiveSpec{}
		return val.Decode(it.PrebuiltArchive)
	}
	if key.Value == "pkg" {
		it.Mgr = "pkg"
		it.Pkg = &PkgSpec{}
		return val.Decode(it.Pkg)
	}
	if key.Value == "apt" && val.Kind == yaml.MappingNode {
		it.Mgr = "apt"
		it.Apt = &AptSpec{}
		return val.Decode(it.Apt)
	}
	if key.Value == "script" {
		it.Mgr = "script"
		it.Script = &ScriptSpec{}
		if val.Kind == yaml.ScalarNode {
			it.Script.Run = val.Value
			return nil
		}
		return val.Decode(it.Script)
	}
	if key.Value == "brew" && val.Kind == yaml.MappingNode {
		var cask struct {
			Cask string `yaml:"cask"`
		}
		if err := val.Decode(&cask); err != nil || cask.Cask == "" {
			return fmt.Errorf("brew object form must be {cask: <name>}")
		}
		it.Mgr, it.Name = "cask", cask.Cask
		return nil
	}
	it.Mgr = key.Value
	return val.Decode(&it.Name)
}

func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("packages file not found: %s", path)
	}
	f := &File{}
	if err := yaml.Unmarshal(b, f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return f, nil
}

func (f *File) Merge(override *File) {
	if override == nil {
		return
	}
	if f.Packages == nil {
		f.Packages = map[string]Entry{}
	}
	maps.Copy(f.Packages, override.Packages)
}

func (f *File) Find(pkg, path string) (Entry, error) {
	e, ok := f.Packages[pkg]
	if !ok {
		return Entry{}, fmt.Errorf("unknown package: %s (add it to %s)", pkg, path)
	}
	return e, nil
}

// [<] 🤖🤖🤖
