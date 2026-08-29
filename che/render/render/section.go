package render

// [>] 🤖🤖

import (
	"cmp"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

type section struct {
	name  string
	open  int
	close int
	depth int
}

type sectionMarkers struct {
	open  *regexp.Regexp
	close *regexp.Regexp
}

func newSectionMarkers(prefix string) sectionMarkers {
	quoted := regexp.QuoteMeta(prefix)
	return sectionMarkers{
		open:  regexp.MustCompile(`^\s*` + quoted + `\s*\[>\]\s+(\S+)`),
		close: regexp.MustCompile(`^\s*` + quoted + `\s*\[<\]\s+(\S+)`),
	}
}

func (m sectionMarkers) name(re *regexp.Regexp, line string) (string, bool) {
	match := re.FindStringSubmatch(line)
	if match == nil {
		return "", false
	}
	return match[1], true
}

// sectionsParse lists every `[>] name` .. `[<] name` section of body, outermost first, by open line.
func sectionsParse(body, prefix string) ([]section, error) {
	markers := newSectionMarkers(prefix)
	lines := strings.Split(body, "\n")
	var stack []section
	var out []section
	for i, line := range lines {
		if name, ok := markers.name(markers.open, line); ok {
			stack = append(stack, section{name: name, open: i, depth: len(stack)})
			continue
		}
		name, ok := markers.name(markers.close, line)
		if !ok {
			continue
		}
		if len(stack) == 0 {
			return nil, fmt.Errorf("line %d: close of section %q without an open", i+1, name)
		}
		top := stack[len(stack)-1]
		if top.name != name {
			return nil, fmt.Errorf("line %d: close of section %q while %q is open", i+1, name, top.name)
		}
		top.close = i
		out = append(out, top)
		stack = stack[:len(stack)-1]
	}
	if len(stack) > 0 {
		return nil, fmt.Errorf("section %q opened at line %d never closes", stack[len(stack)-1].name, stack[len(stack)-1].open+1)
	}
	slices.SortFunc(out, func(a, b section) int { return cmp.Compare(a.open, b.open) })
	return out, nil
}

func topLevel(sections []section) []section {
	var out []section
	for _, s := range sections {
		if s.depth == 0 {
			out = append(out, s)
		}
	}
	return out
}

// sectionsInject replaces, in target, every top-level section rendered carries, by name, markers kept.
// An empty target takes rendered whole. A rendered section absent from target errors, target untouched.
func sectionsInject(target, rendered, prefix string) (string, error) {
	if target == "" {
		return rendered, nil
	}
	renderedSections, err := sectionsParse(rendered, prefix)
	if err != nil {
		return "", fmt.Errorf("rendered body: %w", err)
	}
	targetSections, err := sectionsParse(target, prefix)
	if err != nil {
		return "", fmt.Errorf("existing dest: %w", err)
	}
	renderedLines := strings.Split(rendered, "\n")
	targetLines := strings.Split(target, "\n")
	replacements := map[string][]string{}
	for _, s := range topLevel(renderedSections) {
		if _, dup := replacements[s.name]; dup {
			return "", fmt.Errorf("rendered body: section %q appears twice", s.name)
		}
		replacements[s.name] = renderedLines[s.open+1 : s.close]
	}
	if len(replacements) == 0 {
		return "", fmt.Errorf("rendered body carries no %s [>] <name> section", prefix)
	}
	byName := map[string]section{}
	for _, s := range targetSections {
		if _, seen := byName[s.name]; !seen {
			byName[s.name] = s
		}
	}
	var missing []string
	for _, name := range slices.Sorted(maps.Keys(replacements)) {
		if _, ok := byName[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("existing dest lacks section(s) %s", strings.Join(missing, ", "))
	}
	return injectLines(targetLines, byName, replacements), nil
}

func injectLines(targetLines []string, byName map[string]section, replacements map[string][]string) string {
	var out []string
	for i := 0; i < len(targetLines); i++ {
		s, ok := sectionAt(byName, replacements, i)
		if !ok {
			out = append(out, targetLines[i])
			continue
		}
		out = append(out, targetLines[s.open])
		out = append(out, replacements[s.name]...)
		out = append(out, targetLines[s.close])
		i = s.close
	}
	return strings.Join(out, "\n")
}

func sectionAt(byName map[string]section, replacements map[string][]string, line int) (section, bool) {
	for name, s := range byName {
		if s.open == line {
			_, replaced := replacements[name]
			return s, replaced
		}
	}
	return section{}, false
}

// [<] 🤖🤖
