package fsutil

// TODO: consider redesigning the data model for these types now that they're consolidated in one place.

// [>] 🤖🤖

// OSReader reads the live filesystem.
type OSReader struct{}

// FS runs mutating fs ops, escalating priv per-dest (sudo iff dest outside
// invoking user's Home). Pure execution: no logging, no dry-run gate.
type FS struct {
	Home string
}

// [<] 🤖🤖
