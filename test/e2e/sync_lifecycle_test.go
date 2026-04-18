//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// TestSyncRemovesDeletedResources verifies sync lifecycle semantics for deleted
// resources:
//   - plain `repo sync` keeps stale resources
//   - `repo sync --prune` removes stale resources
//
// Scenario:
//  1. Create source with cmd-a, cmd-b, and skill-a
//  2. Add source to repo
//  3. Verify all 3 resources exist
//  4. Delete cmd-b from source
//  5. Run repo sync (without --prune)
//  6. Verify cmd-a, skill-a, and stale cmd-b all still exist
//  7. Run repo sync --prune
//  8. Verify cmd-a and skill-a still exist, cmd-b is removed
//  9. Verify no metadata for cmd-b
func TestSyncRemovesDeletedResources(t *testing.T) {
	repoDir := t.TempDir()
	configPath := loadTestConfig(t, "e2e-test")
	env := map[string]string{"AIMGR_REPO_PATH": repoDir}

	// Create source directory
	sourceDir := t.TempDir()
	cmdDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	cmdA := "---\ndescription: Command A for sync lifecycle test\n---\n# cmd-a\nCommand A content."
	if err := os.WriteFile(filepath.Join(cmdDir, "sync-life-cmd-a.md"), []byte(cmdA), 0644); err != nil {
		t.Fatalf("Failed to write cmd-a: %v", err)
	}

	cmdB := "---\ndescription: Command B for sync lifecycle test\n---\n# cmd-b\nCommand B content — will be deleted."
	if err := os.WriteFile(filepath.Join(cmdDir, "sync-life-cmd-b.md"), []byte(cmdB), 0644); err != nil {
		t.Fatalf("Failed to write cmd-b: %v", err)
	}

	skillDir := filepath.Join(sourceDir, "skills", "sync-life-skill-a")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}
	skillContent := "---\ndescription: Skill A for sync lifecycle test\n---\n# sync-life-skill-a\nSkill A content."
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill: %v", err)
	}

	// Step 1: Initialize repo
	t.Log("Step 1: Initializing repo...")
	stdout, stderr, err := runAimgrWithEnv(t, configPath, env, "repo", "init")
	if err != nil {
		t.Fatalf("repo init failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	// Step 2: Add source
	t.Log("Step 2: Adding source...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "add", "local:"+sourceDir)
	if err != nil {
		t.Fatalf("repo add failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	t.Logf("Add output:\n%s", stdout)

	// Step 3: Verify all resources exist
	t.Log("Step 3: Verifying all resources imported...")
	for _, name := range []string{"sync-life-cmd-a", "sync-life-cmd-b"} {
		cmdPath := filepath.Join(repoDir, "commands", name+".md")
		if _, err := os.Lstat(cmdPath); err != nil {
			t.Errorf("Command %s not found after add: %v", name, err)
		}
	}
	skillPath := filepath.Join(repoDir, "skills", "sync-life-skill-a")
	if _, err := os.Lstat(skillPath); err != nil {
		t.Errorf("Skill sync-life-skill-a not found after add: %v", err)
	}

	// Step 4: Delete cmd-b from source
	t.Log("Step 4: Deleting cmd-b from source...")
	if err := os.Remove(filepath.Join(cmdDir, "sync-life-cmd-b.md")); err != nil {
		t.Fatalf("Failed to remove cmd-b from source: %v", err)
	}

	// Step 5: Run repo sync (no prune)
	t.Log("Step 5: Running repo sync (without prune)...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "sync")
	if err != nil {
		t.Fatalf("repo sync failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	t.Logf("Sync output:\n%s", stdout)

	// Step 6: Verify cmd-a and skill-a still exist
	t.Log("Step 6: Verifying surviving resources...")
	cmdAPath := filepath.Join(repoDir, "commands", "sync-life-cmd-a.md")
	if _, err := os.Lstat(cmdAPath); err != nil {
		t.Errorf("cmd-a should still exist after sync: %v", err)
	}
	if _, err := os.Lstat(skillPath); err != nil {
		t.Errorf("skill-a should still exist after sync: %v", err)
	}

	// Step 7: Verify cmd-b is NOT removed by default sync
	t.Log("Step 7: Verifying cmd-b preserved without prune...")
	cmdBPath := filepath.Join(repoDir, "commands", "sync-life-cmd-b.md")
	if _, err := os.Lstat(cmdBPath); err != nil {
		t.Errorf("sync-life-cmd-b should still exist after plain sync: %v", err)
	}

	// Step 8: Verify metadata for cmd-b is still present after non-pruning sync
	t.Log("Step 8: Verifying metadata preserved without prune...")
	cmdBMetaPath := metadata.GetMetadataPath("sync-life-cmd-b", resource.Command, repoDir)
	if _, err := os.Stat(cmdBMetaPath); err != nil {
		t.Errorf("Metadata for sync-life-cmd-b should exist after plain sync: %v", err)
	}

	// Step 9: Run repo sync --prune
	t.Log("Step 9: Running repo sync with prune...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "sync", "--prune")
	if err != nil {
		t.Fatalf("repo sync --prune failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	t.Logf("Prune sync output:\n%s", stdout)

	// Step 10: Verify cmd-b is removed by prune sync
	t.Log("Step 10: Verifying cmd-b removed by prune sync...")
	if _, err := os.Lstat(cmdBPath); err == nil {
		t.Error("sync-life-cmd-b should have been removed by repo sync --prune")
	}

	// Step 11: Verify metadata for cmd-b is cleaned up by prune sync
	t.Log("Step 11: Verifying metadata cleaned up by prune sync...")
	if _, err := os.Stat(cmdBMetaPath); !os.IsNotExist(err) {
		t.Errorf("Metadata for sync-life-cmd-b should not exist after prune removal, got: %v", err)
	}

	// Step 12: Verify prune sync output mentions removal
	t.Log("Step 12: Verifying prune sync output mentions removal...")
	combinedOutput := stdout + stderr
	if !strings.Contains(combinedOutput, "sync-life-cmd-b") {
		t.Log("Note: sync output did not mention sync-life-cmd-b removal specifically")
	}

	t.Log("PASS: Sync default and --prune deletion semantics are correct")
}

// TestSyncDoesNotRemoveOnFailedSource verifies failed-source behavior with
// both non-pruning and pruning sync modes:
//   - plain `repo sync` keeps stale resources
//   - `repo sync --prune` removes stale resources only from successfully
//     synced sources and preserves resources from failed sources
//
// Scenario:
//  1. Create two local sources with unique resources
//  2. Add both sources
//  3. Delete a resource from source-a
//  4. Make source-b's path invalid
//  5. Run repo sync (no prune)
//  6. Verify deleted resource from source-a is KEPT (non-pruning default)
//  7. Run repo sync --prune
//  8. Verify deleted resource from source-a IS removed (source synced OK)
//  9. Verify ALL resources from source-b are KEPT (source failed)
func TestSyncDoesNotRemoveOnFailedSource(t *testing.T) {
	repoDir := t.TempDir()
	configPath := loadTestConfig(t, "e2e-test")
	env := map[string]string{"AIMGR_REPO_PATH": repoDir}

	// Create source A with 2 commands
	sourceA := t.TempDir()
	cmdDirA := filepath.Join(sourceA, "commands")
	if err := os.MkdirAll(cmdDirA, 0755); err != nil {
		t.Fatalf("Failed to create source A commands dir: %v", err)
	}
	cmdA1 := "---\ndescription: Source A command 1\n---\n# sync-fail-a1\nWill be kept."
	if err := os.WriteFile(filepath.Join(cmdDirA, "sync-fail-a1.md"), []byte(cmdA1), 0644); err != nil {
		t.Fatalf("Failed to write cmd-a1: %v", err)
	}
	cmdA2 := "---\ndescription: Source A command 2\n---\n# sync-fail-a2\nWill be deleted from source."
	if err := os.WriteFile(filepath.Join(cmdDirA, "sync-fail-a2.md"), []byte(cmdA2), 0644); err != nil {
		t.Fatalf("Failed to write cmd-a2: %v", err)
	}

	// Create source B with 1 command
	sourceB := t.TempDir()
	cmdDirB := filepath.Join(sourceB, "commands")
	if err := os.MkdirAll(cmdDirB, 0755); err != nil {
		t.Fatalf("Failed to create source B commands dir: %v", err)
	}
	cmdB1 := "---\ndescription: Source B command\n---\n# sync-fail-b1\nSource will fail to sync."
	if err := os.WriteFile(filepath.Join(cmdDirB, "sync-fail-b1.md"), []byte(cmdB1), 0644); err != nil {
		t.Fatalf("Failed to write cmd-b1: %v", err)
	}

	// Step 1: Initialize repo
	stdout, stderr, err := runAimgrWithEnv(t, configPath, env, "repo", "init")
	if err != nil {
		t.Fatalf("repo init failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	// Step 2: Add source A
	t.Log("Adding source A...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "add", "local:"+sourceA, "--name", "sync-source-a")
	if err != nil {
		t.Fatalf("repo add source A failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	// Step 3: Add source B
	t.Log("Adding source B...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "add", "local:"+sourceB, "--name", "sync-source-b")
	if err != nil {
		t.Fatalf("repo add source B failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	// Verify all 3 commands exist
	for _, name := range []string{"sync-fail-a1", "sync-fail-a2", "sync-fail-b1"} {
		cmdPath := filepath.Join(repoDir, "commands", name+".md")
		if _, err := os.Lstat(cmdPath); err != nil {
			t.Errorf("Command %s not found after add: %v", name, err)
		}
	}

	// Step 4: Delete cmd-a2 from source A
	t.Log("Deleting sync-fail-a2 from source A...")
	if err := os.Remove(filepath.Join(cmdDirA, "sync-fail-a2.md")); err != nil {
		t.Fatalf("Failed to remove cmd-a2: %v", err)
	}

	// Step 5: Make source B's path invalid by removing the directory contents
	// We remove source B entirely to simulate a failed source
	t.Log("Removing source B directory to simulate failure...")
	if err := os.RemoveAll(sourceB); err != nil {
		t.Fatalf("Failed to remove source B: %v", err)
	}

	// Step 6: Run repo sync (no prune) — source A should succeed, source B should fail
	t.Log("Running repo sync without prune (source A OK, source B will fail)...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "sync")
	// Should not error — partial success is OK
	t.Logf("Sync stdout:\n%s", stdout)
	t.Logf("Sync stderr:\n%s", stderr)
	if err != nil {
		t.Logf("Sync returned error (expected for partial failure): %v", err)
	}

	// Step 7: cmd-a1 should still exist (not removed from source A)
	t.Log("Verifying source A resources...")
	cmdA1Path := filepath.Join(repoDir, "commands", "sync-fail-a1.md")
	if _, err := os.Lstat(cmdA1Path); err != nil {
		t.Errorf("sync-fail-a1 should still exist (not removed from source A): %v", err)
	}

	// cmd-a2 should still exist (deleted from source A but sync is non-pruning)
	cmdA2Path := filepath.Join(repoDir, "commands", "sync-fail-a2.md")
	if _, err := os.Lstat(cmdA2Path); err != nil {
		t.Errorf("sync-fail-a2 should still exist after plain sync: %v", err)
	}

	// Step 8: Run repo sync --prune
	t.Log("Running repo sync with prune (source A OK, source B will fail)...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "sync", "--prune")
	t.Logf("Prune sync stdout:\n%s", stdout)
	t.Logf("Prune sync stderr:\n%s", stderr)
	if err != nil {
		t.Logf("Prune sync returned error (expected for partial failure): %v", err)
	}

	// cmd-a2 should now be removed (deleted from source A, source A synced OK)
	if _, err := os.Lstat(cmdA2Path); err == nil {
		t.Error("sync-fail-a2 should have been removed by repo sync --prune")
	}

	// Step 9: cmd-b1 should still exist (source B failed to sync)
	t.Log("Verifying failed source resources preserved after prune...")
	cmdB1Path := filepath.Join(repoDir, "commands", "sync-fail-b1.md")
	if _, err := os.Lstat(cmdB1Path); err != nil {
		t.Errorf("sync-fail-b1 should still exist (source B failed to sync, resources must be preserved): %v", err)
	}

	t.Log("PASS: Failed source resources preserved; prune only affects successful sources")
}

// TestSyncAddUpdateRemoveCycle verifies the full lifecycle across sync modes:
// add new resources, update existing ones, and remove deleted ones only with
// explicit prune.
//
// Scenario:
//  1. Create source with cmd-a and cmd-b
//  2. Add source to repo
//  3. Modify cmd-a content, add cmd-c, delete cmd-b in source
//  4. Run repo sync (no prune)
//  5. Verify: cmd-a updated, cmd-c added, cmd-b preserved
//  6. Run repo sync --prune
//  7. Verify: cmd-b removed
func TestSyncAddUpdateRemoveCycle(t *testing.T) {
	repoDir := t.TempDir()
	configPath := loadTestConfig(t, "e2e-test")
	env := map[string]string{"AIMGR_REPO_PATH": repoDir}

	// Create source
	sourceDir := t.TempDir()
	cmdDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	cmdAOriginal := "---\ndescription: Original version of cmd-a\n---\n# sync-cycle-cmd-a\nOriginal content."
	if err := os.WriteFile(filepath.Join(cmdDir, "sync-cycle-cmd-a.md"), []byte(cmdAOriginal), 0644); err != nil {
		t.Fatalf("Failed to write cmd-a: %v", err)
	}

	cmdB := "---\ndescription: Command B — will be deleted\n---\n# sync-cycle-cmd-b\nWill be deleted."
	if err := os.WriteFile(filepath.Join(cmdDir, "sync-cycle-cmd-b.md"), []byte(cmdB), 0644); err != nil {
		t.Fatalf("Failed to write cmd-b: %v", err)
	}

	// Step 1: Initialize repo and add source
	stdout, stderr, err := runAimgrWithEnv(t, configPath, env, "repo", "init")
	if err != nil {
		t.Fatalf("repo init failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "add", "local:"+sourceDir)
	if err != nil {
		t.Fatalf("repo add failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	// Verify initial state: 2 commands
	for _, name := range []string{"sync-cycle-cmd-a", "sync-cycle-cmd-b"} {
		cmdPath := filepath.Join(repoDir, "commands", name+".md")
		if _, err := os.Lstat(cmdPath); err != nil {
			t.Errorf("Command %s not found after initial add: %v", name, err)
		}
	}

	// Step 2: Modify source — update cmd-a, add cmd-c, delete cmd-b
	t.Log("Modifying source: update cmd-a, add cmd-c, delete cmd-b...")

	// Update cmd-a content
	cmdAUpdated := "---\ndescription: Updated version of cmd-a\n---\n# sync-cycle-cmd-a\nUpdated content after modification."
	if err := os.WriteFile(filepath.Join(cmdDir, "sync-cycle-cmd-a.md"), []byte(cmdAUpdated), 0644); err != nil {
		t.Fatalf("Failed to update cmd-a: %v", err)
	}

	// Add cmd-c
	cmdC := "---\ndescription: New command C\n---\n# sync-cycle-cmd-c\nBrand new command added between syncs."
	if err := os.WriteFile(filepath.Join(cmdDir, "sync-cycle-cmd-c.md"), []byte(cmdC), 0644); err != nil {
		t.Fatalf("Failed to write cmd-c: %v", err)
	}

	// Delete cmd-b
	if err := os.Remove(filepath.Join(cmdDir, "sync-cycle-cmd-b.md")); err != nil {
		t.Fatalf("Failed to delete cmd-b: %v", err)
	}

	// Step 3: Run repo sync (no prune)
	t.Log("Running repo sync without prune...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "sync")
	if err != nil {
		t.Fatalf("repo sync failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	t.Logf("Sync output:\n%s", stdout)

	// Step 4: Verify results

	// cmd-a should exist (and be the updated version — symlink points to source)
	t.Log("Verifying cmd-a exists (updated)...")
	cmdAPath := filepath.Join(repoDir, "commands", "sync-cycle-cmd-a.md")
	if _, err := os.Lstat(cmdAPath); err != nil {
		t.Errorf("sync-cycle-cmd-a should still exist after sync: %v", err)
	}

	// cmd-c should be added
	t.Log("Verifying cmd-c added...")
	cmdCPath := filepath.Join(repoDir, "commands", "sync-cycle-cmd-c.md")
	if _, err := os.Lstat(cmdCPath); err != nil {
		t.Errorf("sync-cycle-cmd-c should be added by sync: %v", err)
	}

	// cmd-b should still exist after non-pruning sync
	t.Log("Verifying cmd-b preserved without prune...")
	cmdBPath := filepath.Join(repoDir, "commands", "sync-cycle-cmd-b.md")
	if _, err := os.Lstat(cmdBPath); err != nil {
		t.Errorf("sync-cycle-cmd-b should still exist after plain sync: %v", err)
	}

	// Verify metadata for cmd-c exists
	cmdCMetaPath := metadata.GetMetadataPath("sync-cycle-cmd-c", resource.Command, repoDir)
	if _, err := os.Stat(cmdCMetaPath); err != nil {
		t.Errorf("Metadata for sync-cycle-cmd-c should exist after sync: %v", err)
	}

	// Verify metadata for cmd-b still exists after non-pruning sync
	cmdBMetaPath := metadata.GetMetadataPath("sync-cycle-cmd-b", resource.Command, repoDir)
	if _, err := os.Stat(cmdBMetaPath); err != nil {
		t.Errorf("Metadata for sync-cycle-cmd-b should exist after plain sync: %v", err)
	}

	// Step 5: Run repo sync --prune and verify stale cmd-b is removed
	t.Log("Running repo sync with prune...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "sync", "--prune")
	if err != nil {
		t.Fatalf("repo sync --prune failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	t.Logf("Prune sync output:\n%s", stdout)

	if _, err := os.Lstat(cmdBPath); err == nil {
		t.Error("sync-cycle-cmd-b should have been removed by repo sync --prune")
	}

	if _, err := os.Stat(cmdBMetaPath); !os.IsNotExist(err) {
		t.Errorf("Metadata for sync-cycle-cmd-b should not exist after prune removal: %v", err)
	}

	t.Log("PASS: Full add/update/remove lifecycle matches explicit prune semantics")
}
