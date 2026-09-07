package scanner

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/grokify/gogit"
)

// CLIGitBackend implements GitBackend using git CLI commands.
type CLIGitBackend struct{}

// NewCLIGitBackend creates a new CLI git backend.
func NewCLIGitBackend() *CLIGitBackend {
	return &CLIGitBackend{}
}

// IsRepo checks if the path is a git repository. Delegates to the root
// gogit library, which also recognizes worktree gitfiles.
func (c *CLIGitBackend) IsRepo(path string) bool {
	return gogit.IsRepo(path)
}

// GetStatus uses `git status --porcelain -b` to check both uncommitted changes and unpushed commits.
// Output format:
//   - First line: ## branch...upstream [ahead N, behind M]
//   - Remaining lines: file status (if any uncommitted changes)
//
// A non-nil err means status could not be determined; callers must not
// treat that as "clean" (see GitBackend.GetStatus).
func (c *CLIGitBackend) GetStatus(repoPath string, checkUnpushed bool) (hasUncommitted, hasUnpushed bool, err error) {
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain", "-b")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, runErr := cmd.Output()
	if runErr != nil {
		return false, false, fmt.Errorf("git status in %s: %w: %s", repoPath, runErr, strings.TrimSpace(stderr.String()))
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) == 0 {
		return false, false, nil
	}

	// First line is branch info: ## main...origin/main [ahead 1]
	branchLine := lines[0]

	// Check for uncommitted changes (any non-empty lines after the first)
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) != "" {
			hasUncommitted = true
			break
		}
	}

	// Check for unpushed commits if requested
	if checkUnpushed {
		switch {
		case strings.Contains(branchLine, "[ahead"):
			hasUnpushed = true
		case strings.HasPrefix(branchLine, "## HEAD ("):
			// Detached HEAD (e.g. "## HEAD (no branch)"): there is no
			// branch to push, so this is never "unpushed".
		case !strings.Contains(branchLine, "..."):
			// A real branch with no upstream configured; treat as unpushed.
			hasUnpushed = true
		}
	}

	return hasUncommitted, hasUnpushed, nil
}
