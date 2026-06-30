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

// Msg prints 'HH:MM:SS.mmm: <title>: <msg>', matching zsh fn-log-msg.
// Title formats as type(subtype): the type is bold, "(subtype)" plain. Dry-run
// is folded into the subtype as "dry-run" (comma-joined onto any existing one).
func Msg(title, msg string, dryRun bool) {
	stamp := time.Now().Format("15:04:05.000")
	fmt.Printf("%s: %s: %s\n", stamp, formatTitle(title, dryRun), msg)
}

// formatTitle bolds the type and renders a plain "(subtype)". An existing
// "type(subtype)" title is parsed; dryRun adds a "dry-run" subtype.
func formatTitle(title string, dryRun bool) string {
	t, subt, _ := strings.Cut(strings.TrimSuffix(title, ")"), "(")
	var subts []string
	if subt != "" {
		subts = append(subts, subt)
	}
	if dryRun {
		subts = append(subts, "dry-run")
	}
	if len(subts) == 0 {
		return bold(t)
	}
	return bold(t) + "(" + strings.Join(subts, ",") + ")"
}

// bold wraps s in the SGR bold/reset pair, matching zsh fn-print-with bold.
func bold(s string) string { return boldC.Sprint(s) }

// [<] 🤖🤖
