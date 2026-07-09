package log

// [>] 🤖🤖

import (
	"embed"
	"regexp"
	"strings"
	"testing"

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

func plain(t *testing.T, line string) []string {
	t.Helper()
	m := plainRe.FindStringSubmatch(testutil.StripANSI(line[len("00:00:00.000: "):]))
	if m == nil {
		t.Fatalf("output %q does not match HH:MM:SS.mmm: type(subtype): msg", line)
	}
	return m
}

func TestMsg(t *testing.T) {
	type in struct {
		Fn, Title, Msg, Mode, Sub string
	}
	type want struct {
		Type, Subtype, Text string
	}
	type c struct {
		Name string
		In   in
		Want want
	}
	modes := map[string]DryRun{"off": Off, "delta": Delta, "all": All}
	testyml.Run(t, td, "testdata/spec/msg.spec.yml", func(t *testing.T, c c) {
		mode, ok := modes[c.In.Mode]
		if !ok {
			t.Fatalf("unknown mode %q", c.In.Mode)
		}
		var out string
		switch c.In.Fn {
		case "", "msg":
			out = capture(t, func() { Msg(c.In.Title, c.In.Msg, mode) })
		case "msgSub":
			out = capture(t, func() { MsgSub(c.In.Title, c.In.Msg, mode, c.In.Sub) })
		case "debug":
			SetDebug(true)
			t.Cleanup(func() { SetDebug(false) })
			out = capture(t, func() { Debug(c.In.Title, c.In.Msg, mode) })
		default:
			t.Fatalf("unknown fn %q", c.In.Fn)
		}
		m := plain(t, out)
		if m[1] != c.Want.Type || m[2] != c.Want.Subtype || m[3] != c.Want.Text {
			t.Errorf("type/subtype/msg = %q/%q/%q, want %+v", m[1], m[2], m[3], c.Want)
		}
	})
}

func TestDebugGateOff(t *testing.T) {
	t.Cleanup(func() { SetDebug(false) })
	SetDebug(false)
	if out := capture(t, func() { Debug("plugin(p)", "run x", Off) }); out != "" {
		t.Errorf("Debug with gate off printed %q, want nothing", out)
	}
}

func TestBoldEmitted(t *testing.T) {
	out := capture(t, func() { Msg("ln", "/x", Off) })
	if !strings.Contains(out, "\x1b[1mln\x1b[") {
		t.Errorf("output %q must bold the type", out)
	}
}

// [<] 🤖🤖
