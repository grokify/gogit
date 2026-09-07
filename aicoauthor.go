package gogit

import (
	"regexp"
	"strings"
)

// AICoAuthor represents an AI coding assistant identified from a co-author trailer.
type AICoAuthor struct {
	Signature Signature `json:"signature"`
	Tool      string    `json:"tool"`               // e.g., "Claude Code", "GitHub Copilot", "Gemini CLI"
	Provider  string    `json:"provider,omitempty"` // canonical provider slug, e.g. "anthropic"
	Model     string    `json:"model,omitempty"`    // e.g., "Sonnet 4", "Opus 4", "gemini-2.5-pro"
}

// AIToolPattern defines how to recognize an AI tool from co-author email and
// extract model version from the name.
type AIToolPattern struct {
	Name         string   // Display name, e.g., "Claude Code"
	Provider     string   // Canonical provider slug, e.g. "anthropic"
	Emails       []string // Known email addresses (lowercase)
	ModelPattern *regexp.Regexp
}

// DefaultAITools is the canonical, built-in registry of AI coding assistants
// and their co-author signatures. Consumers should use MatchAITool or
// Commit.AICoAuthors rather than maintaining separate lists.
var DefaultAITools = []AIToolPattern{
	{
		Name:     "Claude Code",
		Provider: "anthropic",
		Emails:   []string{"noreply@anthropic.com"},
		// "Claude Sonnet 4" → "Sonnet 4", "Claude Opus 4.5" → "Opus 4.5".
		// Requires a trailing version-number component to distinguish
		// model names from the bare tool name ("Claude Code").
		ModelPattern: regexp.MustCompile(`(?i)^Claude\s+(\S+\s+[\d][\d.]*)$`),
	},
	{
		Name:     "GitHub Copilot",
		Provider: "github",
		Emails:   []string{"noreply@github.com", "copilot@github.com"},
		// Copilot doesn't typically include model in name
		ModelPattern: nil,
	},
	{
		Name:     "Gemini CLI",
		Provider: "google",
		Emails: []string{
			"218195315+gemini-cli@users.noreply.github.com",
			"176961590+gemini-code-assist[bot]@users.noreply.github.com",
			"gemini-cli-agent@google.com",
			"gemini@google.com",
		},
		// "gemini-cli gemini-2.5-pro" → "gemini-2.5-pro"
		ModelPattern: regexp.MustCompile(`(?i)^gemini[-_]?cli\s+(.+)$`),
	},
	{
		Name:     "Cursor",
		Provider: "cursor",
		Emails:   []string{"ai@cursor.sh", "cursor@cursor.sh"},
		// "Cursor claude-3.5-sonnet" → "claude-3.5-sonnet"
		ModelPattern: regexp.MustCompile(`(?i)^Cursor\s+(.+)$`),
	},
	{
		Name:         "Aider",
		Provider:     "aider",
		Emails:       []string{"aider@aider.chat"},
		ModelPattern: nil,
	},
}

// MatchAITool checks if the email matches a known AI tool and extracts the model
// from the name if a pattern is defined.
func MatchAITool(sig Signature, tools []AIToolPattern) *AICoAuthor {
	email := strings.ToLower(sig.Email)

	for _, tool := range tools {
		if !matchEmail(email, tool.Emails) {
			continue
		}

		ai := &AICoAuthor{
			Signature: sig,
			Tool:      tool.Name,
			Provider:  tool.Provider,
		}

		if tool.ModelPattern != nil {
			if m := tool.ModelPattern.FindStringSubmatch(sig.Name); len(m) > 1 {
				ai.Model = strings.TrimSpace(m[1])
			}
		}

		return ai
	}

	return nil
}

// matchEmail checks if email matches any pattern in the list.
// Handles GitHub noreply suffix matching (e.g., "+gemini-cli@users.noreply.github.com").
func matchEmail(email string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.ToLower(pattern)

		if email == pattern {
			return true
		}

		// Suffix match for GitHub noreply with variable user ID prefix
		if strings.Contains(pattern, "+") && strings.HasSuffix(pattern, "@users.noreply.github.com") {
			plusIdx := strings.Index(pattern, "+")
			suffix := pattern[plusIdx:]
			if strings.HasSuffix(email, suffix) {
				return true
			}
		}
	}
	return false
}

// AICoAuthors returns AI coding assistants identified from co-author trailers.
// Uses DefaultAITools for recognition.
func (c Commit) AICoAuthors() []AICoAuthor {
	return c.AICoAuthorsWithTools(DefaultAITools)
}

// AICoAuthorsWithTools returns AI coding assistants using a custom tool registry.
func (c Commit) AICoAuthorsWithTools(tools []AIToolPattern) []AICoAuthor {
	var result []AICoAuthor
	for _, coAuthor := range c.CoAuthors() {
		if ai := MatchAITool(coAuthor, tools); ai != nil {
			result = append(result, *ai)
		}
	}
	return result
}

// IsAIAssisted returns true if any co-author is a recognized AI tool.
func (c Commit) IsAIAssisted() bool {
	return len(c.AICoAuthors()) > 0
}
