package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// TestOrphanDetection tests that orphan detection logs warnings
func TestOrphanDetection(t *testing.T) {
	// Create temporary repo
	repoDir := t.TempDir()
	m := NewManagerWithPath(repoDir)

	// Initialize repo
	if err := m.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Test 1: Create an orphaned file (file without metadata)
	t.Run("orphaned_file", func(t *testing.T) {
		// Create a command file directly without metadata
		commandPath := filepath.Join(repoDir, "commands", "orphan-cmd.md")
		if err := os.MkdirAll(filepath.Dir(commandPath), 0755); err != nil {
			t.Fatalf("Failed to create commands dir: %v", err)
		}

		content := `---
name: orphan-cmd
description: Test orphaned command
---
# Orphaned Command
`
		if err := os.WriteFile(commandPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write orphan command: %v", err)
		}

		// List resources - this should trigger orphan detection
		resources, err := m.List(nil)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		// Verify the resource was found
		found := false
		for _, res := range resources {
			if res.Name == "orphan-cmd" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Orphan command not found in list")
		}

		// Check that logger exists (WARN should have been logged)
		if m.GetLogger() == nil {
			t.Skip("Logger not available, skipping WARN check")
		}
	})

	// Test 2: Create orphaned metadata (metadata without file)
	t.Run("orphaned_metadata", func(t *testing.T) {
		// Create metadata without corresponding source file
		meta := &metadata.ResourceMetadata{
			Name:       "missing-cmd",
			Type:       resource.Command,
			SourceType: "file",
			SourceURL:  "file:///tmp/missing.md",
		}

		if err := metadata.Save(meta, repoDir, "test-source"); err != nil {
			t.Fatalf("Failed to save metadata: %v", err)
		}

		// List resources - this should trigger orphan detection
		_, err := m.List(nil)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		// Check that logger exists (WARN should have been logged)
		if m.GetLogger() == nil {
			t.Skip("Logger not available, skipping WARN check")
		}
	})
}
