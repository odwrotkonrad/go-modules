package packages

// [>] 🤖🤖🤖

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

var versionTokenRe = regexp.MustCompile(`\d+(?:\.\d+)*(?:[.+~-][0-9A-Za-z.~+-]*)?`)

func PinMatches(out, pin string) bool {
	if pin == "" {
		return true
	}
	re, err := pinRegexp(pin)
	if err != nil {
		return strings.Contains(out, pin)
	}
	for _, token := range versionTokenRe.FindAllString(out, -1) {
		if re.MatchString(token) {
			return true
		}
	}
	return false
}

func pinRegexp(pin string) (*regexp.Regexp, error) {
	parts := strings.Split(pin, "*")
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	return regexp.Compile("^" + strings.Join(parts, `[0-9A-Za-z.~+-]+`) + "$")
}

func (in *Installer) pinFor(pkg, specVersion string) string {
	if r, ok := in.requested[pkg]; ok && len(r.Versions) > 0 {
		return r.globalVersion()
	}
	if e, ok := in.File.Packages[pkg]; ok && e.Version != "" {
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
		if v := r.globalVersion(); v != "" && !strings.Contains(v, "*") {
			return v, nil
		}
	}
	if b.Version != "" {
		return b.Version, nil
	}
	if !strings.Contains(b.URL+" "+b.Bin, "{version}") {
		return "", nil
	}
	constraint := ""
	if e, ok := in.File.Packages[pkg]; ok {
		constraint = e.Version
	}
	if constraint != "" && !strings.Contains(constraint, "*") {
		return constraint, nil
	}
	repo := b.VersionsFrom
	if repo == "" {
		repo = deriveTagsRepo(b.URL)
	}
	if repo == "" {
		return "", fmt.Errorf("%s: cannot resolve a concrete version: set version, versionsFrom, or request one", pkg)
	}
	return in.latestTagMatching(pkg, repo, constraint)
}

func deriveTagsRepo(url string) string {
	for _, host := range []string{"https://github.com/", "https://gitlab.com/"} {
		rest, ok := strings.CutPrefix(url, host)
		if !ok {
			continue
		}
		parts := strings.SplitN(rest, "/", 3)
		if len(parts) < 2 {
			return ""
		}
		return host + parts[0] + "/" + parts[1]
	}
	return ""
}

func (in *Installer) latestTagMatching(pkg, repo, constraint string) (string, error) {
	out, ok := in.output([]string{"git", "ls-remote", "--tags", repo})
	if !ok {
		return "", fmt.Errorf("%s: listing versions of %s failed", pkg, repo)
	}
	best := ""
	for line := range strings.Lines(out) {
		_, ref, found := strings.Cut(strings.TrimSpace(line), "refs/tags/")
		if !found || strings.HasSuffix(ref, "^{}") {
			continue
		}
		token := versionTokenRe.FindString(ref)
		if token == "" || strings.ContainsAny(token, "-~+") {
			continue
		}
		if !PinMatches(token, constraint) {
			continue
		}
		if best == "" || versionLess(best, token) {
			best = token
		}
	}
	if best == "" {
		return "", fmt.Errorf("%s: no version matching %q among tags of %s", pkg, constraint, repo)
	}
	in.emit(log.Levels.Debug, "resolved", pkg+" version "+best+" (constraint "+constraint+", tags of "+repo+")")
	return best, nil
}

func versionLess(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		av, bv := 0, 0
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av != bv {
			return av < bv
		}
	}
	return false
}

// [<] 🤖🤖🤖
