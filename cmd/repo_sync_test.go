package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/sourcemetadata"
)

// Test helper: create a minimal test source directory with resources
func createTestSource(t *testing.T) string {
	t.Helper()

	sourceDir := t.TempDir()

	// Create directory structure
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "skills"), 0755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "agents"), 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}

	// Create test commands
	testCommands := []struct {
		name    string
		content string
	}{
		{"sync-test-cmd", "Sync test command"},
		{"test-command", "Test command"},
		{"pdf-command", "PDF command"},
	}
	for _, cmd := range testCommands {
		content := fmt.Sprintf("---\ndescription: %s\n---\n# %s", cmd.content, cmd.name)
		path := filepath.Join(sourceDir, "commands", cmd.name+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create command %s: %v", cmd.name, err)
		}
	}

	// Create test skills
	testSkills := []struct {
		name    string
		content string
	}{
		{"sync-test-skill", "Sync test skill"},
		{"pdf-processing", "PDF processing skill"},
		{"image-processing", "Image processing skill"},
	}
	for _, skill := range testSkills {
		skillDir := filepath.Join(sourceDir, "skills", skill.name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("failed to create skill dir %s: %v", skill.name, err)
		}
		content := fmt.Sprintf("---\ndescription: %s\n---\n# %s", skill.content, skill.name)
		path := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create skill %s: %v", skill.name, err)
		}
	}

	// Create test agents
	testAgents := []struct {
		name    string
		content string
	}{
		{"sync-test-agent", "Sync test agent"},
		{"code-reviewer", "Code reviewer agent"},
	}
	for _, agent := range testAgents {
		content := fmt.Sprintf("---\ndescription: %s\n---\n# %s", agent.content, agent.name)
		path := filepath.Join(sourceDir, "agents", agent.name+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create agent %s: %v", agent.name, err)
		}
	}

	return sourceDir
}

// setupTestManifest creates a test repository with ai.repo.yaml
// Returns the repo path for verification and a cleanup function
func setupTestManifest(t *testing.T, sources []*repomanifest.Source) (string, func()) {
	t.Helper()

	// Create temp directory for repo
	repoPath := t.TempDir()

	// Create manifest
	manifest := &repomanifest.Manifest{
		Version: 1,
		Sources: sources,
	}

	// Save manifest to repo
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to save test manifest: %v", err)
	}

	// Use AIMGR_REPO_PATH to override repo location
	// This is the highest priority override in repo.NewManager()
	originalRepoPath := os.Getenv("AIMGR_REPO_PATH")
	os.Setenv("AIMGR_REPO_PATH", repoPath)

	cleanup := func() {
		if originalRepoPath != "" {
			os.Setenv("AIMGR_REPO_PATH", originalRepoPath)
		} else {
			_ = os.Unsetenv("AIMGR_REPO_PATH")
		}
	}

	return repoPath, cleanup
}

// verifySourceSynced checks that a source was successfully synced
// Verifies that last_synced timestamp was updated
func verifySourceSynced(t *testing.T, repoPath string, sourceName string) {
	t.Helper()

	// Load manifest
	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	// Find source
	_, found := manifest.GetSource(sourceName)
	if !found {
		t.Fatalf("source %s not found in manifest", sourceName)
	}

	// Load source metadata to check last_synced
	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load source metadata: %v", err)
	}

	state := metadata.Get(sourceName)
	if state == nil {
		t.Fatalf("source %s state not found in metadata", sourceName)
	}

	// Verify last_synced was updated
	if state.LastSynced.IsZero() {
		t.Errorf("source %s last_synced timestamp not updated", sourceName)
	}

	// Verify timestamp is recent (within last minute)
	if time.Since(state.LastSynced) > time.Minute {
		t.Errorf("source %s last_synced timestamp is too old: %v", sourceName, state.LastSynced)
	}

}

// Test helper: verify resources exist in repo
func verifyResourcesInRepo(t *testing.T, repoPath string, resourceType resource.ResourceType, names ...string) {
	t.Helper()

	for _, name := range names {
		var path string
		switch resourceType {
		case resource.Command:
			path = filepath.Join(repoPath, "commands", name+".md")
		case resource.Skill:
			path = filepath.Join(repoPath, "skills", name)
		case resource.Agent:
			path = filepath.Join(repoPath, "agents", name+".md")
		}

		if _, err := os.Stat(path); err != nil {
			t.Errorf("resource %s/%s not found in repo at %s: %v", resourceType, name, path, err)
		}
	}
}

// Test helper: verify resources do NOT exist in repo
func verifyResourcesNotInRepo(t *testing.T, repoPath string, resourceType resource.ResourceType, names ...string) {
	t.Helper()

	for _, name := range names {
		var path string
		switch resourceType {
		case resource.Command:
			path = filepath.Join(repoPath, "commands", name+".md")
		case resource.Skill:
			path = filepath.Join(repoPath, "skills", name)
		case resource.Agent:
			path = filepath.Join(repoPath, "agents", name+".md")
		}

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("resource %s/%s should not exist in repo at %s", resourceType, name, path)
		}
	}
}

// TestRunSync_SingleSource tests syncing from a single local source
func TestRunSync_SingleSource(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest with one source
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Verify resources were imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Verify source was marked as synced
	verifySourceSynced(t, repoPath, "test-source-1")
}

// TestRunSync_MultipleSources tests syncing from multiple sources
func TestRunSync_MultipleSources(t *testing.T) {
	source1 := createTestSource(t)
	source2 := createTestSource(t)

	// Create manifest with multiple sources
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
		{
			Name: "test-source-2",
			Path: source2,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Verify resources from both sources were imported (should be deduplicated)
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Verify both sources were marked as synced
	verifySourceSynced(t, repoPath, "test-source-1")
	verifySourceSynced(t, repoPath, "test-source-2")
}

// TestRunSync_DryRun tests that --dry-run doesn't actually import
func TestRunSync_DryRun(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command with --dry-run
	syncDryRunFlag = true
	defer func() { syncDryRunFlag = false }()
	err := runSync(syncCmd, []string{})

	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Verify NO resources were actually imported (dry run)
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesNotInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesNotInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Verify last_synced was NOT updated (dry run)
	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	_, found := manifest.GetSource("test-source-1")
	if !found {
		t.Fatalf("source not found")
	}
	// Check that last_synced was not updated in metadata
	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load source metadata: %v", err)
	}
	state := metadata.Get("test-source-1")
	if state != nil && !state.LastSynced.IsZero() {
		t.Errorf("dry run should not update last_synced timestamp")
	}
}

// TestRunSync_SkipExisting tests that --skip-existing doesn't overwrite
func TestRunSync_SkipExisting(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Pre-populate repo with one resource (modified version)
	existingCmdDir := filepath.Join(repoPath, "commands")
	if err := os.MkdirAll(existingCmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	existingContent := "---\ndescription: EXISTING VERSION\n---\n# existing"
	existingPath := filepath.Join(existingCmdDir, "sync-test-cmd.md")
	if err := os.WriteFile(existingPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create existing resource: %v", err)
	}

	// Read original content
	originalData, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("failed to read existing resource: %v", err)
	}
	originalContent := string(originalData)

	// Run sync command with --skip-existing
	syncSkipExistingFlag = true
	defer func() { syncSkipExistingFlag = false }()
	err = runSync(syncCmd, []string{})

	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Verify existing resource was NOT overwritten
	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("failed to read resource after sync: %v", err)
	}
	newContent := string(data)

	if newContent != originalContent {
		t.Errorf("--skip-existing failed: existing resource was overwritten")
		t.Logf("Original: %s", originalContent)
		t.Logf("After sync: %s", newContent)
	}

	// Verify other resources were imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Verify source was marked as synced
	verifySourceSynced(t, repoPath, "test-source-1")
}

// TestRunSync_DefaultForce tests that by default, existing resources are overwritten
func TestRunSync_DefaultForce(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Pre-populate repo with one resource (modified version)
	existingCmdDir := filepath.Join(repoPath, "commands")
	if err := os.MkdirAll(existingCmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	existingContent := "---\ndescription: EXISTING VERSION\n---\n# existing"
	existingPath := filepath.Join(existingCmdDir, "sync-test-cmd.md")
	if err := os.WriteFile(existingPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create existing resource: %v", err)
	}

	// Read original content
	originalData, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("failed to read existing resource: %v", err)
	}
	originalContent := string(originalData)

	// Run sync command (default force behavior)
	err = runSync(syncCmd, []string{})

	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Verify existing resource WAS overwritten (force is default)
	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("failed to read resource after sync: %v", err)
	}
	newContent := string(data)

	if newContent == originalContent {
		t.Errorf("default force failed: existing resource was not overwritten")
		t.Logf("Content unchanged: %s", newContent)
	}

	// Verify it contains the new content from source
	if !contains(newContent, "Sync test command") {
		t.Errorf("resource content doesn't match source after force update")
		t.Logf("Content: %s", newContent)
	}

	// Verify all resources were imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Verify source was marked as synced
	verifySourceSynced(t, repoPath, "test-source-1")
}

// TestRunSync_NoSources tests error when no sources configured
func TestRunSync_NoSources(t *testing.T) {
	// Create manifest with NO sources
	sources := []*repomanifest.Source{}
	_, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command
	err := runSync(syncCmd, []string{})

	// Should return error
	if err == nil {
		t.Fatal("expected error when no sources configured, got nil")
	}

	// Check error message
	expectedMsg := "no sync sources configured"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("error message doesn't contain expected text\nGot: %s\nWant substring: %s", err.Error(), expectedMsg)
	}
}

// TestRunSync_InvalidSource tests error handling for invalid sources
func TestRunSync_InvalidSource(t *testing.T) {
	// Create manifest with invalid source (non-existent path)
	sources := []*repomanifest.Source{
		{
			Name: "invalid-source",
			Path: "/nonexistent/path/that/does/not/exist",
		},
	}
	_, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command
	err := runSync(syncCmd, []string{})

	// Should return error since all sources failed
	if err == nil {
		t.Fatal("expected error for invalid source, got nil")
	}

	// Check error message indicates failure
	if !contains(err.Error(), "all sources failed") {
		t.Errorf("error message doesn't indicate all sources failed\nGot: %s", err.Error())
	}
}

// TestRunSync_MixedValidInvalidSources tests partial success with mixed sources
func TestRunSync_MixedValidInvalidSources(t *testing.T) {
	validSource := createTestSource(t)

	// Create manifest with one valid and one invalid source
	sources := []*repomanifest.Source{
		{
			Name: "valid-source",
			Path: validSource,
		},
		{
			Name: "invalid-source",
			Path: "/nonexistent/path",
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command
	err := runSync(syncCmd, []string{})

	// Should succeed since at least one source worked
	if err != nil {
		t.Fatalf("sync command should succeed with partial success, got: %v", err)
	}

	// Verify resources from valid source were imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Verify valid source was marked as synced
	verifySourceSynced(t, repoPath, "valid-source")

	// Verify invalid source was NOT marked as synced
	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	_, found := manifest.GetSource("invalid-source")
	if !found {
		t.Fatalf("invalid-source not found in manifest")
	}
	// Check that invalid source has no last_synced in metadata
	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load source metadata: %v", err)
	}
	state := metadata.Get("invalid-source")
	if state != nil && !state.LastSynced.IsZero() {
		t.Errorf("invalid source should not have last_synced timestamp updated")
	}
}

// TestRunSync_UpdatesLastSynced tests that last_synced timestamps are updated
func TestRunSync_UpdatesLastSynced(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest with old last_synced timestamp
	oldTimestamp := time.Now().Add(-24 * time.Hour)
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Load manifest and verify timestamp was updated
	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	_, found := manifest.GetSource("test-source-1")
	if !found {
		t.Fatalf("source not found")
	}

	// Verify timestamp was updated (should be recent, not old)
	// Check via metadata
	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	state := metadata.Get("test-source-1")
	if state == nil {
		t.Fatalf("state not found")
	}
	if state.LastSynced.Equal(oldTimestamp) {
		t.Errorf("last_synced timestamp was not updated")
	}
	if time.Since(state.LastSynced) > time.Minute {
		t.Errorf("last_synced timestamp is not recent: %v", state.LastSynced)
	}
}

// TestRunSync_PreservesManifest tests that manifest structure is preserved
func TestRunSync_PreservesManifest(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest with specific structure
	addedTime := time.Now().Add(-48 * time.Hour)
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Create metadata with specific added time
	metadata := &sourcemetadata.SourceMetadata{
		Version: 1,
		Sources: map[string]*sourcemetadata.SourceState{
			"test-source-1": {
				Added:      addedTime,
				LastSynced: time.Time{}, // Not yet synced
			},
		},
	}
	if err := metadata.Save(repoPath); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	// Run sync command
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Load manifest and verify structure is preserved
	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	// Verify version
	if manifest.Version != 1 {
		t.Errorf("manifest version changed: expected 1, got %d", manifest.Version)
	}

	// Verify source count
	if len(manifest.Sources) != 1 {
		t.Errorf("source count changed: expected 1, got %d", len(manifest.Sources))
	}

	// Verify source fields are preserved
	source := manifest.Sources[0]
	if source.Name != "test-source-1" {
		t.Errorf("source name changed: expected test-source-1, got %s", source.Name)
	}
	if source.Path != source1 {
		t.Errorf("source path changed: expected %s, got %s", source1, source.Path)
	}
	if source.GetMode() != "symlink" {
		t.Errorf("source mode changed: expected symlink, got %s", source.GetMode())
	}

	// Verify added timestamp is preserved in metadata
	loadedMetadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	state := loadedMetadata.Get("test-source-1")
	if state == nil {
		t.Fatalf("state not found")
	}
	if state.Added.Sub(addedTime).Abs() > time.Second {
		t.Errorf("added timestamp changed: expected %v, got %v", addedTime, state.Added)
	}

	// Verify last_synced was added
	if state.LastSynced.IsZero() {
		t.Errorf("last_synced timestamp was not added")
	}
}

// TestRunSync_WithIncludeFilter verifies that a source with include filters
// only imports matching resources and skips non-matching ones.
func TestRunSync_WithIncludeFilter(t *testing.T) {
	sourceDir := createTestSource(t)

	// Source has include: only import pdf-command and the pdf-processing skill
	sources := []*repomanifest.Source{
		{
			Name:    "filtered-source",
			Path:    sourceDir,
			ID:      "src-filter-test",
			Include: []string{"command/pdf-command", "skill/pdf-processing"},
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync with include filter failed: %v", err)
	}

	// Only the included resources should be imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "pdf-processing")

	// Non-matching resources should NOT be imported
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command")
	verifyResourcesNotInRepo(t, repoPath, resource.Skill, "sync-test-skill", "image-processing")
	verifyResourcesNotInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")
}

// TestRunSync_WithIncludeFilter_BackwardCompat verifies that a source without
// include filters imports all resources (backward compatibility).
func TestRunSync_WithIncludeFilter_BackwardCompat(t *testing.T) {
	sourceDir := createTestSource(t)

	// Source has NO include filter — all resources should be imported
	sources := []*repomanifest.Source{
		{
			Name: "unfiltered-source",
			Path: sourceDir,
			ID:   "src-no-filter",
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync without include filter failed: %v", err)
	}

	// All resources from the source should be imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")
}

// TestRunSync_IncludeFilterOrphanDetection verifies that a resource previously
// imported (included) is removed from the repo when it is removed from include
// and a sync is performed.
func TestRunSync_IncludeFilterOrphanDetection(t *testing.T) {
	sourceDir := createTestSource(t)

	// First sync: include both pdf-command and test-command
	sources := []*repomanifest.Source{
		{
			Name:    "shrinking-filter-source",
			Path:    sourceDir,
			ID:      "src-shrink-filter",
			Include: []string{"command/pdf-command", "command/test-command"},
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Both included commands should be in the repo
	verifyResourcesInRepo(t, repoPath, resource.Command, "pdf-command", "test-command")

	// Non-included resources should not be present
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "sync-test-cmd")
	verifyResourcesNotInRepo(t, repoPath, resource.Skill, "pdf-processing")

	// Now narrow the include to only pdf-command (remove test-command from include)
	sources[0].Include = []string{"command/pdf-command"}
	manifest := &repomanifest.Manifest{Version: 1, Sources: sources}
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to update manifest with narrowed include: %v", err)
	}

	// Second sync: test-command was previously imported but is now outside include
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// pdf-command is still included — must remain in repo
	verifyResourcesInRepo(t, repoPath, resource.Command, "pdf-command")

	// test-command is no longer in include — must be removed as orphan
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "test-command")
}

// TestRunSync_MetadataCommitted verifies that metadata changes are committed to git
// This is a regression test for bug ai-config-manager-x7k
func TestRunSync_MetadataCommitted(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest with one source
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Initialize as git repository
	manager, err := repo.NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to initialize git repo: %v", err)
	}

	// Run sync command
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Verify git status is clean (bug fix verification)
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get git status: %v", err)
	}

	status := string(output)
	if status != "" {
		t.Errorf("Expected clean working tree after repo sync, but got uncommitted changes:\n%s", status)
		t.Error("This indicates metadata changes were not committed (bug ai-config-manager-x7k)")
	}

	// Verify we have a commit for metadata update
	cmd = exec.Command("git", "-C", repoPath, "log", "--oneline")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get git log: %v", err)
	}

	logOutput := string(output)
	expectedCommitMsg := "aimgr: update sync timestamps"
	if !contains(logOutput, expectedCommitMsg) {
		t.Errorf("Expected commit message %q not found in git log:\n%s", expectedCommitMsg, logOutput)
	}

	// Verify the commit contains .metadata/sources.json
	cmd = exec.Command("git", "-C", repoPath, "show", "--stat", "--oneline", "HEAD")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to show latest commit: %v", err)
	}

	commitShow := string(output)
	if !contains(commitShow, ".metadata/sources.json") {
		t.Error("Metadata commit should include .metadata/sources.json")
	}
}

// TestScanSourceResources_DiscoversAllTypes tests that scanSourceResources
// finds all four resource types (commands, skills, agents, packages)
func TestScanSourceResources_DiscoversAllTypes(t *testing.T) {
	sourceDir := createTestSource(t) // creates commands, skills, agents

	// Also add a package to the source
	pkgDir := filepath.Join(sourceDir, "packages")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create packages dir: %v", err)
	}
	pkgContent := `{"name": "test-pkg", "description": "A test package", "resources": ["command/sync-test-cmd"]}`
	if err := os.WriteFile(filepath.Join(pkgDir, "test-pkg.package.json"), []byte(pkgContent), 0644); err != nil {
		t.Fatalf("failed to create package file: %v", err)
	}

	result, err := scanSourceResources(sourceDir)
	if err != nil {
		t.Fatalf("scanSourceResources failed: %v", err)
	}

	// Verify commands found
	cmdSet := result[resource.Command]
	if cmdSet == nil {
		t.Fatal("no commands found")
	}
	expectedCmds := []string{"sync-test-cmd", "test-command", "pdf-command"}
	for _, name := range expectedCmds {
		if !cmdSet[name] {
			t.Errorf("command %q not found in scan results", name)
		}
	}

	// Verify skills found
	skillSet := result[resource.Skill]
	if skillSet == nil {
		t.Fatal("no skills found")
	}
	expectedSkills := []string{"sync-test-skill", "pdf-processing", "image-processing"}
	for _, name := range expectedSkills {
		if !skillSet[name] {
			t.Errorf("skill %q not found in scan results", name)
		}
	}

	// Verify agents found
	agentSet := result[resource.Agent]
	if agentSet == nil {
		t.Fatal("no agents found")
	}
	expectedAgents := []string{"sync-test-agent", "code-reviewer"}
	for _, name := range expectedAgents {
		if !agentSet[name] {
			t.Errorf("agent %q not found in scan results", name)
		}
	}

	// Verify packages found
	pkgSet := result[resource.PackageType]
	if pkgSet == nil {
		t.Fatal("no packages found")
	}
	if !pkgSet["test-pkg"] {
		t.Error("package 'test-pkg' not found in scan results")
	}
}

// TestScanSourceResources_EmptySource tests scanning an empty source directory
func TestScanSourceResources_EmptySource(t *testing.T) {
	emptyDir := t.TempDir()

	result, err := scanSourceResources(emptyDir)
	if err != nil {
		t.Fatalf("scanSourceResources failed on empty dir: %v", err)
	}

	// All type sets should be nil or empty
	for resType, typeSet := range result {
		if len(typeSet) > 0 {
			t.Errorf("expected no resources of type %s in empty dir, got %d", resType, len(typeSet))
		}
	}
}

// TestScanSourceResources_PartialTypes tests scanning a source with only some types
func TestScanSourceResources_PartialTypes(t *testing.T) {
	sourceDir := t.TempDir()

	// Create only commands (no skills, agents, or packages)
	cmdDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	content := "---\ndescription: A test command\n---\n# only-command"
	if err := os.WriteFile(filepath.Join(cmdDir, "only-command.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	result, err := scanSourceResources(sourceDir)
	if err != nil {
		t.Fatalf("scanSourceResources failed: %v", err)
	}

	// Verify commands found
	cmdSet := result[resource.Command]
	if cmdSet == nil || !cmdSet["only-command"] {
		t.Error("command 'only-command' not found in scan results")
	}

	// Verify other types NOT found
	if result[resource.Skill] != nil {
		t.Error("expected no skills, but found some")
	}
	if result[resource.Agent] != nil {
		t.Error("expected no agents, but found some")
	}
	if result[resource.PackageType] != nil {
		t.Error("expected no packages, but found some")
	}
}

// TestRunSync_DetectsRemovedResources tests that sync detects and removes resources
// deleted from a source between syncs (bmz1.2 detection + bmz1.3 removal)
func TestRunSync_DetectsRemovedResources(t *testing.T) {
	sourceDir := createTestSource(t)

	// Set up source with an ID so pre-sync inventory matches by ID
	sourceID := "src-test-detect-rm"
	sources := []*repomanifest.Source{
		{
			Name: "test-source",
			Path: sourceDir,
			ID:   sourceID,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// First sync: import all resources
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Verify all resources were imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Now remove a command and a skill from the source
	if err := os.Remove(filepath.Join(sourceDir, "commands", "pdf-command.md")); err != nil {
		t.Fatalf("failed to remove command from source: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(sourceDir, "skills", "image-processing")); err != nil {
		t.Fatalf("failed to remove skill from source: %v", err)
	}

	// Second sync: should detect AND remove the orphans (bmz1.3)
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// Resources that were NOT removed from source should still be in repo
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// The removed resources should now be gone from the repo (bmz1.3 removes them)
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "pdf-command")
	verifyResourcesNotInRepo(t, repoPath, resource.Skill, "image-processing")
}

// TestRunSync_FailedSourceExcludedFromRemovalDetection tests that
// resources from failed sources are not flagged for removal
func TestRunSync_FailedSourceExcludedFromRemovalDetection(t *testing.T) {
	validSource := createTestSource(t)

	// Create manifest with one valid and one invalid source
	sources := []*repomanifest.Source{
		{
			Name: "valid-source",
			Path: validSource,
			ID:   "src-valid",
		},
		{
			Name: "will-fail-source",
			Path: "/nonexistent/path/that/does/not/exist",
			ID:   "src-will-fail",
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// First sync: only valid source succeeds
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed (should succeed with partial): %v", err)
	}

	// Verify resources from valid source are imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")

	// Now remove a resource from the valid source
	if err := os.Remove(filepath.Join(validSource, "commands", "pdf-command.md")); err != nil {
		t.Fatalf("failed to remove command: %v", err)
	}

	// Second sync: should succeed without errors about the failed source
	// The failed source should NOT trigger removal detection
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("second sync failed (should succeed with partial): %v", err)
	}

	// Resources from valid source that still exist should be present
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command")

	// pdf-command was removed from source, so sync should have removed it (bmz1.3)
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "pdf-command")
}

// TestRunSync_NoRemovalsWhenSourceUnchanged tests that no removals are
// detected when the source hasn't changed between syncs
func TestRunSync_NoRemovalsWhenSourceUnchanged(t *testing.T) {
	sourceDir := createTestSource(t)

	sources := []*repomanifest.Source{
		{
			Name: "stable-source",
			Path: sourceDir,
			ID:   "src-stable",
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// First sync
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Verify all resources imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Second sync without any changes — no removals should be detected
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// All resources should still be present
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")
}

// TestSyncSource_ReturnsSourcePath tests that syncSource returns the resolved
// source path for post-sync scanning
func TestSyncSource_ReturnsSourcePath(t *testing.T) {
	sourceDir := createTestSource(t)

	// Set up test repo
	sources := []*repomanifest.Source{
		{
			Name: "path-test-source",
			Path: sourceDir,
		},
	}
	_, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Create manager
	manager, err := repo.NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// syncSource should return the source path
	returnedPath, err := syncSource(sources[0], manager)
	if err != nil {
		t.Fatalf("syncSource failed: %v", err)
	}

	// Returned path should be the absolute path to the source
	absSourceDir, _ := filepath.Abs(sourceDir)
	if returnedPath != absSourceDir {
		t.Errorf("syncSource returned path %q, expected %q", returnedPath, absSourceDir)
	}
}

// TestRunSync_DryRunDoesNotRemoveOrphans tests that --dry-run mode detects
// orphaned resources but does NOT actually remove them from the repo
func TestRunSync_DryRunDoesNotRemoveOrphans(t *testing.T) {
	sourceDir := createTestSource(t)

	sourceID := "src-dryrun-rm"
	sources := []*repomanifest.Source{
		{
			Name: "dryrun-source",
			Path: sourceDir,
			ID:   sourceID,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// First sync: import all resources (non-dry-run)
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Verify all resources imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")

	// Remove a command from the source
	if err := os.Remove(filepath.Join(sourceDir, "commands", "pdf-command.md")); err != nil {
		t.Fatalf("failed to remove command from source: %v", err)
	}

	// Second sync with --dry-run: should NOT remove the orphan
	syncDryRunFlag = true
	defer func() { syncDryRunFlag = false }()
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("dry-run sync failed: %v", err)
	}

	// pdf-command should still exist as a dangling symlink in the repo
	// (dry-run should not remove it). Use Lstat to check the symlink entry itself.
	danglingCmd := filepath.Join(repoPath, "commands", "pdf-command.md")
	if _, err := os.Lstat(danglingCmd); err != nil {
		t.Errorf("dry-run should NOT have removed pdf-command, but it's gone: %v", err)
	}
}

// TestRunSync_RemovesOrphansWithGitCommit tests that orphan removals are
// committed to git during sync
func TestRunSync_RemovesOrphansWithGitCommit(t *testing.T) {
	sourceDir := createTestSource(t)

	sourceID := "src-git-rm"
	sources := []*repomanifest.Source{
		{
			Name: "git-rm-source",
			Path: sourceDir,
			ID:   sourceID,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Initialize as git repo
	manager, err := repo.NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// First sync: import all resources
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Remove resources from source
	if err := os.Remove(filepath.Join(sourceDir, "commands", "pdf-command.md")); err != nil {
		t.Fatalf("failed to remove command: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(sourceDir, "skills", "image-processing")); err != nil {
		t.Fatalf("failed to remove skill: %v", err)
	}

	// Second sync: should remove orphans and commit
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// Verify resources were removed
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "pdf-command")
	verifyResourcesNotInRepo(t, repoPath, resource.Skill, "image-processing")

	// Verify git status is clean
	gitCmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	output, err := gitCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get git status: %v", err)
	}
	status := string(output)
	if status != "" {
		t.Errorf("expected clean git status after sync with removals, got:\n%s", status)
	}

	// Verify git log contains removal commits (Manager.Remove commits each individually)
	gitCmd = exec.Command("git", "-C", repoPath, "log", "--oneline")
	output, err = gitCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get git log: %v", err)
	}
	logOutput := string(output)
	if !contains(logOutput, "aimgr: remove command: pdf-command") {
		t.Errorf("expected removal commit for pdf-command in git log, got:\n%s", logOutput)
	}
	if !contains(logOutput, "aimgr: remove skill: image-processing") {
		t.Errorf("expected removal commit for image-processing in git log, got:\n%s", logOutput)
	}
}

// TestRunSync_RemovalOnlyForSuccessfulSources verifies that resources from
// failed sources are NOT removed even if they appear orphaned
func TestRunSync_RemovalOnlyForSuccessfulSources(t *testing.T) {
	// Create two separate source directories
	sourceA := createTestSource(t)
	sourceB := t.TempDir()

	// Source B has a unique command
	cmdDir := filepath.Join(sourceB, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	content := "---\ndescription: Source B only command\n---\n# source-b-cmd"
	if err := os.WriteFile(filepath.Join(cmdDir, "source-b-cmd.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	sources := []*repomanifest.Source{
		{
			Name: "source-a",
			Path: sourceA,
			ID:   "src-a",
		},
		{
			Name: "source-b",
			Path: sourceB,
			ID:   "src-b",
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// First sync: both sources succeed
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Verify source B's resource is imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "source-b-cmd")

	// Now make source B's path invalid (simulating a failed source)
	sources[1].Path = "/nonexistent/source-b-path"
	manifest := &repomanifest.Manifest{Version: 1, Sources: sources}
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to save updated manifest: %v", err)
	}

	// Second sync: source A succeeds, source B fails
	// Source B's resources should NOT be removed because source B failed
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("second sync failed (should succeed with partial): %v", err)
	}

	// Source B's resource should still exist (not removed because source failed)
	// Note: since it was symlinked from the original source B path, the symlink
	// target still exists. Use Lstat to be safe.
	sourceBCmd := filepath.Join(repoPath, "commands", "source-b-cmd.md")
	if _, err := os.Lstat(sourceBCmd); err != nil {
		t.Errorf("source B's resource should NOT have been removed (source B failed to sync): %v", err)
	}
}

// TestRunSync_CrossSourceNameCollision tests that when two sources provide a
// resource with the same name, orphan cleanup does not incorrectly remove the
// resource that was overwritten by the other source (bmz1.7).
//
// Scenario:
//  1. Source A has "shared-cmd" and "cmd-a"
//  2. Source B has "shared-cmd" and "cmd-b"
//  3. After first sync, "shared-cmd" metadata points to Source B (last-write-wins)
//  4. "shared-cmd" is then removed from Source A
//  5. On second sync, orphan detection for Source A sees "shared-cmd" in pre-sync
//     inventory but not in the source. Without bmz1.7, it would remove it.
//  6. With bmz1.7: metadata re-check sees "shared-cmd" now belongs to Source B,
//     so it skips removal. The resource survives.
func TestRunSync_CrossSourceNameCollision(t *testing.T) {
	// Source A: has "shared-cmd" and "cmd-a"
	sourceA := t.TempDir()
	cmdDirA := filepath.Join(sourceA, "commands")
	if err := os.MkdirAll(cmdDirA, 0755); err != nil {
		t.Fatalf("failed to create commands dir for source A: %v", err)
	}
	sharedContent := "---\ndescription: Shared command from A\n---\n# shared-cmd (A)"
	if err := os.WriteFile(filepath.Join(cmdDirA, "shared-cmd.md"), []byte(sharedContent), 0644); err != nil {
		t.Fatalf("failed to create shared-cmd in source A: %v", err)
	}
	cmdAContent := "---\ndescription: Source A only\n---\n# cmd-a"
	if err := os.WriteFile(filepath.Join(cmdDirA, "cmd-a.md"), []byte(cmdAContent), 0644); err != nil {
		t.Fatalf("failed to create cmd-a: %v", err)
	}

	// Source B: has "shared-cmd" and "cmd-b"
	sourceB := t.TempDir()
	cmdDirB := filepath.Join(sourceB, "commands")
	if err := os.MkdirAll(cmdDirB, 0755); err != nil {
		t.Fatalf("failed to create commands dir for source B: %v", err)
	}
	sharedContentB := "---\ndescription: Shared command from B\n---\n# shared-cmd (B)"
	if err := os.WriteFile(filepath.Join(cmdDirB, "shared-cmd.md"), []byte(sharedContentB), 0644); err != nil {
		t.Fatalf("failed to create shared-cmd in source B: %v", err)
	}
	cmdBContent := "---\ndescription: Source B only\n---\n# cmd-b"
	if err := os.WriteFile(filepath.Join(cmdDirB, "cmd-b.md"), []byte(cmdBContent), 0644); err != nil {
		t.Fatalf("failed to create cmd-b: %v", err)
	}

	// Sources: A is synced first, then B overwrites "shared-cmd"
	sources := []*repomanifest.Source{
		{
			Name: "source-a",
			Path: sourceA,
			ID:   "src-collision-a",
		},
		{
			Name: "source-b",
			Path: sourceB,
			ID:   "src-collision-b",
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// First sync: both sources succeed. Source B syncs after A,
	// so "shared-cmd" metadata ends up pointing to Source B.
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Verify all resources exist
	verifyResourcesInRepo(t, repoPath, resource.Command, "shared-cmd", "cmd-a", "cmd-b")

	// Now remove "shared-cmd" from Source A (simulating the resource being deleted
	// from source A between syncs). Source B still has it.
	if err := os.Remove(filepath.Join(cmdDirA, "shared-cmd.md")); err != nil {
		t.Fatalf("failed to remove shared-cmd from source A: %v", err)
	}

	// Second sync: Source A no longer has "shared-cmd" but Source B still does.
	// Orphan detection for Source A should NOT remove "shared-cmd" because
	// its metadata now points to Source B.
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// "shared-cmd" should still exist (not incorrectly removed)
	verifyResourcesInRepo(t, repoPath, resource.Command, "shared-cmd", "cmd-b")

	// "cmd-a" should still exist (it wasn't removed from source A)
	verifyResourcesInRepo(t, repoPath, resource.Command, "cmd-a")
}
