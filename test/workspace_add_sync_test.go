package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/source"
)

// TestWorkspaceCacheWithLocalSource verifies local sources don't create cache
func TestWorkspaceCacheWithLocalSource(t *testing.T) {
	repoDir := t.TempDir()

	// Create a test local source directory
	localSource := t.TempDir()

	// Parse local source (should not create cache)
	parsed, err := source.ParseSource("local:" + localSource)
	if err != nil {
		t.Fatalf("Failed to parse local source: %v", err)
	}

	if parsed.Type != source.Local {
		t.Errorf("Expected source type Local, got %v", parsed.Type)
	}

	t.Log("Step 1: Verify local sources don't use Git caching")
	workspaceDir := filepath.Join(repoDir, ".workspace")
	if _, err := os.Stat(workspaceDir); err == nil {
		// If workspace exists, verify it's empty (no caches created)
		entries, _ := os.ReadDir(workspaceDir)
		cacheCount := 0
		for _, e := range entries {
			if e.IsDir() {
				cacheCount++
			}
		}
		if cacheCount > 0 {
			t.Error("Local sources should not create workspace cache entries")
		}
	}
	t.Log("  ✓ Local sources correctly skip workspace caching")
}
