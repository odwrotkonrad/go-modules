package spec

// [>] 🤖🤖

import (
	"github.com/invopop/jsonschema"

	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

const schemaID = "https://gitlab.com/konradodwrot/go-modules/-/raw/main/che/assets/data/che.schema.json"

func Schema() *jsonschema.Schema {
	r := &jsonschema.Reflector{
		Anonymous:                  true,
		FieldNameTag:               "yaml",
		RequiredFromJSONSchemaTags: true,
	}
	defs := r.Reflect(ProfileRecipe{}).Definitions
	// [why] both spec.Options and render.Options reflect to an "Options" def:
	optDefs := r.Reflect(Options{}).Definitions
	defs["SpecOptions"] = optDefs["Options"]
	defs["Run"] = optDefs["Run"]
	defs["RenderTemplates"] = optDefs["RenderTemplates"]
	defs["Packages"] = optDefs["Packages"]
	defs["Otel"] = optDefs["Otel"]
	defs["DestSpec"] = DestSpec{}.JSONSchema()
	defs["templateNode"] = templateNode{}.JSONSchema()

	defs["ProfileRecipe"].Description = "one profile block: options self-describe eligibility, include.profiles compose refs in order (local scalars, sourced {source, options, env}), include adds, exclude filters last and wins"
	defs["includeSet"].Description = "additive payload: profile refs, makeLinks globs, makeCopies/renderTemplates/makeDirs perm-groups, runScripts globs"
	defs["excludeSet"].Description = "subtractive glob filter, applied last, wins over every include (rich entries too)"
	defs["SpecOptions"].Description = "reserved top-level options: block: spec-wide defaults (runIf gate, autoDiscover/logLevel/workingDirectory) + che knobs (validateSpec/dryRun/profiles/skipRemoteRefs/renderTemplates.skipSecrets); same shape as the user-config file"
	prop(defs["ProfileOptions"], "runIf").Description = "predicate expressions `<source>` or `<source> == <literal>`, sources builtin:*/env:*; empty: always"

	root := &jsonschema.Schema{
		Version:              jsonschema.Version,
		ID:                   schemaID,
		Title:                "che.yml",
		Description:          "che spec: reserved keys options/env/include, every other top-level key defines a profile block",
		Type:                 "object",
		AdditionalProperties: &jsonschema.Schema{Ref: "#/$defs/ProfileRecipe"},
		Definitions:          defs,
		Properties:           jsonschema.NewProperties(),
	}
	root.Properties.Set("options", &jsonschema.Schema{Ref: "#/$defs/SpecOptions"})
	root.Properties.Set("env", envSchema("environment exported around this spec's preparation and execution"))
	root.Properties.Set("include", topIncludeSchema())
	return root
}

func topIncludeSchema() *jsonschema.Schema {
	o := obj("other specs composed into this one", nil)
	o.Properties.Set("sources", &jsonschema.Schema{
		Description: "spec sources, each a <dir> (absolute, relative, ~/, $VAR) or @<giturl>",
		Type:        "array",
		Items:       &jsonschema.Schema{Type: "string"},
	})
	return o
}

func envSchema(desc string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Description:          desc,
		Type:                 "object",
		AdditionalProperties: &jsonschema.Schema{Type: "string"},
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

func scalarOr(scalarDesc string, o *jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		{Description: scalarDesc, Type: "string"},
		o,
	}}
}

const destPathDesc = "dest path: relative -> repo, ~/ or absolute -> host"

func destRuleSchema() *jsonschema.Schema {
	return &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		{
			Description: "sed-style rewrite s<delim><pattern><delim><replacement><delim>[g] (Go regexp pattern, literal replacement so $HOME survives; g: every match, absent: first only), applied to the workingDirectory-relative dest path before host mapping (:-delimited is the blessed form; e.g. s:^_home:$HOME: targets the invoking user's home)",
			Type:        "string",
			Pattern:     "^s\\W.+\\W(g)?$",
		},
		{
			Description: "prefix-swap sugar <prefix>/**: with source <srcPrefix>/** it desugars to s:^<srcPrefix>:<prefix>: (e.g. dest $HOME/** with source _home/** targets home)",
			Type:        "string",
			Pattern:     "/\\*\\*$",
		},
	}}
}

func (linkEntry) JSONSchema() *jsonschema.Schema {
	o := obj("source file with explicit dests, or source glob with a sed-style dest rewrite", []string{"source", "dest"})
	o.Properties.Set("source", &jsonschema.Schema{
		Description: "file or glob, workingDirectory-relative",
		Type:        "string",
	})
	o.Properties.Set("dest", &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		{
			Description: "dest paths: ~/ or absolute -> host",
			Type:        "array",
			Items:       &jsonschema.Schema{Ref: "#/$defs/DestSpec"},
		},
		destRuleSchema(),
	}})
	return scalarOr("glob over git-tracked files (brace-expanded), workingDirectory-relative, dest derived 1:1", o)
}

func (fileSpec) JSONSchema() *jsonschema.Schema {
	o := obj("one source fanned out to explicit dests", []string{"source"})
	o.Properties.Set("source", &jsonschema.Schema{
		Description: "source path, workingDirectory-relative",
		Type:        "string",
	})
	o.Properties.Set("dest", &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		{
			Description: "dest paths: relative -> repo, ~/ or absolute -> host; omitted -> derived from the workingDirectory-relative source path",
			Type:        "array",
			Items:       &jsonschema.Schema{Ref: "#/$defs/DestSpec"},
		},
		destRuleSchema(),
	}})
	return scalarOr("glob over git-tracked files (brace-expanded)", o)
}

func (templateNode) JSONSchema() *jsonschema.Schema {
	leaf := obj("one template source fanned out to dests", []string{"source"})
	leaf.Properties.Set("source", &jsonschema.Schema{
		Description: "source path (host sources workingDirectory-relative, repo-doc sources checkout-relative), or remote ref @<repo>//<path>[?ref=<ref>] (explicit dest required); joined onto every enclosing group's source prefix",
		Type:        "string",
	})
	leaf.Properties.Set("dest", &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		{
			Description: destPathDesc + "; omitted -> derived from the workingDirectory-relative source path",
			Type:        "string",
			Not:         destRuleSchema(),
		},
		{
			Description: "dest paths: relative -> repo, ~/ or absolute -> host; omitted -> derived from the workingDirectory-relative source path",
			Type:        "array",
			Items:       &jsonschema.Schema{Ref: "#/$defs/DestSpec"},
		},
		destRuleSchema(),
	}})
	leaf.Properties.Set("ctx", ctxSchema())
	leaf.Properties.Set("options", render.Options{}.JSONSchema())
	addPerms(leaf)

	group := obj("a source prefix plus shared perms, ctx and options cascading onto nested nodes (innermost wins)", []string{"renderTemplates"})
	group.Properties.Set("source", &jsonschema.Schema{
		Description: "source prefix joined onto every nested node's source; a remote prefix @<repo>[//<path>][?ref=<ref>] recombines so the ref stays last",
		Type:        "string",
	})
	group.Properties.Set("dest", &jsonschema.Schema{
		Description: "dest prefix joined onto every nested repo-relative dest (host dests: ~/, absolute, $VAR anchor themselves and are left alone)",
		Type:        "string",
	})
	group.Properties.Set("ctx", ctxSchema())
	group.Properties.Set("options", render.Options{}.JSONSchema())
	group.Properties.Set("renderTemplates", &jsonschema.Schema{
		Description: "nested nodes, each a leaf or a further group",
		Type:        "array",
		Items:       &jsonschema.Schema{Ref: "#/$defs/templateNode"},
	})
	addPerms(group)

	return &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		{Description: "glob over git-tracked files (brace-expanded)", Type: "string"},
		leaf,
		group,
	}}
}

func ctxSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Description:          "values exposed as the template's root context (.key), merged under each nested node's ctx",
		Type:                 "object",
		AdditionalProperties: &jsonschema.Schema{Type: "string"},
	}
}

func addPerms(o *jsonschema.Schema) {
	o.Properties.Set("owner", &jsonschema.Schema{Description: "dest owner user; empty: code default", Type: "string"})
	o.Properties.Set("ownerGroup", &jsonschema.Schema{Description: "dest owner group; empty: code default", Type: "string"})
	o.Properties.Set("chmod", &jsonschema.Schema{Description: "dest mode, octal string", Type: "string", Pattern: "^[0-7]{3,4}$"})
}

func (dirSpec) JSONSchema() *jsonschema.Schema {
	o := obj("", []string{"dest"})
	o.Properties.Set("dest", &jsonschema.Schema{
		Description: "dir paths (brace-expanded)",
		Type:        "array",
		Items:       &jsonschema.Schema{Ref: "#/$defs/DestSpec"},
	})
	return scalarOr("dir path (brace-expanded)", o)
}

func (DestSpec) JSONSchema() *jsonschema.Schema {
	o := obj("", []string{"path"})
	o.Properties.Set("path", &jsonschema.Schema{Description: destPathDesc, Type: "string"})
	o.Properties.Set("options", render.Options{}.JSONSchema())
	return scalarOr(destPathDesc, o)
}

func (ProfileSourceRecipe) JSONSchema() *jsonschema.Schema {
	o := obj("sourced profile ref: source is <source>/<spec-file>.yml::<profile>, options override its options, env overlays its run", []string{"source"})
	o.Properties.Set("source", &jsonschema.Schema{
		Description: "<source>/<spec-file>.yml::<profile>: source @<giturl> (remote) or <dir> (local); bare <profile> for the local spec",
		Type:        "string",
	})
	o.Properties.Set("options", &jsonschema.Schema{Ref: "#/$defs/ProfileOptions"})
	o.Properties.Set("env", envSchema("envs exported around everything done for the referenced profile (sourced entries only)"))
	o.Properties.Set("ctx", envSchema("ctx overlay merged over the referenced profile's renderTemplates ctx (entry wins), parameterizing a shared profile per consumer (sourced entries only)"))
	return scalarOr("local profile name, composed depth-first", o)
}

// [<] 🤖🤖
