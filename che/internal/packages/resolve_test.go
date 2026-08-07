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
	archX, archG := "x86_64", "x86_64"
	if arch == "arm64" {
		archX, archG = "aarch64", "arm64"
	}
	return Host{
		OS: osname, Arch: arch, ArchX: archX, ArchG: archG, Euid: 501,
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
	Sha256     map[string]string `yaml:"sha256,omitempty"`
	Run        string            `yaml:"run,omitempty"`
	ScriptOs   string            `yaml:"scriptOs,omitempty"`
	ScriptPath string            `yaml:"scriptPath,omitempty"`
	ScriptURL  string            `yaml:"scriptUrl,omitempty"`
}

func TestItemUnmarshal(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/item_unmarshal.test.spec.yml", func(t *testing.T, c testyml.Case[itemGot]) (itemGot, error) {
		var it Item
		if err := yaml.Unmarshal([]byte(c.Input.Args.String(t, 0)), &it); err != nil {
			return itemGot{}, err
		}
		got := itemGot{Mgr: it.Mgr, Name: it.Name}
		if it.Binary != nil {
			got.Version, got.URL, got.Sha256 = it.Binary.Version, it.Binary.URL, it.Binary.Sha256
		}
		if it.Pkg != nil {
			got.Version, got.URL, got.Sha256 = it.Pkg.Version, it.Pkg.URL, it.Pkg.Sha256
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

type pickGot struct {
	Mgr    string `yaml:"mgr,omitempty"`
	Picked bool   `yaml:"picked"`
}

func TestPick(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/pick.test.spec.yml", func(t *testing.T, c testyml.Case[pickGot]) (pickGot, error) {
		h := testHost(c.Input.Args.String(t, 0), "amd64", cmdMap(c.Input.Args.Strings(t, 1)))
		var entry Entry
		require.NoError(t, yaml.Unmarshal([]byte(c.Input.Args.String(t, 2)), &entry))
		it, ok, err := h.pickPreferred("pkg", entry, c.Input.Args.Strings(t, 3))
		if err != nil {
			return pickGot{}, err
		}
		return pickGot{Mgr: it.Mgr, Picked: ok}, nil
	})
}

func TestExpand(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/expand.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		h := testHost(c.Input.Args.String(t, 0), c.Input.Args.String(t, 1), nil)
		return h.expand(c.Input.Args.String(t, 2), c.Input.Args.String(t, 3)), nil
	})
}

// [<] 🤖🤖
