package scanner

import "testing"

func TestRepoResultMatchesDependency(t *testing.T) {
	r := RepoResult{
		Dependencies:       []string{"github.com/grokify/mogo", "github.com/google/go-github/v88"},
		DirectDependencies: []string{"github.com/grokify/mogo"},
	}

	tests := []struct {
		name       string
		modulePath string
		directOnly bool
		prefix     bool
		want       bool
	}{
		{"direct dep, not direct-only", "github.com/grokify/mogo", false, false, true},
		{"direct dep, direct-only", "github.com/grokify/mogo", true, false, true},
		{"indirect dep, not direct-only", "github.com/google/go-github/v88", false, false, true},
		{"indirect dep, direct-only excludes it", "github.com/google/go-github/v88", true, false, false},
		{"exact match without prefix fails on versioned path", "github.com/google/go-github", false, false, false},
		{"prefix match finds versioned path", "github.com/google/go-github", false, true, true},
		{"prefix match doesn't false-positive on similar name", "github.com/google/go-git", false, true, false},
		{"no match", "github.com/example/other", false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.MatchesDependency(tt.modulePath, tt.directOnly, tt.prefix)
			if got != tt.want {
				t.Errorf("MatchesDependency(%q, directOnly=%v, prefix=%v) = %v, want %v",
					tt.modulePath, tt.directOnly, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestRepoResultMatchesDependencyNested(t *testing.T) {
	r := RepoResult{
		GoModFiles: []GoModResult{
			{
				Dependencies:       []string{"github.com/grokify/mogo"},
				DirectDependencies: []string{"github.com/grokify/mogo"},
			},
		},
	}

	if !r.MatchesDependency("github.com/grokify/mogo", false, false) {
		t.Error("expected nested go.mod dependency to match")
	}
	if r.MatchesDependency("github.com/example/other", false, false) {
		t.Error("expected no match for a module not present anywhere")
	}
}
