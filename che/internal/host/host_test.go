package host

// [>] 🤖🤖

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/config"
	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

func TestToDest(t *testing.T) {
	h := Host{Home: "/Users/x"}
	cases := map[string]string{
		"HOME":                                "/Users/x",
		"HOME/.config/zsh":                    "/Users/x/.config/zsh",
		"etc/zshrc":                           "/etc/zshrc",
		"/var/custom/cache/dir_size_exporter": "/var/custom/cache/dir_size_exporter",
		"/var/log/otelcol":                    "/var/log/otelcol",
	}
	for in, want := range cases {
		assert.Equal(t, want, h.ToDest(in), "ToDest(%q)", in)
	}
}

func TestPrepend(t *testing.T) {
	assert.Contains(t, prepend([]string{"PATH=foo", "OTHER=x"}, "PATH", "bar"), "PATH=bar:foo", "existing: value prepended with ':'")
	assert.Contains(t, prepend([]string{"OTHER=x"}, "PATH", "bar"), "PATH=bar", "absent: appended bare")
}

func TestSrc(t *testing.T) {
	h := New("/repo", "/Users/x", "cli/macos", config.Config{})
	assert.Equal(t, "/repo/root/etc/zshrc", h.Src("etc/zshrc"))
}

// TestResolveScripts: spec order kept, rels mapped to abs, missing errors.
func TestResolveScripts(t *testing.T) {
	scripts := []string{
		"ci/zsh/scripts/installs/90-kitty.zsh",
		"ci/zsh/scripts/installs/00-ci-deps.zsh",
	}
	dir := testutil.Tree(t, map[string]string{
		scripts[0]: "#!/bin/sh\n",
		scripts[1]: "#!/bin/sh\n",
	})
	h := New(dir, "/Users/x", "cli/macos", config.Config{})

	got, err := h.ResolveScripts(scripts)
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(dir, scripts[0]), filepath.Join(dir, scripts[1])}, got)

	_, err = h.ResolveScripts([]string{"ci/zsh/scripts/installs/99-absent.zsh"})
	assert.Error(t, err, "missing script must error")
}

// [<] 🤖🤖
