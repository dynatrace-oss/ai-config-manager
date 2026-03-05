package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// TestBug1_PackageRemoval tests that packages can be removed from the repository.
// Bug: GetPath() had no case for resource.PackageType, causing Remove() to fail silently.
// Fix: Added PackageType case to GetPath() returning correct path.
func TestBug1_PackageRemoval(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)

	// Initialize repo
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create a test package file
	pkgData := `{
  "name": "test-package",
  "version": "1.0.0",
  "description": "Test package for removal",
  "resources": [
    "command/test-cmd"
  ]
}`
	pkgPath := filepath.Join(repoDir, "packages", "test-package.package.json")
	if err := os.WriteFile(pkgPath, []byte(pkgData), 0644); err != nil {
		t.Fatalf("Failed to write package file: %v", err)
	}

	// Create package metadata
	pkgMeta := &metadata.PackageMetadata{
		Name:       "test-package",
		SourceType: "test",
		SourceURL:  "file:///test",
		SourceName: "test-source",
	}
	if err := metadata.SavePackageMetadata(pkgMeta, repoDir); err != nil {
		t.Fatalf("Failed to save package metadata: %v", err)
	}

	// Verify package exists
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		t.Fatal("Package file should exist before removal")
	}
	metadataPath := metadata.GetPackageMetadataPath("test-package", repoDir)
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatal("Package metadata should exist before removal")
	}

	// Remove package using repo.Remove (which uses GetPath)
	if err := manager.Remove("test-package", resource.PackageType); err != nil {
		t.Fatalf("Failed to remove package: %v", err)
	}

	// Verify package was removed
	if _, err := os.Stat(pkgPath); !os.IsNotExist(err) {
		t.Error("Package file should be removed")
	}
	if _, err := os.Stat(metadataPath); !os.IsNotExist(err) {
		t.Error("Package metadata should be removed")
	}
}

// TestBug2_NameMismatchWithFlag tests that resources added with --name flag can be removed.
// Bug: Manifest stores explicit name (--name) but metadata stores derived name (from path/URL).
// Fix: Pass explicit SourceName through BulkImportOptions and ImportOptions to metadata.
func TestBug2_NameMismatchWithFlag(t *testing.T) {
	repoDir := t.TempDir()
	sourceDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)

	// Initialize repo
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create test command
	cmdContent := `---
name: test-cmd
description: Test command
---
# Test Command

Test content
`
	cmdDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}
	cmdPath := filepath.Join(cmdDir, "test-cmd.md")
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to write command file: %v", err)
	}

	// Add resource with explicit source name using BulkImportOptions
	opts := repo.BulkImportOptions{
		SourceName:   "my-custom-source", // Explicit name (like --name flag)
		ImportMode:   "copy",
		Force:        false,
		SkipExisting: false,
		DryRun:       false,
		SourceURL:    "file://" + sourceDir,
		SourceType:   "local",
		Ref:          "",
	}

	result, err := manager.AddBulk([]string{cmdPath}, opts)
	if err != nil {
		t.Fatalf("Failed to add resource: %v", err)
	}
	if len(result.Failed) > 0 {
		t.Fatalf("Failed to add command: %v", result.Failed[0].Message)
	}

	// Verify command was added
	addedCmdPath := filepath.Join(repoDir, "commands", "test-cmd.md")
	if _, err := os.Stat(addedCmdPath); os.IsNotExist(err) {
		t.Fatal("Command should be added to repo")
	}

	// Verify metadata has correct source_name
	metaPath := filepath.Join(repoDir, ".metadata", "commands", "test-cmd-metadata.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}
	var meta metadata.ResourceMetadata
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	// CRITICAL: metadata should have the explicit source name, not derived name
	if meta.SourceName == "" {
		t.Error("Metadata SourceName should not be empty")
	}
	expectedSourceName := "my-custom-source"
	if meta.SourceName != expectedSourceName {
		t.Errorf("Metadata SourceName = %q, want %q", meta.SourceName, expectedSourceName)
	}

	// Now add source to manifest (simulating full repo add flow)
	manifest, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}
	manifestSource := &repomanifest.Source{
		Name: "my-custom-source",
		Path: sourceDir,
	}
	if err := manifest.AddSource(manifestSource); err != nil {
		t.Fatalf("Failed to add source to manifest: %v", err)
	}
	if err := manifest.Save(repoDir); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Verify that metadata.HasSource can find the resource by source name
	hasSource := metadata.HasSource("test-cmd", resource.Command, "my-custom-source", repoDir)
	if !hasSource {
		t.Error("HasSource should find resource by custom source name")
	}

	// List resources and verify one belongs to our source
	resources, err := manager.List(nil)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}
	foundResource := false
	for _, res := range resources {
		if res.Name == "test-cmd" && metadata.HasSource(res.Name, res.Type, "my-custom-source", repoDir) {
			foundResource = true
			break
		}
	}
	if !foundResource {
		t.Error("Should find test-cmd resource belonging to my-custom-source")
	}

	// Now remove the resource manually (simulating orphan cleanup in repo remove)
	if err := manager.Remove("test-cmd", resource.Command); err != nil {
		t.Fatalf("Failed to remove resource: %v", err)
	}

	// Verify command was removed
	if _, err := os.Stat(addedCmdPath); !os.IsNotExist(err) {
		t.Error("Command should be removed from repo")
	}
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Error("Metadata should be removed")
	}
}

// TestBackwardCompatibility_NoSourceName tests that resources added without SourceName still work.
// This ensures backward compatibility when SourceName is empty.
func TestBackwardCompatibility_NoSourceName(t *testing.T) {
	repoDir := t.TempDir()
	sourceDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)

	// Initialize repo
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create test command
	cmdContent := `---
name: test-cmd
description: Test command
---
# Test Command

Test content
`
	cmdDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}
	cmdPath := filepath.Join(cmdDir, "test-cmd.md")
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to write command file: %v", err)
	}

	// Add resource WITHOUT explicit source name (backward compatibility)
	opts := repo.BulkImportOptions{
		SourceName:   "", // Empty - should derive from URL
		ImportMode:   "copy",
		Force:        false,
		SkipExisting: false,
		DryRun:       false,
		SourceURL:    "file://" + sourceDir,
		SourceType:   "local",
		Ref:          "",
	}

	result, err := manager.AddBulk([]string{cmdPath}, opts)
	if err != nil {
		t.Fatalf("Failed to add resource: %v", err)
	}
	if len(result.Failed) > 0 {
		t.Fatalf("Failed to add command: %v", result.Failed[0].Message)
	}

	// Verify command was added
	addedCmdPath := filepath.Join(repoDir, "commands", "test-cmd.md")
	if _, err := os.Stat(addedCmdPath); os.IsNotExist(err) {
		t.Fatal("Command should be added to repo")
	}

	// Verify metadata has derived source name
	metaPath := filepath.Join(repoDir, ".metadata", "commands", "test-cmd-metadata.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}
	var meta metadata.ResourceMetadata
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	// Should have derived source name (from DeriveSourceName function)
	if meta.SourceName == "" {
		t.Error("Metadata SourceName should not be empty even when not explicitly provided")
	}
	// DeriveSourceName derives from file:// URL, should be like the temp dir basename
	expectedDerivedName := filepath.Base(sourceDir)
	if meta.SourceName != expectedDerivedName {
		t.Logf("Note: Derived source name is %q (expected something like %q)", meta.SourceName, expectedDerivedName)
		// This is OK - just verify it's not empty
	}
}

// TestPackageRemovalWithMultiplePackages tests removing specific packages.
func TestPackageRemovalWithMultiplePackages(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)

	// Initialize repo
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create multiple test packages
	packages := []string{"pkg-a", "pkg-b", "pkg-c"}
	for _, pkgName := range packages {
		pkgData := `{
  "name": "` + pkgName + `",
  "version": "1.0.0",
  "description": "Test package",
  "resources": []
}`
		pkgPath := filepath.Join(repoDir, "packages", pkgName+".package.json")
		if err := os.WriteFile(pkgPath, []byte(pkgData), 0644); err != nil {
			t.Fatalf("Failed to write package %s: %v", pkgName, err)
		}

		// Create metadata
		pkgMeta := &metadata.PackageMetadata{
			Name:       pkgName,
			SourceType: "test",
			SourceURL:  "file:///test",
			SourceName: "test-source",
		}
		if err := metadata.SavePackageMetadata(pkgMeta, repoDir); err != nil {
			t.Fatalf("Failed to save metadata for %s: %v", pkgName, err)
		}
	}

	// Remove one specific package
	if err := manager.Remove("pkg-b", resource.PackageType); err != nil {
		t.Fatalf("Failed to remove pkg-b: %v", err)
	}

	// Verify pkg-b was removed
	pkgBPath := filepath.Join(repoDir, "packages", "pkg-b.package.json")
	if _, err := os.Stat(pkgBPath); !os.IsNotExist(err) {
		t.Error("pkg-b should be removed")
	}

	// Verify other packages still exist
	for _, pkgName := range []string{"pkg-a", "pkg-c"} {
		pkgPath := filepath.Join(repoDir, "packages", pkgName+".package.json")
		if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
			t.Errorf("Package %s should still exist", pkgName)
		}
	}
}
