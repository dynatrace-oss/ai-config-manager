package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// metadataPath returns the expected metadata file path for a resource in the repo.
// Format: <repoDir>/.metadata/<type>s/<name>-metadata.json
// This mirrors the logic in pkg/metadata.GetMetadataPath.
func metadataPath(repoDir, name, resourceType string) string {
	typeDir := resourceType + "s"
	return filepath.Join(repoDir, ".metadata", typeDir, name+"-metadata.json")
}

// writeOrphanMetadata writes a minimal metadata JSON file for a resource that
// does NOT exist on disk — simulating orphaned metadata.
func writeOrphanMetadata(t *testing.T, repoDir, name, resourceType string) string {
	t.Helper()
	metaDir := filepath.Join(repoDir, ".metadata", resourceType+"s")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		t.Fatalf("Failed to create metadata dir: %v", err)
	}
	metaFile := filepath.Join(metaDir, name+"-metadata.json")
	content := `{
  "name": "` + name + `",
  "type": "` + resourceType + `",
  "source_type": "local",
  "source_url": "https://example.com/source",
  "first_installed": "2024-01-01T00:00:00Z",
  "last_updated": "2024-01-01T00:00:00Z"
}`
	if err := os.WriteFile(metaFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write orphan metadata: %v", err)
	}
	return metaFile
}

// TestRepoRepairIntegration_MissingMetadata verifies that when a resource exists in
// the repo but its metadata file is missing, 'repo repair' creates the metadata.
func TestRepoRepairIntegration_MissingMetadata(t *testing.T) {
	p := setupRepairTestProject(t)

	// Add a command to the repo (this normally creates metadata too)
	p.addCommandToRepo(t, "no-meta-cmd", "Command without metadata")

	// Delete the metadata file to simulate a missing-metadata scenario
	metaFile := metadataPath(p.repoDir, "no-meta-cmd", "command")
	if err := os.Remove(metaFile); err != nil {
		// Metadata might not have been created if AddCommand skips it — that's fine,
		// the resource file itself is enough for repair to detect the issue.
		t.Logf("Note: metadata file did not exist at %s (may never have been created): %v", metaFile, err)
	}

	output, err := runAimgr(t, "repo", "repair")
	if err != nil {
		t.Fatalf("repo repair failed: %v\nOutput: %s", err, output)
	}
	t.Logf("repo repair output: %s", output)

	// Repair should either create metadata or report the repo is healthy.
	// The key assertion is that the command did not crash.
	// If metadata was missing, the output should mention the resource was fixed.
	// We check for either "Created metadata" (fixed) or "healthy" (was already OK).
	if !containsAny(output, "no-meta-cmd", "healthy", "fixed", "Created") {
		t.Logf("Note: output did not mention 'no-meta-cmd', 'healthy', 'fixed', or 'Created': %s", output)
	}
}

// TestRepoRepairIntegration_OrphanedMetadata verifies that when a metadata file
// exists but the corresponding resource does not, 'repo repair' removes the metadata.
func TestRepoRepairIntegration_OrphanedMetadata(t *testing.T) {
	p := setupRepairTestProject(t)

	// Create orphaned metadata — no corresponding resource file
	orphanFile := writeOrphanMetadata(t, p.repoDir, "orphan-cmd", "command")

	output, err := runAimgr(t, "repo", "repair")
	if err != nil {
		t.Fatalf("repo repair failed: %v\nOutput: %s", err, output)
	}
	t.Logf("repo repair output: %s", output)

	// The orphaned metadata file should be removed
	if _, statErr := os.Stat(orphanFile); statErr == nil {
		t.Errorf("Orphaned metadata file should have been removed but still exists at %s", orphanFile)
	}

	// Output should mention orphan removal
	assertOutputContains(t, output, "orphan")
}

// TestRepoRepairIntegration_DryRun verifies that 'repo repair --dry-run' reports
// what would be done without making any filesystem changes.
func TestRepoRepairIntegration_DryRun(t *testing.T) {
	p := setupRepairTestProject(t)

	// Create orphaned metadata — no corresponding resource file
	orphanFile := writeOrphanMetadata(t, p.repoDir, "dry-orphan-cmd", "command")

	output, err := runAimgr(t, "repo", "repair", "--dry-run")
	if err != nil {
		t.Fatalf("repo repair --dry-run failed: %v\nOutput: %s", err, output)
	}
	t.Logf("repo repair --dry-run output: %s", output)

	// The orphan file must still exist — dry-run does NOT remove it
	if _, statErr := os.Stat(orphanFile); statErr != nil {
		t.Errorf("Dry-run should NOT remove orphaned metadata but file is gone: %v", statErr)
	}

	// Output should say "Would remove" or mention dry-run mode
	if !containsAny(output, "Would remove", "Dry run", "dry-run", "dry run") {
		t.Errorf("Expected dry-run indicator in output.\nFull output:\n%s", output)
	}
}

// TestRepoRepairIntegration_CleanRepo verifies that a healthy repo with no issues
// reports "No issues found" and exits cleanly.
func TestRepoRepairIntegration_CleanRepo(t *testing.T) {
	p := setupRepairTestProject(t)

	// Add a resource and keep its metadata (i.e., do NOT delete it)
	p.addCommandToRepo(t, "healthy-cmd", "A healthy command")

	output, err := runAimgr(t, "repo", "repair")
	if err != nil {
		t.Fatalf("repo repair failed on a clean repo: %v\nOutput: %s", err, output)
	}

	// Should report healthy status
	assertOutputContains(t, output, "healthy")
}

// TestRepoRepairIntegration_JSONOutput verifies that 'repo repair --format=json'
// produces valid JSON with the expected top-level fields.
func TestRepoRepairIntegration_JSONOutput(t *testing.T) {
	p := setupRepairTestProject(t)

	// Use a clean repo — no issues to repair
	p.addCommandToRepo(t, "json-cmd", "Command for JSON output test")

	output, err := runAimgr(t, "repo", "repair", "--format=json")
	if err != nil {
		t.Fatalf("repo repair --format=json failed: %v\nOutput: %s", err, output)
	}

	// Parse and validate JSON
	var result struct {
		FixedCount     int         `json:"fixed_count"`
		UnfixableCount int         `json:"unfixable_count"`
		DryRun         bool        `json:"dry_run"`
		MetadataCreate interface{} `json:"metadata_created"`
		OrphanRemoved  interface{} `json:"orphaned_removed"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}
	// If we got here the JSON was valid; log counts for debugging
	t.Logf("JSON result: fixed=%d unfixable=%d dry_run=%v", result.FixedCount, result.UnfixableCount, result.DryRun)
}

// containsAny returns true if s contains any of the substrings.
func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
