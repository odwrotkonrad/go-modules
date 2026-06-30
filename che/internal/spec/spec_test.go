package spec

// [>] 🤖🤖

import (
	"maps"
	"path/filepath"
	"slices"
	"testing"

	"gitlab.com/konradodwrot/go/che/internal/testutil"
)

// fixtureRepo commits a git repo from files plus che.yml fixture
// (testutil/specs/<spec>.yml), returns dir.
func fixtureRepo(t *testing.T, spec string, files map[string]string) string {
	t.Helper()
	files = maps.Clone(files)
	files["che.yml"] = testutil.Spec(t, spec)
	return testutil.Repo(t, files)
}

func resolve(t *testing.T, dir, profile string) Resolved {
	t.Helper()
	s, err := Load(filepath.Join(dir, "che.yml"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Resolve(profile, filepath.Join(dir, "root"))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// resolveErr asserts Resolve(profile) fails for the given spec fixture.
func resolveErr(t *testing.T, spec, profile string) {
	t.Helper()
	dir := fixtureRepo(t, spec, map[string]string{"root/.gitkeep": ""})
	s, _ := Load(filepath.Join(dir, "che.yml"))
	if _, err := s.Resolve(profile, filepath.Join(dir, "root")); err == nil {
		t.Fatalf("%s/%s: expected error", spec, profile)
	}
}

var mergeFiles = map[string]string{
	"root/etc/zshrc":                                   "zshrc\n",
	"root/HOME/.config/zsh/.zshrc":                     "user zshrc\n",
	"root/etc/grafana/grafana.ini":                     "ini\n",
	"root/Library/LaunchDaemons/otelcol.plist.host.cp": "plist\n",
	"ci/zsh/scripts/installs/10-brew.zsh":              "#!/bin/zsh\n",
	"ci/zsh/scripts/installs/20-kitty.zsh":             "#!/bin/zsh\n",
}

// find returns a pointer to the first item satisfying pred, or nil.
func find(items []FileItem, pred func(FileItem) bool) *FileItem {
	if i := slices.IndexFunc(items, pred); i >= 0 {
		return &items[i]
	}
	return nil
}

func relIs(rel string) func(FileItem) bool {
	return func(it FileItem) bool { return it.Rel == rel }
}

func destIs(path string) func(FileItem) bool {
	return func(it FileItem) bool { return it.Dests[0].Path == path }
}

// hasLink reports whether res.Links carries a file with the given rel.
func hasLink(res Resolved, rel string) bool {
	return find(res.Links, relIs(rel)) != nil
}

func TestResolveMerge(t *testing.T) {
	dir := fixtureRepo(t, "merge", mergeFiles)

	// desktop: base, everything present.
	host := resolve(t, dir, "desktop/macos")
	wantScripts := []string{
		"ci/zsh/scripts/installs/10-brew.zsh",
		"ci/zsh/scripts/installs/20-kitty.zsh",
	}
	if !slices.Equal(host.Scripts, wantScripts) {
		t.Errorf("host scripts order = %v, want %v", host.Scripts, wantScripts)
	}
	if !hasDir(host, "/var/log/grafana") || !hasDir(host, "HOME/.cache/zsh") {
		t.Errorf("host dirs missing merge: %v", dirPaths(host.ExtraDirs))
	}
	if d := findDir(host, "/var/log/grafana"); d == nil || d.Chmod != "2775" {
		t.Errorf("grafana dir lost spec chmod: %+v", d)
	}
	wantServices := []string{"otelcol", "port-exporter", "grafana", "prometheus"}
	if !slices.Equal(host.Services, wantServices) {
		t.Errorf("host services = %v, want %v", host.Services, wantServices)
	}
	if !hasLink(host, "etc/grafana/grafana.ini") {
		t.Errorf("host missing grafana link: %v", host.Links)
	}
	// glob in a perm-bearing copy group stamps perms on matched files.
	if c := find(host.Copies, relIs("Library/LaunchDaemons/otelcol.plist.host.cp")); c == nil || c.Chmod != "0600" {
		t.Errorf("perm-group glob did not stamp copy chmod: %+v", c)
	}

	// cli: base minus exclude-desktop.
	vm := resolve(t, dir, "cli/macos")
	if !slices.Equal(vm.Scripts, []string{"ci/zsh/scripts/installs/10-brew.zsh"}) {
		t.Errorf("vm scripts = %v, want brew only", vm.Scripts)
	}
	if hasDir(vm, "/var/log/grafana") {
		t.Errorf("vm must not keep desktop dirs: %v", dirPaths(vm.ExtraDirs))
	}
	if !slices.Equal(vm.Services, []string{"otelcol", "port-exporter"}) {
		t.Errorf("vm services = %v, want desktop excluded: %v", vm.Services, vm.Services)
	}
	if hasLink(vm, "etc/grafana/grafana.ini") {
		t.Errorf("vm kept desktop-only grafana: %v", vm.Links)
	}
}

func TestResolveClassify(t *testing.T) {
	files := map[string]string{
		"root/etc/zshrc":                                   "zshrc\n",
		"root/etc/zsh/zshenv":                              "env\n",
		"root/HOME/.config/zsh/.zshrc":                     "user zshrc\n",
		"root/HOME/.config/git/config":                     "[user]\n",
		"root/HOME/.config/zsh/x.host.cp":                  "copyme\n",
		"root/HOME/.config/zsh/y.host.tpl":                 "tmpl\n",
		"root/HOME/.config/zsh/.gitkeep":                   "",
		"root/etc/grafana/grafana.ini":                     "ini\n",
		"root/Library/LaunchDaemons/otelcol.plist.host.cp": "plist\n",
	}
	dir := fixtureRepo(t, "classify", files)
	cs := resolve(t, dir, "cli/macos")
	wantLinks := []string{
		"HOME/.config/git/config",
		"HOME/.config/zsh/.zshrc",
		"etc/zsh/zshenv",
		"etc/zshrc",
	}
	if !slices.Equal(rels(cs.Links), wantLinks) {
		t.Errorf("links = %v, want %v", rels(cs.Links), wantLinks)
	}
	if !slices.Equal(rels(cs.Copies), []string{
		"HOME/.config/zsh/x.host.cp",
		"Library/LaunchDaemons/otelcol.plist.host.cp",
	}) {
		t.Errorf("copies = %v", rels(cs.Copies))
	}
	if !slices.Equal(rels(cs.Templates), []string{"HOME/.config/zsh/y.host.tpl"}) {
		t.Errorf("templates = %v", rels(cs.Templates))
	}
	for _, l := range rels(cs.Links) {
		if filepath.Base(l) == ".gitkeep" {
			t.Errorf(".gitkeep leaked into links")
		}
	}
	if !slices.Contains(cs.Dirs, "HOME") || !slices.Contains(cs.Dirs, "HOME/.config/zsh") {
		t.Errorf("dirs missing ancestors: %v", cs.Dirs)
	}
}

// rels extracts the Rel of each FileItem.
func rels(items []FileItem) []string {
	return mapItems(items, func(it FileItem) string { return it.Rel })
}

// dirPaths extracts the first dest path of each dir FileItem.
func dirPaths(items []FileItem) []string {
	return mapItems(items, func(it FileItem) string { return it.Dests[0].Path })
}

// mapItems projects each FileItem through fn.
func mapItems[T any](items []FileItem, fn func(FileItem) T) []T {
	out := make([]T, len(items))
	for i, it := range items {
		out[i] = fn(it)
	}
	return out
}

// findDir returns the dir FileItem with the given dest path, or nil.
func findDir(res Resolved, path string) *FileItem { return find(res.ExtraDirs, destIs(path)) }

// hasDir reports whether res.ExtraDirs carries the given path.
func hasDir(res Resolved, path string) bool { return findDir(res, path) != nil }

func TestResolveUndefinedFails(t *testing.T) {
	dir := fixtureRepo(t, "merge", mergeFiles)
	s, _ := Load(filepath.Join(dir, "che.yml"))
	if _, err := s.Resolve("cli/linux", filepath.Join(dir, "root")); err == nil {
		t.Fatal("expected error for declared-but-undefined profile")
	}
}

func TestMixinProfilesCycle(t *testing.T) {
	resolveErr(t, "cycle", "cli/macos")
}

// TestIncludeExcludeSections: exclude wins over explicit include across every
// key (glob match, not exact), including rich {source,dest} entries.
func TestIncludeExcludeSections(t *testing.T) {
	files := map[string]string{
		"root/etc/zshrc":                       "z\n",
		"root/etc/zsh/zshenv":                  "e\n", // excluded -> must not link
		"root/HOME/.config/extra/x":            "x\n",
		"root/HOME/.config/oneoff/y":           "y\n",
		"root/HOME/.config/zsh/c.host.cp":      "c\n", // rich copy, excluded by glob
		"ci/zsh/scripts/installs/10-brew.zsh":  "#!/bin/zsh\n",
		"ci/zsh/scripts/installs/20-foo.zsh":   "#!/bin/zsh\n", // excluded by run-scripts
	}
	dir := fixtureRepo(t, "include-exclude", files)
	res := resolve(t, dir, "cli/macos")

	if !hasLink(res, "HOME/.config/extra/x") {
		t.Errorf("include.link extra not merged: %v", rels(res.Links))
	}
	if !hasLink(res, "etc/zshrc") {
		t.Errorf("etc/zshrc include missing: %v", rels(res.Links))
	}
	if hasLink(res, "etc/zsh/zshenv") {
		t.Errorf("exclude.link glob not applied: %v", rels(res.Links))
	}
	if find(res.Copies, relIs("HOME/.config/zsh/c.host.cp")) != nil {
		t.Errorf("exclude.copy glob did not drop rich entry: %v", rels(res.Copies))
	}
	if !slices.Contains(res.Scripts, "ci/zsh/scripts/installs/10-brew.zsh") {
		t.Errorf("include.run-scripts brew missing: %v", res.Scripts)
	}
	if slices.Contains(res.Scripts, "ci/zsh/scripts/installs/20-foo.zsh") {
		t.Errorf("exclude.run-scripts did not remove foo: %v", res.Scripts)
	}
	if slices.Contains(res.Services, "grafana") {
		t.Errorf("exclude.services glob did not remove grafana: %v", res.Services)
	}
	if !slices.Contains(res.Services, "otelcol") {
		t.Errorf("otelcol service missing: %v", res.Services)
	}
}

// TestExcludeScriptGlob: a single-file run-scripts exclude drops that file even
// when the include is a directory glob ([why] globs expand before exclude).
func TestExcludeScriptGlob(t *testing.T) {
	files := map[string]string{
		"root/.gitkeep":                        "",
		"ci/zsh/scripts/installs/10-brew.zsh":  "#!/bin/zsh\n",
		"ci/zsh/scripts/installs/20-foo.zsh":   "#!/bin/zsh\n",
		"ci/zsh/scripts/installs/30-tmux.zsh":  "#!/bin/zsh\n",
	}
	dir := fixtureRepo(t, "exclude-script-glob", files)
	res := resolve(t, dir, "cli/macos")

	want := []string{
		"ci/zsh/scripts/installs/10-brew.zsh",
		"ci/zsh/scripts/installs/30-tmux.zsh",
	}
	if !slices.Equal(res.Scripts, want) {
		t.Errorf("glob include + single-file exclude: got %v, want %v", res.Scripts, want)
	}
}

func TestMixinProfilesUndefined(t *testing.T) {
	resolveErr(t, "undefined-include", "cli/macos")
}

// [<] 🤖🤖
