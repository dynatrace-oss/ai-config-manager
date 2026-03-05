package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

func TestPreviewClean(t *testing.T) {
	// Create temp directories
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create tool directories
	claudeDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create tool directory: %v", err)
	}

	// Create symlinks pointing to repo
	repoCommand := filepath.Join(repoDir, "commands", "test-cmd")
	if err := os.MkdirAll(filepath.Dir(repoCommand), 0755); err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}
	if err := os.WriteFile(repoCommand, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create repo command: %v", err)
	}

	symlinkPath := filepath.Join(claudeDir, "test-cmd")
	if err := os.Symlink(repoCommand, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Detect tools
	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	// Preview clean
	resources, err := previewClean(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("previewClean failed: %v", err)
	}

	// Verify results
	if len(resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(resources))
	}

	if len(resources) > 0 {
		if resources[0].Name != "test-cmd" {
			t.Errorf("Resource name = %v, want test-cmd", resources[0].Name)
		}
		if resources[0].Type != "command" {
			t.Errorf("Resource type = %v, want command", resources[0].Type)
		}
	}
}

func TestScanDirectory(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(string, string) error
		expectedCount int
	}{
		{
			name: "finds symlinks pointing to repo",
			setupFunc: func(dir, repoPath string) error {
				target := filepath.Join(repoPath, "commands", "test-cmd")
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(target, filepath.Join(dir, "test-cmd"))
			},
			expectedCount: 1,
		},
		{
			name: "ignores symlinks not pointing to repo",
			setupFunc: func(dir, repoPath string) error {
				otherDir := filepath.Join(os.TempDir(), "other")
				_ = os.MkdirAll(otherDir, 0755)
				target := filepath.Join(otherDir, "test-cmd")
				if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(target, filepath.Join(dir, "test-cmd"))
			},
			expectedCount: 0,
		},
		{
			name: "ignores regular files",
			setupFunc: func(dir, repoPath string) error {
				return os.WriteFile(filepath.Join(dir, "regular-file"), []byte("test"), 0644)
			},
			expectedCount: 0,
		},
		{
			name: "handles non-existent directory",
			setupFunc: func(dir, repoPath string) error {
				// Don't create the directory
				return nil
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			repoDir := t.TempDir()

			// Create directory for test
			testDir := filepath.Join(dir, "commands")
			if tt.name != "handles non-existent directory" {
				if err := os.MkdirAll(testDir, 0755); err != nil {
					t.Fatalf("Failed to create test directory: %v", err)
				}
			}

			// Setup test scenario
			if err := tt.setupFunc(testDir, repoDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			// Scan directory
			resources, err := scanDirectory(testDir, "command", "claude", repoDir)
			if err != nil {
				t.Fatalf("scanDirectory failed: %v", err)
			}

			if len(resources) != tt.expectedCount {
				t.Errorf("Expected %d resources, got %d", tt.expectedCount, len(resources))
			}
		})
	}
}

func TestCleanDirectory(t *testing.T) {
	tests := []struct {
		name            string
		setupFunc       func(string, string) error
		expectedRemoved int
		expectedFailed  int
	}{
		{
			name: "removes symlinks pointing to repo",
			setupFunc: func(dir, repoPath string) error {
				target := filepath.Join(repoPath, "commands", "test-cmd")
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(target, filepath.Join(dir, "test-cmd"))
			},
			expectedRemoved: 1,
			expectedFailed:  0,
		},
		{
			name: "ignores symlinks not pointing to repo",
			setupFunc: func(dir, repoPath string) error {
				otherDir := filepath.Join(os.TempDir(), "other")
				_ = os.MkdirAll(otherDir, 0755)
				target := filepath.Join(otherDir, "test-cmd")
				if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(target, filepath.Join(dir, "test-cmd"))
			},
			expectedRemoved: 0,
			expectedFailed:  0,
		},
		{
			name: "ignores regular files",
			setupFunc: func(dir, repoPath string) error {
				return os.WriteFile(filepath.Join(dir, "regular-file"), []byte("test"), 0644)
			},
			expectedRemoved: 0,
			expectedFailed:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			repoDir := t.TempDir()

			// Create directory
			testDir := filepath.Join(dir, "commands")
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			// Setup test scenario
			if err := tt.setupFunc(testDir, repoDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			// Clean directory
			removed, failed := cleanDirectory(testDir, repoDir)

			if removed != tt.expectedRemoved {
				t.Errorf("Removed = %d, want %d", removed, tt.expectedRemoved)
			}

			if failed != tt.expectedFailed {
				t.Errorf("Failed = %d, want %d", failed, tt.expectedFailed)
			}

			// Verify symlinks were actually removed
			if tt.expectedRemoved > 0 {
				entries, err := os.ReadDir(testDir)
				if err != nil {
					t.Fatalf("Failed to read directory: %v", err)
				}

				// Check that no symlinks pointing to repo remain
				for _, entry := range entries {
					path := filepath.Join(testDir, entry.Name())
					linkInfo, err := os.Lstat(path)
					if err != nil {
						continue
					}

					if linkInfo.Mode()&os.ModeSymlink != 0 {
						target, err := os.Readlink(path)
						if err == nil && strings.HasPrefix(target, repoDir) {
							t.Errorf("Symlink %s pointing to repo was not removed", entry.Name())
						}
					}
				}
			}
		})
	}
}

func TestCleanAll(t *testing.T) {
	// Create temp directories
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Create tool directories
	claudeDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create tool directory: %v", err)
	}

	// Create repo command
	repoCommand := filepath.Join(repoDir, "commands", "test-cmd")
	if err := os.WriteFile(repoCommand, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create repo command: %v", err)
	}

	// Create symlink
	symlinkPath := filepath.Join(claudeDir, "test-cmd")
	if err := os.Symlink(repoCommand, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Detect tools
	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	// Clean all
	removed, failed := cleanAll(projectDir, detectedTools, repoDir)

	if removed != 1 {
		t.Errorf("Removed = %d, want 1", removed)
	}

	if failed != 0 {
		t.Errorf("Failed = %d, want 0", failed)
	}

	// Verify symlink was removed
	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Error("Symlink was not removed")
	}
}

func TestCleanCommand_NoToolDirectories(t *testing.T) {
	// Create temp directory with no tool directories
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Set AIMGR_REPO_PATH
	oldEnv := os.Getenv("AIMGR_REPO_PATH")
	defer func() {
		if oldEnv != "" {
			_ = os.Setenv("AIMGR_REPO_PATH", oldEnv)
		} else {
			_ = os.Unsetenv("AIMGR_REPO_PATH")
		}
	}()
	_ = os.Setenv("AIMGR_REPO_PATH", repoDir)

	// Initialize repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Change to project directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Failed to change to project directory: %v", err)
	}

	// Run clean command - should exit gracefully with no tools
	err = cleanCmd.RunE(cleanCmd, []string{})
	if err != nil {
		t.Errorf("Clean command should succeed with no tool directories, got error: %v", err)
	}
}
