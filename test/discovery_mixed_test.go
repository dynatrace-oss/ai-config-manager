package test

import (
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
)

func TestDiscoverMixed_AllResourceTypes(t *testing.T) {
	fixturePath := filepath.Join("..", "testdata", "repos", "mixed-resources")

	// Discover all three types
	skills, errS := discovery.DiscoverSkills(fixturePath, "")
	commands, errC := discovery.DiscoverCommands(fixturePath, "")
	agents, errA := discovery.DiscoverAgents(fixturePath, "")

	if errS != nil || errC != nil || errA != nil {
		t.Fatalf("Discovery failed: skills=%v, commands=%v, agents=%v", errS, errC, errA)
	}

	// Should find at least one of each
	if len(skills) == 0 {
		t.Errorf("No skills found in mixed-resources")
	}
	if len(commands) == 0 {
		t.Errorf("No commands found in mixed-resources")
	}
	if len(agents) == 0 {
		t.Errorf("No agents found in mixed-resources")
	}

	// Verify specific resources
	foundSkill1 := false
	for _, skill := range skills {
		if skill.Name == "skill1" {
			foundSkill1 = true
			break
		}
	}
	if !foundSkill1 {
		t.Errorf("Expected to find skill1")
	}

	foundCmd1 := false
	for _, cmd := range commands {
		if cmd.Name == "cmd1" {
			foundCmd1 = true
			break
		}
	}
	if !foundCmd1 {
		t.Errorf("Expected to find cmd1")
	}

	foundAgent1 := false
	for _, agent := range agents {
		if agent.Name == "agent1" {
			foundAgent1 = true
			break
		}
	}
	if !foundAgent1 {
		t.Errorf("Expected to find agent1")
	}
}

func TestDiscoverMixed_WithSubpaths(t *testing.T) {
	fixturePath := filepath.Join("..", "testdata", "repos", "subpath-test")

	// Test discovery with subpath
	skills, err := discovery.DiscoverSkills(fixturePath, "level1/level2")
	if err != nil {
		t.Fatalf("Discovery with subpath failed: %v", err)
	}

	if len(skills) == 0 {
		t.Errorf("Expected to find skills in subpath")
	}

	// Verify we found deep-skill
	foundDeepSkill := false
	for _, skill := range skills {
		if skill.Name == "deep-skill" {
			foundDeepSkill = true
			if skill.Description == "" {
				t.Errorf("Skill has empty description")
			}
			break
		}
	}

	if !foundDeepSkill {
		t.Errorf("Expected to find deep-skill in subpath level1/level2")
	}

	// Test with different discovery functions
	commands, err := discovery.DiscoverCommands(fixturePath, "level1/level2")
	if err != nil {
		t.Fatalf("Command discovery with subpath failed: %v", err)
	}
	// Commands might not exist, but should not error
	_ = commands

	agents, err := discovery.DiscoverAgents(fixturePath, "level1/level2")
	if err != nil {
		t.Fatalf("Agent discovery with subpath failed: %v", err)
	}
	// Agents might not exist, but should not error
	_ = agents
}
