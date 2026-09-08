package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/grokify/gogit/scanner"
)

func TestOrder(t *testing.T) {
	all := []scanner.RepoResult{
		{Name: "mogo", ModuleName: "github.com/grokify/mogo"},
		{
			Name:          "gogithub",
			ModuleName:    "github.com/grokify/gogithub",
			Dependencies:  []string{"github.com/grokify/mogo"},
			LatestModTime: time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC),
		},
	}

	var buf bytes.Buffer
	Order(&buf, all, all)
	out := buf.String()

	if !strings.Contains(out, "1.") || !strings.Contains(out, "mogo") {
		t.Errorf("expected mogo listed first: %s", out)
	}
	if !strings.Contains(out, "gogithub") || !strings.Contains(out, "(depends on: mogo)") {
		t.Errorf("expected gogithub with dependency annotation: %s", out)
	}
	if !strings.Contains(out, "2026-09-07 12:00") {
		t.Errorf("expected formatted mod time: %s", out)
	}
	if !strings.Contains(out, "Total: 2 repos in dependency order") {
		t.Errorf("expected total count: %s", out)
	}
}

func TestOrderEmpty(t *testing.T) {
	var buf bytes.Buffer
	Order(&buf, nil, nil)
	if !strings.Contains(buf.String(), "Total: 0 repos in dependency order") {
		t.Errorf("expected zero total: %s", buf.String())
	}
}
