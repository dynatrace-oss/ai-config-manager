package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/sourcemetadata"
)

func setupRemoveTestRepo(t *testing.T, sources ...*repomanifest.Source) (string, *repo.Manager) {
	t.Helper()

	tempDir := t.TempDir()
	mgr := repo.NewManagerWithPath(tempDir)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	manifest, err := repomanifest.Load(tempDir)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	for _, source := range sources {
		if err := manifest.AddSource(source); err != nil {
			t.Fatalf("Failed to add source: %v", err)
		}
	}

	if len(sources) > 0 {
		if err := manifest.Save(tempDir); err != nil {
			t.Fatalf("Failed to save manifest: %v", err)
		}
	}

	return tempDir, mgr
}

func TestRepoRemove_ByName(t *testing.T) {
	source := &repomanifest.Source{
		Name: "test-source",
		Path: "/home/user/resources",
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Add a test resource with metadata pointing to this source
	commandPath := filepath.Join(tempDir, "commands", "test-command.md")
	commandContent := `---
name: test-command
description: Test command
---
# Test Command`
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Create metadata for the resource
	meta := &metadata.ResourceMetadata{
		Name:           "test-command",
		Type:           resource.Command,
		SourceType:     "local",
		SourceURL:      "/home/user/resources",
		SourceName:     "test-source",
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta, tempDir, "test-source"); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Verify source exists
	manifest, _ := repomanifest.Load(tempDir)
	if !manifest.HasSource("test-source") {
		t.Fatal("Source should exist before removal")
	}

	// Verify resource exists
	if _, err := os.Stat(commandPath); os.IsNotExist(err) {
		t.Fatal("Resource should exist before removal")
	}

	// Remove the source by name
	if err := performRemove(mgr, "test-source", false, false); err != nil {
		t.Fatalf("Failed to remove source: %v", err)
	}

	// Verify source is removed from manifest
	manifest, _ = repomanifest.Load(tempDir)
	if manifest.HasSource("test-source") {
		t.Error("Source should not exist after removal")
	}

	// Verify orphaned resource is removed
	if _, err := os.Stat(commandPath); !os.IsNotExist(err) {
		t.Error("Orphaned resource should be removed")
	}

	// Verify metadata is removed
	metadataPath := metadata.GetMetadataPath("test-command", resource.Command, tempDir)
	if _, err := os.Stat(metadataPath); !os.IsNotExist(err) {
		t.Error("Resource metadata should be removed")
	}
}

func TestRepoRemove_ByPath(t *testing.T) {
	sourcePath := "/home/user/my-resources"
	source := &repomanifest.Source{
		Name: "my-resources",
		Path: sourcePath,
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Remove the source by path
	if err := performRemove(mgr, sourcePath, false, false); err != nil {
		t.Fatalf("Failed to remove source: %v", err)
	}

	// Verify source is removed
	manifest, _ := repomanifest.Load(tempDir)
	if manifest.HasSource(sourcePath) {
		t.Error("Source should not exist after removal")
	}
}

func TestRepoRemove_ByURL(t *testing.T) {
	sourceURL := "https://github.com/owner/repo"
	source := &repomanifest.Source{
		Name: "owner-repo",
		URL:  sourceURL,
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Remove the source by URL
	if err := performRemove(mgr, sourceURL, false, false); err != nil {
		t.Fatalf("Failed to remove source: %v", err)
	}

	// Verify source is removed
	manifest, _ := repomanifest.Load(tempDir)
	if manifest.HasSource(sourceURL) {
		t.Error("Source should not exist after removal")
	}
}

func TestRepoRemove_DryRun(t *testing.T) {
	source := &repomanifest.Source{
		Name: "test-source",
		Path: "/home/user/resources",
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Add a test resource
	commandPath := filepath.Join(tempDir, "commands", "test-command.md")
	commandContent := `---
name: test-command
description: Test command
---
# Test Command`
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Create metadata for the resource
	meta := &metadata.ResourceMetadata{
		Name:           "test-command",
		Type:           resource.Command,
		SourceType:     "local",
		SourceURL:      "/home/user/resources",
		SourceName:     "test-source",
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta, tempDir, "test-source"); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Run with dry-run flag
	if err := performRemove(mgr, "test-source", true, false); err != nil {
		t.Fatalf("Failed to run dry-run: %v", err)
	}

	// Verify source still exists (not actually removed)
	manifest, _ := repomanifest.Load(tempDir)
	if !manifest.HasSource("test-source") {
		t.Error("Source should still exist after dry-run")
	}

	// Verify resource still exists (not actually removed)
	if _, err := os.Stat(commandPath); os.IsNotExist(err) {
		t.Error("Resource should still exist after dry-run")
	}
}

func TestRepoRemove_KeepResources(t *testing.T) {
	source := &repomanifest.Source{
		Name: "test-source",
		Path: "/home/user/resources",
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Add a test resource
	commandPath := filepath.Join(tempDir, "commands", "test-command.md")
	commandContent := `---
name: test-command
description: Test command
---
# Test Command`
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Create metadata for the resource
	meta := &metadata.ResourceMetadata{
		Name:           "test-command",
		Type:           resource.Command,
		SourceType:     "local",
		SourceURL:      "/home/user/resources",
		SourceName:     "test-source",
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta, tempDir, "test-source"); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Run with keep-resources flag
	if err := performRemove(mgr, "test-source", false, true); err != nil {
		t.Fatalf("Failed to remove source with keep-resources: %v", err)
	}

	// Verify source is removed from manifest
	manifest, _ := repomanifest.Load(tempDir)
	if manifest.HasSource("test-source") {
		t.Error("Source should not exist after removal")
	}

	// Verify resource still exists (kept with --keep-resources)
	if _, err := os.Stat(commandPath); os.IsNotExist(err) {
		t.Error("Resource should still exist with --keep-resources")
	}
}

func TestRepoRemove_NotFound(t *testing.T) {
	_, mgr := setupRemoveTestRepo(t)

	// Try to remove non-existent source
	err := performRemove(mgr, "nonexistent-source", false, false)
	if err == nil {
		t.Fatal("Expected error when removing non-existent source")
	}

	// Verify error message
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' in error message, got: %v", err)
	}
}

func TestRepoRemove_MultipleResources(t *testing.T) {
	source := &repomanifest.Source{
		Name: "test-source",
		Path: "/home/user/resources",
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Add multiple test resources
	commandPath := filepath.Join(tempDir, "commands", "test-command.md")
	commandContent := `---
name: test-command
description: Test command
---
# Test Command`
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Create skill directory and SKILL.md
	skillPath := filepath.Join(tempDir, "skills", "test-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	skillFile := filepath.Join(skillPath, "SKILL.md")
	skillContent := `---
name: test-skill
description: Test skill
---
# Test Skill`
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create skill: %v", err)
	}

	// Create metadata for command
	commandMeta := &metadata.ResourceMetadata{
		Name:           "test-command",
		Type:           resource.Command,
		SourceType:     "local",
		SourceURL:      "/home/user/resources",
		SourceName:     "test-source",
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(commandMeta, tempDir, "test-source"); err != nil {
		t.Fatalf("Failed to save command metadata: %v", err)
	}

	// Create metadata for skill
	skillMeta := &metadata.ResourceMetadata{
		Name:           "test-skill",
		Type:           resource.Skill,
		SourceType:     "local",
		SourceURL:      "/home/user/resources",
		SourceName:     "test-source",
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(skillMeta, tempDir, "test-source"); err != nil {
		t.Fatalf("Failed to save skill metadata: %v", err)
	}

	// Remove the source
	if err := performRemove(mgr, "test-source", false, false); err != nil {
		t.Fatalf("Failed to remove source: %v", err)
	}

	// Verify source is removed
	manifest, _ := repomanifest.Load(tempDir)
	if manifest.HasSource("test-source") {
		t.Error("Source should not exist after removal")
	}

	// Verify all orphaned resources are removed
	if _, err := os.Stat(commandPath); !os.IsNotExist(err) {
		t.Error("Command should be removed")
	}
	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Error("Skill should be removed")
	}
}

func TestRepoRemove_GitCommit(t *testing.T) {
	source := &repomanifest.Source{
		Name: "test-source",
		Path: "/home/user/resources",
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Commit the initial state
	if err := mgr.CommitChangesForPaths("test: add source", []string{repomanifest.ManifestFileName}); err != nil {
		t.Fatalf("Failed to commit initial state: %v", err)
	}

	// Create unrelated tracked file and leave it modified so remove commit must not sweep it in.
	unrelatedPath := filepath.Join(tempDir, "unrelated.txt")
	if err := os.WriteFile(unrelatedPath, []byte("base\n"), 0644); err != nil {
		t.Fatalf("failed to create unrelated file: %v", err)
	}
	if err := mgr.CommitChangesForPaths("test: add unrelated baseline", []string{"unrelated.txt"}); err != nil {
		t.Fatalf("failed to commit unrelated baseline: %v", err)
	}
	if err := os.WriteFile(unrelatedPath, []byte("base\nlocal change\n"), 0644); err != nil {
		t.Fatalf("failed to modify unrelated file: %v", err)
	}

	// Helper to get git status
	gitStatus := func() string {
		cmd := exec.Command("git", "status", "--porcelain")
		cmd.Dir = tempDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to get git status: %v", err)
		}
		return strings.TrimSpace(string(output))
	}

	// Verify only unrelated file is modified before removal.
	if status := gitStatus(); status != "M unrelated.txt" {
		t.Fatalf("Expected only unrelated.txt modified before removal, got: %s", status)
	}

	// Remove the source
	if err := performRemove(mgr, "test-source", false, false); err != nil {
		t.Fatalf("Failed to remove source: %v", err)
	}

	// Verify manifest changes are committed while unrelated local edit remains.
	if status := gitStatus(); status != "M unrelated.txt" {
		t.Errorf("Expected only unrelated.txt modified after repo remove, got:\n%s", status)
		t.Error("This indicates manifest changes were not committed (bug ai-config-manager-dvg)")
	}

	// Verify the commit message
	cmd := exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get git log: %v", err)
	}
	logOutput := string(output)
	expectedMsg := "aimgr: remove source from manifest"
	if !strings.Contains(logOutput, expectedMsg) {
		t.Errorf("Expected commit message %q not found in git log:\n%s", expectedMsg, logOutput)
	}

	// Verify remove commit did not include unrelated changes.
	cmd = exec.Command("git", "show", "--name-only", "--pretty=format:", "HEAD")
	cmd.Dir = tempDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to inspect latest commit files: %v", err)
	}
	showOutput := string(output)
	if strings.Contains(showOutput, "unrelated.txt") {
		t.Errorf("remove manifest commit should not include unrelated.txt, got:\n%s", showOutput)
	}

	statusAfter := gitStatus()
	if statusAfter != "M unrelated.txt" {
		t.Errorf("expected unrelated.txt to remain modified after repo remove, status:\n%s", statusAfter)
	}
}

func TestRepoRemove_OnlyOrphansFromSource(t *testing.T) {
	source1 := &repomanifest.Source{
		Name: "source-1",
		Path: "/home/user/resources1",
	}
	source2 := &repomanifest.Source{
		Name: "source-2",
		Path: "/home/user/resources2",
	}

	tempDir, mgr := setupRemoveTestRepo(t, source1, source2)

	// Add resources from both sources
	command1Path := filepath.Join(tempDir, "commands", "command1.md")
	command1Content := `---
name: command1
description: Command from source 1
---
# Command 1`
	if err := os.WriteFile(command1Path, []byte(command1Content), 0644); err != nil {
		t.Fatalf("Failed to create command1: %v", err)
	}

	command2Path := filepath.Join(tempDir, "commands", "command2.md")
	command2Content := `---
name: command2
description: Command from source 2
---
# Command 2`
	if err := os.WriteFile(command2Path, []byte(command2Content), 0644); err != nil {
		t.Fatalf("Failed to create command2: %v", err)
	}

	// Create metadata linking to different sources
	meta1 := &metadata.ResourceMetadata{
		Name:           "command1",
		Type:           resource.Command,
		SourceType:     "local",
		SourceURL:      "/home/user/resources1",
		SourceName:     "source-1",
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta1, tempDir, "source-1"); err != nil {
		t.Fatalf("Failed to save metadata for command1: %v", err)
	}

	meta2 := &metadata.ResourceMetadata{
		Name:           "command2",
		Type:           resource.Command,
		SourceType:     "local",
		SourceURL:      "/home/user/resources2",
		SourceName:     "source-2",
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta2, tempDir, "source-2"); err != nil {
		t.Fatalf("Failed to save metadata for command2: %v", err)
	}

	// Remove source-1
	if err := performRemove(mgr, "source-1", false, false); err != nil {
		t.Fatalf("Failed to remove source: %v", err)
	}

	// Verify source-1 is removed but source-2 still exists
	manifest, _ := repomanifest.Load(tempDir)
	if manifest.HasSource("source-1") {
		t.Error("source-1 should not exist after removal")
	}
	if !manifest.HasSource("source-2") {
		t.Error("source-2 should still exist")
	}

	// Verify only command1 is removed (from source-1), command2 should remain
	if _, err := os.Stat(command1Path); !os.IsNotExist(err) {
		t.Error("command1 should be removed")
	}
	if _, err := os.Stat(command2Path); os.IsNotExist(err) {
		t.Error("command2 should still exist (from source-2)")
	}
}

func TestRepoRemove_CleansUpSourceMetadata(t *testing.T) {
	source := &repomanifest.Source{
		Name: "test-source",
		Path: "/home/user/resources",
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Create source metadata entry
	srcMeta, err := sourcemetadata.Load(tempDir)
	if err != nil {
		t.Fatalf("Failed to load source metadata: %v", err)
	}
	srcMeta.SetAdded("test-source", time.Now())
	srcMeta.SetLastSynced("test-source", time.Now())
	if err := srcMeta.Save(tempDir); err != nil {
		t.Fatalf("Failed to save source metadata: %v", err)
	}

	// Verify source metadata exists before removal
	srcMeta, _ = sourcemetadata.Load(tempDir)
	if srcMeta.Get("test-source") == nil {
		t.Fatal("Source metadata should exist before removal")
	}

	// Remove the source
	if err := performRemove(mgr, "test-source", false, false); err != nil {
		t.Fatalf("Failed to remove source: %v", err)
	}

	// Verify source metadata is cleaned up
	srcMeta, err = sourcemetadata.Load(tempDir)
	if err != nil {
		t.Fatalf("Failed to load source metadata after removal: %v", err)
	}
	if srcMeta.Get("test-source") != nil {
		t.Error("Source metadata should be cleaned up after removal")
	}
}

func TestRepoRemove_CleansUpSourceMetadata_KeepResources(t *testing.T) {
	source := &repomanifest.Source{
		Name: "test-source",
		Path: "/home/user/resources",
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Add a test resource so we can verify it's kept
	commandPath := filepath.Join(tempDir, "commands", "test-command.md")
	commandContent := `---
name: test-command
description: Test command
---
# Test Command`
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Create resource metadata
	meta := &metadata.ResourceMetadata{
		Name:           "test-command",
		Type:           resource.Command,
		SourceType:     "local",
		SourceURL:      "/home/user/resources",
		SourceName:     "test-source",
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta, tempDir, "test-source"); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Create source metadata entry
	srcMeta, err := sourcemetadata.Load(tempDir)
	if err != nil {
		t.Fatalf("Failed to load source metadata: %v", err)
	}
	srcMeta.SetAdded("test-source", time.Now())
	srcMeta.SetLastSynced("test-source", time.Now())
	if err := srcMeta.Save(tempDir); err != nil {
		t.Fatalf("Failed to save source metadata: %v", err)
	}

	// Remove with --keep-resources
	if err := performRemove(mgr, "test-source", false, true); err != nil {
		t.Fatalf("Failed to remove source with keep-resources: %v", err)
	}

	// Verify source metadata is STILL cleaned up (regardless of --keep-resources)
	srcMeta, err = sourcemetadata.Load(tempDir)
	if err != nil {
		t.Fatalf("Failed to load source metadata after removal: %v", err)
	}
	if srcMeta.Get("test-source") != nil {
		t.Error("Source metadata should be cleaned up even with --keep-resources")
	}

	// Verify resources are kept
	if _, err := os.Stat(commandPath); os.IsNotExist(err) {
		t.Error("Resource should still exist with --keep-resources")
	}
}

func TestRepoRemove_RenamedSourceMatchesByID(t *testing.T) {
	// Test that resources are found as orphans even after a source is renamed,
	// because the source ID stays the same (it's based on path/URL, not name).
	source := &repomanifest.Source{
		Name: "original-name",
		Path: "/home/user/my-resources",
	}

	// Step 1: Add source with original name
	tempDir, mgr := setupRemoveTestRepo(t, source)

	// AddSource auto-generates ID; capture it
	sourceID := source.ID
	if sourceID == "" {
		t.Fatal("Expected source to have an auto-generated ID")
	}

	// Step 2: Add a resource with metadata that has both source_name and source_id
	commandPath := filepath.Join(tempDir, "commands", "my-command.md")
	commandContent := `---
name: my-command
description: A test command
---
# My Command`
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	meta := &metadata.ResourceMetadata{
		Name:           "my-command",
		Type:           resource.Command,
		SourceType:     "local",
		SourceURL:      "/home/user/my-resources",
		SourceName:     "original-name",
		SourceID:       sourceID,
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta, tempDir, "original-name"); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Step 3: Rename source in manifest (simulating manual edit of ai.repo.yaml).
	// The path stays the same, so the ID stays the same.
	manifest, err := repomanifest.Load(tempDir)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}

	// Remove old and add new with same path (same ID) but different name
	_, err = manifest.RemoveSource("original-name")
	if err != nil {
		t.Fatalf("Failed to remove original source: %v", err)
	}

	renamedSource := &repomanifest.Source{
		ID:   sourceID, // Keep the same ID
		Name: "new-name",
		Path: "/home/user/my-resources",
	}
	if err := manifest.AddSource(renamedSource); err != nil {
		t.Fatalf("Failed to add renamed source: %v", err)
	}

	if err := manifest.Save(tempDir); err != nil {
		t.Fatalf("Failed to save manifest after rename: %v", err)
	}

	// Verify resource still exists before remove
	if _, err := os.Stat(commandPath); os.IsNotExist(err) {
		t.Fatal("Resource should exist before removal")
	}

	// Step 4: Remove using the new name — orphan detection should match by source ID
	if err := performRemove(mgr, "new-name", false, false); err != nil {
		t.Fatalf("Failed to remove renamed source: %v", err)
	}

	// Step 5: Verify source is removed from manifest
	manifest, _ = repomanifest.Load(tempDir)
	if manifest.HasSource("new-name") {
		t.Error("Source should not exist after removal")
	}

	// Verify resource was found as orphan and removed (matched by source_id)
	if _, err := os.Stat(commandPath); !os.IsNotExist(err) {
		t.Error("Resource should be removed as orphan (matched by source ID)")
	}

	// Verify metadata was also removed
	metadataPath := metadata.GetMetadataPath("my-command", resource.Command, tempDir)
	if _, err := os.Stat(metadataPath); !os.IsNotExist(err) {
		t.Error("Resource metadata should be removed")
	}
}

func TestRepoRemove_LegacyResourceWithoutSourceID(t *testing.T) {
	// Test backward compatibility: resources without source_id should still be
	// matched by source_name.
	source := &repomanifest.Source{
		Name: "legacy-source",
		Path: "/home/user/legacy",
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Add a resource with metadata that has source_name but NO source_id (legacy)
	commandPath := filepath.Join(tempDir, "commands", "legacy-cmd.md")
	commandContent := `---
name: legacy-cmd
description: Legacy command
---
# Legacy Command`
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	meta := &metadata.ResourceMetadata{
		Name:       "legacy-cmd",
		Type:       resource.Command,
		SourceType: "local",
		SourceURL:  "/home/user/legacy",
		SourceName: "legacy-source",
		// SourceID intentionally empty — legacy resource
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta, tempDir, "legacy-source"); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Remove the source — should still find the resource by name fallback
	if err := performRemove(mgr, "legacy-source", false, false); err != nil {
		t.Fatalf("Failed to remove source: %v", err)
	}

	// Verify resource was found as orphan and removed
	if _, err := os.Stat(commandPath); !os.IsNotExist(err) {
		t.Error("Legacy resource should be removed as orphan (matched by source name)")
	}
}

// TestRepoRemove_WarnsAboutProjectSymlinks tests that removing orphaned resources
// emits a warning about potentially breaking project symlinks.
func TestRepoRemove_WarnsAboutProjectSymlinks(t *testing.T) {
	source := &repomanifest.Source{
		Name: "warn-source",
		Path: "/home/user/resources",
		ID:   "src-warn",
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Add a test resource with metadata pointing to this source
	commandPath := filepath.Join(tempDir, "commands", "warn-cmd.md")
	commandContent := "---\nname: warn-cmd\ndescription: Warning test command\n---\n# Warning Test Cmd\n"
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}
	meta := &metadata.ResourceMetadata{
		Name:           "warn-cmd",
		Type:           resource.Command,
		SourceType:     "local",
		SourceURL:      "/home/user/resources",
		SourceName:     "warn-source",
		SourceID:       "src-warn",
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta, tempDir, "warn-source"); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Capture stderr to check for warning
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := performRemove(mgr, "warn-source", false, false)

	// Restore stderr and read captured output
	w.Close()
	os.Stderr = oldStderr
	captured := make([]byte, 4096)
	n, _ := r.Read(captured)
	stderrOutput := string(captured[:n])

	if err != nil {
		t.Fatalf("performRemove failed: %v", err)
	}

	// Verify warning was emitted
	if !strings.Contains(stderrOutput, "may break symlinks") {
		t.Errorf("Expected warning about breaking symlinks, got stderr: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "command/warn-cmd") {
		t.Errorf("Expected warning to mention resource name, got stderr: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "aimgr repair") {
		t.Errorf("Expected warning to suggest 'aimgr repair', got stderr: %q", stderrOutput)
	}
}

// TestRepoRemove_WarnsAboutProjectSymlinks_DryRun tests that the warning
// is also shown in dry-run mode.
func TestRepoRemove_WarnsAboutProjectSymlinks_DryRun(t *testing.T) {
	source := &repomanifest.Source{
		Name: "dryrun-warn-source",
		Path: "/home/user/resources",
		ID:   "src-dryrun-warn",
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Add a test resource
	commandPath := filepath.Join(tempDir, "commands", "dryrun-cmd.md")
	commandContent := "---\nname: dryrun-cmd\ndescription: Dry run test command\n---\n# Dry Run Cmd\n"
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}
	meta := &metadata.ResourceMetadata{
		Name:           "dryrun-cmd",
		Type:           resource.Command,
		SourceType:     "local",
		SourceURL:      "/home/user/resources",
		SourceName:     "dryrun-warn-source",
		SourceID:       "src-dryrun-warn",
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta, tempDir, "dryrun-warn-source"); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Capture stderr to check for warning
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := performRemove(mgr, "dryrun-warn-source", true, false) // dry-run = true

	// Restore stderr and read captured output
	w.Close()
	os.Stderr = oldStderr
	captured := make([]byte, 4096)
	n, _ := r.Read(captured)
	stderrOutput := string(captured[:n])

	if err != nil {
		t.Fatalf("performRemove failed: %v", err)
	}

	// Verify warning was emitted even in dry-run mode
	if !strings.Contains(stderrOutput, "may break symlinks") {
		t.Errorf("Expected warning about breaking symlinks in dry-run mode, got stderr: %q", stderrOutput)
	}

	// In dry-run mode, the resource should NOT actually be removed
	if _, err := os.Stat(commandPath); err != nil {
		t.Error("Resource should still exist in dry-run mode")
	}
}

// TestRepoRemove_NoWarningWhenKeepResources tests that no symlink warning
// is emitted when --keep-resources is used (since resources won't break).
func TestRepoRemove_NoWarningWhenKeepResources(t *testing.T) {
	source := &repomanifest.Source{
		Name: "keep-source",
		Path: "/home/user/resources",
		ID:   "src-keep",
	}

	tempDir, mgr := setupRemoveTestRepo(t, source)

	// Add a test resource
	commandPath := filepath.Join(tempDir, "commands", "keep-cmd.md")
	commandContent := "---\nname: keep-cmd\ndescription: Keep test command\n---\n# Keep Cmd\n"
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}
	meta := &metadata.ResourceMetadata{
		Name:           "keep-cmd",
		Type:           resource.Command,
		SourceType:     "local",
		SourceURL:      "/home/user/resources",
		SourceName:     "keep-source",
		SourceID:       "src-keep",
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta, tempDir, "keep-source"); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := performRemove(mgr, "keep-source", false, true) // keepResources = true

	w.Close()
	os.Stderr = oldStderr
	captured := make([]byte, 4096)
	n, _ := r.Read(captured)
	stderrOutput := string(captured[:n])

	if err != nil {
		t.Fatalf("performRemove failed: %v", err)
	}

	// No symlink warning should be emitted when keeping resources
	if strings.Contains(stderrOutput, "may break symlinks") {
		t.Errorf("Should NOT warn about symlinks when --keep-resources is used, got stderr: %q", stderrOutput)
	}

	// Resource should still exist
	if _, err := os.Stat(commandPath); err != nil {
		t.Error("Resource should still exist with --keep-resources")
	}
}
