package test

import (
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
)

func TestDiscoverAgents_StandardLocation(t *testing.T) {
	fixturePath := filepath.Join("..", "testdata", "repos", "mixed-resources")

	agents, err := discovery.DiscoverAgents(fixturePath, "")
	if err != nil {
		t.Fatalf("DiscoverAgents failed: %v", err)
	}

	if len(agents) == 0 {
		t.Errorf("Expected to find agents in standard location")
	}

	// Verify we found agent1
	foundAgent1 := false
	for _, agent := range agents {
		if agent.Name == "agent1" {
			foundAgent1 = true
			if agent.Description == "" {
				t.Errorf("Agent has empty description")
			}
			break
		}
	}

	if !foundAgent1 {
		t.Errorf("Expected to find agent1 in agents/ directory")
	}
}

func TestDiscoverAgents_PriorityLocations(t *testing.T) {
	fixturePath := filepath.Join("..", "testdata", "repos", "dotdir-resources")

	agents, err := discovery.DiscoverAgents(fixturePath, "")
	if err != nil {
		t.Fatalf("DiscoverAgents failed: %v", err)
	}

	// Should find agent in .opencode/agents/
	if len(agents) == 0 {
		t.Errorf("Expected to find agents in priority locations")
	}

	// Verify we found opencode-agent
	foundOpenCode := false
	for _, agent := range agents {
		if agent.Name == "opencode-agent" {
			foundOpenCode = true
			break
		}
	}

	if !foundOpenCode {
		t.Errorf("Expected to find opencode-agent in .opencode/agents/")
	}
}

func TestDiscoverAgents_EmptyRepository(t *testing.T) {
	fixturePath := filepath.Join("..", "testdata", "repos", "empty-repo")

	agents, err := discovery.DiscoverAgents(fixturePath, "")
	if err != nil {
		t.Fatalf("DiscoverAgents should not error on empty repo: %v", err)
	}

	if len(agents) != 0 {
		t.Errorf("Expected 0 agents in empty repo, got %d", len(agents))
	}
}

func TestDiscoverAgents_MalformedFrontmatter(t *testing.T) {
	// Test graceful error handling for malformed resources
	fixturePath := filepath.Join("..", "testdata", "repos", "malformed-resources")

	// Should not panic
	agents, err := discovery.DiscoverAgents(fixturePath, "")

	// Main goal: no panic
	// The function should handle errors gracefully and continue
	if err != nil {
		// Error is acceptable for malformed content
		t.Logf("DiscoverAgents returned error (expected for malformed content): %v", err)
	}

	// If agents were found, they should be valid
	for _, agent := range agents {
		if agent.Name == "" {
			t.Errorf("Found agent with empty name")
		}
		if agent.Description == "" {
			t.Errorf("Found agent %q with empty description", agent.Name)
		}
	}
}

func TestDiscoverAgents_IgnoresNonResourceDirectories(t *testing.T) {
	fixturePath := filepath.Join("..", "testdata", "repos", "discovery-filtering")

	agents, discErrors, err := discovery.DiscoverAgentsWithErrors(fixturePath, "")
	if err != nil {
		t.Fatalf("DiscoverAgents failed: %v", err)
	}

	// Should find only the agent in agents/, not files in documentation/ or root
	if len(agents) != 1 {
		t.Errorf("Expected exactly 1 agent, got %d", len(agents))
		for _, agent := range agents {
			t.Logf("Found agent: %s", agent.Name)
		}
	}

	// Verify we found the valid agent
	if len(agents) > 0 && agents[0].Name != "valid-agent" {
		t.Errorf("Expected to find 'valid-agent', got '%s'", agents[0].Name)
	}

	// Should have 0 discovery errors (files outside agents/ should be ignored, not parsed)
	if len(discErrors) != 0 {
		t.Errorf("Expected 0 discovery errors, got %d", len(discErrors))
		for _, discErr := range discErrors {
			t.Logf("Discovery error: %s - %v", discErr.Path, discErr.Error)
		}
	}
}
