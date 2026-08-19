package packages

// [>] 🤖🤖🤖

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var versionTokenRe = regexp.MustCompile(`\d+(?:\.\d+)*(?:[.+~-][0-9A-Za-z.~+-]*)?`)

func PinMatches(out, pin string) bool {
	if pin == "" {
		return true
	}
	return slices.Contains(versionTokenRe.FindAllString(out, -1), pin) || strings.Contains(out, pin)
}

const VersionLatest = "latest"

func (in *Installer) resolvePin(pkg, specVersion string) string {
	if r, ok := in.requested[pkg]; ok && len(r.Versions) > 0 {
		return r.globalVersion()
	}
	if specVersion == VersionLatest {
		return ""
	}
	if specVersion != "" {
		return specVersion
	}
	if e, ok := in.File.Packages[pkg]; ok && e.Version != "" {
		if e.Version == VersionLatest {
			return ""
		}
		return e.Version
	}
	return ""
}

func (in *Installer) resolveChecksum(pkg, specChecksum string) string {
	if r, ok := in.requested[pkg]; ok && r.Checksum != "" {
		return r.Checksum
	}
	return specChecksum
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

func (in *Installer) checkRequestedPin(pkg, itemVersion string) error {
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

func (in *Installer) resolveArchiveVersion(pkg string, b *BinariesRemoteArchiveSpec) (string, error) {
	if v := in.resolvePin(pkg, b.Version); v != "" {
		return v, nil
	}
	e, ok := in.File.Packages[pkg]
	if b.Version == VersionLatest || (ok && e.Version == VersionLatest) {
		return VersionLatest, nil
	}
	if !strings.Contains(b.URL, "{version}") && !strings.Contains(strings.Join(b.ExtractBinaries, " "), "{version}") {
		return "", nil
	}
	return "", fmt.Errorf("%s: no version pinned: set version on the entry or the binariesRemoteArchive item", pkg)
}

// [<] 🤖🤖🤖
