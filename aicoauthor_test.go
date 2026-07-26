package gogit

import (
	"testing"
)

func TestMatchAITool(t *testing.T) {
	tests := []struct {
		name      string
		sig       Signature
		wantTool  string
		wantModel string
	}{
		{
			name:      "Claude Sonnet 4",
			sig:       Signature{Name: "Claude Sonnet 4", Email: "noreply@anthropic.com"},
			wantTool:  "Claude Code",
			wantModel: "Sonnet 4",
		},
		{
			name:      "Claude Opus 4.5",
			sig:       Signature{Name: "Claude Opus 4.5", Email: "noreply@anthropic.com"},
			wantTool:  "Claude Code",
			wantModel: "Opus 4.5",
		},
		{
			name:      "Claude Code legacy format",
			sig:       Signature{Name: "Claude Code", Email: "noreply@anthropic.com"},
			wantTool:  "Claude Code",
			wantModel: "Code",
		},
		{
			name:      "GitHub Copilot",
			sig:       Signature{Name: "GitHub Copilot", Email: "noreply@github.com"},
			wantTool:  "GitHub Copilot",
			wantModel: "",
		},
		{
			name:      "Gemini CLI with model",
			sig:       Signature{Name: "gemini-cli gemini-2.5-pro", Email: "218195315+gemini-cli@users.noreply.github.com"},
			wantTool:  "Gemini CLI",
			wantModel: "gemini-2.5-pro",
		},
		{
			name:      "Cursor with model",
			sig:       Signature{Name: "Cursor claude-3.5-sonnet", Email: "ai@cursor.sh"},
			wantTool:  "Cursor",
			wantModel: "claude-3.5-sonnet",
		},
		{
			name:      "Aider",
			sig:       Signature{Name: "Aider", Email: "aider@aider.chat"},
			wantTool:  "Aider",
			wantModel: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ai := MatchAITool(tt.sig, DefaultAITools)
			if ai == nil {
				t.Fatal("expected AI match, got nil")
			}
			if ai.Tool != tt.wantTool {
				t.Errorf("Tool: got %q, want %q", ai.Tool, tt.wantTool)
			}
			if ai.Model != tt.wantModel {
				t.Errorf("Model: got %q, want %q", ai.Model, tt.wantModel)
			}
		})
	}
}

func TestMatchAIToolNoMatch(t *testing.T) {
	sig := Signature{Name: "John Doe", Email: "john@example.com"}
	ai := MatchAITool(sig, DefaultAITools)
	if ai != nil {
		t.Errorf("expected nil for non-AI co-author, got %+v", ai)
	}
}

func TestMatchEmailSuffix(t *testing.T) {
	// GitHub noreply with different user ID prefix
	sig := Signature{Name: "gemini-cli gemini-2.0-flash", Email: "999999+gemini-cli@users.noreply.github.com"}
	ai := MatchAITool(sig, DefaultAITools)
	if ai == nil {
		t.Fatal("expected match via suffix, got nil")
	}
	if ai.Tool != "Gemini CLI" {
		t.Errorf("Tool: got %q, want %q", ai.Tool, "Gemini CLI")
	}
}

func TestCommitAICoAuthors(t *testing.T) {
	c := Commit{
		Hash:    "abc123",
		Subject: "feat: add feature",
		Trailers: []Trailer{
			{Key: "Co-authored-by", Value: "Claude Sonnet 4 <noreply@anthropic.com>"},
			{Key: "Reviewed-by", Value: "Jane Doe <jane@example.com>"},
		},
	}

	ais := c.AICoAuthors()
	if len(ais) != 1 {
		t.Fatalf("expected 1 AI co-author, got %d", len(ais))
	}
	if ais[0].Tool != "Claude Code" {
		t.Errorf("Tool: got %q, want %q", ais[0].Tool, "Claude Code")
	}
	if ais[0].Model != "Sonnet 4" {
		t.Errorf("Model: got %q, want %q", ais[0].Model, "Sonnet 4")
	}
}

func TestCommitIsAIAssisted(t *testing.T) {
	aiCommit := Commit{
		Trailers: []Trailer{
			{Key: "Co-authored-by", Value: "Claude Opus 4 <noreply@anthropic.com>"},
		},
	}
	if !aiCommit.IsAIAssisted() {
		t.Error("expected AI-assisted commit to return true")
	}

	humanCommit := Commit{
		Trailers: []Trailer{
			{Key: "Co-authored-by", Value: "Jane Doe <jane@example.com>"},
		},
	}
	if humanCommit.IsAIAssisted() {
		t.Error("expected human-only commit to return false")
	}
}

func TestCommitAICoAuthorsMultiple(t *testing.T) {
	c := Commit{
		Trailers: []Trailer{
			{Key: "Co-authored-by", Value: "Claude Sonnet 4 <noreply@anthropic.com>"},
			{Key: "Co-authored-by", Value: "GitHub Copilot <noreply@github.com>"},
		},
	}

	ais := c.AICoAuthors()
	if len(ais) != 2 {
		t.Fatalf("expected 2 AI co-authors, got %d", len(ais))
	}

	tools := map[string]bool{}
	for _, ai := range ais {
		tools[ai.Tool] = true
	}
	if !tools["Claude Code"] {
		t.Error("expected Claude Code in results")
	}
	if !tools["GitHub Copilot"] {
		t.Error("expected GitHub Copilot in results")
	}
}
