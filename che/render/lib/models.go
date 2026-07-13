package lib

// TODO: consider redesigning the data model for these types now that they're consolidated in one place.

// [>] 🤖🤖🤖

type target struct {
	name  string
	what  string
	vals  string // parameter accepted-values hint, rendered as name=vals
	chain []string
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

type pendingCmt struct{ what, vals string }

// [<] 🤖🤖🤖
