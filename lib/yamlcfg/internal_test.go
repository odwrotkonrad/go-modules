package yamlcfg

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomPathsEnvFallbacks(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	assert.Equal(t, filepath.Join("/xdg", "custom", "cfg.yml"), customPaths("cfg.yml", "")[1])
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/h")
	assert.Equal(t, filepath.Join("/h", ".config", "custom", "cfg.yml"), customPaths("cfg.yml", "")[1])
}

// [<] 🤖🤖
