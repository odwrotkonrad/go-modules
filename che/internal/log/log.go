package log

// [>] 🤖🤖

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

var boldC = func() *color.Color {
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
	for i, n := range levelNames {
		if s == n {
			return Level(i), nil
		}
	}
	return Levels.Info, fmt.Errorf("invalid log level %q: want error, warn, info, debug, or trace", s)
}

var current = Levels.Info

func SetLevel(l Level) { current = l }

func GetLevel() Level { return current }

func IsEnabled(l Level) bool { return l <= current }

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
	pad := strings.Repeat("  ", e.Depth)
	var b strings.Builder
	for l := range strings.SplitSeq(line, "\n") {
		b.WriteString(prefix)
		b.WriteString(pad)
		b.WriteString(l)
		b.WriteString("\n")
	}
	return b.String()
}

func levelPrefix(l Level) string {
	switch l {
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

func displayAction(a string) string { return strings.ReplaceAll(a, "-", " ") }

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

func PrintHeading(l Level, text string) {
	if IsEnabled(l) {
		fmt.Println(bold(text))
	}
}

func PrintItem(l Level, indent int, text string) {
	if IsEnabled(l) {
		fmt.Println(strings.Repeat("  ", indent) + text)
	}
}

func bold(s string) string { return boldC.Sprint(s) }

// [<] 🤖🤖 structural

// [<] 🤖🤖
