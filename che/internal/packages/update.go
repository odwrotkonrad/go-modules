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
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/fetchx"
)

const DefaultUpdateBaseURL = "https://gitlab.com/api/v4/projects/konradodwrot%2Fche-packages/packages/generic/che-packages"

// [why] the URL above still points at go-modules: konradodwrot/che-packages serves
//
//	definitions once it has published a tag. Flip it with the MR that publishes
//	che-packages v0.0.1, or every che run warns and falls back until then.

const DefaultUpdateCooldown = 15 * time.Minute

const (
	currentMarker = "current"
	checkStamp    = "last-check"
)

func ResolveUpdateBaseURL(env map[string]string) string {
	return cmp.Or(env["CHE_PACKAGES_UPDATE_URL"], DefaultUpdateBaseURL)
}

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
	Updated bool
	Skipped string
}

func UpdateDefinitions(cacheDir, baseURL string, cooldown time.Duration, force bool) (UpdateResult, error) {
	stamp := filepath.Join(cacheDir, checkStamp)
	if !force && cooldown > 0 {
		if fi, err := os.Stat(stamp); err == nil && time.Since(fi.ModTime()) < cooldown {
			_, version, _ := ResolveCurrentDefinitions(cacheDir)
			return UpdateResult{Version: version, Skipped: "cooldown"}, nil
		}
	}
	b, err := fetchx.Default.Fetch(baseURL + "/latest/version.txt")
	if err != nil {
		return UpdateResult{}, fmt.Errorf("resolve latest definitions version: %w", err)
	}
	version := strings.TrimSpace(string(b))
	if version == "" {
		return UpdateResult{}, fmt.Errorf("resolve latest definitions version: empty version.txt at %s", baseURL)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return UpdateResult{}, err
	}
	versionDir := filepath.Join(cacheDir, version)
	if _, statErr := os.Stat(filepath.Join(versionDir, "packages.yml")); statErr == nil {
		if err := finishUpdate(cacheDir, version); err != nil {
			return UpdateResult{}, err
		}
		return UpdateResult{Version: version, Skipped: "up-to-date"}, nil
	}
	if err := downloadDefinitions(cacheDir, baseURL, version, versionDir); err != nil {
		return UpdateResult{}, err
	}
	if err := finishUpdate(cacheDir, version); err != nil {
		return UpdateResult{}, err
	}
	pruneDefinitions(cacheDir, version)
	return UpdateResult{Version: version, Updated: true}, nil
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
