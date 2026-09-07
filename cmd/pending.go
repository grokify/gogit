package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/grokify/gogit"
	"github.com/spf13/cobra"
)

var (
	pendingSinceCommit string
	pendingFormat      string
)

var pendingCmd = &cobra.Command{
	Use:   "pending [directory]",
	Short: "List commits that exist locally but haven't been pushed",
	Long: `Report commits in a single git repository that are ahead of its
upstream (not yet pushed), or that come after an explicit commit hash.

By default, pending lists commits reachable from HEAD but not from the
current branch's upstream ("@{upstream}..HEAD"). Use --since-commit to list
commits after a specific commit instead; that mode does not require an
upstream to be configured.

directory defaults to the current directory.

Examples:
  gitscan pending                                # Unpushed commits in the current directory
  gitscan pending ~/go/src/github.com/me/repo    # Unpushed commits in another repo
  gitscan pending --since-commit abc1234         # Commits after abc1234, regardless of upstream
  gitscan pending --format json                  # Machine-readable output for agents`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPending,
}

func init() {
	pendingCmd.Flags().StringVar(&pendingSinceCommit, "since-commit", "", "List commits after this commit hash instead of unpushed commits")
	pendingCmd.Flags().StringVarP(&pendingFormat, "format", "f", "table", "Output format: table or json")
	rootCmd.AddCommand(pendingCmd)
}

// pendingCommitJSON is the JSON representation of one pending commit.
type pendingCommitJSON struct {
	Hash      string `json:"hash"`
	Date      string `json:"date"`      // 2006-01-02, committer date
	Time      string `json:"time"`      // 15:04:05, committer date
	Timestamp string `json:"timestamp"` // RFC3339 committer date, original offset preserved
	Message   string `json:"message"`
}

// pendingReportJSON is the JSON envelope for the pending-commits report.
type pendingReportJSON struct {
	Repo    string              `json:"repo"`
	Mode    string              `json:"mode"` // "unpushed" or "since-commit"
	Ref     string              `json:"ref"`  // "@{upstream}" or the given hash
	Count   int                 `json:"count"`
	Commits []pendingCommitJSON `json:"commits"`
}

func runPending(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	if pendingFormat != "table" && pendingFormat != "json" {
		return fmt.Errorf("invalid format %q, must be 'table' or 'json'", pendingFormat)
	}

	absPath, err := resolvePath(dir)
	if err != nil {
		return err
	}

	repo, err := gogit.Open(absPath)
	if err != nil {
		return err
	}

	commits, err := repo.PendingCommits(context.Background(), pendingSinceCommit)
	if err != nil {
		return err
	}

	mode, ref := "unpushed", "@{upstream}"
	if pendingSinceCommit != "" {
		mode, ref = "since-commit", pendingSinceCommit
	}

	if pendingFormat == "json" {
		return printPendingJSON(absPath, mode, ref, commits)
	}
	printPendingTable(absPath, mode, ref, commits)
	return nil
}

func printPendingJSON(repoPath, mode, ref string, commits []gogit.Commit) error {
	report := pendingReportJSON{
		Repo:    repoPath,
		Mode:    mode,
		Ref:     ref,
		Count:   len(commits),
		Commits: make([]pendingCommitJSON, 0, len(commits)),
	}
	for _, c := range commits {
		report.Commits = append(report.Commits, pendingCommitJSON{
			Hash:      c.Hash,
			Date:      c.CommitDate.Format("2006-01-02"),
			Time:      c.CommitDate.Format("15:04:05"),
			Timestamp: c.CommitDate.Format("2006-01-02T15:04:05Z07:00"),
			Message:   c.Subject,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func printPendingTable(repoPath, mode, ref string, commits []gogit.Commit) {
	fmt.Printf("Repo: %s\n", repoPath)
	if mode == "unpushed" {
		fmt.Printf("Pending commits (not yet pushed to %s): %d\n\n", ref, len(commits))
	} else {
		fmt.Printf("Commits after %s: %d\n\n", ref, len(commits))
	}

	if len(commits) == 0 {
		return
	}

	fmt.Println("| # | Hash | Date | Time | Message |")
	fmt.Println("|---|------|------|------|---------|")
	for i, c := range commits {
		hash := c.Hash
		if len(hash) > 7 {
			hash = hash[:7]
		}
		fmt.Printf("| %d | %s | %s | %s | %s |\n",
			i+1, hash, c.CommitDate.Format("2006-01-02"), c.CommitDate.Format("15:04:05"),
			escapeTableCell(c.Subject))
	}
}

// escapeTableCell escapes characters that would otherwise break a markdown
// table cell.
func escapeTableCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
