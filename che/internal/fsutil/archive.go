package fsutil

// [>] 🤖🤖

import (
	"archive/tar"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dsnet/compress/bzip2"
)

// TsLayout stamps backup filenames + log lines (one stamp per run).
const TsLayout = "20060102T150405"

// ResolveBackupArchivePath resolves the XDG state backups dir + per-run archive
// filename: <ResolveStateHome>/backups/<bin>-<sub>-<ts>.tar.bz2 (default
// ~/.local/state/che/backups).
func ResolveBackupArchivePath(home, bin, sub, ts string) string {
	name := fmt.Sprintf("%s-%s-%s.tar.bz2", bin, sub, ts)
	return filepath.Join(ResolveStateHome(home), "backups", name)
}

// ArchiveDestinations snapshots each existing dest's contents into a single .tar.bz2
// at archivePath, entries named by stripped-absolute path. Symlinks followed
// (linked contents stored, not the link); missing dests, broken links + dirs
// skipped. Always writes the archive, even empty.
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

// [<] 🤖🤖
