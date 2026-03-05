package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// TestPackageAutoImportFromLocalDir tests importing packages from a local directory structure
func TestPackageAutoImportFromLocalDir(t *testing.T) {
	// Create temporary directories
	repoDir := t.TempDir()
	sourceDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create a directory structure with packages
	packagesDir := filepath.Join(sourceDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	// Create test resources using helper functions
	cmdPath := createTestCommandInDir(t, sourceDir, "test-cmd", "A test command for package import testing")
	skillDir := createTestSkillInDir(t, sourceDir, "test-skill", "A test skill for package import testing")

	// Create test package
	pkg1Content := `{
  "name": "test-package",
  "description": "A test package for import testing",
  "resources": [
    "command/test-cmd",
    "skill/test-skill"
  ]
}`
	pkg1Path := filepath.Join(packagesDir, "test-package.package.json")
	if err := os.WriteFile(pkg1Path, []byte(pkg1Content), 0644); err != nil {
		t.Fatalf("Failed to create package file: %v", err)
	}

	// Step 1: Discover packages
	t.Log("Step 1: Discovering packages")
	packages, err := discovery.DiscoverPackages(sourceDir, "")
	if err != nil {
		t.Fatalf("Failed to discover packages: %v", err)
	}

	if len(packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(packages))
	}

	if packages[0].Name != "test-package" {
		t.Errorf("Package name = %v, want test-package", packages[0].Name)
	}

	// Step 2: Import via AddBulk
	t.Log("Step 2: Importing resources and packages")
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Collect all paths to import
	allPaths := []string{cmdPath, skillDir, pkg1Path}

	opts := repo.BulkImportOptions{
		Force:        false,
		SkipExisting: false,
		DryRun:       false,
	}

	result, err := manager.AddBulk(allPaths, opts)
	if err != nil {
		t.Fatalf("Failed to import resources: %v", err)
	}

	// Verify import results
	if len(result.Added) != 3 {
		t.Errorf("Expected 3 resources added, got %d", len(result.Added))
	}
	if len(result.Failed) != 0 {
		t.Errorf("Expected 0 failures, got %d: %v", len(result.Failed), result.Failed)
	}
	if result.CommandCount != 1 {
		t.Errorf("Expected 1 command, got %d", result.CommandCount)
	}
	if result.SkillCount != 1 {
		t.Errorf("Expected 1 skill, got %d", result.SkillCount)
	}
	if result.PackageCount != 1 {
		t.Errorf("Expected 1 package, got %d", result.PackageCount)
	}

	// Step 3: Verify package was stored correctly
	t.Log("Step 3: Verifying package storage")
	pkgPath := resource.GetPackagePath("test-package", repoDir)
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		t.Errorf("Package file not created at %s", pkgPath)
	}

	// Load and verify package
	loadedPkg, err := resource.LoadPackage(pkgPath)
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}
	if loadedPkg.Name != "test-package" {
		t.Errorf("Package name = %v, want test-package", loadedPkg.Name)
	}
	if len(loadedPkg.Resources) != 2 {
		t.Errorf("Package has %d resources, want 2", len(loadedPkg.Resources))
	}

	// Step 4: Verify metadata was created
	t.Log("Step 4: Verifying metadata")
	metadataPath := filepath.Join(repoDir, ".metadata", "packages", "test-package-metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Errorf("Metadata file not created at %s", metadataPath)
	}

	// Step 5: Verify package appears in repo list
	t.Log("Step 5: Verifying package in repo list")
	resources, err := manager.List(nil)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	// Should have command, skill, and package (packages are included in List now)
	if len(resources) != 3 {
		t.Errorf("Expected 3 resources in repo (command, skill, package), got %d", len(resources))
	}

	// Verify packages separately
	pkgList, err := manager.ListPackages()
	if err != nil {
		t.Fatalf("Failed to list packages: %v", err)
	}

	if len(pkgList) != 1 {
		t.Errorf("Expected 1 package, got %d", len(pkgList))
	}

	if len(pkgList) > 0 && pkgList[0].Name != "test-package" {
		t.Errorf("Package name = %v, want test-package", pkgList[0].Name)
	}

	// Verify repository integrity after package import
	t.Log("Verifying repository integrity")
	AssertVerifyClean(t)
}

// TestPackageAutoImportWithFilter tests filtering packages during import
func TestPackageAutoImportWithFilter(t *testing.T) {
	repoDir := t.TempDir()
	sourceDir := t.TempDir()

	// Create packages directory with multiple packages
	packagesDir := filepath.Join(sourceDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	// Create package 1
	pkg1Content := `{
  "name": "web-tools",
  "description": "Web development tools",
  "resources": []
}`
	pkg1Path := filepath.Join(packagesDir, "web-tools.package.json")
	if err := os.WriteFile(pkg1Path, []byte(pkg1Content), 0644); err != nil {
		t.Fatalf("Failed to create package 1: %v", err)
	}

	// Create package 2
	pkg2Content := `{
  "name": "testing-suite",
  "description": "Testing tools",
  "resources": []
}`
	pkg2Path := filepath.Join(packagesDir, "testing-suite.package.json")
	if err := os.WriteFile(pkg2Path, []byte(pkg2Content), 0644); err != nil {
		t.Fatalf("Failed to create package 2: %v", err)
	}

	// Discover packages
	packages, err := discovery.DiscoverPackages(sourceDir, "")
	if err != nil {
		t.Fatalf("Failed to discover packages: %v", err)
	}

	if len(packages) != 2 {
		t.Errorf("Expected 2 packages, got %d", len(packages))
	}

	// Import only packages matching filter "package/web*"
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Simulate filter by only importing web-tools
	opts := repo.BulkImportOptions{
		Force:        false,
		SkipExisting: false,
		DryRun:       false,
	}

	result, err := manager.AddBulk([]string{pkg1Path}, opts)
	if err != nil {
		t.Fatalf("Failed to import package: %v", err)
	}

	if result.PackageCount != 1 {
		t.Errorf("Expected 1 package, got %d", result.PackageCount)
	}

	// Verify only web-tools was imported
	pkgList, err := manager.ListPackages()
	if err != nil {
		t.Fatalf("Failed to list packages: %v", err)
	}

	if len(pkgList) != 1 {
		t.Errorf("Expected 1 package, got %d", len(pkgList))
	}

	if len(pkgList) > 0 && pkgList[0].Name != "web-tools" {
		t.Errorf("Package name = %v, want web-tools", pkgList[0].Name)
	}
}

// TestPackageAutoImportMixedResources tests importing packages along with other resource types
func TestPackageAutoImportMixedResources(t *testing.T) {
	repoDir := t.TempDir()
	sourceDir := t.TempDir()

	// Create OpenCode-style directory structure
	opencodeDir := filepath.Join(sourceDir, ".opencode")

	// Create test resources using helper functions
	_ = createTestCommandInDir(t, opencodeDir, "cmd1", "Command 1")
	_ = createTestCommandInDir(t, opencodeDir, "cmd2", "Command 2")
	_ = createTestSkillInDir(t, opencodeDir, "skill1", "Skill 1")
	createTestAgentInDir(t, opencodeDir, "agent1", "Agent 1")

	// Create packages directory at root level (not in .opencode)
	packagesDir := filepath.Join(sourceDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	pkgContent := `{
  "name": "mixed-package",
  "description": "Package with mixed resources",
  "resources": [
    "command/cmd1",
    "skill/skill1",
    "agent/agent1"
  ]
}`
	pkgPath := filepath.Join(packagesDir, "mixed-package.package.json")
	if err := os.WriteFile(pkgPath, []byte(pkgContent), 0644); err != nil {
		t.Fatalf("Failed to create package: %v", err)
	}

	// Scan OpenCode folder
	contents, err := resource.ScanOpenCodeFolder(opencodeDir)
	if err != nil {
		t.Fatalf("Failed to scan OpenCode folder: %v", err)
	}

	// Discover packages
	packages, err := discovery.DiscoverPackages(sourceDir, "")
	if err != nil {
		t.Fatalf("Failed to discover packages: %v", err)
	}

	// Verify discovery results
	if len(contents.CommandPaths) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(contents.CommandPaths))
	}
	if len(contents.SkillPaths) != 1 {
		t.Errorf("Expected 1 skill, got %d", len(contents.SkillPaths))
	}
	if len(contents.AgentPaths) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(contents.AgentPaths))
	}
	if len(packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(packages))
	}

	// Import all resources
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	allPaths := append([]string{}, contents.CommandPaths...)
	allPaths = append(allPaths, contents.SkillPaths...)
	allPaths = append(allPaths, contents.AgentPaths...)
	allPaths = append(allPaths, pkgPath)

	opts := repo.BulkImportOptions{
		Force:        false,
		SkipExisting: false,
		DryRun:       false,
	}

	result, err := manager.AddBulk(allPaths, opts)
	if err != nil {
		t.Fatalf("Failed to import resources: %v", err)
	}

	// Verify results
	if len(result.Added) != 5 {
		t.Errorf("Expected 5 resources added, got %d", len(result.Added))
	}
	if result.CommandCount != 2 {
		t.Errorf("Expected 2 commands, got %d", result.CommandCount)
	}
	if result.SkillCount != 1 {
		t.Errorf("Expected 1 skill, got %d", result.SkillCount)
	}
	if result.AgentCount != 1 {
		t.Errorf("Expected 1 agent, got %d", result.AgentCount)
	}
	if result.PackageCount != 1 {
		t.Errorf("Expected 1 package, got %d", result.PackageCount)
	}

	// Verify all resources in repo (commands, skills, agents, packages)
	resources, err := manager.List(nil)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if len(resources) != 5 {
		t.Errorf("Expected 5 resources in repo (2 commands, 1 skill, 1 agent, 1 package), got %d", len(resources))
	}

	// Verify package exists (packages are listed separately)
	pkgList, err := manager.ListPackages()
	if err != nil {
		t.Fatalf("Failed to list packages: %v", err)
	}

	if len(pkgList) != 1 {
		t.Errorf("Expected 1 package, got %d", len(pkgList))
	}

	if len(pkgList) > 0 && pkgList[0].Name != "mixed-package" {
		t.Errorf("Package name = %v, want mixed-package", pkgList[0].Name)
	}
}

// TestPackageAutoImportConflicts tests conflict handling during package import
func TestPackageAutoImportConflicts(t *testing.T) {
	tests := []struct {
		name         string
		force        bool
		skipExisting bool
		wantAdded    int
		wantUpdated  int
		wantSkipped  int
		wantFailed   int
		secondImport bool // whether to test second import attempt
	}{
		{
			name:         "conflict without force fails",
			force:        false,
			skipExisting: false,
			wantAdded:    0,
			wantUpdated:  0,
			wantSkipped:  0,
			wantFailed:   1,
			secondImport: true,
		},
		{
			name:         "conflict with force succeeds",
			force:        true,
			skipExisting: false,
			wantAdded:    0,
			wantUpdated:  1,
			wantSkipped:  0,
			wantFailed:   0,
			secondImport: true,
		},
		{
			name:         "conflict with skip skips",
			force:        false,
			skipExisting: true,
			wantAdded:    0,
			wantUpdated:  0,
			wantSkipped:  1,
			wantFailed:   0,
			secondImport: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := t.TempDir()
			sourceDir := t.TempDir()

			// Create package
			packagesDir := filepath.Join(sourceDir, "packages")
			if err := os.MkdirAll(packagesDir, 0755); err != nil {
				t.Fatalf("Failed to create packages directory: %v", err)
			}

			pkgContent := `{
  "name": "conflict-package",
  "description": "Package for conflict testing",
  "resources": []
}`
			pkgPath := filepath.Join(packagesDir, "conflict-package.package.json")
			if err := os.WriteFile(pkgPath, []byte(pkgContent), 0644); err != nil {
				t.Fatalf("Failed to create package: %v", err)
			}

			// Initialize repo and add package first time
			manager := repo.NewManagerWithPath(repoDir)
			if err := manager.Init(); err != nil {
				t.Fatalf("Failed to initialize repo: %v", err)
			}

			opts := repo.BulkImportOptions{
				Force:        false,
				SkipExisting: false,
				DryRun:       false,
			}

			result, err := manager.AddBulk([]string{pkgPath}, opts)
			if err != nil {
				t.Fatalf("Failed to import package first time: %v", err)
			}

			if len(result.Added) != 1 {
				t.Errorf("First import: expected 1 added, got %d", len(result.Added))
			}

			if !tt.secondImport {
				return
			}

			// Try to import again with specified options
			opts.Force = tt.force
			opts.SkipExisting = tt.skipExisting

			result, _ = manager.AddBulk([]string{pkgPath}, opts)

			if len(result.Added) != tt.wantAdded {
				t.Errorf("Added = %d, want %d", len(result.Added), tt.wantAdded)
			}
			if len(result.Updated) != tt.wantUpdated {
				t.Errorf("Updated = %d, want %d", len(result.Updated), tt.wantUpdated)
			}
			if len(result.Skipped) != tt.wantSkipped {
				t.Errorf("Skipped = %d, want %d", len(result.Skipped), tt.wantSkipped)
			}
			if len(result.Failed) != tt.wantFailed {
				t.Errorf("Failed = %d, want %d", len(result.Failed), tt.wantFailed)
			}
		})
	}
}

// TestPackageAutoImportWithMissingResources tests package import when referenced resources don't exist
func TestPackageAutoImportWithMissingResources(t *testing.T) {
	repoDir := t.TempDir()
	sourceDir := t.TempDir()

	// Create package with references to non-existent resources
	packagesDir := filepath.Join(sourceDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	pkgContent := `{
  "name": "incomplete-package",
  "description": "Package with missing resources",
  "resources": [
    "command/missing-cmd",
    "skill/missing-skill",
    "agent/missing-agent"
  ]
}`
	pkgPath := filepath.Join(packagesDir, "incomplete-package.package.json")
	if err := os.WriteFile(pkgPath, []byte(pkgContent), 0644); err != nil {
		t.Fatalf("Failed to create package: %v", err)
	}

	// Import package (should succeed even though resources don't exist yet)
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	opts := repo.BulkImportOptions{
		Force:        false,
		SkipExisting: false,
		DryRun:       false,
	}

	result, err := manager.AddBulk([]string{pkgPath}, opts)
	if err != nil {
		t.Fatalf("Failed to import package: %v", err)
	}

	if result.PackageCount != 1 {
		t.Errorf("Expected 1 package, got %d", result.PackageCount)
	}

	// Verify package was stored
	pkgList, err := manager.ListPackages()
	if err != nil {
		t.Fatalf("Failed to list packages: %v", err)
	}

	if len(pkgList) != 1 {
		t.Errorf("Expected 1 package, got %d", len(pkgList))
	}

	// Load full package and verify resources list
	repoPath := resource.GetPackagePath("incomplete-package", repoDir)
	loadedPkg, err := resource.LoadPackage(repoPath)
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	if len(loadedPkg.Resources) != 3 {
		t.Errorf("Package resources = %d, want 3", len(loadedPkg.Resources))
	}

	// Package should exist even though its resources don't
	if loadedPkg.Name != "incomplete-package" {
		t.Errorf("Package name = %v, want incomplete-package", loadedPkg.Name)
	}
}

// TestPackageAutoImportCLI tests package import via CLI commands
func TestPackageAutoImportCLI(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create source directory with packages
	sourceDir := filepath.Join(testDir, "source")
	packagesDir := filepath.Join(sourceDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	// Create test command using helper function
	createTestCommandInDir(t, sourceDir, "cli-cmd", "CLI test command")

	// Create package
	pkgContent := `{
  "name": "cli-test-package",
  "description": "Package for CLI testing",
  "resources": ["command/cli-cmd"]
}`
	pkgPath := filepath.Join(packagesDir, "cli-test-package.package.json")
	if err := os.WriteFile(pkgPath, []byte(pkgContent), 0644); err != nil {
		t.Fatalf("Failed to create package: %v", err)
	}

	// Run aimgr repo add on source directory
	output, err := runAimgr(t, "repo", "add", "local:"+sourceDir)
	if err != nil {
		t.Fatalf("Failed to add resources: %v\nOutput: %s", err, output)
	}

	// Verify output mentions packages
	if !strings.Contains(output, "packages") || !strings.Contains(output, "1") {
		t.Errorf("Output should mention 1 package, got: %s", output)
	}

	// Verify package was added (using package/* pattern)
	listOutput, err := runAimgr(t, "repo", "list", "package/*")
	if err != nil {
		t.Fatalf("Failed to list packages: %v", err)
	}

	if !strings.Contains(listOutput, "cli-test-package") {
		t.Errorf("List should contain cli-test-package, got: %s", listOutput)
	}

	// Verify command was also added (using pattern)
	cmdListOutput, err := runAimgr(t, "repo", "list", "command/*")
	if err != nil {
		t.Fatalf("Failed to list commands: %v", err)
	}

	if !strings.Contains(cmdListOutput, "cli-cmd") {
		t.Errorf("List should contain cli-cmd, got: %s", cmdListOutput)
	}
}

// TestPackageAutoImportDryRun tests dry-run mode for package import
func TestPackageAutoImportDryRun(t *testing.T) {
	repoDir := t.TempDir()
	sourceDir := t.TempDir()

	// Create package
	packagesDir := filepath.Join(sourceDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	pkgContent := `{
  "name": "dryrun-package",
  "description": "Package for dry-run testing",
  "resources": []
}`
	pkgPath := filepath.Join(packagesDir, "dryrun-package.package.json")
	if err := os.WriteFile(pkgPath, []byte(pkgContent), 0644); err != nil {
		t.Fatalf("Failed to create package: %v", err)
	}

	// Initialize repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Import with dry-run
	opts := repo.BulkImportOptions{
		Force:        false,
		SkipExisting: false,
		DryRun:       true,
	}

	result, err := manager.AddBulk([]string{pkgPath}, opts)
	if err != nil {
		t.Fatalf("Failed dry-run import: %v", err)
	}

	// Verify result shows what would be added
	if len(result.Added) != 1 {
		t.Errorf("Dry-run should show 1 would be added, got %d", len(result.Added))
	}
	if result.PackageCount != 1 {
		t.Errorf("Dry-run should count packages, got PackageCount = %d (expected 1)", result.PackageCount)
	}

	// Verify nothing was actually added
	resources, err := manager.List(nil)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if len(resources) != 0 {
		t.Errorf("Dry-run should not add resources, got %d", len(resources))
	}

	// Verify package file doesn't exist
	pkgInRepo := resource.GetPackagePath("dryrun-package", repoDir)
	if _, err := os.Stat(pkgInRepo); !os.IsNotExist(err) {
		t.Error("Package file should not exist after dry-run")
	}
}

// TestPackageAutoImportUpdateWorkflow tests the update workflow with packages
func TestPackageAutoImportUpdateWorkflow(t *testing.T) {
	repoDir := t.TempDir()
	sourceDir := t.TempDir()

	// Create initial package version
	packagesDir := filepath.Join(sourceDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	pkgContent := `{
  "name": "update-package",
  "description": "Version 1",
  "resources": ["command/cmd1"]
}`
	pkgPath := filepath.Join(packagesDir, "update-package.package.json")
	if err := os.WriteFile(pkgPath, []byte(pkgContent), 0644); err != nil {
		t.Fatalf("Failed to create package: %v", err)
	}

	// Import initial version
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	opts := repo.BulkImportOptions{
		Force:        false,
		SkipExisting: false,
		DryRun:       false,
	}

	result, err := manager.AddBulk([]string{pkgPath}, opts)
	if err != nil {
		t.Fatalf("Failed to import package: %v", err)
	}

	if result.PackageCount != 1 {
		t.Errorf("Expected 1 package, got %d", result.PackageCount)
	}

	// Load initial package
	pkgInRepo := resource.GetPackagePath("update-package", repoDir)
	loadedPkg1, err := resource.LoadPackage(pkgInRepo)
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}
	if loadedPkg1.Description != "Version 1" {
		t.Errorf("Initial description = %v, want Version 1", loadedPkg1.Description)
	}

	// Update package content
	pkgContentV2 := `{
  "name": "update-package",
  "description": "Version 2 - Updated",
  "resources": ["command/cmd1", "command/cmd2"]
}`
	if err := os.WriteFile(pkgPath, []byte(pkgContentV2), 0644); err != nil {
		t.Fatalf("Failed to update package: %v", err)
	}

	// Import with force to update
	opts.Force = true
	result, err = manager.AddBulk([]string{pkgPath}, opts)
	if err != nil {
		t.Fatalf("Failed to update package: %v", err)
	}

	if len(result.Updated) != 1 {
		t.Errorf("Expected 1 updated (not added), got %d", len(result.Updated))
	}
	if len(result.Added) != 0 {
		t.Errorf("Expected 0 added (should be updated), got %d", len(result.Added))
	}

	// Verify package was updated
	loadedPkg2, err := resource.LoadPackage(pkgInRepo)
	if err != nil {
		t.Fatalf("Failed to load updated package: %v", err)
	}

	if loadedPkg2.Description != "Version 2 - Updated" {
		t.Errorf("Updated description = %v, want Version 2 - Updated", loadedPkg2.Description)
	}
	if len(loadedPkg2.Resources) != 2 {
		t.Errorf("Updated resources count = %d, want 2", len(loadedPkg2.Resources))
	}
}
