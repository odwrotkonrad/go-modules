package yamlcfg_test

// [>] 🤖🤖

import (
	"embed"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

//go:embed all:testdata
var td embed.FS

// configDirs materializes user/system fixture refs as config dirs and points
// yamlcfg.SystemDir at the system one (or an absent dir). Returns the user dir.
func configDirs(t *testing.T, user, system string) string {
	t.Helper()
	sysDir := t.TempDir()
	if system == "" {
		sysDir = filepath.Join(sysDir, "no-system")
	} else {
		writeCfg(t, sysDir, system)
	}
	testyml.Swap(t, &yamlcfg.SystemDir, sysDir)
	userDir := t.TempDir()
	if user != "" {
		writeCfg(t, userDir, user)
	}
	return userDir
}

func writeCfg(t *testing.T, dir, fixture string) {
	t.Helper()
	body := testyml.ReadFile(t, td, fixture)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cfg.yml"), []byte(body), 0o644))
}

func TestCode(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/code.test.spec.yml", func(t *testing.T, c testyml.Case[int]) (int, error) {
		var err error
		switch c.Input.Args.Name(0) {
		case "code":
			err = &yamlcfg.CodedError{Code: c.Input.Args.Int(t, 0), Msg: "x"}
		case "plain":
			err = errors.New(c.Input.Args.String(t, 0))
		}
		return yamlcfg.Code(err), nil
	})
}

func TestLoadConfig(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/load_config_node.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		userDir := configDirs(t, c.Input.Args.String(t, 0), c.Input.Args.String(t, 1))
		node, err := yamlcfg.LoadConfigNode("cfg.yml", userDir)
		if c.Expected.Check(t, err) {
			return
		}
		if c.Expected.Files == "" {
			assert.Nil(t, node)
			return
		}
		var got, want any
		require.NoError(t, node.Decode(&got))
		require.NoError(t, yaml.Unmarshal([]byte(testyml.ReadFile(t, td, c.Expected.Files)), &want))
		assert.Equal(t, want, got)
	})
}

func TestLoadConfigDecode(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/load_config.test.spec.yml", func(t *testing.T, c testyml.Case[struct{}]) {
		userDir := configDirs(t, c.Input.Args.String(t, 0), "")
		var got map[string]int
		err := yamlcfg.LoadConfig("cfg.yml", userDir, &got)
		if c.Expected.Check(t, err) {
			return
		}
		if c.Expected.Files == "" {
			assert.Nil(t, got, "output must stay untouched for empty config")
			return
		}
		var want map[string]int
		require.NoError(t, yaml.Unmarshal([]byte(testyml.ReadFile(t, td, c.Expected.Files)), &want))
		assert.Equal(t, want, got)
	})
}

func TestUnwrapNonDocument(t *testing.T) {
	n := &yaml.Node{Kind: yaml.ScalarNode}
	assert.Same(t, n, yamlcfg.Unwrap(n))
	assert.Nil(t, yamlcfg.Unwrap(nil))
}

func TestLoadConfigNodeReadError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads 0o000 files")
	}
	testyml.Swap(t, &yamlcfg.SystemDir, filepath.Join(t.TempDir(), "no-system"))
	userDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "cfg.yml"), []byte("a: 1"), 0o000))
	_, err := yamlcfg.LoadConfigNode("cfg.yml", userDir)
	assert.Equal(t, yamlcfg.CodeConfig, yamlcfg.Code(err), "err: %v", err)
}

// [<] 🤖🤖
