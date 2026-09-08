package cmd

import "github.com/grokify/gogit/scanner"

// createGitBackend returns the git backend (git CLI).
func createGitBackend() scanner.GitBackend {
	return scanner.NewCLIGitBackend()
}

// dirArg returns the directory positional argument at index i, defaulting
// to the current directory when it is absent. Every subcommand treats the
// scan directory as an optional trailing positional so the CLI surface is
// uniform: no command needs a -d/--dir flag.
func dirArg(args []string, i int) string {
	if len(args) > i {
		return args[i]
	}
	return "."
}
