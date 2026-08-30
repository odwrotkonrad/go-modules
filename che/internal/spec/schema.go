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
const (
	schemaID          = "https://gitlab.com/api/v4/projects/konradodwrot%2Fgo-modules/packages/generic/che-schema/latest/che.schema.json"
	variablesSchemaID = "https://gitlab.com/api/v4/projects/konradodwrot%2Fgo-modules/packages/generic/che-variables-schema/latest/che-variables.schema.json"
)

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

	defs["ProfileRecipe"].Description = "one profile block: options self-describe eligibility, variablesDefinitions declare what it reads, include.profiles compose refs in order (local scalars, sourced {url, ref, specDirPath, profileName, options, env, variables}), include adds, exclude filters last and wins"
	defs["includeSet"].Description = "additive payload: profile refs, makeLinks globs, makeCopies/renderTemplates trees, makeDirs perm-groups, runScripts globs"
	defs["excludeSet"].Description = "subtractive glob filter, applied last, wins over every include (rich entries too)"
	defs["SpecOptions"].Description = "reserved top-level options: block: spec-wide defaults (runIf gate, autoDiscover/logLevel/workingDirectory) + che knobs (validateSpec/dryRun/profiles/skipRemoteRefs/renderTemplates.skipVariables); same shape as the user-config file"
	prop(defs["ProfileOptions"], "runIf").Description = "predicate expressions `<source>` or `<source> == <literal>`, sources builtin:*/env:*/cmd:<argv> (exit 0 passes, run on every evaluation, argv split on whitespace, no shell); empty: always"
	varDefs := r.Reflect(SpecVarDefs{}).Definitions
	defs["VarDef"] = varDefs["VarDef"]
	defs["VarDefs"] = varDefs["VarDefs"]
	defs["SpecVarDefs"] = varDefs["SpecVarDefs"]
	defs["VarDefaults"] = varDefs["VarDefaults"]
	defs["VarDefs"].AdditionalProperties = &jsonschema.Schema{OneOf: []*jsonschema.Schema{
		{Type: "null", Description: "a name needing nothing of its own: scope and required from variablesDefinitions.all"},
		{Ref: "#/$defs/VarDef"},
	}}
	defs["VarDef"].Description = "one variable declaration: never a value, values live in cheVariables.defaults.yml, cheVariables.yml, cheVariables.local.yml or an explicit pass"

	root := &jsonschema.Schema{
		Version:              jsonschema.Version,
		ID:                   schemaID,
		Title:                "che.yml",
		Description:          "che spec: options, env, variablesDefinitions, specsInclude (embedded specs), profilesDefinitions (profiles keyed by name); no other top-level key",
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
		Definitions:          defs,
		Properties:           jsonschema.NewProperties(),
	}
	root.Properties.Set("options", &jsonschema.Schema{Ref: "#/$defs/SpecOptions"})
	root.Properties.Set("env", envSchema("environment exported around this spec's preparation and execution, to template renders and scripts; values may interpolate ${{ env.X }} and ${{ var.X }}; env never creates a variable"))
	root.Properties.Set("variablesDefinitions", &jsonschema.Schema{Ref: "#/$defs/SpecVarDefs"})
	root.Properties.Set("specsInclude", &jsonschema.Schema{
		Description: "embedded specs: a local <dir> scalar (absolute, relative, ~/, $VAR), or {url, ref, specDirPath, env, variables, optional}; a dir offers its che.export.yml first, then a plain che.yml",
		Type:        "array",
		Items:       scalarOr("local spec dir", specsIncludeEntrySchema()),
	})
	root.Properties.Set("profilesDefinitions", &jsonschema.Schema{
		Description:          "profiles keyed by name",
		Type:                 "object",
		AdditionalProperties: &jsonschema.Schema{Ref: "#/$defs/ProfileRecipe"},
	})
	return root
}

// VariablesFileSchema describes cheVariables.yml, cheVariables.defaults.yml and cheVariables.local.yml.
func VariablesFileSchema() *jsonschema.Schema {
	entry := obj("value with an explicit scope", []string{"value"})
	entry.Properties.Set("value", &jsonschema.Schema{Description: "the literal; ${{ env.X }} expands from the process env and che.env", Type: "string"})
	entry.Properties.Set("scope", &jsonschema.Schema{
		Description: "how far the value propagates, over the definition's scope: invokingSpec | invokingSpecDefinedProfiles | recursiveSpecs | recursiveSpecsAndProfiles",
		Type:        "string",
		Enum:        []any{"invokingSpec", "invokingSpecDefinedProfiles", "recursiveSpecs", "recursiveSpecsAndProfiles"},
	})
	return &jsonschema.Schema{
		Version:              jsonschema.Version,
		ID:                   variablesSchemaID,
		Title:                "cheVariables.yml",
		Description:          "variable values for one spec, keyed by name: a scalar, or {value, scope}; every name must be declared by the spec's variablesDefinitions",
		Type:                 "object",
		PropertyNames:        &jsonschema.Schema{Pattern: "^[A-Za-z_][A-Za-z0-9_]*$"},
		AdditionalProperties: scalarOr("the literal; ${{ env.X }} expands from the process env and che.env", entry),
	}
}

func specsIncludeEntrySchema() *jsonschema.Schema {
	entry := obj("embedded spec source with overlays applied to every profile of the embedded spec", nil)
	entry.AnyOf = []*jsonschema.Schema{{Required: []string{"specDirPath"}}, {Required: []string{"source"}}}
	addSpecSourceKeys(entry)
	entry.Properties.Set("env", envSchema("env overlaid on the embedded spec's load and run"))
	entry.Properties.Set("variables", envSchema("explicit pass: values for variables the embedded spec declares, over its own files"))
	entry.Properties.Set("optional", &jsonschema.Schema{
		Description: "local dir sources only: an absent dir is skipped with a warning instead of failing the load (a dir another profile renders first)",
		Type:        "boolean",
	})
	return entry
}

func addSpecSourceKeys(o *jsonschema.Schema) {
	o.Properties.Set("url", &jsonschema.Schema{
		Description: "the git repo holding the spec, cloned/pulled into a managed cache checkout: host/path, scheme optional (gitlab.com/g/r, https://gitlab.com/g/r.git, git@gitlab.com:g/r.git); never a local path, a local spec is specDirPath alone",
		Type:        "string",
	})
	o.Properties.Set("ref", &jsonschema.Schema{Description: "git ref pinning a remote url: tag, branch or commit; never on a local dir", Type: "string"})
	o.Properties.Set("specDirPath", &jsonschema.Schema{
		Description: "required: with url the dir under the repo holding the spec (. for the repo root); without url a local dir (./sub, /abs, ~/, $VAR, relative to this spec's checkout, . this repo's own exported spec); the dir offers its che.export.yml, then .che/che.export.yml, then a plain che.yml",
		Type:        "string",
	})
	o.Properties.Set("source", &jsonschema.Schema{Description: "deprecated: the old url key, git::<giturl>[@<ref>] or <dir>, decoded with a warning", Type: "string", Deprecated: true})
	o.Properties.Set("spec", &jsonschema.Schema{Description: "deprecated: the old specDirPath key, decoded with a warning", Type: "string", Deprecated: true})
}

func remoteSourceSchema(desc string) *jsonschema.Schema {
	o := obj("remote source object, rendered as git::<url>[@<ref>][//<filepath>]", []string{"url"})
	o.Properties.Set("url", &jsonschema.Schema{Description: "git url, host/path with an optional scheme (gitlab.com/g/r, https://gitlab.com/g/r.git)", Type: "string"})
	o.Properties.Set("ref", &jsonschema.Schema{Description: "git ref: tag, branch or commit", Type: "string"})
	o.Properties.Set("filepath", &jsonschema.Schema{Description: "path inside the repo; omitted on a group prefix, the nested string sources join onto it", Type: "string"})
	return scalarOr(desc, o)
}

func sourceListSchema(desc string) *jsonschema.Schema {
	return &jsonschema.Schema{Description: desc, Type: "array", MinItems: ptr(uint64(1)), Items: &jsonschema.Schema{Type: "string"}}
}

func ptr[T any](v T) *T { return &v }

func sourceListOnly() *jsonschema.Schema {
	props := jsonschema.NewProperties()
	props.Set("source", &jsonschema.Schema{Type: "array"})
	return &jsonschema.Schema{Required: []string{"source"}, Properties: props, Not: &jsonschema.Schema{Required: []string{"dest"}}}
}

func sourceListDestFree() *jsonschema.Schema {
	props := jsonschema.NewProperties()
	props.Set("source", &jsonschema.Schema{Type: "array"})
	return &jsonschema.Schema{Not: &jsonschema.Schema{Required: []string{"source", "dest"}, Properties: props}}
}

func leafSourceSchema(desc, listDesc string) *jsonschema.Schema {
	base := remoteSourceSchema(desc)
	base.OneOf = append(base.OneOf, sourceListSchema(listDesc))
	return base
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
	o := obj("source file with explicit dests, source glob with a sed-style dest rewrite, or a list of globs", nil)
	o.AnyOf = []*jsonschema.Schema{{Required: []string{"source", "dest"}}, sourceListOnly()}
	o.Properties.Set("source", leafSourceSchema("file or glob, workingDirectory-relative (${{ repoRoot }}/<path> anchors at the git root of the spec's repo)", "globs over git-tracked files, workingDirectory-relative, dest derived 1:1, no dest allowed"))
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
	leaf.Properties.Set("source", leafSourceSchema("source path, any file, workingDirectory-relative (${{ repoRoot }}/<path> anchors at the git root of the spec's repo), or remote ref git::<repo>[@<ref>]//<path> (explicit dest required); joined onto every enclosing group's source prefix", "source paths, one leaf each, no dest allowed: each lands at its path under the enclosing group's dest prefix (top level: derived host dest)"))
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
	leaf.AllOf = []*jsonschema.Schema{sourceListDestFree()}
	addPerms(leaf)

	group := obj("a source and/or dest prefix plus shared perms cascading onto nested nodes (innermost wins); at least one prefix required", []string{"<<<"})
	group.AnyOf = []*jsonschema.Schema{{Required: []string{"source"}}, {Required: []string{"dest"}}}
	group.Properties.Set("source", remoteSourceSchema("source prefix joined onto every nested node's source; a remote prefix git::<repo>[@<ref>][//<path>] concatenates with each leaf path"))
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
	leaf.Properties.Set("source", leafSourceSchema("source path (host sources workingDirectory-relative, repo-doc sources checkout-relative, ${{ repoRoot }}/<path> anchors at the git root of the spec's repo), or remote ref git::<repo>[@<ref>]//<path> (explicit dest required); joined onto every enclosing group's source prefix", "source paths, one leaf each, no dest allowed: each lands at its .tpl-stripped path under the enclosing group's dest prefix (top level: derived host dest)"))
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
	leaf.Properties.Set("options", render.Options{}.JSONSchema())
	leaf.AllOf = []*jsonschema.Schema{sourceListDestFree()}
	addPerms(leaf)

	group := obj("a source and/or dest prefix plus shared perms, variables and options cascading onto nested nodes (innermost wins); at least one prefix required", []string{"<<<"})
	group.AnyOf = []*jsonschema.Schema{{Required: []string{"source"}}, {Required: []string{"dest"}}}
	group.Properties.Set("source", remoteSourceSchema("source prefix joined onto every nested node's source; a remote prefix git::<repo>[@<ref>][//<path>] concatenates with each leaf path"))
	group.Properties.Set("dest", &jsonschema.Schema{
		Description: "dest prefix joined onto every nested repo-relative dest (host dests: ~/, absolute, $VAR anchor themselves and are left alone)",
		Type:        "string",
	})
	group.Properties.Set("variables", ctxSchema())
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
		Description:          "values exposed as the template's .var.<key>, over the profile's variables and under each nested node's variables",
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
	o := obj("sourced profile ref: a profile of another spec, options override its options, env overlays its run, variables override its variables", nil)
	o.AnyOf = []*jsonschema.Schema{{Required: []string{"specDirPath"}}, {Required: []string{"source"}}}
	addSpecSourceKeys(o)
	o.Properties.Set("profileName", &jsonschema.Schema{
		Description: "a top-level profile of that spec (nested profiles are never searched: reference them by their own url, ref, specDirPath, profileName); exclusive with profileNames",
		Type:        "string",
	})
	item := obj("one profile of the shared source, its own spec dir when it differs from the entry's", []string{"profileName"})
	item.Properties.Set("specDirPath", &jsonschema.Schema{Description: "spec dir under url for this profile; default the entry's specDirPath", Type: "string"})
	item.Properties.Set("profileName", &jsonschema.Schema{Description: "profile name", Type: "string"})
	item.Properties.Set("spec", &jsonschema.Schema{Description: "deprecated: the old specDirPath key", Type: "string", Deprecated: true})
	item.Properties.Set("profile", &jsonschema.Schema{Description: "deprecated: the old profileName key", Type: "string", Deprecated: true})
	o.Properties.Set("profileNames", &jsonschema.Schema{
		Description: "several profiles of the same url and ref, written once: names, or {specDirPath, profileName} when a profile sits under another spec dir; options, env and variables apply to each; exclusive with profileName",
		Type:        "array",
		Items:       scalarOr("profile name", item),
	})
	o.Properties.Set("profile", &jsonschema.Schema{Description: "deprecated: the old profileName key, decoded with a warning", Type: "string", Deprecated: true})
	o.Properties.Set("profiles", &jsonschema.Schema{Description: "deprecated: the old profileNames key, decoded with a warning", Type: "array", Items: scalarOr("profile name", item), Deprecated: true})
	o.Properties.Set("options", &jsonschema.Schema{Ref: "#/$defs/ProfileOptions"})
	o.Properties.Set("env", envSchema("envs exported around everything done for the referenced profile (sourced entries only)"))
	o.Properties.Set("variables", envSchema("explicit pass: values for variables the referenced spec declares, over its own cheVariables files, parameterizing a shared profile per consumer (sourced entries only)"))
	return scalarOr("local profile name, composed depth-first", o)
}

// [<] 🤖🤖
