package cmd

import "github.com/grokify/gogit/scanner"

// Common flag variables shared across subcommands
var (
	dirPath        string
	recurse        bool
	checkWorkflows bool
	refRepo        string
)

// createGitBackend returns the git backend (git CLI).
func createGitBackend() scanner.GitBackend {
	return scanner.NewCLIGitBackend()
}
