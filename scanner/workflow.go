package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkflowCompliance represents workflow compliance status for a repository.
type WorkflowCompliance struct {
	HasWorkflows      bool     `json:"hasWorkflows"`
	WorkflowFiles     []string `json:"workflowFiles"`
	UsesReusable      bool     `json:"usesReusable"`
	ReusableWorkflows []string `json:"reusableWorkflows"`
	RefRepoMatch      bool     `json:"refRepoMatch"`
	MissingWorkflows  []string `json:"missingWorkflows"`
	ComplianceLevel   string   `json:"complianceLevel"` // full, partial, none
}

// WorkflowCheckOptions configures workflow compliance checking.
type WorkflowCheckOptions struct {
	Enabled       bool
	RefRepo       string   // e.g., "plexusone/.github"
	RefBranch     string   // e.g., "main"
	RequiredTypes []string // e.g., ["go-ci", "go-lint", "go-sast-codeql"]
}

// DefaultGoWorkflowTypes returns the default required workflow types for Go projects.
func DefaultGoWorkflowTypes() []string {
	return []string{"go-ci", "go-lint", "go-sast-codeql"}
}

// CheckWorkflowCompliance checks workflow compliance for a repository.
func CheckWorkflowCompliance(repoPath string, opts WorkflowCheckOptions) WorkflowCompliance {
	result := WorkflowCompliance{
		ComplianceLevel: "none",
	}

	workflowsDir := filepath.Join(repoPath, ".github", "workflows")
	if !dirExists(workflowsDir) {
		return result
	}

	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		return result
	}

	// Find workflow files
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			result.WorkflowFiles = append(result.WorkflowFiles, name)
		}
	}

	if len(result.WorkflowFiles) == 0 {
		return result
	}

	result.HasWorkflows = true

	// Check each workflow for reusable workflow usage
	for _, wfFile := range result.WorkflowFiles {
		wfPath := filepath.Join(workflowsDir, wfFile)
		refs := parseReusableWorkflowRefs(wfPath)

		for _, ref := range refs {
			result.ReusableWorkflows = append(result.ReusableWorkflows, ref)
			result.UsesReusable = true

			// Check if it references the expected ref repo
			if opts.RefRepo != "" && strings.Contains(ref, opts.RefRepo) {
				result.RefRepoMatch = true
			}
		}
	}

	// Check for missing required workflows
	if len(opts.RequiredTypes) > 0 {
		for _, reqType := range opts.RequiredTypes {
			found := false
			expectedFilenames := getExpectedFilenames(reqType)

			for _, wfFile := range result.WorkflowFiles {
				for _, expected := range expectedFilenames {
					if wfFile == expected {
						found = true
						break
					}
				}
				if found {
					break
				}
			}

			if !found {
				result.MissingWorkflows = append(result.MissingWorkflows, reqType)
			}
		}
	}

	// Determine compliance level
	if len(result.MissingWorkflows) == 0 && result.RefRepoMatch {
		result.ComplianceLevel = "full"
	} else if result.HasWorkflows && len(result.MissingWorkflows) < len(opts.RequiredTypes) {
		result.ComplianceLevel = "partial"
	} else if result.HasWorkflows {
		result.ComplianceLevel = "partial"
	} else {
		result.ComplianceLevel = "none"
	}

	return result
}

// getExpectedFilenames returns expected filenames for a workflow type.
func getExpectedFilenames(workflowType string) []string {
	switch workflowType {
	case "go-ci":
		return []string{"go-ci.yaml", "go-ci.yml", "ci.yaml", "ci.yml"}
	case "go-lint":
		return []string{"go-lint.yaml", "go-lint.yml", "lint.yaml", "lint.yml"}
	case "go-sast-codeql":
		return []string{"go-sast-codeql.yaml", "go-sast-codeql.yml", "sast_codeql.yaml", "sast_codeql.yml", "codeql.yaml", "codeql.yml"}
	default:
		return []string{workflowType + ".yaml", workflowType + ".yml"}
	}
}

// parseReusableWorkflowRefs parses a workflow file and extracts reusable workflow references.
func parseReusableWorkflowRefs(path string) []string {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var wf workflowYAML
	if err := yaml.Unmarshal(content, &wf); err != nil {
		return nil
	}

	var refs []string
	for _, job := range wf.Jobs {
		if job.Uses != "" {
			refs = append(refs, job.Uses)
		}
	}

	return refs
}

// workflowYAML represents the structure of a GitHub Actions workflow file.
type workflowYAML struct {
	Name string                 `yaml:"name"`
	Jobs map[string]workflowJob `yaml:"jobs"`
}

type workflowJob struct {
	Uses string `yaml:"uses"`
}

// dirExists checks if a directory exists.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
