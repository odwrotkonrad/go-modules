// Package log emits che's structured log events: human prose to stdout
// (level-gated), machine events to the telemetry sink (always).
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

// [>] 🤖🤖 levels

// Level is the log verbosity: each level includes every level above it.
type Level int

// Levels namespaces the Level values, severity order: Error < Warn < Info <
// Debug < Trace.
var Levels = struct{ Error, Warn, Info, Debug, Trace Level }{0, 1, 2, 3, 4}

var levelNames = []string{"error", "warn", "info", "debug", "trace"}

// String renders the level word (severity text).
func (l Level) String() string {
	if l < 0 || int(l) >= len(levelNames) {
		return "info"
	}
	return levelNames[l]
}

// ParseLevel parses a CHE_LOG_LEVEL word into its Level.
func ParseLevel(s string) (Level, error) {
	for i, n := range levelNames {
		if s == n {
			return Level(i), nil
		}
	}
	return Levels.Info, fmt.Errorf("invalid log level %q: want error, warn, info, debug, or trace", s)
}

var current = Levels.Info

// SetLevel sets the package-level log level (--log-level / CHE_LOG_LEVEL).
func SetLevel(l Level) { current = l }

// GetLevel reads the current log level (per-profile override restore).
func GetLevel() Level { return current }

// IsEnabled reports whether l prints at the current level.
func IsEnabled(l Level) bool { return l <= current }

// [<] 🤖🤖 levels

// [>] 🤖🤖 events

// Event is one structured log event, consumed by both outputs: the human
// renderer (level-gated stdout prose) and the machine sink (OTLP).
type Event struct {
	Level  Level
	Scope  string // machine scope: discover-profiles, make-links, config, run...
	Action string // machine action token: created, overwritten, cloned, will-run...
	Msg    string // human sentence / path, multi-line allowed
	// Reasons name why the action will not happen; presence renders the human
	// line as "will not <action> <msg>: <reasons>".
	Reasons []string
	Attrs   map[string]string // machine-side attributes (profile, dest...)
	Indent  int               // human indentation depth (two spaces per step)
}

// sink is the telemetry hook: every emitted event mirrors to it regardless of
// the stdout level gate. nil -> stdout-only, the default.
var sink func(Event)

// SetSink installs (or clears, with nil) the machine-log mirror hook the OTLP
// log bridge registers; stdout output is unchanged either way.
func SetSink(fn func(Event)) { sink = fn }

// Emit mirrors e to the sink, then renders it to stdout when its level prints.
func Emit(e Event) {
	if sink != nil {
		sink(e)
	}
	if !IsEnabled(e.Level) {
		return
	}
	fmt.Print(renderHuman(e))
}

// renderHuman renders one event as indented prose: reasons render as
// "will not <action> <msg>: <reasons>", else "<action> <msg>" (action bold,
// hyphens displayed as spaces), a bare Msg as-is. Multi-line Msg indents each line.
func renderHuman(e Event) string {
	line := e.Msg
	switch {
	case len(e.Reasons) > 0:
		line = bold("will not "+displayAction(e.Action)) + " " + e.Msg + ": " + strings.Join(e.Reasons, ", ")
	case e.Action != "":
		line = bold(displayAction(e.Action)) + " " + e.Msg
	}
	pad := strings.Repeat("  ", e.Indent)
	var b strings.Builder
	for l := range strings.SplitSeq(line, "\n") {
		b.WriteString(pad)
		b.WriteString(l)
		b.WriteString("\n")
	}
	return b.String()
}

// displayAction renders a machine action token as prose: hyphens to spaces.
func displayAction(a string) string { return strings.ReplaceAll(a, "-", " ") }

// [<] 🤖🤖 events

// [>] 🤖🤖 emitters

// EmitError emits a failure event.
func EmitError(scope, action, msg string) {
	Emit(Event{Level: Levels.Error, Scope: scope, Action: action, Msg: msg})
}

// EmitWarn emits a warning event.
func EmitWarn(scope, action, msg string) {
	Emit(Event{Level: Levels.Warn, Scope: scope, Action: action, Msg: msg})
}

// EmitInfo emits a completed-fact event.
func EmitInfo(scope, action, msg string) {
	Emit(Event{Level: Levels.Info, Scope: scope, Action: action, Msg: msg})
}

// EmitDebug emits an intention / won't-happen event.
func EmitDebug(scope, action, msg string) {
	Emit(Event{Level: Levels.Debug, Scope: scope, Action: action, Msg: msg})
}

// EmitTrace emits a detail event.
func EmitTrace(scope, action, msg string) {
	Emit(Event{Level: Levels.Trace, Scope: scope, Action: action, Msg: msg})
}

// EmitSkip emits a won't-happen event with its reasons at the given level.
func EmitSkip(level Level, scope, action, msg string, reasons ...string) {
	Emit(Event{Level: level, Scope: scope, Action: action, Msg: msg, Reasons: reasons})
}

// [<] 🤖🤖 emitters

// [>] 🤖🤖 structural

// PrintHeading prints a bold human-only heading line at the given level
// (never mirrored to the sink).
func PrintHeading(l Level, text string) {
	if IsEnabled(l) {
		fmt.Println(bold(text))
	}
}

// PrintItem prints an indented human-only line at the given level (never
// mirrored to the sink).
func PrintItem(l Level, indent int, text string) {
	if IsEnabled(l) {
		fmt.Println(strings.Repeat("  ", indent) + text)
	}
}

func bold(s string) string { return boldC.Sprint(s) }

// [<] 🤖🤖 structural

// [<] 🤖🤖
