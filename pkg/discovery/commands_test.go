package discovery

import (
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestDiscoverCommands_StandardLocation(t *testing.T) {
	basePath := filepath.Join("testdata", "commands-repo")

	commands, err := DiscoverCommands(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverCommands failed: %v", err)
	}

	// Should find commands in standard locations
	if len(commands) == 0 {
		t.Fatal("Expected to find commands, got none")
	}

	// Check that we found the expected commands
	expectedNames := map[string]bool{
		"test-command":     true,
		"claude-command":   true,
		"opencode-command": true,
		"duplicate":        true, // Should be deduplicated to 1
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

	// Verify deduplication - should only have one "duplicate" command
	duplicateCount := 0
	for _, cmd := range commands {
		if cmd.Name == "duplicate" {
			duplicateCount++
		}
	}
	if duplicateCount != 1 {
		t.Errorf("Expected 1 duplicate command after deduplication, got %d", duplicateCount)
	}
}

func TestDiscoverCommands_ExcludesSpecialFiles(t *testing.T) {
	basePath := filepath.Join("testdata", "commands-repo")

	commands, err := DiscoverCommands(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverCommands failed: %v", err)
	}

	// Check that SKILL.md and README.md are excluded
	for _, cmd := range commands {
		if cmd.Name == "SKILL" || cmd.Name == "README" || cmd.Name == "readme" {
			t.Errorf("Found excluded file: %s", cmd.Name)
		}
	}
}

func TestDiscoverCommands_RecursiveFallback(t *testing.T) {
	// Test with a repo that has .md files OUTSIDE commands/ directory
	// These files should be IGNORED (not parsed) per the bug fix
	basePath := filepath.Join("testdata", "recursive-repo")

	commands, err := DiscoverCommands(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverCommands failed: %v", err)
	}

	// Files outside commands/ directories should be ignored
	// This is the CORRECT behavior after the bug fix
	if len(commands) != 0 {
		t.Errorf("Expected 0 commands (files not in commands/ dir should be ignored), got %d", len(commands))
		for _, cmd := range commands {
			t.Logf("Found command: %s", cmd.Name)
		}
	}
}

func TestDiscoverCommands_ValidatesFrontmatter(t *testing.T) {
	basePath := filepath.Join("testdata", "commands-repo")

	commands, err := DiscoverCommands(basePath, "")
	if err != nil {
		t.Fatalf("DiscoverCommands failed: %v", err)
	}

	// Check that all found commands have valid frontmatter
	for _, cmd := range commands {
		if cmd.Description == "" {
			t.Errorf("Command %s has empty description", cmd.Name)
		}
		if cmd.Name == "" {
			t.Error("Found command with empty name")
		}
	}

	// Verify that invalid-command.md was not loaded (no frontmatter)
	for _, cmd := range commands {
		if cmd.Name == "invalid-command" {
			t.Error("Should not have loaded invalid-command.md (missing frontmatter)")
		}
	}
}

func TestDiscoverCommands_NonexistentPath(t *testing.T) {
	_, err := DiscoverCommands("/nonexistent/path", "")
	if err == nil {
		t.Fatal("Expected error for nonexistent path, got nil")
	}
}

func TestDiscoverCommands_WithSubpath(t *testing.T) {
	basePath := filepath.Join("testdata", "commands-repo")

	// Test discovering only from .claude/commands
	commands, err := DiscoverCommands(basePath, ".claude/commands")
	if err != nil {
		t.Fatalf("DiscoverCommands with subpath failed: %v", err)
	}

	// Should find claude-command
	foundClaude := false
	for _, cmd := range commands {
		if cmd.Name == "claude-command" {
			foundClaude = true
		}
	}

	if !foundClaude {
		t.Error("Expected to find claude-command in .claude/commands subpath")
	}
}

func TestIsExcludedFile(t *testing.T) {
	tests := []struct {
		filename string
		excluded bool
	}{
		{"SKILL.md", true},
		{"README.md", true},
		{"readme.md", true},
		{"Readme.md", true},
		{"REFERENCE.md", true},
		{"reference.md", true},
		{"Reference.md", true},
		{"test-command.md", false},
		{"my-command.md", false},
		{"command.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isExcludedMarkdownFile(tt.filename)
			if result != tt.excluded {
				t.Errorf("isExcludedFile(%q) = %v, want %v", tt.filename, result, tt.excluded)
			}
		})
	}
}

func TestDeduplicateCommands(t *testing.T) {
	basePath := filepath.Join("testdata", "commands-repo")

	// Load some test commands
	cmd1Path := filepath.Join(basePath, "commands", "test-command.md")
	cmd2Path := filepath.Join(basePath, "commands", "duplicate.md")
	cmd3Path := filepath.Join(basePath, ".claude", "commands", "duplicate.md")

	testCommands := make([]*resource.Resource, 0)

	// Use actual command files from testdata
	if cmd, err := resource.LoadCommand(cmd1Path); err == nil {
		testCommands = append(testCommands, cmd)
	}
	if cmd, err := resource.LoadCommand(cmd2Path); err == nil {
		testCommands = append(testCommands, cmd)
	}
	if cmd, err := resource.LoadCommand(cmd3Path); err == nil {
		testCommands = append(testCommands, cmd)
	}

	// Test deduplication
	unique := deduplicateCommands(testCommands)

	// Should have 2 unique commands (test-command and duplicate)
	if len(unique) != 2 {
		t.Errorf("Expected 2 unique commands, got %d", len(unique))
	}

	// Count occurrences of each name
	nameCount := make(map[string]int)
	for _, cmd := range unique {
		nameCount[cmd.Name]++
	}

	// Each name should appear exactly once
	for name, count := range nameCount {
		if count != 1 {
			t.Errorf("Command %q appears %d times, expected 1", name, count)
		}
	}
}

func TestSearchDirectory(t *testing.T) {
	basePath := filepath.Join("testdata", "commands-repo", "commands")

	commands, _ := searchDirectory(basePath, basePath)
	// Note: errors are expected for invalid files, we just ignore them for this test

	if len(commands) == 0 {
		t.Error("Expected to find commands in commands/ directory")
	}

	// Should find test-command and duplicate
	foundTest := false
	foundDuplicate := false
	for _, cmd := range commands {
		if cmd.Name == "test-command" {
			foundTest = true
		}
		if cmd.Name == "duplicate" {
			foundDuplicate = true
		}
	}

	if !foundTest {
		t.Error("Expected to find test-command")
	}
	if !foundDuplicate {
		t.Error("Expected to find duplicate")
	}
}

func TestSearchPriorityLocations(t *testing.T) {
	basePath := filepath.Join("testdata", "commands-repo")

	commands, _ := searchPriorityLocations(basePath)
	// Note: errors are expected for invalid files, we just ignore them for this test

	if len(commands) == 0 {
		t.Error("Expected to find commands in priority locations")
	}

	// Should find commands from all three priority locations
	foundTest := false
	foundClaude := false
	foundOpenCode := false

	for _, cmd := range commands {
		switch cmd.Name {
		case "test-command":
			foundTest = true
		case "claude-command":
			foundClaude = true
		case "opencode-command":
			foundOpenCode = true
		}
	}

	if !foundTest {
		t.Error("Expected to find test-command from commands/")
	}
	if !foundClaude {
		t.Error("Expected to find claude-command from .claude/commands/")
	}
	if !foundOpenCode {
		t.Error("Expected to find opencode-command from .opencode/commands/")
	}
}

func TestIsCommandsDirectory(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "plain commands dir", path: "/tmp/repo/commands", want: true},
		{name: "claude commands dir", path: "/tmp/repo/.claude/commands", want: true},
		{name: "opencode commands dir", path: "/tmp/repo/.opencode/commands", want: true},
		{name: "commands subtree file", path: "/tmp/repo/commands/build.md", want: false},
		{name: "windows commands path", path: `C:\repo\commands\build.md`, want: false},
		{name: "non commands path", path: "/tmp/repo/agents/reviewer.md", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCommandsDirectory(tt.path); got != tt.want {
				t.Fatalf("isCommandsDirectory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInCommandsSubtree(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "commands subtree", path: "/tmp/repo/commands/build.md", want: true},
		{name: "claude commands subtree", path: "/tmp/repo/.claude/commands/build.md", want: true},
		{name: "opencode commands subtree", path: "/tmp/repo/.opencode/commands/api/build.md", want: true},
		{name: "windows commands subtree", path: `C:\repo\commands\api\build.md`, want: true},
		{name: "non-commands subtree", path: "/tmp/repo/agents/reviewer.md", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInCommandsSubtree(tt.path); got != tt.want {
				t.Fatalf("isInCommandsSubtree() = %v, want %v", got, tt.want)
			}
		})
	}
}
