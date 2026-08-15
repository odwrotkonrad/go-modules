package fsutil

// [>] 🤖🤖

import (
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

func ParseMode(s string) (os.FileMode, bool) {
	if s == "" {
		return 0, false
	}
	parsed, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, false
	}
	return os.FileMode(parsed), true
}

func DetectPermsDrift(reader FileSystemReader, dest, chmod, owner string) (needChmod, needChown bool) {
	info, err := reader.LstatPath(dest)
	if err != nil {
		return false, false
	}
	if mode, ok := ParseMode(chmod); ok {
		mask := maskMode(mode)
		needChmod = mode&mask != toUnixMode(info.Mode())&mask
	}
	if owner != "" {
		needChown = IsOwnerDrifted(info, owner)
	}
	return needChmod, needChown
}

func maskMode(mode os.FileMode) os.FileMode {
	if mode > 0o777 {
		return 0o7777
	}
	return 0o777
}

func toUnixMode(mode os.FileMode) os.FileMode {
	unix := mode.Perm()
	if mode&os.ModeSetuid != 0 {
		unix |= 0o4000
	}
	if mode&os.ModeSetgid != 0 {
		unix |= 0o2000
	}
	if mode&os.ModeSticky != 0 {
		unix |= 0o1000
	}
	return unix
}

func IsOwnerDrifted(info os.FileInfo, owner string) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	name, group, _ := strings.Cut(owner, ":")
	uid, uidOK := lookupID(name, UserLookup, func(u *user.User) string { return u.Uid })
	gid, gidOK := lookupID(group, GroupLookup, func(g *user.Group) string { return g.Gid })
	return (uidOK && uid != stat.Uid) || (gidOK && gid != stat.Gid)
}

func lookupID[T any](name string, lookup func(string) (T, error), idOf func(T) string) (uint32, bool) {
	if name == "" {
		return 0, false
	}
	record, err := lookup(name)
	if err != nil {
		return 0, false
	}
	id, err := strconv.ParseUint(idOf(record), 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(id), true
}

// [<] 🤖🤖
