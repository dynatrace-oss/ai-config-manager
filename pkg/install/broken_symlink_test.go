package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

// TestInstallOverBrokenSymlink verifies that installing over a broken symlink works correctly
func TestInstallOverBrokenSymlink(t *testing.T) {
	// Create temporary directories
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create a skill in the repo
	tempSkillDir := t.TempDir()
	skillDir := filepath.Join(tempSkillDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte("---\ndescription: A test skill\n---\n\n# Test Skill\n\nA test skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Add skill to repo
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to repo: %v", err)
	}

	// Create installer
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	// Create a broken symlink to simulate old installation with wrong repo path
	skillsDir := filepath.Join(projectDir, ".opencode/skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills directory: %v", err)
	}
	brokenSymlink := filepath.Join(skillsDir, "test-skill")
	brokenTarget := "/nonexistent/path/to/skill"
	if err := os.Symlink(brokenTarget, brokenSymlink); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	// Verify symlink is broken
	if _, err := os.Stat(brokenSymlink); err == nil {
		t.Fatal("Expected broken symlink, but target exists")
	}

	// Try to install - should detect broken symlink and replace it
	if err := installer.InstallSkill("test-skill", manager); err != nil {
		t.Fatalf("Failed to install skill over broken symlink: %v", err)
	}

	// Verify symlink now points to correct target and is valid
	linkInfo, err := os.Lstat(brokenSymlink)
	if err != nil {
		t.Fatalf("Symlink doesn't exist after installation: %v", err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatal("Expected symlink, got regular file/directory")
	}

	// Verify target is valid
	targetInfo, err := os.Stat(brokenSymlink)
	if err != nil {
		t.Fatalf("Symlink target is invalid: %v", err)
	}
	if !targetInfo.IsDir() {
		t.Fatal("Expected symlink to point to directory")
	}

	// Verify target path is correct
	actualTarget, err := os.Readlink(brokenSymlink)
	if err != nil {
		t.Fatalf("Failed to read symlink target: %v", err)
	}
	expectedTarget := filepath.Join(repoDir, "skills", "test-skill")
	if actualTarget != expectedTarget {
		t.Errorf("Symlink points to wrong target: got %s, want %s", actualTarget, expectedTarget)
	}
}

// TestInstallSkipsValidSymlink verifies that installing over a valid symlink is skipped
func TestInstallSkipsValidSymlink(t *testing.T) {
	// Create temporary directories
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create a skill in the repo
	tempSkillDir := t.TempDir()
	skillDir := filepath.Join(tempSkillDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte("---\ndescription: A test skill\n---\n\n# Test Skill\n\nA test skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Add skill to repo
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to repo: %v", err)
	}

	// Create installer and install the skill
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create installer: %v", err)
	}

	if err := installer.InstallSkill("test-skill", manager); err != nil {
		t.Fatalf("Failed to install skill: %v", err)
	}

	// Get the symlink path and target
	symlinkPath := filepath.Join(projectDir, ".opencode/skills/test-skill")
	originalTarget, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	// Try to install again - should skip without error
	if err := installer.InstallSkill("test-skill", manager); err != nil {
		t.Fatalf("Second install failed: %v", err)
	}

	// Verify symlink wasn't modified
	currentTarget, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Symlink missing after second install: %v", err)
	}
	if currentTarget != originalTarget {
		t.Errorf("Symlink was modified: got %s, want %s", currentTarget, originalTarget)
	}
}

// TestEnsureValidSymlink tests the helper function directly
func TestEnsureValidSymlink(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name            string
		setup           func(string) error
		expectedProceed bool
		expectError     bool
	}{
		{
			name:            "no symlink exists",
			setup:           func(path string) error { return nil },
			expectedProceed: true,
			expectError:     false,
		},
		{
			name: "valid symlink exists",
			setup: func(path string) error {
				target := filepath.Join(tempDir, "valid-target")
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
				return os.Symlink(target, path)
			},
			expectedProceed: false,
			expectError:     false,
		},
		{
			name: "broken symlink exists",
			setup: func(path string) error {
				return os.Symlink("/nonexistent/path", path)
			},
			expectedProceed: true,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symlinkPath := filepath.Join(tempDir, "test-"+tt.name)
			if err := tt.setup(symlinkPath); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			proceed, err := ensureValidSymlink(symlinkPath, "dummy-target", tempDir)
			if tt.expectError && err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if proceed != tt.expectedProceed {
				t.Errorf("proceed = %v, want %v", proceed, tt.expectedProceed)
			}

			// If broken symlink, verify it was removed
			if tt.name == "broken symlink exists" {
				if _, err := os.Lstat(symlinkPath); err == nil {
					t.Error("Broken symlink was not removed")
				}
			}
		})
	}
}

// TestInstallFixesWrongRepoSymlink verifies symlinks pointing to old repo path are fixed
func TestInstallFixesWrongRepoSymlink(t *testing.T) {
	// Create temporary directories
	projectDir := t.TempDir()
	oldRepoDir := t.TempDir()
	newRepoDir := t.TempDir()

	// Initialize OLD repo and add skill
	oldManager := repo.NewManagerWithPath(oldRepoDir)
	if err := oldManager.Init(); err != nil {
		t.Fatalf("Failed to init old repo: %v", err)
	}

	tempOldDir := t.TempDir()
	oldSkillDir := filepath.Join(tempOldDir, "test-skill")
	if err := os.MkdirAll(oldSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory in old repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldSkillDir, "SKILL.md"), []byte("---\ndescription: Old skill\n---\n\n# Old Skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md in old repo: %v", err)
	}
	if err := oldManager.AddSkill(oldSkillDir, "file://"+oldSkillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to old repo: %v", err)
	}

	// Install from OLD repo
	oldInstaller, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create old installer: %v", err)
	}
	if err := oldInstaller.InstallSkill("test-skill", oldManager); err != nil {
		t.Fatalf("Failed to install from old repo: %v", err)
	}

	// Initialize NEW repo and add skill
	newManager := repo.NewManagerWithPath(newRepoDir)
	if err := newManager.Init(); err != nil {
		t.Fatalf("Failed to init new repo: %v", err)
	}

	tempNewDir := t.TempDir()
	newSkillDir := filepath.Join(tempNewDir, "test-skill")
	if err := os.MkdirAll(newSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory in new repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(newSkillDir, "SKILL.md"), []byte("---\ndescription: New skill\n---\n\n# New Skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md in new repo: %v", err)
	}
	if err := newManager.AddSkill(newSkillDir, "file://"+newSkillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to new repo: %v", err)
	}

	// Install from NEW repo - should detect old repo symlink and replace
	newInstaller, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("Failed to create new installer: %v", err)
	}
	if err := newInstaller.InstallSkill("test-skill", newManager); err != nil {
		t.Fatalf("Failed to install from new repo: %v", err)
	}

	// Verify symlink now points to new repo
	symlinkPath := filepath.Join(projectDir, ".opencode/skills/test-skill")
	actualTarget, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	expectedTarget := filepath.Join(newRepoDir, "skills", "test-skill")
	if actualTarget != expectedTarget {
		t.Errorf("Symlink still points to old repo: got %s, want %s", actualTarget, expectedTarget)
	}

	// Verify content is from new repo
	content, err := os.ReadFile(filepath.Join(symlinkPath, "SKILL.md"))
	if err != nil {
		t.Fatalf("Failed to read SKILL.md: %v", err)
	}
	if !strings.Contains(string(content), "New skill") {
		t.Errorf("Skill content is from old repo: %s", string(content))
	}
}
