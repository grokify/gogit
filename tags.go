package gogit

import (
	"context"
	"strings"
	"time"
)

// Tags returns the repository's tag names, or an empty slice when there are
// none.
func (r *Repo) Tags(ctx context.Context) ([]string, error) {
	out, err := r.git(ctx, "tag", "--list")
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}

// TagsWithDates returns tag names mapped to their creation dates (the tag
// object date for annotated tags, the commit date for lightweight tags).
func (r *Repo) TagsWithDates(ctx context.Context) (map[string]time.Time, error) {
	out, err := r.git(ctx, "for-each-ref", "--format=%(refname:short)\t%(creatordate:iso-strict)", "refs/tags")
	if err != nil {
		return nil, err
	}
	tags := map[string]time.Time{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		name, dateStr, found := strings.Cut(line, "\t")
		if !found {
			continue
		}
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(dateStr)); err == nil {
			tags[name] = t
		}
	}
	return tags, nil
}
