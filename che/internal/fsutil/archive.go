package fsutil

// [>] 🤖🤖

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dsnet/compress/bzip2"
)

const TsLayout = "20060102T150405"

func ResolveBackupArchivePath(home, profileSlug, op, ts, backupID string) string {
	name := fmt.Sprintf("%s-%s.tar.bz2", ts, backupID)
	return filepath.Join(ResolveStateHome(home), "backups", profileSlug, op, name)
}

func SlugRef(ref string) string {
	s := strings.NewReplacer("/", "-", ":", "-").Replace(ref)
	return strings.Trim(s, "-")
}

func ParseBackupArchiveName(path string) (ts, backupID string) {
	base := strings.TrimSuffix(filepath.Base(path), ".tar.bz2")
	ts, backupID, _ = strings.Cut(base, "-")
	return ts, backupID
}

func (f FS) ArchiveDestinations(archivePath string, dests []string) error {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()
	bz, err := bzip2.NewWriter(out, nil)
	if err != nil {
		return err
	}
	defer bz.Close()
	tw := tar.NewWriter(bz)
	defer tw.Close()
	for _, dest := range dests {
		if err := archiveDest(tw, dest); err != nil {
			return err
		}
	}
	return nil
}

func archiveDest(tw *tar.Writer, dest string) error {
	fi, err := os.Stat(dest) // [why] follow links: back up contents, not the link
	if err != nil {
		return nil // [why] missing dest or broken link: nothing to preserve
	}
	if fi.IsDir() {
		return nil
	}
	hdr, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}
	hdr.Name = filepath.ToSlash(strings.TrimPrefix(dest, "/"))
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	body, err := os.ReadFile(dest)
	if err != nil {
		return err
	}
	_, err = tw.Write(body)
	return err
}

func ListArchiveEntries(archivePath string) ([]string, error) {
	in, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	bz, err := bzip2.NewReader(in, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = bz.Close() }()
	tr := tar.NewReader(bz)
	var out []string
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, "/"+filepath.FromSlash(hdr.Name))
	}
}

func ReadFromArchive(archivePath, dest string) (body []byte, mode os.FileMode, found bool, err error) {
	in, err := os.Open(archivePath)
	if err != nil {
		return nil, 0, false, err
	}
	defer in.Close()
	bz, err := bzip2.NewReader(in, nil)
	if err != nil {
		return nil, 0, false, err
	}
	defer func() { _ = bz.Close() }()
	want := filepath.ToSlash(strings.TrimPrefix(dest, "/"))
	tr := tar.NewReader(bz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, 0, false, nil
		}
		if err != nil {
			return nil, 0, false, err
		}
		if hdr.Name != want {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return nil, 0, false, err
		}
		return body, os.FileMode(hdr.Mode), true, nil
	}
}

// [<] 🤖🤖
