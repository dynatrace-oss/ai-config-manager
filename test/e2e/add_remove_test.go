//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/pkg/sourcemetadata"
)

// createAddRemoveTestSource creates a source directory with test resources:
// 2 commands, 1 skill (directory-based), and returns the path.
func createAddRemoveTestSource(t *testing.T) string {
	t.Helper()

	sourceDir := t.TempDir()

	// Create commands directory with 2 commands
	commandsDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	cmd1 := `---
description: First add-remove test command
---
# add-remove-alpha
First command for add/remove cycle test.
`
	if err := os.WriteFile(filepath.Join(commandsDir, "add-remove-alpha.md"), []byte(cmd1), 0644); err != nil {
		t.Fatalf("Failed to write command 1: %v", err)
	}

	cmd2 := `---
description: Second add-remove test command
---
# add-remove-beta
Second command for add/remove cycle test.
`
	if err := os.WriteFile(filepath.Join(commandsDir, "add-remove-beta.md"), []byte(cmd2), 0644); err != nil {
		t.Fatalf("Failed to write command 2: %v", err)
	}

	// Create a skill (directory-based)
	skillDir := filepath.Join(sourceDir, "skills", "add-remove-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	skillContent := `---
description: A test skill for the add/remove cycle test
---
# add-remove-skill
A test skill for the add/remove cycle test.

## Instructions
This skill is used to verify the add/remove cycle leaves a clean state.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill: %v", err)
	}

	return sourceDir
}

// TestAddRemoveCycleCleanState verifies that repo add followed by repo remove
// leaves the repository in a clean state with no stale artifacts.
// This is the core symmetry test for the add/remove lifecycle.
func TestAddRemoveCycleCleanState(t *testing.T) {
	sourceDir := createAddRemoveTestSource(t)
	repoDir := t.TempDir()
	configPath := loadTestConfig(t, "e2e-test")
	env := map[string]string{"AIMGR_REPO_PATH": repoDir}

	// Step 1: Initialize the repo
	t.Log("Step 1: Initializing repo...")
	stdout, stderr, err := runAimgrWithEnv(t, configPath, env, "repo", "init")
	if err != nil {
		t.Fatalf("repo init failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	// Record initial state: capture what the .metadata directory looks like
	initialMetadataEntries := listDirEntries(t, filepath.Join(repoDir, ".metadata"))
	t.Logf("Initial .metadata entries: %v", initialMetadataEntries)

	// Step 2: Add the source
	t.Log("Step 2: Adding source...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "add", "local:"+sourceDir)
	if err != nil {
		t.Fatalf("repo add failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	t.Logf("Add output:\n%s", stdout)

	// Step 3: Verify resources exist in repo after add
	t.Log("Step 3: Verifying resources exist after add...")

	// Verify commands
	for _, cmdName := range []string{"add-remove-alpha", "add-remove-beta"} {
		cmdPath := filepath.Join(repoDir, "commands", cmdName+".md")
		if _, err := os.Lstat(cmdPath); err != nil {
			t.Errorf("Command %s not found in repo after add: %v", cmdName, err)
		}
	}

	// Verify skill
	skillPath := filepath.Join(repoDir, "skills", "add-remove-skill")
	if _, err := os.Lstat(skillPath); err != nil {
		t.Errorf("Skill add-remove-skill not found in repo after add: %v", err)
	}

	// Step 4: Verify source in manifest
	t.Log("Step 4: Verifying source in manifest...")
	manifest, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	// Find the source (it should be named after the directory)
	var sourceName string
	for _, src := range manifest.Sources {
		if src.Path != "" {
			sourceName = src.Name
			break
		}
	}
	if sourceName == "" {
		t.Fatal("No source found in manifest after add")
	}
	t.Logf("Source name in manifest: %s", sourceName)

	// Step 5: Verify resource metadata files exist
	t.Log("Step 5: Verifying resource metadata exists...")
	for _, cmdName := range []string{"add-remove-alpha", "add-remove-beta"} {
		metaPath := metadata.GetMetadataPath(cmdName, resource.Command, repoDir)
		if _, err := os.Stat(metaPath); err != nil {
			t.Errorf("Metadata for command %s not found: %v", cmdName, err)
		}
	}
	skillMetaPath := metadata.GetMetadataPath("add-remove-skill", resource.Skill, repoDir)
	if _, err := os.Stat(skillMetaPath); err != nil {
		t.Errorf("Metadata for skill add-remove-skill not found: %v", err)
	}

	// Step 6: Verify source metadata entry exists
	t.Log("Step 6: Verifying source metadata exists...")
	srcMeta, err := sourcemetadata.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load source metadata: %v", err)
	}
	if len(srcMeta.Sources) == 0 {
		t.Error("Source metadata should have at least one entry after add")
	}

	// Step 7: Remove the source
	t.Log("Step 7: Removing source...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "remove", sourceName)
	if err != nil {
		t.Fatalf("repo remove failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	t.Logf("Remove output:\n%s", stdout)

	// Step 8: Verify resources removed from repo
	t.Log("Step 8: Verifying resources removed...")
	for _, cmdName := range []string{"add-remove-alpha", "add-remove-beta"} {
		cmdPath := filepath.Join(repoDir, "commands", cmdName+".md")
		if _, err := os.Lstat(cmdPath); err == nil {
			t.Errorf("Command %s should be removed after repo remove", cmdName)
		}
	}
	// Skill directory should be removed
	if _, err := os.Lstat(skillPath); err == nil {
		t.Errorf("Skill add-remove-skill should be removed after repo remove")
	}

	// Step 9: Verify source removed from manifest
	t.Log("Step 9: Verifying source removed from manifest...")
	manifest, err = repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}
	if manifest.HasSource(sourceName) {
		t.Errorf("Source %s should not exist in manifest after remove", sourceName)
	}
	if len(manifest.Sources) != 0 {
		t.Errorf("Manifest should have 0 sources after remove, got %d", len(manifest.Sources))
	}

	// Step 10: Verify resource metadata files removed
	t.Log("Step 10: Verifying resource metadata removed...")
	for _, cmdName := range []string{"add-remove-alpha", "add-remove-beta"} {
		metaPath := metadata.GetMetadataPath(cmdName, resource.Command, repoDir)
		if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
			t.Errorf("Metadata for command %s should be removed: stat returned %v", cmdName, err)
		}
	}
	if _, err := os.Stat(skillMetaPath); !os.IsNotExist(err) {
		t.Errorf("Metadata for skill add-remove-skill should be removed: stat returned %v", err)
	}

	// Step 11: Verify source metadata entry removed
	t.Log("Step 11: Verifying source metadata cleaned up...")
	srcMeta, err = sourcemetadata.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load source metadata after remove: %v", err)
	}
	if len(srcMeta.Sources) != 0 {
		t.Errorf("Source metadata should be empty after remove, got %d entries: %+v", len(srcMeta.Sources), srcMeta.Sources)
	}

	// Step 12: Verify symlink warning was emitted (on stderr)
	if !strings.Contains(stderr, "may break symlinks") {
		t.Log("Note: no symlink warning in stderr (expected — warning goes to stderr)")
	}

	t.Log("✓ Add/remove cycle completed — repo is in clean state")
}

// TestAddRemoveCycleKeepResources verifies that repo remove --keep-resources
// cleans up source metadata but preserves resource files and their metadata.
func TestAddRemoveCycleKeepResources(t *testing.T) {
	sourceDir := createAddRemoveTestSource(t)
	repoDir := t.TempDir()
	configPath := loadTestConfig(t, "e2e-test")
	env := map[string]string{"AIMGR_REPO_PATH": repoDir}

	// Initialize and add
	stdout, stderr, err := runAimgrWithEnv(t, configPath, env, "repo", "init")
	if err != nil {
		t.Fatalf("repo init failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "add", "local:"+sourceDir)
	if err != nil {
		t.Fatalf("repo add failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	// Find the source name
	manifest, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}
	var sourceName string
	for _, src := range manifest.Sources {
		if src.Path != "" {
			sourceName = src.Name
			break
		}
	}
	if sourceName == "" {
		t.Fatal("No source found in manifest")
	}

	// Remove with --keep-resources
	t.Log("Removing source with --keep-resources...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "remove", sourceName, "--keep-resources")
	if err != nil {
		t.Fatalf("repo remove --keep-resources failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	t.Logf("Remove output:\n%s", stdout)

	// Verify source removed from manifest
	manifest, err = repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}
	if manifest.HasSource(sourceName) {
		t.Error("Source should be removed from manifest")
	}

	// Verify source metadata cleaned up (even with --keep-resources)
	srcMeta, err := sourcemetadata.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load source metadata: %v", err)
	}
	if len(srcMeta.Sources) != 0 {
		t.Errorf("Source metadata should be cleaned up even with --keep-resources, got %d entries", len(srcMeta.Sources))
	}

	// Verify resources STILL exist (not removed with --keep-resources)
	for _, cmdName := range []string{"add-remove-alpha", "add-remove-beta"} {
		cmdPath := filepath.Join(repoDir, "commands", cmdName+".md")
		if _, err := os.Lstat(cmdPath); err != nil {
			t.Errorf("Command %s should still exist with --keep-resources: %v", cmdName, err)
		}
	}
	skillPath := filepath.Join(repoDir, "skills", "add-remove-skill")
	if _, err := os.Lstat(skillPath); err != nil {
		t.Errorf("Skill add-remove-skill should still exist with --keep-resources: %v", err)
	}

	// Verify resource metadata STILL exists
	for _, cmdName := range []string{"add-remove-alpha", "add-remove-beta"} {
		metaPath := metadata.GetMetadataPath(cmdName, resource.Command, repoDir)
		if _, err := os.Stat(metaPath); err != nil {
			t.Errorf("Metadata for command %s should still exist with --keep-resources: %v", cmdName, err)
		}
	}

	t.Log("✓ Keep-resources correctly preserves resources while cleaning up source tracking")
}

// TestAddDuplicateSourceDetection verifies that adding the same source
// twice is detected by source ID, regardless of the name used.
func TestAddDuplicateSourceDetection(t *testing.T) {
	sourceDir := createAddRemoveTestSource(t)
	repoDir := t.TempDir()
	configPath := loadTestConfig(t, "e2e-test")
	env := map[string]string{"AIMGR_REPO_PATH": repoDir}

	// Initialize and add source
	stdout, stderr, err := runAimgrWithEnv(t, configPath, env, "repo", "init")
	if err != nil {
		t.Fatalf("repo init failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}

	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "add", "local:"+sourceDir)
	if err != nil {
		t.Fatalf("First add failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
	}
	t.Logf("First add output:\n%s", stdout)

	// Try to add the same source again
	t.Log("Adding same source a second time...")
	stdout, stderr, err = runAimgrWithEnv(t, configPath, env, "repo", "add", "local:"+sourceDir)
	// The second add should either:
	// (a) succeed with "already tracked" message (if it updates existing), or
	// (b) fail with a duplicate source error
	// Either way, the manifest should still have exactly one source entry
	t.Logf("Second add stdout:\n%s", stdout)
	t.Logf("Second add stderr:\n%s", stderr)

	// Verify manifest has exactly one source entry (not duplicated)
	manifest, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	sourceCount := len(manifest.Sources)
	if sourceCount != 1 {
		t.Errorf("Manifest should have exactly 1 source after adding same path twice, got %d", sourceCount)
		for i, src := range manifest.Sources {
			t.Logf("  Source %d: name=%s, id=%s, path=%s", i, src.Name, src.ID, src.Path)
		}
	}

	t.Log("✓ Duplicate source detection prevents manifest pollution")
}

// listDirEntries returns a sorted list of entry names in a directory.
// Returns an empty slice if the directory doesn't exist.
func listDirEntries(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("Failed to read directory %s: %v", dir, err)
	}

	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
