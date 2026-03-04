//go:build integration

package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/pkg/config"
	"gopkg.in/yaml.v3"
)

// TestRepoSyncIdempotency_CLI tests that running `aimgr repo sync` twice on the same source
// should be idempotent - the second sync should succeed without errors.
//
// This test executes the ACTUAL CLI command (not direct API calls) to match real-world behavior.
//
// Current behavior (BUG TO INVESTIGATE):
// - Expected: All resource types should succeed on second sync
// - Observed in manual testing: Commands fail on second sync with "already exists" error
//
// This test will reveal the ACTUAL behavior when using the CLI command.
func TestRepoSyncIdempotency_CLI(t *testing.T) {
	// Setup test environment
	testDir := t.TempDir()
	configDir := filepath.Join(testDir, "config")
	dataDir := filepath.Join(testDir, "data")
	sourceDir := filepath.Join(testDir, "source")

	// Create directories
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}

	// Setup test resources (reuse helpers from old test)
	setupAllResourcesForCLI(t, sourceDir)
	t.Logf("Created test resources in: %s", sourceDir)

	// Create minimal config file (only install.targets required)
	configContent := &config.Config{
		Install: config.InstallConfig{
			Targets: []string{"claude"},
		},
	}

	// Create aimgr subdirectory (expected by LoadGlobal)
	aimgrConfigDir := filepath.Join(configDir, "aimgr")
	if err := os.MkdirAll(aimgrConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create aimgr config dir: %v", err)
	}

	configPath := filepath.Join(aimgrConfigDir, "aimgr.yaml")
	configData, err := yaml.Marshal(configContent)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	t.Logf("Created config file: %s", configPath)

	// Build path to aimgr binary
	binPath := filepath.Join("..", "aimgr")
	repoDir := filepath.Join(dataDir, "repo")

	// Helper function to run aimgr commands
	runCommand := func(testName string, args ...string) (string, int) {
		t.Helper()
		t.Logf("[%s] Running: aimgr %s", testName, strings.Join(args, " "))

		cmd := exec.Command(binPath, args...)
		cmd.Env = append(os.Environ(),
			"XDG_CONFIG_HOME="+configDir,
			"XDG_DATA_HOME="+dataDir,
			"AIMGR_REPO_PATH="+repoDir,
		)

		output, err := cmd.CombinedOutput()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				t.Fatalf("[%s] Failed to execute command: %v", testName, err)
			}
		}

		t.Logf("[%s] Exit code: %d", testName, exitCode)
		t.Logf("[%s] Output:\n%s", testName, string(output))
		return string(output), exitCode
	}

	// STEP 1: Add source to repository (populates ai.repo.yaml)
	t.Log("========================================")
	t.Log("STEP 1: Adding source to repository...")
	t.Log("========================================")

	outputAdd, exitCodeAdd := runCommand("repo_add", "repo", "add", "local:"+sourceDir)
	if exitCodeAdd != 0 {
		t.Fatalf("Failed to add source with exit code %d\nOutput: %s", exitCodeAdd, outputAdd)
	}

	// STEP 2: First sync - should succeed
	t.Log("========================================")
	t.Log("STEP 2: FIRST SYNC (should be no-op)...")
	t.Log("========================================")

	output1, exitCode1 := runCommand("first_sync", "repo", "sync")

	// Verify first sync succeeded
	if exitCode1 != 0 {
		t.Fatalf("First sync failed with exit code %d\nOutput: %s", exitCode1, output1)
	}

	// STEP 3: Second sync - should also succeed (idempotent)
	t.Log("========================================")
	t.Log("STEP 3: SECOND SYNC (should be idempotent)...")
	t.Log("========================================")

	output2, exitCode2 := runCommand("second_sync", "repo", "sync")

	// Analyze second sync results
	t.Log("========================================")
	t.Log("ANALYSIS:")
	t.Log("========================================")

	if exitCode2 != 0 {
		t.Errorf("✗ Second sync FAILED with exit code %d", exitCode2)
		t.Errorf("This confirms the BUG - repo sync is NOT idempotent")
		t.Logf("\nOutput:\n%s", output2)

		// Check which resource types failed
		if strings.Contains(output2, "already exists") {
			t.Logf("\nFound 'already exists' errors:")
			lines := strings.Split(output2, "\n")
			for _, line := range lines {
				if strings.Contains(line, "already exists") {
					t.Logf("  - %s", line)
				}
			}
		}
	} else {
		t.Logf("✓ Second sync SUCCEEDED with exit code 0")
		t.Logf("Output:\n%s", output2)

		// Check if it reports correct status (should not say "Added")
		if strings.Contains(output2, "added") {
			t.Logf("\n⚠ WARNING: Second sync reports resources as 'added'")
			t.Logf("This is misleading - they should be 'skipped' or 'updated'")
		}
	}

	// Verify repository integrity
	t.Log("========================================")
	t.Log("VERIFYING REPOSITORY:")
	t.Log("========================================")

	repoPath := filepath.Join(dataDir, "repo")
	verifyResourcesExist(t, repoPath)
}

// TestRepoSyncIdempotency_CLI_SkipExisting tests that `aimgr repo sync --skip-existing`
// correctly handles existing resources on second sync
func TestRepoSyncIdempotency_CLI_SkipExisting(t *testing.T) {
	// Setup test environment
	testDir := t.TempDir()
	configDir := filepath.Join(testDir, "config")
	dataDir := filepath.Join(testDir, "data")
	sourceDir := filepath.Join(testDir, "source")

	// Create directories
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}

	// Setup test resources
	setupAllResourcesForCLI(t, sourceDir)
	t.Logf("Created test resources in: %s", sourceDir)

	// Create minimal config file (only install.targets required)
	configContent := &config.Config{
		Install: config.InstallConfig{
			Targets: []string{"claude"},
		},
	}

	// Create aimgr subdirectory (expected by LoadGlobal)
	aimgrConfigDir := filepath.Join(configDir, "aimgr")
	if err := os.MkdirAll(aimgrConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create aimgr config dir: %v", err)
	}

	configPath := filepath.Join(aimgrConfigDir, "aimgr.yaml")
	configData, err := yaml.Marshal(configContent)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	t.Logf("Created config file: %s", configPath)

	// Build path to aimgr binary
	binPath := filepath.Join("..", "aimgr")
	repoDir := filepath.Join(dataDir, "repo")

	// Helper function to run aimgr commands
	runCommand := func(testName string, args ...string) (string, int) {
		t.Helper()
		t.Logf("[%s] Running: aimgr %s", testName, strings.Join(args, " "))

		cmd := exec.Command(binPath, args...)
		cmd.Env = append(os.Environ(),
			"XDG_CONFIG_HOME="+configDir,
			"XDG_DATA_HOME="+dataDir,
			"AIMGR_REPO_PATH="+repoDir,
		)

		output, err := cmd.CombinedOutput()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				t.Fatalf("[%s] Failed to execute command: %v", testName, err)
			}
		}

		t.Logf("[%s] Exit code: %d", testName, exitCode)
		t.Logf("[%s] Output:\n%s", testName, string(output))
		return string(output), exitCode
	}

	// STEP 1: Add source to repository (populates ai.repo.yaml)
	t.Log("========================================")
	t.Log("STEP 1: Adding source to repository...")
	t.Log("========================================")

	outputAdd, exitCodeAdd := runCommand("repo_add", "repo", "add", "local:"+sourceDir)
	if exitCodeAdd != 0 {
		t.Fatalf("Failed to add source with exit code %d\nOutput: %s", exitCodeAdd, outputAdd)
	}

	// STEP 2: First sync - should succeed
	t.Log("========================================")
	t.Log("STEP 2: FIRST SYNC (should be no-op)...")
	t.Log("========================================")

	output1, exitCode1 := runCommand("first_sync", "repo", "sync")

	// Verify first sync succeeded
	if exitCode1 != 0 {
		t.Fatalf("First sync failed with exit code %d\nOutput: %s", exitCode1, output1)
	}

	// STEP 3: Second sync with --skip-existing - should also succeed
	t.Log("========================================")
	t.Log("STEP 3: SECOND SYNC with --skip-existing...")
	t.Log("========================================")

	output2, exitCode2 := runCommand("second_sync", "repo", "sync", "--skip-existing")

	// Verify second sync succeeded
	if exitCode2 != 0 {
		t.Errorf("Second sync with --skip-existing failed with exit code %d", exitCode2)
		t.Logf("Output:\n%s", output2)
	} else {
		t.Logf("✓ Second sync with --skip-existing succeeded")

		// Check if it reports resources as skipped
		if !strings.Contains(output2, "skipped") {
			t.Logf("⚠ WARNING: Output doesn't mention 'skipped' resources")
		}
	}

	// Verify repository integrity
	repoPath := filepath.Join(dataDir, "repo")
	verifyResourcesExist(t, repoPath)
}

// Helper functions

// setupAllResourcesForCLI creates test resources (simplified version)
func setupAllResourcesForCLI(t *testing.T, baseDir string) {
	t.Helper()

	// 1. Flat command
	commandsDir := filepath.Join(baseDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}
	cmdPath := filepath.Join(commandsDir, "flat-test.md")
	cmdContent := `---
description: Flat test command for sync idempotency
---
# flat-test
This is a flat test command for verifying sync idempotency.
`
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create flat command file: %v", err)
	}
	t.Logf("Created flat command: %s", cmdPath)

	// 2. Nested command
	nestedDir := filepath.Join(baseDir, "commands", "mydir")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested commands directory: %v", err)
	}
	nestedPath := filepath.Join(nestedDir, "nested-test.md")
	nestedContent := `---
description: Nested test command for sync idempotency
---
# nested-test
This is a nested test command for verifying sync idempotency.
`
	if err := os.WriteFile(nestedPath, []byte(nestedContent), 0644); err != nil {
		t.Fatalf("Failed to create nested command file: %v", err)
	}
	t.Logf("Created nested command: %s", nestedPath)

	// 3. Skill
	skillDir := filepath.Join(baseDir, "skills", "test-sync-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: test-sync-skill
description: Test skill for sync idempotency
license: MIT
---
# test-sync-skill
This is a test skill for verifying sync idempotency.
`
	if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}
	t.Logf("Created skill: %s", skillDir)

	// 4. Agent
	agentsDir := filepath.Join(baseDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents directory: %v", err)
	}
	agentPath := filepath.Join(agentsDir, "test-sync-agent.md")
	agentContent := `---
description: Test agent for sync idempotency
---
# test-sync-agent
This is a test agent for verifying sync idempotency.
`
	if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to create agent file: %v", err)
	}
	t.Logf("Created agent: %s", agentPath)

	// 5. Package
	packagesDir := filepath.Join(baseDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}
	packagePath := filepath.Join(packagesDir, "test-sync-package.package.json")
	packageContent := `{
  "name": "test-sync-package",
  "description": "Test package for sync idempotency",
  "resources": [
    "command/flat-test"
  ]
}
`
	if err := os.WriteFile(packagePath, []byte(packageContent), 0644); err != nil {
		t.Fatalf("Failed to create package file: %v", err)
	}
	t.Logf("Created package: %s", packagePath)
}

// verifyResourcesExist checks that all resources exist in the repository
func verifyResourcesExist(t *testing.T, repoPath string) {
	t.Helper()

	checks := []struct {
		name string
		path string
	}{
		{"flat command", filepath.Join(repoPath, "commands", "flat-test.md")},
		{"nested command", filepath.Join(repoPath, "commands", "mydir", "nested-test.md")},
		{"skill", filepath.Join(repoPath, "skills", "test-sync-skill", "SKILL.md")},
		{"agent", filepath.Join(repoPath, "agents", "test-sync-agent.md")},
		{"package", filepath.Join(repoPath, "packages", "test-sync-package.package.json")},
	}

	for _, check := range checks {
		if _, err := os.Stat(check.path); err != nil {
			if os.IsNotExist(err) {
				t.Errorf("%s not found in repo: %s", check.name, check.path)
			} else {
				t.Errorf("Error checking %s: %v", check.name, err)
			}
		} else {
			t.Logf("✓ %s exists in repo", check.name)
		}
	}
}
