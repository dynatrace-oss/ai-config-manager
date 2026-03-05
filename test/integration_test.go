package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/install"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

// TestCompleteWorkflow tests the complete workflow: add -> list -> install -> remove
func TestCompleteWorkflow(t *testing.T) {
	// Create temporary directories for repo and project
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	// Create test resources
	testCmdPath := filepath.Join(repoDir, "test-cmd.md")
	cmdContent := `---
description: A test command for integration testing
agent: test
model: test-model
---

# Test Command

This is a test command for integration testing.
`
	if err := os.WriteFile(testCmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	skillDir := filepath.Join(repoDir, "test-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: test-skill
description: A test skill for integration testing
license: MIT
metadata:
  author: test
  version: "1.0.0"
---

# Test Skill

This is a test skill for integration testing.
`
	if err := os.WriteFile(skillMdPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	// Add a script to the skill
	scriptPath := filepath.Join(skillDir, "scripts", "test.sh")
	scriptContent := `#!/bin/bash
echo "Test script"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	// Step 1: Create manager and add resources
	t.Log("Step 1: Adding resources to repository")
	manager := repo.NewManagerWithPath(repoDir)

	if err := manager.AddCommand(testCmdPath, "file://"+testCmdPath, "file"); err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill: %v", err)
	}

	// Step 2: List resources
	t.Log("Step 2: Listing resources")
	resources, err := manager.List(nil)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if len(resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(resources))
	}

	// Verify command exists
	cmdRes, err := manager.Get("test-cmd", resource.Command)
	if err != nil {
		t.Errorf("Failed to get command: %v", err)
	}
	if cmdRes.Name != "test-cmd" {
		t.Errorf("Command name = %v, want test-cmd", cmdRes.Name)
	}

	// Verify skill exists
	skillRes, err := manager.Get("test-skill", resource.Skill)
	if err != nil {
		t.Errorf("Failed to get skill: %v", err)
	}
	if skillRes.Name != "test-skill" {
		t.Errorf("Skill name = %v, want test-skill", skillRes.Name)
	}

	// Step 3: Install resources in project
	t.Log("Step 3: Installing resources")
	installer, err := install.NewInstaller(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	if err := installer.InstallCommand("test-cmd", manager); err != nil {
		t.Fatalf("Failed to install command: %v", err)
	}

	if err := installer.InstallSkill("test-skill", manager); err != nil {
		t.Fatalf("Failed to install skill: %v", err)
	}

	// Verify installations
	if !installer.IsInstalled("test-cmd", resource.Command) {
		t.Error("Command should be installed")
	}
	if !installer.IsInstalled("test-skill", resource.Skill) {
		t.Error("Skill should be installed")
	}

	// Verify symlinks were created correctly
	cmdSymlink := filepath.Join(projectDir, ".claude", "commands", "test-cmd.md")
	if _, err := os.Lstat(cmdSymlink); err != nil {
		t.Errorf("Command symlink not created: %v", err)
	}

	skillSymlink := filepath.Join(projectDir, ".claude", "skills", "test-skill")
	if _, err := os.Lstat(skillSymlink); err != nil {
		t.Errorf("Skill symlink not created: %v", err)
	}

	// Step 4: List installed resources
	t.Log("Step 4: Listing installed resources")
	installedResources, err := installer.List()
	if err != nil {
		t.Fatalf("Failed to list installed resources: %v", err)
	}

	if len(installedResources) != 2 {
		t.Errorf("Expected 2 installed resources, got %d", len(installedResources))
	}

	// Step 5: Uninstall resources
	t.Log("Step 5: Uninstalling resources")
	if err := installer.Uninstall("test-cmd", resource.Command, manager); err != nil {
		t.Errorf("Failed to uninstall command: %v", err)
	}

	if err := installer.Uninstall("test-skill", resource.Skill, manager); err != nil {
		t.Errorf("Failed to uninstall skill: %v", err)
	}

	// Verify uninstallations
	if installer.IsInstalled("test-cmd", resource.Command) {
		t.Error("Command should be uninstalled")
	}
	if installer.IsInstalled("test-skill", resource.Skill) {
		t.Error("Skill should be uninstalled")
	}

	// Step 6: Remove resources from repository
	t.Log("Step 6: Removing resources from repository")
	if err := manager.Remove("test-cmd", resource.Command); err != nil {
		t.Errorf("Failed to remove command: %v", err)
	}

	if err := manager.Remove("test-skill", resource.Skill); err != nil {
		t.Errorf("Failed to remove skill: %v", err)
	}

	// Verify removals
	_, err = manager.Get("test-cmd", resource.Command)
	if err == nil {
		t.Error("Command should be removed from repository")
	}

	_, err = manager.Get("test-skill", resource.Skill)
	if err == nil {
		t.Error("Skill should be removed from repository")
	}

	t.Log("Complete workflow test passed!")
}

// TestErrorCases tests various error scenarios
func TestErrorCases(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)

	t.Run("add invalid command", func(t *testing.T) {
		// Try to add a non-existent file
		err := manager.AddCommand("/nonexistent/file.md", "file://"+"/nonexistent/file.md", "file")
		if err == nil {
			t.Error("Expected error for non-existent command, got nil")
		}
	})

	t.Run("add invalid skill", func(t *testing.T) {
		// Try to add a non-existent directory
		err := manager.AddSkill("/nonexistent/skill", "file://"+"/nonexistent/skill", "file")
		if err == nil {
			t.Error("Expected error for non-existent skill, got nil")
		}
	})

	t.Run("get non-existent resource", func(t *testing.T) {
		_, err := manager.Get("nonexistent", resource.Command)
		if err == nil {
			t.Error("Expected error for non-existent resource, got nil")
		}
	})

	t.Run("remove non-existent resource", func(t *testing.T) {
		err := manager.Remove("nonexistent", resource.Command)
		if err == nil {
			t.Error("Expected error for non-existent resource, got nil")
		}
	})

	t.Run("install non-existent resource", func(t *testing.T) {
		projectDir := t.TempDir()
		installer, err := install.NewInstaller(projectDir, []tools.Tool{tools.Claude})
		if err != nil {
			t.Fatalf("Failed to create installer: %v", err)
		}

		err = installer.InstallCommand("nonexistent", manager)
		if err == nil {
			t.Error("Expected error for non-existent resource, got nil")
		}
	})

	t.Run("add duplicate resource", func(t *testing.T) {
		// Create a test command
		testCmdPath := filepath.Join(repoDir, "dup-cmd.md")
		cmdContent := `---
description: A duplicate command
---
# Duplicate
`
		if err := os.WriteFile(testCmdPath, []byte(cmdContent), 0644); err != nil {
			t.Fatalf("Failed to create test command: %v", err)
		}

		// Add it once
		if err := manager.AddCommand(testCmdPath, "file://"+testCmdPath, "file"); err != nil {
			t.Fatalf("Failed to add command: %v", err)
		}

		// AddCommand overwrites by design - use AddBulk for duplicate detection
		// Try to add again with AddBulk and Force=false
		result, err := manager.AddBulk([]string{testCmdPath}, repo.BulkImportOptions{
			Force:        false,
			SkipExisting: false,
		})
		if err != nil {
			t.Fatalf("AddBulk failed: %v", err)
		}
		if len(result.Failed) == 0 {
			t.Error("Expected duplicate error in AddBulk, got none")
		}
	})
}

// TestMultiToolInstallation tests multi-tool installation scenarios
func TestMultiToolInstallation(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)

	// Create a test command for testing
	testCmdPath := filepath.Join(repoDir, "multi-test-cmd.md")
	cmdContent := `---
description: A test command for multi-tool testing
---
# Multi-Tool Test Command
`
	if err := os.WriteFile(testCmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}
	if err := manager.AddCommand(testCmdPath, "file://"+testCmdPath, "file"); err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	t.Run("fresh project uses default tool", func(t *testing.T) {
		projectDir := t.TempDir()

		// Create installer with Claude as default
		installer, err := install.NewInstaller(projectDir, []tools.Tool{tools.Claude})
		if err != nil {
			t.Fatalf("Failed to create installer: %v", err)
		}

		// Install command
		if err := installer.InstallCommand("multi-test-cmd", manager); err != nil {
			t.Fatalf("Failed to install command: %v", err)
		}

		// Verify .claude/commands/ was created
		cmdSymlink := filepath.Join(projectDir, ".claude", "commands", "multi-test-cmd.md")
		if _, err := os.Lstat(cmdSymlink); err != nil {
			t.Errorf("Command symlink not created in .claude: %v", err)
		}

		// Verify NOT created in other directories
		opencodeSymlink := filepath.Join(projectDir, ".opencode", "commands", "multi-test-cmd.md")
		if _, err := os.Lstat(opencodeSymlink); err == nil {
			t.Error("Command symlink should not be created in .opencode when it doesn't exist")
		}
	})

	t.Run("existing opencode folder installs to opencode", func(t *testing.T) {
		projectDir := t.TempDir()

		// Create .opencode directory
		if err := os.MkdirAll(filepath.Join(projectDir, ".opencode"), 0755); err != nil {
			t.Fatalf("Failed to create .opencode directory: %v", err)
		}

		// Create installer with Claude as default (should be ignored)
		installer, err := install.NewInstaller(projectDir, []tools.Tool{tools.Claude})
		if err != nil {
			t.Fatalf("Failed to create installer: %v", err)
		}

		// Install command
		if err := installer.InstallCommand("multi-test-cmd", manager); err != nil {
			t.Fatalf("Failed to install command: %v", err)
		}

		// Verify installed to .opencode
		opencodeSymlink := filepath.Join(projectDir, ".opencode", "commands", "multi-test-cmd.md")
		if _, err := os.Lstat(opencodeSymlink); err != nil {
			t.Errorf("Command symlink not created in .opencode: %v", err)
		}

		// Verify NOT created in .claude (since it doesn't exist)
		claudeSymlink := filepath.Join(projectDir, ".claude", "commands", "multi-test-cmd.md")
		if _, err := os.Lstat(claudeSymlink); err == nil {
			t.Error("Command symlink should not be created in .claude when it doesn't exist")
		}
	})

	t.Run("multiple existing folders install to all", func(t *testing.T) {
		projectDir := t.TempDir()

		// Create both .claude and .opencode directories
		if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0755); err != nil {
			t.Fatalf("Failed to create .claude directory: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(projectDir, ".opencode"), 0755); err != nil {
			t.Fatalf("Failed to create .opencode directory: %v", err)
		}

		// Create installer with any default (should be ignored)
		installer, err := install.NewInstaller(projectDir, []tools.Tool{tools.Copilot})
		if err != nil {
			t.Fatalf("Failed to create installer: %v", err)
		}

		// Verify it detected both tools
		targets := installer.GetTargetTools()
		if len(targets) != 2 {
			t.Errorf("Expected 2 target tools, got %d", len(targets))
		}

		// Install command
		if err := installer.InstallCommand("multi-test-cmd", manager); err != nil {
			t.Fatalf("Failed to install command: %v", err)
		}

		// Verify installed to BOTH .claude and .opencode
		claudeSymlink := filepath.Join(projectDir, ".claude", "commands", "multi-test-cmd.md")
		if _, err := os.Lstat(claudeSymlink); err != nil {
			t.Errorf("Command symlink not created in .claude: %v", err)
		}

		opencodeSymlink := filepath.Join(projectDir, ".opencode", "commands", "multi-test-cmd.md")
		if _, err := os.Lstat(opencodeSymlink); err != nil {
			t.Errorf("Command symlink not created in .opencode: %v", err)
		}

		// Verify both symlinks point to the same target
		claudeTarget, _ := os.Readlink(claudeSymlink)
		opencodeTarget, _ := os.Readlink(opencodeSymlink)
		if claudeTarget != opencodeTarget {
			t.Errorf("Symlinks should point to same target: %s != %s", claudeTarget, opencodeTarget)
		}
	})

	t.Run("copilot skills only no commands", func(t *testing.T) {
		projectDir := t.TempDir()

		// Create .github/skills directory
		if err := os.MkdirAll(filepath.Join(projectDir, ".github", "skills"), 0755); err != nil {
			t.Fatalf("Failed to create .github/skills directory: %v", err)
		}

		// Create installer - should detect Copilot
		installer, err := install.NewInstaller(projectDir, []tools.Tool{tools.Claude})
		if err != nil {
			t.Fatalf("Failed to create installer: %v", err)
		}

		// Verify it detected Copilot
		targets := installer.GetTargetTools()
		if len(targets) != 1 || targets[0] != tools.Copilot {
			t.Errorf("Expected Copilot as only target, got %v", targets)
		}

		// Try to install command - should work but not create anything
		// (Copilot doesn't support commands)
		if err := installer.InstallCommand("multi-test-cmd", manager); err != nil {
			t.Fatalf("Failed to install command: %v", err)
		}

		// Verify NOT created in .github (Copilot doesn't support commands)
		githubSymlink := filepath.Join(projectDir, ".github", "commands", "multi-test-cmd.md")
		if _, err := os.Lstat(githubSymlink); err == nil {
			t.Error("Command symlink should not be created for Copilot (commands not supported)")
		}
	})
}

// TestConfigIntegration tests the config functionality
func TestConfigIntegration(t *testing.T) {
	t.Run("load config with default targets", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a config file with new format
		configPath := filepath.Join(tmpDir, "aimgr.yaml")
		configContent := "install:\n  targets: [opencode]\n"
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}

		// Load config
		cfg, err := config.Load(tmpDir)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify install targets
		if len(cfg.Install.Targets) != 1 || cfg.Install.Targets[0] != "opencode" {
			t.Errorf("Install.Targets = %v, want [opencode]", cfg.Install.Targets)
		}

		// Get default targets as types
		targets, err := cfg.GetDefaultTargets()
		if err != nil {
			t.Fatalf("Failed to get default targets: %v", err)
		}
		if len(targets) != 1 || targets[0] != tools.OpenCode {
			t.Errorf("GetDefaultTargets() = %v, want [OpenCode]", targets)
		}
	})

	t.Run("config requires explicit targets", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Load config without file (should error - no defaults)
		_, err := config.Load(tmpDir)
		if err == nil {
			t.Error("Expected error when loading config without file, got nil")
		}

		// Error message should mention install.targets
		if !strings.Contains(err.Error(), "install") {
			t.Errorf("Error should mention 'install', got: %v", err)
		}
	})

	t.Run("config validates tool names", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a config file with invalid tool
		configPath := filepath.Join(tmpDir, "aimgr.yaml")
		configContent := "install:\n  targets: [invalid]\n"
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}

		// Load config - should fail validation
		_, err := config.Load(tmpDir)
		if err == nil {
			t.Error("Expected error for invalid tool name, got nil")
		}
	})
}

// TestWithExampleResources tests using the actual example resources
func TestWithExampleResources(t *testing.T) {
	// Skip if examples don't exist (e.g., in CI without examples)
	examplesDir := "../examples"
	if _, err := os.Stat(examplesDir); os.IsNotExist(err) {
		t.Skip("Examples directory not found, skipping test")
	}

	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)

	t.Run("add sample command", func(t *testing.T) {
		sampleCmdPath := filepath.Join(examplesDir, "sample-command.md")
		if _, err := os.Stat(sampleCmdPath); err != nil {
			t.Skip("sample-command.md not found")
		}

		if err := manager.AddCommand(sampleCmdPath, "file://"+sampleCmdPath, "file"); err != nil {
			t.Errorf("Failed to add sample command: %v", err)
		}

		// Verify it was added
		res, err := manager.Get("sample-command", resource.Command)
		if err != nil {
			t.Errorf("Failed to get sample command: %v", err)
		}
		if res.Description == "" {
			t.Error("Sample command description is empty")
		}
	})

	t.Run("add sample skill", func(t *testing.T) {
		sampleSkillPath := filepath.Join(examplesDir, "sample-skill")
		if _, err := os.Stat(sampleSkillPath); err != nil {
			t.Skip("sample-skill directory not found")
		}

		if err := manager.AddSkill(sampleSkillPath, "file://"+sampleSkillPath, "file"); err != nil {
			t.Errorf("Failed to add sample skill: %v", err)
		}

		// Verify it was added
		res, err := manager.Get("sample-skill", resource.Skill)
		if err != nil {
			t.Errorf("Failed to get sample skill: %v", err)
		}
		if res.Description == "" {
			t.Error("Sample skill description is empty")
		}
		if res.License != "MIT" {
			t.Errorf("Sample skill license = %v, want MIT", res.License)
		}
	})
}

// TestAgentWorkflow tests the complete agent workflow: add -> list -> install -> uninstall
func TestAgentWorkflow(t *testing.T) {
	// Create temporary directories for repo and project
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	// Create sample agent .md file
	testAgentPath := filepath.Join(repoDir, "test-agent.md")
	agentContent := `---
description: A test agent for integration testing
type: helper
instructions: This agent helps with testing
capabilities:
  - testing
  - debugging
version: "1.0.0"
author: test
license: MIT
---

# Test Agent

This is a test agent for integration testing.

## Instructions

Follow these instructions to test the agent workflow.
`
	if err := os.WriteFile(testAgentPath, []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	// Step 1: Create manager and add agent
	t.Log("Step 1: Adding agent to repository")
	manager := repo.NewManagerWithPath(repoDir)

	if err := manager.AddAgent(testAgentPath, "file://"+testAgentPath, "file"); err != nil {
		t.Fatalf("Failed to add agent: %v", err)
	}

	// Step 2: List resources and verify agent present
	t.Log("Step 2: Listing resources and verifying agent")
	resources, err := manager.List(nil)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if len(resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(resources))
	}

	// Verify agent exists
	agentRes, err := manager.Get("test-agent", resource.Agent)
	if err != nil {
		t.Errorf("Failed to get agent: %v", err)
	}
	if agentRes.Name != "test-agent" {
		t.Errorf("Agent name = %v, want test-agent", agentRes.Name)
	}
	if agentRes.Type != resource.Agent {
		t.Errorf("Agent type = %v, want Agent", agentRes.Type)
	}
	if agentRes.Description != "A test agent for integration testing" {
		t.Errorf("Agent description = %v, want 'A test agent for integration testing'", agentRes.Description)
	}

	// Step 3: Install agent to Claude project
	t.Log("Step 3: Installing agent to Claude project")
	claudeInstaller, err := install.NewInstaller(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("Failed to create Claude installer: %v", err)
	}

	if err := claudeInstaller.InstallAgent("test-agent", manager); err != nil {
		t.Fatalf("Failed to install agent to Claude: %v", err)
	}

	// Verify installation
	if !claudeInstaller.IsInstalled("test-agent", resource.Agent) {
		t.Error("Agent should be installed in Claude")
	}

	// Verify symlink was created correctly in .claude/agents
	agentSymlink := filepath.Join(projectDir, ".claude", "agents", "test-agent.md")
	if _, err := os.Lstat(agentSymlink); err != nil {
		t.Errorf("Agent symlink not created in .claude/agents: %v", err)
	}

	// Step 4: Uninstall agent from Claude
	t.Log("Step 4: Uninstalling agent from Claude")
	if err := claudeInstaller.Uninstall("test-agent", resource.Agent, manager); err != nil {
		t.Errorf("Failed to uninstall agent from Claude: %v", err)
	}

	// Verify uninstallation
	if claudeInstaller.IsInstalled("test-agent", resource.Agent) {
		t.Error("Agent should be uninstalled from Claude")
	}

	// Verify symlink was removed
	if _, err := os.Lstat(agentSymlink); err == nil {
		t.Error("Agent symlink should be removed after uninstall")
	}

	// Step 5: Test with OpenCode
	t.Log("Step 5: Installing agent to OpenCode project")
	opencodeProjectDir := t.TempDir()

	// Create .opencode directory to trigger OpenCode detection
	if err := os.MkdirAll(filepath.Join(opencodeProjectDir, ".opencode"), 0755); err != nil {
		t.Fatalf("Failed to create .opencode directory: %v", err)
	}

	opencodeInstaller, err := install.NewInstaller(opencodeProjectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create OpenCode installer: %v", err)
	}

	if err := opencodeInstaller.InstallAgent("test-agent", manager); err != nil {
		t.Fatalf("Failed to install agent to OpenCode: %v", err)
	}

	// Verify installation
	if !opencodeInstaller.IsInstalled("test-agent", resource.Agent) {
		t.Error("Agent should be installed in OpenCode")
	}

	// Verify symlink was created correctly in .opencode/agents
	opencodeAgentSymlink := filepath.Join(opencodeProjectDir, ".opencode", "agents", "test-agent.md")
	if _, err := os.Lstat(opencodeAgentSymlink); err != nil {
		t.Errorf("Agent symlink not created in .opencode/agents: %v", err)
	}

	// Step 6: Uninstall agent from OpenCode
	t.Log("Step 6: Uninstalling agent from OpenCode")
	if err := opencodeInstaller.Uninstall("test-agent", resource.Agent, manager); err != nil {
		t.Errorf("Failed to uninstall agent from OpenCode: %v", err)
	}

	// Verify uninstallation
	if opencodeInstaller.IsInstalled("test-agent", resource.Agent) {
		t.Error("Agent should be uninstalled from OpenCode")
	}

	// Verify symlink was removed
	if _, err := os.Lstat(opencodeAgentSymlink); err == nil {
		t.Error("Agent symlink should be removed after uninstall")
	}

	t.Log("Agent workflow test passed!")
}

// TestOpenCodeImport tests importing resources from an OpenCode folder structure
func TestOpenCodeImport(t *testing.T) {
	// Create temporary directory for .opencode folder structure
	opencodeDir := t.TempDir()
	opencodePath := filepath.Join(opencodeDir, ".opencode")

	// Create .opencode subdirectories
	commandsDir := filepath.Join(opencodePath, "commands")
	skillsDir := filepath.Join(opencodePath, "skills")
	agentsDir := filepath.Join(opencodePath, "agents")

	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills directory: %v", err)
	}
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents directory: %v", err)
	}

	// Create test command
	cmdPath := filepath.Join(commandsDir, "import-test-cmd.md")
	cmdContent := `---
description: A test command for import testing
---

# Import Test Command

This command is used for testing OpenCode import functionality.
`
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Create test skill
	skillPath := filepath.Join(skillsDir, "import-test-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillMdPath := filepath.Join(skillPath, "SKILL.md")
	skillContent := `---
name: import-test-skill
description: A test skill for import testing
license: MIT
metadata:
  author: test
  version: "1.0.0"
---

# Import Test Skill

This skill is used for testing OpenCode import functionality.
`
	if err := os.WriteFile(skillMdPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	// Create test agent
	agentPath := filepath.Join(agentsDir, "import-test-agent.md")
	agentContent := `---
description: A test agent for import testing
type: assistant
instructions: Help with importing and testing
---

# Import Test Agent

This agent is used for testing OpenCode import functionality.
`
	if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	// Step 1: Scan OpenCode folder
	t.Log("Step 1: Scanning OpenCode folder")
	contents, err := resource.ScanOpenCodeFolder(opencodePath)
	if err != nil {
		t.Fatalf("Failed to scan OpenCode folder: %v", err)
	}

	// Verify correct counts
	if len(contents.CommandPaths) != 1 {
		t.Errorf("Expected 1 command, got %d", len(contents.CommandPaths))
	}
	if len(contents.SkillPaths) != 1 {
		t.Errorf("Expected 1 skill, got %d", len(contents.SkillPaths))
	}
	if len(contents.AgentPaths) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(contents.AgentPaths))
	}

	// Step 2: Import all resources using Manager.AddBulk
	t.Log("Step 2: Importing resources to repository")
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)

	// Combine all paths
	allPaths := append([]string{}, contents.CommandPaths...)
	allPaths = append(allPaths, contents.SkillPaths...)
	allPaths = append(allPaths, contents.AgentPaths...)

	opts := repo.BulkImportOptions{
		Force:        false,
		SkipExisting: false,
		DryRun:       false,
	}

	result, err := manager.AddBulk(allPaths, opts)
	if err != nil {
		t.Fatalf("Failed to import resources: %v", err)
	}

	// Verify import results
	if len(result.Added) != 3 {
		t.Errorf("Expected 3 resources added, got %d", len(result.Added))
	}
	if len(result.Failed) != 0 {
		t.Errorf("Expected 0 failures, got %d", len(result.Failed))
	}

	// Step 3: Verify all resources are in repository
	t.Log("Step 3: Verifying imported resources")
	resources, err := manager.List(nil)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if len(resources) != 3 {
		t.Errorf("Expected 3 resources in repository, got %d", len(resources))
	}

	// Verify command was imported
	cmdRes, err := manager.Get("import-test-cmd", resource.Command)
	if err != nil {
		t.Errorf("Failed to get imported command: %v", err)
	}
	if cmdRes.Name != "import-test-cmd" {
		t.Errorf("Command name = %v, want import-test-cmd", cmdRes.Name)
	}

	// Verify skill was imported
	skillRes, err := manager.Get("import-test-skill", resource.Skill)
	if err != nil {
		t.Errorf("Failed to get imported skill: %v", err)
	}
	if skillRes.Name != "import-test-skill" {
		t.Errorf("Skill name = %v, want import-test-skill", skillRes.Name)
	}

	// Verify agent was imported
	agentRes, err := manager.Get("import-test-agent", resource.Agent)
	if err != nil {
		t.Errorf("Failed to get imported agent: %v", err)
	}
	if agentRes.Name != "import-test-agent" {
		t.Errorf("Agent name = %v, want import-test-agent", agentRes.Name)
	}
	if agentRes.Type != resource.Agent {
		t.Errorf("Agent type = %v, want Agent", agentRes.Type)
	}

	// Step 4: Test installation of imported resources
	t.Log("Step 4: Testing installation of imported resources")
	projectDir := t.TempDir()

	// Create .opencode directory to trigger OpenCode detection
	if err := os.MkdirAll(filepath.Join(projectDir, ".opencode"), 0755); err != nil {
		t.Fatalf("Failed to create .opencode directory: %v", err)
	}

	installer, err := install.NewInstaller(projectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	// Install command
	if err := installer.InstallCommand("import-test-cmd", manager); err != nil {
		t.Errorf("Failed to install command: %v", err)
	}

	// Install skill
	if err := installer.InstallSkill("import-test-skill", manager); err != nil {
		t.Errorf("Failed to install skill: %v", err)
	}

	// Install agent
	if err := installer.InstallAgent("import-test-agent", manager); err != nil {
		t.Errorf("Failed to install agent: %v", err)
	}

	// Verify all are installed
	if !installer.IsInstalled("import-test-cmd", resource.Command) {
		t.Error("Command should be installed")
	}
	if !installer.IsInstalled("import-test-skill", resource.Skill) {
		t.Error("Skill should be installed")
	}
	if !installer.IsInstalled("import-test-agent", resource.Agent) {
		t.Error("Agent should be installed")
	}

	// Verify symlinks exist
	cmdSymlink := filepath.Join(projectDir, ".opencode", "commands", "import-test-cmd.md")
	if _, err := os.Lstat(cmdSymlink); err != nil {
		t.Errorf("Command symlink not created: %v", err)
	}

	skillSymlink := filepath.Join(projectDir, ".opencode", "skills", "import-test-skill")
	if _, err := os.Lstat(skillSymlink); err != nil {
		t.Errorf("Skill symlink not created: %v", err)
	}

	agentSymlink := filepath.Join(projectDir, ".opencode", "agents", "import-test-agent.md")
	if _, err := os.Lstat(agentSymlink); err != nil {
		t.Errorf("Agent symlink not created: %v", err)
	}

	t.Log("OpenCode import test passed!")
}
