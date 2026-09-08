package cmd

import (
	"context"
	"os"

	"github.com/grokify/gogit"
	"github.com/grokify/gogit/internal/cliutil"
	"github.com/grokify/gogit/internal/render"
	"github.com/spf13/cobra"
)

var (
	pendingSinceCommit string
	pendingFormat      string
	pendingTZ          string
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

Timestamps are shown in RFC 3339 with an explicit UTC offset. By default
each commit keeps its own recorded timezone (as committed); --tz local or
--tz utc converts every timestamp to one consistent zone instead.

Examples:
  gitscan pending                                # Unpushed commits, aligned for terminal reading
  gitscan pending ~/go/src/github.com/me/repo    # Unpushed commits in another repo
  gitscan pending --since-commit abc1234         # Commits after abc1234, regardless of upstream
  gitscan pending --format markdown              # Copy-pasteable markdown table
  gitscan pending --format json                  # Machine-readable output for agents
  gitscan pending --tz utc                       # Normalize all timestamps to UTC
  gitscan pending --tz local                     # Convert all timestamps to this machine's local time`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPending,
}

func init() {
	pendingCmd.Flags().StringVar(&pendingSinceCommit, "since-commit", "", "List commits after this commit hash instead of unpushed commits")
	pendingCmd.Flags().StringVarP(&pendingFormat, "format", "f", "table", "Output format: table (aligned for terminals), markdown (copy-pasteable), or json")
	pendingCmd.Flags().StringVar(&pendingTZ, "tz", "original", "Timestamp timezone: original (as recorded by git), local, or utc")
	rootCmd.AddCommand(pendingCmd)
}

func runPending(cmd *cobra.Command, args []string) error {
	absPath, err := cliutil.ResolvePath(dirArg(args, 0))
	if err != nil {
		return err
	}

	repo, err := gogit.Open(absPath)
	if err != nil {
		return err
	}

	res, err := repo.PendingCommits(context.Background(), pendingSinceCommit)
	if err != nil {
		return err
	}

	commits, err := render.ApplyTimezone(res.Commits, pendingTZ)
	if err != nil {
		return err
	}

	mode := "unpushed"
	switch {
	case pendingSinceCommit != "":
		mode = "since-commit"
	case res.Baseline == "":
		// No upstream or remote-tracking branch: every local commit is pending.
		mode = "unpushed-all"
	}

	return render.Pending(os.Stdout, pendingFormat, render.PendingReport{
		Repo:    absPath,
		Mode:    mode,
		Ref:     res.Baseline,
		Commits: commits,
	})
}
