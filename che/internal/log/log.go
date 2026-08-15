package log

// [>] 🤖🤖

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

var boldColor = func() *color.Color {
	c := color.New(color.Bold)
	c.EnableColor()
	return c
}()

// [>] 🤖🤖 levels

type Level int

var Levels = struct{ Error, Warn, Info, Debug, Trace Level }{0, 1, 2, 3, 4}

var levelNames = []string{"error", "warn", "info", "debug", "trace"}

func (l Level) String() string {
	if l < 0 || int(l) >= len(levelNames) {
		return "info"
	}
	return levelNames[l]
}

func ParseLevel(s string) (Level, error) {
	for i, name := range levelNames {
		if s == name {
			return Level(i), nil
		}
	}
	return Levels.Info, fmt.Errorf("invalid log level %q: want error, warn, info, debug, or trace", s)
}

var current = Levels.Info

func SetLevel(level Level) { current = level }

func GetLevel() Level { return current }

func SwapLevel(level Level) func() {
	prev := current
	current = level
	return func() { current = prev }
}

func IsEnabled(level Level) bool { return level <= current }

// [<] 🤖🤖 levels

// [>] 🤖🤖 events

type Event struct {
	Level   Level
	Scope   string
	Action  string
	Msg     string
	Reasons []string
	DryRun  bool
	Attrs   map[string]string
	Heading int
	Depth   int
}

var sink func(Event)

func SetSink(fn func(Event)) { sink = fn }

func Emit(e Event) {
	if sink != nil {
		sink(e)
	}
	if !IsEnabled(e.Level) {
		return
	}
	fmt.Print(renderHuman(e))
}

func renderHuman(e Event) string {
	prefix := levelPrefix(e.Level)
	if e.Heading > 0 {
		return prefix + bold(strings.Repeat("#", e.Heading)+" "+e.Msg) + "\n"
	}
	line := e.Msg
	switch {
	case len(e.Reasons) > 0:
		line = bold("will not "+displayAction(e.Action)) + " " + e.Msg + ": " + strings.Join(e.Reasons, ", ")
	case e.DryRun && e.Action != "":
		line = bold(displayAction(e.Action)) + " " + e.Msg + " (dry run)"
	case e.Action != "":
		line = bold(displayAction(e.Action)) + " " + e.Msg
	}
	indent := strings.Repeat("  ", e.Depth)
	var out strings.Builder
	for wrapped := range strings.SplitSeq(line, "\n") {
		out.WriteString(prefix)
		out.WriteString(indent)
		out.WriteString(wrapped)
		out.WriteString("\n")
	}
	return out.String()
}

func levelPrefix(level Level) string {
	switch level {
	case Levels.Error:
		return "[error] "
	case Levels.Warn:
		return "[warn] "
	case Levels.Debug:
		return "[debug] "
	case Levels.Trace:
		return "[trace] "
	default:
		return ""
	}
}

func displayAction(action string) string { return strings.ReplaceAll(action, "-", " ") }

// [<] 🤖🤖 events

// [>] 🤖🤖 emitters

func EmitError(scope, action, msg string) {
	Emit(Event{Level: Levels.Error, Scope: scope, Action: action, Msg: msg})
}

func EmitWarn(scope, action, msg string) {
	Emit(Event{Level: Levels.Warn, Scope: scope, Action: action, Msg: msg})
}

func EmitInfo(scope, action, msg string) {
	Emit(Event{Level: Levels.Info, Scope: scope, Action: action, Msg: msg})
}

func EmitDebug(scope, action, msg string) {
	Emit(Event{Level: Levels.Debug, Scope: scope, Action: action, Msg: msg})
}

func EmitTrace(scope, action, msg string) {
	Emit(Event{Level: Levels.Trace, Scope: scope, Action: action, Msg: msg})
}

func EmitSkip(level Level, scope, action, msg string, reasons ...string) {
	Emit(Event{Level: level, Scope: scope, Action: action, Msg: msg, Reasons: reasons})
}

func EmitHeading(level Level, heading int, scope, action, msg string) {
	Emit(Event{Level: level, Scope: scope, Action: action, Msg: msg, Heading: heading})
}

// [<] 🤖🤖 emitters

// [>] 🤖🤖 structural

func PrintHeading(level Level, text string) {
	if IsEnabled(level) {
		fmt.Println(bold(text))
	}
}

func PrintItem(level Level, indent int, text string) {
	if IsEnabled(level) {
		fmt.Println(strings.Repeat("  ", indent) + text)
	}
}

func bold(s string) string { return boldColor.Sprint(s) }

// [<] 🤖🤖 structural

// [<] 🤖🤖
