package fsutil

// [>] 🤖🤖

import (
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

// ParseMode parses an octal chmod string ("" -> not set).
func ParseMode(s string) (os.FileMode, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, false
	}
	return os.FileMode(n), true
}

// PermsDrift reports whether dest's live mode/owner differ from the spec chmod/
// owner. Only set fields are enforced (empty chmod/owner -> no drift). Missing
// dest -> no drift (the create path handles it).
func PermsDrift(reader FileSystemReader, dest, chmod, owner string) (needChmod, needChown bool) {
	fi, err := reader.Lstat(dest)
	if err != nil {
		return false, false
	}
	if mode, ok := ParseMode(chmod); ok {
		mask := modeMask(mode)
		needChmod = mode&mask != unixMode(fi.Mode())&mask
	}
	if owner != "" {
		needChown = IsOwnerDrifted(fi, owner)
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
// it compares equal to a ParseMode octal like 0o2775. Perm bits pass through.
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

// IsOwnerDrifted reports whether fi's live uid/gid differ from the "owner[:group]"
// spec. Unresolvable spec names or a missing Stat_t -> no drift (can't compare).
func IsOwnerDrifted(fi os.FileInfo, owner string) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	name, group, _ := strings.Cut(owner, ":")
	uid, uidOK := lookupID(name, UserLookup, func(u *user.User) string { return u.Uid })
	gid, gidOK := lookupID(group, GroupLookup, func(g *user.Group) string { return g.Gid })
	return (uidOK && uid != st.Uid) || (gidOK && gid != st.Gid)
}

// lookupID resolves name to a numeric id: empty or unresolvable -> ok=false (no drift).
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

// [<] 🤖🤖
