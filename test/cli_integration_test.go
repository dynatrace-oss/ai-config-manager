package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Helper function to run aimgr CLI command
func runAimgr(t *testing.T, args ...string) (string, error) {
	t.Helper()

	// Build path to aimgr binary
	binPath := filepath.Join("..", "aimgr")

	cmd := exec.Command(binPath, args...)
	// Start with parent environment
	cmd.Env = os.Environ()

	// Override with test-specific environment variables if set
	// NOTE: t.Setenv() only affects os.Getenv() in the test process,
	// not child processes, so we need to explicitly propagate them

	// Propagate AIMGR_REPO_PATH
	if repoPath := os.Getenv("AIMGR_REPO_PATH"); repoPath != "" {
		// Replace or add AIMGR_REPO_PATH in the environment
		found := false
		for i, env := range cmd.Env {
			if strings.HasPrefix(env, "AIMGR_REPO_PATH=") {
				cmd.Env[i] = "AIMGR_REPO_PATH=" + repoPath
				found = true
				break
			}
		}
		if !found {
			cmd.Env = append(cmd.Env, "AIMGR_REPO_PATH="+repoPath)
		}
	}

	// Propagate XDG_DATA_HOME
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		// Replace or add XDG_DATA_HOME in the environment
		found := false
		for i, env := range cmd.Env {
			if strings.HasPrefix(env, "XDG_DATA_HOME=") {
				cmd.Env[i] = "XDG_DATA_HOME=" + xdgDataHome
				found = true
				break
			}
		}
		if !found {
			cmd.Env = append(cmd.Env, "XDG_DATA_HOME="+xdgDataHome)
		}
	}

	// Propagate XDG_CONFIG_HOME
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		// Replace or add XDG_CONFIG_HOME in the environment
		found := false
		for i, env := range cmd.Env {
			if strings.HasPrefix(env, "XDG_CONFIG_HOME=") {
				cmd.Env[i] = "XDG_CONFIG_HOME=" + xdgConfigHome
				found = true
				break
			}
		}
		if !found {
			cmd.Env = append(cmd.Env, "XDG_CONFIG_HOME="+xdgConfigHome)
		}
	}

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// setupTestEnvironment creates isolated test directories and sets environment variables
// to prevent tests from reading the user's actual config
func setupTestEnvironment(t *testing.T) {
	t.Helper()

	// Create isolated repo directory
	repoDir := t.TempDir()

	// Create isolated config directory
	configDir := t.TempDir()

	// Set environment variables for complete isolation
	t.Setenv("AIMGR_REPO_PATH", repoDir)
	t.Setenv("XDG_CONFIG_HOME", configDir)
	t.Setenv("XDG_DATA_HOME", repoDir)

	// Create a default config file in the isolated config directory
	aimgrConfigDir := filepath.Join(configDir, "aimgr")
	if err := os.MkdirAll(aimgrConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	configContent := `install:
  targets:
    - claude
`
	configPath := filepath.Join(aimgrConfigDir, "aimgr.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
}

// TestCLIRepoAdd tests the 'aimgr repo add' command
func TestCLIRepoAdd(t *testing.T) {
	// Create temporary repo directory
	setupTestEnvironment(t)

	// Create test command using helper
	cmdPath := createTestCommand(t, "test-cmd", "A test command")

	// Test: aimgr repo add (unified command)
	output, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "test-cmd") {
		t.Errorf("Output should mention command name, got: %s", output)
	}
}

// TestCLIRepoList tests the 'aimgr repo list' command
func TestCLIRepoList(t *testing.T) {
	setupTestEnvironment(t)

	// Create test command using helper
	cmdPath := createTestCommand(t, "list-test", "A command for list testing")

	// Add the command
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Test: aimgr repo list
	output, err := runAimgr(t, "repo", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "list-test") {
		t.Errorf("List output should contain 'list-test', got: %s", output)
	}
}

// TestCLIRepoShow tests the 'aimgr repo show' command
func TestCLIRepoShow(t *testing.T) {
	setupTestEnvironment(t)

	// Create test command using helper, but we need custom metadata
	// So we'll create it manually but in proper structure
	tempDir := t.TempDir()
	commandsDir := filepath.Join(tempDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	cmdPath := filepath.Join(commandsDir, "show-test.md")
	cmdContent := `---
description: A command for show testing
version: "1.0.0"
author: test-author
license: MIT
---
# Show Test Command
`
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Add the command
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Test: aimgr repo show command/show-test
	output, err := runAimgr(t, "repo", "show", "command/show-test")
	if err != nil {
		t.Fatalf("Failed to show command: %v\nOutput: %s", err, output)
	}

	// Verify output contains expected fields
	expectedStrings := []string{
		"show-test",
		"A command for show testing",
		"1.0.0",
		"test-author",
		"MIT",
		"Source:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Show output should contain '%s', got: %s", expected, output)
		}
	}
}

// TestCLIRepoShowSkill tests 'aimgr repo show skill'
func TestCLIRepoShowSkill(t *testing.T) {
	setupTestEnvironment(t)

	// Create test skill using helper, but we need custom metadata
	// So we'll create it manually but in proper structure
	tempDir := t.TempDir()
	skillDir := filepath.Join(tempDir, "skills", "show-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillContent := `---
name: show-skill
description: A skill for show testing
version: "2.0.0"
author: skill-author
license: Apache-2.0
---
# Show Skill
`
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	// Add the skill
	addOutput, err := runAimgr(t, "repo", "add", "--force", "local:"+skillDir)
	if err != nil {
		t.Fatalf("Failed to add skill: %v\nOutput: %s", err, addOutput)
	}

	// Test: aimgr repo show skill/show-skill
	output, err := runAimgr(t, "repo", "show", "skill/show-skill")
	if err != nil {
		t.Fatalf("Failed to show skill: %v\nOutput: %s", err, output)
	}

	// Verify output contains expected fields
	expectedStrings := []string{
		"show-skill",
		"A skill for show testing",
		"2.0.0",
		"skill-author",
		"Apache-2.0",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Show output should contain '%s', got: %s", expected, output)
		}
	}
}

// TestCLIRepoShowAgent tests 'aimgr repo show agent'
func TestCLIRepoShowAgent(t *testing.T) {
	setupTestEnvironment(t)

	// Create test agent using helper, but we need custom metadata
	// So we'll create it manually but in proper structure
	tempDir := t.TempDir()
	agentsDir := filepath.Join(tempDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents directory: %v", err)
	}

	agentPath := filepath.Join(agentsDir, "show-agent.md")
	agentContent := `---
description: An agent for show testing
type: helper
version: "1.5.0"
---
# Show Agent
`
	if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	// Add the agent
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+agentPath)
	if err != nil {
		t.Fatalf("Failed to add agent: %v", err)
	}

	// Test: aimgr repo show agent/show-agent
	output, err := runAimgr(t, "repo", "show", "agent/show-agent")
	if err != nil {
		t.Fatalf("Failed to show agent: %v\nOutput: %s", err, output)
	}

	// Verify output contains expected fields
	expectedStrings := []string{
		"show-agent",
		"An agent for show testing",
		"1.5.0",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Show output should contain '%s', got: %s", expected, output)
		}
	}
}

// TestCLIInstall tests 'aimgr install' command
func TestCLIInstall(t *testing.T) {
	setupTestEnvironment(t)
	projectDir := t.TempDir()

	// Create .claude directory to trigger Claude detection
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create test skill using helper
	skillDir := createTestSkill(t, "install-skill", "A skill for install testing")

	// Add to repository
	addOutput, err := runAimgr(t, "repo", "add", "--force", "local:"+skillDir)
	if err != nil {
		t.Fatalf("Failed to add skill: %v\nOutput: %s", err, addOutput)
	}

	// Test: aimgr install skill/install-skill
	output, err := runAimgr(t, "install", "skill/install-skill", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install skill: %v\nOutput: %s", err, output)
	}

	// Verify symlink was created
	symlinkPath := filepath.Join(projectDir, ".claude", "skills", "install-skill")
	if _, err := os.Lstat(symlinkPath); err != nil {
		t.Errorf("Symlink should be created at %s: %v", symlinkPath, err)
	}
}

// TestCLIInstallMultiple tests installing multiple resources at once
func TestCLIInstallMultiple(t *testing.T) {
	setupTestEnvironment(t)
	projectDir := t.TempDir()

	// Create .claude directory
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create test command using helper
	cmdPath := createTestCommand(t, "multi-cmd", "Multi test command")

	// Create test skill using helper
	skillDir := createTestSkill(t, "multi-skill", "Multi test skill")

	// Add both to repository
	addCmdOutput, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v\nOutput: %s", err, addCmdOutput)
	}

	addSkillOutput, err := runAimgr(t, "repo", "add", "--force", "local:"+skillDir)
	if err != nil {
		t.Fatalf("Failed to add skill: %v\nOutput: %s", err, addSkillOutput)
	}

	// Test: install both resources
	output, err := runAimgr(t, "install", "command/multi-cmd", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install command: %v\nOutput: %s", err, output)
	}

	output, err = runAimgr(t, "install", "skill/multi-skill", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install skill: %v\nOutput: %s", err, output)
	}

	// Verify both symlinks were created
	cmdSymlink := filepath.Join(projectDir, ".claude", "commands", "multi-cmd.md")
	if _, err := os.Lstat(cmdSymlink); err != nil {
		t.Errorf("Command symlink should be created: %v", err)
	}

	skillSymlink := filepath.Join(projectDir, ".claude", "skills", "multi-skill")
	if _, err := os.Lstat(skillSymlink); err != nil {
		t.Errorf("Skill symlink should be created: %v", err)
	}
}

// TestCLIUninstall tests the 'aimgr uninstall' command
func TestCLIUninstall(t *testing.T) {
	setupTestEnvironment(t)
	projectDir := t.TempDir()

	// Create .claude directory
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create test skill using helper
	skillDir := createTestSkill(t, "uninstall-skill", "A skill for uninstall testing")

	// Add to repository
	addOutput, err := runAimgr(t, "repo", "add", "--force", "local:"+skillDir)
	if err != nil {
		t.Fatalf("Failed to add skill: %v\nOutput: %s", err, addOutput)
	}

	// Install it
	installOutput, err := runAimgr(t, "install", "skill/uninstall-skill", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install skill: %v\nOutput: %s", err, installOutput)
	}

	// Verify symlink exists
	symlinkPath := filepath.Join(projectDir, ".claude", "skills", "uninstall-skill")
	if _, err := os.Lstat(symlinkPath); err != nil {
		t.Fatalf("Symlink should exist before uninstall: %v", err)
	}

	// Test: aimgr uninstall skill/uninstall-skill
	output, err := runAimgr(t, "uninstall", "skill/uninstall-skill", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to uninstall: %v\nOutput: %s", err, output)
	}

	// Verify symlink was removed
	if _, err := os.Lstat(symlinkPath); err == nil {
		t.Error("Symlink should be removed after uninstall")
	}
}

// TestCLIUninstallSkipsNonSymlinks tests uninstall safety features
func TestCLIUninstallSkipsNonSymlinks(t *testing.T) {
	setupTestEnvironment(t)
	projectDir := t.TempDir()

	initOutput, err := runAimgr(t, "repo", "init")
	if err != nil {
		t.Fatalf("Failed to initialize repo: %v\nOutput: %s", err, initOutput)
	}

	// Create .claude directory with a regular file (not a symlink)
	claudeSkillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(claudeSkillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills directory: %v", err)
	}

	// Create a regular file (not managed by aimgr)
	regularFile := filepath.Join(claudeSkillsDir, "regular-file")
	if err := os.MkdirAll(regularFile, 0755); err != nil {
		t.Fatalf("Failed to create regular directory: %v", err)
	}

	// Test: aimgr uninstall skill/regular-file should skip it
	output, err := runAimgr(t, "uninstall", "skill/regular-file", "--project-path", projectDir)

	// Should not error, but should skip
	if err != nil && !strings.Contains(output, "skipped") {
		t.Fatalf("Unexpected error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(strings.ToLower(output), "skip") {
		t.Errorf("Output should indicate skipping, got: %s", output)
	}

	// Verify the file still exists
	if _, err := os.Stat(regularFile); err != nil {
		t.Error("Regular file should not be removed")
	}
}

// TestCLIUninstallSkipsExternalSymlinks tests uninstall only removes aimgr-managed symlinks
func TestCLIUninstallSkipsExternalSymlinks(t *testing.T) {
	setupTestEnvironment(t)
	projectDir := t.TempDir()
	externalDir := t.TempDir()

	initOutput, err := runAimgr(t, "repo", "init")
	if err != nil {
		t.Fatalf("Failed to initialize repo: %v\nOutput: %s", err, initOutput)
	}

	// Create .claude directory
	claudeSkillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(claudeSkillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills directory: %v", err)
	}

	// Create an external skill directory
	externalSkill := filepath.Join(externalDir, "external-skill")
	if err := os.MkdirAll(externalSkill, 0755); err != nil {
		t.Fatalf("Failed to create external skill: %v", err)
	}

	// Create a symlink pointing to external directory
	symlinkPath := filepath.Join(claudeSkillsDir, "external-skill")
	if err := os.Symlink(externalSkill, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Test: aimgr uninstall skill/external-skill should skip it (not from repo)
	output, err := runAimgr(t, "uninstall", "skill/external-skill", "--project-path", projectDir)

	// Should not error, but should skip
	if err != nil && !strings.Contains(output, "skipped") {
		t.Fatalf("Unexpected error: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(strings.ToLower(output), "skip") && !strings.Contains(strings.ToLower(output), "not managed") {
		t.Errorf("Output should indicate skipping external symlink, got: %s", output)
	}

	// Verify the symlink still exists
	if _, err := os.Lstat(symlinkPath); err != nil {
		t.Error("External symlink should not be removed")
	}
}

// TestCLIMetadataTracking tests that metadata is properly tracked
func TestCLIMetadataTracking(t *testing.T) {
	setupTestEnvironment(t)

	// Create test command using helper
	cmdPath := createTestCommand(t, "metadata-test", "Metadata test command")

	// Add the command
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Show the command to check metadata
	output, err := runAimgr(t, "repo", "show", "command/metadata-test")
	if err != nil {
		t.Fatalf("Failed to show command: %v\nOutput: %s", err, output)
	}

	// Verify metadata fields are present
	metadataFields := []string{
		"Source:",
		"Source Type:",
		"First Installed:",
		"Last Updated:",
	}

	for _, field := range metadataFields {
		if !strings.Contains(output, field) {
			t.Errorf("Metadata should contain '%s', got: %s", field, output)
		}
	}

	// Note: We cannot easily verify metadata file location in CLI tests
	// because the CLI uses XDG data home, not a custom repo path.
	// The metadata fields in the show output above verify that metadata is working.
}

// TestCLIMetadataUpdatedOnUpdate tests metadata timestamps are updated

// TestCLIMetadataDeletedOnRemove tests metadata is deleted with resource
// NOTE: Currently skipped because orphan cleanup doesn't work for file-path sources
// See: https://github.com/dynatrace-oss/ai-config-manager/issues/TBD
func TestCLIMetadataDeletedOnRemove(t *testing.T) {
	t.Skip("Orphan cleanup not yet implemented for file-path sources - needs architecture fix")
	setupTestEnvironment(t)

	// Create test command using helper
	cmdPath := createTestCommand(t, "meta-remove-test", "Remove test")

	// Add the command
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Note: We cannot easily verify metadata file location in CLI tests
	// because the CLI uses XDG data home, not a custom repo path.
	// We'll verify that the show command works before removal.
	showOutput, err := runAimgr(t, "repo", "show", "command/meta-remove-test")
	if err != nil {
		t.Fatalf("Failed to show command before removal: %v", err)
	}
	if !strings.Contains(showOutput, "meta-remove-test") {
		t.Error("Command should exist before removal")
	}

	// First list sources to see what was created
	listSourcesOutput, err := runAimgr(t, "repo", "info")
	if err != nil {
		t.Fatalf("Failed to list sources: %v", err)
	}
	t.Logf("Sources before removal:\n%s", listSourcesOutput)

	// Remove the source (which removes the command as an orphan)
	// repo remove operates on sources, not individual resources
	// The source was auto-named based on the directory path
	removeOutput, err := runAimgr(t, "repo", "remove", cmdPath)
	if err != nil {
		t.Fatalf("Failed to remove source: %v\nOutput: %s", err, removeOutput)
	}
	t.Logf("Remove output:\n%s", removeOutput)

	// List sources after removal
	listSourcesAfter, _ := runAimgr(t, "repo", "info")
	t.Logf("Sources after removal:\n%s", listSourcesAfter)

	// Verify the command is gone by trying to show it (should fail)
	showAfterOutput, err := runAimgr(t, "repo", "show", "command/meta-remove-test")
	if err == nil {
		t.Errorf("Command should not exist after removal. Show output:\n%s", showAfterOutput)
	}
}
