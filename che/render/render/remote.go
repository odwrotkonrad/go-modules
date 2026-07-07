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

type remoteRef struct {
	repoURL string
	sshURL  string
	path    string
	gitRef  string
}

func (r remoteRef) key() string { return r.repoURL + "?" + r.gitRef }

func parseRemoteRef(ref string) (remoteRef, error) {
	scheme, rest := "", ref
	if i := strings.Index(ref, "://"); i >= 0 {
		scheme, rest = ref[:i+3], ref[i+3:]
	}
	repo, path, ok := strings.Cut(rest, "//")
	if !ok || repo == "" || path == "" {
		return remoteRef{}, fmt.Errorf("remoteFile %q: want <repo>//<path>[?ref=<ref>]", ref)
	}
	path, query, _ := strings.Cut(path, "?")
	var gitRef string
	if query != "" {
		v, ok := strings.CutPrefix(query, "ref=")
		if !ok || v == "" {
			return remoteRef{}, fmt.Errorf("remoteFile %q: unknown query %q, want ref=<ref>", ref, query)
		}
		gitRef = v
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

func remoteFileResolver() func(string) (string, error) {
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
