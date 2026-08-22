package render

// [>] 🤖🤖

import (
	"fmt"
	"strings"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
)

func (r remoteRef) key() string { return r.repoURL + "?" + r.gitRef }

// GitMarker prefixes every git remote source: git::<repo>[@<ref>]//<path>.
const GitMarker = "git::"

// CutGitMarker strips the git:: marker, reporting whether the source carried one.
func CutGitMarker(source string) (rest string, ok bool) {
	return strings.CutPrefix(source, GitMarker)
}

// CutRefSuffix splits a marker-less remote source into its repo-and-path and its @<ref>.
//
// [why] the ref anchors to the last '@' inside the repo path, past the authority: a scheme or scp user '@' precedes the host, so it never reads as one
func CutRefSuffix(source string) (rest, gitRef string, err error) {
	start := 0
	if i := strings.Index(source, "://"); i >= 0 {
		start = i + 3
	}
	head, tail := source, ""
	hasPath := false
	if j := strings.Index(source[start:], "//"); j >= 0 {
		head, tail, hasPath = source[:start+j], source[start+j+2:], true
	}
	at := strings.LastIndex(head, "@")
	if at < 0 || at < repoPathStart(head, start) {
		return source, "", nil
	}
	gitRef = head[at+1:]
	if gitRef == "" {
		return "", "", fmt.Errorf("source %q: bare %q, want %s<repo>@<ref>", source, "@", GitMarker)
	}
	rest = head[:at]
	if hasPath {
		rest += "//" + tail
	}
	return rest, gitRef, nil
}

// [why] the authority runs to the first '/' or ':' past any userinfo '@': the repo path starts there
func repoPathStart(head string, start int) int {
	authority := head[start:]
	sep := strings.IndexAny(authority, "/:")
	// [why] userinfo needs a host after it: a trailing '@' is a malformed ref, not a user
	if at := strings.Index(authority, "@"); at >= 0 && at < len(authority)-1 && (sep < 0 || at < sep) {
		start += at + 1
		authority = head[start:]
		sep = strings.IndexAny(authority, "/:")
	}
	if sep >= 0 {
		return start + sep
	}
	// [why] no authority/path boundary means no userinfo either: any '@' left is the ref
	return start
}

func parseRemoteRef(ref string) (remoteRef, error) {
	rest, ok := CutGitMarker(ref)
	if !ok {
		return remoteRef{}, fmt.Errorf("remote source %q: want %s<repo>[@<ref>]//<path>", ref, GitMarker)
	}
	if strings.Contains(rest, "?ref=") {
		return remoteRef{}, fmt.Errorf("remote source %q: ?ref= is gone, pin with %s<repo>@<ref>//<path>", ref, GitMarker)
	}
	rest, gitRef, err := CutRefSuffix(rest)
	if err != nil {
		return remoteRef{}, err
	}
	scheme := ""
	if i := strings.Index(rest, "://"); i >= 0 {
		scheme, rest = rest[:i+3], rest[i+3:]
	}
	repo, path, ok := strings.Cut(rest, "//")
	if !ok || repo == "" || path == "" {
		return remoteRef{}, fmt.Errorf("remote source %q: want %s<repo>[@<ref>]//<path>", ref, GitMarker)
	}
	out := remoteRef{path: path, gitRef: gitRef}
	if scheme == "" {
		out.repoURL = "https://" + repo + ".git"
		host, repoPath, _ := strings.Cut(repo, "/")
		out.sshURL = "ssh://git@" + host + "/" + repoPath + ".git"
	} else {
		out.repoURL = scheme + repo
	}
	return out, nil
}

func IsRemoteRef(ref string) bool {
	_, err := parseRemoteRef(ref)
	return err == nil
}

func NewRemoteFetcher() func(string) (string, error) {
	clones := map[string]billy.Filesystem{}
	return func(ref string) (string, error) {
		src, err := parseRemoteRef(ref)
		if err != nil {
			return "", err
		}
		fs, ok := clones[src.key()]
		if !ok {
			fs, err = cloneRemote(src)
			if err != nil {
				return "", fmt.Errorf("remoteFile %q: %w", ref, err)
			}
			clones[src.key()] = fs
		}
		content, err := util.ReadFile(fs, src.path)
		if err != nil {
			return "", fmt.Errorf("remoteFile %q: %w", ref, err)
		}
		return string(content), nil
	}
}

func cloneRemote(src remoteRef) (billy.Filesystem, error) {
	fs, err := tryClone(src.repoURL, nil, src.gitRef)
	if err == nil {
		return fs, nil
	}
	if src.sshURL != "" {
		if auth, errAuth := gitssh.NewSSHAgentAuth("git"); errAuth == nil {
			if fsSSH, errSSH := tryClone(src.sshURL, auth, src.gitRef); errSSH == nil {
				return fsSSH, nil
			}
		}
	}
	return nil, err
}

func tryClone(url string, auth transport.AuthMethod, gitRef string) (billy.Filesystem, error) {
	names := []plumbing.ReferenceName{""}
	if gitRef != "" {
		names = []plumbing.ReferenceName{
			plumbing.NewBranchReferenceName(gitRef),
			plumbing.NewTagReferenceName(gitRef),
		}
	}
	var err error
	for _, name := range names {
		fs := memfs.New()
		_, err = git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
			URL:           url,
			Auth:          auth,
			Depth:         1,
			SingleBranch:  true,
			ReferenceName: name,
			Tags:          git.NoTags,
		})
		if err == nil {
			return fs, nil
		}
	}
	return nil, err
}

// [<] 🤖🤖
