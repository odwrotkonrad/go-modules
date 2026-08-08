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
	ArchSchemes         map[string]map[string]string `yaml:"archSchemes,omitempty"`
	OSInstallers        map[string]InstallerList     `yaml:"osInstallers,omitempty"`
	InstallerRegistries *InstallerRegistriesSpec     `yaml:"installerRegistries,omitempty"`
	BasePackages        map[string][]string          `yaml:"basePackages,omitempty"`
	Packages            map[string]Entry             `yaml:"packages,omitempty"`
}

type InstallerList []string

func (l *InstallerList) UnmarshalYAML(node *yaml.Node) error {
	var walk func(n *yaml.Node) error
	walk = func(n *yaml.Node) error {
		switch n.Kind {
		case yaml.ScalarNode:
			*l = append(*l, n.Value)
			return nil
		case yaml.SequenceNode:
			for _, c := range n.Content {
				if err := walk(c); err != nil {
					return err
				}
			}
			return nil
		case yaml.AliasNode:
			return walk(n.Alias)
		}
		return fmt.Errorf("osInstallers values must be installer names or list aliases")
	}
	return walk(node)
}

type InstallerRegistriesSpec struct {
	Apt  []*AptRepoSpec `yaml:"apt,omitempty"`
	Brew []string       `yaml:"brew,omitempty"`
}

func (r *InstallerRegistriesSpec) apt() []*AptRepoSpec {
	if r == nil {
		return nil
	}
	return r.Apt
}

func (r *InstallerRegistriesSpec) brew() []string {
	if r == nil {
		return nil
	}
	return r.Brew
}

func stripURLScheme(url string) string {
	for _, p := range []string{"https://", "http://"} {
		if v, ok := strings.CutPrefix(url, p); ok {
			return v
		}
	}
	return url
}

func splitAptRegistryRef(ref string) (url, suites, components string) {
	parts := strings.SplitN(ref, "::", 3)
	url = parts[0]
	if len(parts) > 1 {
		suites = parts[1]
	}
	if len(parts) > 2 {
		components = parts[2]
	}
	return url, suites, components
}

func matchAptRegistries(registries []*AptRepoSpec, ref string) []*AptRepoSpec {
	url, suites, components := splitAptRegistryRef(ref)
	var out []*AptRepoSpec
	for _, r := range registries {
		if stripURLScheme(r.URL) != url {
			continue
		}
		if suites != "" && r.Suites != suites {
			continue
		}
		if components != "" && r.Components != components {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (f *File) aptRegistry(ref string) (*AptRepoSpec, error) {
	matches := matchAptRegistries(f.aptRegistriesOrBuiltin(), ref)
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return nil, fmt.Errorf("unknown apt registry %q (installerRegistries.apt, referenced by scheme-less url)", ref)
	default:
		return nil, fmt.Errorf("ambiguous apt registry %q: narrow with url::suites[::components]", ref)
	}
}

func orBuiltin[T any](own T, present bool, pick func(*File) T) T {
	if present {
		return own
	}
	if builtin, err := builtinFile(); err == nil {
		return pick(builtin)
	}
	var zero T
	return zero
}

func (f *File) aptRegistriesOrBuiltin() []*AptRepoSpec {
	regs := f.InstallerRegistries.apt()
	return orBuiltin(regs, len(regs) > 0, func(b *File) []*AptRepoSpec { return b.InstallerRegistries.apt() })
}

func (f *File) hasBrewTap(tap string) bool {
	return slices.Contains(f.InstallerRegistries.brew(), tap) ||
		slices.Contains(orBuiltin(nil, false, func(b *File) []string { return b.InstallerRegistries.brew() }), tap)
}

func aptRegistrySlug(r *AptRepoSpec) string {
	parts := []string{stripURLScheme(r.URL)}
	if r.Suites != "" {
		parts = append(parts, r.Suites)
	}
	if r.Components != "" {
		parts = append(parts, r.Components)
	}
	return strings.ReplaceAll(strings.Join(parts, "-"), "/", "-")
}

var builtinFile = sync.OnceValues(LoadBuiltin)

func (f *File) archSchemesOrBuiltin() map[string]map[string]string {
	return orBuiltin(f.ArchSchemes, f.ArchSchemes != nil, func(b *File) map[string]map[string]string { return b.ArchSchemes })
}

func (f *File) eligibleInstallersOrBuiltin() map[string]InstallerList {
	return orBuiltin(f.OSInstallers, f.OSInstallers != nil, func(b *File) map[string]InstallerList { return b.OSInstallers })
}

func (f *File) eligibleInstallers(h Host) []string {
	m := f.eligibleInstallersOrBuiltin()
	for _, k := range h.eligibilityKeys() {
		if v, ok := m[k]; ok {
			return v
		}
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
	Name     string `yaml:"name,omitempty"`
	Cmd      string `yaml:"cmd,omitempty"`
	URL      string `yaml:"url,omitempty"`
	Checksum string `yaml:"checksum,omitempty"`
}

type InstallerVocabulary struct {
	PackageName         string            `yaml:"packageName,omitempty"`
	VersionMap          map[string]string `yaml:"versionMap,omitempty"`
	ArchScheme          string            `yaml:"archScheme,omitempty"`
	AliasBinary         map[string]string `yaml:"aliasBinary,omitempty"`
	PlatformEligibility ItemPlatforms     `yaml:"platformEligibility,omitempty"`
	ExtractBinaries     Strings           `yaml:"extractBinaries,omitempty"`
}

func (v InstallerVocabulary) isZero() bool {
	return v.PackageName == "" && len(v.VersionMap) == 0 && v.ArchScheme == "" &&
		len(v.AliasBinary) == 0 && len(v.PlatformEligibility.Names) == 0 && len(v.ExtractBinaries) == 0
}

type itemBody struct {
	InstallerVocabulary `yaml:",inline"`
	Version             string `yaml:"version,omitempty"`
	FromRegistry        string `yaml:"fromRegistry,omitempty"`
}

func installerKey(mgr string) string {
	switch mgr {
	case "cask":
		return "brew/cask"
	case "vscode":
		return "brew/vscode"
	}
	return mgr
}

func mgrForInstallerKey(key string) (string, bool) {
	switch key {
	case "brew":
		return "brew", true
	case "brew/cask":
		return "cask", true
	case "brew/vscode":
		return "vscode", true
	}
	return "", false
}

func parseChecksumHex(c string) (string, error) {
	hex, ok := strings.CutPrefix(c, "sha256:")
	if !ok || hex == "" {
		return "", fmt.Errorf("checksum must be sha256:<hex>: %q", c)
	}
	return hex, nil
}

func (it Item) MarshalYAML() (any, error) {
	switch {
	case it.BinariesRemoteArchive != nil:
		return map[string]*BinariesRemoteArchiveSpec{"binariesRemoteArchive": it.BinariesRemoteArchive}, nil
	case it.Script != nil:
		return map[string]*ScriptSpec{"script": it.Script}, nil
	case it.Apt != nil:
		return map[string]*AptSpec{"apt": it.Apt}, nil
	case it.VersionManager != nil:
		return map[string]*VersionManagerSpec{it.VersionManager.Tool: it.VersionManager}, nil
	}
	body := itemBody{
		InstallerVocabulary: InstallerVocabulary{PackageName: it.Name, AliasBinary: it.AliasBinary},
		Version:             it.Version,
		FromRegistry:        it.Registry,
	}
	if body.isZero() && body.Version == "" && body.FromRegistry == "" {
		return installerKey(it.Mgr), nil
	}
	return map[string]itemBody{installerKey(it.Mgr): body}, nil
}

func (e Entry) MarshalYAML() (any, error) {
	if e.Version == "" && len(e.Requires) == 0 && e.PostInstall == nil && e.Command == "" && e.VersionCommand == "" && e.Completions.Zsh == nil {
		return e.Items, nil
	}
	obj := map[string]any{"installers": e.Items}
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
		{"archSchemes", len(f.ArchSchemes) == 0, f.ArchSchemes},
		{"osInstallers", len(f.OSInstallers) == 0, f.OSInstallers},
		{"installerRegistries", f.InstallerRegistries == nil, f.InstallerRegistries},
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
		Installers     []Item      `yaml:"installers"`
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
	if len(obj.Installers) == 0 && obj.Completions.Zsh == nil {
		return fmt.Errorf("package entry object form requires installers or completions")
	}
	if obj.Version == "__rolling__" {
		return fmt.Errorf("the __rolling__ sentinel is gone: omit version, absence means rolling")
	}
	if z := obj.Completions.Zsh; z != nil {
		if (z.Cmd == "") == (z.URL == "") {
			return fmt.Errorf("completions.zsh requires exactly one of cmd or url")
		}
		if z.Checksum != "" {
			if _, err := parseChecksumHex(z.Checksum); err != nil {
				return fmt.Errorf("completions.zsh: %w", err)
			}
		}
	}
	e.Items = obj.Installers
	e.Version = obj.Version
	e.Requires = obj.Requires
	e.Command = obj.Command
	e.PostInstall = obj.PostInstall
	e.VersionCommand = obj.VersionCommand
	e.Completions = obj.Completions
	return nil
}

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
	Mgr                   string
	Name                  string
	Version               string
	Registry              string
	AliasBinary           map[string]string
	BinariesRemoteArchive *BinariesRemoteArchiveSpec
	Script                *ScriptSpec
	Apt                   *AptSpec
	VersionManager        *VersionManagerSpec
}

type VersionManagerSpec struct {
	Tool     string   `yaml:"-"`
	Versions []string `yaml:"versions"`
	Global   string   `yaml:"global"`
}

type AptSpec struct {
	InstallerVocabulary `yaml:",inline"`
	FromRegistry        string `yaml:"fromRegistry,omitempty"`
}

func (a *AptSpec) packageName(pkg string) string {
	if a.PackageName != "" {
		return a.PackageName
	}
	return pkg
}

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
	if err := node.Decode((*plain)(s)); err != nil {
		return err
	}
	probe := s.InstallerVocabulary
	probe.PlatformEligibility = ItemPlatforms{}
	if !probe.isZero() {
		return fmt.Errorf("script item: of the vocabulary fields only platformEligibility applies")
	}
	return nil
}

type ScriptSpec struct {
	InstallerVocabulary `yaml:",inline"`
	Run                 string            `yaml:"run,omitempty"`
	Args                []string          `yaml:"args,omitempty"`
	Path                string            `yaml:"path,omitempty"`
	URL                 string            `yaml:"url,omitempty"`
	OS                  string            `yaml:"os,omitempty"`
	Version             string            `yaml:"version,omitempty"`
	Env                 map[string]string `yaml:"env,omitempty"`
	ValidateArtifact    string            `yaml:"validateArtifact,omitempty"`
}

type ItemPlatforms struct {
	Names     []string
	Checksums map[string]string
}

func (ip *ItemPlatforms) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("platformEligibility must be a list of platform names or platform: checksum pairs")
	}
	for _, el := range node.Content {
		if el.Kind == yaml.ScalarNode {
			ip.Names = append(ip.Names, el.Value)
			continue
		}
		if el.Kind != yaml.MappingNode || len(el.Content) != 2 {
			return fmt.Errorf("platformEligibility entry must be a platform name or a single platform: checksum pair")
		}
		name := el.Content[0].Value
		if _, err := parseChecksumHex(el.Content[1].Value); err != nil {
			return fmt.Errorf("platformEligibility.%s: %w", name, err)
		}
		ip.Names = append(ip.Names, name)
		if ip.Checksums == nil {
			ip.Checksums = map[string]string{}
		}
		ip.Checksums[name] = el.Content[1].Value
	}
	return nil
}

func (ip ItemPlatforms) MarshalYAML() (any, error) {
	out := make([]any, 0, len(ip.Names))
	for _, n := range ip.Names {
		if checksum, ok := ip.Checksums[n]; ok {
			out = append(out, map[string]string{n: checksum})
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

func (ip ItemPlatforms) IsZero() bool { return len(ip.Names) == 0 }

type BinariesRemoteArchiveSpec struct {
	InstallerVocabulary `yaml:",inline"`
	Version             string `yaml:"version,omitempty"`
	URL                 string `yaml:"url,omitempty"`
}

var legacyItemKeys = map[string][]string{
	"binariesRemoteArchive": {"installerVocabulary"},
	"script":                {"installerVocabulary"},
	"apt":                   {"installerVocabulary", "installPackages", "versions"},
	"brew":                  {"installerVocabulary", "formula", "cask", "vscode", "package"},
	"brew/cask":             {"installerVocabulary", "formula", "cask", "vscode", "package"},
	"brew/vscode":           {"installerVocabulary", "formula", "cask", "vscode", "package"},
	"npm":                   {"installerVocabulary", "package"},
	"gem":                   {"installerVocabulary", "package"},
	"go":                    {"installerVocabulary", "package"},
}

func rejectLegacyKeys(key string, val *yaml.Node) error {
	if val.Kind != yaml.MappingNode {
		return nil
	}
	for _, legacy := range legacyItemKeys[key] {
		if mappingHasKey(val, legacy) {
			return fmt.Errorf("%s item: %s is gone: vocabulary fields (packageName, versionMap, archScheme, aliasBinary, platformEligibility, extractBinaries) live directly on the item", key, legacy)
		}
	}
	return nil
}

func rejectBareBrewKind(v string) error {
	if v == "code" || v == "vscode" || v == "cask" {
		return fmt.Errorf("brew kinds are installer keys: brew/cask or brew/vscode")
	}
	return nil
}

func (it *Item) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		return it.unmarshalScalar(node.Value)
	}
	if node.Kind != yaml.MappingNode || len(node.Content) != 2 {
		return fmt.Errorf("manager item must be a scalar or a single-key map")
	}
	key, val := node.Content[0], node.Content[1]
	if err := rejectLegacyKeys(key.Value, val); err != nil {
		return err
	}
	switch key.Value {
	case "binariesRemoteArchive":
		return it.unmarshalBinariesRemoteArchive(val)
	case "apt":
		if val.Kind == yaml.MappingNode {
			return it.unmarshalApt(val)
		}
	case "versionManager":
		return fmt.Errorf("versionManager items are gone: name the tool as the installer ({pyenv: ...} or {nvm: ...})")
	case "pyenv", "nvm":
		it.Mgr = key.Value
		it.VersionManager = &VersionManagerSpec{Tool: key.Value}
		return val.Decode(it.VersionManager)
	case "script":
		it.Mgr = "script"
		it.Script = &ScriptSpec{}
		if val.Kind == yaml.ScalarNode {
			it.Script.Run = val.Value
			return nil
		}
		return val.Decode(it.Script)
	}
	return it.unmarshalManager(key.Value, val)
}

func (it *Item) unmarshalScalar(v string) error {
	if err := rejectBareBrewKind(v); err != nil {
		return err
	}
	if mgr, ok := mgrForInstallerKey(v); ok {
		it.Mgr = mgr
		return nil
	}
	it.Mgr = v
	return nil
}

func (it *Item) unmarshalBinariesRemoteArchive(val *yaml.Node) error {
	it.Mgr = "binariesRemoteArchive"
	b := &BinariesRemoteArchiveSpec{}
	if err := val.Decode(b); err != nil {
		return err
	}
	if strings.Contains(b.URL+" "+strings.Join(b.ExtractBinaries, " "), "{arch_") {
		return fmt.Errorf("binariesRemoteArchive tokens {arch_x}/{arch_g} are gone: use {arch} with archScheme")
	}
	it.BinariesRemoteArchive = b
	return nil
}

func (it *Item) unmarshalApt(val *yaml.Node) error {
	it.Mgr = "apt"
	a := &AptSpec{}
	if err := val.Decode(a); err != nil {
		return err
	}
	if len(a.VersionMap) > 1 {
		return fmt.Errorf("apt versionMap must map exactly one binary version to one package version")
	}
	if len(a.PlatformEligibility.Names) > 0 || len(a.ExtractBinaries) > 0 || a.ArchScheme != "" {
		return fmt.Errorf("apt item: platformEligibility, extractBinaries, and archScheme apply to binariesRemoteArchive and script items")
	}
	if strings.Contains(a.PackageName, "=") {
		return fmt.Errorf("apt item %s: version pins go in versionMap", a.PackageName)
	}
	it.Apt = a
	it.AliasBinary = a.AliasBinary
	return nil
}

func (it *Item) unmarshalManager(key string, val *yaml.Node) error {
	if err := rejectBareBrewKind(key); err != nil {
		return err
	}
	it.Mgr = key
	if mgr, ok := mgrForInstallerKey(key); ok {
		it.Mgr = mgr
	}
	if val.Kind != yaml.MappingNode {
		return fmt.Errorf("%s item: package names go in packageName", key)
	}
	var body itemBody
	if err := val.Decode(&body); err != nil {
		return err
	}
	probe := body.InstallerVocabulary
	probe.PackageName, probe.AliasBinary = "", nil
	if !probe.isZero() {
		return fmt.Errorf("%s item: versionMap, archScheme, platformEligibility, and extractBinaries apply to apt, binariesRemoteArchive, and script items", key)
	}
	if body.Version == "__rolling__" {
		return fmt.Errorf("%s item: the __rolling__ sentinel is gone: omit version, absence means rolling", key)
	}
	it.Name, it.Version, it.Registry, it.AliasBinary = body.PackageName, body.Version, body.FromRegistry, body.AliasBinary
	isBrew := it.Mgr == "brew" || it.Mgr == "cask"
	switch {
	case it.Mgr == "vscode" && it.Registry != "":
		return fmt.Errorf("brew/vscode item %s: extensions have no registry", it.Name)
	case it.Registry != "" && !isBrew:
		return fmt.Errorf("%s item %s: fromRegistry applies to brew, brew/cask, and apt items", key, it.Name)
	case it.Mgr == "npm" && strings.LastIndex(it.Name, "@") > 0:
		return fmt.Errorf("npm item %s: version pins go in the version field", it.Name)
	case isBrew && strings.Contains(it.Name, "@"):
		return fmt.Errorf("brew item %s: names stay bare, the pinned version is appended automatically", it.Name)
	case isBrew && strings.Contains(it.Name, "/"):
		return fmt.Errorf("brew item %s: tap-qualified names go in installerRegistries.brew, referenced as fromRegistry: <tap>", it.Name)
	}
	return nil
}

func mappingHasKey(node *yaml.Node, key string) bool {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return true
		}
	}
	return false
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
	if override.ArchSchemes != nil {
		f.ArchSchemes = override.ArchSchemes
	}
	if override.OSInstallers != nil {
		f.OSInstallers = override.OSInstallers
	}
	f.BasePackages = fsutil.MergeMap(f.BasePackages, override.BasePackages)
	f.Packages = fsutil.MergeMap(f.Packages, override.Packages)
	f.mergeRegistries(override.InstallerRegistries)
}

func (f *File) mergeRegistries(override *InstallerRegistriesSpec) {
	if override == nil {
		return
	}
	if f.InstallerRegistries == nil {
		f.InstallerRegistries = &InstallerRegistriesSpec{}
	}
	regs := f.InstallerRegistries
	for _, r := range override.Apt {
		if len(matchAptRegistries(regs.Apt, aptRegistryRef(r))) == 0 {
			regs.Apt = append(regs.Apt, r)
		}
	}
	for _, tap := range override.Brew {
		if !slices.Contains(regs.Brew, tap) {
			regs.Brew = append(regs.Brew, tap)
		}
	}
}

func aptRegistryRef(r *AptRepoSpec) string {
	ref := stripURLScheme(r.URL)
	if r.Suites != "" || r.Components != "" {
		ref += "::" + r.Suites
	}
	if r.Components != "" {
		ref += "::" + r.Components
	}
	return ref
}

func (f *File) ValidatePlatforms() error {
	eligible := f.eligibleInstallersOrBuiltin()
	archSchemes := f.archSchemesOrBuiltin()
	if len(eligible) == 0 {
		return nil
	}
	oses := map[string]bool{}
	for id, methods := range eligible {
		osName, _, _ := strings.Cut(id, "-")
		oses[osName] = true
		for _, m := range methods {
			if !slices.Contains(PlatformMethods, m) {
				return fmt.Errorf("osInstallers.%s: unknown installation method %q: want one of %s", id, m, strings.Join(PlatformMethods, ", "))
			}
		}
	}
	arches := map[string]bool{}
	for _, scheme := range archSchemes {
		for arch := range scheme {
			arches[arch] = true
		}
	}
	for _, name := range slices.Sorted(maps.Keys(f.Packages)) {
		for _, it := range f.Packages[name].Items {
			b := it.BinariesRemoteArchive
			if b == nil {
				continue
			}
			for _, p := range b.PlatformEligibility.Names {
				osName, arch, ok := strings.Cut(p, "-")
				if !ok || !oses[osName] || !arches[arch] {
					return fmt.Errorf("package %s: unknown platform %q (want <os>-<arch>: os from osInstallers keys %s, arch from archSchemes %s)",
						name, p, strings.Join(slices.Sorted(maps.Keys(oses)), ", "), strings.Join(slices.Sorted(maps.Keys(arches)), ", "))
				}
			}
			if b.ArchScheme == "" && strings.Contains(b.URL+" "+strings.Join(b.ExtractBinaries, " "), "{arch}") {
				return fmt.Errorf("package %s: {arch} requires archScheme (archSchemes sets: %s)", name, strings.Join(slices.Sorted(maps.Keys(archSchemes)), ", "))
			}
			if _, ok := archSchemes[b.ArchScheme]; b.ArchScheme != "" && !ok {
				return fmt.Errorf("package %s: unknown archScheme %q (archSchemes sets: %s)", name, b.ArchScheme, strings.Join(slices.Sorted(maps.Keys(archSchemes)), ", "))
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

// [<] 🤖🤖🤖
