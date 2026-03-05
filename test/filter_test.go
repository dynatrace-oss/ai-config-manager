package test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
)

// TestFilterSkills tests filtering to import only skills
func TestFilterSkills(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create resources with mixed types using helper functions
	resourcesDir := filepath.Join(testDir, "resources")

	// Create command
	createTestCommandInDir(t, resourcesDir, "test-cmd1", "Test command 1")

	// Create skills
	createTestSkillInDir(t, resourcesDir, "test-skill1", "Test skill 1")
	createTestSkillInDir(t, resourcesDir, "pdf-skill", "PDF skill")

	// Create agent
	createTestAgentInDir(t, resourcesDir, "test-agent1", "Test agent 1")

	// Test: Filter to import only skills
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir, "--filter", "skill/*")
	if err != nil {
		t.Fatalf("Failed to add with filter: %v\nOutput: %s", err, output)
	}

	// Verify output shows filtering
	if !strings.Contains(output, "Filter: skill/*") {
		t.Errorf("Output should mention filter, got: %s", output)
	}

	// Should show 2 skills added
	if !strings.Contains(output, "Summary: 2 added") {
		t.Errorf("Should add 2 skills, got: %s", output)
	}

	// Verify only skills are in repository
	listOutput, err := runAimgr(t, "repo", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if !strings.Contains(listOutput, "test-skill1") {
		t.Errorf("Should contain test-skill1, got: %s", listOutput)
	}
	if !strings.Contains(listOutput, "pdf-skill") {
		t.Errorf("Should contain pdf-skill, got: %s", listOutput)
	}
	if strings.Contains(listOutput, "test-cmd1") {
		t.Errorf("Should NOT contain test-cmd1, got: %s", listOutput)
	}
	if strings.Contains(listOutput, "test-agent1") {
		t.Errorf("Should NOT contain test-agent1, got: %s", listOutput)
	}
}

// TestFilterPattern tests pattern matching filters
func TestFilterPattern(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create resources with various names using helper functions
	resourcesDir := filepath.Join(testDir, "resources")

	// Commands with different patterns
	createTestCommandInDir(t, resourcesDir, "pdf-parser", "PDF command")
	createTestCommandInDir(t, resourcesDir, "pdf-utils", "PDF utility")
	createTestCommandInDir(t, resourcesDir, "image-tool", "Image command")

	// Test: Filter with pattern matching
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir, "--filter", "pdf*")
	if err != nil {
		t.Fatalf("Failed to add with filter: %v\nOutput: %s", err, output)
	}

	// Should show 2 commands added (pdf-parser and pdf-utils)
	if !strings.Contains(output, "Summary: 2 added") {
		t.Errorf("Should add 2 pdf commands, got: %s", output)
	}

	// Verify only pdf commands are in repository
	listOutput, err := runAimgr(t, "repo", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if !strings.Contains(listOutput, "pdf-parser") {
		t.Errorf("Should contain pdf-parser, got: %s", listOutput)
	}
	if !strings.Contains(listOutput, "pdf-utils") {
		t.Errorf("Should contain pdf-utils, got: %s", listOutput)
	}
	if strings.Contains(listOutput, "image-tool") {
		t.Errorf("Should NOT contain image-tool, got: %s", listOutput)
	}
}

// TestFilterExactName tests exact name matching
func TestFilterExactName(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create multiple resources using helper functions
	resourcesDir := filepath.Join(testDir, "resources")

	createTestCommandInDir(t, resourcesDir, "exact-cmd", "Exact command")
	createTestCommandInDir(t, resourcesDir, "other-cmd", "Another command")

	// Test: Filter with exact name
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir, "--filter", "exact-cmd")
	if err != nil {
		t.Fatalf("Failed to add with filter: %v\nOutput: %s", err, output)
	}

	// Should show 1 command added
	if !strings.Contains(output, "Summary: 1 added") {
		t.Errorf("Should add 1 exact command, got: %s", output)
	}

	// Verify only exact-cmd is in repository
	listOutput, err := runAimgr(t, "repo", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if !strings.Contains(listOutput, "exact-cmd") {
		t.Errorf("Should contain exact-cmd, got: %s", listOutput)
	}
	if strings.Contains(listOutput, "other-cmd") {
		t.Errorf("Should NOT contain other-cmd, got: %s", listOutput)
	}
}

// TestFilterNoMatch tests filter that matches no resources
func TestFilterNoMatch(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create resources using helper function
	resourcesDir := filepath.Join(testDir, "resources")

	createTestCommandInDir(t, resourcesDir, "test-cmd", "Test command")

	// Test: Filter with no matches
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir, "--filter", "nomatch*")
	// Should not error, just show warning
	if err != nil {
		t.Fatalf("Should not error on no matches: %v\nOutput: %s", err, output)
	}

	// Should show warning about zero matches
	if !strings.Contains(output, "Warning") && !strings.Contains(output, "matched 0") {
		t.Errorf("Should show warning about zero matches, got: %s", output)
	}

	// Should show no resources added
	if !strings.Contains(output, "Summary: 0 added") && !strings.Contains(output, "0 resources") {
		t.Errorf("Should show 0 resources added, got: %s", output)
	}
}

// TestFilterWithDryRun tests combining filter with dry-run flag
func TestFilterWithDryRun(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create resources using helper function
	resourcesDir := filepath.Join(testDir, "resources")

	createTestSkillInDir(t, resourcesDir, "test-skill", "Test skill")

	// Test: Filter with dry-run
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir, "--filter", "skill/*", "--dry-run")
	if err != nil {
		t.Fatalf("Failed dry-run with filter: %v\nOutput: %s", err, output)
	}

	// Should show dry-run mode
	if !strings.Contains(output, "DRY RUN") {
		t.Errorf("Should mention dry-run mode, got: %s", output)
	}

	// Should show filter
	if !strings.Contains(output, "Filter: skill/*") {
		t.Errorf("Should mention filter, got: %s", output)
	}

	// Verify nothing was actually added
	listOutput, err := runAimgr(t, "repo", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if strings.Contains(listOutput, "test-skill") {
		t.Errorf("Dry-run should not add resources, got: %s", listOutput)
	}
}

// TestFilterWithForce tests combining filter with force flag
func TestFilterWithForce(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create and add a command first using helper function
	initialDir := filepath.Join(testDir, "initial")
	cmd1Path := createTestCommandInDir(t, initialDir, "update-me", "Original version")

	_, err := runAimgr(t, "repo", "add", "local:"+cmd1Path)
	if err != nil {
		t.Fatalf("Failed to add initial command: %v", err)
	}

	// Create resources directory with updated version using helper function
	resourcesDir := filepath.Join(testDir, "resources")

	createTestCommandInDir(t, resourcesDir, "update-me", "Updated version")
	createTestCommandInDir(t, resourcesDir, "other-cmd", "Another command")

	// Test: Filter with force to update only matching resource
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir, "--filter", "update*", "--force")
	if err != nil {
		t.Fatalf("Failed to add with filter and force: %v\nOutput: %s", err, output)
	}

	// Should show 1 command updated (not added, since it already existed)
	if !strings.Contains(output, "Summary: 0 added, 1 updated") {
		t.Errorf("Should update 1 command (not add), got: %s", output)
	}

	// Verify repository contains updated version
	listOutput, err := runAimgr(t, "repo", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if !strings.Contains(listOutput, "update-me") {
		t.Errorf("Should contain update-me, got: %s", listOutput)
	}
	if strings.Contains(listOutput, "other-cmd") {
		t.Errorf("Should NOT contain other-cmd (filtered out), got: %s", listOutput)
	}
}

// TestFilterWithSkipExisting tests combining filter with skip-existing flag
func TestFilterWithSkipExisting(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create and add a command first using helper function
	initialDir := filepath.Join(testDir, "initial")
	cmd1Path := createTestCommandInDir(t, initialDir, "existing", "Existing command")

	_, err := runAimgr(t, "repo", "add", "local:"+cmd1Path)
	if err != nil {
		t.Fatalf("Failed to add initial command: %v", err)
	}

	// Create resources directory using helper functions
	resourcesDir := filepath.Join(testDir, "resources")

	createTestCommandInDir(t, resourcesDir, "existing", "Existing command copy")
	createTestCommandInDir(t, resourcesDir, "new-cmd", "New command")

	// Test: Filter with skip-existing
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir, "--filter", "command/*", "--skip-existing")
	if err != nil {
		t.Fatalf("Failed to add with filter and skip-existing: %v\nOutput: %s", err, output)
	}

	// Should show 1 added, 1 skipped
	if !strings.Contains(output, "1 added") {
		t.Errorf("Should show 1 added, got: %s", output)
	}
	if !strings.Contains(output, "1 skipped") {
		t.Errorf("Should show 1 skipped, got: %s", output)
	}
}

// TestFilterWildcardAll tests using * to match all resources
func TestFilterWildcardAll(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create mixed resources using helper functions
	resourcesDir := filepath.Join(testDir, "resources")

	createTestCommandInDir(t, resourcesDir, "cmd1", "Command 1")
	createTestSkillInDir(t, resourcesDir, "skill1", "Skill 1")

	// Test: Wildcard all
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir, "--filter", "*")
	if err != nil {
		t.Fatalf("Failed to add with wildcard: %v\nOutput: %s", err, output)
	}

	// Should add all resources
	if !strings.Contains(output, "Summary: 2 added") {
		t.Errorf("Should add all 2 resources, got: %s", output)
	}
}

// TestFilterTypeWithExactName tests combining type filter with exact name
func TestFilterTypeWithExactName(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create resources with same name but different types using helper functions
	resourcesDir := filepath.Join(testDir, "resources")

	createTestCommandInDir(t, resourcesDir, "test-util", "Test command")
	createTestSkillInDir(t, resourcesDir, "test-util", "Test skill")

	// Test: Filter by type and exact name
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir, "--filter", "skill/test-util")
	if err != nil {
		t.Fatalf("Failed to add with type filter: %v\nOutput: %s", err, output)
	}

	// Should add only the skill
	if !strings.Contains(output, "Summary: 1 added") {
		t.Errorf("Should add only 1 skill, got: %s", output)
	}

	// Verify only skill is in repository
	listOutput, err := runAimgr(t, "repo", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	// Count occurrences - should be exactly 1 (the skill)
	count := strings.Count(listOutput, "test-util")
	if count != 1 {
		t.Errorf("Should have exactly 1 test-util (skill), got %d occurrences in: %s", count, listOutput)
	}

	// Verify it's the skill type
	if !strings.Contains(listOutput, "skill") && !strings.Contains(listOutput, "Skill") {
		t.Errorf("Should be a skill type, got: %s", listOutput)
	}
}

// TestFilterAgents tests filtering agent resources
func TestFilterAgents(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create mixed resources using helper functions
	resourcesDir := filepath.Join(testDir, "resources")

	createTestCommandInDir(t, resourcesDir, "test-cmd", "Command")
	createTestAgentInDir(t, resourcesDir, "code-reviewer", "Code reviewer agent")
	createTestAgentInDir(t, resourcesDir, "test-agent", "Test agent")

	// Test: Filter agents only
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir, "--filter", "agent/*")
	if err != nil {
		t.Fatalf("Failed to add agents: %v\nOutput: %s", err, output)
	}

	// Should add 2 agents
	if !strings.Contains(output, "Summary: 2 added") {
		t.Errorf("Should add 2 agents, got: %s", output)
	}

	// Verify only agents in repository
	listOutput, err := runAimgr(t, "repo", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if !strings.Contains(listOutput, "code-reviewer") {
		t.Errorf("Should contain code-reviewer, got: %s", listOutput)
	}
	if !strings.Contains(listOutput, "test-agent") {
		t.Errorf("Should contain test-agent, got: %s", listOutput)
	}
	if strings.Contains(listOutput, "test-cmd") {
		t.Errorf("Should NOT contain test-cmd, got: %s", listOutput)
	}
}

// TestFilterMultipleFlags tests that multiple --filter flags are combined with OR logic
func TestFilterMultipleFlags(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create mixed resources
	resourcesDir := filepath.Join(testDir, "resources")

	createTestSkillInDir(t, resourcesDir, "test-util", "Test skill util")
	createTestSkillInDir(t, resourcesDir, "other-skill", "Other skill")
	createTestCommandInDir(t, resourcesDir, "test-cmd", "Test command")
	createTestCommandInDir(t, resourcesDir, "other-cmd", "Other command")
	createTestAgentInDir(t, resourcesDir, "some-agent", "Some agent")

	// Test: import only skill/test-util AND command/*  (OR logic)
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir,
		"--filter", "skill/test-util",
		"--filter", "command/*")
	if err != nil {
		t.Fatalf("Failed to add with multiple filters: %v\nOutput: %s", err, output)
	}

	// Should add 1 skill + 2 commands = 3
	if !strings.Contains(output, "Summary: 3 added") {
		t.Errorf("Should add 3 resources (1 skill + 2 commands), got: %s", output)
	}

	// Verify correct resources are in repository
	listOutput, err := runAimgr(t, "repo", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if !strings.Contains(listOutput, "test-util") {
		t.Errorf("Should contain test-util skill, got: %s", listOutput)
	}
	if !strings.Contains(listOutput, "test-cmd") {
		t.Errorf("Should contain test-cmd, got: %s", listOutput)
	}
	if !strings.Contains(listOutput, "other-cmd") {
		t.Errorf("Should contain other-cmd, got: %s", listOutput)
	}
	if strings.Contains(listOutput, "other-skill") {
		t.Errorf("Should NOT contain other-skill, got: %s", listOutput)
	}
	if strings.Contains(listOutput, "some-agent") {
		t.Errorf("Should NOT contain some-agent, got: %s", listOutput)
	}
}

// TestFilterPersistsToManifest verifies that --filter values are written to ai.repo.yaml source.include
func TestFilterPersistsToManifest(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	resourcesDir := filepath.Join(testDir, "resources")
	createTestSkillInDir(t, resourcesDir, "pdf-skill", "PDF skill")
	createTestSkillInDir(t, resourcesDir, "docx-skill", "Docx skill")
	createTestCommandInDir(t, resourcesDir, "test-cmd", "Test command")

	// Add with two filters
	_, err := runAimgr(t, "repo", "add", "local:"+resourcesDir,
		"--filter", "skill/pdf-skill",
		"--filter", "skill/docx-skill",
		"--name", "test-source")
	if err != nil {
		t.Fatalf("Failed to add with filters: %v", err)
	}

	// Load manifest and verify include field
	manifest, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	src, found := manifest.GetSource("test-source")
	if !found {
		t.Fatalf("Source 'test-source' not found in manifest")
	}

	if len(src.Include) != 2 {
		t.Fatalf("Expected 2 include entries, got %d: %v", len(src.Include), src.Include)
	}

	includeSet := make(map[string]bool)
	for _, inc := range src.Include {
		includeSet[inc] = true
	}
	if !includeSet["skill/pdf-skill"] {
		t.Errorf("Include should contain 'skill/pdf-skill', got: %v", src.Include)
	}
	if !includeSet["skill/docx-skill"] {
		t.Errorf("Include should contain 'skill/docx-skill', got: %v", src.Include)
	}
}

// TestFilterNoFilterClearsInclude verifies that adding without --filter writes no include field
func TestFilterNoFilterClearsInclude(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	resourcesDir := filepath.Join(testDir, "resources")
	createTestCommandInDir(t, resourcesDir, "test-cmd", "Test command")

	// Add without any filter
	_, err := runAimgr(t, "repo", "add", "local:"+resourcesDir, "--name", "no-filter-source")
	if err != nil {
		t.Fatalf("Failed to add without filter: %v", err)
	}

	// Load manifest and verify no include field
	manifest, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	src, found := manifest.GetSource("no-filter-source")
	if !found {
		t.Fatalf("Source 'no-filter-source' not found in manifest")
	}

	if len(src.Include) != 0 {
		t.Errorf("Include should be empty when no --filter given, got: %v", src.Include)
	}
}

// TestFilterReAddReplacesInclude verifies REPLACE semantics: re-adding with different --filter replaces include
func TestFilterReAddReplacesInclude(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	resourcesDir := filepath.Join(testDir, "resources")
	createTestSkillInDir(t, resourcesDir, "pdf-skill", "PDF skill")
	createTestSkillInDir(t, resourcesDir, "docx-skill", "Docx skill")

	// First add with one filter
	_, err := runAimgr(t, "repo", "add", "local:"+resourcesDir,
		"--filter", "skill/pdf-skill",
		"--name", "my-source")
	if err != nil {
		t.Fatalf("Failed initial add: %v", err)
	}

	manifest, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load manifest after first add: %v", err)
	}
	src, _ := manifest.GetSource("my-source")
	if len(src.Include) != 1 || src.Include[0] != "skill/pdf-skill" {
		t.Fatalf("Expected include=[skill/pdf-skill] after first add, got: %v", src.Include)
	}

	// Re-add the same source with a different filter — should REPLACE include
	_, err = runAimgr(t, "repo", "add", "local:"+resourcesDir,
		"--filter", "skill/docx-skill",
		"--name", "my-source",
		"--force")
	if err != nil {
		t.Fatalf("Failed re-add: %v", err)
	}

	manifest, err = repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load manifest after re-add: %v", err)
	}
	src, found := manifest.GetSource("my-source")
	if !found {
		t.Fatalf("Source 'my-source' not found in manifest after re-add")
	}
	if len(src.Include) != 1 || src.Include[0] != "skill/docx-skill" {
		t.Errorf("Expected include=[skill/docx-skill] after re-add, got: %v", src.Include)
	}
}

// TestFilterReAddWithoutFilterClearsInclude verifies REPLACE semantics: re-adding without --filter clears include
func TestFilterReAddWithoutFilterClearsInclude(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	resourcesDir := filepath.Join(testDir, "resources")
	createTestSkillInDir(t, resourcesDir, "pdf-skill", "PDF skill")

	// First add with a filter
	_, err := runAimgr(t, "repo", "add", "local:"+resourcesDir,
		"--filter", "skill/pdf-skill",
		"--name", "my-source")
	if err != nil {
		t.Fatalf("Failed initial add: %v", err)
	}

	manifest, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load manifest after first add: %v", err)
	}
	src, _ := manifest.GetSource("my-source")
	if len(src.Include) == 0 {
		t.Fatalf("Expected non-empty include after first add with filter")
	}

	// Re-add without filter — include should be cleared
	_, err = runAimgr(t, "repo", "add", "local:"+resourcesDir,
		"--name", "my-source",
		"--force")
	if err != nil {
		t.Fatalf("Failed re-add without filter: %v", err)
	}

	manifest, err = repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load manifest after re-add: %v", err)
	}
	src, found := manifest.GetSource("my-source")
	if !found {
		t.Fatalf("Source 'my-source' not found in manifest after re-add")
	}
	if len(src.Include) != 0 {
		t.Errorf("Include should be cleared after re-add without --filter, got: %v", src.Include)
	}
}
