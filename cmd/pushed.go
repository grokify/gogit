package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/grokify/gogit"
	"github.com/grokify/gogit/internal/cliutil"
	"github.com/grokify/gogit/internal/render"
	"github.com/spf13/cobra"
)

const pushedDefaultCount = 10

var (
	pushedFormat string
	pushedTZ     string
)

var pushedCmd = &cobra.Command{
	Use:   "pushed [count] [directory]",
	Short: "List the most recent commits that have been pushed",
	Long: `Report the most recent commits already pushed on a single repository's
current branch — those reachable from its push target (its configured
upstream, or the matching remote-tracking branch such as origin/main), newest
first. It is the counterpart to "pending": together they show the recent
commits on either side of what has reached the remote.

count is the number of commits to show and defaults to ` + strconv.Itoa(pushedDefaultCount) + `. directory
defaults to the current directory. Either positional may be given in either
order (the numeric one is the count).

When the branch has never been pushed and has no remote-tracking branch,
nothing has been pushed and the list is empty.

Timestamps are shown in RFC 3339 with an explicit UTC offset. By default each
commit keeps its own recorded timezone (as committed); --tz local or --tz utc
converts every timestamp to one consistent zone instead.

Examples:
  gitscan pushed                                 # Last 10 pushed commits here
  gitscan pushed 25                              # Last 25 pushed commits here
  gitscan pushed 25 ~/go/src/github.com/me/repo  # ...in another repo
  gitscan pushed --format markdown               # Copy-pasteable markdown table
  gitscan pushed --format json                   # Machine-readable output for agents
  gitscan pushed --tz utc                        # Normalize all timestamps to UTC`,
	Args: cobra.MaximumNArgs(2),
	RunE: runPushed,
}

func init() {
	pushedCmd.Flags().StringVarP(&pushedFormat, "format", "f", "table", "Output format: table (aligned for terminals), markdown (copy-pasteable), or json")
	pushedCmd.Flags().StringVar(&pushedTZ, "tz", "original", "Timestamp timezone: original (as recorded by git), local, or utc")
	rootCmd.AddCommand(pushedCmd)
}

func runPushed(cmd *cobra.Command, args []string) error {
	count := pushedDefaultCount
	dir := "."
	for _, arg := range args {
		if n, err := strconv.Atoi(arg); err == nil {
			if n < 1 {
				return fmt.Errorf("count must be a positive integer, got %q", arg)
			}
			count = n
		} else {
			dir = arg
		}
	}

	absPath, err := cliutil.ResolvePath(dir)
	if err != nil {
		return err
	}

	repo, err := gogit.Open(absPath)
	if err != nil {
		return err
	}

	res, err := repo.PushedCommits(context.Background(), count)
	if err != nil {
		return err
	}

	commits, err := render.ApplyTimezone(res.Commits, pushedTZ)
	if err != nil {
		return err
	}

	return render.Commits(os.Stdout, pushedFormat, render.CommitReport{
		Repo:    absPath,
		Mode:    "pushed",
		Ref:     res.Baseline,
		Commits: commits,
	})
}
