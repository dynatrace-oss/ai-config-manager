package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestManagerInit(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	err := manager.Init()
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify directories were created
	commandsPath := filepath.Join(tmpDir, "commands")
	if _, err := os.Stat(commandsPath); err != nil {
		t.Errorf("commands directory was not created: %v", err)
	}

	skillsPath := filepath.Join(tmpDir, "skills")
	if _, err := os.Stat(skillsPath); err != nil {
		t.Errorf("skills directory was not created: %v", err)
	}
}

func TestInitCreatesManifestInExistingRepo(t *testing.T) {
	tmpDir := t.TempDir()

	// Manually create repo structure without ai.repo.yaml (simulates existing repo before upgrade)
	commandsPath := filepath.Join(tmpDir, "commands")
	if err := os.MkdirAll(commandsPath, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	skillsPath := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsPath, 0755); err != nil {
		t.Fatalf("Failed to create skills directory: %v", err)
	}

	agentsPath := filepath.Join(tmpDir, "agents")
	if err := os.MkdirAll(agentsPath, 0755); err != nil {
		t.Fatalf("Failed to create agents directory: %v", err)
	}

	packagesPath := filepath.Join(tmpDir, "packages")
	if err := os.MkdirAll(packagesPath, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	// Initialize git repository
	gitCmd := exec.Command("git", "init")
	gitCmd.Dir = tmpDir
	if output, err := gitCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to initialize git: %v\nOutput: %s", err, output)
	}

	// Verify ai.repo.yaml does NOT exist yet
	manifestPath := filepath.Join(tmpDir, repomanifest.ManifestFileName)
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("ai.repo.yaml should not exist before Init(), but it does")
	}

	// Now call Init() on existing repo
	manager := NewManagerWithPath(tmpDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify ai.repo.yaml now exists
	if _, err := os.Stat(manifestPath); err != nil {
		t.Errorf("ai.repo.yaml should exist after Init(): %v", err)
	}

	// Verify manifest has empty sources
	manifest, err := repomanifest.Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	if manifest.Version != 1 {
		t.Errorf("manifest.Version = %v, want 1", manifest.Version)
	}

	if len(manifest.Sources) != 0 {
		t.Errorf("manifest.Sources length = %v, want 0 (empty sources)", len(manifest.Sources))
	}
}

func TestAddCommand(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create a test command
	testCmd := filepath.Join(tmpDir, "test-cmd.md")
	cmdContent := `---
description: A test command
---

# Test Command

This is a test command.
`
	if err := os.WriteFile(testCmd, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Add the command
	err := manager.AddCommand(testCmd, "file://"+testCmd, "file")
	if err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	// Verify command was added
	destPath := manager.GetPath("test-cmd", resource.Command)
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("Command was not added to repository: %v", err)
	}

	// Verify we can load it
	res, err := resource.LoadCommand(destPath)
	if err != nil {
		t.Errorf("Failed to load added command: %v", err)
	}
	if res.Name != "test-cmd" {
		t.Errorf("Command name = %v, want test-cmd", res.Name)
	}
}

func TestAddCommandConflict(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create a test command
	testCmd := filepath.Join(tmpDir, "test-cmd.md")
	cmdContent := `---
description: A test command
---

# Test Command
`
	if err := os.WriteFile(testCmd, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Add the command first time - should succeed
	if err := manager.AddCommand(testCmd, "file://"+testCmd, "file"); err != nil {
		t.Fatalf("First AddCommand() error = %v", err)
	}

	// Add the same command again - should succeed (overwrites)
	// Note: AddCommand() doesn't check for duplicates - it always overwrites
	// Use AddBulk() with SkipExisting or without Force for duplicate detection
	err := manager.AddCommand(testCmd, "file://"+testCmd, "file")
	if err != nil {
		t.Errorf("AddCommand() should succeed (overwrites existing), got error = %v", err)
	}
}

func TestAddSkill(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create a test skill
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: test-skill
description: A test skill
---

# Test Skill

This is a test skill.
`
	if err := os.WriteFile(skillMdPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	// Add the skill
	err := manager.AddSkill(skillDir, "file://"+skillDir, "file")
	if err != nil {
		t.Fatalf("AddSkill() error = %v", err)
	}

	// Verify skill was added
	destPath := manager.GetPath("test-skill", resource.Skill)
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("Skill was not added to repository: %v", err)
	}

	// Verify SKILL.md exists
	destSkillMd := filepath.Join(destPath, "SKILL.md")
	if _, err := os.Stat(destSkillMd); err != nil {
		t.Errorf("SKILL.md was not copied: %v", err)
	}

	// Verify we can load it
	res, err := resource.LoadSkill(destPath)
	if err != nil {
		t.Errorf("Failed to load added skill: %v", err)
	}
	if res.Name != "test-skill" {
		t.Errorf("Skill name = %v, want test-skill", res.Name)
	}
}

func TestAddSkillWithSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create a test skill with subdirectories
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0755); err != nil {
		t.Fatalf("Failed to create scripts directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0755); err != nil {
		t.Fatalf("Failed to create references directory: %v", err)
	}

	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: test-skill
description: A test skill with subdirectories
---

# Test Skill
`
	if err := os.WriteFile(skillMdPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	// Create test files in subdirectories
	scriptPath := filepath.Join(skillDir, "scripts", "test.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	refPath := filepath.Join(skillDir, "references", "README.md")
	if err := os.WriteFile(refPath, []byte("# Reference"), 0644); err != nil {
		t.Fatalf("Failed to create reference: %v", err)
	}

	// Add the skill
	err := manager.AddSkill(skillDir, "file://"+skillDir, "file")
	if err != nil {
		t.Fatalf("AddSkill() error = %v", err)
	}

	// Verify subdirectories were copied
	destPath := manager.GetPath("test-skill", resource.Skill)
	destScriptPath := filepath.Join(destPath, "scripts", "test.sh")
	if _, err := os.Stat(destScriptPath); err != nil {
		t.Errorf("Script was not copied: %v", err)
	}

	destRefPath := filepath.Join(destPath, "references", "README.md")
	if _, err := os.Stat(destRefPath); err != nil {
		t.Errorf("Reference was not copied: %v", err)
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Add a command
	testCmd := filepath.Join(tmpDir, "test-cmd.md")
	cmdContent := `---
description: A test command
---

# Test Command
`
	if err := os.WriteFile(testCmd, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}
	if err := manager.AddCommand(testCmd, "file://"+testCmd, "file"); err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	// Add a skill
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: test-skill
description: A test skill
---

# Test Skill
`
	if err := os.WriteFile(skillMdPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("AddSkill() error = %v", err)
	}

	// List all resources
	resources, err := manager.List(nil)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(resources) != 2 {
		t.Errorf("List() returned %d resources, want 2", len(resources))
	}

	// List only commands
	cmdType := resource.Command
	commands, err := manager.List(&cmdType)
	if err != nil {
		t.Fatalf("List(Command) error = %v", err)
	}
	if len(commands) != 1 {
		t.Errorf("List(Command) returned %d resources, want 1", len(commands))
	}
	if commands[0].Type != resource.Command {
		t.Errorf("List(Command) returned non-command resource")
	}

	// List only skills
	skillType := resource.Skill
	skills, err := manager.List(&skillType)
	if err != nil {
		t.Fatalf("List(Skill) error = %v", err)
	}
	if len(skills) != 1 {
		t.Errorf("List(Skill) returned %d resources, want 1", len(skills))
	}
	if skills[0].Type != resource.Skill {
		t.Errorf("List(Skill) returned non-skill resource")
	}
}

func TestGet(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Add a command
	testCmd := filepath.Join(tmpDir, "test-cmd.md")
	cmdContent := `---
description: A test command
---

# Test Command
`
	if err := os.WriteFile(testCmd, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}
	if err := manager.AddCommand(testCmd, "file://"+testCmd, "file"); err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	// Get the command
	res, err := manager.Get("test-cmd", resource.Command)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if res.Name != "test-cmd" {
		t.Errorf("Get() name = %v, want test-cmd", res.Name)
	}
	if res.Type != resource.Command {
		t.Errorf("Get() type = %v, want command", res.Type)
	}
}

func TestGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	_, err := manager.Get("nonexistent", resource.Command)
	if err == nil {
		t.Error("Get() expected error for nonexistent resource, got nil")
	}
}

func TestRemove(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Add a command
	testCmd := filepath.Join(tmpDir, "test-cmd.md")
	cmdContent := `---
description: A test command
---

# Test Command
`
	if err := os.WriteFile(testCmd, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}
	if err := manager.AddCommand(testCmd, "file://"+testCmd, "file"); err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	// Verify it exists
	destPath := manager.GetPath("test-cmd", resource.Command)
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("Command was not added: %v", err)
	}

	// Remove it
	err := manager.Remove("test-cmd", resource.Command)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(destPath); err == nil {
		t.Error("Command still exists after removal")
	}
}

func TestRemoveNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	err := manager.Remove("nonexistent", resource.Command)
	if err == nil {
		t.Error("Remove() expected error for nonexistent resource, got nil")
	}
}

func TestGetPath(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Test command path
	cmdPath := manager.GetPath("test-cmd", resource.Command)
	expectedCmdPath := filepath.Join(tmpDir, "commands", "test-cmd.md")
	if cmdPath != expectedCmdPath {
		t.Errorf("GetPath(command) = %v, want %v", cmdPath, expectedCmdPath)
	}

	// Test skill path
	skillPath := manager.GetPath("test-skill", resource.Skill)
	expectedSkillPath := filepath.Join(tmpDir, "skills", "test-skill")
	if skillPath != expectedSkillPath {
		t.Errorf("GetPath(skill) = %v, want %v", skillPath, expectedSkillPath)
	}
}

func TestAddBulk(t *testing.T) {
	tests := []struct {
		name             string
		setup            func(tmpDir string, manager *Manager) []string
		opts             BulkImportOptions
		wantAddedCount   int
		wantUpdatedCount int
		wantSkippedCount int
		wantFailedCount  int
		wantError        bool
	}{
		{
			name: "import multiple commands successfully",
			setup: func(tmpDir string, manager *Manager) []string {
				// Create commands in proper commands/ directory
				commandsDir := filepath.Join(tmpDir, "commands")
				os.MkdirAll(commandsDir, 0755)

				cmd1Path := filepath.Join(commandsDir, "cmd1.md")
				cmd2Path := filepath.Join(commandsDir, "cmd2.md")

				os.WriteFile(cmd1Path, []byte("---\ndescription: Command 1\n---\n"), 0644)
				os.WriteFile(cmd2Path, []byte("---\ndescription: Command 2\n---\n"), 0644)

				return []string{cmd1Path, cmd2Path}
			},
			opts:             BulkImportOptions{},
			wantAddedCount:   2,
			wantSkippedCount: 0,
			wantFailedCount:  0,
			wantError:        false,
		},
		{
			name: "import multiple skills successfully",
			setup: func(tmpDir string, manager *Manager) []string {
				skill1Dir := filepath.Join(tmpDir, "skill1")
				skill2Dir := filepath.Join(tmpDir, "skill2")

				os.MkdirAll(skill1Dir, 0755)
				os.MkdirAll(skill2Dir, 0755)

				os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"),
					[]byte("---\nname: skill1\ndescription: Skill 1\n---\n"), 0644)
				os.WriteFile(filepath.Join(skill2Dir, "SKILL.md"),
					[]byte("---\nname: skill2\ndescription: Skill 2\n---\n"), 0644)

				return []string{skill1Dir, skill2Dir}
			},
			opts:             BulkImportOptions{},
			wantAddedCount:   2,
			wantSkippedCount: 0,
			wantFailedCount:  0,
			wantError:        false,
		},
		{
			name: "import mixed commands and skills",
			setup: func(tmpDir string, manager *Manager) []string {
				// Create command in proper commands/ directory
				commandsDir := filepath.Join(tmpDir, "commands")
				os.MkdirAll(commandsDir, 0755)
				cmdPath := filepath.Join(commandsDir, "cmd.md")

				skillDir := filepath.Join(tmpDir, "skill")

				os.WriteFile(cmdPath, []byte("---\ndescription: Command\n---\n"), 0644)
				os.MkdirAll(skillDir, 0755)
				os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
					[]byte("---\nname: skill\ndescription: Skill\n---\n"), 0644)

				return []string{cmdPath, skillDir}
			},
			opts:             BulkImportOptions{},
			wantAddedCount:   2,
			wantSkippedCount: 0,
			wantFailedCount:  0,
			wantError:        false,
		},
		{
			name: "conflict with skip existing",
			setup: func(tmpDir string, manager *Manager) []string {
				// Create commands in proper commands/ directory
				commandsDir := filepath.Join(tmpDir, "commands")
				os.MkdirAll(commandsDir, 0755)

				// Add first command
				cmd1Path := filepath.Join(commandsDir, "existing.md")
				os.WriteFile(cmd1Path, []byte("---\ndescription: Existing\n---\n"), 0644)
				_ = manager.AddCommand(cmd1Path, "file://"+cmd1Path, "file")

				// Try to add again
				cmd2Path := filepath.Join(commandsDir, "existing.md")
				os.WriteFile(cmd2Path, []byte("---\ndescription: Existing\n---\n"), 0644)

				return []string{cmd2Path}
			},
			opts:             BulkImportOptions{SkipExisting: true},
			wantAddedCount:   0,
			wantSkippedCount: 1,
			wantFailedCount:  0,
			wantError:        false,
		},
		{
			name: "conflict with force",
			setup: func(tmpDir string, manager *Manager) []string {
				// Create commands in proper commands/ directory
				commandsDir := filepath.Join(tmpDir, "commands")
				os.MkdirAll(commandsDir, 0755)

				// Add first command
				cmd1Path := filepath.Join(commandsDir, "forced.md")
				os.WriteFile(cmd1Path, []byte("---\ndescription: Original\n---\n"), 0644)
				_ = manager.AddCommand(cmd1Path, "file://"+cmd1Path, "file")

				// Force overwrite
				cmd2Path := filepath.Join(commandsDir, "forced.md")
				os.WriteFile(cmd2Path, []byte("---\ndescription: Updated\n---\n"), 0644)

				return []string{cmd2Path}
			},
			opts:             BulkImportOptions{Force: true},
			wantAddedCount:   0, // Changed: Force=true on existing resource goes to Updated
			wantUpdatedCount: 1, // New field to check Updated count
			wantSkippedCount: 0,
			wantFailedCount:  0,
			wantError:        false,
		},
		{
			name: "conflict without flags fails",
			setup: func(tmpDir string, manager *Manager) []string {
				// Create commands in proper commands/ directory
				commandsDir := filepath.Join(tmpDir, "commands")
				os.MkdirAll(commandsDir, 0755)

				// Add first command
				cmd1Path := filepath.Join(commandsDir, "conflict.md")
				os.WriteFile(cmd1Path, []byte("---\ndescription: Conflict\n---\n"), 0644)
				_ = manager.AddCommand(cmd1Path, "file://"+cmd1Path, "file")

				// Try to add again without flags
				cmd2Path := filepath.Join(commandsDir, "conflict.md")
				os.WriteFile(cmd2Path, []byte("---\ndescription: Conflict\n---\n"), 0644)

				return []string{cmd2Path}
			},
			opts:             BulkImportOptions{},
			wantAddedCount:   0,
			wantSkippedCount: 0,
			wantFailedCount:  1,
			wantError:        false, // Now continues processing, collects in Failed
		},
		{
			name: "dry run doesn't actually import",
			setup: func(tmpDir string, manager *Manager) []string {
				// Create command in proper commands/ directory
				commandsDir := filepath.Join(tmpDir, "commands")
				os.MkdirAll(commandsDir, 0755)

				cmdPath := filepath.Join(commandsDir, "dryrun.md")
				os.WriteFile(cmdPath, []byte("---\ndescription: Dry run\n---\n"), 0644)
				return []string{cmdPath}
			},
			opts:             BulkImportOptions{DryRun: true},
			wantAddedCount:   1,
			wantSkippedCount: 0,
			wantFailedCount:  0,
			wantError:        false,
		},
		{
			name: "invalid resource fails",
			setup: func(tmpDir string, manager *Manager) []string {
				invalidPath := filepath.Join(tmpDir, "invalid.txt")
				os.WriteFile(invalidPath, []byte("not a resource"), 0644)
				return []string{invalidPath}
			},
			opts:             BulkImportOptions{},
			wantAddedCount:   0,
			wantSkippedCount: 0,
			wantFailedCount:  1,
			wantError:        false, // Now continues processing, collects in Failed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			repoPath := filepath.Join(tmpDir, "repo")
			manager := NewManagerWithPath(repoPath)

			sources := tt.setup(tmpDir, manager)

			result, err := manager.AddBulk(sources, tt.opts)
			if (err != nil) != tt.wantError {
				t.Errorf("AddBulk() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if len(result.Added) != tt.wantAddedCount {
				t.Errorf("AddBulk() added count = %v, want %v", len(result.Added), tt.wantAddedCount)
			}
			if len(result.Updated) != tt.wantUpdatedCount {
				t.Errorf("AddBulk() updated count = %v, want %v", len(result.Updated), tt.wantUpdatedCount)
			}
			if len(result.Skipped) != tt.wantSkippedCount {
				t.Errorf("AddBulk() skipped count = %v, want %v", len(result.Skipped), tt.wantSkippedCount)
			}
			if len(result.Failed) != tt.wantFailedCount {
				t.Errorf("AddBulk() failed count = %v, want %v", len(result.Failed), tt.wantFailedCount)
			}

			// Verify dry run doesn't actually import
			if tt.opts.DryRun && tt.wantAddedCount > 0 {
				// Check that repo is empty
				resources, _ := manager.List(nil)
				if len(resources) != 0 {
					t.Errorf("DryRun should not import resources, but found %d resources", len(resources))
				}
			}
		})
	}
}

func TestAddCommandCreatesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create a test command
	testCmd := filepath.Join(tmpDir, "test-cmd.md")
	cmdContent := `---
description: A test command
---

# Test Command
`
	if err := os.WriteFile(testCmd, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Add the command
	sourceURL := "gh:owner/repo/test-cmd.md"
	sourceType := "github"
	beforeAdd := time.Now().Add(-time.Second) // Allow for slight clock differences

	err := manager.AddCommand(testCmd, sourceURL, sourceType)
	if err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	afterAdd := time.Now().Add(time.Second)

	// Verify metadata file exists
	metadataPath := metadata.GetMetadataPath("test-cmd", resource.Command, manager.GetRepoPath())
	if _, err := os.Stat(metadataPath); err != nil {
		t.Errorf("Metadata file was not created: %v", err)
	}

	// Verify metadata content
	meta, err := manager.GetMetadata("test-cmd", resource.Command)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	if meta.Name != "test-cmd" {
		t.Errorf("Metadata name = %v, want test-cmd", meta.Name)
	}
	if meta.Type != resource.Command {
		t.Errorf("Metadata type = %v, want command", meta.Type)
	}
	if meta.SourceURL != sourceURL {
		t.Errorf("Metadata sourceURL = %v, want %v", meta.SourceURL, sourceURL)
	}
	if meta.SourceType != sourceType {
		t.Errorf("Metadata sourceType = %v, want %v", meta.SourceType, sourceType)
	}
	if meta.FirstInstalled.Before(beforeAdd) || meta.FirstInstalled.After(afterAdd) {
		t.Errorf("Metadata FirstInstalled = %v, want between %v and %v", meta.FirstInstalled, beforeAdd, afterAdd)
	}
	if meta.LastUpdated.Before(beforeAdd) || meta.LastUpdated.After(afterAdd) {
		t.Errorf("Metadata LastUpdated = %v, want between %v and %v", meta.LastUpdated, beforeAdd, afterAdd)
	}
}

func TestAddSkillCreatesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create a test skill
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: test-skill
description: A test skill
---

# Test Skill
`
	if err := os.WriteFile(skillMdPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	// Add the skill
	sourceURL := "file:///local/path/test-skill"
	sourceType := "local"

	err := manager.AddSkill(skillDir, sourceURL, sourceType)
	if err != nil {
		t.Fatalf("AddSkill() error = %v", err)
	}

	// Verify metadata file exists
	metadataPath := metadata.GetMetadataPath("test-skill", resource.Skill, manager.GetRepoPath())
	if _, err := os.Stat(metadataPath); err != nil {
		t.Errorf("Metadata file was not created: %v", err)
	}

	// Verify metadata content
	meta, err := manager.GetMetadata("test-skill", resource.Skill)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	if meta.Name != "test-skill" {
		t.Errorf("Metadata name = %v, want test-skill", meta.Name)
	}
	if meta.Type != resource.Skill {
		t.Errorf("Metadata type = %v, want skill", meta.Type)
	}
	if meta.SourceURL != sourceURL {
		t.Errorf("Metadata sourceURL = %v, want %v", meta.SourceURL, sourceURL)
	}
	if meta.SourceType != sourceType {
		t.Errorf("Metadata sourceType = %v, want %v", meta.SourceType, sourceType)
	}
}

func TestAddAgentCreatesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create a test agent
	testAgent := filepath.Join(tmpDir, "test-agent.md")
	agentContent := `---
description: A test agent
---

# Test Agent
`
	if err := os.WriteFile(testAgent, []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	// Add the agent
	sourceURL := "gh:owner/repo/agents/test-agent.md"
	sourceType := "github"

	err := manager.AddAgent(testAgent, sourceURL, sourceType)
	if err != nil {
		t.Fatalf("AddAgent() error = %v", err)
	}

	// Verify metadata file exists
	metadataPath := metadata.GetMetadataPath("test-agent", resource.Agent, manager.GetRepoPath())
	if _, err := os.Stat(metadataPath); err != nil {
		t.Errorf("Metadata file was not created: %v", err)
	}

	// Verify metadata content
	meta, err := manager.GetMetadata("test-agent", resource.Agent)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	if meta.Name != "test-agent" {
		t.Errorf("Metadata name = %v, want test-agent", meta.Name)
	}
	if meta.Type != resource.Agent {
		t.Errorf("Metadata type = %v, want agent", meta.Type)
	}
	if meta.SourceURL != sourceURL {
		t.Errorf("Metadata sourceURL = %v, want %v", meta.SourceURL, sourceURL)
	}
	if meta.SourceType != sourceType {
		t.Errorf("Metadata sourceType = %v, want %v", meta.SourceType, sourceType)
	}
}

func TestRemoveDeletesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create and add a test command
	testCmd := filepath.Join(tmpDir, "test-cmd.md")
	cmdContent := `---
description: A test command
---

# Test Command
`
	if err := os.WriteFile(testCmd, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	if err := manager.AddCommand(testCmd, "file://"+testCmd, "file"); err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	// Verify metadata exists
	metadataPath := metadata.GetMetadataPath("test-cmd", resource.Command, manager.GetRepoPath())
	if _, err := os.Stat(metadataPath); err != nil {
		t.Fatalf("Metadata file was not created: %v", err)
	}

	// Remove the command
	err := manager.Remove("test-cmd", resource.Command)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Verify metadata was deleted
	if _, err := os.Stat(metadataPath); err == nil {
		t.Error("Metadata file still exists after removal")
	}
}

func TestGetMetadataNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	_, err := manager.GetMetadata("nonexistent", resource.Command)
	if err == nil {
		t.Error("GetMetadata() expected error for nonexistent resource, got nil")
	}
}

func TestBulkImportCreatesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	manager := NewManagerWithPath(repoPath)

	// Create test commands in proper commands/ directory
	commandsDir := filepath.Join(tmpDir, "commands")
	os.MkdirAll(commandsDir, 0755)

	cmd1Path := filepath.Join(commandsDir, "cmd1.md")
	cmd2Path := filepath.Join(commandsDir, "cmd2.md")

	os.WriteFile(cmd1Path, []byte("---\ndescription: Command 1\n---\n"), 0644)
	os.WriteFile(cmd2Path, []byte("---\ndescription: Command 2\n---\n"), 0644)

	// Bulk import
	sources := []string{cmd1Path, cmd2Path}
	result, err := manager.AddBulk(sources, BulkImportOptions{})
	if err != nil {
		t.Fatalf("AddBulk() error = %v", err)
	}

	if len(result.Added) != 2 {
		t.Errorf("AddBulk() added count = %v, want 2", len(result.Added))
	}

	// Verify metadata files exist
	meta1Path := metadata.GetMetadataPath("cmd1", resource.Command, manager.GetRepoPath())
	if _, err := os.Stat(meta1Path); err != nil {
		t.Errorf("Metadata file for cmd1 was not created: %v", err)
	}

	meta2Path := metadata.GetMetadataPath("cmd2", resource.Command, manager.GetRepoPath())
	if _, err := os.Stat(meta2Path); err != nil {
		t.Errorf("Metadata file for cmd2 was not created: %v", err)
	}

	// Verify metadata contains correct source info (file:// URLs)
	meta1, err := manager.GetMetadata("cmd1", resource.Command)
	if err != nil {
		t.Fatalf("GetMetadata(cmd1) error = %v", err)
	}
	if meta1.SourceType != "file" {
		t.Errorf("cmd1 SourceType = %v, want file", meta1.SourceType)
	}

	meta2, err := manager.GetMetadata("cmd2", resource.Command)
	if err != nil {
		t.Fatalf("GetMetadata(cmd2) error = %v", err)
	}
	if meta2.SourceType != "file" {
		t.Errorf("cmd2 SourceType = %v, want file", meta2.SourceType)
	}
}

// TestBulkImportWithGitSourceURL verifies that Git source URLs are properly stored in metadata
func TestBulkImportWithGitSourceURL(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	manager := NewManagerWithPath(repoPath)

	// Create test command in proper commands/ directory
	commandsDir := filepath.Join(tmpDir, "commands")
	os.MkdirAll(commandsDir, 0755)

	cmdPath := filepath.Join(commandsDir, "test-cmd.md")
	cmdContent := `---
name: test-cmd
description: Test command for Git source metadata
---

# Test Command

Test content.
`
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Test with GitHub source URL
	opts := BulkImportOptions{
		SourceURL:  "https://github.com/owner/repo",
		SourceType: "github",
	}

	result, err := manager.AddBulk([]string{cmdPath}, opts)
	if err != nil {
		t.Fatalf("AddBulk() error = %v", err)
	}

	if len(result.Added) != 1 {
		t.Fatalf("AddBulk() added count = %v, want 1", len(result.Added))
	}

	// Verify metadata has Git source URL, not temp path
	meta, err := manager.GetMetadata("test-cmd", resource.Command)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	if meta.SourceType != "github" {
		t.Errorf("SourceType = %v, want github", meta.SourceType)
	}

	if meta.SourceURL != "https://github.com/owner/repo" {
		t.Errorf("SourceURL = %v, want https://github.com/owner/repo", meta.SourceURL)
	}

	// Verify it doesn't contain temp path or file:// URL
	if strings.Contains(meta.SourceURL, "file://") {
		t.Errorf("SourceURL should not contain file:// for Git sources: %v", meta.SourceURL)
	}
	if strings.Contains(meta.SourceURL, tmpDir) {
		t.Errorf("SourceURL should not contain temp path: %v", meta.SourceURL)
	}
}

// TestBulkImportWithoutSourceInfo verifies fallback to file:// for local sources
func TestBulkImportWithoutSourceInfo(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	manager := NewManagerWithPath(repoPath)

	// Create test command in proper commands/ directory
	commandsDir := filepath.Join(tmpDir, "commands")
	os.MkdirAll(commandsDir, 0755)

	cmdPath := filepath.Join(commandsDir, "local-cmd.md")
	cmdContent := `---
name: local-cmd
description: Local command
---

# Local Command

Test content.
`
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Test without source info (should fall back to file://)
	opts := BulkImportOptions{}

	result, err := manager.AddBulk([]string{cmdPath}, opts)
	if err != nil {
		t.Fatalf("AddBulk() error = %v", err)
	}

	if len(result.Added) != 1 {
		t.Fatalf("AddBulk() added count = %v, want 1", len(result.Added))
	}

	// Verify metadata has file:// URL
	meta, err := manager.GetMetadata("local-cmd", resource.Command)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	if meta.SourceType != "file" {
		t.Errorf("SourceType = %v, want file", meta.SourceType)
	}

	if !strings.HasPrefix(meta.SourceURL, "file://") {
		t.Errorf("SourceURL = %v, should start with file://", meta.SourceURL)
	}
}
