package fsutil

// [>] 🤖🤖

import (
	"slices"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"HOME/.config/zsh/**", "HOME/.config/zsh/.zshrc", true},
		{"HOME/.config/zsh/**", "HOME/.config/zsh/zshrc.d/auto.d/80-tools.zsh", true},
		{"HOME/.config/zsh/**", "HOME/.config/zsh", true},
		{"HOME/.config/zsh/**", "HOME/.config/git/config", false},
		{"HOME/.config/zsh/**", "HOME/.config/zshenv", false},
		{"etc/zshrc", "etc/zshrc", true},
		{"etc/zshrc", "etc/zshrc.d/auto.d/x", false},
		{"Library/LaunchDaemons/otelcol.plist*", "Library/LaunchDaemons/otelcol.plist", true},
		{"Library/LaunchDaemons/otelcol.plist*", "Library/LaunchDaemons/otelcol.plist.host.cp", true},
		{"Library/LaunchDaemons/otelcol.plist*", "Library/LaunchDaemons/grafana.plist", false},
		{"etc/otelcol/**", "etc/otelcol/config.yml", true},
	}
	for _, c := range cases {
		if got := MatchGlob(c.pattern, c.path); got != c.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestExpandBraces(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"no/braces/here", []string{"no/braces/here"}},
		{"x/{a,b}/y", []string{"x/a/y", "x/b/y"}},
		{"{a,b}/{c,d}", []string{"a/c", "a/d", "b/c", "b/d"}},
		{"x/{a,{b,c}}/y", []string{"x/a/y", "x/b/y", "x/c/y"}},
	}
	for _, c := range cases {
		if got := ExpandBraces(c.in); !slices.Equal(got, c.want) {
			t.Errorf("ExpandBraces(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// [<] 🤖🤖
