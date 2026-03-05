//go:build integration

package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

// TestWindsurfSkillInstallation tests the complete workflow for Windsurf:
// import → install → verify
func TestWindsurfSkillInstallation(t *testing.T) {
	// Setup: Create test skill in source directory
	tmpSource := t.TempDir()
	skillDir := filepath.Join(tmpSource, "test-windsurf-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillContent := `---
name: test-windsurf-skill
description: Test skill for Windsurf workflow
license: MIT
---

# Test Windsurf Skill

This skill tests the Windsurf integration workflow.

## Purpose

Verify that skills can be imported, installed, and listed for Windsurf.
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

	// Step 2: Install skill with --tool=windsurf
	t.Log("Step 2: Installing skill with --tool=windsurf")
	installer, err := NewInstallerWithTargets(tmpProject, []tools.Tool{tools.Windsurf})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	if err := installer.InstallSkill("test-windsurf-skill", mgr); err != nil {
		t.Fatalf("Failed to install skill: %v", err)
	}
	t.Logf("✓ Skill installed for Windsurf")

	// Step 3: Verify .windsurf/skills/test-windsurf-skill exists
	t.Log("Step 3: Verifying installation in .windsurf/skills/")
	skillInstallPath := filepath.Join(tmpProject, ".windsurf", "skills", "test-windsurf-skill")
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

	expectedTarget := filepath.Join(tmpRepo, "skills", "test-windsurf-skill")
	if target != expectedTarget {
		t.Errorf("Symlink points to %s, expected %s", target, expectedTarget)
	}

	// Verify SKILL.md is accessible through symlink
	skillMdInstalled := filepath.Join(skillInstallPath, "SKILL.md")
	if _, err := os.Stat(skillMdInstalled); err != nil {
		t.Fatalf("SKILL.md not accessible at %s: %v", skillMdInstalled, err)
	}
	t.Logf("✓ SKILL.md accessible through symlink")

	// Step 4: List installed resources
	t.Log("Step 4: Listing installed resources")
	installed, err := installer.List()
	if err != nil {
		t.Fatalf("Failed to list installed resources: %v", err)
	}

	if len(installed) != 1 {
		t.Fatalf("Expected 1 installed resource, got %d", len(installed))
	}

	if installed[0].Name != "test-windsurf-skill" {
		t.Errorf("Expected skill 'test-windsurf-skill', got '%s'", installed[0].Name)
	}
	t.Logf("✓ Skill listed correctly")
}

// TestMultiToolWithWindsurf tests installing a skill to multiple tools including Windsurf
func TestMultiToolWithWindsurf(t *testing.T) {
	// Setup: Create test skill in source directory
	tmpSource := t.TempDir()
	skillDir := filepath.Join(tmpSource, "multi-tool-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillContent := `---
name: multi-tool-skill
description: Test skill for multi-tool installation
license: MIT
---

# Multi-Tool Skill

This skill tests installation across multiple tools.
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

	// Import skill to repository
	t.Log("Importing skill to repository")
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

	// Install skill to Claude, OpenCode, and Windsurf
	t.Log("Installing skill to Claude, OpenCode, and Windsurf")
	installer, err := NewInstallerWithTargets(tmpProject, []tools.Tool{tools.Claude, tools.OpenCode, tools.Windsurf})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	if err := installer.InstallSkill("multi-tool-skill", mgr); err != nil {
		t.Fatalf("Failed to install skill: %v", err)
	}

	// Verify installation in all three tools
	t.Log("Verifying installations")

	// Check Claude
	claudePath := filepath.Join(tmpProject, ".claude", "skills", "multi-tool-skill")
	if _, err := os.Lstat(claudePath); err != nil {
		t.Errorf("Skill not found in Claude directory: %v", err)
	} else {
		t.Logf("✓ Skill installed in Claude")
	}

	// Check OpenCode
	opencodePath := filepath.Join(tmpProject, ".opencode", "skills", "multi-tool-skill")
	if _, err := os.Lstat(opencodePath); err != nil {
		t.Errorf("Skill not found in OpenCode directory: %v", err)
	} else {
		t.Logf("✓ Skill installed in OpenCode")
	}

	// Check Windsurf
	windsurfPath := filepath.Join(tmpProject, ".windsurf", "skills", "multi-tool-skill")
	if _, err := os.Lstat(windsurfPath); err != nil {
		t.Errorf("Skill not found in Windsurf directory: %v", err)
	} else {
		t.Logf("✓ Skill installed in Windsurf")
	}
}

// TestWindsurfDetection tests that DetectExistingTools finds Windsurf when .windsurf/skills exists
func TestWindsurfDetection(t *testing.T) {
	tmpProject := t.TempDir()

	// Create .windsurf/skills directory
	windsurfSkillsDir := filepath.Join(tmpProject, ".windsurf", "skills")
	if err := os.MkdirAll(windsurfSkillsDir, 0755); err != nil {
		t.Fatalf("Failed to create .windsurf/skills directory: %v", err)
	}

	// Detect existing tools
	detected, err := tools.DetectExistingTools(tmpProject)
	if err != nil {
		t.Fatalf("DetectExistingTools failed: %v", err)
	}

	// Verify Windsurf is detected
	found := false
	for _, tool := range detected {
		if tool == tools.Windsurf {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Windsurf not detected. Found tools: %v", detected)
	} else {
		t.Logf("✓ Windsurf detected successfully")
	}
}
