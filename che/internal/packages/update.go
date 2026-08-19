package packages

// [>] 🤖🤖🤖

import (
	"archive/tar"
	"cmp"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"gitlab.com/konradodwrot/go-modules/che/internal/fetchx"
	"gitlab.com/konradodwrot/go-modules/che/internal/packages/builtin"
)

const DefaultSourceURL = "https://gitlab.com/api/v4/projects/konradodwrot%2Fche-packages/packages/generic/che-packages"

const DefaultUpdateCooldown = 15 * time.Minute

const (
	currentMarker = "current"
	checkStamp    = "last-check"
)

type SourceKind int

const (
	SourceRegistry SourceKind = iota
	SourceGit
)

// Source locates package definitions: where to read them from, and which version.
type Source struct {
	URL string
	Ref string
}

// Kind reports how the source publishes definitions, recognised from the URL's shape.
func (s Source) Kind() SourceKind {
	u := strings.TrimSuffix(s.URL, "/")
	switch {
	case strings.HasPrefix(u, "git+"), strings.HasSuffix(u, ".git"):
		return SourceGit
	case strings.HasPrefix(u, "git@"), strings.HasPrefix(u, "ssh://"):
		return SourceGit
	default:
		return SourceRegistry
	}
}

func (s Source) gitURL() string { return strings.TrimPrefix(s.URL, "git+") }

// Pinned reports whether the source names an exact version rather than the newest one.
func (s Source) Pinned() bool { return s.Ref != "" }

func ResolveSourceURL(env map[string]string) string {
	return cmp.Or(env["CHE_PACKAGES_SOURCE_URL"], env["CHE_PACKAGES_UPDATE_URL"], DefaultSourceURL)
}

func ResolveSourceRef(env map[string]string) string { return env["CHE_PACKAGES_REF"] }

func ResolveDefinitionsCacheDir(env map[string]string, home string) string {
	if che := env["CHE_CACHE_HOME"]; che != "" {
		return filepath.Join(che, "packages")
	}
	base := cmp.Or(env["XDG_CACHE_HOME"], filepath.Join(home, ".cache"))
	return filepath.Join(base, "che", "packages")
}

func ResolveCurrentDefinitions(cacheDir string) (dir, version string, ok bool) {
	b, err := os.ReadFile(filepath.Join(cacheDir, currentMarker))
	if err != nil {
		return "", "", false
	}
	version = strings.TrimSpace(string(b))
	dir = filepath.Join(cacheDir, version)
	if _, err := os.Stat(filepath.Join(dir, "packages.yml")); err != nil {
		return "", "", false
	}
	return dir, version, true
}

type UpdateResult struct {
	Version string
	// Previous is the version that was current before this update, empty on a first fetch.
	Previous string
	// Builtin is the catalog version compiled into this binary.
	Builtin string
	Updated bool
	Skipped string
}

// UpdateDefinitions fetches package definitions from src into cacheDir and marks them current.
//
// [why] the cooldown exists to stop repeated runs re-resolving which version is newest: a
//
//	pinned ref resolves nothing, so applying it there would only withhold the exact
//	version the caller asked for.
func UpdateDefinitions(cacheDir string, src Source, cooldown time.Duration, force bool) (UpdateResult, error) {
	src.URL = cmp.Or(src.URL, DefaultSourceURL)
	if src.Pinned() {
		cooldown = 0
	}
	//[why] read before anything replaces it: the caller reports which version this run moved away
	//   from, and after finishUpdate the marker already names the new one
	_, previous, _ := ResolveCurrentDefinitions(cacheDir)
	res := UpdateResult{Previous: previous, Builtin: builtin.Version()}
	stamp := filepath.Join(cacheDir, checkStamp)
	if !force && cooldown > 0 {
		if fi, err := os.Stat(stamp); err == nil && time.Since(fi.ModTime()) < cooldown {
			res.Version, res.Skipped = previous, "cooldown"
			return res, nil
		}
	}
	version, err := resolveVersion(src)
	if err != nil {
		return UpdateResult{}, err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return UpdateResult{}, err
	}
	res.Version = version
	versionDir := filepath.Join(cacheDir, version)
	if _, statErr := os.Stat(filepath.Join(versionDir, "packages.yml")); statErr == nil {
		if err := finishUpdate(cacheDir, version); err != nil {
			return UpdateResult{}, err
		}
		res.Skipped = "up-to-date"
		return res, nil
	}
	if err := fetchDefinitions(cacheDir, src, version, versionDir); err != nil {
		return UpdateResult{}, err
	}
	if err := finishUpdate(cacheDir, version); err != nil {
		return UpdateResult{}, err
	}
	pruneDefinitions(cacheDir, version)
	res.Updated = true
	return res, nil
}

func resolveVersion(src Source) (string, error) {
	if src.Kind() == SourceGit {
		return gitVersion(src)
	}
	if src.Pinned() {
		return src.Ref, nil
	}
	b, err := fetchx.Default.Fetch(src.URL + "/latest/version.txt")
	if err != nil {
		return "", fmt.Errorf("resolve latest definitions version: %w", err)
	}
	version := strings.TrimSpace(string(b))
	if version == "" {
		return "", fmt.Errorf("resolve latest definitions version: empty version.txt at %s", src.URL)
	}
	return version, nil
}

func fetchDefinitions(cacheDir string, src Source, version, versionDir string) error {
	if src.Kind() == SourceGit {
		return cloneDefinitions(cacheDir, src, versionDir)
	}
	return downloadDefinitions(cacheDir, src.URL, version, versionDir)
}

func downloadDefinitions(cacheDir, baseURL, version, versionDir string) error {
	name := "che-packages_" + version + ".tar.gz"
	tmp, err := os.MkdirTemp(cacheDir, ".update-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	archive := filepath.Join(tmp, name)
	if err := fetchx.Default.Download(baseURL+"/"+version+"/"+name, archive); err != nil {
		return fmt.Errorf("download definitions %s: %w", version, err)
	}
	sums, err := fetchx.Default.Fetch(baseURL + "/" + version + "/checksums.txt")
	if err != nil {
		return fmt.Errorf("fetch definitions checksums %s: %w", version, err)
	}
	if err := verifyArchiveChecksum(archive, name, string(sums)); err != nil {
		return err
	}
	extracted := filepath.Join(tmp, "extracted")
	if err := extractTarGz(archive, extracted); err != nil {
		return fmt.Errorf("extract %s: %w", name, err)
	}
	if _, err := os.Stat(filepath.Join(extracted, "packages.yml")); err != nil {
		return fmt.Errorf("definitions archive %s carries no packages.yml", name)
	}
	if err := os.Rename(extracted, versionDir); err != nil {
		return err
	}
	return nil
}

// [why] a shallow clone into a temp dir, not a fetch of two paths: a git source has no
//
//	manifest naming what the catalog consists of, and scripts/ is an open-ended tree.
func cloneDefinitions(cacheDir string, src Source, versionDir string) error {
	tmp, err := os.MkdirTemp(cacheDir, ".update-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	clone := filepath.Join(tmp, "clone")
	if _, err := cloneAt(src, clone); err != nil {
		return err
	}
	extracted := filepath.Join(tmp, "extracted")
	if err := os.MkdirAll(filepath.Join(extracted, "scripts"), 0o755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(clone, "packages.yml"), filepath.Join(extracted, "packages.yml")); err != nil {
		return fmt.Errorf("definitions repo %s carries no packages.yml: %w", src.URL, err)
	}
	if err := copyTree(filepath.Join(clone, "scripts"), filepath.Join(extracted, "scripts")); err != nil {
		return err
	}
	return os.Rename(extracted, versionDir)
}

// [why] the resolved commit, never the ref: a branch names different content over time, and a
//
//	cache dir keyed on it would serve the first clone forever.
func gitVersion(src Source) (string, error) {
	tmp, err := os.MkdirTemp("", "che-packages-ref-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	head, err := cloneAt(src, filepath.Join(tmp, "clone"))
	if err != nil {
		return "", err
	}
	return head, nil
}

func cloneAt(src Source, dest string) (string, error) {
	opts := &git.CloneOptions{URL: src.gitURL(), Depth: 1, SingleBranch: true, Tags: git.NoTags}
	if src.Pinned() {
		opts.ReferenceName = plumbing.NewBranchReferenceName(src.Ref)
	}
	repo, err := git.PlainClone(dest, false, opts)
	if err != nil && src.Pinned() {
		opts.ReferenceName = plumbing.NewTagReferenceName(src.Ref)
		_ = os.RemoveAll(dest)
		repo, err = git.PlainClone(dest, false, opts)
	}
	if err != nil {
		return "", fmt.Errorf("clone definitions from %s at %s: %w", src.URL, cmp.Or(src.Ref, "default branch"), err)
	}
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("resolve cloned head of %s: %w", src.URL, err)
	}
	return head.Hash().String()[:12], nil
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

func copyTree(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, info.Mode().Perm())
	})
}

func finishUpdate(cacheDir, version string) error {
	if err := os.WriteFile(filepath.Join(cacheDir, currentMarker), []byte(version+"\n"), 0o644); err != nil {
		return err
	}
	return touchCheckStamp(cacheDir)
}

func touchCheckStamp(cacheDir string) error {
	stamp := filepath.Join(cacheDir, checkStamp)
	if err := os.WriteFile(stamp, nil, 0o644); err != nil {
		return err
	}
	now := time.Now()
	return os.Chtimes(stamp, now, now)
}

func pruneDefinitions(cacheDir, keep string) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == keep || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		_ = os.RemoveAll(filepath.Join(cacheDir, e.Name()))
	}
}

func verifyArchiveChecksum(archive, name, sums string) error {
	want := ""
	for line := range strings.Lines(sums) {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksums.txt carries no entry for %s", name)
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("checksum mismatch for %s: want %s, got %s", name, want, got)
	}
	return nil
}

func extractTarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		rel := filepath.Clean(hdr.Name)
		if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			continue
		}
		target := filepath.Join(dest, rel)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil { //nolint:gosec
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
}

// [<] 🤖🤖🤖
