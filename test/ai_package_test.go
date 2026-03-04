package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/pkg/repo"
)

// TestZeroArgInstall tests 'aimgr install' with ai.package.yaml
func TestZeroArgInstall(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create .claude directory to trigger detection
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create test resources using helper functions
	cmdPath := createTestCommand(t, "test-cmd", "Test command for zero-arg install")
	skillDir := createTestSkill(t, "test-skill", "Test skill for zero-arg install")

	// Add resources to repository
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	_, err = runAimgr(t, "repo", "add", "--force", "local:"+skillDir)
	if err != nil {
		t.Fatalf("Failed to add skill: %v", err)
	}

	// Create ai.package.yaml
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	manifestContent := `resources:
  - command/test-cmd
  - skill/test-skill
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Test: aimgr install (zero-arg)
	output, err := runAimgr(t, "install", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install from manifest: %v\nOutput: %s", err, output)
	}

	// Verify output mentions both resources
	if !strings.Contains(output, "test-cmd") {
		t.Errorf("Output should mention test-cmd, got: %s", output)
	}
	if !strings.Contains(output, "test-skill") {
		t.Errorf("Output should mention test-skill, got: %s", output)
	}

	// Verify symlinks were created
	cmdSymlink := filepath.Join(projectDir, ".claude", "commands", "test-cmd.md")
	if _, err := os.Lstat(cmdSymlink); err != nil {
		t.Errorf("Command symlink not created: %v", err)
	}

	skillSymlink := filepath.Join(projectDir, ".claude", "skills", "test-skill")
	if _, err := os.Lstat(skillSymlink); err != nil {
		t.Errorf("Skill symlink not created: %v", err)
	}
}

// TestSaveOnInstall tests that installing a resource updates ai.package.yaml
func TestSaveOnInstall(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create .claude directory
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create test command using helper function
	cmdPath := createTestCommand(t, "save-test", "Test command for save functionality")

	// Add to repository
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Verify ai.package.yaml doesn't exist yet
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	if _, err := os.Stat(manifestPath); err == nil {
		t.Fatal("Manifest should not exist before install")
	}

	// Test: Install resource (should auto-save to manifest)
	output, err := runAimgr(t, "install", "command/save-test", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install command: %v\nOutput: %s", err, output)
	}

	// Verify ai.package.yaml was created
	if _, err := os.Stat(manifestPath); err != nil {
		t.Errorf("Manifest should be created after install: %v", err)
	}

	// Verify manifest contains the resource
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	if !m.Has("command/save-test") {
		t.Errorf("Manifest should contain command/save-test, got: %v", m.Resources)
	}

	// Install another resource using helper function
	skillDir := createTestSkill(t, "save-skill", "Test skill for save functionality")

	_, err = runAimgr(t, "repo", "add", "--force", "local:"+skillDir)
	if err != nil {
		t.Fatalf("Failed to add skill: %v", err)
	}

	output, err = runAimgr(t, "install", "skill/save-skill", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install skill: %v\nOutput: %s", err, output)
	}

	// Reload manifest and verify both resources
	m, err = manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}

	if !m.Has("command/save-test") {
		t.Errorf("Manifest should still contain command/save-test")
	}
	if !m.Has("skill/save-skill") {
		t.Errorf("Manifest should contain skill/save-skill")
	}
	if len(m.Resources) != 2 {
		t.Errorf("Expected 2 resources in manifest, got %d: %v", len(m.Resources), m.Resources)
	}
}

// TestNoSaveFlag tests that --no-save skips yaml update
func TestNoSaveFlag(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create .claude directory
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create test command using helper function
	cmdPath := createTestCommand(t, "no-save-test", "Test command for no-save flag")

	// Add to repository
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Test: Install with --no-save flag
	output, err := runAimgr(t, "install", "command/no-save-test", "--project-path", projectDir, "--no-save")
	if err != nil {
		t.Fatalf("Failed to install command: %v\nOutput: %s", err, output)
	}

	// Verify resource was installed
	cmdSymlink := filepath.Join(projectDir, ".claude", "commands", "no-save-test.md")
	if _, err := os.Lstat(cmdSymlink); err != nil {
		t.Errorf("Command symlink should be created: %v", err)
	}

	// Verify ai.package.yaml was NOT created
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	if _, err := os.Stat(manifestPath); err == nil {
		t.Error("Manifest should not be created with --no-save flag")
	}
}

// TestNoSaveFlagWithExistingManifest tests --no-save doesn't modify existing manifest
func TestNoSaveFlagWithExistingManifest(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create .claude directory
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create existing manifest with one resource
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	manifestContent := `resources:
  - command/existing
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Create test commands using helper functions
	cmd1Path := createTestCommand(t, "existing", "Existing command")
	cmd2Path := createTestCommand(t, "new-cmd", "New command")

	// Add to repository
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmd1Path)
	if err != nil {
		t.Fatalf("Failed to add existing command: %v", err)
	}
	_, err = runAimgr(t, "repo", "add", "--force", "local:"+cmd2Path)
	if err != nil {
		t.Fatalf("Failed to add new command: %v", err)
	}

	// Test: Install with --no-save flag
	output, err := runAimgr(t, "install", "command/new-cmd", "--project-path", projectDir, "--no-save")
	if err != nil {
		t.Fatalf("Failed to install command: %v\nOutput: %s", err, output)
	}

	// Verify new command was installed
	cmdSymlink := filepath.Join(projectDir, ".claude", "commands", "new-cmd.md")
	if _, err := os.Lstat(cmdSymlink); err != nil {
		t.Errorf("Command symlink should be created: %v", err)
	}

	// Verify manifest was NOT modified
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	if m.Has("command/new-cmd") {
		t.Error("Manifest should not contain new-cmd with --no-save flag")
	}
	if !m.Has("command/existing") {
		t.Error("Manifest should still contain existing command")
	}
	if len(m.Resources) != 1 {
		t.Errorf("Expected 1 resource in manifest, got %d", len(m.Resources))
	}
}

// TestInvalidManifest tests error handling for invalid YAML
func TestInvalidManifest(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create .claude directory
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	t.Run("invalid yaml syntax", func(t *testing.T) {
		manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
		invalidContent := `resources:
  - command/test
    invalid: yaml: syntax:
`
		if err := os.WriteFile(manifestPath, []byte(invalidContent), 0644); err != nil {
			t.Fatalf("Failed to create invalid manifest: %v", err)
		}

		// Test: Install should fail with YAML error
		output, err := runAimgr(t, "install", "--project-path", projectDir)
		if err == nil {
			t.Error("Install should fail with invalid YAML syntax")
		}

		if !strings.Contains(strings.ToLower(output), "yaml") && !strings.Contains(strings.ToLower(output), "parse") {
			t.Errorf("Error should mention YAML/parse issue, got: %s", output)
		}
	})

	t.Run("invalid resource format", func(t *testing.T) {
		manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
		invalidContent := `resources:
  - invalid-format
  - also/invalid/format
`
		if err := os.WriteFile(manifestPath, []byte(invalidContent), 0644); err != nil {
			t.Fatalf("Failed to create manifest: %v", err)
		}

		// Test: Install should fail with validation error
		output, err := runAimgr(t, "install", "--project-path", projectDir)
		if err == nil {
			t.Error("Install should fail with invalid resource format")
		}

		// Should mention validation or format error
		if !strings.Contains(strings.ToLower(output), "invalid") {
			t.Errorf("Error should mention invalid format, got: %s", output)
		}
	})

	t.Run("invalid resource type", func(t *testing.T) {
		manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
		invalidContent := `resources:
  - invalid-type/test-name
`
		if err := os.WriteFile(manifestPath, []byte(invalidContent), 0644); err != nil {
			t.Fatalf("Failed to create manifest: %v", err)
		}

		// Test: Install should fail with validation error
		output, err := runAimgr(t, "install", "--project-path", projectDir)
		if err == nil {
			t.Error("Install should fail with invalid resource type")
		}

		if !strings.Contains(strings.ToLower(output), "invalid") {
			t.Errorf("Error should mention invalid resource type, got: %s", output)
		}
	})
}

// TestMissingResources tests error handling when manifest references missing resources
func TestMissingResources(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create .claude directory
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create one real resource using helper function
	cmdPath := createTestCommand(t, "real-cmd", "Real command that exists")

	// Add to repository
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Create manifest with both real and missing resources
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	manifestContent := `resources:
  - command/real-cmd
  - command/missing-cmd
  - skill/missing-skill
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Test: Install should fail but mention which resources failed
	output, err := runAimgr(t, "install", "--project-path", projectDir)
	if err == nil {
		t.Error("Install should fail when resources are missing")
	}

	// Verify output mentions missing resources
	if !strings.Contains(output, "missing-cmd") {
		t.Errorf("Output should mention missing-cmd, got: %s", output)
	}
	if !strings.Contains(output, "missing-skill") {
		t.Errorf("Output should mention missing-skill, got: %s", output)
	}

	// Verify the real resource was installed despite failures
	cmdSymlink := filepath.Join(projectDir, ".claude", "commands", "real-cmd.md")
	if _, err := os.Lstat(cmdSymlink); err != nil {
		t.Errorf("Real command should be installed: %v", err)
	}
}

// TestBackwardCompatibility tests that aimgr works without ai.package.yaml
func TestBackwardCompatibility(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create .claude directory
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create test command using helper function
	cmdPath := createTestCommand(t, "compat-test", "Test command for backward compatibility")

	// Add to repository
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	t.Run("install without manifest using --no-save", func(t *testing.T) {
		// Test: Install resource with --no-save (no manifest interaction)
		output, err := runAimgr(t, "install", "command/compat-test", "--project-path", projectDir, "--no-save")
		if err != nil {
			t.Fatalf("Failed to install command: %v\nOutput: %s", err, output)
		}

		// Verify resource was installed
		cmdSymlink := filepath.Join(projectDir, ".claude", "commands", "compat-test.md")
		if _, err := os.Lstat(cmdSymlink); err != nil {
			t.Errorf("Command symlink should be created: %v", err)
		}

		// Verify no manifest was created
		manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
		if _, err := os.Stat(manifestPath); err == nil {
			t.Error("Manifest should not be created with --no-save")
		}

		// Clean up for next test
		if err := os.Remove(cmdSymlink); err != nil {
			t.Fatalf("Failed to remove symlink: %v", err)
		}
	})

	t.Run("zero-arg install without manifest shows helpful error", func(t *testing.T) {
		// Test: Zero-arg install without manifest should fail with helpful message
		output, err := runAimgr(t, "install", "--project-path", projectDir)
		if err == nil {
			t.Error("Zero-arg install should fail when manifest doesn't exist")
		}

		// Verify error message is helpful
		if !strings.Contains(output, manifest.ManifestFileName) {
			t.Errorf("Error should mention manifest file, got: %s", output)
		}
		if !strings.Contains(output, "not found") || !strings.Contains(output, "no resources") {
			t.Errorf("Error should explain manifest not found, got: %s", output)
		}
	})
}

// TestEmptyManifest tests behavior with empty ai.package.yaml
func TestEmptyManifest(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create .claude directory
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	t.Run("empty resources array", func(t *testing.T) {
		manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
		emptyContent := `resources: []
`
		if err := os.WriteFile(manifestPath, []byte(emptyContent), 0644); err != nil {
			t.Fatalf("Failed to create empty manifest: %v", err)
		}

		// Test: Install with empty manifest should succeed but do nothing
		output, err := runAimgr(t, "install", "--project-path", projectDir)
		if err != nil {
			t.Errorf("Install with empty manifest should succeed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(output, "No resources") {
			t.Errorf("Output should indicate no resources, got: %s", output)
		}
	})

	t.Run("completely empty file", func(t *testing.T) {
		manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
		if err := os.WriteFile(manifestPath, []byte(""), 0644); err != nil {
			t.Fatalf("Failed to create empty manifest: %v", err)
		}

		// Test: Install with empty file should handle gracefully
		output, err := runAimgr(t, "install", "--project-path", projectDir)
		// Should either succeed with no resources or fail with parse error
		// Both are acceptable behaviors
		if err == nil {
			// If it succeeds, should indicate no resources
			if !strings.Contains(output, "No resources") && !strings.Contains(output, "0") {
				t.Errorf("Output should indicate no resources, got: %s", output)
			}
		}
	})
}

// TestManifestWithTargets tests manifest with custom targets
func TestManifestWithTargets(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create both .claude and .opencode directories
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}
	opencodeDir := filepath.Join(projectDir, ".opencode")
	if err := os.MkdirAll(opencodeDir, 0755); err != nil {
		t.Fatalf("Failed to create .opencode directory: %v", err)
	}

	// Create test command using helper function
	cmdPath := createTestCommand(t, "target-test", "Test command for target functionality")

	// Add to repository
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Create manifest with targets specified (only claude)
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	manifestContent := `resources:
  - command/target-test
targets:
  - claude
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Test: Install should only install to claude (not opencode)
	output, err := runAimgr(t, "install", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install from manifest: %v\nOutput: %s", err, output)
	}

	// Verify installed to .claude
	claudeSymlink := filepath.Join(projectDir, ".claude", "commands", "target-test.md")
	if _, err := os.Lstat(claudeSymlink); err != nil {
		t.Errorf("Command should be installed to .claude: %v", err)
	}

	// Verify NOT installed to .opencode
	opencodeSymlink := filepath.Join(projectDir, ".opencode", "commands", "target-test.md")
	if _, err := os.Lstat(opencodeSymlink); err == nil {
		t.Error("Command should not be installed to .opencode (targets only specified claude)")
	}
}

// TestInstallFromManifestVsArgs tests zero-arg doesn't modify manifest
func TestInstallFromManifestVsArgs(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create .claude directory
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create test commands using helper functions
	cmd1Path := createTestCommand(t, "cmd1", "Command 1")
	cmd2Path := createTestCommand(t, "cmd2", "Command 2")

	// Add to repository
	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmd1Path)
	if err != nil {
		t.Fatalf("Failed to add command 1: %v", err)
	}
	_, err = runAimgr(t, "repo", "add", "--force", "local:"+cmd2Path)
	if err != nil {
		t.Fatalf("Failed to add command 2: %v", err)
	}

	// Create manifest with cmd1
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	manifestContent := `resources:
  - command/cmd1
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	// Test: Zero-arg install from manifest
	output, err := runAimgr(t, "install", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install from manifest: %v\nOutput: %s", err, output)
	}

	// Verify cmd1 was installed
	cmd1Symlink := filepath.Join(projectDir, ".claude", "commands", "cmd1.md")
	if _, err := os.Lstat(cmd1Symlink); err != nil {
		t.Errorf("Command 1 should be installed: %v", err)
	}

	// Verify manifest wasn't modified (should still only have cmd1)
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	if len(m.Resources) != 1 {
		t.Errorf("Manifest should still have 1 resource after zero-arg install, got %d", len(m.Resources))
	}
	if !m.Has("command/cmd1") {
		t.Error("Manifest should still contain cmd1")
	}

	// Now install cmd2 explicitly (not from manifest)
	output, err = runAimgr(t, "install", "command/cmd2", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install cmd2: %v\nOutput: %s", err, output)
	}

	// Verify manifest WAS updated to include cmd2
	m, err = manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}

	if len(m.Resources) != 2 {
		t.Errorf("Manifest should have 2 resources after explicit install, got %d", len(m.Resources))
	}
	if !m.Has("command/cmd1") {
		t.Error("Manifest should still contain cmd1")
	}
	if !m.Has("command/cmd2") {
		t.Error("Manifest should now contain cmd2")
	}
}

// TestManifestPersistence tests manifest survives multiple operations
func TestManifestPersistence(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create .claude directory
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create multiple test resources using helper functions
	resources := []string{"cmd1", "cmd2", "cmd3"}
	for _, name := range resources {
		cmdPath := createTestCommand(t, name, name)
		_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
		if err != nil {
			t.Fatalf("Failed to add %s: %v", name, err)
		}
	}

	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)

	// Install cmd1 - should create manifest
	_, err := runAimgr(t, "install", "command/cmd1", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install cmd1: %v", err)
	}

	// Verify manifest created
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatal("Manifest should be created")
	}

	// Install cmd2 - should update manifest
	_, err = runAimgr(t, "install", "command/cmd2", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install cmd2: %v", err)
	}

	// Install cmd3 - should update manifest again
	_, err = runAimgr(t, "install", "command/cmd3", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Failed to install cmd3: %v", err)
	}

	// Verify manifest contains all three
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	if len(m.Resources) != 3 {
		t.Errorf("Expected 3 resources, got %d", len(m.Resources))
	}
	for _, name := range resources {
		ref := "command/" + name
		if !m.Has(ref) {
			t.Errorf("Manifest should contain %s", ref)
		}
	}

	// Verify manifest is still valid YAML
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	// Should be able to parse it
	_, err = manifest.Load(manifestPath)
	if err != nil {
		t.Errorf("Manifest should still be valid YAML: %v\nContent: %s", err, string(data))
	}
}

// TestManifestErrorRecovery tests that manifest errors don't break normal installs
func TestManifestErrorRecovery(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create .claude directory
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create test command using helper function
	cmdPath := createTestCommand(t, "recovery-test", "Test command")

	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Create a read-only manifest directory (simulates permission error)
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	if err := os.WriteFile(manifestPath, []byte("resources: []\n"), 0444); err != nil {
		t.Fatalf("Failed to create read-only manifest: %v", err)
	}

	// Make manifest read-only
	if err := os.Chmod(manifestPath, 0444); err != nil {
		t.Fatalf("Failed to make manifest read-only: %v", err)
	}
	defer os.Chmod(manifestPath, 0644) // Clean up

	// Test: Install should succeed even if manifest update fails
	output, err := runAimgr(t, "install", "command/recovery-test", "--project-path", projectDir)
	if err != nil {
		t.Fatalf("Install should succeed even with manifest error: %v\nOutput: %s", err, output)
	}

	// Verify command was still installed
	cmdSymlink := filepath.Join(projectDir, ".claude", "commands", "recovery-test.md")
	if _, err := os.Lstat(cmdSymlink); err != nil {
		t.Errorf("Command should be installed despite manifest error: %v", err)
	}

	// Output should mention warning about manifest
	if !strings.Contains(strings.ToLower(output), "warn") {
		t.Logf("Warning: Expected warning about manifest update failure, got: %s", output)
	}
}

// TestPackageResourceValidation tests manifest validation for resource references
func TestPackageResourceValidation(t *testing.T) {
	// Use programmatic API to test validation directly
	manager := repo.NewManagerWithPath(t.TempDir())

	t.Run("valid resource references", func(t *testing.T) {
		m := &manifest.Manifest{
			Resources: []string{
				"command/test",
				"skill/pdf-processing",
				"agent/code-reviewer",
				"package/web-tools",
				"command/nested/path",           // Nested commands are valid
				"command/api/deploy",            // Nested commands are valid
				"command/opencode-coder/doctor", // Nested commands are valid
			},
		}

		if err := m.Validate(); err != nil {
			t.Errorf("Valid manifest should pass validation: %v", err)
		}
	})

	t.Run("invalid formats", func(t *testing.T) {
		invalidRefs := []string{
			"command",           // Missing name
			"command/",          // Empty name
			"test",              // Missing type
			"invalid-type/test", // Invalid type
			"",                  // Empty string
		}

		for _, ref := range invalidRefs {
			m := &manifest.Manifest{
				Resources: []string{ref},
			}

			if err := m.Validate(); err == nil {
				t.Errorf("Invalid reference %q should fail validation", ref)
			}
		}
	})

	t.Run("valid targets", func(t *testing.T) {
		m := &manifest.Manifest{
			Resources: []string{"command/test"},
			Targets:   []string{"claude", "opencode", "copilot"},
		}

		if err := m.Validate(); err != nil {
			t.Errorf("Valid targets should pass validation: %v", err)
		}
	})

	t.Run("invalid targets", func(t *testing.T) {
		m := &manifest.Manifest{
			Resources: []string{"command/test"},
			Targets:   []string{"invalid-tool"},
		}

		if err := m.Validate(); err == nil {
			t.Error("Invalid target should fail validation")
		}
	})

	// Prevent unused variable warning
	_ = manager
}
