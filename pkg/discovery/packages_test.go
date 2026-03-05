package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestDiscoverPackages(t *testing.T) {
	tests := []struct {
		name       string
		setupFiles func(dir string)
		wantCount  int
		wantErr    bool
		wantNames  []string
	}{
		{
			name: "valid packages in packages/ directory",
			setupFiles: func(dir string) {
				packagesDir := filepath.Join(dir, "packages")
				os.MkdirAll(packagesDir, 0755)

				// Create two valid packages
				pkg1 := resource.Package{
					Name:        "web-tools",
					Description: "Web development tools",
					Resources:   []string{"command/build", "skill/typescript"},
				}
				writePackage(t, packagesDir, "web-tools.package.json", pkg1)

				pkg2 := resource.Package{
					Name:        "testing-suite",
					Description: "Testing tools",
					Resources:   []string{"command/test", "agent/qa"},
				}
				writePackage(t, packagesDir, "testing-suite.package.json", pkg2)
			},
			wantCount: 2,
			wantErr:   false,
			wantNames: []string{"web-tools", "testing-suite"},
		},
		{
			name: "no packages directory",
			setupFiles: func(dir string) {
				// Don't create packages/ directory
			},
			wantCount: 0,
			wantErr:   false,
			wantNames: []string{},
		},
		{
			name: "empty packages directory",
			setupFiles: func(dir string) {
				packagesDir := filepath.Join(dir, "packages")
				os.MkdirAll(packagesDir, 0755)
				// Don't create any files
			},
			wantCount: 0,
			wantErr:   false,
			wantNames: []string{},
		},
		{
			name: "invalid JSON files are skipped",
			setupFiles: func(dir string) {
				packagesDir := filepath.Join(dir, "packages")
				os.MkdirAll(packagesDir, 0755)

				// Create invalid JSON file
				invalidPath := filepath.Join(packagesDir, "invalid.package.json")
				os.WriteFile(invalidPath, []byte("{ invalid json }"), 0644)

				// Create valid package
				pkg := resource.Package{
					Name:        "valid-pkg",
					Description: "Valid package",
					Resources:   []string{"command/test"},
				}
				writePackage(t, packagesDir, "valid-pkg.package.json", pkg)
			},
			wantCount: 1,
			wantErr:   false,
			wantNames: []string{"valid-pkg"},
		},
		{
			name: "missing required fields are skipped",
			setupFiles: func(dir string) {
				packagesDir := filepath.Join(dir, "packages")
				os.MkdirAll(packagesDir, 0755)

				// Package missing description
				missingDesc := filepath.Join(packagesDir, "no-desc.package.json")
				os.WriteFile(missingDesc, []byte(`{
					"name": "no-desc",
					"resources": ["command/test"]
				}`), 0644)

				// Package missing name
				missingName := filepath.Join(packagesDir, "no-name.package.json")
				os.WriteFile(missingName, []byte(`{
					"description": "Missing name",
					"resources": ["command/test"]
				}`), 0644)

				// Valid package
				pkg := resource.Package{
					Name:        "valid-pkg",
					Description: "Valid package",
					Resources:   []string{"command/test"},
				}
				writePackage(t, packagesDir, "valid-pkg.package.json", pkg)
			},
			wantCount: 1,
			wantErr:   false,
			wantNames: []string{"valid-pkg"},
		},
		{
			name: "multiple packages with deduplication",
			setupFiles: func(dir string) {
				packagesDir := filepath.Join(dir, "packages")
				os.MkdirAll(packagesDir, 0755)

				// Create duplicate packages (same name)
				pkg1 := resource.Package{
					Name:        "duplicate",
					Description: "First occurrence",
					Resources:   []string{"command/test1"},
				}
				writePackage(t, packagesDir, "duplicate.package.json", pkg1)

				pkg2 := resource.Package{
					Name:        "duplicate",
					Description: "Second occurrence",
					Resources:   []string{"command/test2"},
				}
				writePackage(t, packagesDir, "duplicate-2.package.json", pkg2)
			},
			wantCount: 1,
			wantErr:   false,
			wantNames: []string{"duplicate"},
		},
		{
			name: "non-.package.json files are ignored",
			setupFiles: func(dir string) {
				packagesDir := filepath.Join(dir, "packages")
				os.MkdirAll(packagesDir, 0755)

				// Create README.md
				os.WriteFile(filepath.Join(packagesDir, "README.md"), []byte("# Packages"), 0644)

				// Create .json file without .package.json suffix
				os.WriteFile(filepath.Join(packagesDir, "config.json"), []byte(`{"key": "value"}`), 0644)

				// Create valid package
				pkg := resource.Package{
					Name:        "valid-pkg",
					Description: "Valid package",
					Resources:   []string{"command/test"},
				}
				writePackage(t, packagesDir, "valid-pkg.package.json", pkg)
			},
			wantCount: 1,
			wantErr:   false,
			wantNames: []string{"valid-pkg"},
		},
		{
			name: "nested directory structures are not searched",
			setupFiles: func(dir string) {
				packagesDir := filepath.Join(dir, "packages")
				nestedDir := filepath.Join(packagesDir, "nested")
				os.MkdirAll(nestedDir, 0755)

				// Create package in nested directory (should be ignored)
				pkg := resource.Package{
					Name:        "nested-pkg",
					Description: "Nested package",
					Resources:   []string{"command/test"},
				}
				writePackage(t, nestedDir, "nested-pkg.package.json", pkg)

				// Create package in packages/ root (should be found)
				pkg2 := resource.Package{
					Name:        "root-pkg",
					Description: "Root package",
					Resources:   []string{"command/test"},
				}
				writePackage(t, packagesDir, "root-pkg.package.json", pkg2)
			},
			wantCount: 1,
			wantErr:   false,
			wantNames: []string{"root-pkg"},
		},
		{
			name: "packages with invalid names are skipped",
			setupFiles: func(dir string) {
				packagesDir := filepath.Join(dir, "packages")
				os.MkdirAll(packagesDir, 0755)

				// Package with invalid name (uppercase)
				invalidName := filepath.Join(packagesDir, "InvalidName.package.json")
				os.WriteFile(invalidName, []byte(`{
					"name": "InvalidName",
					"description": "Invalid name with uppercase",
					"resources": ["command/test"]
				}`), 0644)

				// Package with invalid name (special chars)
				invalidChars := filepath.Join(packagesDir, "invalid@name.package.json")
				os.WriteFile(invalidChars, []byte(`{
					"name": "invalid@name",
					"description": "Invalid name with special chars",
					"resources": ["command/test"]
				}`), 0644)

				// Valid package
				pkg := resource.Package{
					Name:        "valid-pkg",
					Description: "Valid package",
					Resources:   []string{"command/test"},
				}
				writePackage(t, packagesDir, "valid-pkg.package.json", pkg)
			},
			wantCount: 1,
			wantErr:   false,
			wantNames: []string{"valid-pkg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()

			// Setup test files
			tt.setupFiles(tmpDir)

			// Run discovery
			packages, err := DiscoverPackages(tmpDir, "")

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("DiscoverPackages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check count
			if len(packages) != tt.wantCount {
				t.Errorf("DiscoverPackages() got %d packages, want %d", len(packages), tt.wantCount)
			}

			// Check names
			if len(tt.wantNames) > 0 {
				foundNames := make(map[string]bool)
				for _, pkg := range packages {
					foundNames[pkg.Name] = true
				}

				for _, wantName := range tt.wantNames {
					if !foundNames[wantName] {
						t.Errorf("Expected to find package %q, but didn't", wantName)
					}
				}
			}
		})
	}
}

func TestDiscoverPackages_NonexistentPath(t *testing.T) {
	_, err := DiscoverPackages("/nonexistent/path", "")
	if err == nil {
		t.Fatal("Expected error for nonexistent path, got nil")
	}
}

func TestDiscoverPackages_WithSubpath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subpath with packages
	subPath := filepath.Join(tmpDir, "subdir")
	packagesDir := filepath.Join(subPath, "packages")
	os.MkdirAll(packagesDir, 0755)

	pkg := resource.Package{
		Name:        "subpath-pkg",
		Description: "Package in subpath",
		Resources:   []string{"command/test"},
	}
	writePackage(t, packagesDir, "subpath-pkg.package.json", pkg)

	// Discover with subpath
	packages, err := DiscoverPackages(tmpDir, "subdir")
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	if len(packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(packages))
	}

	if len(packages) > 0 && packages[0].Name != "subpath-pkg" {
		t.Errorf("Expected package name 'subpath-pkg', got '%s'", packages[0].Name)
	}
}

func TestDiscoverPackages_ValidatesFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	os.MkdirAll(packagesDir, 0755)

	// Create valid package
	pkg := resource.Package{
		Name:        "test-pkg",
		Description: "Test package description",
		Resources:   []string{"command/test", "skill/helper"},
	}
	writePackage(t, packagesDir, "test-pkg.package.json", pkg)

	packages, err := DiscoverPackages(tmpDir, "")
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(packages))
	}

	// Validate all fields are present
	if packages[0].Name != "test-pkg" {
		t.Errorf("Expected name 'test-pkg', got '%s'", packages[0].Name)
	}

	if packages[0].Description != "Test package description" {
		t.Errorf("Expected description 'Test package description', got '%s'", packages[0].Description)
	}

	if len(packages[0].Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(packages[0].Resources))
	}
}

func TestSearchPackagesDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two valid packages
	pkg1 := resource.Package{
		Name:        "pkg1",
		Description: "Package 1",
		Resources:   []string{"command/test1"},
	}
	writePackage(t, tmpDir, "pkg1.package.json", pkg1)

	pkg2 := resource.Package{
		Name:        "pkg2",
		Description: "Package 2",
		Resources:   []string{"command/test2"},
	}
	writePackage(t, tmpDir, "pkg2.package.json", pkg2)

	// Search the directory
	packages, err := searchPackagesDirectory(tmpDir)
	if err != nil {
		t.Fatalf("searchPackagesDirectory failed: %v", err)
	}

	if len(packages) != 2 {
		t.Errorf("Expected 2 packages, got %d", len(packages))
	}

	// Check that both packages were found
	foundNames := make(map[string]bool)
	for _, pkg := range packages {
		foundNames[pkg.Name] = true
	}

	if !foundNames["pkg1"] || !foundNames["pkg2"] {
		t.Errorf("Expected to find both pkg1 and pkg2")
	}
}

func TestSearchPackagesDirectory_NonexistentDir(t *testing.T) {
	_, err := searchPackagesDirectory("/nonexistent/directory")
	if err == nil {
		t.Fatal("Expected error for nonexistent directory, got nil")
	}
}

func TestDeduplicatePackages(t *testing.T) {
	// Create duplicate packages
	pkg1 := &resource.Package{
		Name:        "duplicate",
		Description: "First",
		Resources:   []string{"command/test1"},
	}

	pkg2 := &resource.Package{
		Name:        "duplicate",
		Description: "Second",
		Resources:   []string{"command/test2"},
	}

	pkg3 := &resource.Package{
		Name:        "unique",
		Description: "Unique package",
		Resources:   []string{"command/test3"},
	}

	packages := []*resource.Package{pkg1, pkg2, pkg3}

	// Deduplicate
	unique := deduplicatePackages(packages)

	// Should have 2 unique packages (duplicate and unique)
	if len(unique) != 2 {
		t.Errorf("Expected 2 unique packages, got %d", len(unique))
	}

	// Count occurrences of each name
	nameCount := make(map[string]int)
	for _, pkg := range unique {
		nameCount[pkg.Name]++
	}

	// Each name should appear exactly once
	for name, count := range nameCount {
		if count != 1 {
			t.Errorf("Package %q appears %d times, expected 1", name, count)
		}
	}

	// First occurrence should be kept
	for _, pkg := range unique {
		if pkg.Name == "duplicate" && pkg.Description != "First" {
			t.Errorf("Expected first occurrence of duplicate, got description: %s", pkg.Description)
		}
	}
}

func TestDeduplicatePackages_Empty(t *testing.T) {
	packages := []*resource.Package{}
	unique := deduplicatePackages(packages)

	if len(unique) != 0 {
		t.Errorf("Expected 0 packages for empty input, got %d", len(unique))
	}
}

func TestDeduplicatePackages_NoDeduplication(t *testing.T) {
	// Create unique packages
	pkg1 := &resource.Package{
		Name:        "pkg1",
		Description: "Package 1",
		Resources:   []string{"command/test1"},
	}

	pkg2 := &resource.Package{
		Name:        "pkg2",
		Description: "Package 2",
		Resources:   []string{"command/test2"},
	}

	packages := []*resource.Package{pkg1, pkg2}
	unique := deduplicatePackages(packages)

	if len(unique) != 2 {
		t.Errorf("Expected 2 unique packages, got %d", len(unique))
	}
}

func TestDiscoverPackages_PathIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile.txt")
	os.WriteFile(filePath, []byte("test"), 0644)

	_, err := DiscoverPackages(filePath, "")
	if err == nil {
		t.Fatal("Expected error when path is a file, got nil")
	}
}

func TestDiscoverPackages_ComplexResourceReferences(t *testing.T) {
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	os.MkdirAll(packagesDir, 0755)

	// Create package with various resource types
	pkg := resource.Package{
		Name:        "multi-resource",
		Description: "Package with multiple resource types",
		Resources: []string{
			"command/build",
			"command/test",
			"skill/typescript-helper",
			"skill/react-helper",
			"agent/code-reviewer",
		},
	}
	writePackage(t, packagesDir, "multi-resource.package.json", pkg)

	packages, err := DiscoverPackages(tmpDir, "")
	if err != nil {
		t.Fatalf("DiscoverPackages failed: %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(packages))
	}

	if len(packages[0].Resources) != 5 {
		t.Errorf("Expected 5 resources, got %d", len(packages[0].Resources))
	}
}

// Helper function to write a package to a file
func writePackage(t *testing.T, dir, filename string, pkg resource.Package) {
	t.Helper()

	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal package: %v", err)
	}

	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("Failed to write package file: %v", err)
	}
}
