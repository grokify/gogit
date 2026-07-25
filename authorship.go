package gogit

import (
	"regexp"
	"strings"
)

// AIProvider identifies an AI coding assistant provider.
type AIProvider struct {
	// Name is the display name, e.g. "Claude Code".
	Name string
	// Provider is the canonical provider slug, e.g. "anthropic".
	Provider string
	// Emails are known co-author addresses. Entries of the form
	// "<id>+<slug>@users.noreply.github.com" also match any address with
	// the same "+<slug>@users.noreply.github.com" suffix.
	Emails []string
}

// KnownAIProviders lists recognized AI coding assistants and their co-author
// signatures. This is the canonical registry; consumers should use
// AIProviderByEmail rather than maintaining separate lists.
var KnownAIProviders = []AIProvider{
	{
		Name:     "Claude Code",
		Provider: "anthropic",
		Emails:   []string{"noreply@anthropic.com"},
	},
	{
		Name:     "GitHub Copilot",
		Provider: "github",
		Emails:   []string{"noreply@github.com", "copilot@github.com"},
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
	},
	{
		Name:     "Cursor",
		Provider: "cursor",
		Emails:   []string{"ai@cursor.sh", "cursor@cursor.sh"},
	},
	{
		Name:     "Aider",
		Provider: "aider",
		Emails:   []string{"aider@aider.chat"},
	},
}

// AIProviderByEmail returns the AI provider matching a co-author email, or
// nil when the email is not a known AI signature. Matching is case-insensitive
// and supports GitHub noreply suffix forms with variable user-ID prefixes.
func AIProviderByEmail(email string) *AIProvider {
	email = strings.ToLower(email)
	for i := range KnownAIProviders {
		p := &KnownAIProviders[i]
		for _, pattern := range p.Emails {
			pattern = strings.ToLower(pattern)
			if email == pattern {
				return p
			}
			if plus := strings.Index(pattern, "+"); plus >= 0 &&
				strings.HasSuffix(pattern, "@users.noreply.github.com") {
				if strings.HasSuffix(email, pattern[plus:]) {
					return p
				}
			}
		}
	}
	return nil
}

// AIToolByEmail returns the tool name matching a co-author email, or "" when
// the email is not a known AI signature. This is a convenience wrapper around
// AIProviderByEmail for callers that only need the name.
func AIToolByEmail(email string) string {
	if p := AIProviderByEmail(email); p != nil {
		return p.Name
	}
	return ""
}

// AIModel holds a parsed AI model identity from a co-author trailer.
type AIModel struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Name     string `json:"name"`
}

// modelFromNameRE extracts model identifiers from co-author names like
// "Claude Sonnet 5", "Claude Opus 4.6", "Claude Haiku 4.5".
// Requires a version number component to distinguish model names from
// the tool name ("Claude Code").
var modelFromNameRE = regexp.MustCompile(`(?i)^claude\s+(\w+)\s+([\d][\d.]*)\s*$`)

// ParseAIModel extracts the AI model from a co-author signature's name
// and email. Returns nil if the signature is not from a known AI provider
// or the model cannot be determined.
//
// Examples:
//
//	"Claude Sonnet 5 <noreply@anthropic.com>"  → {Provider: "anthropic", Model: "sonnet-5", Name: "Claude Sonnet 5"}
//	"Claude Opus 4.6 <noreply@anthropic.com>"  → {Provider: "anthropic", Model: "opus-4.6", Name: "Claude Opus 4.6"}
//	"github-actions[bot] <noreply@github.com>" → {Provider: "github", Model: "", Name: "github-actions[bot]"}
func ParseAIModel(name, email string) *AIModel {
	provider := AIProviderByEmail(email)
	if provider == nil {
		return nil
	}
	m := &AIModel{
		Provider: provider.Provider,
		Name:     name,
	}
	if provider.Provider == "anthropic" {
		if match := modelFromNameRE.FindStringSubmatch(name); match != nil {
			m.Model = strings.ToLower(match[1]) + "-" + match[2]
		}
	}
	return m
}

// AIAttribution holds the result of analyzing a commit for AI authorship.
type AIAttribution struct {
	IsAIAuthored bool        `json:"isAiAuthored"`
	Tools        []string    `json:"tools,omitempty"`
	Models       []AIModel   `json:"models,omitempty"`
	HumanAuthors []Signature `json:"humanAuthors,omitempty"`
}

// AnalyzeAuthorship examines a commit's co-author trailers and returns
// a complete attribution breakdown: which AI tools contributed, what models
// were used, and which human co-authors were present.
func AnalyzeAuthorship(c Commit) AIAttribution {
	var attr AIAttribution
	seen := make(map[string]bool)

	for _, coAuthor := range c.CoAuthors() {
		provider := AIProviderByEmail(coAuthor.Email)
		if provider == nil {
			attr.HumanAuthors = append(attr.HumanAuthors, coAuthor)
			continue
		}
		if !seen[provider.Name] {
			attr.Tools = append(attr.Tools, provider.Name)
			seen[provider.Name] = true
		}
		if model := ParseAIModel(coAuthor.Name, coAuthor.Email); model != nil {
			attr.Models = append(attr.Models, *model)
		}
	}

	attr.IsAIAuthored = len(attr.Tools) > 0
	return attr
}
