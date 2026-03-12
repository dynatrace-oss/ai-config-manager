package resource

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseResourceReference tests the ParseResourceReference function
func TestParseResourceReference(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantType     ResourceType
		wantName     string
		wantError    bool
		errorMessage string
	}{
		{
			name:      "valid command",
			input:     "command/test",
			wantType:  Command,
			wantName:  "test",
			wantError: false,
		},
		{
			name:      "valid skill",
			input:     "skill/pdf-processing",
			wantType:  Skill,
			wantName:  "pdf-processing",
			wantError: false,
		},
		{
			name:      "valid agent",
			input:     "agent/code-reviewer",
			wantType:  Agent,
			wantName:  "code-reviewer",
			wantError: false,
		},
		{
			name:      "valid command with numbers",
			input:     "command/test123",
			wantType:  Command,
			wantName:  "test123",
			wantError: false,
		},
		{
			name:      "valid skill with hyphens",
			input:     "skill/pdf-to-text-v2",
			wantType:  Skill,
			wantName:  "pdf-to-text-v2",
			wantError: false,
		},
		{
			name:         "invalid format - no slash",
			input:        "commandtest",
			wantType:     "",
			wantName:     "",
			wantError:    true,
			errorMessage: "invalid resource format",
		},
		{
			name:         "invalid format - only slash",
			input:        "/",
			wantType:     "",
			wantName:     "",
			wantError:    true,
			errorMessage: "invalid resource type",
		},
		{
			name:         "invalid type",
			input:        "plugin/test",
			wantType:     "",
			wantName:     "",
			wantError:    true,
			errorMessage: "invalid resource type",
		},
		{
			name:         "invalid type - uppercase",
			input:        "Command/test",
			wantType:     "",
			wantName:     "",
			wantError:    true,
			errorMessage: "invalid resource type",
		},
		{
			name:         "empty name",
			input:        "command/",
			wantType:     "",
			wantName:     "",
			wantError:    true,
			errorMessage: "resource name cannot be empty",
		},
		{
			name:         "empty type",
			input:        "/test",
			wantType:     "",
			wantName:     "",
			wantError:    true,
			errorMessage: "invalid resource type",
		},
		{
			name:         "multiple slashes",
			input:        "command/test/extra",
			wantType:     Command,
			wantName:     "test/extra",
			wantError:    false,
			errorMessage: "",
		},
		{
			name:         "empty string",
			input:        "",
			wantType:     "",
			wantName:     "",
			wantError:    true,
			errorMessage: "invalid resource format",
		},
		{
			name:         "whitespace",
			input:        "   ",
			wantType:     "",
			wantName:     "",
			wantError:    true,
			errorMessage: "invalid resource format",
		},
		{
			name:      "skill type",
			input:     "skill/test",
			wantType:  Skill,
			wantName:  "test",
			wantError: false,
		},
		{
			name:      "agent type",
			input:     "agent/test",
			wantType:  Agent,
			wantName:  "test",
			wantError: false,
		},
		{
			name:         "unknown type",
			input:        "unknown/test",
			wantType:     "",
			wantName:     "",
			wantError:    true,
			errorMessage: "invalid resource type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotName, err := ParseResourceReference(tt.input)

			// Check error expectation
			if (err != nil) != tt.wantError {
				t.Errorf("ParseResourceReference() error = %v, wantError %v", err, tt.wantError)
				return
			}

			// Check error message if error expected
			if err != nil && tt.errorMessage != "" {
				if !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("ParseResourceReference() error message = %v, want to contain %v", err.Error(), tt.errorMessage)
				}
			}

			// Check type
			if gotType != tt.wantType {
				t.Errorf("ParseResourceReference() type = %v, want %v", gotType, tt.wantType)
			}

			// Check name
			if gotName != tt.wantName {
				t.Errorf("ParseResourceReference() name = %v, want %v", gotName, tt.wantName)
			}
		})
	}
}

// TestLoadPackage tests the LoadPackage function
func TestLoadPackage(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) string
		wantPkg      *Package
		wantError    bool
		errorMessage string
	}{
		{
			name: "valid package",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "test.package.json")
				pkg := Package{
					Name:        "test-package",
					Description: "A test package",
					Resources:   []string{"command/test", "skill/pdf"},
				}
				data, _ := json.MarshalIndent(pkg, "", "  ")
				if err := os.WriteFile(pkgFile, data, 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg: &Package{
				Name:        "test-package",
				Description: "A test package",
				Resources:   []string{"command/test", "skill/pdf"},
			},
			wantError: false,
		},
		{
			name: "valid package with empty resources",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "empty.package.json")
				pkg := Package{
					Name:        "empty-package",
					Description: "Package with no resources",
					Resources:   []string{},
				}
				data, _ := json.MarshalIndent(pkg, "", "  ")
				if err := os.WriteFile(pkgFile, data, 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg: &Package{
				Name:        "empty-package",
				Description: "Package with no resources",
				Resources:   []string{},
			},
			wantError: false,
		},
		{
			name: "valid package with multiple resources",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "multi.package.json")
				pkg := Package{
					Name:        "multi-resource",
					Description: "Package with multiple resources",
					Resources: []string{
						"command/test1",
						"command/test2",
						"skill/pdf",
						"agent/reviewer",
					},
				}
				data, _ := json.MarshalIndent(pkg, "", "  ")
				if err := os.WriteFile(pkgFile, data, 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg: &Package{
				Name:        "multi-resource",
				Description: "Package with multiple resources",
				Resources: []string{
					"command/test1",
					"command/test2",
					"skill/pdf",
					"agent/reviewer",
				},
			},
			wantError: false,
		},
		{
			name: "missing file",
			setup: func(t *testing.T) string {
				return "/nonexistent/path/package.json"
			},
			wantPkg:      nil,
			wantError:    true,
			errorMessage: "failed to read package file",
		},
		{
			name: "invalid JSON",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "invalid.package.json")
				content := `{"name": "test", "description": "test", invalid json}`
				if err := os.WriteFile(pkgFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg:      nil,
			wantError:    true,
			errorMessage: "failed to parse package JSON",
		},
		{
			name: "missing name field",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "noname.package.json")
				content := `{"description": "test package", "resources": []}`
				if err := os.WriteFile(pkgFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg:      nil,
			wantError:    true,
			errorMessage: "package name is required",
		},
		{
			name: "empty name field",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "emptyname.package.json")
				pkg := Package{
					Name:        "",
					Description: "test package",
					Resources:   []string{},
				}
				data, _ := json.Marshal(pkg)
				if err := os.WriteFile(pkgFile, data, 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg:      nil,
			wantError:    true,
			errorMessage: "package name is required",
		},
		{
			name: "missing description field",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "nodesc.package.json")
				content := `{"name": "test-package", "resources": []}`
				if err := os.WriteFile(pkgFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg:      nil,
			wantError:    true,
			errorMessage: "package description is required",
		},
		{
			name: "empty description field",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "emptydesc.package.json")
				pkg := Package{
					Name:        "test-package",
					Description: "",
					Resources:   []string{},
				}
				data, _ := json.Marshal(pkg)
				if err := os.WriteFile(pkgFile, data, 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg:      nil,
			wantError:    true,
			errorMessage: "package description is required",
		},
		{
			name: "invalid package name - uppercase",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "invalid.package.json")
				pkg := Package{
					Name:        "Invalid-Name",
					Description: "test package",
					Resources:   []string{},
				}
				data, _ := json.Marshal(pkg)
				if err := os.WriteFile(pkgFile, data, 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg:      nil,
			wantError:    true,
			errorMessage: "invalid package name",
		},
		{
			name: "invalid package name - underscore",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "invalid.package.json")
				pkg := Package{
					Name:        "test_package",
					Description: "test package",
					Resources:   []string{},
				}
				data, _ := json.Marshal(pkg)
				if err := os.WriteFile(pkgFile, data, 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg:      nil,
			wantError:    true,
			errorMessage: "invalid package name",
		},
		{
			name: "invalid package name - leading hyphen",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "invalid.package.json")
				pkg := Package{
					Name:        "-test",
					Description: "test package",
					Resources:   []string{},
				}
				data, _ := json.Marshal(pkg)
				if err := os.WriteFile(pkgFile, data, 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg:      nil,
			wantError:    true,
			errorMessage: "invalid package name",
		},
		{
			name: "empty file",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "empty.package.json")
				if err := os.WriteFile(pkgFile, []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg:      nil,
			wantError:    true,
			errorMessage: "failed to parse package JSON",
		},
		{
			name: "null resources field",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				pkgFile := filepath.Join(tmpDir, "null.package.json")
				content := `{"name": "test-package", "description": "test", "resources": null}`
				if err := os.WriteFile(pkgFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return pkgFile
			},
			wantPkg: &Package{
				Name:        "test-package",
				Description: "test",
				Resources:   nil,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setup(t)
			gotPkg, err := LoadPackage(filePath)

			// Check error expectation
			if (err != nil) != tt.wantError {
				t.Errorf("LoadPackage() error = %v, wantError %v", err, tt.wantError)
				return
			}

			// Check error message if error expected
			if err != nil && tt.errorMessage != "" {
				if !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("LoadPackage() error message = %v, want to contain %v", err.Error(), tt.errorMessage)
				}
				return
			}

			// Check package equality if no error expected
			if !tt.wantError && gotPkg != nil && tt.wantPkg != nil {
				if gotPkg.Name != tt.wantPkg.Name {
					t.Errorf("LoadPackage() name = %v, want %v", gotPkg.Name, tt.wantPkg.Name)
				}
				if gotPkg.Description != tt.wantPkg.Description {
					t.Errorf("LoadPackage() description = %v, want %v", gotPkg.Description, tt.wantPkg.Description)
				}
				if len(gotPkg.Resources) != len(tt.wantPkg.Resources) {
					t.Errorf("LoadPackage() resources length = %v, want %v", len(gotPkg.Resources), len(tt.wantPkg.Resources))
				} else {
					for i := range gotPkg.Resources {
						if gotPkg.Resources[i] != tt.wantPkg.Resources[i] {
							t.Errorf("LoadPackage() resources[%d] = %v, want %v", i, gotPkg.Resources[i], tt.wantPkg.Resources[i])
						}
					}
				}
			}
		})
	}
}

// TestSavePackage tests the SavePackage function
func TestSavePackage(t *testing.T) {
	tests := []struct {
		name         string
		pkg          *Package
		wantError    bool
		errorMessage string
		validate     func(t *testing.T, repoPath string)
	}{
		{
			name: "valid package",
			pkg: &Package{
				Name:        "test-package",
				Description: "A test package",
				Resources:   []string{"command/test", "skill/pdf"},
			},
			wantError: false,
			validate: func(t *testing.T, repoPath string) {
				// Verify file exists
				filePath := GetPackagePath("test-package", repoPath)
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("SavePackage() did not create file at %v", filePath)
				}

				// Verify file contents
				pkg, err := LoadPackage(filePath)
				if err != nil {
					t.Errorf("SavePackage() created invalid package: %v", err)
				}
				if pkg.Name != "test-package" {
					t.Errorf("SavePackage() name = %v, want test-package", pkg.Name)
				}
				if pkg.Description != "A test package" {
					t.Errorf("SavePackage() description = %v, want 'A test package'", pkg.Description)
				}

				// Verify JSON formatting (should be indented)
				data, _ := os.ReadFile(filePath)
				if !strings.Contains(string(data), "\n  ") {
					t.Errorf("SavePackage() did not format JSON with indentation")
				}
			},
		},
		{
			name: "creates directory if missing",
			pkg: &Package{
				Name:        "new-package",
				Description: "New package",
				Resources:   []string{},
			},
			wantError: false,
			validate: func(t *testing.T, repoPath string) {
				// Verify packages directory was created
				packagesDir := filepath.Join(repoPath, "packages")
				if _, err := os.Stat(packagesDir); os.IsNotExist(err) {
					t.Errorf("SavePackage() did not create packages directory")
				}

				// Verify file exists
				filePath := GetPackagePath("new-package", repoPath)
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("SavePackage() did not create file")
				}
			},
		},
		{
			name: "valid package with multiple resources",
			pkg: &Package{
				Name:        "multi-res",
				Description: "Multiple resources",
				Resources: []string{
					"command/test1",
					"skill/pdf",
					"agent/reviewer",
				},
			},
			wantError: false,
			validate: func(t *testing.T, repoPath string) {
				filePath := GetPackagePath("multi-res", repoPath)
				pkg, err := LoadPackage(filePath)
				if err != nil {
					t.Errorf("SavePackage() created invalid package: %v", err)
				}
				if len(pkg.Resources) != 3 {
					t.Errorf("SavePackage() resources length = %v, want 3", len(pkg.Resources))
				}
			},
		},
		{
			name:         "nil package",
			pkg:          nil,
			wantError:    true,
			errorMessage: "package cannot be nil",
			validate:     nil,
		},
		{
			name: "missing name",
			pkg: &Package{
				Name:        "",
				Description: "test",
				Resources:   []string{},
			},
			wantError:    true,
			errorMessage: "package name is required",
			validate:     nil,
		},
		{
			name: "missing description",
			pkg: &Package{
				Name:        "test",
				Description: "",
				Resources:   []string{},
			},
			wantError:    true,
			errorMessage: "package description is required",
			validate:     nil,
		},
		{
			name: "package overwrites existing",
			pkg: &Package{
				Name:        "overwrite",
				Description: "Updated package",
				Resources:   []string{"command/new"},
			},
			wantError: false,
			validate: func(t *testing.T, repoPath string) {
				// First save should succeed
				filePath := GetPackagePath("overwrite", repoPath)
				pkg, err := LoadPackage(filePath)
				if err != nil {
					t.Errorf("SavePackage() created invalid package: %v", err)
				}
				if pkg.Description != "Updated package" {
					t.Errorf("SavePackage() description = %v, want 'Updated package'", pkg.Description)
				}
			},
		},
		{
			name: "empty resources list",
			pkg: &Package{
				Name:        "empty-res",
				Description: "Empty resources",
				Resources:   []string{},
			},
			wantError: false,
			validate: func(t *testing.T, repoPath string) {
				filePath := GetPackagePath("empty-res", repoPath)
				pkg, err := LoadPackage(filePath)
				if err != nil {
					t.Errorf("SavePackage() created invalid package: %v", err)
				}
				if pkg.Resources == nil || len(pkg.Resources) != 0 {
					t.Errorf("SavePackage() resources = %v, want empty slice", pkg.Resources)
				}
			},
		},
		{
			name: "nil resources list",
			pkg: &Package{
				Name:        "nil-res",
				Description: "Nil resources",
				Resources:   nil,
			},
			wantError: false,
			validate: func(t *testing.T, repoPath string) {
				filePath := GetPackagePath("nil-res", repoPath)
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("SavePackage() did not create file")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary repo directory
			repoPath := t.TempDir()

			// If testing overwrite, create an initial file
			if tt.name == "package overwrites existing" {
				initialPkg := &Package{
					Name:        "overwrite",
					Description: "Original package",
					Resources:   []string{"command/old"},
				}
				if err := SavePackage(initialPkg, repoPath); err != nil {
					t.Fatalf("Failed to create initial package: %v", err)
				}
			}

			// Save package
			err := SavePackage(tt.pkg, repoPath)

			// Check error expectation
			if (err != nil) != tt.wantError {
				t.Errorf("SavePackage() error = %v, wantError %v", err, tt.wantError)
				return
			}

			// Check error message if error expected
			if err != nil && tt.errorMessage != "" {
				if !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("SavePackage() error message = %v, want to contain %v", err.Error(), tt.errorMessage)
				}
				return
			}

			// Run validation if provided
			if tt.validate != nil {
				tt.validate(t, repoPath)
			}
		})
	}
}

// TestGetPackagePath tests the GetPackagePath function
func TestGetPackagePath(t *testing.T) {
	tests := []struct {
		name        string
		packageName string
		repoPath    string
		wantPath    string
	}{
		{
			name:        "simple package name",
			packageName: "test-package",
			repoPath:    "/home/user/repo",
			wantPath:    filepath.Join("/home/user/repo", "packages", "test-package.package.json"),
		},
		{
			name:        "package with hyphens",
			packageName: "my-test-package",
			repoPath:    "/tmp/repo",
			wantPath:    filepath.Join("/tmp/repo", "packages", "my-test-package.package.json"),
		},
		{
			name:        "relative repo path",
			packageName: "pkg",
			repoPath:    "./repo",
			wantPath:    filepath.Join("./repo", "packages", "pkg.package.json"),
		},
		{
			name:        "empty package name",
			packageName: "",
			repoPath:    "/home/user/repo",
			wantPath:    filepath.Join("/home/user/repo", "packages", ".package.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath := GetPackagePath(tt.packageName, tt.repoPath)
			if gotPath != tt.wantPath {
				t.Errorf("GetPackagePath() = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}

// TestPackageRoundTrip tests saving and loading a package
func TestPackageRoundTrip(t *testing.T) {
	repoPath := t.TempDir()

	original := &Package{
		Name:        "roundtrip-test",
		Description: "Testing round trip save/load",
		Resources: []string{
			"command/test1",
			"command/test2",
			"skill/pdf-processing",
			"agent/code-reviewer",
		},
	}

	// Save package
	err := SavePackage(original, repoPath)
	if err != nil {
		t.Fatalf("SavePackage() error = %v", err)
	}

	// Load package
	filePath := GetPackagePath(original.Name, repoPath)
	loaded, err := LoadPackage(filePath)
	if err != nil {
		t.Fatalf("LoadPackage() error = %v", err)
	}

	// Compare
	if loaded.Name != original.Name {
		t.Errorf("Round trip: name = %v, want %v", loaded.Name, original.Name)
	}
	if loaded.Description != original.Description {
		t.Errorf("Round trip: description = %v, want %v", loaded.Description, original.Description)
	}
	if len(loaded.Resources) != len(original.Resources) {
		t.Errorf("Round trip: resources length = %v, want %v", len(loaded.Resources), len(original.Resources))
	} else {
		for i := range loaded.Resources {
			if loaded.Resources[i] != original.Resources[i] {
				t.Errorf("Round trip: resources[%d] = %v, want %v", i, loaded.Resources[i], original.Resources[i])
			}
		}
	}
}
