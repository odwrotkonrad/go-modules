package log

// [>] 🤖🤖

import (
	"regexp"
	"strings"
	"testing"

	"gitlab.com/konradodwrot/go-modules/che/internal/testutil"
)

func capture(t *testing.T, fn func()) string {
	t.Helper()
	out, _ := testutil.CaptureStdout(t, func() error { fn(); return nil })
	return out
}

// plainRe matches a stamp-stripped, ANSI-stripped line as 'type(subtype): msg'.
var plainRe = regexp.MustCompile(`^([^(:]+)(\([^:]*\))?: (.*)\n$`)

// plain captures an emitted line as type/subtype/msg, escapes and stamp removed.
func plain(t *testing.T, line string) []string {
	t.Helper()
	m := plainRe.FindStringSubmatch(testutil.StripANSI(line[len("00:00:00.000: "):]))
	if m == nil {
		t.Fatalf("output %q does not match HH:MM:SS.mmm: type(subtype): msg", line)
	}
	return m
}

func TestMsgFormat(t *testing.T) {
	m := plain(t, capture(t, func() { Msg("ln", "/etc/zshrc", Off) }))
	if m[1] != "ln" || m[2] != "" || m[3] != "/etc/zshrc" {
		t.Errorf("type/subtype/msg = %q/%q/%q, want ln//.../etc/zshrc", m[1], m[2], m[3])
	}
}

func TestMsgSubtypedTitle(t *testing.T) {
	m := plain(t, capture(t, func() { Msg("render(dry-run-render-secrets)", "/x", Off) }))
	if m[1] != "render" || m[2] != "(dry-run-render-secrets)" || m[3] != "/x" {
		t.Errorf("type/subtype/msg = %q/%q/%q, want render/(dry-run-render-secrets)//x", m[1], m[2], m[3])
	}
}

func TestMsgDryRunSubtype(t *testing.T) {
	m := plain(t, capture(t, func() { Msg("cp", "/x", Delta) }))
	if m[1] != "cp" || m[2] != "(dry-run=delta)" || m[3] != "/x" {
		t.Errorf("delta line type/subtype/msg = %q/%q/%q, want cp/(dry-run=delta)//x", m[1], m[2], m[3])
	}
	all := plain(t, capture(t, func() { Msg("cp", "/x", All) }))
	if all[2] != "(dry-run=all)" {
		t.Errorf("all line subtype = %q, want (dry-run=all)", all[2])
	}
	wet := capture(t, func() { Msg("cp", "/x", Off) })
	if strings.Contains(wet, "dry-run") {
		t.Errorf("non-dry-run line %q must not carry a dry-run subtype", wet)
	}
}

func TestMsgDryRunCombinesSubtype(t *testing.T) {
	m := plain(t, capture(t, func() { Msg("render(dry-run-render-secrets)", "/x", Delta) }))
	if m[2] != "(dry-run-render-secrets,dry-run=delta)" {
		t.Errorf("dry-run subtype = %q, want comma-joined (dry-run-render-secrets,dry-run=delta)", m[2])
	}
}

// TestBoldEmitted guards that the type is wrapped in SGR bold, escapes intact.
func TestBoldEmitted(t *testing.T) {
	out := capture(t, func() { Msg("ln", "/x", Off) })
	if !strings.Contains(out, "\x1b[1mln\x1b[") {
		t.Errorf("output %q must bold the type", out)
	}
}

// [<] 🤖🤖
