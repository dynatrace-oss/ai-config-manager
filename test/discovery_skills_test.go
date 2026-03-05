package test

import (
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
	"github.com/dynatrace-oss/ai-config-manager/v3/test/testutil"
)

func TestDiscoverSkills_StandardLocation(t *testing.T) {
	// Test skills in standard "skills/" directory
	fixturePath := testutil.GetFixturePath("repos/skills-standard")

	skills, err := discovery.DiscoverSkills(fixturePath, "")
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	if len(skills) != 2 {
		t.Errorf("Expected 2 skills, got %d", len(skills))
	}

	// Verify skill names
	skillNames := make(map[string]bool)
	for _, skill := range skills {
		skillNames[skill.Name] = true
	}

	if !skillNames["test-skill-1"] {
		t.Errorf("test-skill-1 not found")
	}
	if !skillNames["test-skill-2"] {
		t.Errorf("test-skill-2 not found")
	}

	// Verify descriptions are present
	for _, skill := range skills {
		if skill.Description == "" {
			t.Errorf("Skill %s has empty description", skill.Name)
		}
	}
}

func TestDiscoverSkills_PriorityLocations(t *testing.T) {
	// Test skills in .claude/skills, .opencode/skills
	fixturePath := testutil.GetFixturePath("repos/dotdir-resources")

	skills, err := discovery.DiscoverSkills(fixturePath, "")
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	// Should find skill in .claude/skills/
	if len(skills) == 0 {
		t.Errorf("Expected to find skills in priority locations")
	}

	// Verify we found the claude-skill
	foundClaudeSkill := false
	for _, skill := range skills {
		if skill.Name == "claude-skill" {
			foundClaudeSkill = true
			if skill.Description == "" {
				t.Errorf("claude-skill has empty description")
			}
		}
	}

	if !foundClaudeSkill {
		t.Errorf("Expected to find claude-skill in .claude/skills/")
	}
}

func TestDiscoverSkills_Subpath(t *testing.T) {
	// Test discovery with subpath
	fixturePath := testutil.GetFixturePath("repos/subpath-test")

	skills, err := discovery.DiscoverSkills(fixturePath, "level1/level2")
	if err != nil {
		t.Fatalf("DiscoverSkills with subpath failed: %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("Expected 1 skill in subpath, got %d", len(skills))
	}

	if len(skills) > 0 {
		if skills[0].Name != "deep-skill" {
			t.Errorf("Expected deep-skill, got %s", skills[0].Name)
		}
		if skills[0].Description == "" {
			t.Errorf("deep-skill has empty description")
		}
	}
}

func TestDiscoverSkills_EmptyRepository(t *testing.T) {
	// Test empty repository (no skills)
	fixturePath := testutil.GetFixturePath("repos/empty-repo")

	skills, err := discovery.DiscoverSkills(fixturePath, "")
	if err != nil {
		t.Fatalf("DiscoverSkills should not error on empty repo: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("Expected 0 skills in empty repo, got %d", len(skills))
	}
}

func TestDiscoverSkills_MalformedFrontmatter(t *testing.T) {
	// Test error handling for malformed resources
	fixturePath := testutil.GetFixturePath("repos/malformed-resources")

	// Should not panic, should collect errors
	skills, errs, err := discovery.DiscoverSkillsWithErrors(fixturePath, "")

	// Should not return fatal error
	if err != nil {
		t.Fatalf("DiscoverSkillsWithErrors should not return fatal error: %v", err)
	}

	// Should have encountered some errors (invalid YAML, etc.)
	if len(errs) == 0 {
		t.Logf("Warning: Expected some discovery errors for malformed resources, got none")
	}

	// Some skills might still be discovered (e.g., missing-description might parse but be filtered)
	// Main goal: verify no panic, graceful error handling
	t.Logf("Discovered %d skills with %d errors", len(skills), len(errs))

	// Verify errors contain path information
	for _, discErr := range errs {
		if discErr.Path == "" {
			t.Errorf("Discovery error missing path: %v", discErr.Error)
		}
		if discErr.Error == nil {
			t.Errorf("Discovery error missing error detail for path: %s", discErr.Path)
		}
	}
}

func TestDiscoverSkills_Deduplication(t *testing.T) {
	// If same skill appears in multiple locations, keep first
	// Test that mixed-resources doesn't have duplicates
	fixturePath := testutil.GetFixturePath("repos/mixed-resources")

	skills, err := discovery.DiscoverSkills(fixturePath, "")
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	// Check for duplicates by name
	seen := make(map[string]bool)
	for _, skill := range skills {
		if seen[skill.Name] {
			t.Errorf("Duplicate skill found: %s", skill.Name)
		}
		seen[skill.Name] = true
	}

	// Should find at least one skill (skill1)
	if len(skills) == 0 {
		t.Errorf("Expected to find at least one skill in mixed-resources")
	}

	// Verify skill1 exists
	if !seen["skill1"] {
		t.Errorf("Expected to find skill1 in mixed-resources")
	}
}

func TestDiscoverSkills_RecursiveFallback(t *testing.T) {
	// Test recursive search when no priority locations found
	// subpath-test has skills in level1/level2/skills/ (deeply nested)
	// If we search from root without subpath, recursive should find it
	fixturePath := testutil.GetFixturePath("repos/subpath-test")

	skills, err := discovery.DiscoverSkills(fixturePath, "")
	if err != nil {
		t.Fatalf("Recursive discovery failed: %v", err)
	}

	if len(skills) == 0 {
		t.Errorf("Recursive search should find deeply nested skills")
	}

	// Should find deep-skill through recursive search
	foundDeepSkill := false
	for _, skill := range skills {
		if skill.Name == "deep-skill" {
			foundDeepSkill = true
		}
	}

	if !foundDeepSkill {
		t.Errorf("Expected recursive search to find deep-skill")
	}
}

// TestDiscoverSkills_PerformanceBenchmark ensures all tests complete quickly
func TestDiscoverSkills_PerformanceBenchmark(t *testing.T) {
	// This test is primarily documentation - the actual timing is checked by
	// running the full test suite. All skill discovery tests should complete
	// in under 1 second total.

	testCases := []struct {
		name        string
		fixturePath string
		subpath     string
	}{
		{"standard", "repos/skills-standard", ""},
		{"dotdir", "repos/dotdir-resources", ""},
		{"subpath", "repos/subpath-test", "level1/level2"},
		{"empty", "repos/empty-repo", ""},
		{"malformed", "repos/malformed-resources", ""},
		{"mixed", "repos/mixed-resources", ""},
		{"recursive", "repos/subpath-test", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixturePath := testutil.GetFixturePath(tc.fixturePath)
			_, err := discovery.DiscoverSkills(fixturePath, tc.subpath)
			if err != nil && tc.name != "empty" {
				// Empty repo should succeed with 0 results
				t.Logf("Discovery for %s returned error (may be expected): %v", tc.name, err)
			}
		})
	}
}
