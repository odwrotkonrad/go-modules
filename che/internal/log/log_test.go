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

// plain splits one stamped log line into type/subtype/text.
func plain(t *testing.T, line string) []string {
	t.Helper()
	m := plainRe.FindStringSubmatch(testutil.StripANSI(line[len("00:00:00.000: "):]))
	require.NotNilf(t, m, "output %q does not match HH:MM:SS.mmm: type(subtype): msg", line)
	return m
}

// msgWant is msg's expected.output: the printed line split into parts.
type msgWant struct {
	Type    string `yaml:"type"`
	Subtype string `yaml:"subtype"`
	Text    string `yaml:"text"`
}

func TestMsg(t *testing.T) {
	modes := map[string]DryRun{"off": Off, "delta": Delta, "all": All}
	testyml.Run(t, td, "testdata/spec/msg.test.spec.yml", func(t *testing.T, c testyml.Case[msgWant]) {
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

func TestDebugGateOff(t *testing.T) {
	t.Cleanup(func() { SetDebug(false) })
	SetDebug(false)
	out := capture(t, func() { Debug("plugin(p)", "run x", Off) })
	assert.Empty(t, out, "Debug with gate off must print nothing")
}

func TestBoldEmitted(t *testing.T) {
	out := capture(t, func() { Msg("ln", "/x", Off) })
	assert.Contains(t, out, "\x1b[1mln\x1b[", "output must bold the type")
}

// [<] 🤖🤖
