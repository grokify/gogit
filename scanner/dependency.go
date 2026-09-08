package scanner

import "strings"

// MatchesDependency reports whether the repo depends on modulePath.
//
// If directOnly is set, only requirements the repo's go.mod lists without
// a "// indirect" comment count. If prefix is set, modulePath is matched
// as a path prefix (e.g. to match any major version of a module, such as
// "github.com/google/go-github" matching ".../go-github/v88"). When
// GoModFiles is populated (recurse mode), nested go.mod files are checked
// too.
func (r RepoResult) MatchesDependency(modulePath string, directOnly, prefix bool) bool {
	deps := r.Dependencies
	if directOnly {
		deps = r.DirectDependencies
	}
	if dependencyListMatches(deps, modulePath, prefix) {
		return true
	}
	for _, gm := range r.GoModFiles {
		nested := gm.Dependencies
		if directOnly {
			nested = gm.DirectDependencies
		}
		if dependencyListMatches(nested, modulePath, prefix) {
			return true
		}
	}
	return false
}

// dependencyListMatches reports whether modulePath appears in deps,
// honoring prefix matching (a trailing "/" boundary prevents
// ".../go-github2" from falsely matching prefix ".../go-github").
func dependencyListMatches(deps []string, modulePath string, prefix bool) bool {
	for _, dep := range deps {
		if prefix {
			if dep == modulePath || strings.HasPrefix(dep, modulePath+"/") {
				return true
			}
		} else if dep == modulePath {
			return true
		}
	}
	return false
}
