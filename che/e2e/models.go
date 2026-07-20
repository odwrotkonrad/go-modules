// Package e2e drives the built che binary through YAML-declared command flows
// (e2e.spec.yml, dryrun.e2e.spec.yml) over the local/remote fixtures in a
// hermetic temp HOME.
package e2e

// [>] 🤖🤖

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

// Domain model (the e2e spec schema):
//
//	specFile
//	  context.env   extra env for every che invocation (e.g. CHE_DEBUG)
//	  defs          anchor scratch space, ignored by the runner
//	  steps         ordered, fail-fast, sharing one workdir + HOME
//	step: exactly one action (command | write | remove | gitRestore | extract),
//	expected/notExpected allowed on any step. stdOut matchers are substring
//	(literal with {{/regex/}} holes); stdOutFull (expected only) asserts the
//	whole ANSI-stripped output. files entries are positive-only
//	(exists: true); absence is asserted by listing the entry under
//	notExpected.files. files takes one mapping or a sequence; an item may
//	itself be a sequence (a def anchor) and nested sequences flatten, so
//	lists assemble from defs without repeating entries. Paths and matchers
//	expand ${WORK} ${HOME} ${LOCAL} ${REMOTE} ${XDG_STATE_HOME}
//	${XDG_CACHE_HOME}.

type specFile struct {
	Context specContext `yaml:"context"`
	Defs    yaml.Node   `yaml:"defs"`
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
	NotExpected want         `yaml:"notExpected"`
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
	StdOut     testyml.Matchers `yaml:"stdOut"`
	StdOutFull string           `yaml:"stdOutFull"`
	ExitCode   int              `yaml:"exitCode"`
	Files      fileAsserts      `yaml:"files"`
}

// fileAsserts accepts a single mapping or a sequence, flattening nested
// sequences (def anchors spliced into a files list), strict-decoding each
// entry.
type fileAsserts []fileAssert

func (f *fileAsserts) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	switch node.Kind {
	case yaml.MappingNode:
		one, err := decodeFileAssert(node)
		if err != nil {
			return err
		}
		*f = fileAsserts{one}
		return nil
	case yaml.SequenceNode:
	default:
		return fmt.Errorf("files: want a mapping or sequence, got kind %v", node.Kind)
	}
	out := fileAsserts{}
	for _, item := range node.Content {
		if item.Kind == yaml.AliasNode {
			item = item.Alias
		}
		if item.Kind == yaml.SequenceNode {
			var nested fileAsserts
			if err := item.Decode(&nested); err != nil {
				return err
			}
			out = append(out, nested...)
			continue
		}
		one, err := decodeFileAssert(item)
		if err != nil {
			return err
		}
		out = append(out, one)
	}
	*f = out
	return nil
}

func decodeFileAssert(node *yaml.Node) (fileAssert, error) {
	var one fileAssert
	enc, err := yaml.Marshal(node)
	if err != nil {
		return one, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(enc))
	dec.KnownFields(true)
	err = dec.Decode(&one)
	return one, err
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

// [<] 🤖🤖
