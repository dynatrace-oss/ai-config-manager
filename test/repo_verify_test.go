package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/pkg/resource"
)

// TestCLIRepoVerifyBasic tests basic verify command execution
func TestCLIRepoVerifyBasic(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Run verify on empty repo
	output, _ := runAimgr(t, "repo", "verify")

	// Should have the header
	if !strings.Contains(output, "Repository Verification") {
		t.Errorf("Expected 'Repository Verification' header, got: %s", output)
	}
}

// TestCLIRepoVerifyPackageWithMissingRefs tests detection of packages with missing resource references
func TestCLIRepoVerifyPackageWithMissingRefs(t *testing.T) {
	repoDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create a valid command resource that we'll reference in the package
	validCmdPath := createTestCommand(t, "valid-cmd", "Valid command")

	_, err := runAimgr(t, "repo", "add", "--force", "local:"+validCmdPath)
	if err != nil {
		t.Fatalf("Failed to add valid command: %v", err)
	}

	// Create a package directly in the repository (not via add command)
	// Packages need to be in repo/packages/ directory
	packagesDir := filepath.Join(repoDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	packagePath := filepath.Join(packagesDir, "test-package.package.json")
	packageContent := `{
  "name": "test-package",
  "description": "Package with missing refs",
  "resources": [
    "command/valid-cmd",
    "command/missing-cmd",
    "skill/missing-skill",
    "agent/missing-agent"
  ]
}`
	if err := os.WriteFile(packagePath, []byte(packageContent), 0644); err != nil {
		t.Fatalf("Failed to create package: %v", err)
	}

	// Test: verify should detect package with missing resource references
	output, _ := runAimgr(t, "repo", "verify")

	if !strings.Contains(output, "Packages with missing resource references") {
		t.Errorf("Expected 'Packages with missing resource references' error, got: %s", output)
	}

	if !strings.Contains(output, "test-package") {
		t.Errorf("Expected 'test-package' in output, got: %s", output)
	}

	if !strings.Contains(output, "command/missing-cmd") {
		t.Errorf("Expected 'command/missing-cmd' in output, got: %s", output)
	}

	if !strings.Contains(output, "skill/missing-skill") {
		t.Errorf("Expected 'skill/missing-skill' in output, got: %s", output)
	}

	if !strings.Contains(output, "agent/missing-agent") {
		t.Errorf("Expected 'agent/missing-agent' in output, got: %s", output)
	}

	// Should be an error (exit code 1)
	if !strings.Contains(output, "ERRORS found") {
		t.Errorf("Packages with missing refs should be an error")
	}

	// Verify it shows the count of missing resources (in the MISSING COUNT column)
	if !strings.Contains(output, "│ 3             │") && !strings.Contains(output, "│ 3 │") {
		t.Errorf("Expected '3' in MISSING COUNT column in output, got: %s", output)
	}
}

// TestCLIRepoVerifyPackageJSON tests JSON output for package validation
func TestCLIRepoVerifyPackageJSON(t *testing.T) {
	repoDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create a package directly in the repository
	packagesDir := filepath.Join(repoDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	packagePath := filepath.Join(packagesDir, "json-package.package.json")
	packageContent := `{
  "name": "json-package",
  "description": "Package for JSON test",
  "resources": [
    "command/nonexistent-cmd",
    "skill/nonexistent-skill"
  ]
}`
	if err := os.WriteFile(packagePath, []byte(packageContent), 0644); err != nil {
		t.Fatalf("Failed to create package: %v", err)
	}

	// Test: verify --format=json
	output, _ := runAimgr(t, "repo", "verify", "--format=json")

	var result struct {
		PackagesWithMissingRefs []struct {
			Name             string   `json:"name"`
			Path             string   `json:"path"`
			MissingResources []string `json:"missing_resources"`
		} `json:"packages_with_missing_refs"`
		HasErrors   bool `json:"has_errors"`
		HasWarnings bool `json:"has_warnings"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nOutput: %s", err, output)
	}

	// Check that package issue was detected
	if len(result.PackagesWithMissingRefs) != 1 {
		t.Errorf("Expected 1 package with missing refs, got %d", len(result.PackagesWithMissingRefs))
	}

	if len(result.PackagesWithMissingRefs) > 0 {
		pkg := result.PackagesWithMissingRefs[0]
		if pkg.Name != "json-package" {
			t.Errorf("Expected package name 'json-package', got '%s'", pkg.Name)
		}

		if len(pkg.MissingResources) != 2 {
			t.Errorf("Expected 2 missing resources, got %d", len(pkg.MissingResources))
		}

		// Check that the missing resources are listed
		hasCmdRef := false
		hasSkillRef := false
		for _, ref := range pkg.MissingResources {
			if ref == "command/nonexistent-cmd" {
				hasCmdRef = true
			}
			if ref == "skill/nonexistent-skill" {
				hasSkillRef = true
			}
		}

		if !hasCmdRef {
			t.Errorf("Expected 'command/nonexistent-cmd' in missing resources")
		}
		if !hasSkillRef {
			t.Errorf("Expected 'skill/nonexistent-skill' in missing resources")
		}
	}

	// Should have errors
	if !result.HasErrors {
		t.Errorf("Expected has_errors=true for package with missing refs")
	}
}

// TestCLIRepoVerifyHealthyPackage tests verification of a package with all resources present
func TestCLIRepoVerifyHealthyPackage(t *testing.T) {
	repoDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create resources
	cmdPath := createTestCommand(t, "pkg-cmd", "Command for package")

	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	skillPath := createTestSkill(t, "pkg-skill", "Skill for package")

	_, err = runAimgr(t, "repo", "add", "--force", "local:"+skillPath)
	if err != nil {
		t.Fatalf("Failed to add skill: %v", err)
	}

	// Create a package directly in the repository that references these resources
	packagesDir := filepath.Join(repoDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	packagePath := filepath.Join(packagesDir, "healthy-package.package.json")
	packageContent := `{
  "name": "healthy-package",
  "description": "Package with all valid refs",
  "resources": [
    "command/pkg-cmd",
    "skill/pkg-skill"
  ]
}`
	if err := os.WriteFile(packagePath, []byte(packageContent), 0644); err != nil {
		t.Fatalf("Failed to create package: %v", err)
	}

	// Test: verify should find no issues
	output, _ := runAimgr(t, "repo", "verify")

	if !strings.Contains(output, "No issues found") {
		t.Errorf("Expected 'No issues found' for healthy package, got: %s", output)
	}

	if strings.Contains(output, "Packages with missing resource references") {
		t.Errorf("Should not report package issues for healthy package, got: %s", output)
	}

	// Verify in JSON format to check detailed results
	jsonOutput, _ := runAimgr(t, "repo", "verify", "--format=json")

	var result struct {
		PackagesWithMissingRefs []interface{} `json:"packages_with_missing_refs"`
		HasErrors               bool          `json:"has_errors"`
		HasWarnings             bool          `json:"has_warnings"`
	}

	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(result.PackagesWithMissingRefs) > 0 {
		t.Errorf("Healthy package should have no missing refs, got %d", len(result.PackagesWithMissingRefs))
	}

	if result.HasErrors {
		t.Errorf("Healthy package should have has_errors=false")
	}
}

// TestCLIRepoVerifyPackageInvalidRef tests detection of invalid resource reference format
func TestCLIRepoVerifyPackageInvalidRef(t *testing.T) {
	repoDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create a package directly in the repository with invalid resource reference format
	packagesDir := filepath.Join(repoDir, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("Failed to create packages directory: %v", err)
	}

	packagePath := filepath.Join(packagesDir, "invalid-ref-package.package.json")
	packageContent := `{
  "name": "invalid-ref-package",
  "description": "Package with invalid ref format",
  "resources": [
    "invalid-format",
    "unknown-type/resource-name"
  ]
}`
	if err := os.WriteFile(packagePath, []byte(packageContent), 0644); err != nil {
		t.Fatalf("Failed to create package: %v", err)
	}

	// Test: verify should detect invalid references as missing
	output, _ := runAimgr(t, "repo", "verify")

	if !strings.Contains(output, "Packages with missing resource references") {
		t.Errorf("Expected 'Packages with missing resource references' error, got: %s", output)
	}

	if !strings.Contains(output, "invalid-ref-package") {
		t.Errorf("Expected 'invalid-ref-package' in output, got: %s", output)
	}

	// Should report both invalid references as missing
	if !strings.Contains(output, "invalid-format") {
		t.Errorf("Expected 'invalid-format' in output, got: %s", output)
	}

	if !strings.Contains(output, "unknown-type/resource-name") {
		t.Errorf("Expected 'unknown-type/resource-name' in output, got: %s", output)
	}

	// Should be an error
	if !strings.Contains(output, "ERRORS found") {
		t.Errorf("Invalid package refs should be an error")
	}
}

// TestCLIRepoVerifyJSON tests JSON output format
func TestCLIRepoVerifyJSON(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	output, _ := runAimgr(t, "repo", "verify", "--format=json")

	// Parse JSON output
	var result struct {
		ResourcesWithoutMetadata []struct {
			Name string `json:"name"`
			Type string `json:"type"`
			Path string `json:"path"`
		} `json:"resources_without_metadata,omitempty"`
		OrphanedMetadata []struct {
			Name string `json:"name"`
			Type string `json:"type"`
			Path string `json:"path"`
		} `json:"orphaned_metadata,omitempty"`
		MissingSourcePaths []struct {
			Name       string `json:"name"`
			Type       string `json:"type"`
			Path       string `json:"path"`
			SourcePath string `json:"source_path"`
		} `json:"missing_source_paths,omitempty"`
		TypeMismatches []struct {
			Name         string `json:"name"`
			ResourceType string `json:"resource_type"`
			MetadataType string `json:"metadata_type"`
		} `json:"type_mismatches,omitempty"`
		HasErrors   bool `json:"has_errors"`
		HasWarnings bool `json:"has_warnings"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Just verify we got valid JSON with the right structure
	t.Logf("Verify JSON output parsed successfully")
	t.Logf("  Resources without metadata: %d", len(result.ResourcesWithoutMetadata))
	t.Logf("  Orphaned metadata: %d", len(result.OrphanedMetadata))
	t.Logf("  Missing source paths: %d", len(result.MissingSourcePaths))
	t.Logf("  Type mismatches: %d", len(result.TypeMismatches))
	t.Logf("  Has errors: %v", result.HasErrors)
	t.Logf("  Has warnings: %v", result.HasWarnings)
}

// TestCLIRepoVerifyHelp tests the help output
func TestCLIRepoVerifyHelp(t *testing.T) {
	output, err := runAimgr(t, "repo", "verify", "--help")
	if err != nil {
		t.Fatalf("Failed to get help: %v", err)
	}

	// Check for key help sections
	expectedContent := []string{
		"Check for consistency issues",
		"--fix",
		"--format",
		"Resources without metadata",
		"Orphaned metadata",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(output, expected) {
			t.Errorf("Help output missing '%s'\nOutput: %s", expected, output)
		}
	}
}

// TestCLIRepoVerifyResourcesWithoutMetadata tests detection of resources without metadata
func TestCLIRepoVerifyResourcesWithoutMetadata(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Manually create a resource without metadata
	manager := repo.NewManagerWithPath(repoDir)
	commandsDir := filepath.Join(repoDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	cmdPath := filepath.Join(commandsDir, "no-meta-cmd.md")
	cmdContent := `---
description: Command without metadata
---
# No Metadata Command
`
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create command file: %v", err)
	}

	// Test: verify should detect resource without metadata
	output, _ := runAimgr(t, "repo", "verify")

	if !strings.Contains(output, "Resources without metadata") {
		t.Errorf("Expected 'Resources without metadata' warning, got: %s", output)
	}

	if !strings.Contains(output, "no-meta-cmd") {
		t.Errorf("Expected 'no-meta-cmd' in output, got: %s", output)
	}

	// Verify it's a warning, not an error (exit code 0)
	if strings.Contains(output, "ERRORS found") {
		t.Errorf("Resources without metadata should be a warning, not an error")
	}

	// Verify metadata doesn't exist yet
	_, err := manager.GetMetadata("no-meta-cmd", resource.Command)
	if err == nil {
		t.Errorf("Metadata should not exist before --fix")
	}
}

// TestCLIRepoVerifyOrphanedMetadata tests detection of orphaned metadata
func TestCLIRepoVerifyOrphanedMetadata(t *testing.T) {
	repoDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create and add a resource
	cmdPath := createTestCommand(t, "orphan-test", "Will become orphaned")

	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Delete the resource file (but leave metadata)
	manager := repo.NewManagerWithPath(repoDir)
	resourcePath := manager.GetPath("orphan-test", resource.Command)
	if err := os.Remove(resourcePath); err != nil {
		t.Fatalf("Failed to delete resource: %v", err)
	}

	// Test: verify should detect orphaned metadata
	output, _ := runAimgr(t, "repo", "verify")

	if !strings.Contains(output, "Orphaned metadata") {
		t.Errorf("Expected 'Orphaned metadata' error, got: %s", output)
	}

	if !strings.Contains(output, "orphan-test") {
		t.Errorf("Expected 'orphan-test' in output, got: %s", output)
	}

	// Should be an error (exit code 1)
	if !strings.Contains(output, "ERRORS found") {
		t.Errorf("Orphaned metadata should be an error")
	}
}

// TestCLIRepoVerifyMissingSourcePaths tests detection of missing source paths
func TestCLIRepoVerifyMissingSourcePaths(t *testing.T) {
	repoDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create and add a resource with local source
	cmdPath := createTestCommand(t, "source-test", "Test missing source path")

	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Delete the source file (metadata and repo resource remain)
	if err := os.Remove(cmdPath); err != nil {
		t.Fatalf("Failed to delete source file: %v", err)
	}

	// Test: verify should detect missing source path
	output, _ := runAimgr(t, "repo", "verify")

	if !strings.Contains(output, "missing source paths") {
		t.Errorf("Expected 'missing source paths' warning, got: %s", output)
	}

	if !strings.Contains(output, "source-test") {
		t.Errorf("Expected 'source-test' in output, got: %s", output)
	}

	// Should be a warning, not an error
	if strings.Contains(output, "ERRORS found") {
		t.Errorf("Missing source paths should be a warning, not an error")
	}
}

// TestCLIRepoVerifyTypeMismatches tests detection of type mismatches
func TestCLIRepoVerifyTypeMismatches(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create a command resource AND its metadata with matching type
	commandsDir := filepath.Join(repoDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	cmdPath := filepath.Join(commandsDir, "type-mismatch.md")
	cmdContent := `---
description: Type mismatch test
---
# Mismatch Command
`
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Create metadata in the commands directory but with agent type
	// This creates a true type mismatch (resource says command, metadata says agent)
	meta := &metadata.ResourceMetadata{
		Name:       "type-mismatch",
		Type:       resource.Agent, // Wrong type - should be Command!
		SourceType: "local",
		SourceURL:  "file://" + cmdPath,
	}

	// Manually save to commands metadata directory (not agents)
	// This simulates corruption where metadata type doesn't match location
	metadataDir := filepath.Join(repoDir, ".metadata", "commands")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		t.Fatalf("Failed to create metadata directory: %v", err)
	}

	metadataPath := filepath.Join(metadataDir, "type-mismatch-metadata.json")
	metadataJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}

	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		t.Fatalf("Failed to write metadata file: %v", err)
	}

	// Test: verify should detect type mismatch
	output, _ := runAimgr(t, "repo", "verify")

	if !strings.Contains(output, "Type mismatches") {
		t.Errorf("Expected 'Type mismatches' error, got: %s", output)
	}

	if !strings.Contains(output, "type-mismatch") {
		t.Errorf("Expected 'type-mismatch' in output, got: %s", output)
	}

	if !strings.Contains(output, "command") || !strings.Contains(output, "agent") {
		t.Errorf("Expected both 'command' and 'agent' in output, got: %s", output)
	}

	// Should be an error (exit code 1)
	if !strings.Contains(output, "ERRORS found") {
		t.Errorf("Type mismatches should be an error")
	}
}

// TestCLIRepoVerifyFixFlag tests the --fix flag functionality
func TestCLIRepoVerifyFixFlag(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	manager := repo.NewManagerWithPath(repoDir)

	// Create resource without metadata
	commandsDir := filepath.Join(repoDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	cmdPath := filepath.Join(commandsDir, "fix-test.md")
	cmdContent := `---
description: Test --fix flag
---
# Fix Test
`
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Verify metadata doesn't exist
	_, err := manager.GetMetadata("fix-test", resource.Command)
	if err == nil {
		t.Fatalf("Metadata should not exist before --fix")
	}

	// Test: verify --fix should create missing metadata
	output, err := runAimgr(t, "repo", "verify", "--fix")
	if err != nil {
		t.Fatalf("repo verify --fix failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "fix-test") {
		t.Errorf("Expected 'fix-test' in output, got: %s", output)
	}

	if !strings.Contains(output, "Created metadata") {
		t.Errorf("Expected 'Created metadata' in output, got: %s", output)
	}

	// Verify metadata now exists
	meta, err := manager.GetMetadata("fix-test", resource.Command)
	if err != nil {
		t.Errorf("Metadata should exist after --fix, got error: %v", err)
		return
	}

	// Guard against nil pointer dereference
	if meta == nil {
		t.Fatal("Metadata is nil after --fix")
	}

	if meta.Name != "fix-test" {
		t.Errorf("Metadata name = %s, want fix-test", meta.Name)
	}

	if meta.Type != resource.Command {
		t.Errorf("Metadata type = %s, want command", meta.Type)
	}
}

// TestCLIRepoVerifyFixOrphanedMetadata tests --fix removes orphaned metadata
func TestCLIRepoVerifyFixOrphanedMetadata(t *testing.T) {
	repoDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create and add a resource
	cmdPath := createTestCommand(t, "fix-orphan", "Test --fix with orphaned metadata")

	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Delete the resource file
	manager := repo.NewManagerWithPath(repoDir)
	resourcePath := manager.GetPath("fix-orphan", resource.Command)
	if err := os.Remove(resourcePath); err != nil {
		t.Fatalf("Failed to delete resource: %v", err)
	}

	// Verify metadata exists before --fix
	_, err = manager.GetMetadata("fix-orphan", resource.Command)
	if err != nil {
		t.Fatalf("Metadata should exist before --fix, got error: %v", err)
	}

	// Test: verify --fix should remove orphaned metadata
	output, _ := runAimgr(t, "repo", "verify", "--fix")

	if !strings.Contains(output, "fix-orphan") {
		t.Errorf("Expected 'fix-orphan' in output, got: %s", output)
	}

	if !strings.Contains(output, "Removed metadata") {
		t.Errorf("Expected 'Removed metadata' in output, got: %s", output)
	}

	// Verify metadata is now removed
	_, err = manager.GetMetadata("fix-orphan", resource.Command)
	if err == nil {
		t.Errorf("Metadata should be removed after --fix")
	}
}

// TestCLIRepoVerifyJSONWithIssues tests JSON output with actual issues
func TestCLIRepoVerifyJSONWithIssues(t *testing.T) {
	repoDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	manager := repo.NewManagerWithPath(repoDir)

	// Create resource without metadata
	commandsDir := filepath.Join(repoDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	cmdPath := filepath.Join(commandsDir, "json-test.md")
	cmdContent := `---
description: Test JSON output
---
# JSON Test
`
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Create orphaned metadata
	orphanPath := createTestCommand(t, "orphan-json", "Test JSON output")

	_, err := runAimgr(t, "repo", "add", "--force", "local:"+orphanPath)
	if err != nil {
		t.Fatalf("Failed to add orphan command: %v", err)
	}

	// Delete the orphan resource
	orphanRepoPath := manager.GetPath("orphan-json", resource.Command)
	if err := os.Remove(orphanRepoPath); err != nil {
		t.Fatalf("Failed to delete orphan resource: %v", err)
	}

	// Test: verify --format=json
	output, _ := runAimgr(t, "repo", "verify", "--format=json")

	var result struct {
		ResourcesWithoutMetadata []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"resources_without_metadata"`
		OrphanedMetadata []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"orphaned_metadata"`
		HasErrors   bool `json:"has_errors"`
		HasWarnings bool `json:"has_warnings"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nOutput: %s", err, output)
	}

	// Check that issues were detected
	foundNoMeta := false
	for _, issue := range result.ResourcesWithoutMetadata {
		if issue.Name == "json-test" {
			foundNoMeta = true
			break
		}
	}
	if !foundNoMeta {
		t.Errorf("Expected json-test in resources_without_metadata")
	}

	foundOrphan := false
	for _, issue := range result.OrphanedMetadata {
		if issue.Name == "orphan-json" {
			foundOrphan = true
			break
		}
	}
	if !foundOrphan {
		t.Errorf("Expected orphan-json in orphaned_metadata")
	}

	// Should have errors (orphaned metadata)
	if !result.HasErrors {
		t.Errorf("Expected has_errors=true (orphaned metadata)")
	}

	// Should have warnings (resource without metadata)
	if !result.HasWarnings {
		t.Errorf("Expected has_warnings=true (resource without metadata)")
	}
}

// TestCLIRepoVerifyHealthyRepo tests verification of a healthy repository
func TestCLIRepoVerifyHealthyRepo(t *testing.T) {
	repoDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create and add a valid resource
	cmdPath := createTestCommand(t, "healthy-cmd", "A healthy command")

	_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
	if err != nil {
		t.Fatalf("Failed to add command: %v", err)
	}

	// Test: verify should find no issues
	output, _ := runAimgr(t, "repo", "verify")

	if !strings.Contains(output, "No issues found") {
		t.Errorf("Expected 'No issues found', got: %s", output)
	}

	if !strings.Contains(output, "healthy") {
		t.Errorf("Expected 'healthy' in output, got: %s", output)
	}

	// Verify JSON output also shows healthy
	jsonOutput, _ := runAimgr(t, "repo", "verify", "--format=json")

	var result struct {
		HasErrors   bool `json:"has_errors"`
		HasWarnings bool `json:"has_warnings"`
	}

	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result.HasErrors {
		t.Errorf("Healthy repo should have has_errors=false")
	}

	if result.HasWarnings {
		t.Errorf("Healthy repo should have has_warnings=false")
	}
}

// TestCLIRepoVerifySkillIssues tests verification with skill resources
func TestCLIRepoVerifySkillIssues(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create skill without metadata
	manager := repo.NewManagerWithPath(repoDir)
	skillsDir := filepath.Join(repoDir, "skills", "verify-skill")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills directory: %v", err)
	}

	skillPath := filepath.Join(skillsDir, "SKILL.md")
	skillContent := `---
name: verify-skill
description: Skill for verify testing
---
# Verify Skill
`
	if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}

	// Test: verify should detect skill without metadata
	output, _ := runAimgr(t, "repo", "verify")

	if !strings.Contains(output, "Resources without metadata") {
		t.Errorf("Expected 'Resources without metadata', got: %s", output)
	}

	if !strings.Contains(output, "verify-skill") {
		t.Errorf("Expected 'verify-skill' in output, got: %s", output)
	}

	// Test --fix creates metadata for skill
	output, _ = runAimgr(t, "repo", "verify", "--fix")

	if !strings.Contains(output, "Created metadata") {
		t.Errorf("Expected 'Created metadata', got: %s", output)
	}

	// Verify metadata was created
	_, err := manager.GetMetadata("verify-skill", resource.Skill)
	if err != nil {
		t.Errorf("Metadata should be created for skill, got error: %v", err)
	}
}

// TestCLIRepoVerifyAgentIssues tests verification with agent resources
func TestCLIRepoVerifyAgentIssues(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create agent without metadata
	manager := repo.NewManagerWithPath(repoDir)
	agentsDir := filepath.Join(repoDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents directory: %v", err)
	}

	agentPath := filepath.Join(agentsDir, "verify-agent.md")
	agentContent := `---
description: Agent for verify testing
---
# Verify Agent
`
	if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Test: verify should detect agent without metadata
	output, _ := runAimgr(t, "repo", "verify")

	if !strings.Contains(output, "Resources without metadata") {
		t.Errorf("Expected 'Resources without metadata', got: %s", output)
	}

	if !strings.Contains(output, "verify-agent") {
		t.Errorf("Expected 'verify-agent' in output, got: %s", output)
	}

	// Test --fix creates metadata for agent
	output, _ = runAimgr(t, "repo", "verify", "--fix")

	if !strings.Contains(output, "Created metadata") {
		t.Errorf("Expected 'Created metadata', got: %s", output)
	}

	// Verify metadata was created
	_, err := manager.GetMetadata("verify-agent", resource.Agent)
	if err != nil {
		t.Errorf("Metadata should be created for agent, got error: %v", err)
	}
}

// TestCLIRepoVerifyEmptyRepo tests verify on empty repository
func TestCLIRepoVerifyEmptyRepo(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create empty repo structure
	for _, dir := range []string{"commands", "skills", "agents"} {
		if err := os.MkdirAll(filepath.Join(repoDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create %s directory: %v", dir, err)
		}
	}

	// Test: verify on empty repo
	output, _ := runAimgr(t, "repo", "verify")

	if !strings.Contains(output, "No issues found") {
		t.Errorf("Expected 'No issues found' for empty repo, got: %s", output)
	}
}

// TestCLIRepoVerifyMultipleIssues tests verify with multiple types of issues
func TestCLIRepoVerifyMultipleIssues(t *testing.T) {
	repoDir := t.TempDir()

	t.Setenv("AIMGR_REPO_PATH", repoDir)

	manager := repo.NewManagerWithPath(repoDir)

	// Issue 1: Resource without metadata (command)
	commandsDir := filepath.Join(repoDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	noMetaPath := filepath.Join(commandsDir, "no-meta.md")
	if err := os.WriteFile(noMetaPath, []byte(`---
description: No metadata
---
# No Meta
`), 0644); err != nil {
		t.Fatalf("Failed to create no-meta command: %v", err)
	}

	// Issue 2: Orphaned metadata (skill)
	orphanPath := createTestSkill(t, "orphan-skill", "Orphaned skill")

	_, err := runAimgr(t, "repo", "add", "--force", "local:"+orphanPath)
	if err != nil {
		t.Fatalf("Failed to add orphan skill: %v", err)
	}

	// Delete the skill resource to make it orphaned
	skillRepoPath := manager.GetPath("orphan-skill", resource.Skill)
	if err := os.RemoveAll(skillRepoPath); err != nil {
		t.Fatalf("Failed to delete skill resource: %v", err)
	}

	// Test: verify should find both issues
	output, _ := runAimgr(t, "repo", "verify")

	// Should detect resource without metadata
	if !strings.Contains(output, "Resources without metadata") {
		t.Errorf("Expected 'Resources without metadata', got: %s", output)
	}
	if !strings.Contains(output, "no-meta") {
		t.Errorf("Expected 'no-meta' in output, got: %s", output)
	}

	// Should detect orphaned metadata
	if !strings.Contains(output, "Orphaned metadata") {
		t.Errorf("Expected 'Orphaned metadata', got: %s", output)
	}
	if !strings.Contains(output, "orphan-skill") {
		t.Errorf("Expected 'orphan-skill' in output, got: %s", output)
	}

	// Should have errors (orphaned metadata)
	if !strings.Contains(output, "ERRORS found") {
		t.Errorf("Expected 'ERRORS found', got: %s", output)
	}

	// Should suggest repair
	if !strings.Contains(output, "aimgr repo repair") {
		t.Errorf("Expected suggestion to use 'aimgr repo repair', got: %s", output)
	}
}

// TestCLIRepoVerifyNestedPaths tests that verify correctly handles nested command paths
// This is a regression test for bug where verify used metadata filename instead of content
// for lookups, causing false "orphaned metadata" reports for nested paths like "dt/cluster/overview"

// TestCLIRepoVerifyNestedPaths tests that verify correctly handles nested command paths
// This is a regression test for bug where verify used metadata filename instead of content
// for lookups, causing false "orphaned metadata" reports for nested paths like "dt/cluster/overview"
func TestCLIRepoVerifyNestedPaths(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create nested commands individually and import them
	// Commands must be in a commands/ directory per LoadCommand requirements
	tempDir := t.TempDir()
	commandsDir := filepath.Join(tempDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	// Create command files with nested paths (properly structured)
	nestedCommands := []struct {
		path string
		name string
	}{
		{"dt-cluster-overview.md", "dt/cluster/overview"},
		{"dt-critical-incident-global-health-check.md", "dt/critical-incident/global-health-check"},
		{"opencode-coder-doctor.md", "opencode-coder/doctor"},
	}

	var commandPaths []string
	for _, cmd := range nestedCommands {
		cmdPath := filepath.Join(commandsDir, cmd.path)
		cmdContent := fmt.Sprintf(`---
description: Test nested command %s
---
# %s
Test nested command content.
`, cmd.name, cmd.name)

		if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
			t.Fatalf("Failed to create command file %s: %v", cmdPath, err)
		}
		commandPaths = append(commandPaths, cmdPath)
	}

	// Import each command
	for _, cmdPath := range commandPaths {
		_, err := runAimgr(t, "repo", "add", "--force", "local:"+cmdPath)
		if err != nil {
			t.Fatalf("Failed to import command %s: %v", cmdPath, err)
		}
	}

	// Now run verify - should report NO orphaned metadata
	verifyOutput, _ := runAimgr(t, "repo", "verify")

	// Should NOT contain "Orphaned metadata" section
	if strings.Contains(verifyOutput, "Orphaned metadata") {
		t.Errorf("Verify incorrectly reports orphaned metadata for nested commands.\nOutput: %s", verifyOutput)
	}

	// Should NOT contain any of our nested command names as orphaned
	// These are the WRONG keys (from filename) that the bug would generate
	orphanedPatterns := []string{
		"dt-cluster-overview",
		"dt-critical-incident-global-health-check",
		"opencode-coder-doctor",
	}

	for _, pattern := range orphanedPatterns {
		if strings.Contains(verifyOutput, pattern) {
			t.Errorf("Verify incorrectly reports '%s' as orphaned (should use name from metadata content, not filename)", pattern)
		}
	}

	// Verify JSON output also shows healthy
	jsonOutput, _ := runAimgr(t, "repo", "verify", "--format=json")

	var result struct {
		OrphanedMetadata []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"orphaned_metadata"`
		HasErrors bool `json:"has_errors"`
	}

	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		t.Fatalf("Failed to parse verify JSON: %v\nOutput: %s", err, jsonOutput)
	}

	// Should have NO orphaned metadata
	if len(result.OrphanedMetadata) > 0 {
		t.Errorf("Expected no orphaned metadata, but found %d issues:", len(result.OrphanedMetadata))
		for _, issue := range result.OrphanedMetadata {
			t.Errorf("  - %s (%s)", issue.Name, issue.Type)
		}
	}

	// Should have no errors (clean verify)
	if result.HasErrors {
		t.Errorf("Expected has_errors=false for nested commands, got true.\nOutput: %s", verifyOutput)
	}
}
