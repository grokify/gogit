package gogit

import "strings"

// NormalizeRemoteURL converts a git remote URL to a canonical host/path
// repository identifier:
//
//	https://github.com/x/y.git       → github.com/x/y
//	git@github.com:x/y.git           → github.com/x/y
//	ssh://git@github.com/x/y         → github.com/x/y
//	ssh://git@github.com:2222/x/y    → github.com/x/y
//
// An empty input returns "".
func NormalizeRemoteURL(remote string) string {
	repo := strings.TrimSpace(remote)
	if repo == "" {
		return ""
	}
	repo = strings.TrimSuffix(repo, ".git")

	hadScheme := false
	for _, prefix := range []string{"https://", "http://", "ssh://", "git://"} {
		if rest, ok := strings.CutPrefix(repo, prefix); ok {
			repo = rest
			hadScheme = true
			break
		}
	}

	if at := strings.Index(repo, "@"); at >= 0 {
		repo = repo[at+1:]
	}

	if hadScheme {
		// URL form: host[:port]/path. Drop an explicit port so the same
		// repo reached via different ports still normalizes to one
		// identifier, and so the port doesn't get folded into the path.
		host, rest, found := strings.Cut(repo, "/")
		if found {
			if colon := strings.Index(host, ":"); colon >= 0 {
				host = host[:colon]
			}
			repo = host + "/" + rest
		}
	} else {
		// scp-style form: host:path.
		repo = strings.Replace(repo, ":", "/", 1)
	}

	return repo
}
