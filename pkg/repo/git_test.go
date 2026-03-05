//go:build unit

package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// setupGitRepo initializes a git repository in the given directory
func setupGitRepo(t *testing.T, dir string) {
	t.Helper()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to initialize git: %v\nOutput: %s", err, output)
	}

	// Configure git user (required for commits)
	configName := exec.Command("git", "config", "user.name", "Test User")
	configName.Dir = dir
	if err := configName.Run(); err != nil {
		t.Fatalf("Failed to configure git user.name: %v", err)
	}

	configEmail := exec.Command("git", "config", "user.email", "test@example.com")
	configEmail.Dir = dir
	if err := configEmail.Run(); err != nil {
		t.Fatalf("Failed to configure git user.email: %v", err)
	}
}

// getGitLog returns the git log output (empty string if no commits)
func getGitLog(t *testing.T, dir string) string {
	t.Helper()

	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	output, _ := cmd.CombinedOutput() // Ignore error - may have no commits
	return string(output)
}

// getLastCommitMessage returns the last commit message
func getLastCommitMessage(t *testing.T, dir string) string {
	t.Helper()

	cmd := exec.Command("git", "log", "-1", "--pretty=%s")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get last commit message: %v\nOutput: %s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func TestIsGitRepo(t *testing.T) {
	tests := []struct {
		name     string
		setupGit bool
		want     bool
	}{
		{
			name:     "directory with git",
			setupGit: true,
			want:     true,
		},
		{
			name:     "directory without git",
			setupGit: false,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if tt.setupGit {
				setupGitRepo(t, tmpDir)
			}

			manager := NewManagerWithPath(tmpDir)
			got := manager.isGitRepo()

			if got != tt.want {
				t.Errorf("isGitRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommitChanges_Success(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	manager := NewManagerWithPath(tmpDir)

	// Initialize repo structure
	if err := manager.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Commit the changes
	commitMsg := "test: add test file"
	if err := manager.CommitChanges(commitMsg); err != nil {
		t.Fatalf("CommitChanges() error = %v", err)
	}

	// Verify commit was created
	lastMsg := getLastCommitMessage(t, tmpDir)
	if lastMsg != commitMsg {
		t.Errorf("Last commit message = %q, want %q", lastMsg, commitMsg)
	}
}

func TestCommitChanges_NoGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManagerWithPath(tmpDir)

	// Try to commit without a git repo - should not error
	err := manager.CommitChanges("test: should not fail")
	if err != nil {
		t.Errorf("CommitChanges() without git repo error = %v, want nil", err)
	}
}

func TestCommitChanges_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	manager := NewManagerWithPath(tmpDir)

	// Initialize repo and make initial commit
	if err := manager.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tmpDir, "initial.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}
	if err := manager.CommitChanges("initial commit"); err != nil {
		t.Fatalf("Initial CommitChanges() error = %v", err)
	}

	// Try to commit with no changes - should not error
	err := manager.CommitChanges("test: no changes")
	if err != nil {
		t.Errorf("CommitChanges() with no changes error = %v, want nil", err)
	}
}

func TestAddCommand_CreatesCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	manager := NewManagerWithPath(tmpDir)

	// Create a test command file in proper directory structure
	cmdDir := t.TempDir()
	commandsDir := filepath.Join(cmdDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	testCmd := filepath.Join(commandsDir, "test-cmd.md")
	cmdContent := `---
description: A test command
---

# Test Command

This is a test command.
`
	if err := os.WriteFile(testCmd, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Import the command using AddBulk (which triggers commit)
	opts := BulkImportOptions{
		ImportMode: "copy",
		DryRun:     false,
	}
	result, err := manager.AddBulk([]string{testCmd}, opts)
	if err != nil {
		t.Fatalf("AddBulk() error = %v", err)
	}

	if len(result.Added) != 1 {
		t.Fatalf("Expected 1 resource added, got %d. Failed: %v", len(result.Added), result.Failed)
	}

	// Verify commit was created
	lastMsg := getLastCommitMessage(t, tmpDir)
	if !strings.Contains(lastMsg, "aimgr: import") {
		t.Errorf("Last commit message = %q, want to contain 'aimgr: import'", lastMsg)
	}
	if !strings.Contains(lastMsg, "1 command(s)") {
		t.Errorf("Last commit message = %q, want to contain '1 command(s)'", lastMsg)
	}
}

func TestRemove_CreatesCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	manager := NewManagerWithPath(tmpDir)

	// Create and add a test command in proper directory structure
	cmdDir := t.TempDir()
	commandsDir := filepath.Join(cmdDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	testCmd := filepath.Join(commandsDir, "test-cmd.md")
	cmdContent := `---
description: A test command
---

# Test Command
`
	if err := os.WriteFile(testCmd, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Import the command
	opts := BulkImportOptions{
		ImportMode: "copy",
		DryRun:     false,
	}
	result, err := manager.AddBulk([]string{testCmd}, opts)
	if err != nil {
		t.Fatalf("AddBulk() error = %v", err)
	}
	if len(result.Added) != 1 {
		t.Fatalf("Expected 1 resource added, got %d. Failed: %v", len(result.Added), result.Failed)
	}

	// Remove the command
	if err := manager.Remove("test-cmd", resource.Command); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Verify commit was created
	lastMsg := getLastCommitMessage(t, tmpDir)
	expectedMsg := "aimgr: remove command: test-cmd"
	if lastMsg != expectedMsg {
		t.Errorf("Last commit message = %q, want %q", lastMsg, expectedMsg)
	}
}

func TestAddBulk_CreatesCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	manager := NewManagerWithPath(tmpDir)

	// Create test resources in proper directory structure
	resourcesDir := t.TempDir()

	// Create a command in commands/ directory
	commandsDir := filepath.Join(resourcesDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	cmdContent := `---
description: Test command
---
# Command
`
	cmdPath := filepath.Join(commandsDir, "cmd1.md")
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Create a skill
	skillDir := filepath.Join(resourcesDir, "skill1")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	skillContent := `---
description: Test skill
---

# Test Skill

Description: Test skill content
`
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create skill: %v", err)
	}

	// Import bulk resources
	opts := BulkImportOptions{
		ImportMode: "copy",
		DryRun:     false,
	}
	result, err := manager.AddBulk([]string{cmdPath, skillDir}, opts)
	if err != nil {
		t.Fatalf("AddBulk() error = %v", err)
	}

	if len(result.Added) != 2 {
		t.Fatalf("Expected 2 resources added, got %d. Failed: %v", len(result.Added), result.Failed)
	}

	// Verify commit was created with correct message
	lastMsg := getLastCommitMessage(t, tmpDir)
	if !strings.Contains(lastMsg, "aimgr: import 2 resource(s)") {
		t.Errorf("Last commit message = %q, want to contain 'aimgr: import 2 resource(s)'", lastMsg)
	}
	if !strings.Contains(lastMsg, "1 command(s)") {
		t.Errorf("Last commit message = %q, want to contain '1 command(s)'", lastMsg)
	}
	if !strings.Contains(lastMsg, "1 skill(s)") {
		t.Errorf("Last commit message = %q, want to contain '1 skill(s)'", lastMsg)
	}
}

func TestAddBulk_DryRun_NoCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	manager := NewManagerWithPath(tmpDir)

	// Create a test command in proper directory structure
	cmdDir := t.TempDir()
	commandsDir := filepath.Join(cmdDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	testCmd := filepath.Join(commandsDir, "test-cmd.md")
	cmdContent := `---
description: A test command
---
# Test
`
	if err := os.WriteFile(testCmd, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Get initial log (may be empty)
	initialLog := getGitLog(t, tmpDir)

	// Import with dry run
	opts := BulkImportOptions{
		ImportMode: "copy",
		DryRun:     true,
	}
	if _, err := manager.AddBulk([]string{testCmd}, opts); err != nil {
		t.Fatalf("AddBulk() error = %v", err)
	}

	// Verify no commit was created (log should be unchanged)
	finalLog := getGitLog(t, tmpDir)
	if initialLog != finalLog {
		t.Errorf("Git log changed during dry run:\nBefore: %q\nAfter: %q", initialLog, finalLog)
	}
}
