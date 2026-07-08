// [>] 🤖🤖
package checkcmd

import "testing"

func TestRunDispatch(t *testing.T) {
	noArg := Tool{Usage: "u\n", Generate: func(string) (string, error) { return "out", nil }}
	withArg := Tool{Usage: "u\n", NeedsArg: true, Generate: func(string) (string, error) { return "out", nil }}

	cases := []struct {
		name string
		tool Tool
		args []string
		want int
	}{
		{"help short", noArg, []string{"-h"}, 0},
		{"help long", noArg, []string{"--help"}, 0},
		{"no-arg emit", noArg, []string{}, 0},
		{"no-arg rejects positional", noArg, []string{"x"}, 11},
		{"needs-arg emit", withArg, []string{"x"}, 0},
		{"needs-arg rejects empty", withArg, []string{}, 11},
		{"needs-arg rejects flag", withArg, []string{"-x"}, 11},
		{"unknown flag", noArg, []string{"--nope", "y"}, 11},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if code := c.tool.Run(c.args); code != c.want {
				t.Errorf("Run(%v) = %d, want %d", c.args, code, c.want)
			}
		})
	}
}

//[<] 🤖🤖
