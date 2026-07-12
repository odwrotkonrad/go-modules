package spec

// [>] 🤖🤖

import (
	"github.com/invopop/jsonschema"

	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

const schemaID = "https://gitlab.com/konradodwrot/go-modules/-/raw/main/che/assets/data/che.schema.json"

// Schema is the JSON Schema (draft 2020-12) for che.yml, reflected from this
// package's types (union scalar-or-object forms via JSONSchema() methods,
// mirroring each UnmarshalYAML).
func Schema() *jsonschema.Schema {
	r := &jsonschema.Reflector{
		Anonymous:                  true,
		FieldNameTag:               "yaml",
		RequiredFromJSONSchemaTags: true,
	}
	defs := r.Reflect(profileSpec{}).Definitions
	defs["DestSpec"] = DestSpec{}.JSONSchema()

	defs["profileSpec"].Description = "one profile block: options self-describe eligibility, mixinProfiles compose in order, plugins pull remote profiles, include adds, exclude filters last and wins"
	defs["includeSet"].Description = "additive payload: link globs, copy/renderTemplates/mkdirs perm-groups, script globs, service names"
	defs["excludeSet"].Description = "subtractive glob filter, applied last, wins over every include (rich entries too)"
	prop(defs["profileSpec"], "plugins").Description = "profiles loaded at their own checkout: `@<giturl>::<profile>` (remote) or `<dir>::<profile>` (local dir) string, or {ref, env}"
	prop(defs["ProfileOptions"], "execIf").Description = "predicate expressions `<source>` or `<source> == <literal>`, sources builtin:*/env:*; empty: always"

	return &jsonschema.Schema{
		Version:              jsonschema.Version,
		ID:                   schemaID,
		Title:                "che.yml",
		Description:          "che spec: every top-level key defines a profile block",
		Type:                 "object",
		AdditionalProperties: &jsonschema.Schema{Ref: "#/$defs/profileSpec"},
		Definitions:          defs,
	}
}

func prop(def *jsonschema.Schema, name string) *jsonschema.Schema {
	p, ok := def.Properties.Get(name)
	if !ok {
		panic("schema def missing property " + name)
	}
	return p
}

func obj(desc string, required []string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Description:          desc,
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
		Required:             required,
		Properties:           jsonschema.NewProperties(),
	}
}

// scalarOr wraps o into the union with its scalar-string shorthand form.
func scalarOr(scalarDesc, pattern string, o *jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		{Description: scalarDesc, Type: "string", Pattern: pattern},
		o,
	}}
}

const destPathDesc = "dest path: relative -> repo, ~/ or absolute -> host"

func (linkEntry) JSONSchema() *jsonschema.Schema {
	o := obj("source file or glob with a sed-style dest rewrite", []string{"source", "dest"})
	o.Properties.Set("source", &jsonschema.Schema{
		Description: "file or glob, repo-relative under root/",
		Type:        "string",
	})
	o.Properties.Set("dest", &jsonschema.Schema{
		Description: "sed-style rewrite s/<pattern>/<replacement>/[g] (Go regexp, $1 backrefs; g: every match, absent: first only), applied to the repo-relative dest path before host mapping",
		Type:        "string",
		Pattern:     "^s/.+/.*/g?$",
	})
	return scalarOr("glob over git-tracked files under root/ (brace-expanded), dest derived 1:1", "", o)
}

func (fileSpec) JSONSchema() *jsonschema.Schema {
	o := obj("one source fanned out to explicit dests", []string{"source"})
	o.Properties.Set("source", &jsonschema.Schema{
		Description: "repo-relative source path, or remote ref @<repo>//<path>[?ref=<ref>] (renderTemplates only, explicit dest required)",
		Type:        "string",
	})
	o.Properties.Set("dest", &jsonschema.Schema{
		Description: "dest paths: relative -> repo, ~/ or absolute -> host; omitted -> derived from the root/ source path",
		Type:        "array",
		Items:       &jsonschema.Schema{Ref: "#/$defs/DestSpec"},
	})
	o.Properties.Set("ctx", &jsonschema.Schema{
		Description:          "renderTemplates only: values exposed as the template's root context (.key)",
		Type:                 "object",
		AdditionalProperties: &jsonschema.Schema{Type: "string"},
	})
	return scalarOr("glob over git-tracked files (brace-expanded)", "", o)
}

func (dirSpec) JSONSchema() *jsonschema.Schema {
	o := obj("", []string{"dest"})
	o.Properties.Set("dest", &jsonschema.Schema{
		Description: "dir paths (brace-expanded)",
		Type:        "array",
		Items:       &jsonschema.Schema{Ref: "#/$defs/DestSpec"},
	})
	return scalarOr("dir path (brace-expanded)", "", o)
}

func (DestSpec) JSONSchema() *jsonschema.Schema {
	o := obj("", []string{"path"})
	o.Properties.Set("path", &jsonschema.Schema{Description: destPathDesc, Type: "string"})
	o.Properties.Set("options", render.Options{}.JSONSchema())
	return scalarOr(destPathDesc, "", o)
}

func (pluginEntry) JSONSchema() *jsonschema.Schema {
	const refDesc = "`@<giturl>::<profile>` (remote) or `<dir>::<profile>` (local dir) plugin ref"
	o := obj("", []string{"ref"})
	o.Properties.Set("ref", &jsonschema.Schema{
		Description: refDesc,
		Type:        "string",
		Pattern:     "^.+::.+$",
	})
	o.Properties.Set("env", &jsonschema.Schema{
		Description:          "envs exported around everything done for the plugin's unit",
		Type:                 "object",
		AdditionalProperties: &jsonschema.Schema{Type: "string"},
	})
	return scalarOr(refDesc, "^.+::.+$", o)
}

// [<] 🤖🤖
