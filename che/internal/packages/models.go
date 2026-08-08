package packages

// [>] 🤖🤖🤖

import (
	"embed"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
)

//go:embed packages.yml
var builtinPackages []byte

//go:embed all:scripts
var builtinScripts embed.FS

const BuiltinPath = "builtin packages.yml"

const BuiltinSentinel = "__builtin__.packages.yml"

func LoadBuiltin() (*File, error) {
	f := &File{}
	if err := yaml.Unmarshal(builtinPackages, f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", BuiltinPath, err)
	}
	return f, nil
}

type File struct {
	ArchNameConventions map[string]map[string]string `yaml:"archNameConventions,omitempty"`
	Platforms           map[string][]string          `yaml:"platforms,omitempty"`
	BasePackages        map[string][]string          `yaml:"basePackages,omitempty"`
	Packages            map[string]Entry             `yaml:"packages,omitempty"`
}

var builtinFile = sync.OnceValues(LoadBuiltin)

func (f *File) archNameConventionsOrBuiltin() map[string]map[string]string {
	if f.ArchNameConventions != nil {
		return f.ArchNameConventions
	}
	if builtin, err := builtinFile(); err == nil {
		return builtin.ArchNameConventions
	}
	return nil
}

func (f *File) platformsOrBuiltin() map[string][]string {
	if f.Platforms != nil {
		return f.Platforms
	}
	if builtin, err := builtinFile(); err == nil {
		return builtin.Platforms
	}
	return nil
}

const BaseCommon = "common"

type Entry struct {
	Items          []Item
	Version        string
	Requires       []string
	Command        string
	PostInstall    *ScriptSpec
	VersionCommand string
	Completions    Completions
}

type Completions struct {
	Zsh *CompletionFile `yaml:"zsh,omitempty"`
}

type CompletionFile struct {
	Name   string `yaml:"name,omitempty"`
	Cmd    string `yaml:"cmd,omitempty"`
	URL    string `yaml:"url,omitempty"`
	Sha256 string `yaml:"sha256,omitempty"`
}

func (it Item) MarshalYAML() (any, error) {
	switch {
	case it.PrebuiltBinariesArchive != nil:
		return map[string]*PrebuiltBinariesArchiveSpec{"prebuiltBinariesArchive": it.PrebuiltBinariesArchive}, nil
	case it.Script != nil:
		return map[string]*ScriptSpec{"script": it.Script}, nil
	case it.Apt != nil:
		return map[string]*AptSpec{"apt": it.Apt}, nil
	case it.VersionManager != nil:
		return map[string]*VersionManagerSpec{"versionManager": it.VersionManager}, nil
	case it.Name != "":
		switch it.Mgr {
		case "cask":
			return map[string]map[string]string{"brew": {"cask": it.Name}}, nil
		case "vscode":
			return map[string]map[string]string{"brew": {"vscode": it.Name}}, nil
		}
		return map[string]string{it.Mgr: it.Name}, nil
	default:
		return it.Mgr, nil
	}
}

func (e Entry) MarshalYAML() (any, error) {
	if e.Version == "" && len(e.Requires) == 0 && e.PostInstall == nil && e.Command == "" && e.VersionCommand == "" && e.Completions.Zsh == nil {
		return e.Items, nil
	}
	obj := map[string]any{"installMethods": e.Items}
	if e.Version != "" {
		obj["version"] = e.Version
	}
	if len(e.Requires) > 0 {
		obj["requires"] = e.Requires
	}
	if e.PostInstall != nil {
		obj["postInstall"] = e.PostInstall
	}
	if e.Command != "" {
		obj["command"] = e.Command
	}
	if e.VersionCommand != "" {
		obj["versionCommand"] = e.VersionCommand
	}
	if e.Completions.Zsh != nil {
		obj["completions"] = e.Completions
	}
	return obj, nil
}

func (f *File) YAML() (string, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, name := range slices.Sorted(maps.Keys(f.Packages)) {
		entry := &yaml.Node{}
		if err := entry.Encode(f.Packages[name]); err != nil {
			return "", err
		}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: name}, entry)
	}
	doc := &yaml.Node{Kind: yaml.MappingNode}
	for _, top := range []struct {
		key   string
		empty bool
		value any
	}{
		{"archNameConventions", len(f.ArchNameConventions) == 0, f.ArchNameConventions},
		{"platforms", len(f.Platforms) == 0, f.Platforms},
	} {
		if top.empty {
			continue
		}
		node := &yaml.Node{}
		if err := node.Encode(top.value); err != nil {
			return "", err
		}
		doc.Content = append(doc.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: top.key}, node)
	}
	doc.Content = append(doc.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "packages"}, root)
	b, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (f *File) Delta(base *File) *File {
	out := &File{Packages: map[string]Entry{}}
	for name, entry := range f.Packages {
		baseEntry, ok := base.Packages[name]
		if ok && entriesEqual(entry, baseEntry) {
			continue
		}
		out.Packages[name] = entry
	}
	return out
}

func entriesEqual(a, b Entry) bool {
	ay, aerr := yaml.Marshal(a)
	by, berr := yaml.Marshal(b)
	return aerr == nil && berr == nil && string(ay) == string(by)
}

func (e *Entry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.SequenceNode {
		return node.Decode(&e.Items)
	}
	var obj struct {
		Managers       []Item      `yaml:"installMethods"`
		Version        string      `yaml:"version"`
		Requires       []string    `yaml:"requires"`
		PostInstall    *ScriptSpec `yaml:"postInstall"`
		Command        string      `yaml:"command"`
		VersionCommand string      `yaml:"versionCommand"`
		Completions    Completions `yaml:"completions"`
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
	e.Version = obj.Version
	e.Requires = obj.Requires
	e.Command = obj.Command
	e.PostInstall = obj.PostInstall
	e.VersionCommand = obj.VersionCommand
	e.Completions = obj.Completions
	return nil
}

// Strings is a yaml list that also accepts a single scalar
type Strings []string

func (s *Strings) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*s = Strings{node.Value}
		return nil
	}
	var list []string
	if err := node.Decode(&list); err != nil {
		return err
	}
	*s = list
	return nil
}

type Item struct {
	Mgr                     string
	Name                    string
	AliasBinary             map[string]string
	PrebuiltBinariesArchive *PrebuiltBinariesArchiveSpec
	Script                  *ScriptSpec
	Apt                     *AptSpec
	VersionManager          *VersionManagerSpec
}

type VersionManagerSpec struct {
	Tool     string   `yaml:"tool"`
	Versions []string `yaml:"versions"`
	Global   string   `yaml:"global"`
}

type AptSpec struct {
	InstallPackages AptPackages       `yaml:"installPackages,omitempty"`
	Versions        map[string]string `yaml:"versions,omitempty"`
	FromSource      *AptRepoSpec      `yaml:"fromSource,omitempty"`
}

// AptPackages maps each debian package name to its attributes, preserving file order.
// [why] attributes belong to the package that needs them (a rename is one package's quirk),
//
//	and a bare list stays valid for the common attribute-less case
type AptPackages struct {
	Names []string
	Attrs map[string]AptPackage
}

type AptPackage struct {
	AliasBinary map[string]string `yaml:"aliasBinary,omitempty"`
}

func (a *AptPackages) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.SequenceNode {
		return node.Decode(&a.Names)
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("apt packages must be a list or a map of name to attributes")
	}
	a.Attrs = map[string]AptPackage{}
	for i := 0; i < len(node.Content); i += 2 {
		name := node.Content[i].Value
		a.Names = append(a.Names, name)
		var p AptPackage
		if node.Content[i+1].Kind == yaml.MappingNode {
			if err := node.Content[i+1].Decode(&p); err != nil {
				return err
			}
		}
		a.Attrs[name] = p
	}
	return nil
}

func (a AptPackages) MarshalYAML() (any, error) {
	if len(a.Attrs) == 0 {
		return a.Names, nil
	}
	out := map[string]AptPackage{}
	for _, n := range a.Names {
		out[n] = a.Attrs[n]
	}
	return out, nil
}

func (a AptPackages) Aliases() map[string]string {
	out := map[string]string{}
	for _, p := range a.Attrs {
		maps.Copy(out, p.AliasBinary)
	}
	return out
}

// AptRepoSpec declares an external apt repo: source url, verification key, suites, components.
// [why] verificationKey takes a url (key downloaded into /etc/apt/keyrings) or an
//
//	absolute path (keyring already on the host, nothing fetched)
type AptRepoSpec struct {
	URL             string `yaml:"url,omitempty"`
	VerificationKey string `yaml:"verificationKey,omitempty"`
	Suites          string `yaml:"suites,omitempty"`
	Components      string `yaml:"components,omitempty"`
}

func (s *ScriptSpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		s.Run = node.Value
		return nil
	}
	type plain ScriptSpec
	return node.Decode((*plain)(s))
}

type ScriptSpec struct {
	Run              string            `yaml:"run,omitempty"`
	Args             []string          `yaml:"args,omitempty"`
	Path             string            `yaml:"path,omitempty"`
	URL              string            `yaml:"remoteUrl,omitempty"`
	OS               string            `yaml:"os,omitempty"`
	Version          string            `yaml:"version,omitempty"`
	Platforms        ItemPlatforms     `yaml:"platforms,omitempty"`
	Env              map[string]string `yaml:"env,omitempty"`
	ValidateArtifact string            `yaml:"validateArtifact,omitempty"`
}

// ItemPlatforms lists an item's supported platforms, each optionally carrying its asset's sha256.
// [why] a sha references exactly one platform, so one key holds both facts:
//
//   - darwin-arm64            (supported, unverified)
//   - linux-amd64: <sha256>   (supported, verified)
type ItemPlatforms struct {
	Names []string
	Sha   map[string]string
}

func (ip *ItemPlatforms) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("platforms must be a list of platform names or platform: sha256 pairs")
	}
	for _, el := range node.Content {
		if el.Kind == yaml.ScalarNode {
			ip.Names = append(ip.Names, el.Value)
			continue
		}
		if el.Kind != yaml.MappingNode || len(el.Content) != 2 {
			return fmt.Errorf("platforms entry must be a platform name or a single platform: sha256 pair")
		}
		name := el.Content[0].Value
		ip.Names = append(ip.Names, name)
		if ip.Sha == nil {
			ip.Sha = map[string]string{}
		}
		ip.Sha[name] = el.Content[1].Value
	}
	return nil
}

func (ip ItemPlatforms) MarshalYAML() (any, error) {
	out := make([]any, 0, len(ip.Names))
	for _, n := range ip.Names {
		if sha, ok := ip.Sha[n]; ok {
			out = append(out, map[string]string{n: sha})
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

func (ip ItemPlatforms) IsZero() bool { return len(ip.Names) == 0 }

type PrebuiltBinariesArchiveSpec struct {
	Version         string        `yaml:"version,omitempty"`
	URL             string        `yaml:"url,omitempty"`
	ArchConvention  string        `yaml:"archConvention,omitempty"`
	ExtractBinaries Strings       `yaml:"extractBinaries,omitempty"`
	Platforms       ItemPlatforms `yaml:"platforms,omitempty"`
}

func (it *Item) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		if node.Value == "code" || node.Value == "vscode" {
			return fmt.Errorf("vscode extension items must be written {brew: {vscode: <extension-id>}}")
		}
		it.Mgr = node.Value
		return nil
	}
	if node.Kind != yaml.MappingNode || len(node.Content) != 2 {
		return fmt.Errorf("manager item must be a scalar or a single-key map")
	}
	key, val := node.Content[0], node.Content[1]
	if key.Value == "prebuiltBinariesArchive" {
		it.Mgr = "prebuiltBinariesArchive"
		it.PrebuiltBinariesArchive = &PrebuiltBinariesArchiveSpec{}
		if err := val.Decode(it.PrebuiltBinariesArchive); err != nil {
			return err
		}
		if strings.Contains(it.PrebuiltBinariesArchive.URL+" "+strings.Join(it.PrebuiltBinariesArchive.ExtractBinaries, " "), "{arch_") {
			return fmt.Errorf("prebuiltBinariesArchive tokens {arch_x}/{arch_g} are gone: use {arch} with archConvention")
		}
		return nil
	}
	if key.Value == "apt" && val.Kind == yaml.MappingNode {
		it.Mgr = "apt"
		it.Apt = &AptSpec{}
		if err := val.Decode(it.Apt); err != nil {
			return err
		}
		if len(it.Apt.Versions) > 1 {
			return fmt.Errorf("apt versions must map exactly one binary version to one package version")
		}
		if len(it.Apt.Versions) == 1 && len(it.Apt.InstallPackages.Names) > 1 {
			return fmt.Errorf("apt versions requires a single installPackages entry")
		}
		it.AliasBinary = it.Apt.InstallPackages.Aliases()
		return nil
	}
	if key.Value == "versionManager" {
		it.Mgr = "versionManager"
		it.VersionManager = &VersionManagerSpec{}
		return val.Decode(it.VersionManager)
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
		var sub struct {
			Cask   string `yaml:"cask"`
			Code   string `yaml:"code"`
			Vscode string `yaml:"vscode"`
		}
		if err := val.Decode(&sub); err != nil || sub.Code != "" || (sub.Cask == "") == (sub.Vscode == "") {
			return fmt.Errorf("brew object form must be {cask: <name>} or {vscode: <extension-id>}")
		}
		if sub.Cask != "" {
			it.Mgr, it.Name = "cask", sub.Cask
			return nil
		}
		it.Mgr, it.Name = "vscode", sub.Vscode
		return nil
	}
	if key.Value == "code" || key.Value == "vscode" {
		return fmt.Errorf("vscode extension items must be written {brew: {vscode: <extension-id>}}")
	}
	it.Mgr = key.Value
	if err := val.Decode(&it.Name); err != nil {
		return err
	}
	if it.Mgr == "npm" && strings.LastIndex(it.Name, "@") > 0 {
		return fmt.Errorf("npm item %s: version pins go in the entry version field", it.Name)
	}
	if it.Mgr == "brew" && strings.Contains(it.Name, "@") && !strings.HasSuffix(it.Name, "@{version}") {
		return fmt.Errorf("brew item %s: version pins go in the entry version field, referenced as @{version}", it.Name)
	}
	if it.Mgr == "apt" && strings.Contains(it.Name, "=") {
		return fmt.Errorf("apt item %s: version pins go in the entry version field", it.Name)
	}
	return nil
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
	if override.ArchNameConventions != nil {
		f.ArchNameConventions = override.ArchNameConventions
	}
	if override.Platforms != nil {
		f.Platforms = override.Platforms
	}
	f.BasePackages = fsutil.MergeMap(f.BasePackages, override.BasePackages)
	f.Packages = fsutil.MergeMap(f.Packages, override.Packages)
}

func (f *File) ValidatePlatforms() error {
	platforms := f.platformsOrBuiltin()
	archNameConventions := f.archNameConventionsOrBuiltin()
	if len(platforms) == 0 {
		return nil
	}
	supported := slices.Sorted(maps.Keys(platforms))
	for id, methods := range platforms {
		for _, m := range methods {
			if !slices.Contains(PlatformMethods, m) {
				return fmt.Errorf("platforms.%s: unknown installation method %q: want one of %s", id, m, strings.Join(PlatformMethods, ", "))
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(f.Packages)) {
		for _, it := range f.Packages[name].Items {
			if it.PrebuiltBinariesArchive == nil {
				continue
			}
			for _, p := range it.PrebuiltBinariesArchive.Platforms.Names {
				if !slices.Contains(supported, p) {
					return fmt.Errorf("package %s: unknown platform %q (platforms keys: %s)", name, p, strings.Join(supported, ", "))
				}
			}
			c := it.PrebuiltBinariesArchive.ArchConvention
			if c == "" && strings.Contains(it.PrebuiltBinariesArchive.URL+" "+strings.Join(it.PrebuiltBinariesArchive.ExtractBinaries, " "), "{arch}") {
				return fmt.Errorf("package %s: {arch} requires archConvention (archNameConventions sets: %s)", name, strings.Join(slices.Sorted(maps.Keys(archNameConventions)), ", "))
			}
			if c != "" {
				if _, ok := archNameConventions[c]; !ok {
					return fmt.Errorf("package %s: unknown archConvention %q (archNameConventions sets: %s)", name, c, strings.Join(slices.Sorted(maps.Keys(archNameConventions)), ", "))
				}
			}
		}
	}
	return nil
}

func (f *File) Find(pkg, path string) (Entry, error) {
	e, ok := f.Packages[pkg]
	if !ok {
		return Entry{}, fmt.Errorf("unknown package: %s (add it to %s)", pkg, path)
	}
	return e, nil
}

// [<] 🤖🤖🤖

// [>] 🤖🤖

// ResolveScriptPaths anchors relative script paths to the file that declared them.
// [why] override entries merge into a base file: without anchoring, their relative
//
//	paths would resolve against the base (or the builtin embed, by basename)
func (f *File) ResolveScriptPaths(dir string) {
	for _, e := range f.Packages {
		for _, it := range e.Items {
			anchorScriptPath(dir, it.Script)
		}
		anchorScriptPath(dir, e.PostInstall)
	}
}

func anchorScriptPath(dir string, s *ScriptSpec) {
	if s == nil || s.Path == "" || filepath.IsAbs(s.Path) {
		return
	}
	s.Path = filepath.Join(dir, s.Path)
}

// [<] 🤖🤖
