package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLIBulkAdd tests the 'aimgr repo add bulk' command
func TestCLIBulkAdd(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create a folder with mixed resources using helper functions
	resourcesDir := filepath.Join(testDir, "resources")
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		t.Fatalf("Failed to create resources dir: %v", err)
	}

	// Create commands using helper - note: helper returns full path to .md file
	cmd1Path := createTestCommand(t, "bulk-cmd1", "Bulk test command 1")
	cmd2Path := createTestCommand(t, "bulk-cmd2", "Bulk test command 2")

	// Copy commands to shared resources directory
	commandsDir := filepath.Join(resourcesDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}
	copyFile(t, cmd1Path, filepath.Join(commandsDir, "bulk-cmd1.md"))
	copyFile(t, cmd2Path, filepath.Join(commandsDir, "bulk-cmd2.md"))

	// Create skill using helper - note: helper returns directory path
	skill1Path := createTestSkill(t, "bulk-skill1", "Bulk test skill 1")

	// Copy skill to shared resources directory
	skillsDir := filepath.Join(resourcesDir, "skills", "bulk-skill1")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}
	copyFile(t, filepath.Join(skill1Path, "SKILL.md"), filepath.Join(skillsDir, "SKILL.md"))

	// Create agent using helper - note: helper returns full path to .md file
	agent1Path := createTestAgent(t, "bulk-agent1", "Bulk test agent 1")

	// Copy agent to shared resources directory
	agentsDir := filepath.Join(resourcesDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}
	copyFile(t, agent1Path, filepath.Join(agentsDir, "bulk-agent1.md"))

	// Test: aimgr repo add (unified command)
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir)
	if err != nil {
		t.Fatalf("Failed to add resources: %v\nOutput: %s", err, output)
	}

	// Verify output mentions resources
	if !strings.Contains(output, "Found: 2 commands, 1 skills, 1 agents") {
		t.Errorf("Output should show discovered resources, got: %s", output)
	}

	if !strings.Contains(output, "Summary: 4 added") {
		t.Errorf("Output should show 4 added resources, got: %s", output)
	}

	// Verify resources are in repository
	listOutput, err := runAimgr(t, "repo", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	expectedResources := []string{"bulk-cmd1", "bulk-cmd2", "bulk-skill1", "bulk-agent1"}
	for _, resource := range expectedResources {
		if !strings.Contains(listOutput, resource) {
			t.Errorf("List should contain '%s', got: %s", resource, listOutput)
		}
	}

	// Verify repository integrity after import
	// This catches bugs where import succeeds but verify reports issues
	t.Log("Verifying repository integrity after import")
	AssertVerifyClean(t)
}

// TestCLIBulkAddForce tests bulk add with force flag
func TestCLIBulkAddForce(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create and add a command first using helper
	cmdPath := createTestCommand(t, "conflict", "Original command")

	_, err := runAimgr(t, "repo", "add", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command first time: %v", err)
	}

	// Create resources dir with same command name but updated content
	resourcesDir := filepath.Join(testDir, "resources")
	commandsDir := filepath.Join(resourcesDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	// Create a command with same name but different content
	cmdPath2 := createTestCommand(t, "conflict", "Updated command")
	copyFile(t, cmdPath2, filepath.Join(commandsDir, "conflict.md"))

	// Try unified add with force
	output, err := runAimgr(t, "repo", "add", "--force", "local:"+resourcesDir)
	if err != nil {
		t.Fatalf("Force add failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Summary: 0 added, 1 updated") {
		t.Errorf("Should show updated (not added) when overwriting with force flag, got: %s", output)
	}
}

// TestCLIBulkAddSkipExisting tests bulk add with skip-existing flag
func TestCLIBulkAddSkipExisting(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create and add a command using helper
	cmdPath := createTestCommand(t, "skip", "Existing command")

	_, err := runAimgr(t, "repo", "add", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Create resources dir with same command
	resourcesDir := filepath.Join(testDir, "resources")
	commandsDir := filepath.Join(resourcesDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	cmdPath2 := createTestCommand(t, "skip", "Existing command")
	copyFile(t, cmdPath2, filepath.Join(commandsDir, "skip.md"))

	// Try unified add with skip-existing
	output, err := runAimgr(t, "repo", "add", "--skip-existing", "local:"+resourcesDir)
	if err != nil {
		t.Fatalf("Skip-existing add failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "1 skipped") {
		t.Errorf("Should skip existing resource, got: %s", output)
	}
	if !strings.Contains(output, "0 added") {
		t.Errorf("Should not add any resources, got: %s", output)
	}
}

// TestCLIBulkAddNoFlagsFailsOnConflict tests that bulk add without flags fails on conflict
func TestCLIBulkAddNoFlagsFailsOnConflict(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create and add a command using helper
	cmdPath := createTestCommand(t, "fail", "Existing command")

	_, err := runAimgr(t, "repo", "add", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Create resources dir with same command
	resourcesDir := filepath.Join(testDir, "resources")
	commandsDir := filepath.Join(resourcesDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	cmdPath2 := createTestCommand(t, "fail", "Existing command")
	copyFile(t, cmdPath2, filepath.Join(commandsDir, "fail.md"))

	// Try unified add without flags (should fail)
	_, err = runAimgr(t, "repo", "add", "local:"+resourcesDir)
	if err == nil {
		t.Error("Expected error on conflict without flags, got nil")
	}
}

// TestCLIBulkAddDryRun tests bulk add with dry-run flag
func TestCLIBulkAddDryRun(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create a resources directory with one command using helper
	resourcesDir := filepath.Join(testDir, "resources")
	commandsDir := filepath.Join(resourcesDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	cmdPath := createTestCommand(t, "dryrun", "Dry run test")
	copyFile(t, cmdPath, filepath.Join(commandsDir, "dryrun.md"))

	// Test unified add with dry-run
	output, err := runAimgr(t, "repo", "add", "--dry-run", "local:"+resourcesDir)
	if err != nil {
		t.Fatalf("Dry run add failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "DRY RUN") {
		t.Errorf("Output should mention dry run mode, got: %s", output)
	}

	if !strings.Contains(output, "Summary: 1 added") {
		t.Errorf("Output should show 1 added in preview, got: %s", output)
	}

	// Verify nothing was actually added
	listOutput, err := runAimgr(t, "repo", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if strings.Contains(listOutput, "dryrun") {
		t.Errorf("Dry run should not add resources, but found 'dryrun' in: %s", listOutput)
	}
}

// TestCLIBulkAddEmptyFolder tests unified add on an empty folder
func TestCLIBulkAddEmptyFolder(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create an empty resources directory
	resourcesDir := filepath.Join(testDir, "empty")
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		t.Fatalf("Failed to create empty dir: %v", err)
	}

	// Test unified add on empty folder (should fail)
	_, err := runAimgr(t, "repo", "add", "local:"+resourcesDir)
	if err == nil {
		t.Error("Expected error on empty folder, got nil")
	}
}

// TestCLIAddSingleFile tests unified add on a single file (should succeed for .md files)
func TestCLIAddSingleFile(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create a command file using helper
	cmdPath := createTestCommand(t, "test-cmd", "Test command")

	// Test unified add on single file (should succeed)
	output, err := runAimgr(t, "repo", "add", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add single file: %v\nOutput: %s", err, output)
	}

	// Verify success
	if !strings.Contains(output, "Added command") {
		t.Errorf("Output should show added command, got: %s", output)
	}
}
