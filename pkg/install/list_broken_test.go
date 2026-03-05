package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

// TestListReturnsBrokenSkillSymlink verifies that List() returns a broken skill resource
// with Health == HealthBroken when the symlink target doesn't exist.
func TestListReturnsBrokenSkillSymlink(t *testing.T) {
	projectDir := t.TempDir()

	// Create .opencode/skills directory
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills directory: %v", err)
	}

	// Create a broken symlink
	brokenTarget := "/nonexistent/path/to/my-skill"
	symlinkPath := filepath.Join(skillsDir, "my-skill")
	if err := os.Symlink(brokenTarget, symlinkPath); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	// Create installer
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	// Call List()
	resources, err := installer.List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}

	// Assert: exactly one resource returned
	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}

	res := resources[0]
	if res.Name != "my-skill" {
		t.Errorf("Expected name 'my-skill', got '%s'", res.Name)
	}
	if res.Type != resource.Skill {
		t.Errorf("Expected type Skill, got '%s'", res.Type)
	}
	if res.Health != resource.HealthBroken {
		t.Errorf("Expected health HealthBroken, got '%s'", res.Health)
	}
	if res.Path != brokenTarget {
		t.Errorf("Expected path '%s', got '%s'", brokenTarget, res.Path)
	}
}

// TestListReturnsBrokenCommandSymlink verifies that List() returns a broken command resource
// with Health == HealthBroken when the symlink target doesn't exist.
func TestListReturnsBrokenCommandSymlink(t *testing.T) {
	projectDir := t.TempDir()

	// Create .opencode/commands directory
	commandsDir := filepath.Join(projectDir, ".opencode", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	// Create a broken symlink for a command (commands are .md files)
	brokenTarget := "/nonexistent/path/to/my-cmd.md"
	symlinkPath := filepath.Join(commandsDir, "my-cmd.md")
	if err := os.Symlink(brokenTarget, symlinkPath); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	// Create installer
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	// Call List()
	resources, err := installer.List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}

	// Assert
	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}

	res := resources[0]
	if res.Name != "my-cmd" {
		t.Errorf("Expected name 'my-cmd', got '%s'", res.Name)
	}
	if res.Type != resource.Command {
		t.Errorf("Expected type Command, got '%s'", res.Type)
	}
	if res.Health != resource.HealthBroken {
		t.Errorf("Expected health HealthBroken, got '%s'", res.Health)
	}
}

// TestListReturnsBrokenAgentSymlink verifies that List() returns a broken agent resource
// with Health == HealthBroken when the symlink target doesn't exist.
func TestListReturnsBrokenAgentSymlink(t *testing.T) {
	projectDir := t.TempDir()

	// Create .opencode/agents directory
	agentsDir := filepath.Join(projectDir, ".opencode", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents directory: %v", err)
	}

	// Create a broken symlink for an agent (agents are .md files)
	brokenTarget := "/nonexistent/path/to/my-agent.md"
	symlinkPath := filepath.Join(agentsDir, "my-agent.md")
	if err := os.Symlink(brokenTarget, symlinkPath); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	// Create installer
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	// Call List()
	resources, err := installer.List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}

	// Assert
	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}

	res := resources[0]
	if res.Name != "my-agent" {
		t.Errorf("Expected name 'my-agent', got '%s'", res.Name)
	}
	if res.Type != resource.Agent {
		t.Errorf("Expected type Agent, got '%s'", res.Type)
	}
	if res.Health != resource.HealthBroken {
		t.Errorf("Expected health HealthBroken, got '%s'", res.Health)
	}
}

// TestListReturnsMixedHealthResources verifies that List() correctly returns a mix of
// healthy and broken resources with appropriate health status.
func TestListReturnsMixedHealthResources(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create and add a skill to the repo
	tempSkillDir := t.TempDir()
	skillDir := filepath.Join(tempSkillDir, "healthy-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte("---\ndescription: A healthy skill\n---\n\n# Healthy Skill\n\nA healthy skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to repo: %v", err)
	}

	// Create installer and install the healthy skill
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}
	if err := installer.InstallSkill("healthy-skill", manager); err != nil {
		t.Fatalf("Failed to install skill: %v", err)
	}

	// Create a broken skill symlink
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	brokenSymlink := filepath.Join(skillsDir, "broken-skill")
	if err := os.Symlink("/nonexistent/path/to/broken-skill", brokenSymlink); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	// Create a broken command symlink
	commandsDir := filepath.Join(projectDir, ".opencode", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}
	brokenCmdSymlink := filepath.Join(commandsDir, "broken-cmd.md")
	if err := os.Symlink("/nonexistent/path/to/broken-cmd.md", brokenCmdSymlink); err != nil {
		t.Fatalf("Failed to create broken command symlink: %v", err)
	}

	// Call List()
	resources, err := installer.List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}

	// Assert: 3 resources total
	if len(resources) != 3 {
		t.Fatalf("Expected 3 resources, got %d", len(resources))
	}

	// Count healthy and broken
	healthyCount := 0
	brokenCount := 0
	for _, res := range resources {
		switch res.Health {
		case resource.HealthOK:
			healthyCount++
		case resource.HealthBroken:
			brokenCount++
		default:
			t.Errorf("Unexpected health status '%s' for resource '%s'", res.Health, res.Name)
		}
	}

	if healthyCount != 1 {
		t.Errorf("Expected 1 healthy resource, got %d", healthyCount)
	}
	if brokenCount != 2 {
		t.Errorf("Expected 2 broken resources, got %d", brokenCount)
	}
}

// TestListHealthyResourceHasHealthOK verifies that a properly installed resource
// gets Health == HealthOK from List().
func TestListHealthyResourceHasHealthOK(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create and add a skill
	tempSkillDir := t.TempDir()
	skillDir := filepath.Join(tempSkillDir, "good-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte("---\ndescription: A good skill\n---\n\n# Good Skill\n\nA good skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to repo: %v", err)
	}

	// Install the skill
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}
	if err := installer.InstallSkill("good-skill", manager); err != nil {
		t.Fatalf("Failed to install skill: %v", err)
	}

	// Call List()
	resources, err := installer.List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource, got %d", len(resources))
	}

	res := resources[0]
	if res.Health != resource.HealthOK {
		t.Errorf("Expected health HealthOK, got '%s'", res.Health)
	}
	if res.Name != "good-skill" {
		t.Errorf("Expected name 'good-skill', got '%s'", res.Name)
	}
}

// TestListBrokenSymlinkAfterTargetDeletion tests the realistic scenario where
// a resource is properly installed, then its target is deleted (e.g., repo cleanup).
func TestListBrokenSymlinkAfterTargetDeletion(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create and add a skill
	tempSkillDir := t.TempDir()
	skillDir := filepath.Join(tempSkillDir, "temp-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte("---\ndescription: A temp skill\n---\n\n# Temp Skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to repo: %v", err)
	}

	// Install the skill
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}
	if err := installer.InstallSkill("temp-skill", manager); err != nil {
		t.Fatalf("Failed to install skill: %v", err)
	}

	// Verify it's healthy first
	resources, err := installer.List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(resources) != 1 || resources[0].Health != resource.HealthOK {
		t.Fatalf("Expected 1 healthy resource initially")
	}

	// Now delete the repo skill target to simulate deletion
	repoSkillDir := filepath.Join(repoDir, "skills", "temp-skill")
	if err := os.RemoveAll(repoSkillDir); err != nil {
		t.Fatalf("Failed to remove repo skill: %v", err)
	}

	// List again — should show as broken
	resources, err = installer.List()
	if err != nil {
		t.Fatalf("List() returned error after deletion: %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("Expected 1 resource after target deletion, got %d", len(resources))
	}

	res := resources[0]
	if res.Health != resource.HealthBroken {
		t.Errorf("Expected health HealthBroken after target deletion, got '%s'", res.Health)
	}
	if res.Name != "temp-skill" {
		t.Errorf("Expected name 'temp-skill', got '%s'", res.Name)
	}
	if res.Type != resource.Skill {
		t.Errorf("Expected type Skill, got '%s'", res.Type)
	}
}

// TestListDeduplicatesBrokenAcrossTools verifies that broken resources are
// deduplicated when installed across multiple tools.
func TestListDeduplicatesBrokenAcrossTools(t *testing.T) {
	projectDir := t.TempDir()

	// Create skill directories for both Claude and OpenCode
	for _, dir := range []string{".claude/skills", ".opencode/skills"} {
		skillsDir := filepath.Join(projectDir, dir)
		if err := os.MkdirAll(skillsDir, 0755); err != nil {
			t.Fatalf("Failed to create %s: %v", dir, err)
		}

		// Create broken symlink in each
		brokenTarget := "/nonexistent/path/to/dup-skill"
		symlinkPath := filepath.Join(skillsDir, "dup-skill")
		if err := os.Symlink(brokenTarget, symlinkPath); err != nil {
			t.Fatalf("Failed to create broken symlink in %s: %v", dir, err)
		}
	}

	// Create installer with both tools
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.Claude, tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	// Call List()
	resources, err := installer.List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}

	// Should deduplicate to 1 resource
	if len(resources) != 1 {
		t.Fatalf("Expected 1 deduplicated resource, got %d", len(resources))
	}

	if resources[0].Health != resource.HealthBroken {
		t.Errorf("Expected health HealthBroken, got '%s'", resources[0].Health)
	}
}
