package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

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
  gitscan pending                                # Unpushed commits, aligned for terminal reading
  gitscan pending ~/go/src/github.com/me/repo    # Unpushed commits in another repo
  gitscan pending --since-commit abc1234         # Commits after abc1234, regardless of upstream
  gitscan pending --format markdown              # Copy-pasteable markdown table
  gitscan pending --format json                  # Machine-readable output for agents`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPending,
}

func init() {
	pendingCmd.Flags().StringVar(&pendingSinceCommit, "since-commit", "", "List commits after this commit hash instead of unpushed commits")
	pendingCmd.Flags().StringVarP(&pendingFormat, "format", "f", "table", "Output format: table (aligned for terminals), markdown (copy-pasteable), or json")
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

	if pendingFormat != "table" && pendingFormat != "markdown" && pendingFormat != "json" {
		return fmt.Errorf("invalid format %q, must be 'table', 'markdown', or 'json'", pendingFormat)
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

	switch pendingFormat {
	case "json":
		return printPendingJSON(absPath, mode, ref, commits)
	case "markdown":
		printPendingMarkdown(absPath, mode, ref, commits)
	default: // "table"
		printPendingTable(absPath, mode, ref, commits)
	}
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

// printPendingHeader prints the report summary line shared by every
// non-JSON format.
func printPendingHeader(repoPath, mode, ref string, count int) {
	fmt.Printf("Repo: %s\n", repoPath)
	if mode == "unpushed" {
		fmt.Printf("Pending commits (not yet pushed to %s): %d\n\n", ref, count)
	} else {
		fmt.Printf("Commits after %s: %d\n\n", ref, count)
	}
}

// printPendingTable renders an aligned plain-text table via text/tabwriter,
// meant to be read directly in a terminal (the default format).
func printPendingTable(repoPath, mode, ref string, commits []gogit.Commit) {
	printPendingHeader(repoPath, mode, ref, len(commits))
	if len(commits) == 0 {
		return
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tHASH\tDATE\tTIME\tMESSAGE")
	for i, c := range commits {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			i+1, shortHash(c.Hash), c.CommitDate.Format("2006-01-02"), c.CommitDate.Format("15:04:05"),
			sanitizeTabwriterCell(c.Subject))
	}
	tw.Flush()
}

// printPendingMarkdown renders a copy-pasteable GitHub-flavored markdown
// table (e.g. for pasting into a PR description or issue).
func printPendingMarkdown(repoPath, mode, ref string, commits []gogit.Commit) {
	printPendingHeader(repoPath, mode, ref, len(commits))
	if len(commits) == 0 {
		return
	}

	fmt.Println("| # | Hash | Date | Time | Message |")
	fmt.Println("|---|------|------|------|---------|")
	for i, c := range commits {
		fmt.Printf("| %d | %s | %s | %s | %s |\n",
			i+1, shortHash(c.Hash), c.CommitDate.Format("2006-01-02"), c.CommitDate.Format("15:04:05"),
			escapeMarkdownCell(c.Subject))
	}
}

// shortHash returns the standard 7-character abbreviated form of a commit
// hash.
func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// escapeMarkdownCell escapes characters that would otherwise break a
// markdown table cell.
func escapeMarkdownCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

// sanitizeTabwriterCell strips characters that would otherwise confuse
// tabwriter's column alignment (it uses raw tab bytes as its own column
// separator).
func sanitizeTabwriterCell(s string) string {
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.ReplaceAll(s, "\n", " ")
}
