package log

// [>] 🤖🤖

import (
	"embed"
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
	return testutil.StripANSI(out)
}

func setLevel(t *testing.T, word string) {
	t.Helper()
	l, err := ParseLevel(word)
	require.NoError(t, err)
	prev := GetLevel()
	SetLevel(l)
	t.Cleanup(func() { SetLevel(prev) })
}

// eventFromArgs builds the Event a case's args describe.
func eventFromArgs(t *testing.T, a testyml.Args) Event {
	t.Helper()
	level, err := ParseLevel(a.String(t, 1))
	require.NoError(t, err)
	e := Event{Level: level, Scope: a.String(t, 2), Action: a.String(t, 3), Msg: a.String(t, 4)}
	for i := 5; i < 9; i++ {
		switch a.Name(i) {
		case "reasons":
			e.Reasons = a.Strings(t, i)
		case "depth":
			e.Depth = a.Int(t, i)
		case "heading":
			e.Heading = a.Int(t, i)
		case "dryRun":
			e.DryRun = a.Bool(t, i)
		}
	}
	return e
}

func TestEmit(t *testing.T) {
	testyml.Run(t, td, "testdata/spec/funcs/emit.test.spec.yml", func(t *testing.T, c testyml.Case[string]) {
		setLevel(t, c.Input.Args.String(t, 0))
		e := eventFromArgs(t, c.Input.Args)
		out := capture(t, func() { Emit(e) })
		assert.Equal(t, c.Expected.Output, out)
	})
}

func TestParseLevel(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/parse_level.test.spec.yml", func(t *testing.T, c testyml.Case[string]) (string, error) {
		l, err := ParseLevel(c.Input.Args.String(t, 0))
		if err != nil {
			return "", err
		}
		return l.String(), nil
	})
}

// TestEmitMirrorsSinkWhenGated asserts every event reaches the sink, the
// stdout gate notwithstanding.
func TestEmitMirrorsSinkWhenGated(t *testing.T) {
	setLevel(t, "error")
	var got []Event
	SetSink(func(e Event) { got = append(got, e) })
	t.Cleanup(func() { SetSink(nil) })
	out := capture(t, func() {
		Emit(Event{Level: Levels.Trace, Scope: "ledger", Action: "error", Msg: "boom"})
	})
	assert.Empty(t, out)
	require.Len(t, got, 1)
	assert.Equal(t, "ledger", got[0].Scope)
	assert.Equal(t, Levels.Trace, got[0].Level)
}

// TestEmitBoldsAction asserts the raw bytes carry SGR bold around the action.
func TestEmitBoldsAction(t *testing.T) {
	setLevel(t, "info")
	out, _ := testutil.CaptureStdout(t, func() error {
		Emit(Event{Level: Levels.Info, Scope: "make-links", Action: "created", Msg: "/x"})
		return nil
	})
	assert.Contains(t, out, "\x1b[1mcreated\x1b[")
}

// TestPrintHelpers asserts heading/item print human-only, level-gated.
func TestPrintHelpers(t *testing.T) {
	setLevel(t, "info")
	var sunk []Event
	SetSink(func(e Event) { sunk = append(sunk, e) })
	t.Cleanup(func() { SetSink(nil) })
	out := capture(t, func() {
		PrintHeading(Levels.Info, "profile plain:")
		PrintItem(Levels.Info, 1, "make-links: 1 change")
		PrintHeading(Levels.Debug, "hidden:")
		PrintItem(Levels.Debug, 1, "hidden")
	})
	assert.Equal(t, "profile plain:\n  make-links: 1 change\n", out)
	assert.Empty(t, sunk)
}

// [<] 🤖🤖
