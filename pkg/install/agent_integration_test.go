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

// TestAgentInstallationWorkflow tests the complete agent workflow:
// import → install → list → uninstall
func TestAgentInstallationWorkflow(t *testing.T) {
	// Setup: Create test agent in source directory
	tmpSource := t.TempDir()
	agentFile := filepath.Join(tmpSource, "workflow-agent.md")
	agentContent := `---
description: Test agent for workflow validation
type: code-assistant
version: "1.0.0"
author: Test Author
license: MIT
---

# Workflow Agent

This agent tests the complete installation workflow.

## Purpose

Verify that agents can be imported, installed, listed, and uninstalled correctly.

## Capabilities

- Code analysis
- Documentation generation
- Testing assistance
`
	if err := os.WriteFile(agentFile, []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	// Setup temp repository
	tmpRepo := t.TempDir()
	mgr := repo.NewManagerWithPath(tmpRepo)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Setup temp project directory
	tmpProject := t.TempDir()

	// Step 1: Import agent to repository
	t.Log("Step 1: Importing agent to repository")
	if err := mgr.AddAgent(agentFile, "file://"+agentFile, "file"); err != nil {
		t.Fatalf("Failed to import agent: %v", err)
	}
	t.Logf("✓ Agent imported successfully")

	// Verify agent exists in repository
	agentPath := mgr.GetPath("workflow-agent", resource.Agent)
	if _, err := os.Stat(agentPath); err != nil {
		t.Fatalf("Agent not found in repository: %v", err)
	}

	// Step 2: Install agent to Claude
	t.Log("Step 2: Installing agent to Claude")
	installer, err := NewInstallerWithTargets(tmpProject, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	if err := installer.InstallAgent("workflow-agent", mgr); err != nil {
		t.Fatalf("Failed to install agent: %v", err)
	}
	t.Logf("✓ Agent installed to Claude")

	// Step 3: Verify .claude/agents/workflow-agent.md exists
	t.Log("Step 3: Verifying installation in .claude/agents/")
	agentInstallPath := filepath.Join(tmpProject, ".claude", "agents", "workflow-agent.md")
	info, err := os.Lstat(agentInstallPath)
	if err != nil {
		t.Fatalf("Agent not found at %s: %v", agentInstallPath, err)
	}

	// Verify it's a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Expected symlink at %s, but got regular file", agentInstallPath)
	}

	// Verify symlink target
	target, err := os.Readlink(agentInstallPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	expectedTarget := filepath.Join(tmpRepo, "agents", "workflow-agent.md")
	if target != expectedTarget {
		t.Errorf("Symlink target = %s, want %s", target, expectedTarget)
	}
	t.Logf("✓ Agent correctly symlinked to repository")

	// Verify agent file exists in the target
	if _, err := os.Stat(target); err != nil {
		t.Errorf("Agent file not found in symlink target: %v", err)
	}
	t.Logf("✓ Agent file exists in repository")

	// Step 4: List agents
	t.Log("Step 4: Listing installed agents")
	installed, err := installer.List()
	if err != nil {
		t.Fatalf("Failed to list installed agents: %v", err)
	}

	// Step 5: Verify agent appears in list
	t.Log("Step 5: Verifying agent in list")
	found := false
	for _, res := range installed {
		if res.Type == resource.Agent && res.Name == "workflow-agent" {
			found = true
			if res.Description != "Test agent for workflow validation" {
				t.Errorf("Agent description = %s, want 'Test agent for workflow validation'", res.Description)
			}
			if res.Version != "1.0.0" {
				t.Errorf("Agent version = %s, want '1.0.0'", res.Version)
			}
			break
		}
	}
	if !found {
		t.Errorf("Agent 'workflow-agent' not found in list")
	}
	t.Logf("✓ Agent found in list with correct metadata")

	// Step 6: Verify IsInstalled returns true
	t.Log("Step 6: Verifying IsInstalled status")
	if !installer.IsInstalled("workflow-agent", resource.Agent) {
		t.Error("IsInstalled() should return true for installed agent")
	}
	t.Logf("✓ IsInstalled correctly reports agent as installed")

	// Step 7: Uninstall agent
	t.Log("Step 7: Uninstalling agent")
	if err := installer.Uninstall("workflow-agent", resource.Agent, mgr); err != nil {
		t.Fatalf("Failed to uninstall agent: %v", err)
	}
	t.Logf("✓ Agent uninstalled")

	// Step 8: Verify .claude/agents/workflow-agent.md is removed
	t.Log("Step 8: Verifying agent was removed")
	if _, err := os.Lstat(agentInstallPath); err == nil {
		t.Errorf("Agent still exists at %s after uninstall", agentInstallPath)
	} else if !os.IsNotExist(err) {
		t.Errorf("Unexpected error checking agent path: %v", err)
	}
	t.Logf("✓ Agent removed from .claude/agents/")

	// Verify agent no longer in list
	installed, err = installer.List()
	if err != nil {
		t.Fatalf("Failed to list installed agents after uninstall: %v", err)
	}
	for _, res := range installed {
		if res.Type == resource.Agent && res.Name == "workflow-agent" {
			t.Errorf("Agent 'workflow-agent' still in list after uninstall")
		}
	}
	t.Logf("✓ Agent not in list after uninstall")

	// Verify IsInstalled returns false
	if installer.IsInstalled("workflow-agent", resource.Agent) {
		t.Error("IsInstalled() should return false after uninstall")
	}
	t.Logf("✓ IsInstalled correctly reports agent as uninstalled")
}

// TestMultiToolAgentInstallation verifies agents can be installed to multiple tools
func TestMultiToolAgentInstallation(t *testing.T) {
	// Setup: Create test agent
	tmpSource := t.TempDir()
	agentFile := filepath.Join(tmpSource, "multi-tool-agent.md")
	agentContent := `---
description: Test agent for multi-tool installation
type: general-assistant
version: "2.0.0"
---

# Multi-Tool Agent

This agent tests installation to multiple tools simultaneously.

## Supported Tools

- Claude Code
- OpenCode

Note: Copilot and Windsurf do not support agents.
`
	if err := os.WriteFile(agentFile, []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	// Setup repository
	tmpRepo := t.TempDir()
	mgr := repo.NewManagerWithPath(tmpRepo)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Import agent
	if err := mgr.AddAgent(agentFile, "file://"+agentFile, "file"); err != nil {
		t.Fatalf("Failed to import agent: %v", err)
	}

	// Setup project
	tmpProject := t.TempDir()

	// Install to multiple tools: Claude, OpenCode, and Copilot
	// (Copilot should be skipped since it doesn't support agents)
	t.Log("Installing agent to Claude, OpenCode, and Copilot")
	allTools := []tools.Tool{tools.Claude, tools.OpenCode, tools.Copilot}
	installer, err := NewInstallerWithTargets(tmpProject, allTools)
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	if err := installer.InstallAgent("multi-tool-agent", mgr); err != nil {
		t.Fatalf("Failed to install agent to multiple tools: %v", err)
	}

	// Verify installation in tools that support agents
	supportedPaths := map[tools.Tool]string{
		tools.Claude:   filepath.Join(tmpProject, ".claude", "agents", "multi-tool-agent.md"),
		tools.OpenCode: filepath.Join(tmpProject, ".opencode", "agents", "multi-tool-agent.md"),
	}

	for tool, expectedPath := range supportedPaths {
		t.Logf("Verifying installation for %s at %s", tool, expectedPath)

		info, err := os.Lstat(expectedPath)
		if err != nil {
			t.Errorf("Agent not found for %s at %s: %v", tool, expectedPath, err)
			continue
		}

		// Verify it's a symlink
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("Expected symlink for %s, but got regular file", tool)
			continue
		}

		// Verify target
		target, err := os.Readlink(expectedPath)
		if err != nil {
			t.Errorf("Failed to read symlink for %s: %v", tool, err)
			continue
		}

		expectedTarget := filepath.Join(tmpRepo, "agents", "multi-tool-agent.md")
		if target != expectedTarget {
			t.Errorf("Symlink target for %s = %s, want %s", tool, target, expectedTarget)
		}

		t.Logf("✓ %s installation verified", tool)
	}

	// Verify Copilot did NOT create an agents directory
	t.Log("Verifying Copilot did not create agents directory")
	copilotAgentsDir := filepath.Join(tmpProject, ".github", "agents")
	if _, err := os.Stat(copilotAgentsDir); err == nil {
		t.Error("Copilot should not create agents directory (agents not supported)")
	}
	t.Logf("✓ Copilot correctly skipped agent installation")

	// List should return only one agent (deduplicated)
	t.Log("Verifying list deduplication")
	installed, err := installer.List()
	if err != nil {
		t.Fatalf("Failed to list installed agents: %v", err)
	}

	agentCount := 0
	for _, res := range installed {
		if res.Type == resource.Agent && res.Name == "multi-tool-agent" {
			agentCount++
		}
	}

	if agentCount != 1 {
		t.Errorf("Expected 1 agent in list (deduplicated), got %d", agentCount)
	}
	t.Logf("✓ Agent correctly deduplicated in list")

	// Uninstall should remove from all tools
	t.Log("Uninstalling from all tools")
	if err := installer.Uninstall("multi-tool-agent", resource.Agent, mgr); err != nil {
		t.Fatalf("Failed to uninstall agent: %v", err)
	}

	// Verify removal from all supported tools
	for tool, expectedPath := range supportedPaths {
		if _, err := os.Lstat(expectedPath); err == nil {
			t.Errorf("Agent still exists for %s after uninstall", tool)
		} else if !os.IsNotExist(err) {
			t.Errorf("Unexpected error checking %s path: %v", tool, err)
		}
	}
	t.Logf("✓ Agent removed from all tools")
}

// TestAgentWithFixtures tests using agents from the testdata fixtures
func TestAgentWithFixtures(t *testing.T) {
	// Use existing fixture agents
	fixturePath := filepath.Join("..", "..", "testdata", "repos", "mixed-resources", "agents")

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

	// Import fixture agent
	t.Log("Importing fixture agent")
	agentPath := filepath.Join(fixturePath, "agent1.md")

	if err := mgr.AddAgent(agentPath, "file://"+agentPath, "file"); err != nil {
		t.Fatalf("Failed to import fixture agent: %v", err)
	}
	t.Logf("✓ Imported fixture agent")

	// Setup project and install to Claude
	tmpProject := t.TempDir()
	installer, err := NewInstallerWithTargets(tmpProject, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	// Install agent
	if err := installer.InstallAgent("agent1", mgr); err != nil {
		t.Fatalf("Failed to install agent1: %v", err)
	}
	t.Logf("✓ Installed fixture agent to Claude")

	// Verify it's installed
	installPath := filepath.Join(tmpProject, ".claude", "agents", "agent1.md")
	info, err := os.Lstat(installPath)
	if err != nil {
		t.Errorf("Agent not found at %s: %v", installPath, err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Expected symlink at %s", installPath)
	}

	// List and verify it appears
	installed, err := installer.List()
	if err != nil {
		t.Fatalf("Failed to list installed agents: %v", err)
	}

	foundAgent := false
	for _, res := range installed {
		if res.Type == resource.Agent && res.Name == "agent1" {
			foundAgent = true
			if res.Description != "First agent in mixed repository" {
				t.Errorf("Agent description = %s, want 'First agent in mixed repository'", res.Description)
			}
			break
		}
	}

	if !foundAgent {
		t.Error("Fixture agent not found in list")
	}
	t.Logf("✓ Fixture agent appears in list with correct metadata")
}

// TestAgentToolSupport verifies which tools support agents
func TestAgentToolSupport(t *testing.T) {
	tests := []struct {
		tool          tools.Tool
		shouldSupport bool
		expectedDir   string
	}{
		{
			tool:          tools.Claude,
			shouldSupport: true,
			expectedDir:   ".claude/agents",
		},
		{
			tool:          tools.OpenCode,
			shouldSupport: true,
			expectedDir:   ".opencode/agents",
		},
		{
			tool:          tools.Copilot,
			shouldSupport: false,
			expectedDir:   "",
		},
		{
			tool:          tools.Windsurf,
			shouldSupport: false,
			expectedDir:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.tool.String(), func(t *testing.T) {
			toolInfo := tools.GetToolInfo(tt.tool)

			if toolInfo.SupportsAgents != tt.shouldSupport {
				t.Errorf("Tool %s: SupportsAgents = %v, want %v",
					tt.tool, toolInfo.SupportsAgents, tt.shouldSupport)
			}

			if toolInfo.AgentsDir != tt.expectedDir {
				t.Errorf("Tool %s: AgentsDir = %s, want %s",
					tt.tool, toolInfo.AgentsDir, tt.expectedDir)
			}

			if tt.shouldSupport {
				t.Logf("✓ %s correctly supports agents", tt.tool)
			} else {
				t.Logf("✓ %s correctly does not support agents", tt.tool)
			}
		})
	}
}

// TestBrokenSymlinkReplacement verifies that broken agent symlinks are replaced
func TestBrokenSymlinkReplacement(t *testing.T) {
	// Setup test repository with an agent
	tmpRepo := t.TempDir()
	manager := repo.NewManagerWithPath(tmpRepo)

	// Create test agent
	agentFile := filepath.Join(tmpRepo, "broken-link-agent.md")
	agentContent := `---
description: Agent for testing broken symlink replacement
type: assistant
---

# Broken Link Agent
`
	if err := os.WriteFile(agentFile, []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	// Add agent to repository
	if err := manager.AddAgent(agentFile, "file://"+agentFile, "file"); err != nil {
		t.Fatalf("Failed to add agent to repo: %v", err)
	}

	// Setup project directory
	projectDir := t.TempDir()
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("NewInstallerWithTargets() error = %v", err)
	}

	// Create a broken symlink manually
	agentsDir := filepath.Join(projectDir, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents directory: %v", err)
	}

	brokenSymlink := filepath.Join(agentsDir, "broken-link-agent.md")
	// Create symlink pointing to non-existent location
	if err := os.Symlink("/nonexistent/path/agent.md", brokenSymlink); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	t.Log("Created broken symlink")

	// Install agent - should replace the broken symlink
	if err := installer.InstallAgent("broken-link-agent", manager); err != nil {
		t.Fatalf("InstallAgent() should replace broken symlink, got error: %v", err)
	}

	t.Log("✓ Broken symlink replaced successfully")

	// Verify symlink now points to correct location
	target, err := os.Readlink(brokenSymlink)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	expectedTarget := manager.GetPath("broken-link-agent", resource.Agent)
	if target != expectedTarget {
		t.Errorf("Symlink target = %v, want %v", target, expectedTarget)
	}

	// Verify the target actually exists (symlink is not broken)
	if _, err := os.Stat(brokenSymlink); err != nil {
		t.Errorf("Symlink should now point to valid location, got error: %v", err)
	}

	t.Log("✓ Symlink now points to correct location")
}
