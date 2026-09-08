package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/grokify/gogit"
	"github.com/grokify/gogit/internal/cliutil"
	"github.com/grokify/gogit/internal/render"
	"github.com/grokify/mogo/fmt/progress"
	"github.com/spf13/cobra"
)

var (
	pendingSinceCommit string
	pendingFormat      string
	pendingTZ          string
	pendingDepth       int
)

var pendingCmd = &cobra.Command{
	Use:   "pending [path...]",
	Short: "List commits that exist locally but haven't been pushed",
	Long: `Report commits that are ahead of their upstream (not yet pushed) across one
or more repositories.

Each path may be a git repository (reported directly) or a directory whose
git repositories are discovered and each reported in turn. Pass several paths
— e.g. one per GitHub org you manage — to sweep them together:

  gitscan pending ~/go/src/github.com/{myorg,myuser}

The output shape is the same whether one repository or many match: a single
repo is just a fleet of one. In a multi-repo sweep, repositories with nothing
pending are omitted from the table/markdown views (the summary still counts
them); JSON always reports a repos array plus a summary.

By default pending lists commits reachable from HEAD but not from each branch's
push target — its configured upstream, or the matching remote-tracking branch
(e.g. origin/main). A branch that was never pushed has no such target, so all
of its commits are reported as pending. Use --since-commit to list commits
after a specific commit instead; that applies to a single repository only.

With no path, the current directory is used. Timestamps are shown in RFC 3339
with an explicit UTC offset; --tz local or --tz utc converts every timestamp
to one consistent zone.

Examples:
  gitscan pending                                # Unpushed commits in the current repo
  gitscan pending ~/go/src/github.com/me/repo    # ...in another repo
  gitscan pending ~/go/src/github.com/{me,myorg} # Sweep several orgs at once
  gitscan pending --since-commit abc1234         # Commits after abc1234 (single repo)
  gitscan pending --format markdown              # Copy-pasteable markdown table
  gitscan pending --format json                  # Machine-readable output for agents
  gitscan pending --tz utc                       # Normalize all timestamps to UTC`,
	Args: cobra.ArbitraryArgs,
	RunE: runPending,
}

func init() {
	pendingCmd.Flags().StringVar(&pendingSinceCommit, "since-commit", "", "List commits after this commit hash instead of unpushed commits (single repo only)")
	pendingCmd.Flags().StringVarP(&pendingFormat, "format", "f", "table", "Output format: table (aligned for terminals), markdown (copy-pasteable), or json")
	pendingCmd.Flags().StringVar(&pendingTZ, "tz", "original", "Timestamp timezone: original (as recorded by git), local, or utc")
	pendingCmd.Flags().IntVar(&pendingDepth, "depth", 1, "How many directory levels below each path to search for repositories")
	rootCmd.AddCommand(pendingCmd)
}

func runPending(cmd *cobra.Command, args []string) error {
	repos, err := discoverRepos(args, pendingDepth)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return render.Commits(os.Stdout, pendingFormat, nil)
	}
	if pendingSinceCommit != "" && len(repos) > 1 {
		return fmt.Errorf("--since-commit applies to a single repository, but %d were found under the given path(s)", len(repos))
	}

	var progressFn gogit.ProgressFunc
	if len(repos) > 1 {
		// Progress goes to stderr so stdout stays clean for JSON and piping.
		renderer := progress.NewSingleStageRenderer(os.Stderr).WithBarWidth(progressBarWidth)
		progressFn = func(done, total int, path string) {
			renderer.Update(done, total, filepath.Base(path))
		}
		defer renderer.Done("")
	}

	results := gogit.RunAllWithProgress(context.Background(), repos,
		func(ctx context.Context, r *gogit.Repo) (gogit.PendingResult, error) {
			return r.PendingCommits(ctx, pendingSinceCommit)
		}, 0, progressFn)

	reports := make([]render.CommitReport, len(results))
	for i, res := range results {
		report := render.CommitReport{Repo: res.Path}
		if res.Err != nil {
			report.Err = res.Err.Error()
			reports[i] = report
			continue
		}
		commits, err := render.ApplyTimezone(res.Value.Commits, pendingTZ)
		if err != nil {
			return err
		}
		report.Commits = commits
		report.Ref = res.Value.Baseline
		report.Mode = pendingMode(pendingSinceCommit, res.Value.Baseline)
		reports[i] = report
	}

	return render.Commits(os.Stdout, pendingFormat, reports)
}

// pendingMode maps a pending query to its render mode.
func pendingMode(sinceCommit, baseline string) string {
	switch {
	case sinceCommit != "":
		return "since-commit"
	case baseline == "":
		return "unpushed-all"
	default:
		return "unpushed"
	}
}

// discoverRepos resolves each path (defaulting to the current directory) and
// returns the git repositories found under them, sorted by path. A path that
// is itself a repository is included directly.
func discoverRepos(paths []string, depth int) ([]string, error) {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	roots := make([]string, 0, len(paths))
	for _, p := range paths {
		abs, err := cliutil.ResolvePath(p)
		if err != nil {
			return nil, err
		}
		roots = append(roots, abs)
	}
	repos, err := gogit.Discover(roots, depth)
	if err != nil {
		return nil, err
	}
	sort.Strings(repos)
	return repos, nil
}
