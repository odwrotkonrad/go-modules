package packages

// [>] 🤖🤖

import (
	"embed"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func testHost(osname, arch string, cmds map[string]string) Host {
	distro := ""
	if osname == "linux" {
		distro = "debian"
	}
	return Host{
		OS: osname, Arch: arch, Distro: distro, Euid: 501,
		LookPath: func(name string) (string, error) {
			if p, ok := cmds[name]; ok {
				return p, nil
			}
			return "", fmt.Errorf("%s not found", name)
		},
		PathDirs: func() []string { return nil },
		Getenv: func(k string) string {
			if k == "HOME" {
				return "/home/u"
			}
			return ""
		},
		ReadFile: func(path string) ([]byte, error) { return nil, fmt.Errorf("read %s: not stubbed", path) },
	}
}

func cmdMap(names []string) map[string]string {
	m := map[string]string{}
	for _, n := range names {
		m[n] = "/usr/bin/" + n
	}
	return m
}

type itemGot struct {
	Mgr        string            `yaml:"mgr"`
	Name       string            `yaml:"name"`
	Version    string            `yaml:"version,omitempty"`
	URL        string            `yaml:"url,omitempty"`
	Platforms  []string          `yaml:"platformNames,omitempty"`
	Checksums  map[string]string `yaml:"checksums,omitempty"`
	Run        string            `yaml:"run,omitempty"`
	ScriptOs   string            `yaml:"scriptOs,omitempty"`
	ScriptPath string            `yaml:"scriptPath,omitempty"`
	ScriptURL  string            `yaml:"scriptUrl,omitempty"`
	ExtractMan []string          `yaml:"extractManpages,omitempty"`
	Manpages   string            `yaml:"manpages,omitempty"`
}

func formatManpages(pages []string) string {
	if pages == nil {
		return ""
	}
	if len(pages) == 0 {
		return "[]"
	}
	return strings.Join(pages, ", ")
}

func TestItemUnmarshal(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/item_unmarshal.test.spec.yml", func(t *testing.T, c testyml.Case[itemGot]) (itemGot, error) {
		var it Item
		if err := yaml.Unmarshal([]byte(c.Input.Args.String(t, 0)), &it); err != nil {
			return itemGot{}, err
		}
		got := itemGot{Mgr: it.Mgr, Name: it.Name, Manpages: formatManpages(it.Manpages)}
		if it.Apt != nil {
			got.Name = it.Apt.PackageName
		}
		if it.BinariesRemoteArchive != nil {
			got.ExtractMan = it.BinariesRemoteArchive.ExtractManpages
			got.Version, got.URL, got.Platforms, got.Checksums = it.BinariesRemoteArchive.Version, it.BinariesRemoteArchive.URL, it.BinariesRemoteArchive.PlatformEligibility.Names, it.BinariesRemoteArchive.PlatformEligibility.Checksums
		}
		if it.BuildFromSource != nil {
			got.Version, got.URL, got.Platforms = it.BuildFromSource.Version, it.BuildFromSource.URL, it.BuildFromSource.PlatformEligibility.Names
		}
		if it.Script != nil {
			got.Run, got.ScriptOs = strings.TrimSpace(it.Script.Run), it.Script.OS
			got.ScriptPath, got.ScriptURL = it.Script.Path, it.Script.URL
		}
		return got, nil
	})
}

func TestMerge(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/merge.test.spec.yml", func(t *testing.T, c testyml.Case[map[string][]string]) (map[string][]string, error) {
		var base, override File
		require.NoError(t, yaml.Unmarshal([]byte(c.Input.Args.String(t, 0)), &base))
		require.NoError(t, yaml.Unmarshal([]byte(c.Input.Args.String(t, 1)), &override))
		base.Merge(&override)
		got := map[string][]string{}
		for name, entry := range base.Packages {
			for _, it := range entry.Items {
				got[name] = append(got[name], it.Mgr)
			}
		}
		return got, nil
	})
}

type zshCompGot struct {
	Name string `yaml:"name,omitempty"`
	URL  string `yaml:"url,omitempty"`
	Cmd  string `yaml:"cmd,omitempty"`
}

type entryGot struct {
	Mgrs         []string          `yaml:"mgrs,omitempty"`
	Verify       string            `yaml:"verify,omitempty"`
	ItemVerifies []string          `yaml:"itemVerifies,omitempty"`
	Name         string            `yaml:"name,omitempty"`
	Aliases      map[string]string `yaml:"aliases,omitempty"`
	Completions  *zshCompGot       `yaml:"completionsZsh,omitempty"`
	Manpages     string            `yaml:"manpages,omitempty"`
	ItemManpages []string          `yaml:"itemManpages,omitempty"`
}

func formatVerify(v *VerifySpec) string {
	switch {
	case v == nil:
		return ""
	case v.VersionCmd:
		return "versionCmd"
	case v.PkgMgrVersionCheck:
		return "pkgMgrVersionCheck"
	case v.Cmd != "":
		return "cmd: " + v.Cmd
	}
	return "set"
}

func summarizeEntries(f *File) map[string]entryGot {
	got := map[string]entryGot{}
	for name, e := range f.Packages {
		g := entryGot{Verify: formatVerify(e.Verify), Manpages: formatManpages(e.Manpages)}
		anyItemVerify := false
		anyItemManpages := false
		for _, it := range e.Items {
			g.Mgrs = append(g.Mgrs, it.Mgr)
			v := formatVerify(it.Verify)
			g.ItemVerifies = append(g.ItemVerifies, v)
			anyItemVerify = anyItemVerify || v != ""
			mp := formatManpages(it.Manpages)
			g.ItemManpages = append(g.ItemManpages, mp)
			anyItemManpages = anyItemManpages || mp != ""
			if it.Apt != nil && it.Apt.PackageName != "" {
				g.Name = it.Apt.PackageName
			}
			if len(it.AliasBinary) > 0 {
				g.Aliases = it.AliasBinary
			}
		}
		if !anyItemVerify {
			g.ItemVerifies = nil
		}
		if !anyItemManpages {
			g.ItemManpages = nil
		}
		if z := e.Completions.Zsh; z != nil {
			g.Completions = &zshCompGot{Name: z.Name, URL: z.URL, Cmd: z.Cmd}
		}
		got[name] = g
	}
	return got
}

func TestEntryUnmarshal(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/entry_unmarshal.test.spec.yml", func(t *testing.T, c testyml.Case[map[string]entryGot]) (map[string]entryGot, error) {
		var f File
		if err := yaml.Unmarshal([]byte(c.Input.Args.String(t, 0)), &f); err != nil {
			return nil, err
		}
		if c.Input.Args.Bool(t, 1) {
			if err := f.ValidatePlatforms(); err != nil {
				return nil, err
			}
		}
		if c.Input.Args.Bool(t, 2) {
			out, err := f.YAML()
			require.NoError(t, err)
			var reparsed File
			require.NoError(t, yaml.Unmarshal([]byte(out), &reparsed))
			out2, err := reparsed.YAML()
			require.NoError(t, err)
			require.Equal(t, out, out2)
			f = reparsed
		}
		return summarizeEntries(&f), nil
	})
}

type pickGot struct {
	Mgr    string `yaml:"mgr,omitempty"`
	Picked bool   `yaml:"picked"`
}

func TestPick(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/pick.test.spec.yml", func(t *testing.T, c testyml.Case[pickGot]) (pickGot, error) {
		h := testHost(c.Input.Args.String(t, 0), "amd64", cmdMap(c.Input.Args.Strings(t, 1)))
		var entry Entry
		require.NoError(t, yaml.Unmarshal([]byte(c.Input.Args.String(t, 2)), &entry))
		it, ok, err := h.pickPreferred("pkg", entry, c.Input.Args.Strings(t, 3), c.Input.Args.Strings(t, 4), nil, false)
		if err != nil {
			return pickGot{}, err
		}
		return pickGot{Mgr: it.Mgr, Picked: ok}, nil
	})
}

func TestExpand(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/expand.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		h := testHost(c.Input.Args.String(t, 0), "amd64", nil)
		return h.expandTokens(c.Input.Args.String(t, 2), c.Input.Args.String(t, 3), c.Input.Args.String(t, 1)), nil
	})
}

// [<] 🤖🤖
