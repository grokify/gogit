package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/grokify/gogit/gitgrep"
	"github.com/grokify/gogit/internal/cliutil"
	"github.com/spf13/cobra"
)

var (
	grepPatterns   []string
	grepPaths      []string
	grepIgnoreCase bool
	grepRegex      bool
	grepStaged     bool
	grepRev        string
	grepHistory    bool
	grepJSON       bool
)

var grepCmd = &cobra.Command{
	Use:   "grep -e <pattern> [-e <pattern>...] [directory]",
	Short: "Search a repository's content or history for terms",
	Long: `Search a single git repository for one or more patterns.

By default the working tree is searched (tracked files). Use --staged to search
the index, --rev to search a revision, or --history to find commits whose diff
introduced or removed a pattern (git pickaxe).

The -i/--ignore-case and -E/--regex flags apply to all patterns.

directory defaults to the current directory and must be a git repository.

Examples:
  gitscan grep -e ACME -e "Acme Corp" -i ./       # working tree, case-insensitive
  gitscan grep -e ExampleCo --staged              # index (pre-commit surface)
  gitscan grep -e "SECRET-[0-9]+" -E              # extended regex
  gitscan grep -e "Acme Corp" --history           # which commit introduced it
  gitscan grep -e ACME --json                     # machine-readable output`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGrep,
}

func init() {
	grepCmd.Flags().StringArrayVarP(&grepPatterns, "pattern", "e", nil, "Search pattern (repeatable)")
	grepCmd.Flags().StringArrayVar(&grepPaths, "path", nil, "Limit to path (repeatable)")
	grepCmd.Flags().BoolVarP(&grepIgnoreCase, "ignore-case", "i", false, "Case-insensitive matching")
	grepCmd.Flags().BoolVarP(&grepRegex, "regex", "E", false, "Treat patterns as extended regexes")
	grepCmd.Flags().BoolVar(&grepStaged, "staged", false, "Search the index instead of the working tree")
	grepCmd.Flags().StringVar(&grepRev, "rev", "", "Search a specific revision instead of the working tree")
	grepCmd.Flags().BoolVar(&grepHistory, "history", false, "Search history via pickaxe (commits that changed a pattern)")
	grepCmd.Flags().BoolVar(&grepJSON, "json", false, "Output JSON")
	rootCmd.AddCommand(grepCmd)
}

func runGrep(cmd *cobra.Command, args []string) error {
	if len(grepPatterns) == 0 {
		return fmt.Errorf("at least one -e/--pattern is required")
	}
	absPath, err := cliutil.ResolvePath(dirArg(args, 0))
	if err != nil {
		return err
	}

	patterns := make([]gitgrep.Pattern, 0, len(grepPatterns))
	for _, v := range grepPatterns {
		patterns = append(patterns, gitgrep.Pattern{
			Value:      v,
			Regex:      grepRegex,
			IgnoreCase: grepIgnoreCase,
		})
	}
	opts := gitgrep.Options{
		Patterns:   patterns,
		Pathspecs:  grepPaths,
		SkipBinary: true,
		Rev:        grepRev,
		Staged:     grepStaged,
	}

	ctx := cmd.Context()
	if grepHistory {
		hits, err := gitgrep.HistoryPickaxe(ctx, absPath, opts)
		if err != nil {
			return err
		}
		return renderHistory(hits)
	}

	matches, err := gitgrep.GrepTree(ctx, absPath, opts)
	if err != nil {
		return err
	}
	return renderMatches(matches)
}

func renderMatches(matches []gitgrep.Match) error {
	if grepJSON {
		return json.NewEncoder(os.Stdout).Encode(matches)
	}
	for _, m := range matches {
		if m.Rev != "" {
			fmt.Printf("%s:%s:%d:%s\n", m.Rev, m.Path, m.Line, m.Text)
		} else {
			fmt.Printf("%s:%d:%s\n", m.Path, m.Line, m.Text)
		}
	}
	return nil
}

func renderHistory(hits []gitgrep.HistoryMatch) error {
	if grepJSON {
		return json.NewEncoder(os.Stdout).Encode(hits)
	}
	for _, h := range hits {
		fmt.Printf("%s %s %s: %s\n", shortSHA(h.Commit), h.Date, h.Path, h.Pattern)
	}
	return nil
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
