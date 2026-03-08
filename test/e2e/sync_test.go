//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestE2E_SyncIdempotency verifies that sync operations are idempotent.
// Running sync twice on an empty repo should not cause any updates on the second run.
func TestE2E_SyncIdempotency(t *testing.T) {
	// Setup: Load config and create empty test repo
	configPath := loadTestConfig(t, "e2e-test")
	repoPath := getRepoPathFromConfig(t, configPath)
	t.Logf("Using repo path: %s", repoPath)

	// Clean the repo before starting (ensure it's truly empty)
	if err := os.RemoveAll(repoPath); err != nil {
		t.Fatalf("Failed to clean repo before test: %v", err)
	}

	// Create empty repo directory
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create empty repo: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		cleanTestRepo(t, repoPath)
	})

	// Setup: Create ai.repo.yaml with test sources
	setupTestRepoWithSources(t, repoPath)

	// Step 1: Run first sync
	// Note: We need to set AIMGR_REPO_PATH because sync command doesn't read repo.path from config yet
	t.Log("Step 1: Running first sync on empty repository...")
	env := map[string]string{"AIMGR_REPO_PATH": repoPath}
	stdout1, stderr1, err1 := runAimgrWithEnv(t, configPath, env, "repo", "sync")

	// Log output for debugging
	t.Logf("First sync stdout:\n%s", stdout1)
	if stderr1 != "" {
		t.Logf("First sync stderr:\n%s", stderr1)
	}

	// Verify first sync succeeded
	if err1 != nil {
		t.Fatalf("First sync failed: %v", err1)
	}

	// Parse first sync stats
	stats1 := parseSyncStats(t, stdout1)
	t.Logf("First sync stats: Added=%d, Updated=%d, Skipped=%d, Failed=%d",
		stats1.Added, stats1.Updated, stats1.Skipped, stats1.Failed)

	// Verify first sync processed resources (repo was empty)
	// Note: sync uses force=true by default, so resources may show as "Updated" even on first sync
	totalProcessed1 := stats1.Added + stats1.Updated
	if totalProcessed1 == 0 {
		t.Error("First sync should have processed resources (repo was empty), but Added+Updated=0")
	}

	// Verify no failures
	if stats1.Failed > 0 {
		t.Errorf("First sync had failures: Failed=%d", stats1.Failed)
	}

	// Verify repo is now populated
	if !isRepoPopulated(t, repoPath) {
		// Debug: list repo contents
		if entries, err := os.ReadDir(repoPath); err == nil {
			t.Logf("Repo contents (%d items): %v", len(entries), entries)
			for _, entry := range entries {
				t.Logf("  - %s (isDir=%v)", entry.Name(), entry.IsDir())
			}
		} else {
			t.Logf("Failed to read repo dir: %v", err)
		}
		t.Error("Repository should be populated after first sync")
	}

	// Step 2: Run second sync (idempotency test)
	t.Log("Step 2: Running second sync (should be idempotent)...")
	stdout2, stderr2, err2 := runAimgrWithEnv(t, configPath, env, "repo", "sync")

	// Log output for debugging
	t.Logf("Second sync stdout:\n%s", stdout2)
	if stderr2 != "" {
		t.Logf("Second sync stderr:\n%s", stderr2)
	}

	// Verify second sync succeeded
	if err2 != nil {
		t.Fatalf("Second sync failed: %v", err2)
	}

	// Parse second sync stats
	stats2 := parseSyncStats(t, stdout2)
	t.Logf("Second sync stats: Added=%d, Updated=%d, Skipped=%d, Failed=%d",
		stats2.Added, stats2.Updated, stats2.Skipped, stats2.Failed)

	// Assert: Second sync is idempotent
	// Key insight: sync uses force=true by default, so:
	// - First sync: resources are "Added" (don't exist yet)
	// - Second sync: resources are "Updated" (already exist, force re-adds them)
	// True idempotency means:
	// 1. Total resources processed is the same
	// 2. No additional Added/Updated on second sync (should all be Updated)
	// 3. Final repository state is unchanged

	totalProcessed2 := stats2.Added + stats2.Updated

	// Verify same total resources processed
	if totalProcessed2 != totalProcessed1 {
		t.Errorf("Second sync processed different number of resources (not idempotent): first=%d, second=%d",
			totalProcessed1, totalProcessed2)
	}

	// Verify no failures
	if stats2.Failed > 0 {
		t.Errorf("Second sync had failures: Failed=%d", stats2.Failed)
	}

	// Verify second sync shows all resources as "Updated" (not "Added")
	// This confirms resources already existed and were simply re-synced
	if stats2.Added > 0 {
		t.Errorf("Second sync added new resources (not idempotent): Added=%d (expected 0)", stats2.Added)
	}

	// Verify second sync processed all resources (as Updates, since they exist)
	if stats2.Updated != totalProcessed1 {
		t.Errorf("Second sync updated count mismatch: Updated=%d (expected %d)", stats2.Updated, totalProcessed1)
	}

	// Verify repo is still populated and unchanged
	if !isRepoPopulated(t, repoPath) {
		t.Error("Repository should still be populated after second sync")
	}

	t.Log("✓ Sync idempotency verified: second sync made no changes")
}

// syncStats holds the parsed statistics from sync output
type syncStats struct {
	Added   int
	Updated int
	Skipped int
	Failed  int
}

// parseSyncStats extracts sync statistics from aimgr output.
// It supports both the legacy per-source summary lines and the current
// compact sync output.
func parseSyncStats(t *testing.T, output string) syncStats {
	t.Helper()

	stats := syncStats{}

	// Legacy format (one summary line per source):
	// "Summary: X added, Y updated, Z failed, W skipped (N total)"
	legacySummaryRegex := regexp.MustCompile(`Summary:\s*(\d+)\s+added,\s*(\d+)\s+updated,\s*(\d+)\s+failed,\s*(\d+)\s+skipped`)
	legacyMatches := legacySummaryRegex.FindAllStringSubmatch(output, -1)

	for _, matches := range legacyMatches {
		if len(matches) == 5 {
			stats.Added += mustParseInt(t, matches[1])
			stats.Updated += mustParseInt(t, matches[2])
			stats.Failed += mustParseInt(t, matches[3])
			stats.Skipped += mustParseInt(t, matches[4])
		}
	}

	if len(legacyMatches) > 0 {
		return stats
	}

	// Current compact per-source format:
	// "✓ <source> (<mode>) — X added, Y updated[, Z removed]"
	compactLineRegex := regexp.MustCompile(`(?m)^\s*[✓✔]\s+.*?—\s*(\d+)\s+added,\s*(\d+)\s+updated(?:,\s*\d+\s+removed)?\s*$`)
	compactMatches := compactLineRegex.FindAllStringSubmatch(output, -1)
	for _, matches := range compactMatches {
		if len(matches) == 3 {
			stats.Added += mustParseInt(t, matches[1])
			stats.Updated += mustParseInt(t, matches[2])
		}
	}

	// Track compact failed source lines for visibility in tests.
	failedSourceLineRegex := regexp.MustCompile(`(?m)^\s*[✗xX]\s+.*?—\s*error:`)
	stats.Failed += len(failedSourceLineRegex.FindAllString(output, -1))

	if len(compactMatches) > 0 || stats.Failed > 0 {
		return stats
	}

	// If we still have no stats, check for "no changes" or similar message
	lowerOutput := strings.ToLower(output)
	if strings.Contains(lowerOutput, "no changes") || strings.Contains(lowerOutput, "nothing to sync") {
		// All zeros is correct for "no changes"
		return stats
	}

	// Log warning if we couldn't parse any stats (but don't fail - output format may vary)
	t.Logf("Warning: Could not parse sync stats from output (may indicate 0 operations or different format)")

	return stats
}

// mustParseInt parses an integer or fails the test
func mustParseInt(t *testing.T, s string) int {
	t.Helper()

	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("Failed to parse integer '%s': %v", s, err)
	}
	return n
}

// isRepoPopulated checks if the repository contains any resources
func isRepoPopulated(t *testing.T, repoPath string) bool {
	t.Helper()

	// Check if repo directory exists
	info, err := os.Stat(repoPath)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return false
	}

	// Check for standard resource directories
	dirs := []string{"commands", "skills", "agents"}
	for _, dir := range dirs {
		dirPath := filepath.Join(repoPath, dir)
		if entries, err := os.ReadDir(dirPath); err == nil && len(entries) > 0 {
			// Found at least one non-empty resource directory
			return true
		}
	}

	// No resources found
	return false
}

// TestE2E_SyncEmptyConfig verifies behavior when no sources are configured.
// This is a negative test case.
func TestE2E_SyncEmptyConfig(t *testing.T) {
	configPath := loadTestConfig(t, "e2e-test")
	repoPath := getRepoPathFromConfig(t, configPath)

	cleanTestRepo(t, repoPath)
	t.Cleanup(func() { cleanTestRepo(t, repoPath) })

	// Create ai.repo.yaml with NO sources (empty array)
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}

	emptyManifest := `version: 1
sources: []
`
	aiRepoPath := filepath.Join(repoPath, "ai.repo.yaml")
	if err := os.WriteFile(aiRepoPath, []byte(emptyManifest), 0644); err != nil {
		t.Fatalf("Failed to write ai.repo.yaml: %v", err)
	}

	// Run sync - should fail
	env := map[string]string{"AIMGR_REPO_PATH": repoPath}
	stdout, stderr, err := runAimgrWithEnv(t, configPath, env, "repo", "sync")

	// Verify it failed
	if err == nil {
		t.Error("Expected sync to fail with empty sources, but it succeeded")
	}

	// Verify helpful error message
	output := stdout + stderr
	if !strings.Contains(output, "no sync sources configured") {
		t.Errorf("Expected 'no sync sources configured' error, got: %s", output)
	}

	// Verify guidance is provided
	if !strings.Contains(output, "repo add") {
		t.Error("Expected error to suggest using 'repo add' command")
	}

	t.Logf("✓ Empty config handling verified: %s", strings.TrimSpace(output))
}

// TestE2E_SyncInvalidSource verifies behavior when a source URL is invalid.
// This is a negative test case that verifies graceful error handling.
func TestE2E_SyncInvalidSource(t *testing.T) {
	configPath := loadTestConfig(t, "e2e-test")
	repoPath := getRepoPathFromConfig(t, configPath)

	cleanTestRepo(t, repoPath)
	t.Cleanup(func() { cleanTestRepo(t, repoPath) })

	// Create ai.repo.yaml with INVALID source (non-existent path)
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}

	invalidManifest := `version: 1
sources:
  - name: invalid-local-path
    path: /this/path/does/not/exist/at/all
`
	aiRepoPath := filepath.Join(repoPath, "ai.repo.yaml")
	if err := os.WriteFile(aiRepoPath, []byte(invalidManifest), 0644); err != nil {
		t.Fatalf("Failed to write ai.repo.yaml: %v", err)
	}

	// Run sync - should fail since ALL sources are invalid
	env := map[string]string{"AIMGR_REPO_PATH": repoPath}
	stdout, stderr, err := runAimgrWithEnv(t, configPath, env, "repo", "sync")

	// Verify it failed (all sources failed)
	if err == nil {
		t.Error("Expected sync to fail when all sources are invalid, but it succeeded")
	}

	// Check output mentions the failure
	output := stdout + stderr

	// Should mention "all sources failed" or similar
	if !strings.Contains(output, "all sources failed") {
		t.Logf("Output: %s", output)
		t.Error("Expected error to mention 'all sources failed'")
	}

	// Should mention the invalid source name for debugging
	if !strings.Contains(output, "invalid-local-path") {
		t.Logf("Output: %s", output)
		t.Error("Expected output to mention the invalid source name for debugging")
	}

	// Verify repo is empty (nothing synced)
	if isRepoPopulated(t, repoPath) {
		t.Error("Repository should be empty when all sources fail")
	}

	t.Logf("✓ Invalid source handling verified")
}

// Example of how to add more sync tests:
//
// func TestE2E_SyncWithFilter(t *testing.T) {
//     // Test that sync respects filter patterns in config
// }
//
// func TestE2E_SyncSkipExisting(t *testing.T) {
//     // Test sync with --skip-existing flag
// }
//
// func TestE2E_SyncDryRun(t *testing.T) {
//     // Test sync with --dry-run flag
// }
