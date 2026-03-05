package test

import (
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
)

func TestDiscoverCommands_StandardLocation(t *testing.T) {
	fixturePath := filepath.Join("..", "testdata", "repos", "commands-nested")

	commands, err := discovery.DiscoverCommands(fixturePath, "")
	if err != nil {
		t.Fatalf("DiscoverCommands failed: %v", err)
	}

	// Should find build.md, test.md, nested/deploy.md
	if len(commands) < 3 {
		t.Errorf("Expected at least 3 commands, got %d", len(commands))
	}

	// Verify we found the expected commands
	// Note: deploy is nested, so its name is "nested/deploy"
	expectedNames := map[string]bool{
		"build":         true,
		"test":          true,
		"nested/deploy": true,
	}

	foundNames := make(map[string]bool)
	for _, cmd := range commands {
		foundNames[cmd.Name] = true
	}

	for name := range expectedNames {
		if !foundNames[name] {
			t.Errorf("Expected to find command %q, but didn't", name)
		}
	}
}

func TestDiscoverCommands_NestedCommands(t *testing.T) {
	fixturePath := filepath.Join("..", "testdata", "repos", "commands-nested")

	commands, err := discovery.DiscoverCommands(fixturePath, "")
	if err != nil {
		t.Fatalf("DiscoverCommands failed: %v", err)
	}

	// Verify nested command is found with full nested path
	foundNested := false
	for _, cmd := range commands {
		if cmd.Name == "nested/deploy" {
			foundNested = true
			if cmd.Description == "" {
				t.Errorf("Nested command has empty description")
			}
			break
		}
	}

	if !foundNested {
		t.Errorf("Nested command 'nested/deploy' not found")
	}
}

func TestDiscoverCommands_PriorityLocations(t *testing.T) {
	fixturePath := filepath.Join("..", "testdata", "repos", "dotdir-resources")

	commands, err := discovery.DiscoverCommands(fixturePath, "")
	if err != nil {
		t.Fatalf("DiscoverCommands failed: %v", err)
	}

	// Should find commands in .claude/commands
	// Note: this fixture only has .claude/commands, not .opencode/commands
	if len(commands) < 1 {
		t.Errorf("Expected at least 1 command, got %d", len(commands))
	}

	// Verify we found the claude command
	foundClaude := false

	for _, cmd := range commands {
		if cmd.Name == "claude-cmd" {
			foundClaude = true
			if cmd.Description != "Claude-specific command" {
				t.Errorf("Wrong description for claude-cmd: %s", cmd.Description)
			}
		}
	}

	if !foundClaude {
		t.Error("Did not find command 'claude-cmd' from .claude/commands")
	}
}

func TestDiscoverCommands_IgnoresNonResourceDirectories(t *testing.T) {
	fixturePath := filepath.Join("..", "testdata", "repos", "discovery-filtering")

	commands, discErrors, err := discovery.DiscoverCommandsWithErrors(fixturePath, "")
	if err != nil {
		t.Fatalf("DiscoverCommands failed: %v", err)
	}

	// Should find only the command in commands/, not the ones in documentation/ or node_modules/
	if len(commands) != 1 {
		t.Errorf("Expected exactly 1 command, got %d", len(commands))
		for _, cmd := range commands {
			t.Logf("Found command: %s", cmd.Name)
		}
	}

	// Verify we found the valid command
	if len(commands) > 0 && commands[0].Name != "valid-command" {
		t.Errorf("Expected to find 'valid-command', got '%s'", commands[0].Name)
	}

	// Should have 0 discovery errors (files outside commands/ should be ignored, not parsed)
	if len(discErrors) != 0 {
		t.Errorf("Expected 0 discovery errors, got %d", len(discErrors))
		for _, discErr := range discErrors {
			t.Logf("Discovery error: %s - %v", discErr.Path, discErr.Error)
		}
	}
}
