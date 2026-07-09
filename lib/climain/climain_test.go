// [>] 🤖🤖
package climain

import (
	"errors"
	"testing"

	"gitlab.com/konradodwrot/go-modules/lib/yamlcfg"
)

func TestHelpVersion(t *testing.T) {
	cases := []struct {
		name string
		args []string
		out  string
		done bool
	}{
		{"help_long", []string{"--help"}, "usage text", true},
		{"help_short", []string{"-h"}, "usage text", true},
		{"version_long", []string{"--version"}, "tool version 1.2.3", true},
		{"version_short", []string{"-v"}, "tool version 1.2.3", true},
		{"no_args", nil, "", false},
		{"other_arg", []string{"any"}, "", false},
		{"help_plus_extra", []string{"--help", "x"}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, done := HelpVersion(c.args, "usage text", "tool", "1.2.3")
			if done != c.done || out != c.out {
				t.Fatalf("got (%q, %v) want (%q, %v)", out, done, c.out, c.done)
			}
		})
	}
}

func TestExitCode(t *testing.T) {
	if got := exitCode(nil); got != 0 {
		t.Errorf("nil: got %d", got)
	}
	if got := exitCode(&yamlcfg.CodedError{Code: 13, Msg: "x"}); got != 13 {
		t.Errorf("coded: got %d", got)
	}
	if got := exitCode(errors.New("plain")); got != 1 {
		t.Errorf("plain: got %d", got)
	}
}

//[<] 🤖🤖
