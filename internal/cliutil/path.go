// Package cliutil provides small CLI-input-normalization helpers shared
// across gitscan's subcommands. It is not part of the gogit library's
// public API.
package cliutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolvePath expands a leading ~ to the user's home directory, resolves
// the result to an absolute path, and validates that it exists and is a
// directory.
func ResolvePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("directory path required")
	}

	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("error getting home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("error resolving path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("error accessing directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", absPath)
	}

	return absPath, nil
}
