package main

// [>] 🤖🤖

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/konradodwrot/go-modules/che/internal/cli"
)

// docgen renders che's reference docs from the Go source: the che.yml JSON
// Schema (assets/data/che.schema.json) and the CLI reference (docs/cli.md).
// Run from the repo root: `go run ./internal/docgen`.

const (
	schemaPath   = "assets/data/che.schema.json"
	cliDocPath   = "docs/cli.md"
	cliUsagePath = "assets/data/cli-usage.md"
	schemaID     = "https://gitlab.com/konradodwrot/go-modules/-/raw/main/che/assets/data/che.schema.json"
)

func main() {
	must(os.MkdirAll("docs", 0o755))
	root := cli.Attach()
	must(os.WriteFile(schemaPath, schemaJSON(), 0o644))
	must(os.WriteFile(cliDocPath, []byte(cliDoc(root)), 0o644))
	must(os.WriteFile(cliUsagePath, []byte(cliUsage(root)), 0o644))
	for _, p := range []string{schemaPath, cliDocPath, cliUsagePath} {
		fmt.Println("wrote", p)
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
}

// schemaJSON marshals the schema deterministically (sorted keys, indented,
// trailing newline).
func schemaJSON() []byte {
	b, err := json.MarshalIndent(schema(), "", "  ")
	must(err)
	return append(b, '\n')
}

// schema is the JSON Schema (draft 2020-12) for che.yml, mirroring
// internal/spec's types and their UnmarshalYAML union forms.
func schema() map[string]any {
	strList := func(desc string) map[string]any {
		return map[string]any{
			"description": desc,
			"type":        "array",
			"items":       map[string]any{"type": "string"},
		}
	}
	permProps := map[string]any{
		"owner":      map[string]any{"description": "dest owner user; empty: code default", "type": "string"},
		"ownerGroup": map[string]any{"description": "dest owner group; empty: code default", "type": "string"},
		"chmod":      map[string]any{"description": "dest mode, octal string", "type": "string", "pattern": "^[0-7]{3,4}$"},
	}
	fileGroup := func(itemsRef, desc string) map[string]any {
		props := map[string]any{
			"files": map[string]any{
				"description": "the group's items, each inheriting the group's perms",
				"type":        "array",
				"items":       map[string]any{"$ref": itemsRef},
			},
		}
		maps.Copy(props, permProps)
		return map[string]any{
			"description": desc,
			"type":        "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []any{"files"},
				"properties":           props,
			},
		}
	}
	return map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  schemaID,
		"title":                "che.yml",
		"description":          "che spec: every top-level key defines a profile block",
		"type":                 "object",
		"additionalProperties": map[string]any{"$ref": "#/$defs/profile"},
		"$defs": map[string]any{
			"profile": map[string]any{
				"description":          "one profile block: options self-describe eligibility, mixinProfiles compose in order, plugins pull remote profiles, include adds, exclude filters last and wins",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"options": map[string]any{
						"description":          "when the profile runs: autoExec opts in to bare-che runs, execIf predicates must ALL pass",
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"execIf": strList("predicate expressions `<source>` or `<source> == <literal>`, sources builtin:*/env:*; empty: always"),
							"autoExec": map[string]any{
								"description": "run on bare che (default false: runs only via --profile or mixinProfiles)",
								"type":        "boolean",
							},
						},
					},
					"mixinProfiles": strList("local profile names composed depth-first, in order"),
					"plugins": map[string]any{
						"description": "profiles loaded at their own checkout: `@<giturl>::<profile>` (remote) or `<dir>::<profile>` (local dir) string, or {ref, env}",
						"type":        "array",
						"items":       map[string]any{"$ref": "#/$defs/pluginEntry"},
					},
					"include": map[string]any{"$ref": "#/$defs/includeSet"},
					"exclude": map[string]any{"$ref": "#/$defs/excludeSet"},
				},
			},
			"pluginEntry": map[string]any{
				"oneOf": []any{
					map[string]any{
						"description": "`@<giturl>::<profile>` (remote) or `<dir>::<profile>` (local dir) plugin ref",
						"type":        "string",
						"pattern":     "^.+::.+$",
					},
					map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []any{"ref"},
						"properties": map[string]any{
							"ref": map[string]any{
								"description": "`@<giturl>::<profile>` (remote) or `<dir>::<profile>` (local dir) plugin ref",
								"type":        "string",
								"pattern":     "^.+::.+$",
							},
							"env": map[string]any{
								"description":          "envs exported around everything done for the plugin's unit",
								"type":                 "object",
								"additionalProperties": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
			"includeSet": map[string]any{
				"description":          "additive payload: link globs, copy/renderTemplates/mkdirs perm-groups, script globs, service names",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"link": map[string]any{
						"description": "symlink-op entries, repo-relative under root/: glob string (dest derived 1:1) or {source, dest} sed-style rewrite",
						"type":        "array",
						"items":       map[string]any{"$ref": "#/$defs/linkEntry"},
					},
					"copy":            fileGroup("#/$defs/fileEntry", "*.ontoHost.cp copy-op perm-groups"),
					"renderTemplates": fileGroup("#/$defs/fileEntry", "*.tpl render-op perm-groups; sources repo-root-relative, glob and derived-dest forms must be root/-prefixed"),
					"mkdirs":          fileGroup("#/$defs/dirEntry", "extra-dir perm-groups; each item one dir path (brace-expanded)"),
					"runScripts":      strList("script paths or globs, repo-relative, run in spec order"),
					"services":        strList("launchd service names"),
				},
			},
			"excludeSet": map[string]any{
				"description":          "subtractive glob filter, applied last, wins over every include (rich entries too)",
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"link":            strList("drop matching link items"),
					"copy":            strList("drop matching copy items (source or dest)"),
					"renderTemplates": strList("drop matching template items (source or dest)"),
					"mkdirs":          strList("drop matching dirs"),
					"runScripts":      strList("drop matching scripts (resolved file paths)"),
					"services":        strList("drop matching services"),
				},
			},
			"linkEntry": map[string]any{
				"oneOf": []any{
					map[string]any{
						"description": "glob over git-tracked files under root/ (brace-expanded), dest derived 1:1",
						"type":        "string",
					},
					map[string]any{
						"description":          "source file or glob with a sed-style dest rewrite",
						"type":                 "object",
						"additionalProperties": false,
						"required":             []any{"source", "dest"},
						"properties": map[string]any{
							"source": map[string]any{"description": "file or glob, repo-relative under root/", "type": "string"},
							"dest": map[string]any{
								"description": "sed-style rewrite s/<pattern>/<replacement>/[g] (Go regexp, $1 backrefs; g: every match, absent: first only), applied to the repo-relative dest path before host mapping",
								"type":        "string",
								"pattern":     "^s/.+/.*/g?$",
							},
						},
					},
				},
			},
			"fileEntry": map[string]any{
				"oneOf": []any{
					map[string]any{
						"description": "glob over git-tracked files (brace-expanded)",
						"type":        "string",
					},
					map[string]any{
						"description":          "one source fanned out to explicit dests",
						"type":                 "object",
						"additionalProperties": false,
						"required":             []any{"source"},
						"properties": map[string]any{
							"source": map[string]any{"description": "repo-relative source path", "type": "string"},
							"dest": map[string]any{
								"description": "dest paths: relative -> repo, ~/ or absolute -> host; omitted -> derived from the root/ source path",
								"type":        "array",
								"items":       map[string]any{"$ref": "#/$defs/destEntry"},
							},
						},
					},
				},
			},
			"dirEntry": map[string]any{
				"oneOf": []any{
					map[string]any{
						"description": "dir path (brace-expanded)",
						"type":        "string",
					},
					map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []any{"dest"},
						"properties": map[string]any{
							"dest": map[string]any{
								"description": "dir paths (brace-expanded)",
								"type":        "array",
								"items":       map[string]any{"$ref": "#/$defs/destEntry"},
							},
						},
					},
				},
			},
			"destEntry": map[string]any{
				"oneOf": []any{
					map[string]any{
						"description": "dest path: relative -> repo, ~/ or absolute -> host",
						"type":        "string",
					},
					map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []any{"path"},
						"properties": map[string]any{
							"path": map[string]any{"description": "dest path: relative -> repo, ~/ or absolute -> host", "type": "string"},
							"options": map[string]any{
								"description":          "per-dest render options",
								"type":                 "object",
								"additionalProperties": false,
								"properties": map[string]any{
									"writeType": map[string]any{
										"description": "how the rendered body lands: overwrite (default: header + body) | mergeUpsert (env KEY=VALUE union under the existing dest) | raw (body verbatim, no autogen header)",
										"enum":        []any{"", "mergeUpsert", "raw"},
									},
									"renderReferencedFiles": map[string]any{
										"description": "inline @-includes into the rendered body (overwrite only)",
										"type":        "boolean",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// envVars is che's environment-variable surface, documented in docs/cli.md.
var envVars = [][2]string{
	{"CHE_PROFILE", "as `--profile` (flag wins)"},
	{"CHE_DRY_RUN", "as `--dry-run` (`delta` or `all`)"},
	{"CHE_OMIT_EXEC_IF", "truthy: as `--omit-exec-if`"},
	{"CHE_SKIP_PLUGINS", "truthy: as `--skip-plugins`"},
	{"CHE_RENDER_TEMPLATES_DRY_RUN_SECRETS", "render-templates: skip sources carrying op:// secret refs (logged, dests untouched)"},
}

// cliDoc renders the CLI reference: root Long + global flags, every
// subcommand (depth-first, name-sorted), then the env table.
func cliDoc(root *cobra.Command) string {
	var b strings.Builder
	b.WriteString("<!-- autogenerated using internal/docgen -->\n")
	b.WriteString("# che CLI\n\n")
	b.WriteString(root.Long)
	b.WriteString("\n\n")
	b.WriteString("## Global flags\n\n```\n")
	b.WriteString(root.PersistentFlags().FlagUsages())
	b.WriteString("```\n\n## Commands\n")
	walkCommands(root, &b)
	b.WriteString("\n## Environment variables\n\n| Variable | Effect |\n| --- | --- |\n")
	for _, e := range envVars {
		fmt.Fprintf(&b, "| `%s` | %s |\n", e[0], e[1])
	}
	return b.String()
}

// cliUsage renders the README usage listing: every command (subcommands
// indented) with its Short, then the global flags. Included by the README
// template via renderMarkdown.
func cliUsage(root *cobra.Command) string {
	var rows [][2]string
	var collect func(cmd *cobra.Command, indent string)
	collect = func(cmd *cobra.Command, indent string) {
		subs := slices.Clone(cmd.Commands())
		slices.SortFunc(subs, func(a, c *cobra.Command) int { return strings.Compare(a.Name(), c.Name()) })
		for _, sub := range subs {
			if !sub.IsAvailableCommand() {
				continue
			}
			rows = append(rows, [2]string{indent + sub.Name(), sub.Short})
			collect(sub, indent+"  ")
		}
	}
	collect(root, "  ")
	width := 0
	for _, r := range rows {
		width = max(width, len(r[0]))
	}
	var b strings.Builder
	b.WriteString("che [command]\n\nAvailable Commands:\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-*s  %s\n", width, r[0], r[1])
	}
	b.WriteString("\nFlags:\n")
	b.WriteString(root.PersistentFlags().FlagUsages())
	return b.String()
}

func walkCommands(cmd *cobra.Command, b *strings.Builder) {
	subs := slices.Clone(cmd.Commands())
	slices.SortFunc(subs, func(a, c *cobra.Command) int { return strings.Compare(a.Name(), c.Name()) })
	for _, sub := range subs {
		if !sub.IsAvailableCommand() {
			continue
		}
		fmt.Fprintf(b, "\n### `$ %s`\n\n%s.\n", sub.CommandPath(), strings.TrimSuffix(sub.Short, "."))
		if sub.Long != "" {
			fmt.Fprintf(b, "\n%s\n", sub.Long)
		}
		if use := strings.TrimSuffix(sub.UseLine(), " [flags]"); use != sub.CommandPath() {
			fmt.Fprintf(b, "\nUsage: `%s`\n", sub.UseLine())
		}
		if fl := sub.NonInheritedFlags().FlagUsages(); fl != "" {
			fmt.Fprintf(b, "\n```\n%s```\n", fl)
		}
		walkCommands(sub, b)
	}
}

// [<] 🤖🤖
