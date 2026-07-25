package gogit

import "testing"

func TestAIProviderByEmail(t *testing.T) {
	tests := []struct {
		email    string
		wantName string
	}{
		{"noreply@anthropic.com", "Claude Code"},
		{"NOREPLY@ANTHROPIC.COM", "Claude Code"},
		{"noreply@github.com", "GitHub Copilot"},
		{"copilot@github.com", "GitHub Copilot"},
		{"ai@cursor.sh", "Cursor"},
		{"aider@aider.chat", "Aider"},
		{"gemini-cli-agent@google.com", "Gemini CLI"},
		{"218195315+gemini-cli@users.noreply.github.com", "Gemini CLI"},
		{"999999+gemini-cli@users.noreply.github.com", "Gemini CLI"},
		{"alice@example.com", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			got := AIToolByEmail(tt.email)
			if got != tt.wantName {
				t.Fatalf("AIToolByEmail(%q) = %q, want %q", tt.email, got, tt.wantName)
			}
		})
	}
}

func TestParseAIModel(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		wantModel string
		wantNil   bool
	}{
		{"Claude Sonnet 5", "noreply@anthropic.com", "sonnet-5", false},
		{"Claude Opus 4.6", "noreply@anthropic.com", "opus-4.6", false},
		{"Claude Haiku 4.5", "noreply@anthropic.com", "haiku-4.5", false},
		{"Claude Code", "noreply@anthropic.com", "", false},
		{"github-actions[bot]", "noreply@github.com", "", false},
		{"Alice", "alice@example.com", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAIModel(tt.name, tt.email)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil")
			}
			if got.Model != tt.wantModel {
				t.Fatalf("Model = %q, want %q", got.Model, tt.wantModel)
			}
		})
	}
}

func TestAnalyzeAuthorship(t *testing.T) {
	c := Commit{
		Author: Signature{Name: "John", Email: "john@example.com"},
		Trailers: []Trailer{
			{Key: "Co-authored-by", Value: "Claude Opus 4.6 <noreply@anthropic.com>"},
			{Key: "Co-authored-by", Value: "Alice <alice@example.com>"},
		},
	}

	attr := AnalyzeAuthorship(c)
	if !attr.IsAIAuthored {
		t.Fatal("expected AI-authored")
	}
	if len(attr.Tools) != 1 || attr.Tools[0] != "Claude Code" {
		t.Fatalf("Tools = %v, want [Claude Code]", attr.Tools)
	}
	if len(attr.Models) != 1 || attr.Models[0].Model != "opus-4.6" {
		t.Fatalf("Models = %v, want opus-4.6", attr.Models)
	}
	if len(attr.HumanAuthors) != 1 || attr.HumanAuthors[0].Name != "Alice" {
		t.Fatalf("HumanAuthors = %v, want [Alice]", attr.HumanAuthors)
	}
}

func TestAnalyzeAuthorshipNoAI(t *testing.T) {
	c := Commit{
		Author: Signature{Name: "John", Email: "john@example.com"},
		Trailers: []Trailer{
			{Key: "Co-authored-by", Value: "Alice <alice@example.com>"},
		},
	}
	attr := AnalyzeAuthorship(c)
	if attr.IsAIAuthored {
		t.Fatal("expected not AI-authored")
	}
	if len(attr.Tools) != 0 {
		t.Fatalf("Tools = %v, want empty", attr.Tools)
	}
}

func TestAnalyzeAuthorshipMultipleAI(t *testing.T) {
	c := Commit{
		Trailers: []Trailer{
			{Key: "Co-authored-by", Value: "Claude Sonnet 5 <noreply@anthropic.com>"},
			{Key: "Co-authored-by", Value: "Aider <aider@aider.chat>"},
		},
	}
	attr := AnalyzeAuthorship(c)
	if !attr.IsAIAuthored {
		t.Fatal("expected AI-authored")
	}
	if len(attr.Tools) != 2 {
		t.Fatalf("Tools = %v, want 2 entries", attr.Tools)
	}
	if len(attr.Models) != 2 {
		t.Fatalf("Models = %v, want 2 entries", attr.Models)
	}
	if attr.Models[0].Model != "sonnet-5" {
		t.Fatalf("first model = %q, want sonnet-5", attr.Models[0].Model)
	}
}
