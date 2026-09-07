package gogit

import "testing"

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
	if len(attr.Models) != 1 || attr.Models[0].Model != "Opus 4.6" {
		t.Fatalf("Models = %v, want Opus 4.6", attr.Models)
	}
	if attr.Models[0].Provider != "anthropic" {
		t.Fatalf("Models[0].Provider = %q, want anthropic", attr.Models[0].Provider)
	}
	if len(attr.HumanAuthors) != 1 || attr.HumanAuthors[0].Name != "Alice" {
		t.Fatalf("HumanAuthors = %v, want [Alice]", attr.HumanAuthors)
	}
}

func TestAnalyzeAuthorshipNoModel(t *testing.T) {
	// "Claude Code" alone (no version) must not be misread as a model name.
	c := Commit{
		Trailers: []Trailer{
			{Key: "Co-authored-by", Value: "Claude Code <noreply@anthropic.com>"},
		},
	}
	attr := AnalyzeAuthorship(c)
	if !attr.IsAIAuthored {
		t.Fatal("expected AI-authored")
	}
	if len(attr.Models) != 0 {
		t.Fatalf("Models = %v, want empty", attr.Models)
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
	if len(attr.Models) != 1 {
		t.Fatalf("Models = %v, want 1 entry (Aider has no model pattern)", attr.Models)
	}
	if attr.Models[0].Model != "Sonnet 5" {
		t.Fatalf("first model = %q, want Sonnet 5", attr.Models[0].Model)
	}
}
