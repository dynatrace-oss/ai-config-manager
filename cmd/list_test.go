package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// setupTestRepo creates a test repository with sample resources
func setupTestRepo(t *testing.T) *repo.Manager {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	mgr, err := repo.NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create test resources
	testDir := t.TempDir()

	// Create command resources
	cmdFiles := map[string]string{
		"test-cmd.md": `---
description: Test command
---
# Test Command`,
		"pdf-extract.md": `---
description: Extract text from PDF
---
# PDF Extract`,
		"data-process.md": `---
description: Process data
---
# Data Process`,
	}

	for name, content := range cmdFiles {
		path := filepath.Join(testDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test command %s: %v", name, err)
		}
		if err := mgr.AddCommand(path, "file://"+path, "file"); err != nil {
			t.Fatalf("failed to add command %s: %v", name, err)
		}
	}

	// Create skill resources
	skillDirs := map[string]string{
		"test-skill": `---
description: Test skill
---
# Test Skill`,
		"pdf-processing": `---
description: Process PDF files
---
# PDF Processing`,
	}

	for name, content := range skillDirs {
		skillDir := filepath.Join(testDir, name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("failed to create skill dir %s: %v", name, err)
		}
		skillMd := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillMd, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create skill file %s: %v", name, err)
		}
		if err := mgr.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
			t.Fatalf("failed to add skill %s: %v", name, err)
		}
	}

	// Create agent resources
	agentFiles := map[string]string{
		"test-agent.md": `---
description: Test agent
---
# Test Agent`,
		"code-reviewer.md": `---
description: Reviews code quality
---
# Code Reviewer`,
	}

	for name, content := range agentFiles {
		path := filepath.Join(testDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test agent %s: %v", name, err)
		}
		if err := mgr.AddAgent(path, "file://"+path, "file"); err != nil {
			t.Fatalf("failed to add agent %s: %v", name, err)
		}
	}

	return mgr
}

// filterResources applies pattern matching to resources (mimics list command logic)
func filterResources(t *testing.T, mgr *repo.Manager, patternStr string) []resource.Resource {
	t.Helper()

	// Parse pattern
	matcher, err := pattern.NewMatcher(patternStr)
	if err != nil {
		t.Fatalf("invalid pattern '%s': %v", patternStr, err)
	}

	// Get resource type filter if pattern specifies it
	resourceType, _, _ := pattern.ParsePattern(patternStr)
	var typeFilter *resource.ResourceType
	if resourceType != "" {
		typeFilter = &resourceType
	}

	// List resources with optional type filter
	resources, err := mgr.List(typeFilter)
	if err != nil {
		t.Fatalf("failed to list resources: %v", err)
	}

	// Apply pattern matching
	var filtered []resource.Resource
	for _, res := range resources {
		if matcher.Match(&res) {
			filtered = append(filtered, res)
		}
	}

	return filtered
}

func TestListCmd_NoPattern(t *testing.T) {
	mgr := setupTestRepo(t)

	// List all resources
	resources, err := mgr.List(nil)
	if err != nil {
		t.Fatalf("failed to list resources: %v", err)
	}

	// Should have all 7 resources
	if len(resources) != 7 {
		t.Errorf("expected 7 resources, got %d", len(resources))
	}

	// Check that all types are present
	var cmdCount, skillCount, agentCount int
	for _, res := range resources {
		switch res.Type {
		case resource.Command:
			cmdCount++
		case resource.Skill:
			skillCount++
		case resource.Agent:
			agentCount++
		}
	}

	if cmdCount != 3 {
		t.Errorf("expected 3 commands, got %d", cmdCount)
	}
	if skillCount != 2 {
		t.Errorf("expected 2 skills, got %d", skillCount)
	}
	if agentCount != 2 {
		t.Errorf("expected 2 agents, got %d", agentCount)
	}
}

func TestListCmd_TypeFilter(t *testing.T) {
	mgr := setupTestRepo(t)

	tests := []struct {
		name          string
		pattern       string
		expectedCount int
		expectedType  resource.ResourceType
	}{
		{
			name:          "filter skills only",
			pattern:       "skill/*",
			expectedCount: 2,
			expectedType:  resource.Skill,
		},
		{
			name:          "filter commands only",
			pattern:       "command/*",
			expectedCount: 3,
			expectedType:  resource.Command,
		},
		{
			name:          "filter agents only",
			pattern:       "agent/*",
			expectedCount: 2,
			expectedType:  resource.Agent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := filterResources(t, mgr, tt.pattern)

			if len(filtered) != tt.expectedCount {
				t.Errorf("expected %d resources, got %d", tt.expectedCount, len(filtered))
			}

			// All should be the expected type
			for _, res := range filtered {
				if res.Type != tt.expectedType {
					t.Errorf("expected all resources to be %s, got type: %s", tt.expectedType, res.Type)
				}
			}
		})
	}
}

func TestListCmd_NamePattern(t *testing.T) {
	mgr := setupTestRepo(t)

	tests := []struct {
		name          string
		pattern       string
		expectedNames []string
	}{
		{
			name:          "wildcard prefix",
			pattern:       "command/test*",
			expectedNames: []string{"test-cmd"},
		},
		{
			name:          "wildcard contains",
			pattern:       "*pdf*",
			expectedNames: []string{"pdf-extract", "pdf-processing"},
		},
		{
			name:          "wildcard suffix",
			pattern:       "agent/*-reviewer",
			expectedNames: []string{"code-reviewer"},
		},
		{
			name:          "exact match with type",
			pattern:       "skill/test-skill",
			expectedNames: []string{"test-skill"},
		},
		{
			name:          "wildcard matches multiple types",
			pattern:       "*test*",
			expectedNames: []string{"test-cmd", "test-skill", "test-agent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := filterResources(t, mgr, tt.pattern)

			if len(filtered) != len(tt.expectedNames) {
				t.Errorf("expected %d resources, got %d", len(tt.expectedNames), len(filtered))
			}

			// Check each expected name is present
			foundNames := make(map[string]bool)
			for _, res := range filtered {
				foundNames[res.Name] = true
			}

			for _, expectedName := range tt.expectedNames {
				if !foundNames[expectedName] {
					t.Errorf("expected to find resource '%s', but it was not in results", expectedName)
				}
			}
		})
	}
}

func TestListCmd_NoMatches(t *testing.T) {
	mgr := setupTestRepo(t)

	filtered := filterResources(t, mgr, "command/nonexistent*")

	if len(filtered) != 0 {
		t.Errorf("expected 0 resources for non-matching pattern, got %d", len(filtered))
	}
}

func TestListCmd_EmptyRepository(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	mgr, err := repo.NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	resources, err := mgr.List(nil)
	if err != nil {
		t.Fatalf("failed to list resources: %v", err)
	}

	if len(resources) != 0 {
		t.Errorf("expected 0 resources in empty repository, got %d", len(resources))
	}
}

func TestListCmd_InvalidPattern(t *testing.T) {
	_ = setupTestRepo(t)

	// Try to create matcher with invalid pattern
	_, err := pattern.NewMatcher("command/[invalid")
	if err == nil {
		t.Error("expected error for invalid pattern, got nil")
	}
}

func TestListCmd_TypeAndNameCombinations(t *testing.T) {
	mgr := setupTestRepo(t)

	tests := []struct {
		name          string
		pattern       string
		expectedCount int
		description   string
	}{
		{
			name:          "type with exact name",
			pattern:       "command/pdf-extract",
			expectedCount: 1,
			description:   "should match exact command name",
		},
		{
			name:          "type with prefix wildcard",
			pattern:       "skill/*-processing",
			expectedCount: 1,
			description:   "should match skill ending with -processing",
		},
		{
			name:          "type with suffix wildcard",
			pattern:       "command/data-*",
			expectedCount: 1,
			description:   "should match command starting with data-",
		},
		{
			name:          "type with contains wildcard",
			pattern:       "agent/*code*",
			expectedCount: 1,
			description:   "should match agent containing 'code'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := filterResources(t, mgr, tt.pattern)

			if len(filtered) != tt.expectedCount {
				t.Errorf("%s: expected %d resources, got %d", tt.description, tt.expectedCount, len(filtered))
			}
		})
	}
}
