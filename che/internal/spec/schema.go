package spec

// [>] 🤖🤖

import (
	"github.com/invopop/jsonschema"

	"gitlab.com/konradodwrot/go-modules/che/render/render"
)

// [why] the artifact's own package path, never raw/main: this document is published as the
// che-schema artifact, and its $id should name where that artifact lives.
// [why] the version-less latest alias, not a pinned version: the version is decided after
// generation by comparing these bytes with the last published ones, so stamping it in here would
// make every generation differ from the last and mint a version on every release.
const schemaID = "https://gitlab.com/api/v4/projects/konradodwrot%2Fgo-modules/packages/generic/che-schema/latest/che.schema.json"

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
	defs["Backup"] = optDefs["Backup"]
	defs["BackupAutoCreate"] = optDefs["BackupAutoCreate"]
	defs["RenderTemplates"] = optDefs["RenderTemplates"]
	defs["Packages"] = optDefs["Packages"]
	defs["Otel"] = optDefs["Otel"]
	defs["DestSpec"] = DestSpec{}.JSONSchema()
	defs["templateNode"] = templateNode{}.JSONSchema()
	defs["copyNode"] = copyNode{}.JSONSchema()

	defs["ProfileRecipe"].Description = "one profile block: options self-describe eligibility, include.profiles compose refs in order (local scalars, sourced {source, spec, profile, options, env, variables}), include adds, exclude filters last and wins"
	defs["includeSet"].Description = "additive payload: profile refs, makeLinks globs, makeCopies/renderTemplates trees, makeDirs perm-groups, runScripts globs"
	defs["excludeSet"].Description = "subtractive glob filter, applied last, wins over every include (rich entries too)"
	defs["SpecOptions"].Description = "reserved top-level options: block: spec-wide defaults (runIf gate, autoDiscover/logLevel/workingDirectory) + che knobs (validateSpec/dryRun/profiles/skipRemoteRefs/renderTemplates.skipVariables); same shape as the user-config file"
	prop(defs["ProfileOptions"], "runIf").Description = "predicate expressions `<source>` or `<source> == <literal>`, sources builtin:*/env:*/cmd:<argv> (exit 0 passes, run on every evaluation, argv split on whitespace, no shell); empty: always"

	root := &jsonschema.Schema{
		Version:              jsonschema.Version,
		ID:                   schemaID,
		Title:                "che.yml",
		Description:          "che spec: reserved keys options/env/variables/include, every other top-level key defines a profile block",
		Type:                 "object",
		AdditionalProperties: &jsonschema.Schema{Ref: "#/$defs/ProfileRecipe"},
		Definitions:          defs,
		Properties:           jsonschema.NewProperties(),
	}
	root.Properties.Set("options", &jsonschema.Schema{Ref: "#/$defs/SpecOptions"})
	root.Properties.Set("env", envSchema("environment exported around this spec's preparation and execution; values may interpolate ${{ env.X }} and ${{ var.X }}"))
	root.Properties.Set("variables", envSchema("spec variables, ${{ var.X }} in this spec and .var.X in its templates, over the repo's che.variables.yml and under a ref's variables:; values may interpolate ${{ env.X }}"))
	root.Properties.Set("include", topIncludeSchema())
	return root
}

func topIncludeSchema() *jsonschema.Schema {
	o := obj("other specs composed into this one", nil)
	entry := obj("spec source with overlays applied to every profile of the included spec", []string{"source"})
	entry.Properties.Set("source", &jsonschema.Schema{Description: "<dir> (absolute, relative, ~/, $VAR) or git::<giturl>[@<git-ref>] (@<ref> pins a tag or branch); never a file, never //<path>", Type: "string"})
	entry.Properties.Set("spec", &jsonschema.Schema{Description: "where the spec sits under source: a dir (che.export.yml first) or, not recommended, a spec file path; default the source root", Type: "string"})
	entry.Properties.Set("env", envSchema("env overlaid on the included spec's load and run"))
	entry.Properties.Set("variables", envSchema("variables overriding the included spec's own"))
	o.Properties.Set("sources", &jsonschema.Schema{
		Description: "spec sources, each a <dir> (absolute, relative, ~/, $VAR) or git::<giturl>[@<git-ref>] (@<ref> pins a tag or branch), scalar or {source, spec, env, variables}; a dir offers its che.export.yml first, then a plain che.yml",
		Type:        "array",
		Items:       scalarOr("spec source", entry),
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

func (copyNode) JSONSchema() *jsonschema.Schema {
	leaf := obj("one copy source fanned out to dests, bytes copied verbatim", []string{"source"})
	leaf.Properties.Set("source", &jsonschema.Schema{
		Description: "source path, any file, workingDirectory-relative, or remote ref git::<repo>[@<ref>]//<path> (explicit dest required); joined onto every enclosing group's source prefix",
		Type:        "string",
	})
	leaf.Properties.Set("dest", &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		{
			Description: "dest path: relative -> the invoking spec's git root, ~/ or absolute -> host; omitted -> derived from the workingDirectory-relative source path, .ontoHost.cp stripped, host",
			Type:        "string",
			Not:         destRuleSchema(),
		},
		{
			Description: "dest paths: relative -> the invoking spec's git root, ~/ or absolute -> host; omitted -> derived from the workingDirectory-relative source path, .ontoHost.cp stripped, host",
			Type:        "array",
			Items:       &jsonschema.Schema{Ref: "#/$defs/DestSpec"},
		},
		destRuleSchema(),
	}})
	addPerms(leaf)

	group := obj("a source and/or dest prefix plus shared perms cascading onto nested nodes (innermost wins); at least one prefix required", []string{"<<<"})
	group.AnyOf = []*jsonschema.Schema{{Required: []string{"source"}}, {Required: []string{"dest"}}}
	group.Properties.Set("source", &jsonschema.Schema{
		Description: "source prefix joined onto every nested node's source; a remote prefix git::<repo>[@<ref>][//<path>] concatenates with each leaf path",
		Type:        "string",
	})
	group.Properties.Set("dest", &jsonschema.Schema{
		Description: "dest prefix joined onto every nested relative dest (~/, absolute and $VAR dests anchor themselves and are left alone)",
		Type:        "string",
	})
	group.Properties.Set("<<<", &jsonschema.Schema{
		Description: "nested nodes, each a leaf or a further group",
		Type:        "array",
		Items:       &jsonschema.Schema{Ref: "#/$defs/copyNode"},
	})
	addPerms(group)

	return &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		{Description: "glob over git-tracked *.ontoHost.cp files (brace-expanded)", Type: "string"},
		leaf,
		group,
	}}
}

func (templateNode) JSONSchema() *jsonschema.Schema {
	leaf := obj("one template source fanned out to dests", []string{"source"})
	leaf.Properties.Set("source", &jsonschema.Schema{
		Description: "source path (host sources workingDirectory-relative, repo-doc sources checkout-relative), or remote ref git::<repo>[@<ref>]//<path> (explicit dest required); joined onto every enclosing group's source prefix",
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
	leaf.Properties.Set("variables", ctxSchema())
	leaf.Properties.Set("ctx", deprecatedCtxSchema())
	leaf.Properties.Set("options", render.Options{}.JSONSchema())
	addPerms(leaf)

	group := obj("a source and/or dest prefix plus shared perms, variables and options cascading onto nested nodes (innermost wins); at least one prefix required", []string{"<<<"})
	group.AnyOf = []*jsonschema.Schema{{Required: []string{"source"}}, {Required: []string{"dest"}}}
	group.Properties.Set("source", &jsonschema.Schema{
		Description: "source prefix joined onto every nested node's source; a remote prefix git::<repo>[@<ref>][//<path>] concatenates with each leaf path",
		Type:        "string",
	})
	group.Properties.Set("dest", &jsonschema.Schema{
		Description: "dest prefix joined onto every nested repo-relative dest (host dests: ~/, absolute, $VAR anchor themselves and are left alone)",
		Type:        "string",
	})
	group.Properties.Set("variables", ctxSchema())
	group.Properties.Set("ctx", deprecatedCtxSchema())
	group.Properties.Set("options", render.Options{}.JSONSchema())
	group.Properties.Set("<<<", &jsonschema.Schema{
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
		Description:          "values exposed as the template's .var.<key>, over the spec's variables and under each nested node's variables",
		Type:                 "object",
		AdditionalProperties: &jsonschema.Schema{Type: "string"},
	}
}

func deprecatedCtxSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Description:          "deprecated: the old name of variables, decoded as variables with a warning",
		Type:                 "object",
		AdditionalProperties: &jsonschema.Schema{Type: "string"},
		Deprecated:           true,
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
	o := obj("sourced profile ref: a profile of another spec, options override its options, env overlays its run, variables override its variables", []string{"source"})
	o.Properties.Set("source", &jsonschema.Schema{
		Description: "the repo or dir holding the spec: git::<giturl>[@<ref>] (remote, @<ref> pins a tag or branch) or <dir> (local: $VAR and ~ expand, relative to this spec's checkout, . for this repo's own exported spec); never a file, never //<path>",
		Type:        "string",
	})
	o.Properties.Set("spec", &jsonschema.Schema{
		Description: "where the spec sits under source: a dir (its che.export.yml, then .che/che.export.yml, then a plain che.yml) or, not recommended, a spec file path used as is; default the source root",
		Type:        "string",
	})
	o.Properties.Set("profile", &jsonschema.Schema{
		Description: "a top-level profile of that spec, or a path a::b::c walking include.profiles from top-level a down to the nested profile c (nested profiles are never searched by default); exclusive with profiles",
		Type:        "string",
	})
	item := obj("one profile of the shared source, its own spec when it differs from the entry's", []string{"profile"})
	item.Properties.Set("spec", &jsonschema.Schema{Description: "dir or spec file under source for this profile; default the entry's spec", Type: "string"})
	item.Properties.Set("profile", &jsonschema.Schema{Description: "profile name or a::b path", Type: "string"})
	o.Properties.Set("profiles", &jsonschema.Schema{
		Description: "several profiles of the same source and ref, written once: names or a::b paths, or {spec, profile} when a profile sits under another spec; options, env and variables apply to each; exclusive with profile",
		Type:        "array",
		Items:       scalarOr("profile name or a::b path", item),
	})
	o.Properties.Set("options", &jsonschema.Schema{Ref: "#/$defs/ProfileOptions"})
	o.Properties.Set("env", envSchema("envs exported around everything done for the referenced profile (sourced entries only)"))
	o.Properties.Set("variables", envSchema("variables overriding the referenced spec's own (che.variables.yml and its variables: block), parameterizing a shared profile per consumer (sourced entries only)"))
	o.Properties.Set("ctx", deprecatedCtxSchema())
	return scalarOr("local profile name, composed depth-first", o)
}

// [<] 🤖🤖
