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
// Supported units: h (hours), d (days), w (weeks), m (months, 30 days).
func parseDuration(s string) (time.Duration, error) {
	// Try standard Go duration first (e.g., "24h", "1h30m")
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Parse custom formats: 7d, 2w, 1m
	re := regexp.MustCompile(`^(\d+)([dwm])$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format")
	}

	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case "m":
		return time.Duration(value) * 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}
