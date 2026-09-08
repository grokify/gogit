package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/grokify/gogit/scanner"
)

func depFixture() []scanner.RepoResult {
	return []scanner.RepoResult{
		{
			Name:               "gogithub",
			ModuleName:         "github.com/grokify/gogithub",
			Dependencies:       []string{"github.com/grokify/mogo"},
			DirectDependencies: []string{"github.com/grokify/mogo"},
		},
		{
			Name:         "transitive-only",
			ModuleName:   "github.com/example/transitive-only",
			Dependencies: []string{"github.com/grokify/mogo"},
		},
		{
			Name:       "unrelated",
			ModuleName: "github.com/example/unrelated",
		},
	}
}

func TestDepBasic(t *testing.T) {
	var buf bytes.Buffer
	Dep(&buf, depFixture(), DepOptions{ModulePath: "github.com/grokify/mogo"})
	out := buf.String()

	if !strings.Contains(out, "gogithub") || !strings.Contains(out, "transitive-only") {
		t.Errorf("expected both dependents listed: %s", out)
	}
	if strings.Contains(out, "unrelated") {
		t.Errorf("unrelated repo should not be listed: %s", out)
	}
	if !strings.Contains(out, "Summary: 3 repos scanned, 2 depend on github.com/grokify/mogo") {
		t.Errorf("unexpected summary: %s", out)
	}
}

func TestDepDirectOnly(t *testing.T) {
	var buf bytes.Buffer
	Dep(&buf, depFixture(), DepOptions{ModulePath: "github.com/grokify/mogo", DirectOnly: true})
	out := buf.String()

	if !strings.Contains(out, "gogithub") {
		t.Errorf("expected direct dependent listed: %s", out)
	}
	if strings.Contains(out, "transitive-only") {
		t.Errorf("transitive-only dependent should be excluded by --direct-only: %s", out)
	}
}

func TestDepPrefix(t *testing.T) {
	results := []scanner.RepoResult{
		{Name: "uses-v88", ModuleName: "m1", Dependencies: []string{"github.com/google/go-github/v88"}},
	}
	var buf bytes.Buffer
	Dep(&buf, results, DepOptions{ModulePath: "github.com/google/go-github", Prefix: true})
	if !strings.Contains(buf.String(), "uses-v88") {
		t.Errorf("expected prefix match to find versioned dependency: %s", buf.String())
	}
}

func TestDepRecurseAnnotation(t *testing.T) {
	results := []scanner.RepoResult{
		{
			Name:         "monorepo",
			ModuleName:   "m1",
			Dependencies: []string{"github.com/grokify/mogo"},
			GoModFiles: []scanner.GoModResult{
				{Path: "sub/go.mod"},
			},
		},
	}
	var buf bytes.Buffer
	Dep(&buf, results, DepOptions{ModulePath: "github.com/grokify/mogo", Recurse: true})
	if !strings.Contains(buf.String(), "+ 1 nested") {
		t.Errorf("expected nested go.mod annotation: %s", buf.String())
	}
}
