package discovery

import (
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestDiscoverSkills_SingleSkill(t *testing.T) {
	basePath := filepath.Join("testdata", "single-skill")

	skills, err := DiscoverSkills(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	if skills[0].Name != "skill1" {
		t.Errorf("expected skill name 'skill1', got '%s'", skills[0].Name)
	}

	if skills[0].Description != "Test skill 1 for single skill discovery" {
		t.Errorf("unexpected description: %s", skills[0].Description)
	}
}

func TestDiscoverSkills_MultipleStandardLocations(t *testing.T) {
	basePath := filepath.Join("testdata", "multi-skill-repo")

	skills, err := DiscoverSkills(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	// Should find skills in all priority locations
	if len(skills) < 4 {
		t.Fatalf("expected at least 4 skills, got %d", len(skills))
	}

	// Check that we found skills from different locations
	skillNames := make(map[string]bool)
	for _, skill := range skills {
		skillNames[skill.Name] = true
	}

	expectedNames := []string{"skill-priority1", "skill-claude", "skill-opencode", "skill-github"}
	for _, name := range expectedNames {
		if !skillNames[name] {
			t.Errorf("expected to find skill '%s'", name)
		}
	}
}

func TestDiscoverSkills_SubpathFiltering(t *testing.T) {
	basePath := filepath.Join("testdata", "subpath-test")

	// Search with subpath
	skills, err := DiscoverSkills(basePath, "docs")
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	if skills[0].Name != "skill2" {
		t.Errorf("expected skill name 'skill2', got '%s'", skills[0].Name)
	}
}

func TestDiscoverSkills_RecursiveFallback(t *testing.T) {
	basePath := filepath.Join("testdata", "recursive-test")

	// Should use recursive search since no priority locations exist
	skills, err := DiscoverSkills(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	if skills[0].Name != "skill3" {
		t.Errorf("expected skill name 'skill3', got '%s'", skills[0].Name)
	}
}

func TestDiscoverSkills_RecursiveWithPriority(t *testing.T) {
	basePath := filepath.Join("testdata", "multi-skill-repo")

	// Should find both priority and recursive skills
	skills, err := DiscoverSkills(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	// Should find at least 4 priority skills + recursive skill4
	skillNames := make(map[string]bool)
	for _, skill := range skills {
		skillNames[skill.Name] = true
	}

	// Since priority locations have skills, recursive search shouldn't happen
	// So we should NOT find skill4
	if skillNames["skill4"] {
		t.Errorf("should not find skill4 when priority locations have skills")
	}
}

func TestDiscoverSkills_Deduplication(t *testing.T) {
	basePath := filepath.Join("testdata", "duplicate-test")

	skills, err := DiscoverSkills(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	// Should only find one instance despite duplicates
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (deduplicated), got %d", len(skills))
	}

	// Should keep the first occurrence (from skills/ directory, not .claude/)
	if skills[0].Description != "First occurrence of duplicate skill" {
		t.Errorf("expected first occurrence, got: %s", skills[0].Description)
	}
}

func TestDiscoverSkills_EmptyRepo(t *testing.T) {
	// Use t.TempDir() for true isolation - don't rely on testdata which might
	// resolve differently in CI vs local
	basePath := t.TempDir()
	// basePath is now an empty temporary directory

	skills, err := DiscoverSkills(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverSkills should not error on empty repo: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("expected 0 skills in empty repo, got %d", len(skills))
	}
}

func TestDiscoverSkills_DirectSkillDirectory(t *testing.T) {
	basePath := filepath.Join("testdata", "direct-skill-test")

	// When the search path itself is a skill directory
	skills, err := DiscoverSkills(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	if skills[0].Name != "direct-skill-test" {
		t.Errorf("expected skill name 'direct-skill-test', got '%s'", skills[0].Name)
	}
}

func TestDiscoverSkills_InvalidSkillIgnored(t *testing.T) {
	basePath := filepath.Join("testdata", "invalid-skill")

	// Should return empty list when skill is invalid (missing description)
	skills, err := DiscoverSkills(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverSkills should not error on invalid skill: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("expected 0 skills (invalid skill should be skipped), got %d", len(skills))
	}
}

func TestDiscoverSkills_NonExistentPath(t *testing.T) {
	// Use a path where even the parent doesn't exist
	basePath := filepath.Join("testdata", "nonexistent-parent", "nonexistent")

	_, err := DiscoverSkills(basePath, "")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestDiscoverSkills_NonExistentSubpath(t *testing.T) {
	basePath := filepath.Join("testdata", "single-skill")

	// With lenient path handling, nonexistent subpath falls back to basePath
	// and should find the skill in the parent directory
	skills, err := DiscoverSkills(basePath, "nonexistent")
	if err != nil {
		t.Errorf("unexpected error for nonexistent subpath (should fall back to parent): %v", err)
	}

	// Should find at least the skill1 in basePath
	if len(skills) == 0 {
		t.Error("expected to find skills after falling back to parent directory")
	}
}

func TestIsSkillDir(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "valid skill directory",
			path:     filepath.Join("testdata", "single-skill", "skill1"),
			expected: true,
		},
		{
			name:     "directory without SKILL.md",
			path:     filepath.Join("testdata", "empty-repo"),
			expected: false,
		},
		{
			name:     "nonexistent directory",
			path:     filepath.Join("testdata", "nonexistent"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSkillDir(tt.path)
			if result != tt.expected {
				t.Errorf("isSkillDir(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestSearchSkillsInDir(t *testing.T) {
	tests := []struct {
		name          string
		dirPath       string
		expectedCount int
	}{
		{
			name:          "directory with skills",
			dirPath:       filepath.Join("testdata", "multi-skill-repo", "skills"),
			expectedCount: 1,
		},
		{
			name:          "directory without skills",
			dirPath:       filepath.Join("testdata", "empty-repo"),
			expectedCount: 0,
		},
		{
			name:          "nonexistent directory",
			dirPath:       filepath.Join("testdata", "nonexistent"),
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skills, _ := searchSkillsInDir(tt.dirPath)
			if len(skills) != tt.expectedCount {
				t.Errorf("searchSkillsInDir(%s) returned %d skills, expected %d",
					tt.dirPath, len(skills), tt.expectedCount)
			}
		})
	}
}

func TestRecursiveSearchSkills_MaxDepth(t *testing.T) {
	basePath := filepath.Join("testdata", "recursive-test")

	// Test that recursive search respects max depth
	skills, _, err := recursiveSearchSkills(basePath, 0)
	if err != nil {
		t.Fatalf("recursiveSearchSkills failed: %v", err)
	}

	// Should find skill3 which is at depth 2
	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
}

func TestRecursiveSearchSkills_StopsAtMaxDepth(t *testing.T) {
	// Test that search stops at max depth (5)
	// skill3 is at depth 2 (some/nested/skill3), so it should be found
	basePath := filepath.Join("testdata", "recursive-test")

	skills, _, err := recursiveSearchSkills(basePath, 4) // Start at depth 4
	if err != nil {
		t.Fatalf("recursiveSearchSkills failed: %v", err)
	}

	// Should return empty since we're at max depth
	if len(skills) != 0 {
		t.Errorf("expected 0 skills at max depth, got %d", len(skills))
	}
}

func TestMapToSlice(t *testing.T) {
	// Test the deduplication helper
	basePath := filepath.Join("testdata", "single-skill", "skill1")
	skill, err := DiscoverSkills(basePath, "")
	if err != nil {
		t.Fatalf("Failed to load test skill: %v", err)
	}

	skillMap := make(map[string]*resource.Resource)
	skillMap["skill1"] = skill[0]
	skillMap["skill1-dup"] = skill[0] // Add duplicate

	result := mapToSlice(skillMap)

	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
}

func TestSkillCandidate(t *testing.T) {
	// Test that SkillCandidate struct is properly defined
	candidate := SkillCandidate{
		Path:     "/test/path",
		Resource: nil,
	}

	if candidate.Path != "/test/path" {
		t.Errorf("SkillCandidate Path not set correctly")
	}
}
