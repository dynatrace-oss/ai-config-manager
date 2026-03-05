package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
)

func TestInitCommand(t *testing.T) {
	tests := []struct {
		name          string
		existingFile  bool
		expectError   bool
		errorContains string
	}{
		{
			name:         "creates manifest when none exists",
			existingFile: false,
			expectError:  false,
		},
		{
			name:          "errors when manifest already exists",
			existingFile:  true,
			expectError:   true,
			errorContains: "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()

			// Create existing file if needed
			manifestPath := filepath.Join(tmpDir, manifest.ManifestFileName)
			if tt.existingFile {
				err := os.WriteFile(manifestPath, []byte("resources: []\n"), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			// Change to temp directory
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current directory: %v", err)
			}
			defer func() { _ = os.Chdir(originalDir) }()

			err = os.Chdir(tmpDir)
			if err != nil {
				t.Fatalf("Failed to change to temp directory: %v", err)
			}

			// Run init command
			err = runInit(initCmd, []string{})

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Fatalf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify file was created
			if !manifest.Exists(manifestPath) {
				t.Fatal("Manifest file was not created")
			}

			// Verify file contents
			m, err := manifest.Load(manifestPath)
			if err != nil {
				t.Fatalf("Failed to load created manifest: %v", err)
			}

			if m.Resources == nil {
				t.Fatal("Resources array is nil")
			}

			if len(m.Resources) != 0 {
				t.Fatalf("Expected empty resources array, got %d items", len(m.Resources))
			}
		})
	}
}

func TestInitCommandWithYesFlag(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Set yes flag
	initYesFlag = true
	defer func() { initYesFlag = false }()

	// Run init command
	err = runInit(initCmd, []string{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify file was created
	manifestPath := filepath.Join(tmpDir, manifest.ManifestFileName)
	if !manifest.Exists(manifestPath) {
		t.Fatal("Manifest file was not created")
	}

	// Verify file contents
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load created manifest: %v", err)
	}

	if m.Resources == nil {
		t.Fatal("Resources array is nil")
	}

	if len(m.Resources) != 0 {
		t.Fatalf("Expected empty resources array, got %d items", len(m.Resources))
	}
}
