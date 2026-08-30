package cli

// [>] 🤖🤖

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/term"

	"gitlab.com/konradodwrot/go-modules/che/internal/che"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

type targetPrompt struct {
	in         io.Reader
	out        io.Writer
	isTerminal func() bool
}

func stdioPrompt() targetPrompt {
	return targetPrompt{
		in:  os.Stdin,
		out: os.Stdout,
		isTerminal: func() bool {
			return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
		},
	}
}

type typeSummary struct {
	Type     string
	Profiles int
	Counts   map[string]int
}

var opNouns = []struct{ op, noun string }{
	{"make-links", "link"},
	{"make-copies", "copy"},
	{"render-templates", "render"},
	{"make-dirs", "dir"},
	{"install-packages", "package"},
	{"run-scripts", "script"},
}

func (s typeSummary) String() string {
	parts := []string{fmt.Sprintf("%d %s", s.Profiles, plural(s.Profiles, "profile"))}
	for _, on := range opNouns {
		if n := s.Counts[on.op]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, plural(n, on.noun)))
		}
	}
	return s.Type + ": " + strings.Join(parts, ", ")
}

func plural(n int, noun string) string {
	switch {
	case n == 1:
		return noun
	case noun == "copy":
		return "copies"
	}
	return noun + "s"
}

func summarizeTypes(profiles []*che.ProfileReady) []typeSummary {
	by := map[string]*typeSummary{}
	for _, p := range profiles {
		key := string(p.Type)
		s, ok := by[key]
		if !ok {
			s = &typeSummary{Type: key, Counts: map[string]int{}}
			by[key] = s
		}
		s.Profiles++
		for op, n := range p.OpCounts() {
			s.Counts[op] += n
		}
	}
	var out []typeSummary
	for _, name := range append(slices.Clone(spec.ProfileTypeNames), "") {
		if s, ok := by[name]; ok {
			out = append(out, *s)
		}
	}
	return out
}

// [why] a run selects by type: on a TTY with nothing chosen yet, the user picks; anywhere else
// every type runs, CI passes one type explicitly
func (a *app) selectTargetTypes() error {
	if len(a.opts.Profiles) > 0 || len(a.opts.TargetProfileTypes) > 0 || !a.prompt.isTerminal() {
		return nil
	}
	summaries := summarizeTypes(a.specs.AllProfiles())
	typed := slices.DeleteFunc(slices.Clone(summaries), func(s typeSummary) bool { return s.Type == "" })
	if len(typed) < 2 {
		return nil
	}
	chosen, err := a.prompt.choose(summaries)
	if err != nil {
		return err
	}
	a.opts.TargetProfileTypes = chosen
	a.specs, err = che.PrepareSpecs(a.ctx, a.opts, spec.SpecSourceRecipe{})
	return err
}

func (t targetPrompt) choose(summaries []typeSummary) ([]string, error) {
	var typed []typeSummary
	var menu strings.Builder
	menu.WriteString("Discovered profile types:\n")
	for _, s := range summaries {
		if s.Type == "" {
			fmt.Fprintf(&menu, "  -  %s  (no type: set type: on each, skipped by any choice)\n", s.String())
			continue
		}
		typed = append(typed, s)
		fmt.Fprintf(&menu, "  %d) %s\n", len(typed), s.String())
	}
	menu.WriteString("Run which types? (numbers, comma-separated, or all; empty aborts): ")
	if _, err := io.WriteString(t.out, menu.String()); err != nil {
		return nil, err
	}
	line, err := bufio.NewReader(t.in).ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}
	return parseTypeChoice(strings.TrimSpace(line), typed)
}

func parseTypeChoice(answer string, typed []typeSummary) ([]string, error) {
	if answer == "" {
		return nil, fmt.Errorf("no profile type chosen")
	}
	var out []string
	for _, s := range typed {
		if answer == "all" {
			out = append(out, s.Type)
		}
	}
	if answer == "all" {
		return out, nil
	}
	for _, field := range strings.Split(answer, ",") {
		i, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || i < 1 || i > len(typed) {
			return nil, fmt.Errorf("invalid profile type choice %q: want 1-%d or all", field, len(typed))
		}
		if !slices.Contains(out, typed[i-1].Type) {
			out = append(out, typed[i-1].Type)
		}
	}
	return out, nil
}

// [<] 🤖🤖
