package repo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestAddPackage(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create test resources first
	cmdPath := filepath.Join(tmpDir, "test-cmd.md")
	cmdContent := `---
description: A test command
---

# Test Command
`
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}
	if err := manager.AddCommand(cmdPath, "file://"+cmdPath, "file"); err != nil {
		t.Fatalf("AddCommand() error = %v", err)
	}

	// Create a test package
	testPkg := filepath.Join(tmpDir, "test-pkg.package.json")
	pkgContent := map[string]interface{}{
		"name":        "test-pkg",
		"description": "A test package",
		"resources":   []string{"command/test-cmd"},
	}
	pkgData, _ := json.MarshalIndent(pkgContent, "", "  ")
	if err := os.WriteFile(testPkg, pkgData, 0644); err != nil {
		t.Fatalf("Failed to create test package: %v", err)
	}

	// Add the package
	err := manager.AddPackage(testPkg, "file://"+testPkg, "file")
	if err != nil {
		t.Fatalf("AddPackage() error = %v", err)
	}

	// Verify package was added
	destPath := resource.GetPackagePath("test-pkg", manager.GetRepoPath())
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("Package was not added to repository: %v", err)
	}

	// Verify we can load it
	pkg, err := resource.LoadPackage(destPath)
	if err != nil {
		t.Errorf("Failed to load added package: %v", err)
	}
	if pkg.Name != "test-pkg" {
		t.Errorf("Package name = %v, want test-pkg", pkg.Name)
	}
	if len(pkg.Resources) != 1 {
		t.Errorf("Package has %d resources, want 1", len(pkg.Resources))
	}
}

func TestAddPackageWithMissingResources(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create a test package with missing resources
	testPkg := filepath.Join(tmpDir, "test-pkg.package.json")
	pkgContent := map[string]interface{}{
		"name":        "test-pkg",
		"description": "A test package with missing resources",
		"resources":   []string{"command/missing-cmd", "skill/missing-skill"},
	}
	pkgData, _ := json.MarshalIndent(pkgContent, "", "  ")
	if err := os.WriteFile(testPkg, pkgData, 0644); err != nil {
		t.Fatalf("Failed to create test package: %v", err)
	}

	// Add the package - should succeed but warn about missing resources
	err := manager.AddPackage(testPkg, "file://"+testPkg, "file")
	if err != nil {
		t.Fatalf("AddPackage() error = %v", err)
	}

	// Verify package was added despite missing resources
	destPath := resource.GetPackagePath("test-pkg", manager.GetRepoPath())
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("Package was not added to repository: %v", err)
	}
}

func TestAddPackageConflict(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create a test package
	testPkg := filepath.Join(tmpDir, "test-pkg.package.json")
	pkgContent := map[string]interface{}{
		"name":        "test-pkg",
		"description": "A test package",
		"resources":   []string{},
	}
	pkgData, _ := json.MarshalIndent(pkgContent, "", "  ")
	if err := os.WriteFile(testPkg, pkgData, 0644); err != nil {
		t.Fatalf("Failed to create test package: %v", err)
	}

	// Add the package first time - should succeed
	if err := manager.AddPackage(testPkg, "file://"+testPkg, "file"); err != nil {
		t.Fatalf("First AddPackage() error = %v", err)
	}

	// Add the same package again - should succeed (overwrites)
	// Note: AddPackage() doesn't check for duplicates - it always overwrites
	// Use AddBulk() with SkipExisting or without Force for duplicate detection
	err := manager.AddPackage(testPkg, "file://"+testPkg, "file")
	if err != nil {
		t.Errorf("AddPackage() should succeed (overwrites existing), got error = %v", err)
	}
}

func TestAddPackageCreatesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create a test package
	testPkg := filepath.Join(tmpDir, "test-pkg.package.json")
	pkgContent := map[string]interface{}{
		"name":        "test-pkg",
		"description": "A test package",
		"resources":   []string{"command/test-cmd", "skill/test-skill"},
	}
	pkgData, _ := json.MarshalIndent(pkgContent, "", "  ")
	if err := os.WriteFile(testPkg, pkgData, 0644); err != nil {
		t.Fatalf("Failed to create test package: %v", err)
	}

	// Add the package
	sourceURL := "gh:owner/repo/test-pkg.package.json"
	sourceType := "github"

	err := manager.AddPackage(testPkg, sourceURL, sourceType)
	if err != nil {
		t.Fatalf("AddPackage() error = %v", err)
	}

	// Verify metadata file exists
	metadataPath := metadata.GetPackageMetadataPath("test-pkg", manager.GetRepoPath())
	if _, err := os.Stat(metadataPath); err != nil {
		t.Errorf("Metadata file was not created: %v", err)
	}

	// Verify metadata content
	meta, err := metadata.LoadPackageMetadata("test-pkg", manager.GetRepoPath())
	if err != nil {
		t.Fatalf("LoadPackageMetadata() error = %v", err)
	}

	if meta.Name != "test-pkg" {
		t.Errorf("Metadata name = %v, want test-pkg", meta.Name)
	}
	if meta.SourceURL != sourceURL {
		t.Errorf("Metadata sourceURL = %v, want %v", meta.SourceURL, sourceURL)
	}
	if meta.SourceType != sourceType {
		t.Errorf("Metadata sourceType = %v, want %v", meta.SourceType, sourceType)
	}
	if meta.ResourceCount != 2 {
		t.Errorf("Metadata resourceCount = %v, want 2", meta.ResourceCount)
	}
}

func TestBulkImportPackages(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	manager := NewManagerWithPath(repoPath)

	// Create test packages
	pkg1Path := filepath.Join(tmpDir, "pkg1.package.json")
	pkg1Content := map[string]interface{}{
		"name":        "pkg1",
		"description": "Package 1",
		"resources":   []string{},
	}
	pkg1Data, _ := json.MarshalIndent(pkg1Content, "", "  ")
	os.WriteFile(pkg1Path, pkg1Data, 0644)

	pkg2Path := filepath.Join(tmpDir, "pkg2.package.json")
	pkg2Content := map[string]interface{}{
		"name":        "pkg2",
		"description": "Package 2",
		"resources":   []string{},
	}
	pkg2Data, _ := json.MarshalIndent(pkg2Content, "", "  ")
	os.WriteFile(pkg2Path, pkg2Data, 0644)

	// Bulk import
	sources := []string{pkg1Path, pkg2Path}
	result, err := manager.AddBulk(sources, BulkImportOptions{})
	if err != nil {
		t.Fatalf("AddBulk() error = %v", err)
	}

	if len(result.Added) != 2 {
		t.Errorf("AddBulk() added count = %v, want 2", len(result.Added))
	}
	if result.PackageCount != 2 {
		t.Errorf("AddBulk() packageCount = %v, want 2", result.PackageCount)
	}

	// Verify packages were added
	pkg1Dest := resource.GetPackagePath("pkg1", manager.GetRepoPath())
	if _, err := os.Stat(pkg1Dest); err != nil {
		t.Errorf("Package 1 was not added: %v", err)
	}

	pkg2Dest := resource.GetPackagePath("pkg2", manager.GetRepoPath())
	if _, err := os.Stat(pkg2Dest); err != nil {
		t.Errorf("Package 2 was not added: %v", err)
	}
}

func TestBulkImportMixedResourcesAndPackages(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	manager := NewManagerWithPath(repoPath)

	// Create a command in proper commands/ directory
	commandsDir := filepath.Join(tmpDir, "commands")
	os.MkdirAll(commandsDir, 0755)
	cmdPath := filepath.Join(commandsDir, "cmd.md")
	os.WriteFile(cmdPath, []byte("---\ndescription: Command\n---\n"), 0644)

	// Create a skill
	skillDir := filepath.Join(tmpDir, "skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: skill\ndescription: Skill\n---\n"), 0644)

	// Create an agent in proper agents/ directory
	agentsDir := filepath.Join(tmpDir, "agents")
	os.MkdirAll(agentsDir, 0755)
	agentPath := filepath.Join(agentsDir, "agent.md")
	os.WriteFile(agentPath, []byte("---\ndescription: Agent\ntype: test-agent\n---\n"), 0644)

	// Create a package
	pkgPath := filepath.Join(tmpDir, "pkg.package.json")
	pkgContent := map[string]interface{}{
		"name":        "pkg",
		"description": "Package",
		"resources":   []string{},
	}
	pkgData, _ := json.MarshalIndent(pkgContent, "", "  ")
	os.WriteFile(pkgPath, pkgData, 0644)

	// Bulk import all
	sources := []string{cmdPath, skillDir, agentPath, pkgPath}
	result, err := manager.AddBulk(sources, BulkImportOptions{})
	if err != nil {
		t.Fatalf("AddBulk() error = %v", err)
	}

	if len(result.Added) != 4 {
		t.Errorf("AddBulk() added count = %v, want 4", len(result.Added))
	}
	if result.CommandCount != 1 {
		t.Errorf("AddBulk() commandCount = %v, want 1", result.CommandCount)
	}
	if result.SkillCount != 1 {
		t.Errorf("AddBulk() skillCount = %v, want 1", result.SkillCount)
	}
	if result.AgentCount != 1 {
		t.Errorf("AddBulk() agentCount = %v, want 1", result.AgentCount)
	}
	if result.PackageCount != 1 {
		t.Errorf("AddBulk() packageCount = %v, want 1", result.PackageCount)
	}
}

func TestBulkImportPackageWithForce(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	manager := NewManagerWithPath(repoPath)

	// Create a package
	pkgPath := filepath.Join(tmpDir, "pkg.package.json")
	pkgContent := map[string]interface{}{
		"name":        "pkg",
		"description": "Original",
		"resources":   []string{},
	}
	pkgData, _ := json.MarshalIndent(pkgContent, "", "  ")
	os.WriteFile(pkgPath, pkgData, 0644)

	// Add first time
	_, err := manager.AddBulk([]string{pkgPath}, BulkImportOptions{})
	if err != nil {
		t.Fatalf("First AddBulk() error = %v", err)
	}

	// Update package content
	pkgContent["description"] = "Updated"
	pkgData, _ = json.MarshalIndent(pkgContent, "", "  ")
	os.WriteFile(pkgPath, pkgData, 0644)

	// Add with force
	result, err := manager.AddBulk([]string{pkgPath}, BulkImportOptions{Force: true})
	if err != nil {
		t.Fatalf("Force AddBulk() error = %v", err)
	}

	if len(result.Updated) != 1 {
		t.Errorf("Force AddBulk() updated count = %v, want 1", len(result.Updated))
	}

	// Verify package was updated
	destPath := resource.GetPackagePath("pkg", manager.GetRepoPath())
	pkg, err := resource.LoadPackage(destPath)
	if err != nil {
		t.Fatalf("LoadPackage() error = %v", err)
	}
	if pkg.Description != "Updated" {
		t.Errorf("Package description = %v, want Updated", pkg.Description)
	}
}

func TestBulkImportPackageWithSkipExisting(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	manager := NewManagerWithPath(repoPath)

	// Create a package
	pkgPath := filepath.Join(tmpDir, "pkg.package.json")
	pkgContent := map[string]interface{}{
		"name":        "pkg",
		"description": "Package",
		"resources":   []string{},
	}
	pkgData, _ := json.MarshalIndent(pkgContent, "", "  ")
	os.WriteFile(pkgPath, pkgData, 0644)

	// Add first time
	_, err := manager.AddBulk([]string{pkgPath}, BulkImportOptions{})
	if err != nil {
		t.Fatalf("First AddBulk() error = %v", err)
	}

	// Try to add again with SkipExisting
	result, err := manager.AddBulk([]string{pkgPath}, BulkImportOptions{SkipExisting: true})
	if err != nil {
		t.Fatalf("SkipExisting AddBulk() error = %v", err)
	}

	if len(result.Skipped) != 1 {
		t.Errorf("SkipExisting AddBulk() skipped count = %v, want 1", len(result.Skipped))
	}
	if len(result.Added) != 0 {
		t.Errorf("SkipExisting AddBulk() added count = %v, want 0", len(result.Added))
	}
}

func TestValidatePackageResources(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Add some resources
	cmdPath := filepath.Join(tmpDir, "existing-cmd.md")
	os.WriteFile(cmdPath, []byte("---\ndescription: Existing command\n---\n"), 0644)
	_ = manager.AddCommand(cmdPath, "file://"+cmdPath, "file")

	skillDir := filepath.Join(tmpDir, "existing-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: existing-skill\ndescription: Existing skill\n---\n"), 0644)
	_ = manager.AddSkill(skillDir, "file://"+skillDir, "file")

	// Test package with mix of existing and missing resources
	pkg := &resource.Package{
		Name:        "test-pkg",
		Description: "Test package",
		Resources: []string{
			"command/existing-cmd", // exists
			"skill/existing-skill", // exists
			"command/missing-cmd",  // missing
			"agent/missing-agent",  // missing
			"invalid-format",       // invalid format
		},
	}

	missing := manager.validatePackageResources(pkg)

	if len(missing) != 3 {
		t.Errorf("validatePackageResources() returned %d missing, want 3", len(missing))
	}

	// Check that missing resources are reported
	expectedMissing := map[string]bool{
		"command/missing-cmd": true,
		"agent/missing-agent": true,
		"invalid-format":      true,
	}

	for _, ref := range missing {
		if !expectedMissing[ref] {
			t.Errorf("Unexpected missing resource: %v", ref)
		}
	}
}

func TestValidatePackageResources_NestedCommandReferences(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	if err := os.MkdirAll(filepath.Join(tmpDir, "commands", "ops"), 0755); err != nil {
		t.Fatalf("mkdir nested commands: %v", err)
	}
	cmdPath := filepath.Join(tmpDir, "commands", "ops", "deploy.md")
	if err := os.WriteFile(cmdPath, []byte("---\ndescription: deploy\n---\n# Deploy\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}

	pkg := &resource.Package{
		Name:        "nested-pkg",
		Description: "Nested package",
		Resources: []string{
			"command/ops/deploy",
			"command/ops/missing",
		},
	}

	missing := manager.ValidatePackageResources(pkg)
	if len(missing) != 1 {
		t.Fatalf("ValidatePackageResources() missing=%v, want 1", missing)
	}
	if missing[0] != "command/ops/missing" {
		t.Fatalf("unexpected missing reference: %v", missing[0])
	}
}

func TestListPackages(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Create and add test packages
	pkg1Path := filepath.Join(tmpDir, "pkg1.package.json")
	pkg1Content := map[string]interface{}{
		"name":        "pkg1",
		"description": "Package 1",
		"resources":   []string{"command/cmd1"},
	}
	pkg1Data, _ := json.MarshalIndent(pkg1Content, "", "  ")
	os.WriteFile(pkg1Path, pkg1Data, 0644)
	_ = manager.AddPackage(pkg1Path, "file://"+pkg1Path, "file")

	pkg2Path := filepath.Join(tmpDir, "pkg2.package.json")
	pkg2Content := map[string]interface{}{
		"name":        "pkg2",
		"description": "Package 2",
		"resources":   []string{"command/cmd1", "skill/skill1"},
	}
	pkg2Data, _ := json.MarshalIndent(pkg2Content, "", "  ")
	os.WriteFile(pkg2Path, pkg2Data, 0644)
	_ = manager.AddPackage(pkg2Path, "file://"+pkg2Path, "file")

	// List packages
	packages, err := manager.ListPackages()
	if err != nil {
		t.Fatalf("ListPackages() error = %v", err)
	}

	if len(packages) != 2 {
		t.Errorf("ListPackages() returned %d packages, want 2", len(packages))
	}

	// Verify package info
	pkg1Found := false
	pkg2Found := false
	for _, pkg := range packages {
		if pkg.Name == "pkg1" {
			pkg1Found = true
			if pkg.Description != "Package 1" {
				t.Errorf("Package 1 description = %v, want 'Package 1'", pkg.Description)
			}
			if pkg.ResourceCount != 1 {
				t.Errorf("Package 1 resource count = %v, want 1", pkg.ResourceCount)
			}
		}
		if pkg.Name == "pkg2" {
			pkg2Found = true
			if pkg.Description != "Package 2" {
				t.Errorf("Package 2 description = %v, want 'Package 2'", pkg.Description)
			}
			if pkg.ResourceCount != 2 {
				t.Errorf("Package 2 resource count = %v, want 2", pkg.ResourceCount)
			}
		}
	}

	if !pkg1Found {
		t.Error("Package 1 not found in list")
	}
	if !pkg2Found {
		t.Error("Package 2 not found in list")
	}
}

func TestListPackages_SortsAlphabetically(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	createPackage := func(name, description string, resources []string) {
		t.Helper()
		path := filepath.Join(tmpDir, name+".package.json")
		content := map[string]interface{}{
			"name":        name,
			"description": description,
			"resources":   resources,
		}
		data, _ := json.MarshalIndent(content, "", "  ")
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("failed to create package %s: %v", name, err)
		}
		if err := manager.AddPackage(path, "file://"+path, "file"); err != nil {
			t.Fatalf("failed to add package %s: %v", name, err)
		}
	}

	createPackage("z-package", "Z package", []string{"command/cmd-z"})
	createPackage("a-package", "A package", []string{"command/cmd-a"})
	createPackage("m-package", "M package", []string{"command/cmd-m"})

	packages, err := manager.ListPackages()
	if err != nil {
		t.Fatalf("ListPackages() error = %v", err)
	}

	if len(packages) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(packages))
	}

	want := []string{"a-package", "m-package", "z-package"}
	for i := range want {
		if packages[i].Name != want[i] {
			t.Fatalf("expected package %d to be %q, got %q", i, want[i], packages[i].Name)
		}
	}
}

func TestListPackages_IncludesPackagesWithInvalidReferences(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	packagesDir := filepath.Join(tmpDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	pkgPath := filepath.Join(packagesDir, "invalid-ref-package.package.json")
	pkgContent := `{
  "name": "invalid-ref-package",
  "description": "Package with invalid refs",
  "resources": ["invalid-format", "unknown-type/resource-name"]
}`
	if err := os.WriteFile(pkgPath, []byte(pkgContent), 0644); err != nil {
		t.Fatalf("failed to write package: %v", err)
	}

	packages, err := manager.ListPackages()
	if err != nil {
		t.Fatalf("ListPackages() error = %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("ListPackages() returned %d packages, want 1", len(packages))
	}

	if packages[0].Name != "invalid-ref-package" {
		t.Fatalf("ListPackages()[0].Name = %q, want %q", packages[0].Name, "invalid-ref-package")
	}

	if packages[0].ResourceCount != 2 {
		t.Fatalf("ListPackages()[0].ResourceCount = %d, want 2", packages[0].ResourceCount)
	}
}
