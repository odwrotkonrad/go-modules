package lib

// [>] 🤖🤖🤖

type target struct {
	name string
	what string
	vals string
	deps []string
}

type section struct {
	heading string
	level   int
	targets []target
}

type frame struct {
	heading string
	depth   int
	level   int
	kept    bool
	targets []target
}

type pendingComment struct{ what, vals string }

// [<] 🤖🤖🤖
