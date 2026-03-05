//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// TestSourceRenamePreservesConnections verifies that renaming a source
// in the manifest does not break resource-to-source tracking.
// This is the core guarantee of Epic 1: the source_id (hash of path/URL)
// is the stable identifier, so changing the human-readable name is safe.
func TestSourceRenamePreservesConnections(t *testing.T) {
	// --- Setup: create a source directory with 2 command resources ---
	sourceDir := t.TempDir()

	commandsDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	// Command 1
	cmd1Content := `---
description: First test command for source rename
---
# rename-test-alpha
First command used to verify source rename preserves connections.
`
	if err := os.WriteFile(filepath.Join(commandsDir, "rename-test-alpha.md"), []byte(cmd1Content), 0644); err != nil {
		t.Fatalf("Failed to write command 1: %v", err)
	}

	// Command 2
	cmd2Content := `---
description: Second test command for source rename
---
# rename-test-beta
Second command used to verify source rename preserves connections.
`
	if err := os.WriteFile(filepath.Join(commandsDir, "rename-test-beta.md"), []byte(cmd2Content), 0644); err != nil {
		t.Fatalf("Failed to write command 2: %v", err)
	}

	// --- Setup: create the repo directory ---
	repoDir := t.TempDir()
	configPath := loadTestConfig(t, "e2e-test")

	// Compute expected source ID (based on the absolute path of sourceDir)
	absSourceDir, err := filepath.Abs(sourceDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}
	expectedSourceID := repomanifest.GenerateSourceID(&repomanifest.Source{Path: absSourceDir})
	t.Logf("Source path: %s", absSourceDir)
	t.Logf("Expected source_id: %s", expectedSourceID)

	// Write initial ai.repo.yaml with source name "original-name"
	manifestContent := fmt.Sprintf(`version: 1
sources:
  - id: %s
    name: original-name
    path: %s
`, expectedSourceID, sourceDir)

	if err := os.WriteFile(filepath.Join(repoDir, "ai.repo.yaml"), []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to write ai.repo.yaml: %v", err)
	}

	env := map[string]string{"AIMGR_REPO_PATH": repoDir}

	// --- Step 1: First sync with "original-name" ---
	t.Log("Step 1: Syncing with source name 'original-name'...")
	stdout, stderr, err := runAimgrWithEnv(t, configPath, env, "repo", "sync")
	if err != nil {
		t.Fatalf("First sync failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	t.Logf("First sync output:\n%s", stdout)

	// Verify both resources exist and have correct metadata
	for _, cmdName := range []string{"rename-test-alpha", "rename-test-beta"} {
		meta := loadResourceMetadata(t, cmdName, resource.Command, repoDir)

		if meta.SourceName != "original-name" {
			t.Errorf("Before rename: %s source_name = %q, want %q", cmdName, meta.SourceName, "original-name")
		}
		if meta.SourceID != expectedSourceID {
			t.Errorf("Before rename: %s source_id = %q, want %q", cmdName, meta.SourceID, expectedSourceID)
		}
		t.Logf("Before rename: %s has source_name=%q, source_id=%q", cmdName, meta.SourceName, meta.SourceID)
	}

	// --- Step 2: Rename the source in the manifest ---
	t.Log("Step 2: Renaming source from 'original-name' to 'renamed-source' in manifest...")
	renamedManifest := fmt.Sprintf(`version: 1
sources:
  - id: %s
    name: renamed-source
    path: %s
`, expectedSourceID, sourceDir)

	if err := os.WriteFile(filepath.Join(repoDir, "ai.repo.yaml"), []byte(renamedManifest), 0644); err != nil {
		t.Fatalf("Failed to write renamed ai.repo.yaml: %v", err)
	}

	// --- Step 3: Sync again after rename ---
	t.Log("Step 3: Syncing with renamed source 'renamed-source'...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "sync")
	if err != nil {
		t.Fatalf("Second sync (after rename) failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	t.Logf("Second sync output:\n%s", stdout)

	// --- Step 4: Verify resources survived the rename ---
	t.Log("Step 4: Verifying resources preserved after rename...")
	for _, cmdName := range []string{"rename-test-alpha", "rename-test-beta"} {
		// 4a. Resource still exists (not orphaned)
		cmdPath := filepath.Join(repoDir, "commands", cmdName+".md")
		if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
			t.Errorf("Resource %s was orphaned after rename — file missing: %s", cmdName, cmdPath)
			continue
		}

		// 4b. Load metadata and check fields
		meta := loadResourceMetadata(t, cmdName, resource.Command, repoDir)

		// source_id should be UNCHANGED (same path = same hash)
		if meta.SourceID != expectedSourceID {
			t.Errorf("After rename: %s source_id = %q, want %q (should be unchanged)", cmdName, meta.SourceID, expectedSourceID)
		}

		// source_name should be UPDATED to "renamed-source"
		if meta.SourceName != "renamed-source" {
			t.Errorf("After rename: %s source_name = %q, want %q", cmdName, meta.SourceName, "renamed-source")
		}

		t.Logf("After rename: %s has source_name=%q, source_id=%q", cmdName, meta.SourceName, meta.SourceID)
	}

	// --- Step 5: Verify HasSource checks pass with both name and ID ---
	t.Log("Step 5: Verifying HasSource lookups work with both name and ID...")
	for _, cmdName := range []string{"rename-test-alpha", "rename-test-beta"} {
		// HasSource by source_id should always work
		if !metadata.HasSource(cmdName, resource.Command, expectedSourceID, repoDir) {
			t.Errorf("HasSource(%q, source_id=%q) = false, want true", cmdName, expectedSourceID)
		}

		// HasSource by new source_name should work
		if !metadata.HasSource(cmdName, resource.Command, "renamed-source", repoDir) {
			t.Errorf("HasSource(%q, 'renamed-source') = false, want true", cmdName)
		}

		// HasSource by OLD source_name should NOT match anymore
		if metadata.HasSource(cmdName, resource.Command, "original-name", repoDir) {
			t.Errorf("HasSource(%q, 'original-name') = true, want false (old name should not match after rename+sync)", cmdName)
		}
	}

	t.Log("✓ Source rename preserves connections: source_id unchanged, source_name updated")
}

// TestSourcePathChangeCreatesNewID verifies that changing a source's
// path produces a different source ID (it's a different source).
func TestSourcePathChangeCreatesNewID(t *testing.T) {
	// --- Setup: create two separate source directories ---
	pathA := t.TempDir()
	pathB := t.TempDir()

	// Create a command in path A
	commandsDirA := filepath.Join(pathA, "commands")
	if err := os.MkdirAll(commandsDirA, 0755); err != nil {
		t.Fatalf("Failed to create commands dir A: %v", err)
	}
	cmdContent := `---
description: Test command for path change
---
# path-change-test
Command used to verify path change creates a new source ID.
`
	if err := os.WriteFile(filepath.Join(commandsDirA, "path-change-test.md"), []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to write command in path A: %v", err)
	}

	// Create the same command in path B (so sync works after path change)
	commandsDirB := filepath.Join(pathB, "commands")
	if err := os.MkdirAll(commandsDirB, 0755); err != nil {
		t.Fatalf("Failed to create commands dir B: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandsDirB, "path-change-test.md"), []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to write command in path B: %v", err)
	}

	// --- Setup: create repo ---
	repoDir := t.TempDir()
	configPath := loadTestConfig(t, "e2e-test")

	// Compute source IDs for both paths
	absPathA, _ := filepath.Abs(pathA)
	absPathB, _ := filepath.Abs(pathB)
	sourceIDA := repomanifest.GenerateSourceID(&repomanifest.Source{Path: absPathA})
	sourceIDB := repomanifest.GenerateSourceID(&repomanifest.Source{Path: absPathB})

	t.Logf("Path A: %s → source_id: %s", absPathA, sourceIDA)
	t.Logf("Path B: %s → source_id: %s", absPathB, sourceIDB)

	// Sanity check: IDs must differ
	if sourceIDA == sourceIDB {
		t.Fatalf("Source IDs for different paths should differ, but both are %q", sourceIDA)
	}

	// --- Step 1: Add source at path A with name "tools" ---
	manifestA := fmt.Sprintf(`version: 1
sources:
  - id: %s
    name: tools
    path: %s
`, sourceIDA, pathA)

	if err := os.WriteFile(filepath.Join(repoDir, "ai.repo.yaml"), []byte(manifestA), 0644); err != nil {
		t.Fatalf("Failed to write initial ai.repo.yaml: %v", err)
	}

	env := map[string]string{"AIMGR_REPO_PATH": repoDir}

	t.Log("Step 1: Syncing with source at path A...")
	stdout, stderr, err := runAimgrWithEnv(t, configPath, env, "repo", "sync")
	if err != nil {
		t.Fatalf("Sync with path A failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	// Record source_id from metadata
	metaA := loadResourceMetadata(t, "path-change-test", resource.Command, repoDir)
	t.Logf("After path A sync: source_id=%q, source_name=%q", metaA.SourceID, metaA.SourceName)

	if metaA.SourceID != sourceIDA {
		t.Errorf("After path A sync: source_id = %q, want %q", metaA.SourceID, sourceIDA)
	}

	// --- Step 2: Change path in manifest to path B ---
	t.Log("Step 2: Changing source path from A to B in manifest...")
	manifestB := fmt.Sprintf(`version: 1
sources:
  - id: %s
    name: tools
    path: %s
`, sourceIDB, pathB)

	if err := os.WriteFile(filepath.Join(repoDir, "ai.repo.yaml"), []byte(manifestB), 0644); err != nil {
		t.Fatalf("Failed to write updated ai.repo.yaml: %v", err)
	}

	// --- Step 3: Sync again ---
	t.Log("Step 3: Syncing with source at path B...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "sync")
	if err != nil {
		t.Fatalf("Sync with path B failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	// --- Step 4: Verify source_id changed ---
	metaB := loadResourceMetadata(t, "path-change-test", resource.Command, repoDir)
	t.Logf("After path B sync: source_id=%q, source_name=%q", metaB.SourceID, metaB.SourceName)

	if metaB.SourceID == sourceIDA {
		t.Errorf("After path change: source_id should have changed from %q, but it didn't", sourceIDA)
	}

	if metaB.SourceID != sourceIDB {
		t.Errorf("After path change: source_id = %q, want %q (new path = new hash)", metaB.SourceID, sourceIDB)
	}

	// Source name should still be "tools" (we didn't rename it)
	if metaB.SourceName != "tools" {
		t.Errorf("After path change: source_name = %q, want %q", metaB.SourceName, "tools")
	}

	t.Log("✓ Source path change creates new source ID: different path = different hash")
}

// loadResourceMetadata loads and returns the metadata for a resource, failing the test if not found.
func loadResourceMetadata(t *testing.T, name string, resType resource.ResourceType, repoDir string) *metadata.ResourceMetadata {
	t.Helper()

	metaPath := metadata.GetMetadataPath(name, resType, repoDir)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read metadata for %s/%s at %s: %v", resType, name, metaPath, err)
	}

	var meta metadata.ResourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("Failed to parse metadata for %s/%s: %v", resType, name, err)
	}

	return &meta
}
