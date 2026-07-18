package scanner

// GitBackend provides git operations for repository scanning.
type GitBackend interface {
	// IsRepo checks if the path is a git repository.
	IsRepo(path string) bool
	// GetStatus returns uncommitted changes and unpushed commits status.
	GetStatus(repoPath string, checkUnpushed bool) (hasUncommitted, hasUnpushed bool)
}

// DefaultGitBackend returns the default git backend (git CLI).
func DefaultGitBackend() GitBackend {
	return NewCLIGitBackend()
}
