package host

// [>] 🤖🤖

import (
	"bytes"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gitlab.com/konradodwrot/go-modules/che/internal/fsutil"
	"gitlab.com/konradodwrot/go-modules/che/internal/spec"
)

// Bin is che's binary name, carried in per-run backup archive filenames.
const Bin = "che"

// archiveBefore snapshots every existing dest into one per-run .tar.bz2 under
// the XDG backups dir before a mutating op runs (sub = op identity).
func (h Host) archiveBefore(sub string, dests []string) error {
	ts := time.Now().Format(fsutil.TsLayout)
	path := fsutil.BackupArchivePath(h.Home, Bin, sub, ts)
	return h.fs.ArchiveDests(path, dests)
}

// failItem logs "<op>(fail): <dest>: <err>" and returns err, the per-item
// continue-on-error hook: ops collect these and errors.Join at the end.
func (h Host) failItem(op, dest string, err error) error {
	h.fs.Log(op+"(fail)", dest+": "+err.Error())
	return err
}

// MkDirs creates repo-tree ancestor dirs (parents first) plus profile
// extra-dirs. A failing dir is logged and the rest still run; failures join
// into the returned error.
func (h Host) MkDirs(dirRels []string, extraDirs []spec.FileItem) error {
	errs := []error{h.ensureConfigDirs(dirRels)}
	for _, item := range extraDirs {
		dest := h.ToDest(item.Dests[0].Path)
		if err := h.upsertExtraDir(item, dest); err != nil {
			errs = append(errs, h.failItem("mkdir", dest, err))
		}
	}
	return errors.Join(errs...)
}

// upsertExtraDir settles one extra-dir: existing dest -> perms drift fix only,
// else create with spec perms.
func (h Host) upsertExtraDir(item spec.FileItem, dest string) error {
	if h.dirSettled(dest) {
		return h.fixPerms("mkdir", dest, item)
	}
	return h.mkExtraDir(item, dest)
}

// ensureConfigDirs creates repo-tree ancestor dirs (parents first), no spec
// perms: mode 0 -> mkdir honors umask. Idempotent; failures collect per dir.
func (h Host) ensureConfigDirs(dirRels []string) error {
	var errs []error
	for _, rel := range dirRels {
		dest := h.ToDest(rel)
		if h.dirSettled(dest) {
			continue
		}
		if err := h.fs.Mkdir(dest, 0, false); err != nil {
			errs = append(errs, h.failItem("mkdir", dest, err))
		}
	}
	return errors.Join(errs...)
}

// dirSettled reports whether dest already exists as a dir and may be skipped
// (DryRunAll forces every dest to report, so it never skips).
func (h Host) dirSettled(dest string) bool {
	return !h.DryRunAll() && fsutil.IsDir(dest)
}

// mkExtraDir creates one extra-dir with -p. Owner applied via chown (not mkdir
// -u). Mode 0 -> umask. Set-bits (>0777) reapplied via chmod since mkdir -m may
// drop them.
func (h Host) mkExtraDir(item spec.FileItem, dest string) error {
	mode, _ := parseMode(item.Chmod)
	if err := h.fs.Mkdir(dest, mode, true); err != nil {
		return err
	}
	if mode > 0o777 {
		if err := h.fs.Chmod("mkdir(chmod)", fsutil.ModeArg(mode), dest); err != nil {
			return err
		}
	}
	if owner := ownerSpec(item); owner != "" {
		return h.fs.Chown("mkdir(chown)", owner, dest)
	}
	return nil
}

// fixPerms applies spec mode/owner to an existing dest when they drift, labeling
// the fixes with the owning op ("<op>(chmod)" / "<op>(chown)"). In DryRunDelta
// these lines report the drift; in DryRunOff they correct it. A settled dest
// (no drift) emits nothing. DryRunAll never reaches here (dests are re-created).
func (h Host) fixPerms(op, dest string, item spec.FileItem) error {
	needChmod, needChown := h.permsDrift(dest, item)
	if needChmod {
		mode, _ := parseMode(item.Chmod)
		if err := h.fs.Chmod(op+"(chmod)", fsutil.ModeArg(mode), dest); err != nil {
			return err
		}
	}
	if needChown {
		if err := h.fs.Chown(op+"(chown)", ownerSpec(item), dest); err != nil {
			return err
		}
	}
	return nil
}

// permsDrift reports whether dest's live mode/owner differ from the spec. Only
// spec-set fields are enforced (empty Chmod/owner -> no drift). Missing dest ->
// no drift (the create path handles it).
func (h Host) permsDrift(dest string, item spec.FileItem) (needChmod, needChown bool) {
	fi, err := os.Lstat(dest)
	if err != nil {
		return false, false
	}
	if mode, ok := parseMode(item.Chmod); ok {
		mask := modeMask(mode)
		needChmod = mode&mask != unixMode(fi.Mode())&mask
	}
	if owner := ownerSpec(item); owner != "" {
		needChown = ownerDrift(fi, owner)
	}
	return needChmod, needChown
}

// modeMask is the raw-unix bit set the spec controls: perm bits always, plus
// setuid/setgid/sticky when the spec mode carries them (>0777, matching mkExtraDir).
func modeMask(mode os.FileMode) os.FileMode {
	if mode > 0o777 {
		return 0o7777
	}
	return 0o777
}

// unixMode maps an os.FileMode's Go-encoded special bits (ModeSetuid/Setgid/
// Sticky live in high bits, not 0o7000) down to raw-unix perm+special bits, so
// it compares equal to a parseMode octal like 0o2775. Perm bits pass through.
func unixMode(m os.FileMode) os.FileMode {
	u := m.Perm()
	if m&os.ModeSetuid != 0 {
		u |= 0o4000
	}
	if m&os.ModeSetgid != 0 {
		u |= 0o2000
	}
	if m&os.ModeSticky != 0 {
		u |= 0o1000
	}
	return u
}

// ownerDrift reports whether fi's live uid/gid differ from the "owner[:group]"
// spec. Unresolvable spec names or a missing Stat_t -> no drift (can't compare).
func ownerDrift(fi os.FileInfo, owner string) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	name, group, _ := strings.Cut(owner, ":")
	uid, uidOK := lookupID(name, user.Lookup, func(u *user.User) string { return u.Uid })
	gid, gidOK := lookupID(group, user.LookupGroup, func(g *user.Group) string { return g.Gid })
	return (uidOK && uid != st.Uid) || (gidOK && gid != st.Gid)
}

// lookupID resolves name to a numeric id via lookup+idOf ("" name -> not set).
// Unresolvable name -> not set (ok=false), so the caller treats it as no drift.
func lookupID[T any](name string, lookup func(string) (T, error), idOf func(T) string) (uint32, bool) {
	if name == "" {
		return 0, false
	}
	rec, err := lookup(name)
	if err != nil {
		return 0, false
	}
	id, err := strconv.ParseUint(idOf(rec), 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(id), true
}

// MkLinks symlinks each config into its live dest (ln -fhs), archiving existing
// dests upfront, skipping links already pointing into the repo. A failing link
// is logged and the rest still run; failures join into the returned error.
func (h Host) MkLinks(links []spec.FileItem, dirRels []string) error {
	errs := []error{h.ensureConfigDirs(dirRels)}
	dests := make([]string, len(links))
	for i, item := range links {
		dests[i] = h.ToDest(item.Rel)
	}
	if err := h.archiveBefore("link", dests); err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, item := range links {
		src := h.Src(item.Rel)
		dest := h.ToDest(item.Rel)
		if h.linkSettled(src, dest) {
			continue
		}
		if err := h.fs.Symlink(src, dest); err != nil {
			errs = append(errs, h.failItem("ln", dest, err))
		}
	}
	return errors.Join(errs...)
}

// linkSettled reports whether dest already resolves to src (skippable). DryRunAll
// forces every dest to report, so it never skips.
func (h Host) linkSettled(src, dest string) bool {
	if h.DryRunAll() {
		return false
	}
	destResolved, err := filepath.EvalSymlinks(dest)
	if err != nil {
		return false
	}
	srcResolved, err := filepath.EvalSymlinks(src)
	return err == nil && destResolved == srcResolved
}

// MkCopies copies each *.ontoHost.cp to its dest(s) (marker stripped, or explicit
// dest) when contents differ, archiving existing dests upfront, applying spec
// perms (else default). A failing dest is logged and the rest still run;
// failures join into the returned error.
func (h Host) MkCopies(copies []spec.FileItem, dirRels []string) error {
	errs := []error{h.ensureConfigDirs(dirRels)}
	var dests []string
	for _, item := range copies {
		dests = append(dests, h.copyDests(item)...)
	}
	if err := h.archiveBefore("copy", dests); err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, item := range copies {
		for _, dest := range h.copyDests(item) {
			if err := h.copyOne(item, dest); err != nil {
				errs = append(errs, h.failItem("cp", dest, err))
			}
		}
	}
	return errors.Join(errs...)
}

// copyOne settles one copy dest: same content -> perms drift fix only, else
// copy with spec mode then chown when an owner is set.
func (h Host) copyOne(item spec.FileItem, dest string) error {
	src := h.Src(item.Rel)
	if !h.DryRunAll() && sameContent(src, dest) {
		return h.fixPerms("cp", dest, item)
	}
	mode, _ := parseMode(item.Chmod)
	if err := h.fs.Copy(src, dest, mode); err != nil {
		return err
	}
	if owner := ownerSpec(item); owner != "" {
		return h.fs.Chown("cp(chown)", owner, dest)
	}
	return nil
}

// copyDests returns the explicit dests (~/ resolved), else the marker-stripped derived dest.
func (h Host) copyDests(item spec.FileItem) []string {
	if len(item.Dests) == 0 {
		return []string{h.ToDest(strings.TrimSuffix(item.Rel, spec.CpExt))}
	}
	out := make([]string, len(item.Dests))
	for i, d := range item.Dests {
		out[i] = h.expandHome(d.Path)
	}
	return out
}

// ownerSpec combines owner + owner-group into "owner:group" for fs.Chown ("" -> no chown).
func ownerSpec(item spec.FileItem) string {
	if item.Owner == "" {
		return ""
	}
	if item.OwnerGroup == "" {
		return item.Owner
	}
	return item.Owner + ":" + item.OwnerGroup
}

// parseMode parses an octal chmod string ("" -> not set).
func parseMode(s string) (os.FileMode, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, false
	}
	return os.FileMode(n), true
}

// expandHome resolves a leading ~/ to the host home.
func (h Host) expandHome(p string) string {
	return fsutil.ExpandHome(p, h.Home)
}

func sameContent(a, b string) bool {
	x, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	y, err := os.ReadFile(b)
	if err != nil {
		return false
	}
	return bytes.Equal(x, y)
}

// PruneBrokenLinks removes broken symlinks in config-set dirs (live dests). A
// failing removal is logged and the rest still run; failures join into the
// returned error.
func (h Host) PruneBrokenLinks(dirRels []string) error {
	h.fs.Log("prune-links", h.Root)
	var errs []error
	seen := map[string]bool{}
	for _, rel := range dirRels {
		dest := h.ToDest(rel)
		if seen[dest] {
			continue
		}
		seen[dest] = true
		entries, derr := os.ReadDir(dest)
		if derr != nil {
			continue // [why] dir may not exist on host yet
		}
		for _, e := range entries {
			p := filepath.Join(dest, e.Name())
			if !h.brokenRepoLink(p) {
				continue
			}
			if err := h.fs.Remove(p); err != nil {
				errs = append(errs, h.failItem("prune-links", p, err))
			}
		}
	}
	return errors.Join(errs...)
}

// brokenRepoLink: check if p is a symlink into root/ whose target is gone.
func (h Host) brokenRepoLink(p string) bool {
	target, err := os.Readlink(p) // [what] non-symlink -> err
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(p), target)
	}
	target = filepath.Clean(target)
	if target != h.Root && !strings.HasPrefix(target, h.Root+"/") {
		return false
	}
	_, err = os.Stat(p) // [what] broken
	return err != nil
}

// [<] 🤖🤖
