package modifications

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestNewGenerator(t *testing.T) {
	repoPath := "/test/repo"
	mappings := config.TypeMappings{}

	gen := NewGenerator(repoPath, mappings, nil)

	if gen.repoPath != repoPath {
		t.Errorf("repoPath = %q, want %q", gen.repoPath, repoPath)
	}
}

func TestModificationsDir(t *testing.T) {
	repoPath := "/test/repo"
	gen := NewGenerator(repoPath, config.TypeMappings{}, nil)

	want := "/test/repo/.modifications"
	got := gen.ModificationsDir()

	if got != want {
		t.Errorf("ModificationsDir() = %q, want %q", got, want)
	}
}

func TestGenerateForResource_SkillWithModelMapping(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create skill directory with SKILL.md
	skillDir := filepath.Join(repoPath, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill directory: %v", err)
	}

	skillContent := `---
name: my-skill
description: A test skill
model: sonnet-4.5
---
# My Skill

This is the skill content.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Create mappings
	mappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
				},
			},
		},
	}

	gen := NewGenerator(repoPath, mappings, nil)

	// Load resource
	res, err := resource.LoadSkill(skillDir)
	if err != nil {
		t.Fatalf("failed to load skill: %v", err)
	}

	// Generate modifications
	tools, err := gen.GenerateForResource(res)
	if err != nil {
		t.Fatalf("GenerateForResource() error = %v", err)
	}

	// Should have generated for opencode
	if len(tools) != 1 || tools[0] != "opencode" {
		t.Errorf("GenerateForResource() tools = %v, want [opencode]", tools)
	}

	// Check modification file exists
	modPath := filepath.Join(repoPath, ".modifications", "opencode", "skills", "my-skill", "SKILL.md")
	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		t.Errorf("modification file not created at %s", modPath)
	}

	// Verify content
	modContent, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("failed to read modification file: %v", err)
	}

	// Should contain the mapped model value
	if !containsString(string(modContent), "langdock/claude-sonnet-4-5") {
		t.Errorf("modification file does not contain mapped model value")
	}

	// Should still contain the skill content
	if !containsString(string(modContent), "This is the skill content.") {
		t.Errorf("modification file does not preserve markdown content")
	}
}

func TestGenerateForResource_SkillNoClaudeMapping(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create skill directory with SKILL.md
	skillDir := filepath.Join(repoPath, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill directory: %v", err)
	}

	skillContent := `---
name: my-skill
description: A test skill
model: sonnet-4.5
---
# My Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Create mappings - only opencode, no claude
	mappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
					// No claude mapping
				},
			},
		},
	}

	gen := NewGenerator(repoPath, mappings, nil)

	res, err := resource.LoadSkill(skillDir)
	if err != nil {
		t.Fatalf("failed to load skill: %v", err)
	}

	tools, err := gen.GenerateForResource(res)
	if err != nil {
		t.Fatalf("GenerateForResource() error = %v", err)
	}

	// Should only have generated for opencode
	if len(tools) != 1 || tools[0] != "opencode" {
		t.Errorf("GenerateForResource() tools = %v, want [opencode]", tools)
	}

	// Claude modification should NOT exist
	claudeModPath := filepath.Join(repoPath, ".modifications", "claude", "skills", "my-skill", "SKILL.md")
	if _, err := os.Stat(claudeModPath); !os.IsNotExist(err) {
		t.Errorf("claude modification should not exist but found at %s", claudeModPath)
	}
}

func TestGenerateForResource_NullMapping(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create skill WITHOUT model field
	skillDir := filepath.Join(repoPath, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill directory: %v", err)
	}

	skillContent := `---
name: my-skill
description: A test skill without model
---
# My Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Create mappings with null mapping (adds field when missing)
	mappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"null": {
					"opencode": "langdock/default-model",
				},
			},
		},
	}

	gen := NewGenerator(repoPath, mappings, nil)

	res, err := resource.LoadSkill(skillDir)
	if err != nil {
		t.Fatalf("failed to load skill: %v", err)
	}

	tools, err := gen.GenerateForResource(res)
	if err != nil {
		t.Fatalf("GenerateForResource() error = %v", err)
	}

	// Should have generated for opencode
	if len(tools) != 1 || tools[0] != "opencode" {
		t.Errorf("GenerateForResource() tools = %v, want [opencode]", tools)
	}

	// Verify the model field was added
	modPath := filepath.Join(repoPath, ".modifications", "opencode", "skills", "my-skill", "SKILL.md")
	modContent, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("failed to read modification file: %v", err)
	}

	if !containsString(string(modContent), "langdock/default-model") {
		t.Errorf("modification file should contain the default model value added via null mapping")
	}
}

func TestGenerateForResource_AgentFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create agent file
	agentsDir := filepath.Join(repoPath, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents directory: %v", err)
	}

	agentContent := `---
description: A code reviewer agent
type: reviewer
model: gpt-4
---
# Code Reviewer

Review code for quality.
`
	agentPath := filepath.Join(agentsDir, "reviewer.md")
	if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	// Create mappings for agents
	mappings := config.TypeMappings{
		Agent: config.FieldMappings{
			"model": {
				"gpt-4": {
					"opencode": "langdock/gpt-4-turbo",
				},
			},
		},
	}

	gen := NewGenerator(repoPath, mappings, nil)

	res, err := resource.LoadAgent(agentPath)
	if err != nil {
		t.Fatalf("failed to load agent: %v", err)
	}

	tools, err := gen.GenerateForResource(res)
	if err != nil {
		t.Fatalf("GenerateForResource() error = %v", err)
	}

	if len(tools) != 1 || tools[0] != "opencode" {
		t.Errorf("GenerateForResource() tools = %v, want [opencode]", tools)
	}

	// Check modification file path (should be file, not directory)
	modPath := filepath.Join(repoPath, ".modifications", "opencode", "agents", "reviewer.md")
	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		t.Errorf("modification file not created at %s", modPath)
	}

	// Verify content
	modContent, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("failed to read modification file: %v", err)
	}

	if !containsString(string(modContent), "langdock/gpt-4-turbo") {
		t.Errorf("modification file does not contain mapped model value")
	}
}

func TestGenerateForResource_NoMappings(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create skill
	skillDir := filepath.Join(repoPath, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill directory: %v", err)
	}

	skillContent := `---
name: my-skill
description: A test skill
model: sonnet-4.5
---
# My Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Empty mappings
	mappings := config.TypeMappings{}

	gen := NewGenerator(repoPath, mappings, nil)

	res, err := resource.LoadSkill(skillDir)
	if err != nil {
		t.Fatalf("failed to load skill: %v", err)
	}

	tools, err := gen.GenerateForResource(res)
	if err != nil {
		t.Fatalf("GenerateForResource() error = %v", err)
	}

	// Should return empty list
	if len(tools) != 0 {
		t.Errorf("GenerateForResource() tools = %v, want []", tools)
	}

	// No modifications directory should be created
	modDir := filepath.Join(repoPath, ".modifications")
	if _, err := os.Stat(modDir); !os.IsNotExist(err) {
		t.Errorf(".modifications directory should not exist")
	}
}

func TestGenerateForResource_NoFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create skill without frontmatter
	skillDir := filepath.Join(repoPath, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill directory: %v", err)
	}

	// SKILL.md without frontmatter (but we still need to be able to load it)
	// The resource.LoadSkill requires frontmatter, so we need minimal frontmatter
	skillContent := `---
name: my-skill
description: A test skill
---
# My Skill - No model field
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Mappings for a field that doesn't exist in the file (and no null mapping)
	mappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
				},
				// No "null" mapping, so missing model field won't trigger modification
			},
		},
	}

	gen := NewGenerator(repoPath, mappings, nil)

	res, err := resource.LoadSkill(skillDir)
	if err != nil {
		t.Fatalf("failed to load skill: %v", err)
	}

	tools, err := gen.GenerateForResource(res)
	if err != nil {
		t.Fatalf("GenerateForResource() error = %v", err)
	}

	// Should return empty list (no matching mappings)
	if len(tools) != 0 {
		t.Errorf("GenerateForResource() tools = %v, want []", tools)
	}
}

func TestCleanupForResource(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create skill and generate modifications first
	skillDir := filepath.Join(repoPath, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill directory: %v", err)
	}

	skillContent := `---
name: my-skill
description: A test skill
model: sonnet-4.5
---
# My Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	mappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
					"claude":   "claude-sonnet-4",
				},
			},
		},
	}

	gen := NewGenerator(repoPath, mappings, nil)

	res, err := resource.LoadSkill(skillDir)
	if err != nil {
		t.Fatalf("failed to load skill: %v", err)
	}

	// Generate modifications first
	_, err = gen.GenerateForResource(res)
	if err != nil {
		t.Fatalf("GenerateForResource() error = %v", err)
	}

	// Verify modifications exist
	opencodeModPath := filepath.Join(repoPath, ".modifications", "opencode", "skills", "my-skill")
	claudeModPath := filepath.Join(repoPath, ".modifications", "claude", "skills", "my-skill")

	if _, err := os.Stat(opencodeModPath); os.IsNotExist(err) {
		t.Fatalf("opencode modification should exist before cleanup")
	}
	if _, err := os.Stat(claudeModPath); os.IsNotExist(err) {
		t.Fatalf("claude modification should exist before cleanup")
	}

	// Cleanup
	err = gen.CleanupForResource(res)
	if err != nil {
		t.Fatalf("CleanupForResource() error = %v", err)
	}

	// Verify modifications are removed
	if _, err := os.Stat(opencodeModPath); !os.IsNotExist(err) {
		t.Errorf("opencode modification should be removed")
	}
	if _, err := os.Stat(claudeModPath); !os.IsNotExist(err) {
		t.Errorf("claude modification should be removed")
	}
}

func TestCleanupAll(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create modifications directory with some content
	modDir := filepath.Join(repoPath, ".modifications", "opencode", "skills", "my-skill")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatalf("failed to create modifications directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	gen := NewGenerator(repoPath, config.TypeMappings{}, nil)

	// Verify .modifications exists
	if _, err := os.Stat(gen.ModificationsDir()); os.IsNotExist(err) {
		t.Fatalf(".modifications should exist before cleanup")
	}

	// Cleanup all
	err := gen.CleanupAll()
	if err != nil {
		t.Fatalf("CleanupAll() error = %v", err)
	}

	// Verify .modifications is removed
	if _, err := os.Stat(gen.ModificationsDir()); !os.IsNotExist(err) {
		t.Errorf(".modifications directory should be removed")
	}
}

func TestCleanupAll_NoModificationsDir(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create repo without modifications directory
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create repo directory: %v", err)
	}

	gen := NewGenerator(repoPath, config.TypeMappings{}, nil)

	// Should not error when .modifications doesn't exist
	err := gen.CleanupAll()
	if err != nil {
		t.Errorf("CleanupAll() should not error when .modifications doesn't exist: %v", err)
	}
}

func TestGetModificationPath_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create modification directory for skill
	modDir := filepath.Join(repoPath, ".modifications", "opencode", "skills", "my-skill")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatalf("failed to create modifications directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	gen := NewGenerator(repoPath, config.TypeMappings{}, nil)

	res := &resource.Resource{
		Name: "my-skill",
		Type: resource.Skill,
		Path: filepath.Join(repoPath, "skills", "my-skill"),
	}

	// Should return the directory path for skills
	path := gen.GetModificationPath(res, "opencode")
	if path != modDir {
		t.Errorf("GetModificationPath() = %q, want %q", path, modDir)
	}
}

func TestGetModificationPath_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create repo directory: %v", err)
	}

	gen := NewGenerator(repoPath, config.TypeMappings{}, nil)

	res := &resource.Resource{
		Name: "my-skill",
		Type: resource.Skill,
		Path: filepath.Join(repoPath, "skills", "my-skill"),
	}

	// Should return empty string when modification doesn't exist
	path := gen.GetModificationPath(res, "opencode")
	if path != "" {
		t.Errorf("GetModificationPath() = %q, want empty string", path)
	}
}

func TestGetModificationPath_AgentFile(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create modification file for agent
	modDir := filepath.Join(repoPath, ".modifications", "opencode", "agents")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatalf("failed to create modifications directory: %v", err)
	}
	modFile := filepath.Join(modDir, "reviewer.md")
	if err := os.WriteFile(modFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	gen := NewGenerator(repoPath, config.TypeMappings{}, nil)

	res := &resource.Resource{
		Name: "reviewer",
		Type: resource.Agent,
		Path: filepath.Join(repoPath, "agents", "reviewer.md"),
	}

	path := gen.GetModificationPath(res, "opencode")
	if path != modFile {
		t.Errorf("GetModificationPath() = %q, want %q", path, modFile)
	}
}

func TestHasModification(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create modification for skill
	modDir := filepath.Join(repoPath, ".modifications", "opencode", "skills", "my-skill")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatalf("failed to create modifications directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	gen := NewGenerator(repoPath, config.TypeMappings{}, nil)

	res := &resource.Resource{
		Name: "my-skill",
		Type: resource.Skill,
		Path: filepath.Join(repoPath, "skills", "my-skill"),
	}

	// Should return true for opencode
	if !gen.HasModification(res, "opencode") {
		t.Errorf("HasModification(opencode) = false, want true")
	}

	// Should return false for claude (no modification)
	if gen.HasModification(res, "claude") {
		t.Errorf("HasModification(claude) = true, want false")
	}
}

func TestGenerateForResource_NilResource(t *testing.T) {
	gen := NewGenerator("/test/repo", config.TypeMappings{}, nil)

	_, err := gen.GenerateForResource(nil)
	if err == nil {
		t.Error("GenerateForResource(nil) should return error")
	}
}

func TestCleanupForResource_NilResource(t *testing.T) {
	gen := NewGenerator("/test/repo", config.TypeMappings{}, nil)

	err := gen.CleanupForResource(nil)
	if err == nil {
		t.Error("CleanupForResource(nil) should return error")
	}
}

func TestGetModificationPath_NilResource(t *testing.T) {
	gen := NewGenerator("/test/repo", config.TypeMappings{}, nil)

	path := gen.GetModificationPath(nil, "opencode")
	if path != "" {
		t.Errorf("GetModificationPath(nil) = %q, want empty string", path)
	}
}

func TestGenerateAll(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create skills directory with a skill
	skillDir := filepath.Join(repoPath, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill directory: %v", err)
	}

	skillContent := `---
name: my-skill
description: A test skill
model: sonnet-4.5
---
# My Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Create agents directory with an agent
	agentsDir := filepath.Join(repoPath, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents directory: %v", err)
	}

	agentContent := `---
description: A code reviewer
model: gpt-4
---
# Reviewer
`
	if err := os.WriteFile(filepath.Join(agentsDir, "reviewer.md"), []byte(agentContent), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	// Create mappings
	mappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
				},
			},
		},
		Agent: config.FieldMappings{
			"model": {
				"gpt-4": {
					"opencode": "langdock/gpt-4-turbo",
				},
			},
		},
	}

	gen := NewGenerator(repoPath, mappings, nil)

	// Generate all
	err := gen.GenerateAll()
	if err != nil {
		t.Fatalf("GenerateAll() error = %v", err)
	}

	// Verify skill modification exists
	skillModPath := filepath.Join(repoPath, ".modifications", "opencode", "skills", "my-skill", "SKILL.md")
	if _, err := os.Stat(skillModPath); os.IsNotExist(err) {
		t.Errorf("skill modification not created at %s", skillModPath)
	}

	// Verify agent modification exists
	agentModPath := filepath.Join(repoPath, ".modifications", "opencode", "agents", "reviewer.md")
	if _, err := os.Stat(agentModPath); os.IsNotExist(err) {
		t.Errorf("agent modification not created at %s", agentModPath)
	}
}

func TestGenerateAll_CleansUpExistingModifications(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create pre-existing modifications directory with stale content
	staleDir := filepath.Join(repoPath, ".modifications", "opencode", "skills", "old-skill")
	if err := os.MkdirAll(staleDir, 0755); err != nil {
		t.Fatalf("failed to create stale modifications directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staleDir, "SKILL.md"), []byte("stale"), 0644); err != nil {
		t.Fatalf("failed to write stale file: %v", err)
	}

	// Create skill directory
	skillDir := filepath.Join(repoPath, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill directory: %v", err)
	}

	skillContent := `---
name: my-skill
description: A test skill
model: sonnet-4.5
---
# My Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	mappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
				},
			},
		},
	}

	gen := NewGenerator(repoPath, mappings, nil)

	// Generate all
	err := gen.GenerateAll()
	if err != nil {
		t.Fatalf("GenerateAll() error = %v", err)
	}

	// Verify stale modification is gone
	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Errorf("stale modification should be removed")
	}

	// Verify new modification exists
	newModPath := filepath.Join(repoPath, ".modifications", "opencode", "skills", "my-skill", "SKILL.md")
	if _, err := os.Stat(newModPath); os.IsNotExist(err) {
		t.Errorf("new modification not created at %s", newModPath)
	}
}

func TestGenerateForResource_CommandFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create commands directory (required for LoadCommand)
	commandsDir := filepath.Join(repoPath, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("failed to create commands directory: %v", err)
	}

	commandContent := `---
description: Deploy the application
model: opus-4
---
# Deploy

Deploy the application to production.
`
	commandPath := filepath.Join(commandsDir, "deploy.md")
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("failed to write command file: %v", err)
	}

	// Create mappings for commands
	mappings := config.TypeMappings{
		Command: config.FieldMappings{
			"model": {
				"opus-4": {
					"opencode": "langdock/claude-opus-4",
				},
			},
		},
	}

	gen := NewGenerator(repoPath, mappings, nil)

	res, err := resource.LoadCommand(commandPath)
	if err != nil {
		t.Fatalf("failed to load command: %v", err)
	}

	tools, err := gen.GenerateForResource(res)
	if err != nil {
		t.Fatalf("GenerateForResource() error = %v", err)
	}

	if len(tools) != 1 || tools[0] != "opencode" {
		t.Errorf("GenerateForResource() tools = %v, want [opencode]", tools)
	}

	// Check modification file path (should be file, not directory)
	modPath := filepath.Join(repoPath, ".modifications", "opencode", "commands", "deploy.md")
	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		t.Errorf("modification file not created at %s", modPath)
	}

	// Verify content
	modContent, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("failed to read modification file: %v", err)
	}

	if !containsString(string(modContent), "langdock/claude-opus-4") {
		t.Errorf("modification file does not contain mapped model value")
	}
}

func TestGenerateForResource_MultipleTools(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create skill directory with SKILL.md
	skillDir := filepath.Join(repoPath, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill directory: %v", err)
	}

	skillContent := `---
name: my-skill
description: A test skill
model: sonnet-4.5
---
# My Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Create mappings for multiple tools
	mappings := config.TypeMappings{
		Skill: config.FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
					"claude":   "claude-sonnet-4",
					"windsurf": "anthropic/claude-sonnet-4",
				},
			},
		},
	}

	gen := NewGenerator(repoPath, mappings, nil)

	res, err := resource.LoadSkill(skillDir)
	if err != nil {
		t.Fatalf("failed to load skill: %v", err)
	}

	tools, err := gen.GenerateForResource(res)
	if err != nil {
		t.Fatalf("GenerateForResource() error = %v", err)
	}

	// Should have generated for all 3 tools (sorted alphabetically)
	if len(tools) != 3 {
		t.Errorf("GenerateForResource() generated for %d tools, want 3", len(tools))
	}

	// Check all modification files exist
	for _, toolName := range []string{"opencode", "claude", "windsurf"} {
		modPath := filepath.Join(repoPath, ".modifications", toolName, "skills", "my-skill", "SKILL.md")
		if _, err := os.Stat(modPath); os.IsNotExist(err) {
			t.Errorf("%s modification file not created at %s", toolName, modPath)
		}
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
