package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/grokify/gogit/scanner"
)

// Common flag variables shared across subcommands
var (
	dirPath        string
	recurse        bool
	checkWorkflows bool
	refRepo        string
)

// resolvePath expands ~ and resolves to an absolute path, then validates it exists as a directory.
func resolvePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("directory path required")
	}

	// Expand ~ to home directory
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("error getting home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("error resolving path: %w", err)
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("error accessing directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", absPath)
	}

	return absPath, nil
}

// createGitBackend returns the git backend (git CLI).
func createGitBackend() scanner.GitBackend {
	return scanner.NewCLIGitBackend()
}

// parseDuration parses duration strings like "7d", "2w", "1m", "24h".
// Supported custom units: d (days), w (weeks), m (months, 30 days). Bare
// N-unit values are matched against the custom format first, since Go's
// stdlib time.ParseDuration also accepts "m" as minutes and would otherwise
// shadow the "months" unit — e.g. "1m" is ambiguous between 1 minute and
// 1 month, and callers of this CLI mean months.
func parseDuration(s string) (time.Duration, error) {
	re := regexp.MustCompile(`^(\d+)([dwm])$`)
	if matches := re.FindStringSubmatch(s); matches != nil {
		value, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, fmt.Errorf("invalid duration value %q: %w", matches[1], err)
		}
		switch matches[2] {
		case "d":
			return time.Duration(value) * 24 * time.Hour, nil
		case "w":
			return time.Duration(value) * 7 * 24 * time.Hour, nil
		case "m":
			return time.Duration(value) * 30 * 24 * time.Hour, nil
		}
	}

	// Fall back to standard Go duration syntax (e.g., "24h", "1h30m").
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	return 0, fmt.Errorf("invalid duration format")
}
