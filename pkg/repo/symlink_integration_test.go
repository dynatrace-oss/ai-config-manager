//go:build integration

package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// TestImportModes_URLSourceCopies verifies that resources from Git URLs are copied (not symlinked)
func TestImportModes_URLSourceCopies(t *testing.T) {
	// Setup temp repo
	tmpRepo := t.TempDir()
	mgr := NewManagerWithPath(tmpRepo)

	// Create a temp source to simulate a cloned Git repo
	tmpSource := t.TempDir()
	commandsDir := filepath.Join(tmpSource, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	commandPath := filepath.Join(commandsDir, "test.md")
	commandContent := `---
description: Test command from Git source
---

# Test Command

Test content from Git repository.
`
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Import with copy mode and Git source metadata
	// This simulates importing from a Git URL
	opts := BulkImportOptions{
		ImportMode: "copy",
		SourceURL:  "https://github.com/test/repo",
		SourceType: "github",
	}

	result, err := mgr.AddBulk([]string{commandPath}, opts)
	if err != nil {
		t.Fatalf("AddBulk failed: %v", err)
	}

	if len(result.Added) != 1 {
		t.Fatalf("Expected 1 resource added, got %d", len(result.Added))
	}

	// Verify the file is a regular file, not a symlink
	repoCommandPath := filepath.Join(tmpRepo, "commands", "test.md")
	fi, err := os.Lstat(repoCommandPath)
	if err != nil {
		t.Fatalf("Failed to stat command in repo: %v", err)
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("Expected regular file, but got symlink for Git source")
	}

	t.Logf("✓ Git source correctly copied (not symlinked)")
}

// TestImportModes_PathSourceSymlinks verifies that resources from local paths are symlinked
func TestImportModes_PathSourceSymlinks(t *testing.T) {
	// Setup temp source directory with test skill
	tmpSource := t.TempDir()
	skillDir := filepath.Join(tmpSource, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: test-skill
description: A test skill for symlink testing
---

# Test Skill

This skill tests symlink behavior.
`
	if err := os.WriteFile(skillMdPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	// Setup temp repo
	tmpRepo := t.TempDir()
	mgr := NewManagerWithPath(tmpRepo)

	// Import with symlink mode (for local path sources)
	opts := BulkImportOptions{
		ImportMode: "symlink",
		SourceURL:  "file://" + skillDir,
		SourceType: "file",
	}

	result, err := mgr.AddBulk([]string{skillDir}, opts)
	if err != nil {
		t.Fatalf("AddBulk failed: %v", err)
	}

	if len(result.Added) != 1 {
		t.Fatalf("Expected 1 resource added, got %d", len(result.Added))
	}

	// Verify symlink was created
	skillPath := filepath.Join(tmpRepo, "skills", "test-skill")
	fi, err := os.Lstat(skillPath)
	if err != nil {
		t.Fatalf("Failed to stat skill in repo: %v", err)
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Expected symlink, but got regular directory for local path source")
	}

	// Verify symlink target is correct
	target, err := os.Readlink(skillPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	expectedTarget, err := filepath.Abs(skillDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	if target != expectedTarget {
		t.Errorf("Symlink target = %s, want %s", target, expectedTarget)
	}

	t.Logf("✓ Local path correctly symlinked (target: %s)", target)
}

// TestImportModes_SymlinkEditReflectsImmediately verifies that edits to symlinked resources are visible immediately
func TestImportModes_SymlinkEditReflectsImmediately(t *testing.T) {
	// Setup temp source directory with test command
	tmpSource := t.TempDir()
	commandsDir := filepath.Join(tmpSource, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	commandPath := filepath.Join(commandsDir, "edit-test.md")
	originalContent := `---
description: Original description
---

# Edit Test Command

Original content.
`
	if err := os.WriteFile(commandPath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Setup temp repo
	tmpRepo := t.TempDir()
	mgr := NewManagerWithPath(tmpRepo)

	// Import with symlink mode
	opts := BulkImportOptions{
		ImportMode: "symlink",
		SourceURL:  "file://" + commandPath,
		SourceType: "file",
	}

	result, err := mgr.AddBulk([]string{commandPath}, opts)
	if err != nil {
		t.Fatalf("AddBulk failed: %v", err)
	}

	if len(result.Added) != 1 {
		t.Fatalf("Expected 1 resource added, got %d", len(result.Added))
	}

	// Verify symlink exists
	repoCommandPath := filepath.Join(tmpRepo, "commands", "edit-test.md")
	fi, err := os.Lstat(repoCommandPath)
	if err != nil {
		t.Fatalf("Failed to stat command in repo: %v", err)
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("Expected symlink, but got regular file")
	}

	// Read original content through symlink
	originalRead, err := os.ReadFile(repoCommandPath)
	if err != nil {
		t.Fatalf("Failed to read command through symlink: %v", err)
	}

	if string(originalRead) != originalContent {
		t.Errorf("Original content mismatch")
	}

	// Edit the source file
	updatedContent := `---
description: Updated description
---

# Edit Test Command

Updated content after edit.
`
	if err := os.WriteFile(commandPath, []byte(updatedContent), 0644); err != nil {
		t.Fatalf("Failed to update command: %v", err)
	}

	// Read through symlink again - should see updated content immediately
	updatedRead, err := os.ReadFile(repoCommandPath)
	if err != nil {
		t.Fatalf("Failed to read command through symlink after edit: %v", err)
	}

	if string(updatedRead) != updatedContent {
		t.Errorf("Updated content not visible through symlink.\nExpected: %s\nGot: %s",
			updatedContent, string(updatedRead))
	}

	t.Logf("✓ Edit to symlinked file reflected immediately")
}

// TestImportModes_CommandSymlink verifies symlink behavior for commands
func TestImportModes_CommandSymlink(t *testing.T) {
	// Setup temp source directory with test command
	tmpSource := t.TempDir()
	commandsDir := filepath.Join(tmpSource, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	commandPath := filepath.Join(commandsDir, "test-cmd.md")
	commandContent := `---
description: Test command for symlink
---

# Test Command

Test content.
`
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Setup temp repo
	tmpRepo := t.TempDir()
	mgr := NewManagerWithPath(tmpRepo)

	// Import with symlink mode
	opts := BulkImportOptions{
		ImportMode: "symlink",
	}

	result, err := mgr.AddBulk([]string{commandPath}, opts)
	if err != nil {
		t.Fatalf("AddBulk failed: %v", err)
	}

	if result.CommandCount != 1 {
		t.Fatalf("Expected 1 command, got %d", result.CommandCount)
	}

	// Verify symlink
	repoCommandPath := filepath.Join(tmpRepo, "commands", "test-cmd.md")
	fi, err := os.Lstat(repoCommandPath)
	if err != nil {
		t.Fatalf("Failed to stat command: %v", err)
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Expected symlink for command")
	}

	// Verify target
	target, err := os.Readlink(repoCommandPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	expectedTarget, _ := filepath.Abs(commandPath)
	if target != expectedTarget {
		t.Errorf("Command symlink target = %s, want %s", target, expectedTarget)
	}

	t.Logf("✓ Command symlinked correctly")
}

// TestImportModes_AgentSymlink verifies symlink behavior for agents
func TestImportModes_AgentSymlink(t *testing.T) {
	// Setup temp source directory with test agent in agents/ directory
	tmpSource := t.TempDir()
	agentsDir := filepath.Join(tmpSource, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents directory: %v", err)
	}

	agentPath := filepath.Join(agentsDir, "test-agent.md")
	agentContent := `---
description: Test agent for symlink
type: assistant
---

# Test Agent

Test agent content.
`
	if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Setup temp repo
	tmpRepo := t.TempDir()
	mgr := NewManagerWithPath(tmpRepo)

	// Import with symlink mode
	opts := BulkImportOptions{
		ImportMode: "symlink",
	}

	result, err := mgr.AddBulk([]string{agentPath}, opts)
	if err != nil {
		t.Fatalf("AddBulk failed: %v", err)
	}

	if result.AgentCount != 1 {
		t.Fatalf("Expected 1 agent, got %d", result.AgentCount)
	}

	// Verify symlink
	repoAgentPath := filepath.Join(tmpRepo, "agents", "test-agent.md")
	fi, err := os.Lstat(repoAgentPath)
	if err != nil {
		t.Fatalf("Failed to stat agent: %v", err)
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Expected symlink for agent")
	}

	// Verify target
	target, err := os.Readlink(repoAgentPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	expectedTarget, _ := filepath.Abs(agentPath)
	if target != expectedTarget {
		t.Errorf("Agent symlink target = %s, want %s", target, expectedTarget)
	}

	t.Logf("✓ Agent symlinked correctly")
}

// TestImportModes_NestedCommandSymlink verifies symlink behavior for nested commands
func TestImportModes_NestedCommandSymlink(t *testing.T) {
	// Setup temp source directory with nested command
	tmpSource := t.TempDir()
	commandsDir := filepath.Join(tmpSource, "commands", "api")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create nested commands directory: %v", err)
	}

	commandPath := filepath.Join(commandsDir, "deploy.md")
	commandContent := `---
description: Nested deploy command
---

# Deploy Command

Deploy API.
`
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create nested command: %v", err)
	}

	// Setup temp repo
	tmpRepo := t.TempDir()
	mgr := NewManagerWithPath(tmpRepo)

	// Import with symlink mode
	opts := BulkImportOptions{
		ImportMode: "symlink",
	}

	result, err := mgr.AddBulk([]string{commandPath}, opts)
	if err != nil {
		t.Fatalf("AddBulk failed: %v", err)
	}

	if result.CommandCount != 1 {
		t.Fatalf("Expected 1 command, got %d", result.CommandCount)
	}

	// Verify symlink preserves nested structure
	repoCommandPath := filepath.Join(tmpRepo, "commands", "api", "deploy.md")
	fi, err := os.Lstat(repoCommandPath)
	if err != nil {
		t.Fatalf("Failed to stat nested command: %v", err)
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Expected symlink for nested command")
	}

	// Verify we can load it with correct name
	res, err := resource.LoadCommand(repoCommandPath)
	if err != nil {
		t.Fatalf("Failed to load nested command: %v", err)
	}

	if res.Name != "api/deploy" {
		t.Errorf("Nested command name = %s, want api/deploy", res.Name)
	}

	t.Logf("✓ Nested command symlinked correctly with name: %s", res.Name)
}

// TestImportModes_PackageSymlink verifies symlink behavior for packages
func TestImportModes_PackageSymlink(t *testing.T) {
	// Setup temp source directory with test package
	tmpSource := t.TempDir()
	packagePath := filepath.Join(tmpSource, "test-package.package.json")
	packageContent := `{
  "name": "test-package",
  "description": "Test package for symlink",
  "resources": []
}`
	if err := os.WriteFile(packagePath, []byte(packageContent), 0644); err != nil {
		t.Fatalf("Failed to create package: %v", err)
	}

	// Setup temp repo
	tmpRepo := t.TempDir()
	mgr := NewManagerWithPath(tmpRepo)

	// Import with symlink mode
	opts := BulkImportOptions{
		ImportMode: "symlink",
	}

	result, err := mgr.AddBulk([]string{packagePath}, opts)
	if err != nil {
		t.Fatalf("AddBulk failed: %v", err)
	}

	if result.PackageCount != 1 {
		t.Fatalf("Expected 1 package, got %d", result.PackageCount)
	}

	// Verify symlink
	repoPackagePath := filepath.Join(tmpRepo, "packages", "test-package.package.json")
	fi, err := os.Lstat(repoPackagePath)
	if err != nil {
		t.Fatalf("Failed to stat package: %v", err)
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Expected symlink for package")
	}

	// Verify target
	target, err := os.Readlink(repoPackagePath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	expectedTarget, _ := filepath.Abs(packagePath)
	if target != expectedTarget {
		t.Errorf("Package symlink target = %s, want %s", target, expectedTarget)
	}

	t.Logf("✓ Package symlinked correctly")
}

// TestImportModes_CopyMode verifies copy mode creates regular files
func TestImportModes_CopyMode(t *testing.T) {
	// Setup temp source directory with test skill
	tmpSource := t.TempDir()
	skillDir := filepath.Join(tmpSource, "copy-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: copy-skill
description: Test skill for copy mode
---

# Copy Skill

This skill tests copy behavior.
`
	if err := os.WriteFile(skillMdPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	// Setup temp repo
	tmpRepo := t.TempDir()
	mgr := NewManagerWithPath(tmpRepo)

	// Import with explicit copy mode
	opts := BulkImportOptions{
		ImportMode: "copy",
	}

	result, err := mgr.AddBulk([]string{skillDir}, opts)
	if err != nil {
		t.Fatalf("AddBulk failed: %v", err)
	}

	if len(result.Added) != 1 {
		t.Fatalf("Expected 1 resource added, got %d", len(result.Added))
	}

	// Verify it's a regular directory, not a symlink
	skillPath := filepath.Join(tmpRepo, "skills", "copy-skill")
	fi, err := os.Lstat(skillPath)
	if err != nil {
		t.Fatalf("Failed to stat skill in repo: %v", err)
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("Expected regular directory for copy mode, but got symlink")
	}

	// Verify SKILL.md was copied
	copiedSkillMd := filepath.Join(skillPath, "SKILL.md")
	if _, err := os.Stat(copiedSkillMd); err != nil {
		t.Errorf("SKILL.md was not copied: %v", err)
	}

	t.Logf("✓ Copy mode correctly created regular directory")
}
