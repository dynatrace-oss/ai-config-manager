package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/install"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/modifications"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

// createTestMappings creates TypeMappings for testing
func createTestMappings() config.TypeMappings {
	return config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
					"claude":   "claude-sonnet-4",
				},
			},
		},
		Agent: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
					"claude":   "claude-sonnet-4",
				},
			},
		},
		Command: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
					"claude":   "claude-sonnet-4",
				},
			},
		},
	}
}

// TestModificationsFullWorkflow tests the complete modifications workflow:
// repo add -> generate modifications -> install with tool-specific modifications
func TestModificationsFullWorkflow(t *testing.T) {
	// Setup isolated test directories
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	// Create test mappings
	mappings := createTestMappings()

	// Create test skill with model frontmatter
	sourceDir := t.TempDir()
	skillDir := filepath.Join(sourceDir, "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}
	skillContent := `---
name: test-skill
description: A test skill
model: sonnet-4.5
---
# Test Skill

This skill tests modifications.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill: %v", err)
	}

	// Step 1: Add skill to repo
	t.Log("Step 1: Adding skill to repository")
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill: %v", err)
	}

	// Step 2: Generate modifications explicitly (since config.LoadGlobal() won't work in tests)
	t.Log("Step 2: Generating modifications")
	gen := modifications.NewGenerator(repoDir, mappings, nil)
	if err := gen.GenerateAll(); err != nil {
		t.Fatalf("Failed to generate modifications: %v", err)
	}

	// Verify .modifications/ directories were created
	opencodeMod := filepath.Join(repoDir, ".modifications", "opencode", "skills", "test-skill", "SKILL.md")
	claudeMod := filepath.Join(repoDir, ".modifications", "claude", "skills", "test-skill", "SKILL.md")

	if _, err := os.Stat(opencodeMod); os.IsNotExist(err) {
		t.Errorf("OpenCode modification not created at %s", opencodeMod)
	}
	if _, err := os.Stat(claudeMod); os.IsNotExist(err) {
		t.Errorf("Claude modification not created at %s", claudeMod)
	}

	// Verify modifications have correct tool-specific values
	opencodeContent, err := os.ReadFile(opencodeMod)
	if err != nil {
		t.Fatalf("Failed to read opencode modification: %v", err)
	}
	if !strings.Contains(string(opencodeContent), "langdock/claude-sonnet-4-5") {
		t.Errorf("OpenCode modification should contain 'langdock/claude-sonnet-4-5', got:\n%s", string(opencodeContent))
	}

	claudeContent, err := os.ReadFile(claudeMod)
	if err != nil {
		t.Fatalf("Failed to read claude modification: %v", err)
	}
	if !strings.Contains(string(claudeContent), "claude-sonnet-4") {
		t.Errorf("Claude modification should contain 'claude-sonnet-4', got:\n%s", string(claudeContent))
	}

	// Step 3: Install to project (should use modifications)
	// Note: We use NewInstallerWithTargets to bypass config detection
	t.Log("Step 3: Installing skill to project")

	// Create .claude and .opencode directories to trigger installation to both
	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".opencode"), 0755); err != nil {
		t.Fatalf("Failed to create .opencode dir: %v", err)
	}

	installer, err := install.NewInstallerWithTargets(projectDir, []tools.Tool{tools.Claude, tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	if err := installer.InstallSkill("test-skill", manager); err != nil {
		t.Fatalf("Failed to install skill: %v", err)
	}

	// Verify Claude symlink exists
	claudeSymlink := filepath.Join(projectDir, ".claude", "skills", "test-skill")
	if _, err := os.Lstat(claudeSymlink); err != nil {
		t.Fatalf("Claude symlink not created: %v", err)
	}

	// Verify OpenCode symlink exists
	opencodeSymlink := filepath.Join(projectDir, ".opencode", "skills", "test-skill")
	if _, err := os.Lstat(opencodeSymlink); err != nil {
		t.Fatalf("OpenCode symlink not created: %v", err)
	}

	// Note: The install package's getSymlinkSource checks config.LoadGlobal() which
	// won't work in tests. So the symlinks will point to original paths.
	// This test verifies the modification files exist and have correct content.
}

// TestNoModificationsWithoutMappings verifies that no modifications are created
// when no mappings are provided
func TestNoModificationsWithoutMappings(t *testing.T) {
	// Setup isolated test directories
	repoDir := t.TempDir()

	// Create test skill with model field
	sourceDir := t.TempDir()
	skillDir := filepath.Join(sourceDir, "skills", "nomapping-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}
	skillContent := `---
name: nomapping-skill
description: A test skill without mappings
model: sonnet-4.5
---
# Test Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill: %v", err)
	}

	// Add skill to repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill: %v", err)
	}

	// Generate with empty mappings
	emptyMappings := config.TypeMappings{}
	gen := modifications.NewGenerator(repoDir, emptyMappings, nil)
	if err := gen.GenerateAll(); err != nil {
		t.Fatalf("Failed to run generate: %v", err)
	}

	// Verify .modifications/ directory does NOT exist (or is empty)
	modificationsDir := filepath.Join(repoDir, ".modifications")
	if _, err := os.Stat(modificationsDir); !os.IsNotExist(err) {
		// Check if it's empty (GenerateAll cleans up first)
		entries, readErr := os.ReadDir(modificationsDir)
		if readErr == nil && len(entries) > 0 {
			t.Errorf(".modifications directory should be empty when no mappings configured")
		}
	}
}

// TestNullValueMapping tests that null mappings add fields to resources
// that don't have them
func TestNullValueMapping(t *testing.T) {
	// Setup isolated test directories
	repoDir := t.TempDir()

	// Create mappings with null mapping
	mappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"null": {
					"opencode": "langdock/default-model",
				},
			},
		},
	}

	// Create test skill WITHOUT model field
	sourceDir := t.TempDir()
	skillDir := filepath.Join(sourceDir, "skills", "null-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}
	skillContent := `---
name: null-skill
description: A test skill without model field
---
# Test Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill: %v", err)
	}

	// Add skill to repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill: %v", err)
	}

	// Generate modifications
	gen := modifications.NewGenerator(repoDir, mappings, nil)
	if err := gen.GenerateAll(); err != nil {
		t.Fatalf("Failed to generate modifications: %v", err)
	}

	// Verify modification has the model field added
	modPath := filepath.Join(repoDir, ".modifications", "opencode", "skills", "null-skill", "SKILL.md")
	modContent, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("Failed to read modification: %v", err)
	}
	if !strings.Contains(string(modContent), "langdock/default-model") {
		t.Errorf("Modification should have 'langdock/default-model' from null mapping, got:\n%s", string(modContent))
	}
}

// TestSyncRegeneratesModifications tests that GenerateAll regenerates
// modifications when called again with different mappings
func TestSyncRegeneratesModifications(t *testing.T) {
	// Setup isolated test directories
	repoDir := t.TempDir()

	// Create initial mappings
	initialMappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/initial-model",
				},
			},
		},
	}

	// Create test skill
	sourceDir := t.TempDir()
	skillDir := filepath.Join(sourceDir, "skills", "sync-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}
	skillContent := `---
name: sync-skill
description: A test skill
model: sonnet-4.5
---
# Test Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill: %v", err)
	}

	// Add skill to repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill: %v", err)
	}

	// Generate initial modifications
	gen := modifications.NewGenerator(repoDir, initialMappings, nil)
	if err := gen.GenerateAll(); err != nil {
		t.Fatalf("Failed to generate initial modifications: %v", err)
	}

	// Verify initial modification
	modPath := filepath.Join(repoDir, ".modifications", "opencode", "skills", "sync-skill", "SKILL.md")
	initialContent, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("Failed to read initial modification: %v", err)
	}
	if !strings.Contains(string(initialContent), "langdock/initial-model") {
		t.Errorf("Initial modification should have 'langdock/initial-model', got:\n%s", string(initialContent))
	}

	// Update mappings with new value
	updatedMappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/updated-model",
				},
			},
		},
	}

	// Regenerate modifications (simulating sync behavior)
	gen2 := modifications.NewGenerator(repoDir, updatedMappings, nil)
	if err := gen2.GenerateAll(); err != nil {
		t.Fatalf("Failed to regenerate modifications: %v", err)
	}

	// Verify modification was updated
	updatedContent, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("Failed to read updated modification: %v", err)
	}
	if !strings.Contains(string(updatedContent), "langdock/updated-model") {
		t.Errorf("Updated modification should have 'langdock/updated-model', got:\n%s", string(updatedContent))
	}
	if strings.Contains(string(updatedContent), "langdock/initial-model") {
		t.Errorf("Updated modification should NOT have old 'langdock/initial-model'")
	}
}

// TestRemoveCleansModifications tests that removing a resource also
// removes its modifications
func TestRemoveCleansModifications(t *testing.T) {
	// Setup isolated test directories
	repoDir := t.TempDir()

	// Create mappings
	mappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/model",
					"claude":   "claude-model",
				},
			},
		},
	}

	// Create test skill
	sourceDir := t.TempDir()
	skillDir := filepath.Join(sourceDir, "skills", "remove-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}
	skillContent := `---
name: remove-skill
description: A test skill
model: sonnet-4.5
---
# Test Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill: %v", err)
	}

	// Add skill to repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill: %v", err)
	}

	// Generate modifications
	gen := modifications.NewGenerator(repoDir, mappings, nil)
	if err := gen.GenerateAll(); err != nil {
		t.Fatalf("Failed to generate modifications: %v", err)
	}

	// Verify modifications exist
	opencodeMod := filepath.Join(repoDir, ".modifications", "opencode", "skills", "remove-skill")
	claudeMod := filepath.Join(repoDir, ".modifications", "claude", "skills", "remove-skill")

	if _, err := os.Stat(opencodeMod); os.IsNotExist(err) {
		t.Fatalf("OpenCode modification should exist before remove")
	}
	if _, err := os.Stat(claudeMod); os.IsNotExist(err) {
		t.Fatalf("Claude modification should exist before remove")
	}

	// Get the resource for cleanup
	res, err := manager.Get("remove-skill", resource.Skill)
	if err != nil {
		t.Fatalf("Failed to get skill: %v", err)
	}

	// Cleanup modifications for the resource
	if err := gen.CleanupForResource(res); err != nil {
		t.Fatalf("Failed to cleanup modifications: %v", err)
	}

	// Remove the skill
	if err := manager.Remove("remove-skill", resource.Skill); err != nil {
		t.Fatalf("Failed to remove skill: %v", err)
	}

	// Verify modifications were cleaned up
	if _, err := os.Stat(opencodeMod); !os.IsNotExist(err) {
		t.Errorf("OpenCode modification should be removed")
	}
	if _, err := os.Stat(claudeMod); !os.IsNotExist(err) {
		t.Errorf("Claude modification should be removed")
	}
}

// TestAgentModifications tests modifications for agent resources
func TestAgentModifications(t *testing.T) {
	// Setup isolated test directories
	repoDir := t.TempDir()

	// Create mappings for agents
	mappings := config.TypeMappings{
		Agent: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
				},
			},
		},
	}

	// Create test agent
	sourceDir := t.TempDir()
	agentsDir := filepath.Join(sourceDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}
	agentContent := `---
description: A test agent
model: sonnet-4.5
---
# Test Agent
`
	if err := os.WriteFile(filepath.Join(agentsDir, "test-agent.md"), []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to write agent: %v", err)
	}

	// Add agent to repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.AddAgent(filepath.Join(agentsDir, "test-agent.md"), "file://"+agentsDir, "file"); err != nil {
		t.Fatalf("Failed to add agent: %v", err)
	}

	// Generate modifications
	gen := modifications.NewGenerator(repoDir, mappings, nil)
	if err := gen.GenerateAll(); err != nil {
		t.Fatalf("Failed to generate modifications: %v", err)
	}

	// Verify modification was created
	modPath := filepath.Join(repoDir, ".modifications", "opencode", "agents", "test-agent.md")
	modContent, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("Failed to read agent modification: %v", err)
	}
	if !strings.Contains(string(modContent), "langdock/claude-sonnet-4-5") {
		t.Errorf("Agent modification should have 'langdock/claude-sonnet-4-5', got:\n%s", string(modContent))
	}
}

// TestCommandModifications tests modifications for command resources
func TestCommandModifications(t *testing.T) {
	// Setup isolated test directories
	repoDir := t.TempDir()

	// Create mappings for commands
	mappings := config.TypeMappings{
		Command: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
				},
			},
		},
	}

	// Create test command
	sourceDir := t.TempDir()
	commandsDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}
	commandContent := `---
description: A test command
model: sonnet-4.5
---
# Test Command
`
	if err := os.WriteFile(filepath.Join(commandsDir, "test-cmd.md"), []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}

	// Add command to repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.AddCommand(filepath.Join(commandsDir, "test-cmd.md"), "file://"+commandsDir, "file"); err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Generate modifications
	gen := modifications.NewGenerator(repoDir, mappings, nil)
	if err := gen.GenerateAll(); err != nil {
		t.Fatalf("Failed to generate modifications: %v", err)
	}

	// Verify modification was created
	modPath := filepath.Join(repoDir, ".modifications", "opencode", "commands", "test-cmd.md")
	modContent, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("Failed to read command modification: %v", err)
	}
	if !strings.Contains(string(modContent), "langdock/claude-sonnet-4-5") {
		t.Errorf("Command modification should have 'langdock/claude-sonnet-4-5', got:\n%s", string(modContent))
	}
}

// TestMultipleToolMappings tests that modifications are generated for multiple tools
func TestMultipleToolMappings(t *testing.T) {
	// Setup isolated test directories
	repoDir := t.TempDir()

	// Create mappings for multiple tools
	mappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
					"claude":   "claude-sonnet-4",
					"windsurf": "windsurf-sonnet-4.5",
				},
			},
		},
	}

	// Create test skill
	sourceDir := t.TempDir()
	skillDir := filepath.Join(sourceDir, "skills", "multi-tool-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}
	skillContent := `---
name: multi-tool-skill
description: A test skill
model: sonnet-4.5
---
# Test Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill: %v", err)
	}

	// Add skill to repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill: %v", err)
	}

	// Generate modifications
	gen := modifications.NewGenerator(repoDir, mappings, nil)
	if err := gen.GenerateAll(); err != nil {
		t.Fatalf("Failed to generate modifications: %v", err)
	}

	// Verify modifications for all three tools
	testCases := []struct {
		tool     string
		expected string
	}{
		{"opencode", "langdock/claude-sonnet-4-5"},
		{"claude", "claude-sonnet-4"},
		{"windsurf", "windsurf-sonnet-4.5"},
	}

	for _, tc := range testCases {
		modPath := filepath.Join(repoDir, ".modifications", tc.tool, "skills", "multi-tool-skill", "SKILL.md")
		modContent, err := os.ReadFile(modPath)
		if err != nil {
			t.Errorf("Failed to read %s modification: %v", tc.tool, err)
			continue
		}
		if !strings.Contains(string(modContent), tc.expected) {
			t.Errorf("%s modification should have '%s', got:\n%s", tc.tool, tc.expected, string(modContent))
		}
	}
}
