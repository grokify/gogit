package gogit

import "strings"

// NormalizeRemoteURL converts a git remote URL to a canonical host/path
// repository identifier:
//
//	https://github.com/x/y.git  → github.com/x/y
//	git@github.com:x/y.git      → github.com/x/y
//	ssh://git@github.com/x/y    → github.com/x/y
//
// An empty input returns "".
func NormalizeRemoteURL(remote string) string {
	repo := strings.TrimSpace(remote)
	if repo == "" {
		return ""
	}
	repo = strings.TrimSuffix(repo, ".git")
	for _, prefix := range []string{"https://", "http://", "ssh://", "git://"} {
		repo = strings.TrimPrefix(repo, prefix)
	}
	if at := strings.Index(repo, "@"); at >= 0 {
		repo = repo[at+1:]
		repo = strings.Replace(repo, ":", "/", 1)
	}
	return repo
}
