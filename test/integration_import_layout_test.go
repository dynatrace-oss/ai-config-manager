//go:build integration

package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// TestImportLayout_NestedCommands verifies that importing resources creates
// the correct on-disk layout with proper metadata files and content.
func TestImportLayout_NestedCommands(t *testing.T) {
	// Create temporary repository
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Initialize repo manager
	mgr := repo.NewManagerWithPath(repoPath)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Get fixture path (relative to test directory)
	fixturePath, err := filepath.Abs("../testdata/repos/comprehensive-fixture")
	if err != nil {
		t.Fatalf("Failed to get fixture path: %v", err)
	}

	// Import commands from fixture
	commandsPath := filepath.Join(fixturePath, "commands")
	commands := []string{
		filepath.Join(commandsPath, "api", "deploy.md"),
		filepath.Join(commandsPath, "api", "status.md"),
		filepath.Join(commandsPath, "db", "migrate.md"),
		filepath.Join(commandsPath, "test.md"),
	}

	for _, cmdPath := range commands {
		if err := mgr.AddCommand(cmdPath, "file://"+cmdPath, "file"); err != nil {
			t.Fatalf("Failed to add command %s: %v", cmdPath, err)
		}
	}

	// Verify file layout
	t.Run("FileLayout", func(t *testing.T) {
		assertFileExists(t, filepath.Join(repoPath, "commands", "api", "deploy.md"))
		assertFileExists(t, filepath.Join(repoPath, "commands", "api", "status.md"))
		assertFileExists(t, filepath.Join(repoPath, "commands", "db", "migrate.md"))
		assertFileExists(t, filepath.Join(repoPath, "commands", "test.md"))
	})

	// Verify metadata filenames (slashes escaped)
	t.Run("MetadataFilenames", func(t *testing.T) {
		assertFileExists(t, filepath.Join(repoPath, ".metadata", "commands", "api-deploy-metadata.json"))
		assertFileExists(t, filepath.Join(repoPath, ".metadata", "commands", "api-status-metadata.json"))
		assertFileExists(t, filepath.Join(repoPath, ".metadata", "commands", "db-migrate-metadata.json"))
		assertFileExists(t, filepath.Join(repoPath, ".metadata", "commands", "test-metadata.json"))
	})

	// Verify metadata content (nested names preserved)
	t.Run("MetadataContent", func(t *testing.T) {
		assertMetadataName(t, filepath.Join(repoPath, ".metadata", "commands", "api-deploy-metadata.json"), "api/deploy")
		assertMetadataName(t, filepath.Join(repoPath, ".metadata", "commands", "api-status-metadata.json"), "api/status")
		assertMetadataName(t, filepath.Join(repoPath, ".metadata", "commands", "db-migrate-metadata.json"), "db/migrate")
		assertMetadataName(t, filepath.Join(repoPath, ".metadata", "commands", "test-metadata.json"), "test")
	})

	// Verify List() returns correct names
	t.Run("ListCommands", func(t *testing.T) {
		cmdType := resource.Command
		resources, err := mgr.List(&cmdType)
		if err != nil {
			t.Fatalf("Failed to list commands: %v", err)
		}

		expectedNames := map[string]bool{
			"api/deploy": false,
			"api/status": false,
			"db/migrate": false,
			"test":       false,
		}

		for _, res := range resources {
			if res.Type == resource.Command {
				if _, ok := expectedNames[res.Name]; ok {
					expectedNames[res.Name] = true
				} else {
					t.Errorf("Unexpected command name: %s", res.Name)
				}
			}
		}

		for name, found := range expectedNames {
			if !found {
				t.Errorf("Expected command not found: %s", name)
			}
		}
	})
}

// assertMetadataName checks that metadata file contains the expected name
func assertMetadataName(t *testing.T, metadataPath, expectedName string) {
	t.Helper()

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata file %s: %v", metadataPath, err)
	}

	var meta metadata.ResourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("Failed to unmarshal metadata from %s: %v", metadataPath, err)
	}

	if meta.Name != expectedName {
		t.Errorf("Metadata name mismatch in %s: got %q, want %q", metadataPath, meta.Name, expectedName)
	}
}
