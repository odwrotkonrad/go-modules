package packages

// [>] 🤖🤖🤖

import (
	"fmt"
	"regexp"
	"strings"
)

var versionTokenRe = regexp.MustCompile(`\d+(?:\.\d+)*(?:[.+~-][0-9A-Za-z.~+-]*)?`)

// PinMatches reports whether prose version output carries the pinned version.
// [why] output is prose ("go version go1.26.4 darwin/arm64"): compare against its version tokens
func PinMatches(out, pin string) bool {
	if pin == "" {
		return true
	}
	for _, token := range versionTokenRe.FindAllString(out, -1) {
		if token == pin {
			return true
		}
	}
	return strings.Contains(out, pin)
}

// VersionUnversionedLatest marks a package the manager ships in exactly one, unversioned stream:
// stating it explicitly keeps an absent version meaning "not yet decided" rather than "nothing to pin".
const VersionUnversionedLatest = "__unversioned_latest__"

func (in *Installer) pinFor(pkg, specVersion string) string {
	if r, ok := in.requested[pkg]; ok && len(r.Versions) > 0 {
		return r.globalVersion()
	}
	if e, ok := in.File.Packages[pkg]; ok && e.Version != "" {
		if e.Version == VersionUnversionedLatest {
			return ""
		}
		return e.Version
	}
	return specVersion
}

func (r Request) globalVersion() string {
	if r.Global != "" {
		return r.Global
	}
	if len(r.Versions) > 0 {
		return r.Versions[0]
	}
	return ""
}

// [why] a pinned asset's sha256 only covers the version the item declares
func (in *Installer) requestedOverridesPin(pkg, itemVersion string) error {
	r, ok := in.requested[pkg]
	if !ok || len(r.Versions) == 0 || itemVersion == "" {
		return nil
	}
	if len(r.Versions) > 1 {
		return fmt.Errorf("%s: multiple versions need a version-manager installation method (%s)", pkg, in.FilePath)
	}
	if r.Versions[0] == itemVersion {
		return nil
	}
	return fmt.Errorf("%s: requested version %s but %s pins %s (no checksum for the requested version)", pkg, r.Versions[0], in.FilePath, itemVersion)
}

func (in *Installer) resolveArchiveVersion(pkg string, b *PrebuiltArchiveSpec) (string, error) {
	if r, ok := in.requested[pkg]; ok {
		if v := r.globalVersion(); v != "" {
			return v, nil
		}
	}
	if b.Version != "" {
		return b.Version, nil
	}
	if e, ok := in.File.Packages[pkg]; ok && e.Version != "" && e.Version != VersionUnversionedLatest {
		return e.Version, nil
	}
	// [why] a version-less url (vendor "latest" endpoint) needs no pin
	if !strings.Contains(b.URL, "{version}") && !strings.Contains(b.Bin, "{version}") {
		return "", nil
	}
	return "", fmt.Errorf("%s: no version pinned: set version on the entry or the prebuiltArchive item", pkg)
}

// [<] 🤖🤖🤖
