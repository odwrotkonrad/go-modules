// Package log prints che's op log lines.
package log

// [>] 🤖🤖

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// boldC: SGR bold even when stdout is not a tty (matches zsh fn-print-with).
var boldC = func() *color.Color {
	c := color.New(color.Bold)
	c.EnableColor()
	return c
}()

// DryRun is the legacy dry-run mode parameter the call sites still thread;
// rendering ignores it (the dry-run announce replaced the subtypes).
type DryRun int

const (
	Off DryRun = iota
	Delta
	All
)

var debugOn bool

// SetDebug flips the package-level debug gate (--debug / CHE_DEBUG).
func SetDebug(on bool) { debugOn = on }

// sink is the optional telemetry hook: every emitted line is mirrored to it
// (title, msg, level "info"|"debug"). nil -> stdout-only, the default.
var sink func(title, msg, level string)

// SetSink installs (or clears, with nil) the log mirror hook the OTLP log bridge
// registers; stdout output is unchanged either way.
func SetSink(fn func(title, msg, level string)) { sink = fn }

// Debug: Msg, gated by SetDebug, the printed line prefixed with "debug "
// (info lines carry no prefix). Always mirrored to the sink (level "debug"),
// even when the stdout gate is off.
func Debug(title, msg string, dr DryRun) {
	if sink != nil {
		sink(title, msg, "debug")
	}
	if !debugOn {
		return
	}
	fmt.Printf("debug %s: %s\n", formatTitle(title, dr, ""), msg)
}

// Msg prints '<title>: <msg>', matching zsh fn-log-msg.
// Title formats as type(subtype): type bold, "(subtype)" plain, the dry-run
// mode comma-joined onto any existing subtype.
func Msg(title, msg string, dr DryRun) { MsgSub(title, msg, dr, "") }

// MsgSub: Msg with an extra trailing subtype word (e.g. "profile=<name>"),
// comma-joined last.
func MsgSub(title, msg string, dr DryRun, sub string) {
	if sink != nil {
		sink(title, msg, "info")
	}
	print(title, msg, dr, sub)
}

// print writes the formatted line to stdout (the sink mirror is the caller's).
func print(title, msg string, dr DryRun, sub string) {
	fmt.Printf("%s: %s\n", formatTitle(title, dr, sub), msg)
}

// formatTitle renders type(subtype), the dry-run mode NOT folded in: dry run
// announces once at output start instead (spec/che/LogBehavior.md).
func formatTitle(title string, _ DryRun, sub string) string {
	t, subt, _ := strings.Cut(strings.TrimSuffix(title, ")"), "(")
	var subts []string
	if subt != "" {
		subts = append(subts, subt)
	}
	if sub != "" {
		subts = append(subts, sub)
	}
	if len(subts) == 0 {
		return bold(t)
	}
	return bold(t) + "(" + strings.Join(subts, ",") + ")"
}

func bold(s string) string { return boldC.Sprint(s) }

// [<] 🤖🤖
