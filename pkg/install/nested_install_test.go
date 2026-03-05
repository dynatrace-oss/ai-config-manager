package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

// TestNestedCommandInstallation verifies that installation preserves nested structure
func TestNestedCommandInstallation(t *testing.T) {
	// Create test repository with nested command
	repoPath := t.TempDir()
	mgr := repo.NewManagerWithPath(repoPath)

	// Create nested command source
	testDir := t.TempDir()
	nestedPath := filepath.Join(testDir, "commands", "api", "v2")
	if err := os.MkdirAll(nestedPath, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	cmdPath := filepath.Join(nestedPath, "deploy.md")
	content := `---
description: Deploy API v2
---
# Deploy
`
	if err := os.WriteFile(cmdPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}

	// Import command (preserves nested structure in repo)
	sourceURL := "file://" + cmdPath
	if err := mgr.AddCommand(cmdPath, sourceURL, "file"); err != nil {
		t.Fatalf("Failed to import command: %v", err)
	}

	// Verify repo storage is nested
	expectedRepoPath := filepath.Join(repoPath, "commands", "api", "v2", "deploy.md")
	if _, err := os.Stat(expectedRepoPath); os.IsNotExist(err) {
		t.Fatalf("Command not stored in nested structure: %s", expectedRepoPath)
	}

	// Load the resource to get nested name
	res, err := resource.LoadCommandWithBase(expectedRepoPath, filepath.Join(repoPath, "commands"))
	if err != nil {
		t.Fatalf("Failed to load command: %v", err)
	}

	// Name field now contains the nested path (e.g., "api/v2/deploy")
	if res.Name != "api/v2/deploy" {
		t.Errorf("Name mismatch. Got: %s, Want: api/v2/deploy", res.Name)
	}

	// Create test project
	projectPath := t.TempDir()
	tool := tools.Claude
	toolInfo := tools.GetToolInfo(tool)
	commandsDir := filepath.Join(projectPath, toolInfo.CommandsDir)
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	// Test nested installation logic
	// For nested commands, Name contains the full path (e.g., "api/v2/deploy")
	var symlinkPath string
	if filepath.Dir(res.Name) != "." {
		// Nested command - Name contains slashes
		symlinkPath = filepath.Join(commandsDir, res.Name+".md")
		if err := os.MkdirAll(filepath.Dir(symlinkPath), 0755); err != nil {
			t.Fatalf("Failed to create nested directories: %v", err)
		}
	} else {
		// Flat command - Name is just the basename
		symlinkPath = filepath.Join(commandsDir, filepath.Base(res.Path))
	}

	if err := os.Symlink(res.Path, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Verify nested symlink created
	expectedSymlink := filepath.Join(projectPath, ".claude", "commands", "api", "v2", "deploy.md")
	if _, err := os.Lstat(expectedSymlink); os.IsNotExist(err) {
		t.Errorf("Nested symlink not created: %s", expectedSymlink)
	}

	// Verify symlink target
	target, err := os.Readlink(expectedSymlink)
	if err != nil {
		t.Errorf("Not a valid symlink: %v", err)
	}
	if target != expectedRepoPath {
		t.Errorf("Symlink target mismatch. Got: %s, Want: %s", target, expectedRepoPath)
	}

	// Verify parent directories exist
	if _, err := os.Stat(filepath.Join(commandsDir, "api", "v2")); os.IsNotExist(err) {
		t.Errorf("Parent directories not created")
	}
}
