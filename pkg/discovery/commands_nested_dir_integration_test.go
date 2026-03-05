package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// TestNestedCommandsDirectory_EndToEnd tests the complete workflow:
// 1. Discovery of commands in nested 'commands' directory
// 2. Import via AddBulk
// 3. File layout verification
// 4. Metadata creation and correctness
// 5. List() returns correct names
//
// This reproduces the bug where knowledge-base/company/commands/ was skipped
func TestNestedCommandsDirectory_EndToEnd(t *testing.T) {
	// Create temp directories
	sourceDir := t.TempDir()
	repoDir := t.TempDir()

	// Create nested commands directory structure:
	// sourceDir/knowledge-base/company/commands/dept/critical-incident/test-cmd.md
	nestedCmdsDir := filepath.Join(sourceDir, "knowledge-base", "company", "commands", "dept", "critical-incident")
	if err := os.MkdirAll(nestedCmdsDir, 0755); err != nil {
		t.Fatalf("Failed to create nested commands dir: %v", err)
	}

	cmdPath := filepath.Join(nestedCmdsDir, "test-cmd.md")
	cmdContent := `---
description: Test command in nested commands directory
---
# Test Command

This is a test command in a deeply nested commands directory.
`
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to write command file: %v", err)
	}

	// Step 1: Discovery from source root
	t.Log("Step 1: Discovering commands")
	commands, err := DiscoverCommands(sourceDir, "")
	if err != nil {
		t.Fatalf("DiscoverCommands failed: %v", err)
	}

	if len(commands) == 0 {
		t.Fatalf("Discovery found 0 commands - nested 'commands' directory was skipped!")
	}

	// Should find command with nested path relative to commands directory
	foundCommand := false
	for _, cmd := range commands {
		if cmd.Name == "dept/critical-incident/test-cmd" {
			foundCommand = true
			t.Logf("✓ Discovery found command: %s", cmd.Name)
			break
		}
	}

	if !foundCommand {
		var foundNames []string
		for _, cmd := range commands {
			foundNames = append(foundNames, cmd.Name)
		}
		t.Fatalf("Command 'dept/critical-incident/test-cmd' not found. Found: %v", foundNames)
	}

	// Step 2: Import via AddBulk (using discovered file paths, not directory)
	t.Log("Step 2: Importing commands")
	mgr := repo.NewManagerWithPath(repoDir)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Extract file paths from discovered commands (this is what CLI does)
	var commandPaths []string
	for _, cmd := range commands {
		commandPaths = append(commandPaths, cmd.Path)
	}

	opts := repo.BulkImportOptions{
		Force:        false,
		SkipExisting: false,
		DryRun:       false,
	}

	result, err := mgr.AddBulk(commandPaths, opts)
	if err != nil {
		t.Fatalf("AddBulk failed: %v", err)
	}

	if result.CommandCount != 1 {
		t.Errorf("Expected 1 command imported, got %d", result.CommandCount)
	}

	// Step 3: Verify file layout
	t.Log("Step 3: Verifying file layout")
	expectedFile := filepath.Join(repoDir, "commands", "dept", "critical-incident", "test-cmd.md")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Command file not created at expected path: %s", expectedFile)
	} else {
		t.Logf("✓ File layout correct: %s", expectedFile)
	}

	// Step 4: Verify metadata file exists
	t.Log("Step 4: Verifying metadata files")
	expectedMetadata := filepath.Join(repoDir, ".metadata", "commands", "dept-critical-incident-test-cmd-metadata.json")
	if _, err := os.Stat(expectedMetadata); os.IsNotExist(err) {
		t.Errorf("Metadata file not created at: %s", expectedMetadata)
	} else {
		t.Logf("✓ Metadata file created: %s", expectedMetadata)
	}

	// Step 5: Verify metadata content
	t.Log("Step 5: Verifying metadata content")
	data, err := os.ReadFile(expectedMetadata)
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}

	var meta metadata.ResourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	// Verify metadata fields
	if meta.Name != "dept/critical-incident/test-cmd" {
		t.Errorf("Metadata name = %q, want %q", meta.Name, "dept/critical-incident/test-cmd")
	}
	if meta.Type != resource.Command {
		t.Errorf("Metadata type = %q, want %q", meta.Type, resource.Command)
	}
	if meta.SourceURL == "" {
		t.Errorf("Metadata source_url is empty")
	}

	t.Logf("✓ Metadata content correct: name=%s, type=%s, source=%s", meta.Name, meta.Type, meta.SourceURL)

	// Step 6: Verify List() returns correct name
	t.Log("Step 6: Verifying List() returns correct names")
	cmdType := resource.Command
	resources, err := mgr.List(&cmdType)
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	foundInList := false
	for _, res := range resources {
		if res.Name == "dept/critical-incident/test-cmd" {
			foundInList = true
			t.Logf("✓ List() returns command: %s", res.Name)
			break
		}
	}

	if !foundInList {
		var listedNames []string
		for _, res := range resources {
			listedNames = append(listedNames, res.Name)
		}
		t.Errorf("Command not found in List(). Got: %v", listedNames)
	}

	t.Log("SUCCESS: All steps completed for nested commands directory import")
}
