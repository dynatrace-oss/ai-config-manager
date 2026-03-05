//go:build integration

package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/install"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

// TestAutoDetect_RepoImportDirectory tests importing directories with nested command structures
func TestAutoDetect_RepoImportDirectory(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)

	t.Run("import directory with nested commands", func(t *testing.T) {
		// Use the comprehensive fixture which has nested commands
		fixtureDir := filepath.Join("..", "testdata", "repos", "comprehensive-fixture")

		// Get the absolute path
		absFixtureDir, err := filepath.Abs(fixtureDir)
		if err != nil {
			t.Fatalf("Failed to get absolute path: %v", err)
		}

		// Import all paths from the fixture
		opts := repo.BulkImportOptions{
			Force:        false,
			SkipExisting: false,
			DryRun:       false,
		}

		commandsDir := filepath.Join(absFixtureDir, "commands")
		commandPaths := []string{
			filepath.Join(commandsDir, "test.md"),
			filepath.Join(commandsDir, "api", "status.md"),
			filepath.Join(commandsDir, "api", "deploy.md"),
			filepath.Join(commandsDir, "db", "migrate.md"),
		}

		result, err := manager.AddBulk(commandPaths, opts)
		if err != nil {
			t.Fatalf("Failed to import commands: %v", err)
		}

		// Verify all commands were imported
		if len(result.Added) != 4 {
			t.Errorf("Expected 4 commands imported, got %d", len(result.Added))
		}
		if len(result.Failed) != 0 {
			t.Errorf("Expected 0 failures, got %d: %v", len(result.Failed), result.Failed)
		}

		// Verify nested command names are preserved
		apiStatus, err := manager.Get("api/status", resource.Command)
		if err != nil {
			t.Errorf("Failed to get api/status command: %v", err)
		}
		if apiStatus.Name != "api/status" {
			t.Errorf("Command name = %v, want api/status", apiStatus.Name)
		}

		apiDeploy, err := manager.Get("api/deploy", resource.Command)
		if err != nil {
			t.Errorf("Failed to get api/deploy command: %v", err)
		}
		if apiDeploy.Name != "api/deploy" {
			t.Errorf("Command name = %v, want api/deploy", apiDeploy.Name)
		}

		dbMigrate, err := manager.Get("db/migrate", resource.Command)
		if err != nil {
			t.Errorf("Failed to get db/migrate command: %v", err)
		}
		if dbMigrate.Name != "db/migrate" {
			t.Errorf("Command name = %v, want db/migrate", dbMigrate.Name)
		}
	})

	t.Run("import .opencode directory with nested structure", func(t *testing.T) {
		// Create temporary .opencode structure
		opencodeDir := t.TempDir()
		commandsDir := filepath.Join(opencodeDir, ".opencode", "commands")
		if err := os.MkdirAll(filepath.Join(commandsDir, "tools"), 0755); err != nil {
			t.Fatalf("Failed to create directories: %v", err)
		}

		// Create nested command
		nestedCmdPath := filepath.Join(commandsDir, "tools", "build.md")
		cmdContent := `---
description: Build command in nested structure
---

# Build Command

Nested command for testing.
`
		if err := os.WriteFile(nestedCmdPath, []byte(cmdContent), 0644); err != nil {
			t.Fatalf("Failed to create test command: %v", err)
		}

		// Import the nested command
		opts := repo.BulkImportOptions{
			Force:        false,
			SkipExisting: false,
			DryRun:       false,
		}

		result, err := manager.AddBulk([]string{nestedCmdPath}, opts)
		if err != nil {
			t.Fatalf("Failed to import nested command: %v", err)
		}

		if len(result.Added) != 1 {
			t.Errorf("Expected 1 command added, got %d", len(result.Added))
		}

		// Verify nested name is preserved
		cmd, err := manager.Get("tools/build", resource.Command)
		if err != nil {
			t.Errorf("Failed to get tools/build command: %v", err)
		}
		if cmd.Name != "tools/build" {
			t.Errorf("Command name = %v, want tools/build", cmd.Name)
		}
	})

	t.Run("import .claude directory with nested structure", func(t *testing.T) {
		// Create temporary .claude structure
		claudeDir := t.TempDir()
		commandsDir := filepath.Join(claudeDir, ".claude", "commands")
		if err := os.MkdirAll(filepath.Join(commandsDir, "dev"), 0755); err != nil {
			t.Fatalf("Failed to create directories: %v", err)
		}

		// Create nested command
		nestedCmdPath := filepath.Join(commandsDir, "dev", "test.md")
		cmdContent := `---
description: Test command in .claude nested structure
---

# Test Command

Nested Claude command.
`
		if err := os.WriteFile(nestedCmdPath, []byte(cmdContent), 0644); err != nil {
			t.Fatalf("Failed to create test command: %v", err)
		}

		// Import the nested command
		opts := repo.BulkImportOptions{
			Force:        false,
			SkipExisting: false,
			DryRun:       false,
		}

		result, err := manager.AddBulk([]string{nestedCmdPath}, opts)
		if err != nil {
			t.Fatalf("Failed to import nested command: %v", err)
		}

		if len(result.Added) != 1 {
			t.Errorf("Expected 1 command added, got %d", len(result.Added))
		}

		// Verify nested name is preserved
		cmd, err := manager.Get("dev/test", resource.Command)
		if err != nil {
			t.Errorf("Failed to get dev/test command: %v", err)
		}
		if cmd.Name != "dev/test" {
			t.Errorf("Command name = %v, want dev/test", cmd.Name)
		}
	})
}

// TestAutoDetect_RepoImportSingleFile tests importing single files from various locations
func TestAutoDetect_RepoImportSingleFile(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)

	t.Run("import single file from commands/ directory", func(t *testing.T) {
		// Use fixture with proper structure
		fixtureDir := filepath.Join("..", "testdata", "repos", "commands-nested")
		absFixtureDir, err := filepath.Abs(fixtureDir)
		if err != nil {
			t.Fatalf("Failed to get absolute path: %v", err)
		}

		cmdPath := filepath.Join(absFixtureDir, "commands", "test.md")

		// Import single file
		opts := repo.BulkImportOptions{
			Force:        false,
			SkipExisting: false,
			DryRun:       false,
		}

		result, err := manager.AddBulk([]string{cmdPath}, opts)
		if err != nil {
			t.Fatalf("Failed to import command: %v", err)
		}

		if len(result.Added) != 1 {
			t.Errorf("Expected 1 command added, got %d", len(result.Added))
		}

		// Verify command was imported
		cmd, err := manager.Get("test", resource.Command)
		if err != nil {
			t.Errorf("Failed to get test command: %v", err)
		}
		if cmd.Name != "test" {
			t.Errorf("Command name = %v, want test", cmd.Name)
		}
	})

	t.Run("import single nested file", func(t *testing.T) {
		// Use fixture with nested structure
		fixtureDir := filepath.Join("..", "testdata", "repos", "commands-nested")
		absFixtureDir, err := filepath.Abs(fixtureDir)
		if err != nil {
			t.Fatalf("Failed to get absolute path: %v", err)
		}

		cmdPath := filepath.Join(absFixtureDir, "commands", "nested", "deploy.md")

		// Import single nested file
		opts := repo.BulkImportOptions{
			Force:        false,
			SkipExisting: false,
			DryRun:       false,
		}

		result, err := manager.AddBulk([]string{cmdPath}, opts)
		if err != nil {
			t.Fatalf("Failed to import nested command: %v", err)
		}

		if len(result.Added) != 1 {
			t.Errorf("Expected 1 command added, got %d", len(result.Added))
		}

		// Verify nested name is preserved
		cmd, err := manager.Get("nested/deploy", resource.Command)
		if err != nil {
			t.Errorf("Failed to get nested/deploy command: %v", err)
		}
		if cmd.Name != "nested/deploy" {
			t.Errorf("Command name = %v, want nested/deploy", cmd.Name)
		}
	})
}

// TestAutoDetect_ErrorWhenNotInCommandsDir tests error handling for files not in commands/ directory
func TestAutoDetect_ErrorWhenNotInCommandsDir(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)

	t.Run("error when importing from random location", func(t *testing.T) {
		// Create a command file in random location (not in commands/)
		tmpDir := t.TempDir()
		randomCmdPath := filepath.Join(tmpDir, "random.md")
		cmdContent := `---
description: Random command not in commands/ directory
---

# Random Command
`
		if err := os.WriteFile(randomCmdPath, []byte(cmdContent), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Try to import - should fail
		opts := repo.BulkImportOptions{
			Force:        false,
			SkipExisting: false,
			DryRun:       false,
		}

		result, err := manager.AddBulk([]string{randomCmdPath}, opts)
		if err != nil {
			t.Fatalf("AddBulk returned error: %v", err)
		}

		// Should have 0 successes and 1 failure
		if len(result.Added) != 0 {
			t.Errorf("Expected 0 commands added, got %d", len(result.Added))
		}
		if len(result.Failed) != 1 {
			t.Errorf("Expected 1 failure, got %d", len(result.Failed))
		}

		// Verify error message mentions commands/ directory
		if len(result.Failed) > 0 {
			errMsg := result.Failed[0].Message
			if !strings.Contains(errMsg, "commands/") {
				t.Errorf("Error message should mention 'commands/' directory, got: %s", errMsg)
			}
		}
	})
}

// TestAutoDetect_MetadataTracking tests that metadata correctly tracks nested commands
func TestAutoDetect_MetadataTracking(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)

	t.Run("metadata preserves nested paths", func(t *testing.T) {
		// Use comprehensive fixture
		fixtureDir := filepath.Join("..", "testdata", "repos", "comprehensive-fixture")
		absFixtureDir, err := filepath.Abs(fixtureDir)
		if err != nil {
			t.Fatalf("Failed to get absolute path: %v", err)
		}

		// Import nested commands
		commandsDir := filepath.Join(absFixtureDir, "commands")
		commandPaths := []string{
			filepath.Join(commandsDir, "api", "status.md"),
			filepath.Join(commandsDir, "db", "migrate.md"),
		}

		opts := repo.BulkImportOptions{
			Force:        false,
			SkipExisting: false,
			DryRun:       false,
		}

		result, err := manager.AddBulk(commandPaths, opts)
		if err != nil {
			t.Fatalf("Failed to import commands: %v", err)
		}

		if len(result.Added) != 2 {
			t.Errorf("Expected 2 commands added, got %d", len(result.Added))
		}

		// Check metadata file for api/status
		metadataPath := filepath.Join(repoDir, ".metadata", "commands", "api-status-metadata.json")
		if _, err := os.Stat(metadataPath); err != nil {
			t.Errorf("Metadata file not found at %s: %v", metadataPath, err)
		}

		// Load metadata and verify name
		meta, err := metadata.Load("api/status", resource.Command, repoDir)
		if err != nil {
			t.Errorf("Failed to load metadata: %v", err)
		}
		if meta.Name != "api/status" {
			t.Errorf("Metadata name = %v, want api/status", meta.Name)
		}

		// Check metadata file for db/migrate
		metadataPath = filepath.Join(repoDir, ".metadata", "commands", "db-migrate-metadata.json")
		if _, err := os.Stat(metadataPath); err != nil {
			t.Errorf("Metadata file not found at %s: %v", metadataPath, err)
		}

		// Load metadata and verify name
		meta, err = metadata.Load("db/migrate", resource.Command, repoDir)
		if err != nil {
			t.Errorf("Failed to load metadata: %v", err)
		}
		if meta.Name != "db/migrate" {
			t.Errorf("Metadata name = %v, want db/migrate", meta.Name)
		}
	})
}

// TestAutoDetect_Install tests installation workflow with nested commands
func TestAutoDetect_Install(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)
	projectDir := t.TempDir()

	t.Run("install nested command preserves nested structure", func(t *testing.T) {
		// Import nested command
		fixtureDir := filepath.Join("..", "testdata", "repos", "comprehensive-fixture")
		absFixtureDir, err := filepath.Abs(fixtureDir)
		if err != nil {
			t.Fatalf("Failed to get absolute path: %v", err)
		}

		cmdPath := filepath.Join(absFixtureDir, "commands", "api", "status.md")

		opts := repo.BulkImportOptions{
			Force:        false,
			SkipExisting: false,
			DryRun:       false,
		}

		_, err = manager.AddBulk([]string{cmdPath}, opts)
		if err != nil {
			t.Fatalf("Failed to import command: %v", err)
		}

		// Install to Claude
		installer, err := install.NewInstaller(projectDir, []tools.Tool{tools.Claude})
		if err != nil {
			t.Fatalf("Failed to create installer: %v", err)
		}

		// Install the nested command
		if err := installer.InstallCommand("api/status", manager); err != nil {
			t.Fatalf("Failed to install command: %v", err)
		}

		// Verify installed with nested structure
		// Expected: .claude/commands/api/status.md (nested structure preserved)
		installedPath := filepath.Join(projectDir, ".claude", "commands", "api", "status.md")
		if _, err := os.Lstat(installedPath); err != nil {
			t.Errorf("Command not installed at expected path %s: %v", installedPath, err)
		}

		// Verify it's a symlink
		linkTarget, err := os.Readlink(installedPath)
		if err != nil {
			t.Errorf("Installed file is not a symlink: %v", err)
		}

		// Verify symlink points to repository
		if !strings.Contains(linkTarget, "commands") {
			t.Errorf("Symlink target should be in commands/ directory, got: %s", linkTarget)
		}
	})

	t.Run("install multiple nested commands", func(t *testing.T) {
		// Import multiple nested commands
		fixtureDir := filepath.Join("..", "testdata", "repos", "comprehensive-fixture")
		absFixtureDir, err := filepath.Abs(fixtureDir)
		if err != nil {
			t.Fatalf("Failed to get absolute path: %v", err)
		}

		commandsDir := filepath.Join(absFixtureDir, "commands")
		commandPaths := []string{
			filepath.Join(commandsDir, "api", "deploy.md"),
			filepath.Join(commandsDir, "db", "migrate.md"),
		}

		opts := repo.BulkImportOptions{
			Force:        false,
			SkipExisting: false,
			DryRun:       false,
		}

		_, err = manager.AddBulk(commandPaths, opts)
		if err != nil {
			t.Fatalf("Failed to import commands: %v", err)
		}

		// Create new project dir for this test
		newProjectDir := t.TempDir()
		installer, err := install.NewInstaller(newProjectDir, []tools.Tool{tools.Claude})
		if err != nil {
			t.Fatalf("Failed to create installer: %v", err)
		}

		// Install both commands
		if err := installer.InstallCommand("api/deploy", manager); err != nil {
			t.Fatalf("Failed to install api/deploy: %v", err)
		}
		if err := installer.InstallCommand("db/migrate", manager); err != nil {
			t.Fatalf("Failed to install db/migrate: %v", err)
		}

		// Verify both installed with nested structure
		apiDeployPath := filepath.Join(newProjectDir, ".claude", "commands", "api", "deploy.md")
		if _, err := os.Lstat(apiDeployPath); err != nil {
			t.Errorf("api/deploy.md not installed: %v", err)
		}

		dbMigratePath := filepath.Join(newProjectDir, ".claude", "commands", "db", "migrate.md")
		if _, err := os.Lstat(dbMigratePath); err != nil {
			t.Errorf("db/migrate.md not installed: %v", err)
		}
	})
}

// TestAutoDetect_LoadCommand tests the LoadCommand function auto-detect behavior
func TestAutoDetect_LoadCommand(t *testing.T) {
	t.Run("load command from commands/ directory", func(t *testing.T) {
		fixtureDir := filepath.Join("..", "testdata", "repos", "commands-nested")
		absFixtureDir, err := filepath.Abs(fixtureDir)
		if err != nil {
			t.Fatalf("Failed to get absolute path: %v", err)
		}

		cmdPath := filepath.Join(absFixtureDir, "commands", "test.md")

		// Load command using auto-detect
		res, err := resource.LoadCommand(cmdPath)
		if err != nil {
			t.Fatalf("Failed to load command: %v", err)
		}

		if res.Name != "test" {
			t.Errorf("Command name = %v, want test", res.Name)
		}
	})

	t.Run("load nested command preserves path", func(t *testing.T) {
		fixtureDir := filepath.Join("..", "testdata", "repos", "comprehensive-fixture")
		absFixtureDir, err := filepath.Abs(fixtureDir)
		if err != nil {
			t.Fatalf("Failed to get absolute path: %v", err)
		}

		cmdPath := filepath.Join(absFixtureDir, "commands", "api", "status.md")

		// Load command using auto-detect
		res, err := resource.LoadCommand(cmdPath)
		if err != nil {
			t.Fatalf("Failed to load command: %v", err)
		}

		if res.Name != "api/status" {
			t.Errorf("Command name = %v, want api/status", res.Name)
		}
	})

	t.Run("load deeply nested command", func(t *testing.T) {
		fixtureDir := filepath.Join("..", "testdata", "repos", "comprehensive-fixture")
		absFixtureDir, err := filepath.Abs(fixtureDir)
		if err != nil {
			t.Fatalf("Failed to get absolute path: %v", err)
		}

		cmdPath := filepath.Join(absFixtureDir, "commands", "db", "migrate.md")

		// Load command using auto-detect
		res, err := resource.LoadCommand(cmdPath)
		if err != nil {
			t.Fatalf("Failed to load command: %v", err)
		}

		if res.Name != "db/migrate" {
			t.Errorf("Command name = %v, want db/migrate", res.Name)
		}
	})

	t.Run("error when not in commands/ directory", func(t *testing.T) {
		// Create a temporary file outside commands/ directory
		tmpDir := t.TempDir()
		cmdPath := filepath.Join(tmpDir, "test.md")
		cmdContent := `---
description: Test command
---

# Test
`
		if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Try to load - should fail with clear error
		_, err := resource.LoadCommand(cmdPath)
		if err == nil {
			t.Error("Expected error when loading command not in commands/ directory")
		}

		// Verify error message mentions commands/ directory
		if !strings.Contains(err.Error(), "commands/") {
			t.Errorf("Error should mention 'commands/' directory, got: %v", err)
		}
	})

	t.Run("load from .opencode/commands/", func(t *testing.T) {
		// Create temporary .opencode structure
		tmpDir := t.TempDir()
		commandsDir := filepath.Join(tmpDir, ".opencode", "commands")
		if err := os.MkdirAll(commandsDir, 0755); err != nil {
			t.Fatalf("Failed to create directories: %v", err)
		}

		cmdPath := filepath.Join(commandsDir, "opencode-test.md")
		cmdContent := `---
description: OpenCode test command
---

# OpenCode Test
`
		if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Load command - should succeed
		res, err := resource.LoadCommand(cmdPath)
		if err != nil {
			t.Fatalf("Failed to load command: %v", err)
		}

		if res.Name != "opencode-test" {
			t.Errorf("Command name = %v, want opencode-test", res.Name)
		}
	})

	t.Run("load from .claude/commands/", func(t *testing.T) {
		// Create temporary .claude structure
		tmpDir := t.TempDir()
		commandsDir := filepath.Join(tmpDir, ".claude", "commands")
		if err := os.MkdirAll(commandsDir, 0755); err != nil {
			t.Fatalf("Failed to create directories: %v", err)
		}

		cmdPath := filepath.Join(commandsDir, "claude-test.md")
		cmdContent := `---
description: Claude test command
---

# Claude Test
`
		if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Load command - should succeed
		res, err := resource.LoadCommand(cmdPath)
		if err != nil {
			t.Fatalf("Failed to load command: %v", err)
		}

		if res.Name != "claude-test" {
			t.Errorf("Command name = %v, want claude-test", res.Name)
		}
	})
}
