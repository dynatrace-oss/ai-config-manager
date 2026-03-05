//go:build integration

package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

// TestVSCodeCopilotSkillWorkflow tests the complete workflow for VSCode/Copilot:
// import → install → list → uninstall
func TestVSCodeCopilotSkillWorkflow(t *testing.T) {
	// Setup: Create test skill in source directory
	tmpSource := t.TempDir()
	skillDir := filepath.Join(tmpSource, "test-copilot-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillContent := `---
name: test-copilot-skill
description: Test skill for VSCode/Copilot workflow
license: MIT
---

# Test Copilot Skill

This skill tests the VSCode/Copilot integration workflow.

## Purpose

Verify that skills can be imported, installed, listed, and uninstalled for Copilot.
`
	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMdPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	// Setup temp repository
	tmpRepo := t.TempDir()
	mgr := repo.NewManagerWithPath(tmpRepo)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Setup temp project directory
	tmpProject := t.TempDir()

	// Step 1: Import skill to repository
	t.Log("Step 1: Importing skill to repository")
	opts := repo.BulkImportOptions{
		ImportMode: "copy",
		SourceType: "file",
	}
	result, err := mgr.AddBulk([]string{skillDir}, opts)
	if err != nil {
		t.Fatalf("Failed to import skill: %v", err)
	}
	if result.SkillCount != 1 {
		t.Fatalf("Expected 1 skill imported, got %d", result.SkillCount)
	}
	t.Logf("✓ Skill imported successfully")

	// Step 2: Install skill with --tool=copilot
	t.Log("Step 2: Installing skill with --tool=copilot")
	installer, err := NewInstallerWithTargets(tmpProject, []tools.Tool{tools.Copilot})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	if err := installer.InstallSkill("test-copilot-skill", mgr); err != nil {
		t.Fatalf("Failed to install skill: %v", err)
	}
	t.Logf("✓ Skill installed for Copilot")

	// Step 3: Verify .github/skills/test-copilot-skill exists
	t.Log("Step 3: Verifying installation in .github/skills/")
	skillInstallPath := filepath.Join(tmpProject, ".github", "skills", "test-copilot-skill")
	info, err := os.Lstat(skillInstallPath)
	if err != nil {
		t.Fatalf("Skill not found at %s: %v", skillInstallPath, err)
	}

	// Verify it's a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Expected symlink at %s, but got regular directory", skillInstallPath)
	}

	// Verify symlink target
	target, err := os.Readlink(skillInstallPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	expectedTarget := filepath.Join(tmpRepo, "skills", "test-copilot-skill")
	if target != expectedTarget {
		t.Errorf("Symlink target = %s, want %s", target, expectedTarget)
	}
	t.Logf("✓ Skill correctly symlinked to repository")

	// Verify SKILL.md exists in the target
	targetSkillMd := filepath.Join(target, "SKILL.md")
	if _, err := os.Stat(targetSkillMd); err != nil {
		t.Errorf("SKILL.md not found in symlink target: %v", err)
	}
	t.Logf("✓ SKILL.md exists in repository")

	// Step 4: List skills with --tool=copilot
	t.Log("Step 4: Listing installed skills")
	installed, err := installer.List()
	if err != nil {
		t.Fatalf("Failed to list installed skills: %v", err)
	}

	// Step 5: Verify skill appears in list
	t.Log("Step 5: Verifying skill in list")
	found := false
	for _, res := range installed {
		if res.Type == resource.Skill && res.Name == "test-copilot-skill" {
			found = true
			if res.Description != "Test skill for VSCode/Copilot workflow" {
				t.Errorf("Skill description = %s, want 'Test skill for VSCode/Copilot workflow'", res.Description)
			}
			break
		}
	}
	if !found {
		t.Errorf("Skill 'test-copilot-skill' not found in list")
	}
	t.Logf("✓ Skill found in list with correct metadata")

	// Step 6: Uninstall skill
	t.Log("Step 6: Uninstalling skill")
	if err := installer.Uninstall("test-copilot-skill", resource.Skill, mgr); err != nil {
		t.Fatalf("Failed to uninstall skill: %v", err)
	}
	t.Logf("✓ Skill uninstalled")

	// Step 7: Verify .github/skills/test-copilot-skill is removed
	t.Log("Step 7: Verifying skill was removed")
	if _, err := os.Lstat(skillInstallPath); err == nil {
		t.Errorf("Skill still exists at %s after uninstall", skillInstallPath)
	} else if !os.IsNotExist(err) {
		t.Errorf("Unexpected error checking skill path: %v", err)
	}
	t.Logf("✓ Skill removed from .github/skills/")

	// Verify skill no longer in list
	installed, err = installer.List()
	if err != nil {
		t.Fatalf("Failed to list installed skills after uninstall: %v", err)
	}
	for _, res := range installed {
		if res.Type == resource.Skill && res.Name == "test-copilot-skill" {
			t.Errorf("Skill 'test-copilot-skill' still in list after uninstall")
		}
	}
	t.Logf("✓ Skill not in list after uninstall")
}

// TestVSCodeToolNameAlias verifies that "vscode" works as an alias for "copilot"
func TestVSCodeToolNameAlias(t *testing.T) {
	// Parse both tool names
	copilotTool, err := tools.ParseTool("copilot")
	if err != nil {
		t.Fatalf("Failed to parse 'copilot': %v", err)
	}

	vscodeTool, err := tools.ParseTool("vscode")
	if err != nil {
		// If VSCode is not recognized, check if it's documented as not implemented
		t.Skipf("VSCode tool name not yet implemented (copilot=%v)", copilotTool)
	}

	// Verify they resolve to the same tool
	if copilotTool != vscodeTool {
		t.Errorf("'copilot' and 'vscode' should resolve to the same tool, got copilot=%v vscode=%v",
			copilotTool, vscodeTool)
	}

	// Verify both produce same ToolInfo
	copilotInfo := tools.GetToolInfo(copilotTool)
	vscodeInfo := tools.GetToolInfo(vscodeTool)

	if copilotInfo.SkillsDir != vscodeInfo.SkillsDir {
		t.Errorf("Tool skills directories differ: copilot=%s vscode=%s",
			copilotInfo.SkillsDir, vscodeInfo.SkillsDir)
	}

	if copilotInfo.SkillsDir != ".github/skills" {
		t.Errorf("Expected .github/skills, got %s", copilotInfo.SkillsDir)
	}

	t.Logf("✓ Both 'copilot' and 'vscode' use .github/skills directory")
}

// TestCopilotToolDetection verifies that .github/skills/ is detected for Copilot
func TestCopilotToolDetection(t *testing.T) {
	tmpProject := t.TempDir()

	// Initially, no tools should be detected
	detected, err := tools.DetectExistingTools(tmpProject)
	if err != nil {
		t.Fatalf("DetectExistingTools failed: %v", err)
	}
	if len(detected) != 0 {
		t.Errorf("Expected no tools in empty directory, got %v", detected)
	}

	// Create .github/skills directory
	skillsDir := filepath.Join(tmpProject, ".github", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create .github/skills: %v", err)
	}

	// Now Copilot should be detected
	detected, err = tools.DetectExistingTools(tmpProject)
	if err != nil {
		t.Fatalf("DetectExistingTools failed: %v", err)
	}

	if len(detected) != 1 {
		t.Fatalf("Expected 1 tool detected, got %d: %v", len(detected), detected)
	}

	if detected[0] != tools.Copilot {
		t.Errorf("Expected Copilot tool, got %v", detected[0])
	}

	t.Logf("✓ Copilot detected when .github/skills/ exists")
}

// TestMultiToolSkillInstallation verifies skills can be installed to multiple tools
func TestMultiToolSkillInstallation(t *testing.T) {
	// Setup: Create test skill
	tmpSource := t.TempDir()
	skillDir := filepath.Join(tmpSource, "multi-tool-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillContent := `---
name: multi-tool-skill
description: Test skill for multi-tool installation
---

# Multi-Tool Skill

This skill tests installation to multiple tools simultaneously.
`
	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMdPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	// Setup repository
	tmpRepo := t.TempDir()
	mgr := repo.NewManagerWithPath(tmpRepo)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Import skill
	opts := repo.BulkImportOptions{ImportMode: "copy"}
	result, err := mgr.AddBulk([]string{skillDir}, opts)
	if err != nil {
		t.Fatalf("Failed to import skill: %v", err)
	}
	if result.SkillCount != 1 {
		t.Fatalf("Expected 1 skill imported, got %d", result.SkillCount)
	}

	// Setup project
	tmpProject := t.TempDir()

	// Install to multiple tools: Claude, OpenCode, and Copilot
	t.Log("Installing skill to Claude, OpenCode, and Copilot")
	allTools := []tools.Tool{tools.Claude, tools.OpenCode, tools.Copilot}
	installer, err := NewInstallerWithTargets(tmpProject, allTools)
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	if err := installer.InstallSkill("multi-tool-skill", mgr); err != nil {
		t.Fatalf("Failed to install skill to multiple tools: %v", err)
	}

	// Verify installation in all three tools
	expectedPaths := map[tools.Tool]string{
		tools.Claude:   filepath.Join(tmpProject, ".claude", "skills", "multi-tool-skill"),
		tools.OpenCode: filepath.Join(tmpProject, ".opencode", "skills", "multi-tool-skill"),
		tools.Copilot:  filepath.Join(tmpProject, ".github", "skills", "multi-tool-skill"),
	}

	for tool, expectedPath := range expectedPaths {
		t.Logf("Verifying installation for %s at %s", tool, expectedPath)

		info, err := os.Lstat(expectedPath)
		if err != nil {
			t.Errorf("Skill not found for %s at %s: %v", tool, expectedPath, err)
			continue
		}

		// Verify it's a symlink
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("Expected symlink for %s, but got regular directory", tool)
			continue
		}

		// Verify target
		target, err := os.Readlink(expectedPath)
		if err != nil {
			t.Errorf("Failed to read symlink for %s: %v", tool, err)
			continue
		}

		expectedTarget := filepath.Join(tmpRepo, "skills", "multi-tool-skill")
		if target != expectedTarget {
			t.Errorf("Symlink target for %s = %s, want %s", tool, target, expectedTarget)
		}

		t.Logf("✓ %s installation verified", tool)
	}

	// List should return only one skill (deduplicated)
	t.Log("Verifying list deduplication")
	installed, err := installer.List()
	if err != nil {
		t.Fatalf("Failed to list installed skills: %v", err)
	}

	skillCount := 0
	for _, res := range installed {
		if res.Type == resource.Skill && res.Name == "multi-tool-skill" {
			skillCount++
		}
	}

	if skillCount != 1 {
		t.Errorf("Expected 1 skill in list (deduplicated), got %d", skillCount)
	}
	t.Logf("✓ Skill correctly deduplicated in list")

	// Uninstall should remove from all tools
	t.Log("Uninstalling from all tools")
	if err := installer.Uninstall("multi-tool-skill", resource.Skill, mgr); err != nil {
		t.Fatalf("Failed to uninstall skill: %v", err)
	}

	// Verify removal from all tools
	for tool, expectedPath := range expectedPaths {
		if _, err := os.Lstat(expectedPath); err == nil {
			t.Errorf("Skill still exists for %s after uninstall", tool)
		} else if !os.IsNotExist(err) {
			t.Errorf("Unexpected error checking %s path: %v", tool, err)
		}
	}
	t.Logf("✓ Skill removed from all tools")
}

// TestCopilotSkillsOnlySupport verifies Copilot only supports skills (not commands or agents)
func TestCopilotSkillsOnlySupport(t *testing.T) {
	copilotInfo := tools.GetToolInfo(tools.Copilot)

	if copilotInfo.SupportsCommands {
		t.Errorf("Copilot should not support commands")
	}

	if !copilotInfo.SupportsSkills {
		t.Errorf("Copilot should support skills")
	}

	if copilotInfo.SupportsAgents {
		t.Errorf("Copilot should not support agents")
	}

	if copilotInfo.CommandsDir != "" {
		t.Errorf("Copilot CommandsDir should be empty, got %s", copilotInfo.CommandsDir)
	}

	if copilotInfo.AgentsDir != "" {
		t.Errorf("Copilot AgentsDir should be empty, got %s", copilotInfo.AgentsDir)
	}

	if copilotInfo.SkillsDir != ".github/skills" {
		t.Errorf("Copilot SkillsDir = %s, want .github/skills", copilotInfo.SkillsDir)
	}

	t.Logf("✓ Copilot correctly supports only skills")
}

// TestCopilotWithFixtureSkills tests using skills from the testdata fixtures
func TestCopilotWithFixtureSkills(t *testing.T) {
	// Use existing fixture skills
	fixturePath := filepath.Join("..", "..", "testdata", "repos", "skills-standard", "skills")

	// Verify fixture exists
	if _, err := os.Stat(fixturePath); err != nil {
		t.Skipf("Skipping: fixture not found at %s", fixturePath)
	}

	// Setup repository
	tmpRepo := t.TempDir()
	mgr := repo.NewManagerWithPath(tmpRepo)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Import fixture skills
	t.Log("Importing fixture skills")
	skill1Path := filepath.Join(fixturePath, "test-skill-1")
	skill2Path := filepath.Join(fixturePath, "test-skill-2")

	opts := repo.BulkImportOptions{ImportMode: "copy"}
	result, err := mgr.AddBulk([]string{skill1Path, skill2Path}, opts)
	if err != nil {
		t.Fatalf("Failed to import fixture skills: %v", err)
	}
	if result.SkillCount != 2 {
		t.Fatalf("Expected 2 skills imported, got %d", result.SkillCount)
	}
	t.Logf("✓ Imported %d fixture skills", result.SkillCount)

	// Setup project and install to Copilot
	tmpProject := t.TempDir()
	installer, err := NewInstallerWithTargets(tmpProject, []tools.Tool{tools.Copilot})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	// Install both skills
	if err := installer.InstallSkill("test-skill-1", mgr); err != nil {
		t.Fatalf("Failed to install test-skill-1: %v", err)
	}
	if err := installer.InstallSkill("test-skill-2", mgr); err != nil {
		t.Fatalf("Failed to install test-skill-2: %v", err)
	}
	t.Logf("✓ Installed fixture skills to Copilot")

	// Verify both are installed
	skill1Path = filepath.Join(tmpProject, ".github", "skills", "test-skill-1")
	skill2Path = filepath.Join(tmpProject, ".github", "skills", "test-skill-2")

	for _, path := range []string{skill1Path, skill2Path} {
		info, err := os.Lstat(path)
		if err != nil {
			t.Errorf("Skill not found at %s: %v", path, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("Expected symlink at %s", path)
		}
	}

	// List and verify both appear
	installed, err := installer.List()
	if err != nil {
		t.Fatalf("Failed to list installed skills: %v", err)
	}

	foundSkills := make(map[string]bool)
	for _, res := range installed {
		if res.Type == resource.Skill {
			foundSkills[res.Name] = true
		}
	}

	if !foundSkills["test-skill-1"] || !foundSkills["test-skill-2"] {
		t.Errorf("Not all fixture skills found in list. Found: %v", foundSkills)
	}
	t.Logf("✓ Both fixture skills appear in list")
}
