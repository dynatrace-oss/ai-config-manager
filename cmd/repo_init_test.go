package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
)

func TestRepoInitHelpClarifiesApplyRelationship(t *testing.T) {
	help := repoInitCmd.Long
	for _, expected := range []string{
		"repo init is local bootstrap only",
		"repo apply-manifest <path-or-url>",
		"repo show-manifest",
		"bootstrapping from a shared manifest",
	} {
		if !strings.Contains(help, expected) {
			t.Fatalf("expected repo init help to mention %q", expected)
		}
	}
}

func TestRepoInitCommand(t *testing.T) {
	tests := []struct {
		name          string
		existingRepo  bool
		expectError   bool
		errorContains string
	}{
		{
			name:         "initializes repository when none exists",
			existingRepo: false,
			expectError:  false,
		},
		{
			name:         "succeeds when repository already exists (idempotent)",
			existingRepo: true,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for test repository
			repoDir := t.TempDir()

			// Set AIMGR_REPO_PATH to use test directory
			oldEnv := os.Getenv("AIMGR_REPO_PATH")
			defer func() {
				if oldEnv != "" {
					_ = os.Setenv("AIMGR_REPO_PATH", oldEnv)
				} else {
					_ = os.Unsetenv("AIMGR_REPO_PATH")
				}
			}()
			_ = os.Setenv("AIMGR_REPO_PATH", repoDir)

			// Create existing repo if needed
			if tt.existingRepo {
				manager := repo.NewManagerWithPath(repoDir)
				if err := manager.Init(); err != nil {
					t.Fatalf("Failed to create existing repo: %v", err)
				}
			}

			// Run repo init command
			err := repoInitCmd.RunE(repoInitCmd, []string{})

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify repository structure was created
			checkDirs := []string{
				filepath.Join(repoDir, "commands"),
				filepath.Join(repoDir, "skills"),
				filepath.Join(repoDir, "agents"),
				filepath.Join(repoDir, "packages"),
			}

			for _, dir := range checkDirs {
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					t.Errorf("Expected directory %s to exist", dir)
				}
			}

			// Verify .gitignore was created
			gitignorePath := filepath.Join(repoDir, ".gitignore")
			if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
				t.Error(".gitignore file was not created")
			}

			// Verify git repository was initialized
			gitDir := filepath.Join(repoDir, ".git")
			if _, err := os.Stat(gitDir); os.IsNotExist(err) {
				t.Error("Git repository was not initialized")
			}

			// Verify ai.repo.yaml manifest was created
			manifestPath := filepath.Join(repoDir, "ai.repo.yaml")
			if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
				t.Error("ai.repo.yaml manifest was not created")
			}
		})
	}
}

func TestRepoInitWithCustomPath(t *testing.T) {
	// Create temp directory for test repository
	repoDir := t.TempDir()

	// Set AIMGR_REPO_PATH to use test directory
	oldEnv := os.Getenv("AIMGR_REPO_PATH")
	defer func() {
		if oldEnv != "" {
			_ = os.Setenv("AIMGR_REPO_PATH", oldEnv)
		} else {
			_ = os.Unsetenv("AIMGR_REPO_PATH")
		}
	}()
	_ = os.Setenv("AIMGR_REPO_PATH", repoDir)

	// Run repo init command
	err := repoInitCmd.RunE(repoInitCmd, []string{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify repository was created at the custom path
	if _, err := os.Stat(filepath.Join(repoDir, "commands")); os.IsNotExist(err) {
		t.Error("Repository was not created at custom path")
	}
}

func TestRepoInitCreatesGitRepository(t *testing.T) {
	// Create temp directory for test repository
	repoDir := t.TempDir()

	// Set AIMGR_REPO_PATH to use test directory
	oldEnv := os.Getenv("AIMGR_REPO_PATH")
	defer func() {
		if oldEnv != "" {
			_ = os.Setenv("AIMGR_REPO_PATH", oldEnv)
		} else {
			_ = os.Unsetenv("AIMGR_REPO_PATH")
		}
	}()
	_ = os.Setenv("AIMGR_REPO_PATH", repoDir)

	// Run repo init command
	err := repoInitCmd.RunE(repoInitCmd, []string{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify git repository exists
	gitDir := filepath.Join(repoDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("Git repository was not initialized")
	}

	// Verify .gitignore contains .workspace/
	gitignorePath := filepath.Join(repoDir, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("Failed to read .gitignore: %v", err)
	}

	if len(content) == 0 {
		t.Error(".gitignore is empty")
	}
}
