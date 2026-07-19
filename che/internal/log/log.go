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
func Debug(title, msg string) {
	if sink != nil {
		sink(title, msg, "debug")
	}
	if !debugOn {
		return
	}
	fmt.Printf("debug %s: %s\n", formatTitle(title, ""), msg)
}

// Msg prints '<title>: <msg>', title formatted as type(subtype): type bold,
// "(subtype)" plain.
func Msg(title, msg string) { MsgSub(title, msg, "") }

// MsgSub: Msg with an extra trailing subtype word (e.g. "profile=<name>"),
// comma-joined last.
func MsgSub(title, msg, sub string) {
	if sink != nil {
		sink(title, msg, "info")
	}
	fmt.Printf("%s: %s\n", formatTitle(title, sub), msg)
}

// formatTitle renders type(subtype); the dry-run mode is never folded in: dry
// run announces once at output start instead (spec/che/LogBehavior.md).
func formatTitle(title, sub string) string {
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
