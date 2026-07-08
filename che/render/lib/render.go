package lib

import "strings"

// [>] 🤖🤖🤖
func render(sections []section) string {
	var b strings.Builder
	b.WriteString("## `./Makefile`\n")
	for _, s := range sections {
		b.WriteString("\n")
		b.WriteString(strings.Repeat("#", s.level))
		b.WriteString(" ")
		b.WriteString(s.heading)
		b.WriteString(":\n\n")
		for _, t := range s.targets {
			b.WriteString(renderTarget(t))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func renderTarget(t target) string {
	var b strings.Builder
	b.WriteString("`")
	b.WriteString(t.name)
	if t.vals != "" {
		b.WriteString("=")
		b.WriteString(t.vals)
	}
	b.WriteString("`")
	if len(t.chain) > 0 {
		b.WriteString(": `")
		b.WriteString(strings.Join(t.chain, " -> "))
		b.WriteString("`")
	}
	if t.what != "" {
		b.WriteString(" ")
		b.WriteString(t.what)
	}
	return b.String()
}

//[<] 🤖🤖🤖
