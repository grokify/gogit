package scanner

// GitBackend provides git operations for repository scanning.
type GitBackend interface {
	// IsRepo checks if the path is a git repository.
	IsRepo(path string) bool
	// GetStatus returns uncommitted changes and unpushed commits status.
	// err is non-nil when the status could not be determined (e.g. the git
	// invocation failed); callers must not treat a non-nil err as "clean".
	GetStatus(repoPath string, checkUnpushed bool) (hasUncommitted, hasUnpushed bool, err error)
}

// DefaultGitBackend returns the default git backend (git CLI).
func DefaultGitBackend() GitBackend {
	return NewCLIGitBackend()
}
