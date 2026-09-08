package scanner

import "testing"

func TestGetInternalDeps(t *testing.T) {
	all := []RepoResult{
		{Name: "mogo", ModuleName: "github.com/grokify/mogo"},
		{Name: "gogithub", ModuleName: "github.com/grokify/gogithub", Dependencies: []string{
			"github.com/grokify/mogo",
			"github.com/spf13/cobra", // external, not one of the managed repos
		}},
	}

	got := GetInternalDeps(all[1], all)
	if len(got) != 1 || got[0] != "mogo" {
		t.Errorf("GetInternalDeps = %v, want [mogo]", got)
	}
}

func TestGetInternalDepsNoModuleName(t *testing.T) {
	all := []RepoResult{
		{Name: "no-gomod", Dependencies: []string{"github.com/grokify/mogo"}},
	}
	if got := GetInternalDeps(all[0], all); got != nil {
		t.Errorf("expected nil for a dependency with no matching managed module, got %v", got)
	}
}

func TestGetTransitiveDependents(t *testing.T) {
	// base <- mid <- top (top depends on mid, mid depends on base)
	all := []RepoResult{
		{Name: "base", ModuleName: "github.com/example/base"},
		{Name: "mid", ModuleName: "github.com/example/mid", Dependencies: []string{"github.com/example/base"}},
		{Name: "top", ModuleName: "github.com/example/top", Dependencies: []string{"github.com/example/mid"}},
		{Name: "unrelated", ModuleName: "github.com/example/unrelated"},
	}

	got := GetTransitiveDependents([]RepoResult{all[0]}, all)

	names := map[string]bool{}
	for _, r := range got {
		names[r.Name] = true
	}
	if !names["base"] || !names["mid"] || !names["top"] {
		t.Errorf("expected base, mid, and top all included, got %v", names)
	}
	if names["unrelated"] {
		t.Errorf("unrelated repo should not be included: %v", names)
	}
}

func TestGetTransitiveDependentsNoDependents(t *testing.T) {
	all := []RepoResult{
		{Name: "leaf", ModuleName: "github.com/example/leaf"},
		{Name: "other", ModuleName: "github.com/example/other"},
	}
	got := GetTransitiveDependents([]RepoResult{all[0]}, all)
	if len(got) != 1 || got[0].Name != "leaf" {
		t.Errorf("expected only the seed itself, got %v", got)
	}
}

func TestTopologicalSortLinearChain(t *testing.T) {
	// c depends on b, b depends on a
	results := []RepoResult{
		{Name: "c-dir", ModuleName: "c", Dependencies: []string{"b"}},
		{Name: "a-dir", ModuleName: "a"},
		{Name: "b-dir", ModuleName: "b", Dependencies: []string{"a"}},
	}

	sorted, cycles := TopologicalSort(results)
	if len(cycles) != 0 {
		t.Fatalf("expected no cycles, got %v", cycles)
	}

	order := make([]string, len(sorted))
	for i, r := range sorted {
		order[i] = r.ModuleName
	}
	want := []string{"a", "b", "c"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order = %v, want %v", order, want)
			break
		}
	}
}

func TestTopologicalSortDiamond(t *testing.T) {
	// d depends on b and c; b and c both depend on a
	results := []RepoResult{
		{Name: "d", ModuleName: "d", Dependencies: []string{"b", "c"}},
		{Name: "b", ModuleName: "b", Dependencies: []string{"a"}},
		{Name: "c", ModuleName: "c", Dependencies: []string{"a"}},
		{Name: "a", ModuleName: "a"},
	}

	sorted, cycles := TopologicalSort(results)
	if len(cycles) != 0 {
		t.Fatalf("expected no cycles, got %v", cycles)
	}
	if len(sorted) != 4 {
		t.Fatalf("expected all 4 repos sorted, got %d", len(sorted))
	}
	// a must come before b and c; b and c must come before d.
	pos := map[string]int{}
	for i, r := range sorted {
		pos[r.ModuleName] = i
	}
	if pos["a"] > pos["b"] || pos["a"] > pos["c"] {
		t.Errorf("expected a before b and c: %v", pos)
	}
	if pos["b"] > pos["d"] || pos["c"] > pos["d"] {
		t.Errorf("expected b and c before d: %v", pos)
	}
}

func TestTopologicalSortCycle(t *testing.T) {
	// a depends on b, b depends on a: a genuine cycle.
	results := []RepoResult{
		{Name: "a-dir", ModuleName: "a", Dependencies: []string{"b"}},
		{Name: "b-dir", ModuleName: "b", Dependencies: []string{"a"}},
	}

	sorted, cycles := TopologicalSort(results)
	if len(sorted) != 0 {
		t.Errorf("expected no repos resolved from a pure 2-cycle, got %v", sorted)
	}
	if len(cycles) != 2 {
		t.Fatalf("expected both modules reported as cycles, got %v", cycles)
	}
}

func TestTopologicalSortPartialCycle(t *testing.T) {
	// independent resolves cleanly; a and b cycle with each other.
	results := []RepoResult{
		{Name: "independent", ModuleName: "independent"},
		{Name: "a-dir", ModuleName: "a", Dependencies: []string{"b"}},
		{Name: "b-dir", ModuleName: "b", Dependencies: []string{"a"}},
	}

	sorted, cycles := TopologicalSort(results)
	if len(sorted) != 1 || sorted[0].ModuleName != "independent" {
		t.Errorf("expected only 'independent' resolved, got %v", sorted)
	}
	if len(cycles) != 2 {
		t.Errorf("expected a and b reported as cycles, got %v", cycles)
	}
}

func TestTopologicalSortExternalDepsIgnored(t *testing.T) {
	// Depending on a module outside the result set must not block sorting
	// or count toward cycle detection.
	results := []RepoResult{
		{Name: "solo", ModuleName: "solo", Dependencies: []string{"github.com/spf13/cobra"}},
	}
	sorted, cycles := TopologicalSort(results)
	if len(cycles) != 0 {
		t.Errorf("expected no cycles for an external-only dependency, got %v", cycles)
	}
	if len(sorted) != 1 || sorted[0].ModuleName != "solo" {
		t.Errorf("expected solo resolved despite its external dependency, got %v", sorted)
	}
}
