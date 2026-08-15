package fsutil

// [>] 🤖🤖

import (
	"bytes"
	"strings"
)

func IsDirSettled(reader FileSystemReader, dest string) bool {
	info, err := reader.StatPath(dest)
	return err == nil && info.IsDir()
}

func IsLinkSettled(reader FileSystemReader, src, dest string) bool {
	destResolved, err := reader.EvaluateSymlinks(dest)
	if err != nil {
		return false
	}
	srcResolved, err := reader.EvaluateSymlinks(src)
	return err == nil && destResolved == srcResolved
}

func IsSameContent(reader FileSystemReader, src, dest string) bool {
	srcData, err := reader.ReadFileBytes(src)
	if err != nil {
		return false
	}
	destData, err := reader.ReadFileBytes(dest)
	if err != nil {
		return false
	}
	return bytes.Equal(srcData, destData)
}

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
