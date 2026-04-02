package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// setupRepairTestRepo initialises a temp repo and sets AIMGR_REPO_PATH.
// Returns the manager and a cleanup function.
func setupRepairTestRepo(t *testing.T) (*repo.Manager, func()) {
	t.Helper()
	repoDir := t.TempDir()

	oldEnv := os.Getenv("AIMGR_REPO_PATH")
	cleanup := func() {
		if oldEnv != "" {
			_ = os.Setenv("AIMGR_REPO_PATH", oldEnv)
		} else {
			_ = os.Unsetenv("AIMGR_REPO_PATH")
		}
	}
	_ = os.Setenv("AIMGR_REPO_PATH", repoDir)

	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	return manager, cleanup
}

// createTestCommand creates a valid .md command file in the repo commands/ directory.
func createTestCommand(t *testing.T, repoPath, name string) {
	t.Helper()
	cmdPath := filepath.Join(repoPath, "commands", name+".md")
	content := fmt.Sprintf("---\ndescription: Test command %s\n---\n\n# %s\n", name, name)
	if err := os.WriteFile(cmdPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create command %s: %v", name, err)
	}
}

// createTestMetadata creates metadata for a given resource (always as a Command type).
// Uses a non-file:// SourceURL to avoid triggering missing-source-path warnings.
func createTestMetadata(t *testing.T, repoPath, name string) {
	t.Helper()
	meta := &metadata.ResourceMetadata{
		Name:       name,
		Type:       resource.Command,
		SourceType: "local",
		SourceURL:  "https://example.com/source", // not file://, so no missing-source-path check
	}
	if err := metadata.Save(meta, repoPath, "local"); err != nil {
		t.Fatalf("Failed to save metadata for %s: %v", name, err)
	}
}

// createTypeMismatchMetadata writes a metadata file into the "correct" commands dir
// but with a different Type field inside, triggering a type mismatch.
func createTypeMismatchMetadata(t *testing.T, repoPath, name string, storedAsType, contentType resource.ResourceType) {
	t.Helper()
	// Build the JSON with contentType but save it at the storedAsType path
	meta := &metadata.ResourceMetadata{
		Name:       name,
		Type:       contentType, // Wrong type in the content
		SourceType: "local",
		SourceURL:  "https://example.com/source",
	}
	// We want the file at the path for storedAsType so GetMetadata(name, storedAsType) finds it
	metaPath := metadata.GetMetadataPath(name, storedAsType, repoPath)
	dir := filepath.Dir(metaPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create metadata dir: %v", err)
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		t.Fatalf("Failed to write metadata file: %v", err)
	}
}

// -----------------------------------------------------------------------
// TestRepoRepair_NoIssues — clean repo, nothing to fix
// -----------------------------------------------------------------------

func TestRepoRepair_NoIssues(t *testing.T) {
	manager, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	repoPath := manager.GetRepoPath()

	// Create a command with proper metadata
	createTestCommand(t, repoPath, "healthy-cmd")
	createTestMetadata(t, repoPath, "healthy-cmd")

	// Run the diagnostic scan
	verifyResult, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository failed: %v", err)
	}

	if verifyResult.HasErrors || verifyResult.HasWarnings {
		t.Fatalf("Expected clean repo but got errors=%v warnings=%v (resources_without_meta=%d orphaned=%d)",
			verifyResult.HasErrors, verifyResult.HasWarnings,
			len(verifyResult.ResourcesWithoutMetadata), len(verifyResult.OrphanedMetadata))
	}

	// Build the repair result — there should be nothing to do
	result := &RepoRepairResult{
		TypeMismatches:          verifyResult.TypeMismatches,
		PackagesWithMissingRefs: verifyResult.PackagesWithMissingRefs,
	}
	result.UnfixableCount = len(result.TypeMismatches) + len(result.PackagesWithMissingRefs)

	if result.FixedCount != 0 {
		t.Errorf("FixedCount = %d, want 0", result.FixedCount)
	}
	if result.UnfixableCount != 0 {
		t.Errorf("UnfixableCount = %d, want 0", result.UnfixableCount)
	}
}

// -----------------------------------------------------------------------
// TestRepoRepair_MissingMetadata — resource exists without metadata → repair creates it
// -----------------------------------------------------------------------

func TestRepoRepair_MissingMetadata(t *testing.T) {
	manager, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	repoPath := manager.GetRepoPath()

	// Create a command without metadata
	createTestCommand(t, repoPath, "no-meta-cmd")

	// Verify the issue is detected
	verifyResult, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository failed: %v", err)
	}
	if len(verifyResult.ResourcesWithoutMetadata) != 1 {
		t.Fatalf("Expected 1 resource without metadata, got %d", len(verifyResult.ResourcesWithoutMetadata))
	}
	if verifyResult.ResourcesWithoutMetadata[0].Name != "no-meta-cmd" {
		t.Errorf("ResourcesWithoutMetadata[0].Name = %q, want %q",
			verifyResult.ResourcesWithoutMetadata[0].Name, "no-meta-cmd")
	}

	// Apply the fix
	result := &RepoRepairResult{}
	for _, issue := range verifyResult.ResourcesWithoutMetadata {
		res := resource.Resource{Name: issue.Name, Type: issue.Type}
		if err := createMetadataForResource(manager, res); err != nil {
			t.Fatalf("createMetadataForResource failed: %v", err)
		}
		result.MetadataCreated = append(result.MetadataCreated, issue)
	}
	result.FixedCount = len(result.MetadataCreated)

	if result.FixedCount != 1 {
		t.Errorf("FixedCount = %d, want 1", result.FixedCount)
	}

	// Verify metadata was actually created
	meta, err := manager.GetMetadata("no-meta-cmd", resource.Command)
	if err != nil {
		t.Errorf("Metadata was not created by repair: %v", err)
	}
	if meta.Name != "no-meta-cmd" {
		t.Errorf("Metadata name = %q, want %q", meta.Name, "no-meta-cmd")
	}

	// Re-run verify — should be clean now
	verifyResult2, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository after repair failed: %v", err)
	}
	if verifyResult2.HasWarnings {
		t.Error("Expected no warnings after repair")
	}
}

// -----------------------------------------------------------------------
// TestRepoRepair_OrphanedMetadata — metadata without resource → repair removes it
// -----------------------------------------------------------------------

func TestRepoRepair_OrphanedMetadata(t *testing.T) {
	manager, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	repoPath := manager.GetRepoPath()

	// Create metadata without corresponding resource
	createTestMetadata(t, repoPath, "orphaned-cmd")

	// Verify the issue is detected
	verifyResult, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository failed: %v", err)
	}
	if len(verifyResult.OrphanedMetadata) != 1 {
		t.Fatalf("Expected 1 orphaned metadata, got %d", len(verifyResult.OrphanedMetadata))
	}

	orphanPath := verifyResult.OrphanedMetadata[0].Path

	// Apply the fix
	result := &RepoRepairResult{}
	for _, issue := range verifyResult.OrphanedMetadata {
		if err := os.Remove(issue.Path); err != nil {
			t.Fatalf("os.Remove failed: %v", err)
		}
		result.OrphanedRemoved = append(result.OrphanedRemoved, issue)
	}
	result.FixedCount = len(result.OrphanedRemoved)

	if result.FixedCount != 1 {
		t.Errorf("FixedCount = %d, want 1", result.FixedCount)
	}

	// Verify the metadata file is gone
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Errorf("Orphaned metadata file still exists at %s", orphanPath)
	}

	// Re-run verify — should be clean now
	verifyResult2, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository after repair failed: %v", err)
	}
	if verifyResult2.HasErrors {
		t.Error("Expected no errors after repair")
	}
}

// -----------------------------------------------------------------------
// TestRepoRepair_TypeMismatch — reported as unfixable
// -----------------------------------------------------------------------

func TestRepoRepair_TypeMismatch(t *testing.T) {
	manager, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	repoPath := manager.GetRepoPath()

	// Create a command with metadata stored at the commands path but with wrong type inside
	createTestCommand(t, repoPath, "mismatch-cmd")
	// Save metadata at .metadata/commands/mismatch-cmd-metadata.json, but with Type=skill inside
	createTypeMismatchMetadata(t, repoPath, "mismatch-cmd", resource.Command, resource.Skill)

	// Run the diagnostic scan
	verifyResult, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository failed: %v", err)
	}

	// Build repair result — type mismatches go to unfixable
	result := &RepoRepairResult{
		TypeMismatches: verifyResult.TypeMismatches,
	}
	result.UnfixableCount = len(result.TypeMismatches)

	if len(result.TypeMismatches) == 0 {
		t.Error("Expected type mismatch to be reported as unfixable")
	}
	if result.UnfixableCount != 1 {
		t.Errorf("UnfixableCount = %d, want 1", result.UnfixableCount)
	}
	if result.FixedCount != 0 {
		t.Errorf("FixedCount = %d, want 0 (type mismatches are not auto-fixed)", result.FixedCount)
	}
}

// -----------------------------------------------------------------------
// TestRepoRepair_MissingPackageRefs — reported as unfixable
// -----------------------------------------------------------------------

func TestRepoRepair_MissingPackageRefs(t *testing.T) {
	manager, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	repoPath := manager.GetRepoPath()

	// Create a package that references non-existent resources
	pkgContent := `{
  "name": "test-pkg",
  "description": "Test package",
  "resources": ["command/ghost-cmd", "skill/ghost-skill"]
}`
	pkgPath := filepath.Join(repoPath, "packages", "test-pkg.package.json")
	if err := os.WriteFile(pkgPath, []byte(pkgContent), 0644); err != nil {
		t.Fatalf("Failed to create package: %v", err)
	}

	// Run diagnostic scan
	verifyResult, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository failed: %v", err)
	}

	// Build repair result — package issues are unfixable
	result := &RepoRepairResult{
		PackagesWithMissingRefs: verifyResult.PackagesWithMissingRefs,
	}
	result.UnfixableCount = len(result.PackagesWithMissingRefs)

	if len(result.PackagesWithMissingRefs) == 0 {
		t.Error("Expected package missing refs to be reported as unfixable")
	}
	if result.UnfixableCount == 0 {
		t.Error("UnfixableCount should be > 0")
	}
	if result.FixedCount != 0 {
		t.Errorf("FixedCount = %d, want 0 (missing package refs are not auto-fixed)", result.FixedCount)
	}
}

// -----------------------------------------------------------------------
// TestRepoRepair_DryRun — no filesystem changes, only output
// -----------------------------------------------------------------------

func TestRepoRepair_DryRun(t *testing.T) {
	manager, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	repoPath := manager.GetRepoPath()

	// Create a resource without metadata
	createTestCommand(t, repoPath, "dry-run-cmd")

	// Create orphaned metadata (no corresponding resource file)
	createTestMetadata(t, repoPath, "orphan-dry")
	orphanPath := metadata.GetMetadataPath("orphan-dry", resource.Command, repoPath)

	// Run diagnostic scan
	verifyResult, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository failed: %v", err)
	}

	// Simulate dry-run: populate "would fix" lists without touching filesystem
	result := &RepoRepairResult{
		DryRun:                  true,
		MetadataCreated:         verifyResult.ResourcesWithoutMetadata,
		OrphanedRemoved:         verifyResult.OrphanedMetadata,
		TypeMismatches:          verifyResult.TypeMismatches,
		PackagesWithMissingRefs: verifyResult.PackagesWithMissingRefs,
	}
	result.FixedCount = len(result.MetadataCreated) + len(result.OrphanedRemoved)
	result.UnfixableCount = len(result.TypeMismatches) + len(result.PackagesWithMissingRefs)

	// Dry-run should report what it would do
	if result.FixedCount == 0 {
		t.Error("Dry-run should report planned actions (FixedCount > 0)")
	}
	if len(result.MetadataCreated) == 0 {
		t.Error("Dry-run should report would-create-metadata entries")
	}
	if len(result.OrphanedRemoved) == 0 {
		t.Error("Dry-run should report would-remove-orphan entries")
	}

	// Verify NO filesystem changes were made
	// The command without metadata should still lack metadata
	_, err = manager.GetMetadata("dry-run-cmd", resource.Command)
	if err == nil {
		t.Error("Dry-run should NOT create metadata — but it did")
	}

	// The orphaned metadata file should still exist
	if _, err := os.Stat(orphanPath); os.IsNotExist(err) {
		t.Error("Dry-run should NOT remove orphaned metadata — but it did")
	}
}

// -----------------------------------------------------------------------
// TestRepoRepair_MixedIssues — some fixable, some not
// -----------------------------------------------------------------------

func TestRepoRepair_MixedIssues(t *testing.T) {
	manager, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	repoPath := manager.GetRepoPath()

	// 1. Resource without metadata (fixable)
	createTestCommand(t, repoPath, "missing-meta-cmd")

	// 2. Orphaned metadata (fixable)
	createTestMetadata(t, repoPath, "orphan-mixed")

	// 3. Type mismatch (unfixable): command file with metadata saying "agent"
	createTestCommand(t, repoPath, "type-mismatch-cmd")
	// Save at .metadata/commands/type-mismatch-cmd-metadata.json but with Type=agent inside
	createTypeMismatchMetadata(t, repoPath, "type-mismatch-cmd", resource.Command, resource.Agent)

	// Run diagnostic scan
	verifyResult, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository failed: %v", err)
	}

	// Build repair result
	result := &RepoRepairResult{
		TypeMismatches:          verifyResult.TypeMismatches,
		PackagesWithMissingRefs: verifyResult.PackagesWithMissingRefs,
	}
	result.UnfixableCount = len(result.TypeMismatches) + len(result.PackagesWithMissingRefs)

	// Apply fixable issues
	for _, issue := range verifyResult.ResourcesWithoutMetadata {
		res := resource.Resource{Name: issue.Name, Type: issue.Type}
		if err := createMetadataForResource(manager, res); err != nil {
			t.Fatalf("createMetadataForResource failed: %v", err)
		}
		result.MetadataCreated = append(result.MetadataCreated, issue)
	}
	for _, issue := range verifyResult.OrphanedMetadata {
		if err := os.Remove(issue.Path); err != nil {
			t.Fatalf("os.Remove failed: %v", err)
		}
		result.OrphanedRemoved = append(result.OrphanedRemoved, issue)
	}
	result.FixedCount = len(result.MetadataCreated) + len(result.OrphanedRemoved)

	// Assertions
	if result.FixedCount < 1 {
		t.Errorf("FixedCount = %d, want >= 1 (should have fixed missing metadata and orphan)", result.FixedCount)
	}
	if result.UnfixableCount < 1 {
		t.Errorf("UnfixableCount = %d, want >= 1 (type mismatch is unfixable)", result.UnfixableCount)
	}
	if len(result.TypeMismatches) == 0 {
		t.Error("Expected type mismatch in unfixable list")
	}

	// Verify the fixable items were actually fixed
	_, err = manager.GetMetadata("missing-meta-cmd", resource.Command)
	if err != nil {
		t.Errorf("Metadata was not created for missing-meta-cmd: %v", err)
	}

	// The orphan metadata should be gone
	orphanPath := metadata.GetMetadataPath("orphan-mixed", resource.Command, repoPath)
	if _, statErr := os.Stat(orphanPath); !os.IsNotExist(statErr) {
		t.Error("Orphaned metadata should have been removed")
	}
}

// -----------------------------------------------------------------------
// TestRepoRepair_OutputJSON — JSON output format round-trip
// -----------------------------------------------------------------------

func TestRepoRepair_OutputJSON(t *testing.T) {
	result := &RepoRepairResult{
		Status: repoRepairStatusClean,
		MetadataCreated: []ResourceIssue{
			{Name: "test-cmd", Type: resource.Command, Path: "/repo/commands/test-cmd.md"},
		},
		OrphanedRemoved:         []MetadataIssue{},
		TypeMismatches:          []TypeMismatch{},
		PackagesWithMissingRefs: []PackageIssue{},
		DryRun:                  false,
		FixedCount:              1,
		UnfixableCount:          0,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	// Unmarshal back and check fields
	var decoded RepoRepairResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if decoded.FixedCount != 1 {
		t.Errorf("FixedCount = %d, want 1", decoded.FixedCount)
	}
	if len(decoded.MetadataCreated) != 1 {
		t.Errorf("MetadataCreated len = %d, want 1", len(decoded.MetadataCreated))
	}
	if decoded.MetadataCreated[0].Name != "test-cmd" {
		t.Errorf("MetadataCreated[0].Name = %q, want %q", decoded.MetadataCreated[0].Name, "test-cmd")
	}
}

func TestSetRepoRepairStatusForCompletedResult(t *testing.T) {
	t.Run("clean result", func(t *testing.T) {
		result := &RepoRepairResult{UnfixableCount: 0}
		setRepoRepairStatusForCompletedResult(result)
		if result.Status != repoRepairStatusClean {
			t.Fatalf("status=%q want %q", result.Status, repoRepairStatusClean)
		}
	})

	t.Run("unfixable findings", func(t *testing.T) {
		result := &RepoRepairResult{UnfixableCount: 2}
		setRepoRepairStatusForCompletedResult(result)
		if result.Status != repoRepairStatusCompletedWithFindings {
			t.Fatalf("status=%q want %q", result.Status, repoRepairStatusCompletedWithFindings)
		}
	})
}

func TestOutputRepoRepairOperationalFailure(t *testing.T) {
	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("pipe: %v", pipeErr)
	}
	os.Stdout = w

	err := outputRepoRepairOperationalFailure(output.JSON, os.ErrNotExist)
	_ = w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("outputRepoRepairOperationalFailure returned error: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	raw := string(buf[:n])

	var parsed RepoRepairResult
	if unmarshalErr := json.Unmarshal([]byte(raw), &parsed); unmarshalErr != nil {
		t.Fatalf("unmarshal output: %v; output=%s", unmarshalErr, raw)
	}

	if parsed.Status != repoRepairStatusExecutionFailed {
		t.Fatalf("status=%q want %q", parsed.Status, repoRepairStatusExecutionFailed)
	}
	if parsed.Error == nil {
		t.Fatalf("expected error payload")
	}
	if parsed.Error.Category != string(commandErrorCategoryInternal) && parsed.Error.Category != string(commandErrorCategoryIOError) {
		t.Fatalf("unexpected category=%q", parsed.Error.Category)
	}
}

func TestRepoRepairLockContentionReturnsTypedExitError(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("init repo: %v", err)
	}
	lock, err := manager.AcquireRepoWriteLock(context.Background())
	if err != nil {
		t.Fatalf("acquire setup lock: %v", err)
	}
	defer func() { _ = lock.Unlock() }()

	oldFormat := repoRepairFormatFlag
	repoRepairFormatFlag = "json"
	defer func() {
		repoRepairFormatFlag = oldFormat
	}()

	err = repoRepairCmd.RunE(repoRepairCmd, nil)
	if err == nil {
		t.Fatalf("expected error")
	}

	var cmdErr *commandExitError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected commandExitError, got %T", err)
	}
	if cmdErr.ExitCode != commandExitCodeOperationalFailure {
		t.Fatalf("exit code=%d want %d", cmdErr.ExitCode, commandExitCodeOperationalFailure)
	}
	if cmdErr.Category != commandErrorCategoryRepoBusy {
		t.Fatalf("category=%q want %q", cmdErr.Category, commandErrorCategoryRepoBusy)
	}
}

func TestRepoRepairUnfixableReturnsCompletedWithFindingsExit(t *testing.T) {
	manager, cleanup := setupRepairTestRepo(t)
	defer cleanup()

	repoPath := manager.GetRepoPath()
	createTestCommand(t, repoPath, "mismatch-exit")
	createTypeMismatchMetadata(t, repoPath, "mismatch-exit", resource.Command, resource.Skill)

	oldFormat := repoRepairFormatFlag
	repoRepairFormatFlag = "json"
	defer func() { repoRepairFormatFlag = oldFormat }()

	err := repoRepairCmd.RunE(repoRepairCmd, nil)
	if err == nil {
		t.Fatalf("expected completed-with-findings error")
	}

	var cmdErr *commandExitError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected commandExitError, got %T", err)
	}
	if cmdErr.ExitCode != commandExitCodeCompletedWithFindings {
		t.Fatalf("exit code=%d want %d", cmdErr.ExitCode, commandExitCodeCompletedWithFindings)
	}
}
