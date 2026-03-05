package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestDiscoverAgents_StandardLocations(t *testing.T) {
	basePath := "testdata/agents-repo"

	agents, err := DiscoverAgents(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverAgents() error = %v", err)
	}

	// Should find agents in priority locations (agents/, .claude/agents/, .opencode/agents/)
	// Should NOT trigger recursive search since we found agents in priority locations
	expectedNames := map[string]bool{
		"standard-agent":  true,
		"duplicate-agent": true, // Should find the first occurrence in agents/
		"claude-agent":    true,
		"opencode-agent":  true,
	}

	if len(agents) != len(expectedNames) {
		t.Errorf("DiscoverAgents() found %d agents, want %d", len(agents), len(expectedNames))
	}

	for _, agent := range agents {
		if !expectedNames[agent.Name] {
			t.Errorf("Unexpected agent found: %s", agent.Name)
		}
		if agent.Type != resource.Agent {
			t.Errorf("Agent %s has wrong type: %v, want %v", agent.Name, agent.Type, resource.Agent)
		}
	}

	// Verify specific agents
	foundStandard := false
	foundClaude := false
	foundOpenCode := false
	for _, agent := range agents {
		switch agent.Name {
		case "standard-agent":
			foundStandard = true
			if agent.Description != "Standard agent in agents directory" {
				t.Errorf("standard-agent description = %v, want 'Standard agent in agents directory'", agent.Description)
			}
		case "claude-agent":
			foundClaude = true
			if agent.Description != "Claude format agent" {
				t.Errorf("claude-agent description = %v, want 'Claude format agent'", agent.Description)
			}
		case "opencode-agent":
			foundOpenCode = true
			if agent.Description != "OpenCode format agent" {
				t.Errorf("opencode-agent description = %v, want 'OpenCode format agent'", agent.Description)
			}
		}
	}

	if !foundStandard {
		t.Error("standard-agent not found")
	}
	if !foundClaude {
		t.Error("claude-agent not found")
	}
	if !foundOpenCode {
		t.Error("opencode-agent not found")
	}
}

func TestDiscoverAgents_RecursiveFallback(t *testing.T) {
	basePath := "testdata/recursive-only"

	agents, err := DiscoverAgents(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverAgents() error = %v", err)
	}

	// Files outside agents/ directories should be ignored
	// This is the CORRECT behavior after the bug fix
	if len(agents) != 0 {
		t.Errorf("Expected 0 agents (files not in agents/ dir should be ignored), got %d", len(agents))
		for _, agent := range agents {
			t.Logf("Found agent: %s", agent.Name)
		}
	}
}

func TestDiscoverAgents_WithSubpath(t *testing.T) {
	basePath := "testdata"
	subpath := "agents-repo"

	agents, err := DiscoverAgents(basePath, subpath)
	if err != nil {
		t.Fatalf("DiscoverAgents() error = %v", err)
	}

	// Should find same agents as TestDiscoverAgents_StandardLocations
	if len(agents) < 3 {
		t.Errorf("DiscoverAgents() found %d agents, want at least 3", len(agents))
	}
}

func TestDiscoverAgents_Deduplication(t *testing.T) {
	basePath := "testdata/agents-repo"

	agents, err := DiscoverAgents(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverAgents() error = %v", err)
	}

	// Check for duplicate names
	nameCount := make(map[string]int)
	for _, agent := range agents {
		nameCount[agent.Name]++
	}

	for name, count := range nameCount {
		if count > 1 {
			t.Errorf("Agent %s found %d times, want 1 (deduplication failed)", name, count)
		}
	}

	// Verify duplicate-agent is present (should keep first occurrence from agents/)
	foundDuplicate := false
	for _, agent := range agents {
		if agent.Name == "duplicate-agent" {
			foundDuplicate = true
			if agent.Description != "First occurrence of duplicate agent" {
				t.Errorf("duplicate-agent kept wrong occurrence: description = %v", agent.Description)
			}
			break
		}
	}

	if !foundDuplicate {
		t.Error("duplicate-agent not found after deduplication")
	}
}

func TestDiscoverAgents_ValidatesFrontmatter(t *testing.T) {
	basePath := "testdata/agents-repo"

	agents, err := DiscoverAgents(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverAgents() error = %v", err)
	}

	// All returned agents should have valid descriptions
	for _, agent := range agents {
		if agent.Description == "" {
			t.Errorf("Agent %s has empty description", agent.Name)
		}
	}

	// Verify invalid-agent (missing description) is NOT in results
	for _, agent := range agents {
		if agent.Name == "invalid-agent" {
			t.Errorf("invalid-agent should not be discovered (missing description)")
		}
	}
}

func TestDiscoverAgents_EmptyBasePath(t *testing.T) {
	_, err := DiscoverAgents("", "")
	if err == nil {
		t.Error("DiscoverAgents() expected error for empty basePath, got nil")
	}
}

func TestDiscoverAgents_NonExistentPath(t *testing.T) {
	_, err := DiscoverAgents("testdata/nonexistent", "")
	if err == nil {
		t.Error("DiscoverAgents() expected error for non-existent path, got nil")
	}
}

func TestDiscoverAgents_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	agents, err := DiscoverAgents(tmpDir, "")
	if err != nil {
		t.Fatalf("DiscoverAgents() error = %v", err)
	}

	if len(agents) != 0 {
		t.Errorf("DiscoverAgents() found %d agents in empty directory, want 0", len(agents))
	}
}

func TestDiscoverAgentsInDirectory(t *testing.T) {
	tests := []struct {
		name          string
		dirPath       string
		recursive     bool
		wantError     bool
		wantMinCount  int
		wantAgentName string
	}{
		{
			name:          "standard agents directory",
			dirPath:       "testdata/agents-repo/agents",
			recursive:     false,
			wantError:     false,
			wantMinCount:  2, // standard-agent and duplicate-agent
			wantAgentName: "standard-agent",
		},
		{
			name:          "claude agents directory",
			dirPath:       "testdata/agents-repo/.claude/agents",
			recursive:     false,
			wantError:     false,
			wantMinCount:  1,
			wantAgentName: "claude-agent",
		},
		{
			name:          "opencode agents directory",
			dirPath:       "testdata/agents-repo/.opencode/agents",
			recursive:     false,
			wantError:     false,
			wantMinCount:  1,
			wantAgentName: "opencode-agent",
		},
		{
			name:         "non-existent directory",
			dirPath:      "testdata/nonexistent",
			recursive:    false,
			wantError:    false, // Returns empty, not error
			wantMinCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents, err := discoverAgentsInDirectory(tt.dirPath, tt.recursive)
			if (err != nil) != tt.wantError {
				t.Errorf("discoverAgentsInDirectory() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if len(agents) < tt.wantMinCount {
				t.Errorf("discoverAgentsInDirectory() found %d agents, want at least %d", len(agents), tt.wantMinCount)
			}

			if tt.wantAgentName != "" {
				found := false
				for _, agent := range agents {
					if agent.Name == tt.wantAgentName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected agent %s not found", tt.wantAgentName)
				}
			}
		})
	}
}

func TestDiscoverAgentsRecursive(t *testing.T) {
	tests := []struct {
		name         string
		dirPath      string
		currentDepth int
		wantMinCount int
		wantMaxCount int
	}{
		{
			name:         "recursive search from root",
			dirPath:      "testdata/agents-repo",
			currentDepth: 0,
			wantMinCount: 4, // All agents in all locations
			wantMaxCount: 10,
		},
		{
			name:         "recursive search in nested",
			dirPath:      "testdata/agents-repo/nested",
			currentDepth: 0,
			wantMinCount: 0, // Files outside agents/ should be ignored per bug fix
			wantMaxCount: 0,
		},
		{
			name:         "max depth exceeded",
			dirPath:      "testdata/agents-repo",
			currentDepth: MaxRecursiveDepth + 1,
			wantMinCount: 0,
			wantMaxCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents, _, err := discoverAgentsRecursive(tt.dirPath, tt.currentDepth)
			if err != nil {
				t.Errorf("discoverAgentsRecursive() error = %v", err)
				return
			}

			if len(agents) < tt.wantMinCount {
				t.Errorf("discoverAgentsRecursive() found %d agents, want at least %d", len(agents), tt.wantMinCount)
			}

			if len(agents) > tt.wantMaxCount {
				t.Errorf("discoverAgentsRecursive() found %d agents, want at most %d", len(agents), tt.wantMaxCount)
			}
		})
	}
}

func TestShouldSkipDirectory(t *testing.T) {
	tests := []struct {
		name     string
		dirName  string
		wantSkip bool
	}{
		{"node_modules", "node_modules", true},
		{"git", ".git", true},
		{"vendor", "vendor", true},
		{"build", "build", true},
		{"dist", "dist", true},
		{"venv", "venv", true},
		{"normal directory", "src", false},
		{"agents directory", "agents", false},
		{"claude directory", ".claude", false},
		{"opencode directory", ".opencode", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkipDirectory(tt.dirName); got != tt.wantSkip {
				t.Errorf("shouldSkipDirectory(%s) = %v, want %v", tt.dirName, got, tt.wantSkip)
			}
		})
	}
}

func TestDeduplicateAgents(t *testing.T) {
	// Create test agents with duplicates
	agents := []*resource.Resource{
		{Name: "agent-1", Type: resource.Agent, Description: "First"},
		{Name: "agent-2", Type: resource.Agent, Description: "Second"},
		{Name: "agent-1", Type: resource.Agent, Description: "Duplicate first"},
		{Name: "agent-3", Type: resource.Agent, Description: "Third"},
		{Name: "agent-2", Type: resource.Agent, Description: "Duplicate second"},
	}

	result := deduplicateAgents(agents)

	// Should have 3 unique agents
	if len(result) != 3 {
		t.Errorf("deduplicateAgents() returned %d agents, want 3", len(result))
	}

	// Verify first occurrences are kept
	expectedDescriptions := map[string]string{
		"agent-1": "First",
		"agent-2": "Second",
		"agent-3": "Third",
	}

	for _, agent := range result {
		if expectedDesc, ok := expectedDescriptions[agent.Name]; ok {
			if agent.Description != expectedDesc {
				t.Errorf("Agent %s has description %v, want %v (should keep first occurrence)", agent.Name, agent.Description, expectedDesc)
			}
		} else {
			t.Errorf("Unexpected agent in result: %s", agent.Name)
		}
	}
}

func TestDeduplicateAgents_EmptyList(t *testing.T) {
	agents := []*resource.Resource{}
	result := deduplicateAgents(agents)

	if len(result) != 0 {
		t.Errorf("deduplicateAgents() on empty list returned %d agents, want 0", len(result))
	}
}

func TestDeduplicateAgents_NoDuplicates(t *testing.T) {
	agents := []*resource.Resource{
		{Name: "agent-1", Type: resource.Agent, Description: "First"},
		{Name: "agent-2", Type: resource.Agent, Description: "Second"},
		{Name: "agent-3", Type: resource.Agent, Description: "Third"},
	}

	result := deduplicateAgents(agents)

	if len(result) != 3 {
		t.Errorf("deduplicateAgents() returned %d agents, want 3", len(result))
	}
}

func TestDiscoverAgents_IgnoresNonMarkdownFiles(t *testing.T) {
	basePath := "testdata/agents-repo"

	agents, err := DiscoverAgents(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverAgents() error = %v", err)
	}

	// Should not find not-an-agent.txt
	for _, agent := range agents {
		if agent.Name == "not-an-agent" {
			t.Error("Non-markdown file should not be discovered as agent")
		}
	}
}

func TestDiscoverAgentsInDirectory_NotADirectory(t *testing.T) {
	// Create a regular file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not-a-dir.txt")
	err := os.WriteFile(filePath, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, errors := discoverAgentsInDirectory(filePath, false)
	if len(errors) == 0 {
		t.Error("discoverAgentsInDirectory() expected error for non-directory path, got none")
	}
}

func TestDiscoverAgents_MaxDepthRespected(t *testing.T) {
	// Create deeply nested structure beyond MaxRecursiveDepth
	tmpDir := t.TempDir()

	// Create nested directories beyond max depth
	currentPath := tmpDir
	for i := 0; i <= MaxRecursiveDepth+2; i++ {
		currentPath = filepath.Join(currentPath, "level")
		err := os.MkdirAll(currentPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create nested directory: %v", err)
		}
	}

	// Create agent at maximum depth + 2
	agentPath := filepath.Join(currentPath, "too-deep-agent.md")
	content := `---
description: Agent beyond max depth
---

# Too Deep Agent
`
	err := os.WriteFile(agentPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	// Discovery should not find this agent (beyond max depth)
	agents, err := DiscoverAgents(tmpDir, "")
	if err != nil {
		t.Fatalf("DiscoverAgents() error = %v", err)
	}

	// Should not find the too-deep agent
	for _, agent := range agents {
		if agent.Name == "too-deep-agent" {
			t.Error("Agent beyond MaxRecursiveDepth should not be discovered")
		}
	}
}
