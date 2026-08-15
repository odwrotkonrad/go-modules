package packages

// [>] 🤖🤖

import (
	"bytes"
	"encoding/json"
	"sync"

	"github.com/invopop/jsonschema"
	sj "github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaID = "https://gitlab.com/konradodwrot/go-modules/-/raw/main/che/assets/data/packages.schema.json"

const checksumPattern = "^sha256:.+"

func Schema() *jsonschema.Schema {
	defs := jsonschema.Definitions{
		"InstallerList":         installerListDef(),
		"AptRegistry":           aptRegistryDef(),
		"NixRegistry":           nixRegistryDef(),
		"Entry":                 entryDef(),
		"Item":                  itemDef(),
		"ItemBody":              itemBodyDef(),
		"BinariesRemoteArchive": binariesRemoteArchiveDef(),
		"BuildFromSource":       buildFromSourceDef(),
		"Script":                scriptDef(),
		"Apt":                   aptDef(),
		"Nix":                   nixDef(),
		"VersionManager":        versionManagerDef(),
		"PlatformEligibility":   platformEligibilityDef(),
		"Verify":                verifyDef(),
		"ExtractBinaries":       extractBinariesDef(),
		"ExtractManpages":       extractManpagesDef(),
		"Completions":           completionsDef(),
		"Manpages":              manpagesDef(),
	}
	root := &jsonschema.Schema{
		Version:              jsonschema.Version,
		ID:                   schemaID,
		Title:                "packages.yml",
		Description:          "che packages file: installer eligibility per platform, installer registries, base packages, package entries",
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
		Definitions:          defs,
		Properties:           jsonschema.NewProperties(),
	}
	root.Properties.Set("archSchemes", &jsonschema.Schema{
		Description:          "named {arch: token} sets substituted for {arch} in binariesRemoteArchive urls and extractBinaries",
		Type:                 "object",
		AdditionalProperties: stringMap(""),
	})
	root.Properties.Set("osInstallers", &jsonschema.Schema{
		Description:          "eligible installers per <os>[-<distro>] key, most specific key wins",
		Type:                 "object",
		AdditionalProperties: ref("InstallerList"),
	})
	registries := obj("installer package sources referenced by fromRegistry", nil)
	registries.Properties.Set("apt", arr(ref("AptRegistry")))
	registries.Properties.Set("brew", arr(str("brew tap name (<user>/<repo>)")))
	registries.Properties.Set("nix", arr(ref("NixRegistry")))
	root.Properties.Set("installerRegistries", registries)
	root.Properties.Set("basePackages", &jsonschema.Schema{
		Description:          "prerequisite package names per group: common always, an installer name when that installer is eligible",
		Type:                 "object",
		AdditionalProperties: arr(str("")),
	})
	toolPkgs := &jsonschema.Schema{
		Description:          "tool-scoped packages per host tool: package name to version pin, null or empty value means rolling",
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
		Properties:           jsonschema.NewProperties(),
	}
	for _, tool := range KnownTools() {
		toolPkgs.Properties.Set(tool, &jsonschema.Schema{
			Type: "object",
			AdditionalProperties: &jsonschema.Schema{OneOf: []*jsonschema.Schema{
				{Description: "version pin", Type: "string"},
				{Description: "rolling", Type: "null"},
			}},
		})
	}
	root.Properties.Set("toolPackages", toolPkgs)
	root.Properties.Set("packages", &jsonschema.Schema{
		Description:          "package entries by canonical command name",
		Type:                 "object",
		AdditionalProperties: ref("Entry"),
	})
	return root
}

func installerListDef() *jsonschema.Schema {
	return &jsonschema.Schema{
		Description: "installer names, scalars or alias-nested lists (flattened on load)",
		OneOf: []*jsonschema.Schema{
			str(""),
			arr(ref("InstallerList")),
		},
	}
}

func aptRegistryDef() *jsonschema.Schema {
	o := obj("one apt repository, referenced by scheme-less url[::suites[::components]]", []string{"url"})
	o.Properties.Set("url", str("repository url"))
	o.Properties.Set("verificationKey", str("keyring path or key url"))
	o.Properties.Set("suites", str(""))
	o.Properties.Set("components", str(""))
	return o
}

func entryDef() *jsonschema.Schema {
	o := obj("entry object: installers plus shared fields", nil)
	o.Properties.Set("installers", arr(ref("Item")))
	o.Properties.Set("version", str("pinned version, absent means rolling"))
	o.Properties.Set("requires", arr(str("package names installed first")))
	o.Properties.Set("postInstall", ref("Script"))
	o.Properties.Set("command", str("binary name when it differs from the entry key"))
	o.Properties.Set("versionCommand", str("command printing the installed version"))
	o.Properties.Set("verify", ref("Verify"))
	o.Properties.Set("completions", ref("Completions"))
	o.Properties.Set("manpages", ref("Manpages"))
	return &jsonschema.Schema{
		Description: "one package: installer item list, or an object adding version/requires/postInstall/command/versionCommand/completions/manpages",
		OneOf: []*jsonschema.Schema{
			arr(ref("Item")),
			o,
		},
	}
}

func itemDef() *jsonschema.Schema {
	enum := make([]any, len(PlatformMethods))
	for i, m := range PlatformMethods {
		enum[i] = m
	}
	one := uint64(1)
	m := &jsonschema.Schema{
		Description:          "single-key map: installer name to its spec",
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
		MinProperties:        &one,
		MaxProperties:        &one,
		Properties:           jsonschema.NewProperties(),
	}
	m.Properties.Set("binariesRemoteArchive", ref("BinariesRemoteArchive"))
	m.Properties.Set("buildFromSource", ref("BuildFromSource"))
	m.Properties.Set("script", ref("Script"))
	m.Properties.Set("apt", ref("Apt"))
	m.Properties.Set("nix", ref("Nix"))
	m.Properties.Set("pyenv", ref("VersionManager"))
	m.Properties.Set("nvm", ref("VersionManager"))
	for _, mgr := range []string{"brew", "brew/cask", "npm", "go", "gem"} {
		m.Properties.Set(mgr, ref("ItemBody"))
	}
	return &jsonschema.Schema{
		Description: "one installer: bare installer name, or a single-key map with its spec",
		OneOf: []*jsonschema.Schema{
			{Description: "bare installer name", Type: "string", Enum: enum},
			m,
		},
	}
}

func itemBodyDef() *jsonschema.Schema {
	o := obj("manager item spec", nil)
	o.Properties.Set("packageName", str("manager-side package name when it differs from the entry key"))
	o.Properties.Set("aliasBinary", stringMap("alias name in the bin dir, or dest path (~ and $VARS expand, parent dirs created), to target binary"))
	o.Properties.Set("version", str("pinned version, absent means rolling"))
	o.Properties.Set("fromRegistry", str("installerRegistries ref (brew tap, apt url), brew/brew-cask/apt only"))
	o.Properties.Set("verify", ref("Verify"))
	o.Properties.Set("manpages", ref("Manpages"))
	return o
}

func binariesRemoteArchiveDef() *jsonschema.Schema {
	o := obj("download an archive and extract binaries onto PATH", nil)
	o.Properties.Set("packageName", str(""))
	o.Properties.Set("versionMap", stringMap("binary version to archive version"))
	o.Properties.Set("archScheme", str("archSchemes set substituted for {arch}"))
	o.Properties.Set("aliasBinary", stringMap("alias name in the bin dir, or dest path (~ and $VARS expand, parent dirs created), to target binary"))
	o.Properties.Set("platformEligibility", ref("PlatformEligibility"))
	o.Properties.Set("extractBinaries", ref("ExtractBinaries"))
	o.Properties.Set("version", str("pinned version, absent means rolling"))
	o.Properties.Set("url", str("archive url with {version}/{os}/{arch} tokens"))
	o.Properties.Set("verify", ref("Verify"))
	o.Properties.Set("extractManpages", ref("ExtractManpages"))
	o.Properties.Set("manpages", ref("Manpages"))
	return o
}

func buildFromSourceDef() *jsonschema.Schema {
	o := obj("download a source tarball, then ./configure && make && make install", []string{"url"})
	o.Properties.Set("version", str("pinned version, absent means rolling"))
	o.Properties.Set("url", str("source tarball url with a {version} token"))
	o.Properties.Set("checksum", &jsonschema.Schema{Description: "sha256:<hex> of the source tarball", Type: "string", Pattern: checksumPattern})
	o.Properties.Set("configureArgs", arr(str("extra ./configure arguments")))
	o.Properties.Set("platformEligibility", &jsonschema.Schema{
		Description: "eligible <os>-<arch> platforms, names only; empty means all",
		Type:        "array",
		Items:       str("platform name <os>-<arch>"),
	})
	o.Properties.Set("verify", ref("Verify"))
	o.Properties.Set("manpages", ref("Manpages"))
	return o
}

func scriptDef() *jsonschema.Schema {
	o := obj("script spec: run inline, path on disk, or url", nil)
	o.Properties.Set("run", str("inline command"))
	o.Properties.Set("args", arr(str("")))
	o.Properties.Set("path", str("script path, packages-file-relative"))
	o.Properties.Set("url", str("script url, downloaded then run"))
	o.Properties.Set("os", str("run only on this os"))
	o.Properties.Set("version", str(""))
	o.Properties.Set("env", stringMap("env exported around the run"))
	o.Properties.Set("validateArtifact", str("path whose presence marks the install done"))
	o.Properties.Set("platformEligibility", ref("PlatformEligibility"))
	o.Properties.Set("manpages", ref("Manpages"))
	return scalarOr("inline command", o)
}

func aptDef() *jsonschema.Schema {
	one := uint64(1)
	vm := stringMap("exactly one binary version to package version pin")
	vm.MaxProperties = &one
	o := obj("apt item spec", nil)
	o.Properties.Set("packageName", str("apt package name when it differs from the entry key"))
	o.Properties.Set("versionMap", vm)
	o.Properties.Set("aliasBinary", stringMap("alias name in the bin dir, or dest path (~ and $VARS expand, parent dirs created), to target binary"))
	o.Properties.Set("fromRegistry", str("installerRegistries.apt ref: scheme-less url[::suites[::components]]"))
	o.Properties.Set("verify", ref("Verify"))
	o.Properties.Set("manpages", ref("Manpages"))
	return o
}

func nixRegistryDef() *jsonschema.Schema {
	o := obj("one nix flake source, referenced by name", []string{"name", "url"})
	o.Properties.Set("name", str("registry name referenced by fromRegistry"))
	o.Properties.Set("url", str("flake base url without a branch (github:NixOS/nixpkgs)"))
	o.Properties.Set("ref", str("branch or tag installs track when no revision pins"))
	return o
}

func nixDef() *jsonschema.Schema {
	one := uint64(1)
	vm := stringMap("exactly one binary version to registry-repo revision pin")
	vm.MaxProperties = &one
	o := obj("nix item spec", nil)
	o.Properties.Set("packageName", str("flake attribute when it differs from the entry key"))
	o.Properties.Set("versionMap", vm)
	o.Properties.Set("aliasBinary", stringMap("alias name in the bin dir, or dest path (~ and $VARS expand, parent dirs created), to target binary"))
	o.Properties.Set("fromRegistry", str("installerRegistries.nix ref by name; default nixpkgs"))
	o.Properties.Set("verify", ref("Verify"))
	o.Properties.Set("manpages", ref("Manpages"))
	return o
}

func verifyDef() *jsonschema.Schema {
	o := obj("verify options: strategy keys plus presence-check tuning, multiple keys combine", nil)
	o.Properties.Set(VerifyVersionCmd, &jsonschema.Schema{Description: "run the entry command with --version (fallback version), exit 0 and non-empty output required", Type: "boolean"})
	o.Properties.Set(VerifyPkgMgrVersionCheck, &jsonschema.Schema{Description: "ask the installing manager for the installed version, exit 0 and non-empty output required", Type: "boolean"})
	o.Properties.Set("cmd", str("command run after install, exit 0 = verified"))
	o.Properties.Set("checkInPath", &jsonschema.Schema{Description: "probe PATH for the package command during presence checks (default true); false for command-less packages", Type: "boolean"})
	return &jsonschema.Schema{
		Description: "install verification: a named strategy shorthand or an options object",
		OneOf: []*jsonschema.Schema{
			{Description: "strategy: versionCmd runs the entry command with --version, pkgMgrVersionCheck asks the installing manager for the installed version", Type: "string", Enum: []any{VerifyVersionCmd, VerifyPkgMgrVersionCheck}},
			o,
		},
	}
}

func versionManagerDef() *jsonschema.Schema {
	o := obj("version-manager item: installed tool versions", nil)
	o.Properties.Set("versions", arr(str("")))
	o.Properties.Set("global", str("version set as the global default"))
	return o
}

func platformEligibilityDef() *jsonschema.Schema {
	one := uint64(1)
	return &jsonschema.Schema{
		Description: "eligible <os>-<arch> platforms, bare or with an archive checksum",
		Type:        "array",
		Items: &jsonschema.Schema{OneOf: []*jsonschema.Schema{
			str("platform name <os>-<arch>"),
			{
				Description:          "single platform: sha256:<hex> checksum pair",
				Type:                 "object",
				AdditionalProperties: &jsonschema.Schema{Type: "string", Pattern: checksumPattern},
				MinProperties:        &one,
				MaxProperties:        &one,
			},
		}},
	}
}

func extractBinariesDef() *jsonschema.Schema {
	return &jsonschema.Schema{
		Description: "archive paths extracted onto PATH, {version}/{os}/{arch} tokens allowed",
		OneOf: []*jsonschema.Schema{
			str(""),
			arr(str("")),
		},
	}
}

func extractManpagesDef() *jsonschema.Schema {
	return &jsonschema.Schema{
		Description: "archive manpage paths linked into the man dir's man<section> subdir, {version}/{os}/{arch} tokens allowed",
		OneOf: []*jsonschema.Schema{
			str(""),
			arr(str("")),
		},
	}
}

func manpagesDef() *jsonschema.Schema {
	return &jsonschema.Schema{
		Description: "manual pages the package ships (<name>.<section>); on an item it overrides the entry list for that method, [] means none",
		Type:        "array",
		Items:       str("manpage name <name>.<section>"),
	}
}

func completionsDef() *jsonschema.Schema {
	z := obj("zsh completion file from a command or url, exactly one of cmd/url", nil)
	z.Properties.Set("name", str("completion file name, defaults to _<entry>"))
	z.Properties.Set("cmd", str("command printing the completion file"))
	z.Properties.Set("url", str("completion file url"))
	z.Properties.Set("checksum", &jsonschema.Schema{Description: "sha256:<hex> of the fetched file", Type: "string", Pattern: checksumPattern})
	o := obj("", nil)
	o.Properties.Set("zsh", z)
	return o
}

func ref(name string) *jsonschema.Schema { return &jsonschema.Schema{Ref: "#/$defs/" + name} }

func str(desc string) *jsonschema.Schema {
	return &jsonschema.Schema{Description: desc, Type: "string"}
}

func arr(items *jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{Type: "array", Items: items}
}

func stringMap(desc string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Description:          desc,
		Type:                 "object",
		AdditionalProperties: &jsonschema.Schema{Type: "string"},
	}
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

var compiledSchema = sync.OnceValues(func() (*sj.Schema, error) {
	b, err := json.Marshal(Schema())
	if err != nil {
		return nil, err
	}
	doc, err := sj.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	c := sj.NewCompiler()
	if err := c.AddResource("packages.schema.json", doc); err != nil {
		return nil, err
	}
	return c.Compile("packages.schema.json")
})

func CompiledSchema() (*sj.Schema, error) { return compiledSchema() }

// [<] 🤖🤖
