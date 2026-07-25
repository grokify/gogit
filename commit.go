package gogit

import (
	"regexp"
	"strings"
)

var conventionalCommitRE = regexp.MustCompile(`^(\w+)(?:\(([^)]*)\))?(!)?:\s*(.+)`)

// ConventionalCommit holds parsed conventional commit components.
type ConventionalCommit struct {
	Type     string `json:"type"`
	Scope    string `json:"scope,omitempty"`
	Breaking bool   `json:"breaking"`
	Subject  string `json:"subject"`
}

// ParseConventionalCommit parses the subject line of a conventional commit.
// Returns nil if the subject does not match the pattern.
func ParseConventionalCommit(subject string) *ConventionalCommit {
	m := conventionalCommitRE.FindStringSubmatch(subject)
	if m == nil {
		return nil
	}
	return &ConventionalCommit{
		Type:     m[1],
		Scope:    m[2],
		Breaking: m[3] == "!",
		Subject:  strings.TrimSpace(m[4]),
	}
}

// TrailerValue returns the first trailer value matching the given key
// (case-insensitive), or "" if not found.
func (c Commit) TrailerValue(key string) string {
	for _, t := range c.Trailers {
		if strings.EqualFold(t.Key, key) {
			return t.Value
		}
	}
	return ""
}

// TrailerValues returns all trailer values matching the given key
// (case-insensitive).
func (c Commit) TrailerValues(key string) []string {
	var vals []string
	for _, t := range c.Trailers {
		if strings.EqualFold(t.Key, key) {
			vals = append(vals, t.Value)
		}
	}
	return vals
}

// ParseConventional parses this commit's subject as a conventional commit.
// Returns nil if the subject does not match.
func (c Commit) ParseConventional() *ConventionalCommit {
	return ParseConventionalCommit(c.Subject)
}
