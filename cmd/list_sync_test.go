package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
)

func TestGetSyncStatus_NoManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Test when no manifest file exists
	status := getSyncStatus(tmpDir, "skill/test-skill", true)
	if status != SyncStatusNoManifest {
		t.Errorf("expected SyncStatusNoManifest when no manifest exists, got %s", status)
	}

	// Test with not installed - still should return no-manifest
	status = getSyncStatus(tmpDir, "skill/test-skill", false)
	if status != SyncStatusNoManifest {
		t.Errorf("expected SyncStatusNoManifest when no manifest exists (not installed), got %s", status)
	}
}

func TestGetSyncStatus_InSync(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with a resource
	m := &manifest.Manifest{
		Resources: []string{"skill/test-skill", "command/test-cmd"},
	}

	manifestPath := filepath.Join(tmpDir, manifest.ManifestFileName)
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	// Test: resource is in manifest and installed
	status := getSyncStatus(tmpDir, "skill/test-skill", true)
	if status != SyncStatusInSync {
		t.Errorf("expected SyncStatusInSync for installed resource in manifest, got %s", status)
	}

	// Test: another resource in manifest and installed
	status = getSyncStatus(tmpDir, "command/test-cmd", true)
	if status != SyncStatusInSync {
		t.Errorf("expected SyncStatusInSync for installed command in manifest, got %s", status)
	}
}

func TestGetSyncStatus_NotInstalled(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with resources
	m := &manifest.Manifest{
		Resources: []string{"skill/test-skill", "agent/code-reviewer"},
	}

	manifestPath := filepath.Join(tmpDir, manifest.ManifestFileName)
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	// Test: resource is in manifest but not installed
	status := getSyncStatus(tmpDir, "skill/test-skill", false)
	if status != SyncStatusNotInstalled {
		t.Errorf("expected SyncStatusNotInstalled for uninstalled resource in manifest, got %s", status)
	}

	// Test: another resource in manifest but not installed
	status = getSyncStatus(tmpDir, "agent/code-reviewer", false)
	if status != SyncStatusNotInstalled {
		t.Errorf("expected SyncStatusNotInstalled for uninstalled agent in manifest, got %s", status)
	}
}

func TestGetSyncStatus_NotInManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with some resources
	m := &manifest.Manifest{
		Resources: []string{"skill/test-skill"},
	}

	manifestPath := filepath.Join(tmpDir, manifest.ManifestFileName)
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	// Test: resource is installed but not in manifest
	status := getSyncStatus(tmpDir, "skill/other-skill", true)
	if status != SyncStatusNotInManifest {
		t.Errorf("expected SyncStatusNotInManifest for installed resource not in manifest, got %s", status)
	}

	// Test: command installed but not in manifest
	status = getSyncStatus(tmpDir, "command/unlisted-cmd", true)
	if status != SyncStatusNotInManifest {
		t.Errorf("expected SyncStatusNotInManifest for installed command not in manifest, got %s", status)
	}
}

func TestGetSyncStatus_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid manifest file (invalid YAML)
	manifestPath := filepath.Join(tmpDir, manifest.ManifestFileName)
	invalidYAML := `resources: [skill/test-skill
this is invalid yaml`
	if err := os.WriteFile(manifestPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write invalid manifest: %v", err)
	}

	// Test: invalid manifest should be treated as no-manifest
	status := getSyncStatus(tmpDir, "skill/test-skill", true)
	if status != SyncStatusNoManifest {
		t.Errorf("expected SyncStatusNoManifest for invalid manifest, got %s", status)
	}
}

func TestGetSyncStatus_EmptyManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with no resources
	m := &manifest.Manifest{
		Resources: []string{},
	}

	manifestPath := filepath.Join(tmpDir, manifest.ManifestFileName)
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	// Test: resource not in empty manifest but installed
	status := getSyncStatus(tmpDir, "skill/test-skill", true)
	if status != SyncStatusNotInManifest {
		t.Errorf("expected SyncStatusNotInManifest for resource not in empty manifest, got %s", status)
	}

	// Test: resource not in empty manifest and not installed
	status = getSyncStatus(tmpDir, "skill/test-skill", false)
	if status != SyncStatusNotInManifest {
		t.Errorf("expected SyncStatusNotInManifest when empty manifest and not installed, got %s", status)
	}
}

func TestGetSyncStatus_AllStateCombinations(t *testing.T) {
	tests := []struct {
		name          string
		inManifest    bool
		isInstalled   bool
		expectedState SyncStatus
		description   string
	}{
		{
			name:          "in-sync",
			inManifest:    true,
			isInstalled:   true,
			expectedState: SyncStatusInSync,
			description:   "resource in manifest and installed",
		},
		{
			name:          "not-installed",
			inManifest:    true,
			isInstalled:   false,
			expectedState: SyncStatusNotInstalled,
			description:   "resource in manifest but not installed",
		},
		{
			name:          "not-in-manifest",
			inManifest:    false,
			isInstalled:   true,
			expectedState: SyncStatusNotInManifest,
			description:   "resource installed but not in manifest",
		},
		{
			name:          "neither",
			inManifest:    false,
			isInstalled:   false,
			expectedState: SyncStatusNotInManifest,
			description:   "resource neither in manifest nor installed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create manifest based on test case
			var resources []string
			if tt.inManifest {
				resources = append(resources, "skill/test-skill")
			}

			m := &manifest.Manifest{
				Resources: resources,
			}

			manifestPath := filepath.Join(tmpDir, manifest.ManifestFileName)
			if err := m.Save(manifestPath); err != nil {
				t.Fatalf("failed to save manifest: %v", err)
			}

			// Test sync status
			status := getSyncStatus(tmpDir, "skill/test-skill", tt.isInstalled)
			if status != tt.expectedState {
				t.Errorf("%s: expected %s, got %s", tt.description, tt.expectedState, status)
			}
		})
	}
}

func TestGetSyncStatus_PackageReferences(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with package reference
	m := &manifest.Manifest{
		Resources: []string{"package/my-package", "skill/test-skill"},
	}

	manifestPath := filepath.Join(tmpDir, manifest.ManifestFileName)
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	// Test: package reference in manifest and "installed"
	status := getSyncStatus(tmpDir, "package/my-package", true)
	if status != SyncStatusInSync {
		t.Errorf("expected SyncStatusInSync for package in manifest, got %s", status)
	}

	// Test: package not installed
	status = getSyncStatus(tmpDir, "package/my-package", false)
	if status != SyncStatusNotInstalled {
		t.Errorf("expected SyncStatusNotInstalled for package in manifest but not installed, got %s", status)
	}
}

func TestGetSyncStatus_MultipleResourceTypes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with various resource types
	m := &manifest.Manifest{
		Resources: []string{
			"command/test-cmd",
			"skill/pdf-processing",
			"agent/code-reviewer",
			"package/my-tools",
		},
	}

	manifestPath := filepath.Join(tmpDir, manifest.ManifestFileName)
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	tests := []struct {
		resourceRef string
		isInstalled bool
		expected    SyncStatus
	}{
		{"command/test-cmd", true, SyncStatusInSync},
		{"command/test-cmd", false, SyncStatusNotInstalled},
		{"skill/pdf-processing", true, SyncStatusInSync},
		{"skill/pdf-processing", false, SyncStatusNotInstalled},
		{"agent/code-reviewer", true, SyncStatusInSync},
		{"agent/code-reviewer", false, SyncStatusNotInstalled},
		{"package/my-tools", true, SyncStatusInSync},
		{"package/my-tools", false, SyncStatusNotInstalled},
		{"command/other-cmd", true, SyncStatusNotInManifest},
		{"skill/other-skill", true, SyncStatusNotInManifest},
		{"agent/other-agent", true, SyncStatusNotInManifest},
	}

	for _, tt := range tests {
		t.Run(tt.resourceRef, func(t *testing.T) {
			status := getSyncStatus(tmpDir, tt.resourceRef, tt.isInstalled)
			if status != tt.expected {
				t.Errorf("resource %s (installed=%v): expected %s, got %s",
					tt.resourceRef, tt.isInstalled, tt.expected, status)
			}
		})
	}
}
