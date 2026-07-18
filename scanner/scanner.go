package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// GoModResult holds analysis results for a single go.mod file.
type GoModResult struct {
	Path         string   // Path to go.mod relative to repo root
	ModuleName   string   // Module name from go.mod
	Dependencies []string // Required module paths
	ReplaceCount int      // Number of replace directives
}

// RepoResult holds the analysis results for a single repository.
type RepoResult struct {
	Name                  string
	Path                  string
	IsGitRepo             bool
	HasGoMod              bool
	HasUncommittedChanges bool
	HasUnpushedCommits    bool
	HasReplaceDirectives  bool
	HasModuleMismatch     bool
	ModuleName            string
	ReplaceCount          int
	Dependencies          []string           // Dependencies from root go.mod
	GoModFiles            []GoModResult      // All go.mod files (when recurse=true)
	LatestModTime         time.Time          // Most recent file modification time
	WorkflowCompliance    WorkflowCompliance // Workflow compliance status (when workflow check enabled)
}

// HasDependency checks if the repo depends on the given module path.
// When GoModFiles is populated (recurse mode), checks all go.mod files.
func (r RepoResult) HasDependency(modulePath string) bool {
	// Check root dependencies
	if slices.Contains(r.Dependencies, modulePath) {
		return true
	}
	// Check nested go.mod files
	for _, gm := range r.GoModFiles {
		if slices.Contains(gm.Dependencies, modulePath) {
			return true
		}
	}
	return false
}

// ModifiedSince returns true if the repo has files modified within the given duration.
func (r RepoResult) ModifiedSince(d time.Duration) bool {
	if r.LatestModTime.IsZero() {
		return false
	}
	cutoff := time.Now().Add(-d)
	return r.LatestModTime.After(cutoff)
}

// NeedsPush returns true if the repo has uncommitted changes or unpushed commits.
func (r RepoResult) NeedsPush() bool {
	return r.HasUncommittedChanges || r.HasUnpushedCommits
}

// ProgressFunc is called during scanning with current progress.
type ProgressFunc func(current, total int, name string)

// ScanOptions configures the scanning behavior.
type ScanOptions struct {
	Recurse       bool                 // Search for nested go.mod files
	CheckModTime  bool                 // Compute latest modification time (expensive)
	CheckUnpushed bool                 // Check for unpushed commits
	Workers       int                  // Number of parallel workers (0 = GOMAXPROCS)
	GitBackend    GitBackend           // Git backend to use (nil = default go-git backend)
	Workflow      WorkflowCheckOptions // Workflow compliance checking options
}

// CountDirectories counts the number of scannable directories.
func CountDirectories(dirPath string) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		count++
	}
	return count, nil
}

// ScanDirectory scans all direct subdirectories in the given path.
func ScanDirectory(dirPath string) ([]RepoResult, error) {
	return ScanDirectoryWithProgress(dirPath, nil, ScanOptions{})
}

// ScanDirectoryWithProgress scans directories and reports progress via callback.
func ScanDirectoryWithProgress(dirPath string, progressFn ProgressFunc, opts ScanOptions) ([]RepoResult, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	// First pass: count directories
	var dirs []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dirs = append(dirs, entry)
	}

	total := len(dirs)

	// Determine number of workers
	numWorkers := opts.Workers
	if numWorkers <= 0 {
		numWorkers = runtime.GOMAXPROCS(0)
	}
	// Don't use more workers than directories
	if numWorkers > total {
		numWorkers = total
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	// Create work channel and results channel
	type workItem struct {
		index int
		entry os.DirEntry
	}
	type resultItem struct {
		index  int
		result RepoResult
	}

	workCh := make(chan workItem, total)
	resultCh := make(chan resultItem, total)

	// Start workers
	var wg sync.WaitGroup
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range workCh {
				subPath := filepath.Join(dirPath, work.entry.Name())
				result := analyzeRepo(subPath, work.entry.Name(), opts)
				resultCh <- resultItem{index: work.index, result: result}
			}
		}()
	}

	// Send work
	go func() {
		for i, entry := range dirs {
			workCh <- workItem{index: i, entry: entry}
		}
		close(workCh)
	}()

	// Collect results and report progress
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make([]RepoResult, total)
	completed := 0
	for item := range resultCh {
		results[item.index] = item.result
		completed++
		if progressFn != nil {
			progressFn(completed, total, item.result.Name)
		}
	}

	return results, nil
}

func analyzeRepo(repoPath, name string, opts ScanOptions) RepoResult {
	result := RepoResult{
		Name: name,
		Path: repoPath,
	}

	// Get latest modification time (only if requested - expensive operation)
	if opts.CheckModTime {
		result.LatestModTime = getLatestModTime(repoPath)
	}

	// Get git backend (default to go-git)
	backend := opts.GitBackend
	if backend == nil {
		backend = DefaultGitBackend()
	}

	// Check if it's a git repository
	result.IsGitRepo = backend.IsRepo(repoPath)

	// Check git status (uncommitted changes and optionally unpushed commits)
	if result.IsGitRepo {
		result.HasUncommittedChanges, result.HasUnpushedCommits = backend.GetStatus(repoPath, opts.CheckUnpushed)
	}

	// Analyze go.mod at root
	goModPath := filepath.Join(repoPath, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		result.HasGoMod = true
		moduleName, replaceCount, dependencies := analyzeGoMod(goModPath)
		result.ModuleName = moduleName
		result.ReplaceCount = replaceCount
		result.HasReplaceDirectives = replaceCount > 0
		result.Dependencies = dependencies

		// Check if module name matches directory structure
		if moduleName != "" {
			result.HasModuleMismatch = !moduleMatchesPath(moduleName, name)
		}
	}

	// Find nested go.mod files if recurse is enabled
	if opts.Recurse {
		goModFiles := findGoModFiles(repoPath)
		for _, goModFile := range goModFiles {
			relPath, _ := filepath.Rel(repoPath, goModFile)
			moduleName, replaceCount, dependencies := analyzeGoMod(goModFile)
			result.GoModFiles = append(result.GoModFiles, GoModResult{
				Path:         relPath,
				ModuleName:   moduleName,
				Dependencies: dependencies,
				ReplaceCount: replaceCount,
			})
		}
	}

	// Check workflow compliance if enabled
	if opts.Workflow.Enabled {
		result.WorkflowCompliance = CheckWorkflowCompliance(repoPath, opts.Workflow)
	}

	return result
}

// findGoModFiles recursively finds all go.mod files in the given directory.
// Skips vendor directories and hidden directories.
func findGoModFiles(rootPath string) []string {
	var goModFiles []string

	_ = filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip directories we can't read
		}

		// Skip hidden directories and vendor
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Collect go.mod files (excluding the root one)
		if d.Name() == "go.mod" && path != filepath.Join(rootPath, "go.mod") {
			goModFiles = append(goModFiles, path)
		}

		return nil
	})

	return goModFiles
}

func analyzeGoMod(goModPath string) (moduleName string, replaceCount int, dependencies []string) {
	file, err := os.Open(goModPath)
	if err != nil {
		return "", 0, nil
	}
	defer func() {
		_ = file.Close()
	}()

	s := bufio.NewScanner(file)
	inReplaceBlock := false
	inRequireBlock := false

	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		// Get module name
		if mod, found := strings.CutPrefix(line, "module "); found {
			moduleName = strings.TrimSpace(mod)
		}

		// Count replace directives
		if strings.HasPrefix(line, "replace ") && !strings.HasPrefix(line, "replace (") {
			replaceCount++
		}

		// Handle replace block
		if strings.HasPrefix(line, "replace (") {
			inReplaceBlock = true
			continue
		}
		if inReplaceBlock {
			if line == ")" {
				inReplaceBlock = false
				continue
			}
			if line != "" && !strings.HasPrefix(line, "//") {
				replaceCount++
			}
		}

		// Parse single-line require
		if strings.HasPrefix(line, "require ") && !strings.HasPrefix(line, "require (") {
			if dep := parseRequireLine(strings.TrimPrefix(line, "require ")); dep != "" {
				dependencies = append(dependencies, dep)
			}
		}

		// Handle require block
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}
		if inRequireBlock {
			if line == ")" {
				inRequireBlock = false
				continue
			}
			if dep := parseRequireLine(line); dep != "" {
				dependencies = append(dependencies, dep)
			}
		}
	}

	return moduleName, replaceCount, dependencies
}

// parseRequireLine extracts the module path from a require line.
// Input: "github.com/foo/bar v1.2.3" or "github.com/foo/bar v1.2.3 // indirect"
// Output: "github.com/foo/bar"
func parseRequireLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "//") {
		return ""
	}
	// Split on whitespace, first part is module path
	parts := strings.Fields(line)
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

// moduleMatchesPath checks if the module name ends with the directory name.
// For example: github.com/grokify/gogit should match directory "gitscan"
func moduleMatchesPath(moduleName, dirName string) bool {
	// Get the last segment of the module path
	parts := strings.Split(moduleName, "/")
	if len(parts) == 0 {
		return false
	}
	lastPart := parts[len(parts)-1]
	return lastPart == dirName
}

// GetInternalDeps returns dependencies that are also in the results set (managed modules).
// It maps from directory name to module name for matching.
func GetInternalDeps(result RepoResult, allResults []RepoResult) []string {
	// Build map of module names to directory names
	moduleToDir := make(map[string]string)
	for _, r := range allResults {
		if r.ModuleName != "" {
			moduleToDir[r.ModuleName] = r.Name
		}
	}

	// Find which dependencies are internal
	var internal []string
	for _, dep := range result.Dependencies {
		if dirName, ok := moduleToDir[dep]; ok {
			internal = append(internal, dirName)
		}
	}
	return internal
}

// GetTransitiveDependents returns all repos that transitively depend on the given seed repos.
// This finds repos that may need updating when seed repos are updated.
func GetTransitiveDependents(seeds []RepoResult, allResults []RepoResult) []RepoResult {
	// Build module name to result map
	moduleToResult := make(map[string]*RepoResult)
	for i := range allResults {
		if allResults[i].ModuleName != "" {
			moduleToResult[allResults[i].ModuleName] = &allResults[i]
		}
	}

	// Build reverse dependency graph: module -> modules that depend on it
	dependents := make(map[string][]string)
	for _, r := range allResults {
		if r.ModuleName == "" {
			continue
		}
		for _, dep := range r.Dependencies {
			if _, isManaged := moduleToResult[dep]; isManaged {
				dependents[dep] = append(dependents[dep], r.ModuleName)
			}
		}
	}

	// BFS to find all transitive dependents
	seedModules := make(map[string]bool)
	for _, s := range seeds {
		if s.ModuleName != "" {
			seedModules[s.ModuleName] = true
		}
	}

	visited := make(map[string]bool)
	queue := make([]string, 0, len(seedModules))
	for mod := range seedModules {
		queue = append(queue, mod)
		visited[mod] = true
	}

	for len(queue) > 0 {
		mod := queue[0]
		queue = queue[1:]

		for _, dependent := range dependents[mod] {
			if !visited[dependent] {
				visited[dependent] = true
				queue = append(queue, dependent)
			}
		}
	}

	// Collect results for all visited modules
	var result []RepoResult
	for _, r := range allResults {
		if r.ModuleName != "" && visited[r.ModuleName] {
			result = append(result, r)
		}
	}

	return result
}

// TopologicalSort returns repos in dependency order (dependencies before dependents).
// Uses Kahn's algorithm. Returns sorted results and any cycles detected.
func TopologicalSort(results []RepoResult) ([]RepoResult, []string) {
	// Build map of module names to results
	moduleToResult := make(map[string]*RepoResult)
	dirToResult := make(map[string]*RepoResult)
	for i := range results {
		if results[i].ModuleName != "" {
			moduleToResult[results[i].ModuleName] = &results[i]
		}
		dirToResult[results[i].Name] = &results[i]
	}

	// Build adjacency list and in-degree count
	// Edge A -> B means A depends on B (B must be updated before A)
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // module -> modules that depend on it

	for _, r := range results {
		if r.ModuleName == "" {
			continue
		}
		inDegree[r.ModuleName] = 0
	}

	for _, r := range results {
		if r.ModuleName == "" {
			continue
		}
		for _, dep := range r.Dependencies {
			if _, isManaged := moduleToResult[dep]; isManaged {
				inDegree[r.ModuleName]++
				dependents[dep] = append(dependents[dep], r.ModuleName)
			}
		}
	}

	// Kahn's algorithm
	var queue []string
	for mod, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, mod)
		}
	}
	// Sort for deterministic output
	slices.Sort(queue)

	var sorted []RepoResult
	for len(queue) > 0 {
		mod := queue[0]
		queue = queue[1:]

		if r, ok := moduleToResult[mod]; ok {
			sorted = append(sorted, *r)
		}

		// Decrease in-degree for dependents
		deps := dependents[mod]
		slices.Sort(deps)
		for _, dep := range deps {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	// Check for cycles
	var cycles []string
	for mod, deg := range inDegree {
		if deg > 0 {
			cycles = append(cycles, mod)
		}
	}

	return sorted, cycles
}

// getLatestModTime walks the directory tree and returns the most recent modification time.
// Skips .git, vendor, and node_modules directories for performance.
func getLatestModTime(rootPath string) time.Time {
	var latest time.Time

	_ = filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		// Skip .git, vendor, and node_modules for performance
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Get file info for modification time
		info, err := d.Info()
		if err != nil {
			return nil
		}

		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}

		return nil
	})

	return latest
}
