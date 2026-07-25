package gogit

import "testing"

func TestParseConventionalCommit(t *testing.T) {
	tests := []struct {
		subject string
		want    *ConventionalCommit
	}{
		{
			"feat(store): add unit-of-work",
			&ConventionalCommit{Type: "feat", Scope: "store", Subject: "add unit-of-work"},
		},
		{
			"fix: resolve race condition",
			&ConventionalCommit{Type: "fix", Subject: "resolve race condition"},
		},
		{
			"refactor(api)!: rename endpoints",
			&ConventionalCommit{Type: "refactor", Scope: "api", Breaking: true, Subject: "rename endpoints"},
		},
		{"not a conventional commit", nil},
		{"", nil},
	}
	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			got := ParseConventionalCommit(tt.subject)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil")
			}
			if got.Type != tt.want.Type || got.Scope != tt.want.Scope || got.Breaking != tt.want.Breaking || got.Subject != tt.want.Subject {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCommitTrailerValue(t *testing.T) {
	c := Commit{
		Trailers: []Trailer{
			{Key: "Refs", Value: "RMI-FOO-001"},
			{Key: "Co-authored-by", Value: "Claude Sonnet 5 <noreply@anthropic.com>"},
			{Key: "Co-authored-by", Value: "Alice <alice@example.com>"},
		},
	}

	if got := c.TrailerValue("Refs"); got != "RMI-FOO-001" {
		t.Fatalf("TrailerValue(Refs) = %q, want %q", got, "RMI-FOO-001")
	}
	if got := c.TrailerValue("refs"); got != "RMI-FOO-001" {
		t.Fatalf("TrailerValue(refs) case-insensitive = %q, want %q", got, "RMI-FOO-001")
	}
	if got := c.TrailerValue("Nonexistent"); got != "" {
		t.Fatalf("TrailerValue(Nonexistent) = %q, want empty", got)
	}

	coAuthors := c.TrailerValues("Co-authored-by")
	if len(coAuthors) != 2 {
		t.Fatalf("TrailerValues(Co-authored-by) = %v, want 2 entries", coAuthors)
	}
}

func TestCommitParseConventional(t *testing.T) {
	c := Commit{Subject: "feat(auth): add OAuth2 login"}
	cc := c.ParseConventional()
	if cc == nil {
		t.Fatal("expected non-nil")
	}
	if cc.Type != "feat" || cc.Scope != "auth" {
		t.Fatalf("got %+v", cc)
	}

	c2 := Commit{Subject: "not conventional"}
	if c2.ParseConventional() != nil {
		t.Fatal("expected nil for non-conventional subject")
	}
}
