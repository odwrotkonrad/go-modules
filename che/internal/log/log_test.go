package log

// [>] 🤖🤖

import (
	"embed"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

func capture(t *testing.T, fn func()) string {
	t.Helper()
	out, _ := testutil.CaptureStdout(t, func() error { fn(); return nil })
	return out
}

var plainRe = regexp.MustCompile(`^([^(:]+)(\([^:]*\))?: (.*)\n$`)

// plain splits one log line into type/subtype/text.
func plain(t *testing.T, line string) []string {
	t.Helper()
	m := plainRe.FindStringSubmatch(testutil.StripANSI(line))
	require.NotNilf(t, m, "output %q does not match type(subtype): msg", line)
	return m
}

// msgWant is msg's expected.output: the printed line split into parts.
type msgWant struct {
	Type    string `yaml:"type"`
	Subtype string `yaml:"subtype"`
	Text    string `yaml:"text"`
}

var modes = map[string]DryRun{"off": Off, "delta": Delta, "all": All}

func TestMsg(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/msg.test.spec.yml", func(t *testing.T, c testyml.Case[msgWant]) {
		a := c.Input.Args
		title, msg := a.String(t, 0), a.String(t, 1)
		mode, ok := modes[a.String(t, 2)]
		require.Truef(t, ok, "unknown mode %q", a.String(t, 2))
		var out string
		switch c.Context.Function {
		case "log.Msg":
			out = capture(t, func() { Msg(title, msg, mode) })
		case "log.MsgSub":
			out = capture(t, func() { MsgSub(title, msg, mode, a.String(t, 3)) })
		case "log.Debug":
			SetDebug(true)
			t.Cleanup(func() { SetDebug(false) })
			out = capture(t, func() { Debug(title, msg, mode) })
		default:
			t.Fatalf("unknown function %q", c.Context.Function)
		}
		m := plain(t, out)
		assert.Equal(t, c.Expected.Output, msgWant{Type: m[1], Subtype: m[2], Text: m[3]})
	})
}

// TestMsgRaw asserts the raw emitted bytes: the debug gate silences Debug,
// Msg bolds the type.
func TestMsgRaw(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/msg_raw.test.spec.yml", func(t *testing.T, c testyml.Case[string]) {
		a := c.Input.Args
		mode, ok := modes[a.String(t, 2)]
		require.Truef(t, ok, "unknown mode %q", a.String(t, 2))
		var out string
		switch c.Context.Function {
		case "log.Msg":
			out = capture(t, func() { Msg(a.String(t, 0), a.String(t, 1), mode) })
		case "log.Debug":
			t.Cleanup(func() { SetDebug(false) })
			SetDebug(false)
			out = capture(t, func() { Debug(a.String(t, 0), a.String(t, 1), mode) })
		default:
			t.Fatalf("unknown function %q", c.Context.Function)
		}
		if len(c.Expected.StdOut) > 0 {
			for _, m := range c.Expected.StdOut {
				testyml.MustMatch(t, out, m)
			}
			return
		}
		assert.Equal(t, c.Expected.Output, out)
	})
}

// [<] 🤖🤖
