// Package e2e drives the built che binary through YAML-declared command flows
// (e2e.spec.yml) over the local/remote fixtures in a hermetic temp HOME.
package e2e

// [>] 🤖🤖

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// Domain model (the e2e spec schema):
//
//	specFile
//	  defs          anchor scratch space shared by all cases, ignored by the runner
//	  testCases     named flows; each carries context.env (extra env for every
//	                che invocation, e.g. CHE_LOG_LEVEL) and its ordered steps,
//	                fail-fast, sharing one fresh workdir + HOME per case
//	step: exactly one action (command | write | remove | gitRestore | extract).
//	expected: stdOut substring matchers (literal with {{/regex/}} holes),
//	stdOutFull (whole ANSI-stripped output, anchored), exitCode, files
//	(exists: true entries with type/symlinkTo/content/countGlob).
//	notExpected: stdOut matchers plus bare {path, exists: true} entries
//	asserting absence. A files value is one mapping or a sequence; nested
//	sequences (def anchors) flatten, so lists assemble from defs without
//	repeating entries. Paths and matchers expand ${WORK} ${HOME} ${LOCAL}
//	${REMOTE} ${XDG_STATE_HOME} ${XDG_CACHE_HOME}.

type specFile struct {
	Defs      any        `yaml:"defs"`
	TestCases []testCase `yaml:"testCases"`
}

type testCase struct {
	Name    string      `yaml:"name"`
	Context specContext `yaml:"context"`
	Steps   []step      `yaml:"steps"`
}

type specContext struct {
	Env map[string]string `yaml:"env"`
}

type step struct {
	Name        string       `yaml:"name"`
	Command     string       `yaml:"command"`
	Write       *writeSpec   `yaml:"write"`
	Remove      *removeSpec  `yaml:"remove"`
	GitRestore  *gitSpec     `yaml:"gitRestore"`
	Extract     *extractSpec `yaml:"extract"`
	Expected    want         `yaml:"expected"`
	NotExpected notWant      `yaml:"notExpected"`
}

// countActions counts the step's declared actions (exactly one is valid).
func (s step) countActions() int {
	n := 0
	for _, set := range []bool{s.Command != "", s.Write != nil, s.Remove != nil, s.GitRestore != nil, s.Extract != nil} {
		if set {
			n++
		}
	}
	return n
}

type writeSpec struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
}

type removeSpec struct {
	Paths []string `yaml:"paths"`
}

type gitSpec struct {
	Repo string `yaml:"repo"`
	Path string `yaml:"path"`
}

type extractSpec struct {
	ArchiveGlob string `yaml:"archiveGlob"`
	Dest        string `yaml:"dest"`
}

type want struct {
	StdOut     testyml.Matchers  `yaml:"stdOut"`
	StdOutFull string            `yaml:"stdOutFull"`
	ExitCode   int               `yaml:"exitCode"`
	Files      files[fileAssert] `yaml:"files"`
}

type notWant struct {
	StdOut testyml.Matchers    `yaml:"stdOut"`
	Files  files[absentAssert] `yaml:"files"`
}

type fileAssert struct {
	Path      string `yaml:"path"`
	Exists    bool   `yaml:"exists"`
	Type      string `yaml:"type"`
	SymlinkTo string `yaml:"symlinkTo"`
	Content   string `yaml:"content"`
	CountGlob string `yaml:"countGlob"`
	Count     int    `yaml:"count"`
}

type absentAssert struct {
	Path   string `yaml:"path"`
	Exists bool   `yaml:"exists"`
}

// files accepts a single mapping or a sequence, flattening nested sequences
// (def anchors spliced into a list), strict-decoding each entry.
type files[T any] []T

func (f *files[T]) UnmarshalYAML(node *yaml.Node) error {
	node = derefAlias(node)
	if node.Kind == yaml.MappingNode {
		node = &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{node}}
	}
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("files: want a mapping or sequence, got kind %v", node.Kind)
	}
	out := files[T]{}
	for _, item := range node.Content {
		item = derefAlias(item)
		if item.Kind == yaml.SequenceNode {
			var nested files[T]
			if err := item.Decode(&nested); err != nil {
				return err
			}
			out = append(out, nested...)
			continue
		}
		var one T
		if err := testyml.StrictDecodeNode(item, &one); err != nil {
			return err
		}
		out = append(out, one)
	}
	*f = out
	return nil
}

func derefAlias(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.AliasNode {
		return n.Alias
	}
	return n
}

// [<] 🤖🤖
