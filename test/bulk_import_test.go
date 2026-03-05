package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// setupTestPlugin creates a test plugin structure
func setupTestPlugin(t *testing.T, pluginDir string) {
	// Create plugin metadata
	pluginJsonDir := filepath.Join(pluginDir, ".claude-plugin")
	if err := os.MkdirAll(pluginJsonDir, 0755); err != nil {
		t.Fatalf("Failed to create plugin metadata dir: %v", err)
	}

	pluginJson := []byte(`{
		"name": "test-plugin",
		"description": "A test plugin",
		"version": "1.0.0"
	}`)
	if err := os.WriteFile(filepath.Join(pluginJsonDir, "plugin.json"), pluginJson, 0644); err != nil {
		t.Fatalf("Failed to write plugin.json: %v", err)
	}

	// Create commands
	commandsDir := filepath.Join(pluginDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	cmd1 := []byte("---\ndescription: Test command 1\n---\nTest content")
	if err := os.WriteFile(filepath.Join(commandsDir, "test-cmd1.md"), cmd1, 0644); err != nil {
		t.Fatalf("Failed to write command 1: %v", err)
	}

	cmd2 := []byte("---\ndescription: Test command 2\n---\nTest content")
	if err := os.WriteFile(filepath.Join(commandsDir, "test-cmd2.md"), cmd2, 0644); err != nil {
		t.Fatalf("Failed to write command 2: %v", err)
	}

	// Create skills
	skillsDir := filepath.Join(pluginDir, "skills")
	skill1Dir := filepath.Join(skillsDir, "test-skill1")
	if err := os.MkdirAll(skill1Dir, 0755); err != nil {
		t.Fatalf("Failed to create skill1 dir: %v", err)
	}

	skill1 := []byte("---\nname: test-skill1\ndescription: Test skill 1\n---\nTest content")
	if err := os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), skill1, 0644); err != nil {
		t.Fatalf("Failed to write skill 1: %v", err)
	}
}

// setupTestClaudeFolder creates a test .claude folder structure
func setupTestClaudeFolder(t *testing.T, claudeDir string) {
	// Create commands
	commandsDir := filepath.Join(claudeDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	cmd1 := []byte("---\ndescription: Claude command 1\n---\nTest content")
	if err := os.WriteFile(filepath.Join(commandsDir, "claude-cmd1.md"), cmd1, 0644); err != nil {
		t.Fatalf("Failed to write command 1: %v", err)
	}

	// Create skills
	skillsDir := filepath.Join(claudeDir, "skills")
	skill1Dir := filepath.Join(skillsDir, "claude-skill1")
	if err := os.MkdirAll(skill1Dir, 0755); err != nil {
		t.Fatalf("Failed to create skill1 dir: %v", err)
	}

	skill1 := []byte("---\nname: claude-skill1\ndescription: Claude skill 1\n---\nTest content")
	if err := os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), skill1, 0644); err != nil {
		t.Fatalf("Failed to write skill 1: %v", err)
	}
}

func TestPluginImport(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	pluginDir := filepath.Join(tmpDir, "test-plugin")

	// Setup test plugin
	setupTestPlugin(t, pluginDir)

	// Create manager
	manager := repo.NewManagerWithPath(repoPath)

	// Scan plugin resources
	commandPaths, skillPaths, _, err := resource.ScanPluginResources(pluginDir)
	if err != nil {
		t.Fatalf("Failed to scan plugin: %v", err)
	}

	if len(commandPaths) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(commandPaths))
	}
	if len(skillPaths) != 1 {
		t.Errorf("Expected 1 skill, got %d", len(skillPaths))
	}

	// Import resources
	allPaths := append(commandPaths, skillPaths...)
	result, err := manager.AddBulk(allPaths, repo.BulkImportOptions{})
	if err != nil {
		t.Fatalf("Failed to import plugin: %v", err)
	}

	if len(result.Added) != 3 {
		t.Errorf("Expected 3 added resources, got %d", len(result.Added))
	}
	if len(result.Failed) != 0 {
		t.Errorf("Expected 0 failed resources, got %d", len(result.Failed))
	}

	// Verify resources are in repository
	resources, err := manager.List(nil)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}
	if len(resources) != 3 {
		t.Errorf("Expected 3 resources in repo, got %d", len(resources))
	}
}

func TestClaudeImport(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	claudeDir := filepath.Join(tmpDir, ".claude")

	// Setup test Claude folder
	setupTestClaudeFolder(t, claudeDir)

	// Create manager
	manager := repo.NewManagerWithPath(repoPath)

	// Scan Claude folder
	contents, err := resource.ScanClaudeFolder(claudeDir)
	if err != nil {
		t.Fatalf("Failed to scan Claude folder: %v", err)
	}

	if len(contents.CommandPaths) != 1 {
		t.Errorf("Expected 1 command, got %d", len(contents.CommandPaths))
	}
	if len(contents.SkillPaths) != 1 {
		t.Errorf("Expected 1 skill, got %d", len(contents.SkillPaths))
	}

	// Import resources
	allPaths := append(contents.CommandPaths, contents.SkillPaths...)
	result, err := manager.AddBulk(allPaths, repo.BulkImportOptions{})
	if err != nil {
		t.Fatalf("Failed to import Claude folder: %v", err)
	}

	if len(result.Added) != 2 {
		t.Errorf("Expected 2 added resources, got %d", len(result.Added))
	}

	// Verify resources are in repository
	resources, err := manager.List(nil)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}
	if len(resources) != 2 {
		t.Errorf("Expected 2 resources in repo, got %d", len(resources))
	}
}

func TestBulkImportConflicts(t *testing.T) {
	t.Run("force flag overwrites existing", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoPath := filepath.Join(tmpDir, "repo")
		manager := repo.NewManagerWithPath(repoPath)

		// Add first command in proper commands/ directory
		commandsDir := filepath.Join(tmpDir, "commands")
		os.MkdirAll(commandsDir, 0755)
		cmd1Path := filepath.Join(commandsDir, "conflict.md")
		os.WriteFile(cmd1Path, []byte("---\ndescription: Original\n---\n"), 0644)
		if err := manager.AddCommand(cmd1Path, "file://"+cmd1Path, "file"); err != nil {
			t.Fatalf("Failed to add first command: %v", err)
		}

		// Try to add with same name using force
		result, err := manager.AddBulk([]string{cmd1Path}, repo.BulkImportOptions{Force: true})
		if err != nil {
			t.Fatalf("Force import failed: %v", err)
		}

		if len(result.Updated) != 1 {
			t.Errorf("Expected 1 updated with force (not added), got %d", len(result.Updated))
		}
		if len(result.Added) != 0 {
			t.Errorf("Expected 0 added (should be updated instead), got %d", len(result.Added))
		}
	})

	t.Run("skip existing flag skips conflicts", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoPath := filepath.Join(tmpDir, "repo")
		manager := repo.NewManagerWithPath(repoPath)

		// Add first command in proper commands/ directory
		commandsDir := filepath.Join(tmpDir, "commands")
		os.MkdirAll(commandsDir, 0755)
		cmd1Path := filepath.Join(commandsDir, "skip.md")
		os.WriteFile(cmd1Path, []byte("---\ndescription: Existing\n---\n"), 0644)
		if err := manager.AddCommand(cmd1Path, "file://"+cmd1Path, "file"); err != nil {
			t.Fatalf("Failed to add first command: %v", err)
		}

		// Try to add with same name using skip-existing
		result, err := manager.AddBulk([]string{cmd1Path}, repo.BulkImportOptions{SkipExisting: true})
		if err != nil {
			t.Fatalf("Skip existing import failed: %v", err)
		}

		if len(result.Skipped) != 1 {
			t.Errorf("Expected 1 skipped, got %d", len(result.Skipped))
		}
	})

	t.Run("no flags fails on conflict", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoPath := filepath.Join(tmpDir, "repo")
		manager := repo.NewManagerWithPath(repoPath)

		// Add first command in proper commands/ directory
		commandsDir := filepath.Join(tmpDir, "commands")
		os.MkdirAll(commandsDir, 0755)
		cmd1Path := filepath.Join(commandsDir, "fail.md")
		os.WriteFile(cmd1Path, []byte("---\ndescription: Existing\n---\n"), 0644)
		if err := manager.AddCommand(cmd1Path, "file://"+cmd1Path, "file"); err != nil {
			t.Fatalf("Failed to add first command: %v", err)
		}

		// Try to add with same name without flags
		result, err := manager.AddBulk([]string{cmd1Path}, repo.BulkImportOptions{})
		if err != nil {
			t.Fatalf("AddBulk should not return error: %v", err)
		}
		if len(result.Failed) == 0 {
			t.Error("Expected conflict in result.Failed, got none")
		}
	})
}

func TestDryRunMode(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	manager := repo.NewManagerWithPath(repoPath)

	// Create test command in proper commands/ directory
	commandsDir := filepath.Join(tmpDir, "commands")
	os.MkdirAll(commandsDir, 0755)
	cmdPath := filepath.Join(commandsDir, "dryrun.md")
	os.WriteFile(cmdPath, []byte("---\ndescription: Dry run test\n---\n"), 0644)

	// Import with dry-run
	result, err := manager.AddBulk([]string{cmdPath}, repo.BulkImportOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Dry run import failed: %v", err)
	}

	if len(result.Added) != 1 {
		t.Errorf("Dry run should show 1 added, got %d", len(result.Added))
	}

	// Verify nothing was actually imported
	resources, err := manager.List(nil)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("Dry run should not import, but found %d resources", len(resources))
	}
}

func TestInvalidPaths(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	manager := repo.NewManagerWithPath(repoPath)

	t.Run("non-existent path fails", func(t *testing.T) {
		nonExistentPath := filepath.Join(tmpDir, "nonexistent.md")
		result, _ := manager.AddBulk([]string{nonExistentPath}, repo.BulkImportOptions{SkipExisting: true})

		// Should fail but with result showing the failure
		if len(result.Failed) != 1 {
			t.Errorf("Expected 1 failed, got %d", len(result.Failed))
		}
	})

	t.Run("invalid resource format fails", func(t *testing.T) {
		invalidPath := filepath.Join(tmpDir, "invalid.txt")
		os.WriteFile(invalidPath, []byte("not a resource"), 0644)

		result, _ := manager.AddBulk([]string{invalidPath}, repo.BulkImportOptions{SkipExisting: true})

		// Should fail but with result showing the failure
		if len(result.Failed) != 1 {
			t.Errorf("Expected 1 failed, got %d", len(result.Failed))
		}
	})
}
