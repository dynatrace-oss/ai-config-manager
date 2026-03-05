package repo

import (
	"os"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// TestRemove_OrphanedMetadata verifies that Remove() cleans up orphaned metadata
// when the resource file is already gone but metadata persists.
// This is the core bug fix scenario: sync detects a resource to remove, but the
// file was already deleted — only metadata remains.
func TestRemove_OrphanedMetadata(t *testing.T) {
	repoDir := t.TempDir()
	m := NewManagerWithPath(repoDir)

	// Initialize repo structure (creates commands/, skills/, .metadata/, etc.)
	if err := m.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create orphaned metadata (metadata exists, but no resource file)
	meta := &metadata.ResourceMetadata{
		Name:       "orphan-cmd",
		Type:       resource.Command,
		SourceType: "github",
		SourceURL:  "https://github.com/owner/repo",
	}
	if err := metadata.Save(meta, repoDir, "test-source"); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Verify metadata file exists
	metadataPath := metadata.GetMetadataPath("orphan-cmd", resource.Command, repoDir)
	if _, err := os.Stat(metadataPath); err != nil {
		t.Fatalf("Metadata file was not created: %v", err)
	}

	// Verify resource file does NOT exist
	resourcePath := m.GetPath("orphan-cmd", resource.Command)
	if _, err := os.Lstat(resourcePath); err == nil {
		t.Fatalf("Resource file should not exist for this test")
	}

	// Remove should succeed (cleaning up orphaned metadata)
	err := m.Remove("orphan-cmd", resource.Command)
	if err != nil {
		t.Fatalf("Remove() should succeed for orphaned metadata, got error: %v", err)
	}

	// Verify metadata was cleaned up
	if _, err := os.Stat(metadataPath); err == nil {
		t.Error("Metadata file still exists after Remove()")
	}
}

// TestRemove_NeitherExists verifies that Remove() returns "not found" error
// when neither the resource file nor metadata exist.
func TestRemove_NeitherExists(t *testing.T) {
	repoDir := t.TempDir()
	m := NewManagerWithPath(repoDir)

	// Initialize repo structure
	if err := m.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Remove a resource that doesn't exist at all
	err := m.Remove("nonexistent", resource.Command)
	if err == nil {
		t.Fatal("Remove() should return error when neither file nor metadata exist")
	}

	// Verify error message
	expected := "resource 'nonexistent' not found"
	if err.Error() != expected {
		t.Errorf("Remove() error = %q, want %q", err.Error(), expected)
	}
}

// TestRemove_Normal verifies that Remove() cleans up both the resource file
// and its metadata when both exist.
func TestRemove_Normal(t *testing.T) {
	repoDir := t.TempDir()
	m := NewManagerWithPath(repoDir)

	// Initialize repo structure
	if err := m.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Add a command properly (creates both file and metadata)
	testCmd := m.GetPath("normal-cmd", resource.Command)
	// commands dir already created by Init, but ensure it exists
	_ = os.MkdirAll(m.GetPath("", resource.Command), 0755)
	cmdContent := "---\ndescription: A normal command\n---\n\n# Normal Command\n"
	if err := os.WriteFile(testCmd, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command file: %v", err)
	}

	// Save metadata
	meta := &metadata.ResourceMetadata{
		Name:       "normal-cmd",
		Type:       resource.Command,
		SourceType: "file",
		SourceURL:  "file:///test/normal-cmd.md",
	}
	if err := metadata.Save(meta, repoDir, "test-source"); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Verify both exist
	resourcePath := m.GetPath("normal-cmd", resource.Command)
	if _, err := os.Lstat(resourcePath); err != nil {
		t.Fatalf("Resource file should exist: %v", err)
	}
	metadataPath := metadata.GetMetadataPath("normal-cmd", resource.Command, repoDir)
	if _, err := os.Stat(metadataPath); err != nil {
		t.Fatalf("Metadata file should exist: %v", err)
	}

	// Remove should succeed
	err := m.Remove("normal-cmd", resource.Command)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Verify both are gone
	if _, err := os.Lstat(resourcePath); err == nil {
		t.Error("Resource file still exists after Remove()")
	}
	if _, err := os.Stat(metadataPath); err == nil {
		t.Error("Metadata file still exists after Remove()")
	}
}

// TestRemove_OrphanedMetadataSkill verifies the fix works for skill type resources too.
func TestRemove_OrphanedMetadataSkill(t *testing.T) {
	repoDir := t.TempDir()
	m := NewManagerWithPath(repoDir)

	if err := m.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create orphaned metadata for a skill
	meta := &metadata.ResourceMetadata{
		Name:       "orphan-skill",
		Type:       resource.Skill,
		SourceType: "github",
		SourceURL:  "https://github.com/owner/repo",
	}
	if err := metadata.Save(meta, repoDir, "test-source"); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Verify metadata exists but resource doesn't
	metadataPath := metadata.GetMetadataPath("orphan-skill", resource.Skill, repoDir)
	if _, err := os.Stat(metadataPath); err != nil {
		t.Fatalf("Metadata file was not created: %v", err)
	}
	resourcePath := m.GetPath("orphan-skill", resource.Skill)
	if _, err := os.Lstat(resourcePath); err == nil {
		t.Fatalf("Resource should not exist for this test")
	}

	// Remove should succeed
	err := m.Remove("orphan-skill", resource.Skill)
	if err != nil {
		t.Fatalf("Remove() should succeed for orphaned skill metadata, got error: %v", err)
	}

	// Verify metadata cleaned up
	if _, err := os.Stat(metadataPath); err == nil {
		t.Error("Metadata file still exists after Remove()")
	}
}
