package fsutil

// [>] 🤖🤖

import (
	"bytes"
	"strings"
)

// IsDirSettled reports whether dest already exists as a dir and may be skipped.
// forceAll (dry-run=all) forces every dest to report, so it never skips.
func IsDirSettled(reader FileSystemReader, dest string, forceAll bool) bool {
	if forceAll {
		return false
	}
	fi, err := reader.StatPath(dest)
	return err == nil && fi.IsDir()
}

// IsLinkSettled reports whether dest already resolves to src (skippable).
// forceAll (dry-run=all) forces every dest to report, so it never skips.
func IsLinkSettled(reader FileSystemReader, src, dest string, forceAll bool) bool {
	if forceAll {
		return false
	}
	destResolved, err := reader.EvaluateSymlinks(dest)
	if err != nil {
		return false
	}
	srcResolved, err := reader.EvaluateSymlinks(src)
	return err == nil && destResolved == srcResolved
}

// IsSameContent reports whether a and b have byte-identical content (either
// unreadable -> false).
func IsSameContent(reader FileSystemReader, a, b string) bool {
	x, err := reader.ReadFileBytes(a)
	if err != nil {
		return false
	}
	y, err := reader.ReadFileBytes(b)
	if err != nil {
		return false
	}
	return bytes.Equal(x, y)
}

// PrependEnvVar sets key=value:<existing> in env copy.
func PrependEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	found := false
	for _, kv := range env {
		if rest, ok := strings.CutPrefix(kv, prefix); ok {
			out = append(out, prefix+value+":"+rest)
			found = true
		} else {
			out = append(out, kv)
		}
	}
	if !found {
		out = append(out, prefix+value)
	}
	return out
}

// [<] 🤖🤖
