package gogit

// AIModel holds a parsed AI model identity from a co-author trailer.
type AIModel struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Name     string `json:"name"`
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
// were used, and which human co-authors were present. AI tools and models
// are recognized via the canonical registry in DefaultAITools.
//
// Examples:
//
//	"Claude Sonnet 5 <noreply@anthropic.com>"  → Tools: ["Claude Code"], Models: [{Provider: "anthropic", Model: "Sonnet 5"}]
//	"Claude Code <noreply@anthropic.com>"      → Tools: ["Claude Code"], Models: nil (no version in name)
//	"github-actions[bot] <noreply@github.com>" → Tools: ["GitHub Copilot"], Models: nil
func AnalyzeAuthorship(c Commit) AIAttribution {
	var attr AIAttribution
	seen := make(map[string]bool)

	for _, coAuthor := range c.CoAuthors() {
		ai := MatchAITool(coAuthor, DefaultAITools)
		if ai == nil {
			attr.HumanAuthors = append(attr.HumanAuthors, coAuthor)
			continue
		}
		if !seen[ai.Tool] {
			attr.Tools = append(attr.Tools, ai.Tool)
			seen[ai.Tool] = true
		}
		if ai.Model != "" {
			attr.Models = append(attr.Models, AIModel{
				Provider: ai.Provider,
				Model:    ai.Model,
				Name:     coAuthor.Name,
			})
		}
	}

	attr.IsAIAuthored = len(attr.Tools) > 0
	return attr
}
