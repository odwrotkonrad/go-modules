package yamlcfg

// [>] 🤖🤖

import (
	"embed"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func overrideSystemDir(t *testing.T, dir string) {
	t.Helper()
	prev := SystemDir
	SystemDir = dir
	t.Cleanup(func() { SystemDir = prev })
}

func TestCode(t *testing.T) {
	type in struct {
		Code  *int
		Plain string
	}
	type c struct {
		Name string
		In   in
		Want int
	}
	testyml.Run(t, td, "testdata/spec/code.spec.yml", func(t *testing.T, c c) {
		var err error
		switch {
		case c.In.Code != nil:
			err = &CodedError{Code: *c.In.Code, Msg: "x"}
		case c.In.Plain != "":
			err = errors.New(c.In.Plain)
		}
		if got := Code(err); got != c.Want {
			t.Errorf("Code = %d, want %d", got, c.Want)
		}
	})
}

func TestLoadConfig(t *testing.T) {
	type in struct {
		System string
		User   string
	}
	type c struct {
		Name string
		In   in
		Want testyml.Want
	}
	testyml.Run(t, td, "testdata/spec/load_config.spec.yml", func(t *testing.T, c c) {
		sysDir := t.TempDir()
		if c.In.System == "" {
			sysDir = filepath.Join(sysDir, "no-system")
		} else if err := os.WriteFile(filepath.Join(sysDir, "cfg.yml"), []byte(testyml.ReadFile(t, td, c.In.System)), 0o644); err != nil {
			t.Fatal(err)
		}
		overrideSystemDir(t, sysDir)
		userDir := t.TempDir()
		if c.In.User != "" {
			if err := os.WriteFile(filepath.Join(userDir, "cfg.yml"), []byte(testyml.ReadFile(t, td, c.In.User)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		node, err := LoadConfigNode("cfg.yml", userDir)
		if c.Want.IsErrorWanted() {
			c.Want.CheckErr(t, err)
			if got := Code(err); c.Want.ExitCode != 0 && got != c.Want.ExitCode {
				t.Fatalf("Code = %d (%v), want %d", got, err, c.Want.ExitCode)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if c.Want.FilesOut == "" {
			if node != nil {
				t.Fatalf("node = %v, want nil", node)
			}
			return
		}
		var got, want any
		if err := node.Decode(&got); err != nil {
			t.Fatal(err)
		}
		if err := yaml.Unmarshal([]byte(testyml.ReadFile(t, td, c.Want.FilesOut)), &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("decoded = %v, want %v", got, want)
		}
	})
}

func TestLoadConfigDecode(t *testing.T) {
	type in struct {
		User string
	}
	type c struct {
		Name string
		In   in
		Want testyml.Want
	}
	testyml.Run(t, td, "testdata/spec/load_config_decode.spec.yml", func(t *testing.T, c c) {
		overrideSystemDir(t, filepath.Join(t.TempDir(), "no-system"))
		userDir := t.TempDir()
		if c.In.User != "" {
			if err := os.WriteFile(filepath.Join(userDir, "cfg.yml"), []byte(testyml.ReadFile(t, td, c.In.User)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		var got map[string]int
		err := LoadConfig("cfg.yml", userDir, &got)
		if c.Want.IsErrorWanted() {
			c.Want.CheckErr(t, err)
			if code := Code(err); code != c.Want.ExitCode {
				t.Fatalf("Code = %d (%v), want %d", code, err, c.Want.ExitCode)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if c.Want.FilesOut == "" {
			if got != nil {
				t.Fatalf("decoded = %v, want untouched nil", got)
			}
			return
		}
		var want map[string]int
		if err := yaml.Unmarshal([]byte(testyml.ReadFile(t, td, c.Want.FilesOut)), &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("decoded = %v, want %v", got, want)
		}
	})
}

func TestCustomPathsEnvFallbacks(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := customPaths("cfg.yml", ""); got[1] != filepath.Join("/xdg", "custom", "cfg.yml") {
		t.Errorf("xdg path = %q", got[1])
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/h")
	if got := customPaths("cfg.yml", ""); got[1] != filepath.Join("/h", ".config", "custom", "cfg.yml") {
		t.Errorf("home path = %q", got[1])
	}
}

func TestUnwrapNonDocument(t *testing.T) {
	n := &yaml.Node{Kind: yaml.ScalarNode}
	if Unwrap(n) != n {
		t.Error("scalar node not returned as-is")
	}
	if Unwrap(nil) != nil {
		t.Error("nil not returned as-is")
	}
}

func TestLoadConfigNodeReadError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads 0o000 files")
	}
	overrideSystemDir(t, filepath.Join(t.TempDir(), "no-system"))
	userDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(userDir, "cfg.yml"), []byte("a: 1"), 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfigNode("cfg.yml", userDir)
	if Code(err) != CodeConfig {
		t.Fatalf("Code = %d (%v), want %d", Code(err), err, CodeConfig)
	}
}

// [<] 🤖🤖
