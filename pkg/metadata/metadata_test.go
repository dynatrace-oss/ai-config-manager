package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestGetMetadataPath(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		resourceType resource.ResourceType
		repoPath     string
		wantPath     string
	}{
		{
			name:         "command metadata path",
			resourceName: "test-cmd",
			resourceType: resource.Command,
			repoPath:     "/home/user/.local/share/ai-config/repo",
			wantPath:     "/home/user/.local/share/ai-config/repo/.metadata/commands/test-cmd-metadata.json",
		},
		{
			name:         "skill metadata path",
			resourceName: "pdf-processor",
			resourceType: resource.Skill,
			repoPath:     "/home/user/.local/share/ai-config/repo",
			wantPath:     "/home/user/.local/share/ai-config/repo/.metadata/skills/pdf-processor-metadata.json",
		},
		{
			name:         "agent metadata path",
			resourceName: "code-reviewer",
			resourceType: resource.Agent,
			repoPath:     "/home/user/.local/share/ai-config/repo",
			wantPath:     "/home/user/.local/share/ai-config/repo/.metadata/agents/code-reviewer-metadata.json",
		},
		{
			name:         "command with different repo path",
			resourceName: "my-command",
			resourceType: resource.Command,
			repoPath:     "/tmp/test-repo",
			wantPath:     "/tmp/test-repo/.metadata/commands/my-command-metadata.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath := GetMetadataPath(tt.resourceName, tt.resourceType, tt.repoPath)
			if gotPath != tt.wantPath {
				t.Errorf("GetMetadataPath() = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}

func TestSaveMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		metadata  *ResourceMetadata
		repoPath  string
		wantError bool
	}{
		{
			name: "save command metadata",
			metadata: &ResourceMetadata{
				Name:           "test-command",
				Type:           resource.Command,
				SourceType:     "github",
				SourceURL:      "gh:owner/repo/commands/test-command.md",
				FirstInstalled: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				LastUpdated:    time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
			},
			repoPath:  tmpDir,
			wantError: false,
		},
		{
			name: "save skill metadata",
			metadata: &ResourceMetadata{
				Name:           "pdf-skill",
				Type:           resource.Skill,
				SourceType:     "local",
				SourceURL:      "file:///home/user/skills/pdf-skill",
				FirstInstalled: time.Date(2024, 1, 5, 10, 30, 0, 0, time.UTC),
				LastUpdated:    time.Date(2024, 1, 5, 10, 30, 0, 0, time.UTC),
			},
			repoPath:  tmpDir,
			wantError: false,
		},
		{
			name: "save agent metadata",
			metadata: &ResourceMetadata{
				Name:           "code-agent",
				Type:           resource.Agent,
				SourceType:     "file",
				SourceURL:      "file:///tmp/code-agent.md",
				FirstInstalled: time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC),
				LastUpdated:    time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC),
			},
			repoPath:  tmpDir,
			wantError: false,
		},
		{
			name:      "nil metadata",
			metadata:  nil,
			repoPath:  tmpDir,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Save(tt.metadata, tt.repoPath, "test-source")
			if (err != nil) != tt.wantError {
				t.Errorf("Save() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				// Verify file exists
				metadataPath := GetMetadataPath(tt.metadata.Name, tt.metadata.Type, tt.repoPath)
				if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
					t.Errorf("Save() did not create file at %s", metadataPath)
				}

				// Verify file content
				data, err := os.ReadFile(metadataPath)
				if err != nil {
					t.Fatalf("Failed to read saved metadata file: %v", err)
				}

				var savedMetadata ResourceMetadata
				if err := json.Unmarshal(data, &savedMetadata); err != nil {
					t.Fatalf("Failed to unmarshal saved metadata: %v", err)
				}

				// Verify fields
				if savedMetadata.Name != tt.metadata.Name {
					t.Errorf("Name = %v, want %v", savedMetadata.Name, tt.metadata.Name)
				}
				if savedMetadata.Type != tt.metadata.Type {
					t.Errorf("Type = %v, want %v", savedMetadata.Type, tt.metadata.Type)
				}
				if savedMetadata.SourceType != tt.metadata.SourceType {
					t.Errorf("SourceType = %v, want %v", savedMetadata.SourceType, tt.metadata.SourceType)
				}
				if savedMetadata.SourceURL != tt.metadata.SourceURL {
					t.Errorf("SourceURL = %v, want %v", savedMetadata.SourceURL, tt.metadata.SourceURL)
				}

				// Verify timestamps (using Equal for time.Time comparison)
				if !savedMetadata.FirstInstalled.Equal(tt.metadata.FirstInstalled) {
					t.Errorf("FirstInstalled = %v, want %v", savedMetadata.FirstInstalled, tt.metadata.FirstInstalled)
				}
				if !savedMetadata.LastUpdated.Equal(tt.metadata.LastUpdated) {
					t.Errorf("LastUpdated = %v, want %v", savedMetadata.LastUpdated, tt.metadata.LastUpdated)
				}
			}
		})
	}
}

func TestLoadMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test metadata
	testMetadata := &ResourceMetadata{
		Name:           "test-load",
		Type:           resource.Command,
		SourceType:     "github",
		SourceURL:      "gh:owner/repo/test.md",
		FirstInstalled: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		LastUpdated:    time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
	}

	// Save it first
	if err := Save(testMetadata, tmpDir, "test-source"); err != nil {
		t.Fatalf("Failed to save test metadata: %v", err)
	}

	tests := []struct {
		name         string
		resourceName string
		resourceType resource.ResourceType
		repoPath     string
		wantError    bool
		wantMetadata *ResourceMetadata
	}{
		{
			name:         "load existing metadata",
			resourceName: "test-load",
			resourceType: resource.Command,
			repoPath:     tmpDir,
			wantError:    false,
			wantMetadata: testMetadata,
		},
		{
			name:         "load non-existent metadata",
			resourceName: "does-not-exist",
			resourceType: resource.Command,
			repoPath:     tmpDir,
			wantError:    true,
			wantMetadata: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := Load(tt.resourceName, tt.resourceType, tt.repoPath)
			if (err != nil) != tt.wantError {
				t.Errorf("Load() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				if metadata == nil {
					t.Fatal("Load() returned nil metadata")
				}

				// Verify fields
				if metadata.Name != tt.wantMetadata.Name {
					t.Errorf("Name = %v, want %v", metadata.Name, tt.wantMetadata.Name)
				}
				if metadata.Type != tt.wantMetadata.Type {
					t.Errorf("Type = %v, want %v", metadata.Type, tt.wantMetadata.Type)
				}
				if metadata.SourceType != tt.wantMetadata.SourceType {
					t.Errorf("SourceType = %v, want %v", metadata.SourceType, tt.wantMetadata.SourceType)
				}
				if metadata.SourceURL != tt.wantMetadata.SourceURL {
					t.Errorf("SourceURL = %v, want %v", metadata.SourceURL, tt.wantMetadata.SourceURL)
				}
				if !metadata.FirstInstalled.Equal(tt.wantMetadata.FirstInstalled) {
					t.Errorf("FirstInstalled = %v, want %v", metadata.FirstInstalled, tt.wantMetadata.FirstInstalled)
				}
				if !metadata.LastUpdated.Equal(tt.wantMetadata.LastUpdated) {
					t.Errorf("LastUpdated = %v, want %v", metadata.LastUpdated, tt.wantMetadata.LastUpdated)
				}
			}
		})
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now().UTC()
	original := &ResourceMetadata{
		Name:           "roundtrip-test",
		Type:           resource.Skill,
		SourceType:     "local",
		SourceURL:      "file:///home/user/skills/roundtrip",
		FirstInstalled: now,
		LastUpdated:    now,
	}

	// Save
	if err := Save(original, tmpDir, "test-source"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	loaded, err := Load(original.Name, original.Type, tmpDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Compare
	if loaded.Name != original.Name {
		t.Errorf("Name = %v, want %v", loaded.Name, original.Name)
	}
	if loaded.Type != original.Type {
		t.Errorf("Type = %v, want %v", loaded.Type, original.Type)
	}
	if loaded.SourceType != original.SourceType {
		t.Errorf("SourceType = %v, want %v", loaded.SourceType, original.SourceType)
	}
	if loaded.SourceURL != original.SourceURL {
		t.Errorf("SourceURL = %v, want %v", loaded.SourceURL, original.SourceURL)
	}

	// Time comparison - allow for microsecond differences due to JSON marshaling
	if !loaded.FirstInstalled.Equal(original.FirstInstalled) {
		t.Errorf("FirstInstalled = %v, want %v", loaded.FirstInstalled, original.FirstInstalled)
	}
	if !loaded.LastUpdated.Equal(original.LastUpdated) {
		t.Errorf("LastUpdated = %v, want %v", loaded.LastUpdated, original.LastUpdated)
	}
}

func TestJSONMarshaling(t *testing.T) {
	now := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)

	metadata := &ResourceMetadata{
		Name:           "json-test",
		Type:           resource.Agent,
		SourceType:     "github",
		SourceURL:      "gh:owner/repo/agents/json-test.md",
		FirstInstalled: now,
		LastUpdated:    now,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Verify JSON structure
	var rawJSON map[string]interface{}
	if err := json.Unmarshal(data, &rawJSON); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Check all fields are present
	expectedFields := []string{"name", "type", "source_type", "source_url", "first_installed", "last_updated"}
	for _, field := range expectedFields {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("JSON missing field: %s", field)
		}
	}

	// Unmarshal back
	var unmarshaled ResourceMetadata
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal() into struct error = %v", err)
	}

	// Verify all fields
	if unmarshaled.Name != metadata.Name {
		t.Errorf("Name = %v, want %v", unmarshaled.Name, metadata.Name)
	}
	if unmarshaled.Type != metadata.Type {
		t.Errorf("Type = %v, want %v", unmarshaled.Type, metadata.Type)
	}
	if unmarshaled.SourceType != metadata.SourceType {
		t.Errorf("SourceType = %v, want %v", unmarshaled.SourceType, metadata.SourceType)
	}
	if unmarshaled.SourceURL != metadata.SourceURL {
		t.Errorf("SourceURL = %v, want %v", unmarshaled.SourceURL, metadata.SourceURL)
	}
	if !unmarshaled.FirstInstalled.Equal(metadata.FirstInstalled) {
		t.Errorf("FirstInstalled = %v, want %v", unmarshaled.FirstInstalled, metadata.FirstInstalled)
	}
	if !unmarshaled.LastUpdated.Equal(metadata.LastUpdated) {
		t.Errorf("LastUpdated = %v, want %v", unmarshaled.LastUpdated, metadata.LastUpdated)
	}
}

func TestTimestampFormat(t *testing.T) {
	// Test that timestamps are stored in RFC3339 format
	tmpDir := t.TempDir()

	testTime := time.Date(2024, 6, 1, 15, 30, 45, 0, time.UTC)
	metadata := &ResourceMetadata{
		Name:           "time-format-test",
		Type:           resource.Command,
		SourceType:     "local",
		SourceURL:      "file:///test",
		FirstInstalled: testTime,
		LastUpdated:    testTime,
	}

	// Save
	if err := Save(metadata, tmpDir, "test-source"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read raw JSON
	metadataPath := GetMetadataPath(metadata.Name, metadata.Type, tmpDir)
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	// Parse as map to check raw string format
	var rawJSON map[string]interface{}
	if err := json.Unmarshal(data, &rawJSON); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify timestamps are strings in RFC3339 format
	firstInstalled, ok := rawJSON["first_installed"].(string)
	if !ok {
		t.Fatal("first_installed is not a string")
	}

	lastUpdated, ok := rawJSON["last_updated"].(string)
	if !ok {
		t.Fatal("last_updated is not a string")
	}

	// Parse timestamps to verify RFC3339 format
	_, err = time.Parse(time.RFC3339, firstInstalled)
	if err != nil {
		t.Errorf("first_installed not in RFC3339 format: %v", err)
	}

	_, err = time.Parse(time.RFC3339, lastUpdated)
	if err != nil {
		t.Errorf("last_updated not in RFC3339 format: %v", err)
	}

	// Expected format: 2024-06-01T15:30:45Z
	expectedFormat := "2024-06-01T15:30:45Z"
	if firstInstalled != expectedFormat {
		t.Errorf("first_installed = %v, want %v", firstInstalled, expectedFormat)
	}
	if lastUpdated != expectedFormat {
		t.Errorf("last_updated = %v, want %v", lastUpdated, expectedFormat)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create invalid JSON file
	invalidPath := filepath.Join(tmpDir, ".metadata", "commands", "invalid-metadata.json")
	if err := os.MkdirAll(filepath.Dir(invalidPath), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	invalidJSON := []byte(`{"name": "invalid", "type": "command", invalid json}`)
	if err := os.WriteFile(invalidPath, invalidJSON, 0644); err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	// Try to load
	_, err := Load("invalid", resource.Command, tmpDir)
	if err == nil {
		t.Error("Load() expected error for invalid JSON, got nil")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Use a nested path that doesn't exist
	nestedRepo := filepath.Join(tmpDir, "nested", "repo", "path")

	metadata := &ResourceMetadata{
		Name:           "nested-test",
		Type:           resource.Skill,
		SourceType:     "local",
		SourceURL:      "file:///test",
		FirstInstalled: time.Now().UTC(),
		LastUpdated:    time.Now().UTC(),
	}

	// Save should create all necessary directories
	if err := Save(metadata, nestedRepo, "test-source"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	metadataPath := GetMetadataPath(metadata.Name, metadata.Type, nestedRepo)
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Errorf("Save() did not create file at %s", metadataPath)
	}
}

func TestSourceNameTracking(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := &ResourceMetadata{
		Name:           "source-test",
		Type:           resource.Command,
		SourceType:     "github",
		SourceURL:      "gh:owner/repo",
		FirstInstalled: time.Now().UTC(),
		LastUpdated:    time.Now().UTC(),
	}

	// Save with source name
	sourceName := "my-source"
	if err := Save(metadata, tmpDir, sourceName); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and verify source name
	loaded, err := Load(metadata.Name, metadata.Type, tmpDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.SourceName != sourceName {
		t.Errorf("SourceName = %v, want %v", loaded.SourceName, sourceName)
	}
}

func TestHasSource(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := &ResourceMetadata{
		Name:           "source-check-test",
		Type:           resource.Skill,
		SourceType:     "local",
		SourceURL:      "file:///test",
		FirstInstalled: time.Now().UTC(),
		LastUpdated:    time.Now().UTC(),
	}

	sourceName := "test-source"
	if err := Save(metadata, tmpDir, sourceName); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	tests := []struct {
		name       string
		resName    string
		resType    resource.ResourceType
		sourceName string
		want       bool
	}{
		{
			name:       "matching source",
			resName:    "source-check-test",
			resType:    resource.Skill,
			sourceName: "test-source",
			want:       true,
		},
		{
			name:       "different source",
			resName:    "source-check-test",
			resType:    resource.Skill,
			sourceName: "other-source",
			want:       false,
		},
		{
			name:       "non-existent resource",
			resName:    "does-not-exist",
			resType:    resource.Skill,
			sourceName: "test-source",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasSource(tt.resName, tt.resType, tt.sourceName, tmpDir)
			if got != tt.want {
				t.Errorf("HasSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeriveSourceName(t *testing.T) {
	tests := []struct {
		name      string
		sourceURL string
		want      string
	}{
		{
			name:      "gh prefix",
			sourceURL: "gh:owner/repo",
			want:      "owner-repo",
		},
		{
			name:      "gh prefix with path",
			sourceURL: "gh:owner/repo/path/to/resource",
			want:      "owner-repo",
		},
		{
			name:      "github https URL",
			sourceURL: "https://github.com/owner/repo",
			want:      "owner-repo",
		},
		{
			name:      "github https URL with path",
			sourceURL: "https://github.com/owner/repo/tree/main/commands",
			want:      "owner-repo",
		},
		{
			name:      "file URL",
			sourceURL: "file:///home/user/resources",
			want:      "resources",
		},
		{
			name:      "file URL nested",
			sourceURL: "file:///home/user/my-resources",
			want:      "my-resources",
		},
		{
			name:      "absolute path",
			sourceURL: "/home/user/resources",
			want:      "resources",
		},
		{
			name:      "relative path",
			sourceURL: "./resources",
			want:      "resources",
		},
		{
			name:      "parent relative path",
			sourceURL: "../resources",
			want:      "resources",
		},
		{
			name:      "empty string",
			sourceURL: "",
			want:      "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveSourceName(tt.sourceURL)
			if got != tt.want {
				t.Errorf("DeriveSourceName(%q) = %v, want %v", tt.sourceURL, got, tt.want)
			}
		})
	}
}

func TestBackwardCompatibilityWithLegacyMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	// Create metadata without source_name (legacy format)
	legacyMetadata := &ResourceMetadata{
		Name:           "legacy-test",
		Type:           resource.Agent,
		SourceType:     "github",
		SourceURL:      "gh:owner/repo",
		FirstInstalled: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		LastUpdated:    time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
	}

	// Save without source name (empty string)
	if err := Save(legacyMetadata, tmpDir, ""); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load should work without error
	loaded, err := Load(legacyMetadata.Name, legacyMetadata.Type, tmpDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// SourceName should be empty for legacy metadata
	if loaded.SourceName != "" {
		t.Errorf("SourceName = %v, want empty string for legacy metadata", loaded.SourceName)
	}

	// HasSource should return false for legacy metadata
	if HasSource(legacyMetadata.Name, legacyMetadata.Type, "any-source", tmpDir) {
		t.Error("HasSource() should return false for legacy metadata")
	}
}

// Package Metadata Tests

func TestGetPackageMetadataPath(t *testing.T) {
	tests := []struct {
		name        string
		packageName string
		repoPath    string
		wantPath    string
	}{
		{
			name:        "simple package name",
			packageName: "my-package",
			repoPath:    "/home/user/.local/share/ai-config/repo",
			wantPath:    "/home/user/.local/share/ai-config/repo/.metadata/packages/my-package-metadata.json",
		},
		{
			name:        "package with hyphens",
			packageName: "my-awesome-package",
			repoPath:    "/tmp/test-repo",
			wantPath:    "/tmp/test-repo/.metadata/packages/my-awesome-package-metadata.json",
		},
		{
			name:        "package with numbers",
			packageName: "package123",
			repoPath:    "/var/repo",
			wantPath:    "/var/repo/.metadata/packages/package123-metadata.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath := GetPackageMetadataPath(tt.packageName, tt.repoPath)
			if gotPath != tt.wantPath {
				t.Errorf("GetPackageMetadataPath() = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}

func TestCreatePackageMetadata(t *testing.T) {
	tests := []struct {
		name          string
		packageName   string
		sourceType    string
		sourceURL     string
		resourceCount int
	}{
		{
			name:          "github source package",
			packageName:   "github-pkg",
			sourceType:    "github",
			sourceURL:     "https://github.com/owner/repo",
			resourceCount: 5,
		},
		{
			name:          "local source package",
			packageName:   "local-pkg",
			sourceType:    "file",
			sourceURL:     "file:///home/user/packages",
			resourceCount: 3,
		},
		{
			name:          "package with zero resources",
			packageName:   "empty-pkg",
			sourceType:    "local",
			sourceURL:     "",
			resourceCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beforeCreate := time.Now()
			metadata := CreatePackageMetadata(tt.packageName, tt.sourceType, tt.sourceURL, tt.resourceCount)
			afterCreate := time.Now()

			// Verify all fields are set correctly
			if metadata.Name != tt.packageName {
				t.Errorf("Name = %v, want %v", metadata.Name, tt.packageName)
			}
			if metadata.SourceType != tt.sourceType {
				t.Errorf("SourceType = %v, want %v", metadata.SourceType, tt.sourceType)
			}
			if metadata.SourceURL != tt.sourceURL {
				t.Errorf("SourceURL = %v, want %v", metadata.SourceURL, tt.sourceURL)
			}
			if metadata.ResourceCount != tt.resourceCount {
				t.Errorf("ResourceCount = %v, want %v", metadata.ResourceCount, tt.resourceCount)
			}

			// Verify timestamps are set and reasonable
			if metadata.FirstAdded.IsZero() {
				t.Error("FirstAdded should not be zero")
			}
			if metadata.LastUpdated.IsZero() {
				t.Error("LastUpdated should not be zero")
			}

			// Timestamps should be within the test time range
			if metadata.FirstAdded.Before(beforeCreate) || metadata.FirstAdded.After(afterCreate) {
				t.Errorf("FirstAdded %v not within expected range [%v, %v]",
					metadata.FirstAdded, beforeCreate, afterCreate)
			}
			if metadata.LastUpdated.Before(beforeCreate) || metadata.LastUpdated.After(afterCreate) {
				t.Errorf("LastUpdated %v not within expected range [%v, %v]",
					metadata.LastUpdated, beforeCreate, afterCreate)
			}

			// FirstAdded and LastUpdated should be equal for newly created metadata
			if !metadata.FirstAdded.Equal(metadata.LastUpdated) {
				t.Errorf("FirstAdded %v != LastUpdated %v for new metadata",
					metadata.FirstAdded, metadata.LastUpdated)
			}
		})
	}
}

func TestSavePackageMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		metadata  *PackageMetadata
		repoPath  string
		wantError bool
		checkMsg  string
	}{
		{
			name: "save valid package metadata",
			metadata: &PackageMetadata{
				Name:          "test-package",
				SourceType:    "github",
				SourceURL:     "https://github.com/owner/repo",
				FirstAdded:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				LastUpdated:   time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
				ResourceCount: 10,
			},
			repoPath:  tmpDir,
			wantError: false,
		},
		{
			name: "save with source ref",
			metadata: &PackageMetadata{
				Name:           "ref-package",
				SourceType:     "github",
				SourceURL:      "https://github.com/owner/repo",
				SourceRef:      "v1.0.0",
				FirstAdded:     time.Now(),
				LastUpdated:    time.Now(),
				ResourceCount:  5,
				OriginalFormat: "package",
			},
			repoPath:  tmpDir,
			wantError: false,
		},
		{
			name:      "nil metadata",
			metadata:  nil,
			repoPath:  tmpDir,
			wantError: true,
			checkMsg:  "metadata cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SavePackageMetadata(tt.metadata, tt.repoPath)
			if (err != nil) != tt.wantError {
				t.Errorf("SavePackageMetadata() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if tt.wantError {
				if tt.checkMsg != "" && err != nil {
					if !strings.Contains(err.Error(), tt.checkMsg) {
						t.Errorf("Error message %q should contain %q", err.Error(), tt.checkMsg)
					}
				}
				return
			}

			// Verify file exists
			metadataPath := GetPackageMetadataPath(tt.metadata.Name, tt.repoPath)
			if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
				t.Errorf("SavePackageMetadata() did not create file at %s", metadataPath)
				return
			}

			// Verify file content
			data, err := os.ReadFile(metadataPath)
			if err != nil {
				t.Fatalf("Failed to read saved metadata file: %v", err)
			}

			var savedMetadata PackageMetadata
			if err := json.Unmarshal(data, &savedMetadata); err != nil {
				t.Fatalf("Failed to unmarshal saved metadata: %v", err)
			}

			// Verify all fields
			if savedMetadata.Name != tt.metadata.Name {
				t.Errorf("Name = %v, want %v", savedMetadata.Name, tt.metadata.Name)
			}
			if savedMetadata.SourceType != tt.metadata.SourceType {
				t.Errorf("SourceType = %v, want %v", savedMetadata.SourceType, tt.metadata.SourceType)
			}
			if savedMetadata.SourceURL != tt.metadata.SourceURL {
				t.Errorf("SourceURL = %v, want %v", savedMetadata.SourceURL, tt.metadata.SourceURL)
			}
			if savedMetadata.SourceRef != tt.metadata.SourceRef {
				t.Errorf("SourceRef = %v, want %v", savedMetadata.SourceRef, tt.metadata.SourceRef)
			}
			if savedMetadata.ResourceCount != tt.metadata.ResourceCount {
				t.Errorf("ResourceCount = %v, want %v", savedMetadata.ResourceCount, tt.metadata.ResourceCount)
			}
			if savedMetadata.OriginalFormat != tt.metadata.OriginalFormat {
				t.Errorf("OriginalFormat = %v, want %v", savedMetadata.OriginalFormat, tt.metadata.OriginalFormat)
			}

			// Verify timestamps
			if !savedMetadata.FirstAdded.Equal(tt.metadata.FirstAdded) {
				t.Errorf("FirstAdded = %v, want %v", savedMetadata.FirstAdded, tt.metadata.FirstAdded)
			}
			if !savedMetadata.LastUpdated.Equal(tt.metadata.LastUpdated) {
				t.Errorf("LastUpdated = %v, want %v", savedMetadata.LastUpdated, tt.metadata.LastUpdated)
			}
		})
	}
}

func TestLoadPackageMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test metadata
	testMetadata := &PackageMetadata{
		Name:          "test-load-pkg",
		SourceType:    "github",
		SourceURL:     "https://github.com/test/repo",
		SourceRef:     "main",
		FirstAdded:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		LastUpdated:   time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		ResourceCount: 7,
	}

	// Save it first
	if err := SavePackageMetadata(testMetadata, tmpDir); err != nil {
		t.Fatalf("Failed to save test metadata: %v", err)
	}

	tests := []struct {
		name         string
		packageName  string
		repoPath     string
		wantError    bool
		wantMetadata *PackageMetadata
	}{
		{
			name:         "load existing package metadata",
			packageName:  "test-load-pkg",
			repoPath:     tmpDir,
			wantError:    false,
			wantMetadata: testMetadata,
		},
		{
			name:         "load non-existent package metadata",
			packageName:  "does-not-exist",
			repoPath:     tmpDir,
			wantError:    true,
			wantMetadata: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := LoadPackageMetadata(tt.packageName, tt.repoPath)
			if (err != nil) != tt.wantError {
				t.Errorf("LoadPackageMetadata() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				if metadata == nil {
					t.Fatal("LoadPackageMetadata() returned nil metadata")
				}

				// Verify all fields
				if metadata.Name != tt.wantMetadata.Name {
					t.Errorf("Name = %v, want %v", metadata.Name, tt.wantMetadata.Name)
				}
				if metadata.SourceType != tt.wantMetadata.SourceType {
					t.Errorf("SourceType = %v, want %v", metadata.SourceType, tt.wantMetadata.SourceType)
				}
				if metadata.SourceURL != tt.wantMetadata.SourceURL {
					t.Errorf("SourceURL = %v, want %v", metadata.SourceURL, tt.wantMetadata.SourceURL)
				}
				if metadata.SourceRef != tt.wantMetadata.SourceRef {
					t.Errorf("SourceRef = %v, want %v", metadata.SourceRef, tt.wantMetadata.SourceRef)
				}
				if metadata.ResourceCount != tt.wantMetadata.ResourceCount {
					t.Errorf("ResourceCount = %v, want %v", metadata.ResourceCount, tt.wantMetadata.ResourceCount)
				}

				// Verify timestamps
				if !metadata.FirstAdded.Equal(tt.wantMetadata.FirstAdded) {
					t.Errorf("FirstAdded = %v, want %v", metadata.FirstAdded, tt.wantMetadata.FirstAdded)
				}
				if !metadata.LastUpdated.Equal(tt.wantMetadata.LastUpdated) {
					t.Errorf("LastUpdated = %v, want %v", metadata.LastUpdated, tt.wantMetadata.LastUpdated)
				}
			}
		})
	}
}

func TestLoadPackageMetadataCorruptedJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create corrupted JSON file
	corruptedPath := filepath.Join(tmpDir, ".metadata", "packages", "corrupted-metadata.json")
	if err := os.MkdirAll(filepath.Dir(corruptedPath), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	corruptedJSON := []byte(`{"name": "corrupted", "source_type": invalid json}`)
	if err := os.WriteFile(corruptedPath, corruptedJSON, 0644); err != nil {
		t.Fatalf("Failed to write corrupted JSON: %v", err)
	}

	// Try to load
	_, err := LoadPackageMetadata("corrupted", tmpDir)
	if err == nil {
		t.Error("LoadPackageMetadata() expected error for corrupted JSON, got nil")
	}

	// Verify error mentions unmarshaling
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("Error should mention unmarshaling, got: %v", err)
	}
}

func TestPackageMetadataSaveAndLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now().UTC()
	original := &PackageMetadata{
		Name:           "roundtrip-pkg",
		SourceType:     "github",
		SourceURL:      "https://github.com/test/roundtrip",
		SourceName:     "test-roundtrip",
		SourceRef:      "v1.2.3",
		FirstAdded:     now,
		LastUpdated:    now,
		ResourceCount:  15,
		OriginalFormat: "package",
	}

	// Save
	if err := SavePackageMetadata(original, tmpDir); err != nil {
		t.Fatalf("SavePackageMetadata() error = %v", err)
	}

	// Load
	loaded, err := LoadPackageMetadata(original.Name, tmpDir)
	if err != nil {
		t.Fatalf("LoadPackageMetadata() error = %v", err)
	}

	// Compare all fields
	if loaded.Name != original.Name {
		t.Errorf("Name = %v, want %v", loaded.Name, original.Name)
	}
	if loaded.SourceType != original.SourceType {
		t.Errorf("SourceType = %v, want %v", loaded.SourceType, original.SourceType)
	}
	if loaded.SourceURL != original.SourceURL {
		t.Errorf("SourceURL = %v, want %v", loaded.SourceURL, original.SourceURL)
	}
	if loaded.SourceName != original.SourceName {
		t.Errorf("SourceName = %v, want %v", loaded.SourceName, original.SourceName)
	}
	if loaded.SourceRef != original.SourceRef {
		t.Errorf("SourceRef = %v, want %v", loaded.SourceRef, original.SourceRef)
	}
	if loaded.ResourceCount != original.ResourceCount {
		t.Errorf("ResourceCount = %v, want %v", loaded.ResourceCount, original.ResourceCount)
	}
	if loaded.OriginalFormat != original.OriginalFormat {
		t.Errorf("OriginalFormat = %v, want %v", loaded.OriginalFormat, original.OriginalFormat)
	}

	// Time comparison
	if !loaded.FirstAdded.Equal(original.FirstAdded) {
		t.Errorf("FirstAdded = %v, want %v", loaded.FirstAdded, original.FirstAdded)
	}
	if !loaded.LastUpdated.Equal(original.LastUpdated) {
		t.Errorf("LastUpdated = %v, want %v", loaded.LastUpdated, original.LastUpdated)
	}
}

func TestPackageMetadataCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Use a nested path that doesn't exist
	nestedRepo := filepath.Join(tmpDir, "nested", "repo", "path")

	metadata := &PackageMetadata{
		Name:          "nested-pkg-test",
		SourceType:    "local",
		SourceURL:     "file:///test",
		FirstAdded:    time.Now().UTC(),
		LastUpdated:   time.Now().UTC(),
		ResourceCount: 1,
	}

	// Save should create all necessary directories
	if err := SavePackageMetadata(metadata, nestedRepo); err != nil {
		t.Fatalf("SavePackageMetadata() error = %v", err)
	}

	// Verify file exists
	metadataPath := GetPackageMetadataPath(metadata.Name, nestedRepo)
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Errorf("SavePackageMetadata() did not create file at %s", metadataPath)
	}

	// Verify parent directories exist
	metadataDir := filepath.Dir(metadataPath)
	if _, err := os.Stat(metadataDir); os.IsNotExist(err) {
		t.Errorf("SavePackageMetadata() did not create directory at %s", metadataDir)
	}
}

func TestPackageMetadataJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()

	testTime := time.Date(2024, 6, 1, 15, 30, 45, 0, time.UTC)
	metadata := &PackageMetadata{
		Name:          "json-format-pkg",
		SourceType:    "github",
		SourceURL:     "https://github.com/test/repo",
		FirstAdded:    testTime,
		LastUpdated:   testTime,
		ResourceCount: 3,
	}

	// Save
	if err := SavePackageMetadata(metadata, tmpDir); err != nil {
		t.Fatalf("SavePackageMetadata() error = %v", err)
	}

	// Read raw JSON
	metadataPath := GetPackageMetadataPath(metadata.Name, tmpDir)
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	// Parse as map to check structure
	var rawJSON map[string]interface{}
	if err := json.Unmarshal(data, &rawJSON); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Check required fields are present
	requiredFields := []string{"name", "source_type", "first_added", "last_updated", "resource_count"}
	for _, field := range requiredFields {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("JSON missing required field: %s", field)
		}
	}

	// Verify timestamps are in RFC3339 format
	firstAdded, ok := rawJSON["first_added"].(string)
	if !ok {
		t.Fatal("first_added is not a string")
	}

	_, err = time.Parse(time.RFC3339, firstAdded)
	if err != nil {
		t.Errorf("first_added not in RFC3339 format: %v", err)
	}
}
