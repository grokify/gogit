package gogit

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Log field and record separators: ASCII unit separator and record
// separator, which cannot appear in git log format-field output.
const (
	fieldSep  = "\x1f"
	recordSep = "\x1e"
)

// logFormat begins each record with recordSep and terminates the field list
// with a trailing fieldSep, so multi-line trailers and any following
// numstat block are unambiguously delimited: a record is
// RS f0 FS f1 ... f8 FS <numstat lines until the next RS>.
const logFormat = recordSep + "%H" + fieldSep + "%an" + fieldSep + "%ae" + fieldSep + "%aI" + fieldSep +
	"%cn" + fieldSep + "%ce" + fieldSep + "%cI" + fieldSep + "%s" + fieldSep +
	"%(trailers:only,unfold)" + fieldSep

// logFieldCount is the number of fieldSep-delimited parts per record: nine
// fields plus the trailing chunk after the terminating separator.
const logFieldCount = 10

// LogOptions filters a commit-log query. Zero values leave a filter unset.
type LogOptions struct {
	// Since and Until bound the commit date (half-open in practice: git
	// treats both bounds inclusively at second resolution).
	Since time.Time
	Until time.Time
	// Author filters by author name or email (git regex semantics).
	Author string
	// NoMerges excludes merge commits.
	NoMerges bool
	// MaxCount caps the number of commits returned (0 = unlimited).
	MaxCount int
	// IncludeStats adds per-commit insertions/deletions/files-changed via
	// --numstat. Costs proportionally more; leave false when not needed.
	IncludeStats bool
}

// Signature is an author or committer identity.
type Signature struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Trailer is one commit-message trailer line, e.g.
// "Co-authored-by: Jane <jane@example.com>".
type Trailer struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Commit is one parsed log entry. Bodies are not extracted — subjects and
// trailers cover attribution and classification needs; consumers that need
// full messages can run git directly.
type Commit struct {
	Hash         string    `json:"hash"`
	Author       Signature `json:"author"`
	AuthorDate   time.Time `json:"authorDate"`
	Committer    Signature `json:"committer"`
	CommitDate   time.Time `json:"commitDate"`
	Subject      string    `json:"subject"`
	Trailers     []Trailer `json:"trailers,omitempty"`
	Insertions   int       `json:"insertions"`
	Deletions    int       `json:"deletions"`
	FilesChanged int       `json:"filesChanged"`
}

// CoAuthors returns identities from Co-authored-by trailers.
func (c Commit) CoAuthors() []Signature {
	var coAuthors []Signature
	for _, t := range c.Trailers {
		if !strings.EqualFold(t.Key, "Co-authored-by") {
			continue
		}
		if sig, ok := parseSignature(t.Value); ok {
			coAuthors = append(coAuthors, sig)
		}
	}
	return coAuthors
}

// parseSignature splits "Name <email>" into a Signature.
func parseSignature(s string) (Signature, bool) {
	open := strings.LastIndex(s, "<")
	close_ := strings.LastIndex(s, ">")
	if open < 0 || close_ < open {
		return Signature{}, false
	}
	return Signature{
		Name:  strings.TrimSpace(s[:open]),
		Email: strings.TrimSpace(s[open+1 : close_]),
	}, true
}

// Log returns commits matching opts, newest first (git log order).
func (r *Repo) Log(ctx context.Context, opts LogOptions) ([]Commit, error) {
	args := []string{"log", "--format=" + logFormat}
	if !opts.Since.IsZero() {
		args = append(args, "--since="+opts.Since.UTC().Format(time.RFC3339))
	}
	if !opts.Until.IsZero() {
		args = append(args, "--until="+opts.Until.UTC().Format(time.RFC3339))
	}
	if opts.Author != "" {
		args = append(args, "--author="+opts.Author)
	}
	if opts.NoMerges {
		args = append(args, "--no-merges")
	}
	if opts.MaxCount > 0 {
		args = append(args, fmt.Sprintf("--max-count=%d", opts.MaxCount))
	}
	if opts.IncludeStats {
		args = append(args, "--numstat")
	}

	out, err := r.git(ctx, args...)
	if err != nil {
		// An empty repository (no HEAD yet) has no commits.
		if strings.Contains(err.Error(), "does not have any commits") ||
			strings.Contains(err.Error(), "unknown revision") {
			return nil, nil
		}
		return nil, err
	}
	return parseLog(out)
}

// parseLog splits records on the leading recordSep; within each record the
// terminating fieldSep cleanly separates the nine fields from any numstat
// block that follows.
func parseLog(out string) ([]Commit, error) {
	var commits []Commit
	for i, record := range strings.Split(out, recordSep) {
		if i == 0 {
			continue // content before the first record marker (empty)
		}
		parts := strings.SplitN(record, fieldSep, logFieldCount)
		if len(parts) != logFieldCount {
			return nil, fmt.Errorf("gogit: malformed log record %d: %d fields", i, len(parts))
		}

		authorDate, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[3]))
		if err != nil {
			return nil, fmt.Errorf("gogit: parse author date %q: %w", parts[3], err)
		}
		commitDate, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[6]))
		if err != nil {
			return nil, fmt.Errorf("gogit: parse commit date %q: %w", parts[6], err)
		}

		commit := Commit{
			Hash:       strings.TrimSpace(parts[0]),
			Author:     Signature{Name: parts[1], Email: parts[2]},
			AuthorDate: authorDate,
			Committer:  Signature{Name: parts[4], Email: parts[5]},
			CommitDate: commitDate,
			Subject:    parts[7],
			Trailers:   parseTrailers(parts[8]),
		}
		applyStats(&commit, parts[9])
		commits = append(commits, commit)
	}
	return commits, nil
}

// parseTrailers converts unfolded trailer lines into key/value pairs.
func parseTrailers(block string) []Trailer {
	var trailers []Trailer
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		trailers = append(trailers, Trailer{
			Key:   strings.TrimSpace(key),
			Value: strings.TrimSpace(value),
		})
	}
	return trailers
}

// applyStats parses a numstat block ("insertions<TAB>deletions<TAB>path"
// per line; "-" for binary files) onto a commit.
func applyStats(c *Commit, block string) {
	for _, line := range strings.Split(block, "\n") {
		parts := strings.Split(strings.TrimSpace(line), "\t")
		if len(parts) < 3 {
			continue
		}
		c.FilesChanged++
		if ins, err := strconv.Atoi(parts[0]); err == nil {
			c.Insertions += ins
		}
		if del, err := strconv.Atoi(parts[1]); err == nil {
			c.Deletions += del
		}
	}
}
