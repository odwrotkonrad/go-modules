package log

// [>] 🤖🤖

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

// boldC emits the SGR bold pair unconditionally (matching zsh fn-print-with
// bold), even when stdout is not a tty.
var boldC = func() *color.Color {
	c := color.New(color.Bold)
	c.EnableColor()
	return c
}()

// DryRun is the dry-run mode folded into a log subtype: Off (no subtype), Delta
// ("dry-run=delta"), All ("dry-run=all"). The label words match che's --dry-run
// CLI flag.
type DryRun int

const (
	Off DryRun = iota
	Delta
	All
)

// subtype renders the "dry-run=<mode>" subtype word ("" when Off).
func (d DryRun) subtype() string {
	switch d {
	case Delta:
		return "dry-run=delta"
	case All:
		return "dry-run=all"
	default:
		return ""
	}
}

// debugOn gates Debug lines, set once at startup (--debug / CHE_DEBUG).
var debugOn bool

// SetDebug flips the package-level debug gate.
func SetDebug(on bool) { debugOn = on }

// Debug prints like Msg, but only when the debug gate is on.
func Debug(title, msg string, dr DryRun) {
	if !debugOn {
		return
	}
	Msg(title, msg, dr)
}

// Msg prints 'HH:MM:SS.mmm: <title>: <msg>', matching zsh fn-log-msg.
// Title formats as type(subtype): the type is bold, "(subtype)" plain. A dry-run
// mode is folded into the subtype as "dry-run=<mode>" (comma-joined onto any
// existing one).
func Msg(title, msg string, dr DryRun) { MsgSub(title, msg, dr, "") }

// MsgSub prints like Msg with an extra trailing subtype word (e.g.
// "profile=<name>"), comma-joined after the dry-run subtype. Empty sub -> Msg.
func MsgSub(title, msg string, dr DryRun, sub string) {
	stamp := time.Now().Format("15:04:05.000")
	fmt.Printf("%s: %s: %s\n", stamp, formatTitle(title, dr, sub), msg)
}

// formatTitle bolds the type and renders a plain "(subtype)". An existing
// "type(subtype)" title is parsed; a set dry-run mode adds a "dry-run=<mode>"
// subtype, a set sub appends last.
func formatTitle(title string, dr DryRun, sub string) string {
	t, subt, _ := strings.Cut(strings.TrimSuffix(title, ")"), "(")
	var subts []string
	if subt != "" {
		subts = append(subts, subt)
	}
	if s := dr.subtype(); s != "" {
		subts = append(subts, s)
	}
	if sub != "" {
		subts = append(subts, sub)
	}
	if len(subts) == 0 {
		return bold(t)
	}
	return bold(t) + "(" + strings.Join(subts, ",") + ")"
}

// bold wraps s in the SGR bold/reset pair, matching zsh fn-print-with bold.
func bold(s string) string { return boldC.Sprint(s) }

// [<] 🤖🤖
