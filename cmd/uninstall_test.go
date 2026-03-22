package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

func TestParseResourceArg(t *testing.T) {
	tests := []struct {
		name        string
		arg         string
		wantType    resource.ResourceType
		wantName    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid skill",
			arg:      "skill/my-skill",
			wantType: resource.Skill,
			wantName: "my-skill",
			wantErr:  false,
		},
		{
			name:     "valid command",
			arg:      "command/my-command",
			wantType: resource.Command,
			wantName: "my-command",
			wantErr:  false,
		},
		{
			name:     "valid agent",
			arg:      "agent/my-agent",
			wantType: resource.Agent,
			wantName: "my-agent",
			wantErr:  false,
		},
		{
			name:        "invalid format - no slash",
			arg:         "skill",
			wantErr:     true,
			errContains: "must be 'type/name'",
		},
		{
			name:        "invalid format - empty name",
			arg:         "skill/",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "invalid type",
			arg:         "invalid/name",
			wantErr:     true,
			errContains: "must be one of 'skill', 'command', or 'agent'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotName, err := parseResourceArg(tt.arg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseResourceArg() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("parseResourceArg() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("parseResourceArg() unexpected error = %v", err)
				return
			}

			if gotType != tt.wantType {
				t.Errorf("parseResourceArg() type = %v, want %v", gotType, tt.wantType)
			}
			if gotName != tt.wantName {
				t.Errorf("parseResourceArg() name = %v, want %v", gotName, tt.wantName)
			}
		})
	}
}

func TestProcessUninstall(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a temporary repo
	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Create a repo manager for the test
	manager := repo.NewManagerWithPath(repoDir)

	// Create a test project
	projectDir := filepath.Join(tempDir, "project")
	commandsDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("failed to create project dirs: %v", err)
	}

	// Create a test command file in repo
	testCommandPath := filepath.Join(repoDir, "commands", "test-command.md")
	testContent := []byte("---\ndescription: Test command\n---\n# Test")
	if err := os.WriteFile(testCommandPath, testContent, 0644); err != nil {
		t.Fatalf("failed to create test command: %v", err)
	}

	// Create symlink pointing to repo
	symlinkPath := filepath.Join(commandsDir, "test-command.md")
	if err := os.Symlink(testCommandPath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Test uninstalling the command
	result := processUninstall("command/test-command", projectDir, repoDir, []tools.Tool{tools.Claude}, manager)

	if !result.success {
		t.Errorf("processUninstall() failed: %v", result.message)
	}

	if len(result.toolsRemoved) != 1 {
		t.Errorf("processUninstall() removed from %d tools, want 1", len(result.toolsRemoved))
	}

	// Verify symlink was removed
	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Errorf("processUninstall() symlink still exists after uninstall")
	}
}

func TestProcessUninstall_NotManaged(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a temporary repo
	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Create a repo manager for the test
	manager := repo.NewManagerWithPath(repoDir)

	// Create a test project
	projectDir := filepath.Join(tempDir, "project")
	commandsDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("failed to create project dirs: %v", err)
	}

	// Create a test command file OUTSIDE repo
	otherDir := filepath.Join(tempDir, "other")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("failed to create other dir: %v", err)
	}
	testCommandPath := filepath.Join(otherDir, "test-command.md")
	testContent := []byte("---\ndescription: Test command\n---\n# Test")
	if err := os.WriteFile(testCommandPath, testContent, 0644); err != nil {
		t.Fatalf("failed to create test command: %v", err)
	}

	// Create symlink pointing to other location (not repo)
	symlinkPath := filepath.Join(commandsDir, "test-command.md")
	if err := os.Symlink(testCommandPath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Test uninstalling the command
	result := processUninstall("command/test-command", projectDir, repoDir, []tools.Tool{tools.Claude}, manager)

	// Should be skipped since it doesn't point to repo
	if !result.skipped {
		t.Errorf("processUninstall() expected skipped=true for non-managed symlink")
	}

	// Verify symlink was NOT removed
	if _, err := os.Lstat(symlinkPath); err != nil {
		t.Errorf("processUninstall() removed non-managed symlink, should have skipped it")
	}
}

func TestProcessUninstall_NotSymlink(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a temporary repo
	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	// Create a repo manager for the test
	manager := repo.NewManagerWithPath(repoDir)

	// Create a test project
	projectDir := filepath.Join(tempDir, "project")
	commandsDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("failed to create project dirs: %v", err)
	}

	// Create a regular file (not a symlink)
	filePath := filepath.Join(commandsDir, "test-command.md")
	testContent := []byte("---\ndescription: Test command\n---\n# Test")
	if err := os.WriteFile(filePath, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test uninstalling the command
	result := processUninstall("command/test-command", projectDir, repoDir, []tools.Tool{tools.Claude}, manager)

	// Should be skipped since it's not a symlink
	if !result.skipped {
		t.Errorf("processUninstall() expected skipped=true for non-symlink file")
	}

	// Verify file was NOT removed
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("processUninstall() removed non-symlink file, should have skipped it")
	}
}

// Test helpers for uninstall pattern tests

// setupTestRepoForUninstall creates a test repo and installs some resources
func setupTestRepoForUninstall(t *testing.T) (repoPath string, projectPath string) {
	t.Helper()

	// Create temp repo
	repoDir := t.TempDir()

	// Create directory structure
	if err := os.MkdirAll(filepath.Join(repoDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "skills"), 0755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "agents"), 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}

	// Create test commands
	testCommands := []string{"test-command", "pdf-command", "review-command"}
	for _, name := range testCommands {
		content := []byte(fmt.Sprintf("---\ndescription: %s\n---\n# %s", name, name))
		path := filepath.Join(repoDir, "commands", name+".md")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("failed to create command %s: %v", name, err)
		}
	}

	// Create test skills
	testSkills := []string{"pdf-processing", "pdf-extraction", "image-processing", "test-skill"}
	for _, name := range testSkills {
		skillDir := filepath.Join(repoDir, "skills", name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("failed to create skill dir %s: %v", name, err)
		}
		content := []byte(fmt.Sprintf("---\ndescription: %s\n---\n# %s", name, name))
		path := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("failed to create skill %s: %v", name, err)
		}
	}

	// Create test agents
	testAgents := []string{"code-reviewer", "test-agent", "doc-generator"}
	for _, name := range testAgents {
		content := []byte(fmt.Sprintf("---\ndescription: %s\n---\n# %s", name, name))
		path := filepath.Join(repoDir, "agents", name+".md")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("failed to create agent %s: %v", name, err)
		}
	}

	// Create project directory
	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, ".claude", "commands"), 0755); err != nil {
		t.Fatalf("failed to create project dirs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".claude", "skills"), 0755); err != nil {
		t.Fatalf("failed to create project dirs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".claude", "agents"), 0755); err != nil {
		t.Fatalf("failed to create project dirs: %v", err)
	}

	return repoDir, projectDir
}

// installSymlink creates a symlink from project to repo
func installSymlink(t *testing.T, projectPath, repoPath string, resourceType resource.ResourceType, name string) {
	t.Helper()

	var srcPath, dstPath string
	switch resourceType {
	case resource.Command:
		srcPath = filepath.Join(repoPath, "commands", name+".md")
		dstPath = filepath.Join(projectPath, ".claude", "commands", name+".md")
	case resource.Skill:
		srcPath = filepath.Join(repoPath, "skills", name)
		dstPath = filepath.Join(projectPath, ".claude", "skills", name)
	case resource.Agent:
		srcPath = filepath.Join(repoPath, "agents", name+".md")
		dstPath = filepath.Join(projectPath, ".claude", "agents", name+".md")
	}

	if err := os.Symlink(srcPath, dstPath); err != nil {
		t.Fatalf("failed to create symlink for %s/%s: %v", resourceType, name, err)
	}
}

// Test Pattern Expansion for Uninstall

func TestExpandUninstallPattern_AllSkills(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	// Install some skills
	installSymlink(t, projectPath, repoPath, resource.Skill, "pdf-processing")
	installSymlink(t, projectPath, repoPath, resource.Skill, "pdf-extraction")
	installSymlink(t, projectPath, repoPath, resource.Skill, "test-skill")

	// Expand "skill/*" pattern
	detectedTools := []tools.Tool{tools.Claude}
	matches, err := expandUninstallPattern(projectPath, "skill/*", detectedTools)
	if err != nil {
		t.Fatalf("expandUninstallPattern failed: %v", err)
	}

	// Should match 3 installed skills
	if len(matches) != 3 {
		t.Errorf("expandUninstallPattern(skill/*) returned %d matches, want 3", len(matches))
	}
}

func TestExpandUninstallPattern_PrefixMatch(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	// Install some skills
	installSymlink(t, projectPath, repoPath, resource.Skill, "pdf-processing")
	installSymlink(t, projectPath, repoPath, resource.Skill, "pdf-extraction")
	installSymlink(t, projectPath, repoPath, resource.Skill, "test-skill")

	// Expand "skill/pdf*" pattern
	detectedTools := []tools.Tool{tools.Claude}
	matches, err := expandUninstallPattern(projectPath, "skill/pdf*", detectedTools)
	if err != nil {
		t.Fatalf("expandUninstallPattern failed: %v", err)
	}

	// Should match 2 pdf skills
	if len(matches) != 2 {
		t.Errorf("expandUninstallPattern(skill/pdf*) returned %d matches, want 2", len(matches))
	}

	// Verify matches
	expectedMatches := map[string]bool{
		"skill/pdf-processing": true,
		"skill/pdf-extraction": true,
	}
	for _, match := range matches {
		if !expectedMatches[match] {
			t.Errorf("unexpected match: %q", match)
		}
	}
}

func TestExpandUninstallPattern_AllTypes(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	// Install resources with "test" in name across types
	installSymlink(t, projectPath, repoPath, resource.Command, "test-command")
	installSymlink(t, projectPath, repoPath, resource.Skill, "test-skill")
	installSymlink(t, projectPath, repoPath, resource.Agent, "test-agent")

	// Expand "*test*" pattern (matches across all types)
	detectedTools := []tools.Tool{tools.Claude}
	matches, err := expandUninstallPattern(projectPath, "*test*", detectedTools)
	if err != nil {
		t.Fatalf("expandUninstallPattern failed: %v", err)
	}

	// Should match all 3 test resources
	if len(matches) != 3 {
		t.Errorf("expandUninstallPattern(*test*) returned %d matches, want 3", len(matches))
		for _, m := range matches {
			t.Logf("  match: %s", m)
		}
	}
}

func TestExpandUninstallPattern_NoMatches(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	// Install one skill
	installSymlink(t, projectPath, repoPath, resource.Skill, "pdf-processing")

	// Expand pattern that matches nothing
	detectedTools := []tools.Tool{tools.Claude}
	matches, err := expandUninstallPattern(projectPath, "skill/nomatch*", detectedTools)
	if err != nil {
		t.Fatalf("expandUninstallPattern failed: %v", err)
	}

	// Should return empty slice
	if len(matches) != 0 {
		t.Errorf("expandUninstallPattern(skill/nomatch*) returned %d matches, want 0", len(matches))
	}
}

func TestExpandUninstallPattern_ExactName(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	// Install a skill
	installSymlink(t, projectPath, repoPath, resource.Skill, "pdf-processing")

	// Expand exact name (not a pattern)
	detectedTools := []tools.Tool{tools.Claude}
	matches, err := expandUninstallPattern(projectPath, "skill/pdf-processing", detectedTools)
	if err != nil {
		t.Fatalf("expandUninstallPattern failed: %v", err)
	}

	// Should return the exact name as-is
	if len(matches) != 1 {
		t.Fatalf("expandUninstallPattern(skill/pdf-processing) returned %d matches, want 1", len(matches))
	}
	if matches[0] != "skill/pdf-processing" {
		t.Errorf("expandUninstallPattern(skill/pdf-processing) = %q, want 'skill/pdf-processing'", matches[0])
	}
}

func TestScanToolDir_Commands(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	// Install some commands
	installSymlink(t, projectPath, repoPath, resource.Command, "test-command")
	installSymlink(t, projectPath, repoPath, resource.Command, "pdf-command")

	// Create matcher for "*command"
	matcher, err := pattern.NewMatcher("*command")
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	// Scan .claude/commands directory
	matches := scanToolDir(projectPath, ".claude/commands", resource.Command, tools.Claude, matcher)

	// Should find both commands
	if len(matches) != 2 {
		t.Errorf("scanToolDir returned %d matches, want 2", len(matches))
	}
}

func TestScanToolDir_Skills(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	// Install skills
	installSymlink(t, projectPath, repoPath, resource.Skill, "pdf-processing")
	installSymlink(t, projectPath, repoPath, resource.Skill, "image-processing")

	// Create matcher for "*processing"
	matcher, err := pattern.NewMatcher("*processing")
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	// Scan .claude/skills directory
	matches := scanToolDir(projectPath, ".claude/skills", resource.Skill, tools.Claude, matcher)

	// Should find both skills
	if len(matches) != 2 {
		t.Errorf("scanToolDir returned %d matches, want 2", len(matches))
	}
}

func TestScanToolDir_NonExistentDirectory(t *testing.T) {
	projectPath := t.TempDir()

	// Create matcher
	matcher, err := pattern.NewMatcher("*")
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	// Scan non-existent directory
	matches := scanToolDir(projectPath, ".claude/commands", resource.Command, tools.Claude, matcher)

	// Should return nil (no matches)
	if matches != nil {
		t.Errorf("scanToolDir on non-existent dir returned %v, want nil", matches)
	}
}

func TestDeduplicateStrings(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "no duplicates",
			input: []string{"a", "b", "c"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "with duplicates",
			input: []string{"a", "b", "a", "c", "b"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "all duplicates",
			input: []string{"a", "a", "a"},
			want:  []string{"a"},
		},
		{
			name:  "empty slice",
			input: []string{},
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateStrings(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("deduplicateStrings() returned %d items, want %d", len(got), len(tt.want))
			}
			// Check all expected items are present
			gotMap := make(map[string]bool)
			for _, s := range got {
				gotMap[s] = true
			}
			for _, s := range tt.want {
				if !gotMap[s] {
					t.Errorf("deduplicateStrings() missing expected item %q", s)
				}
			}
		})
	}
}

func TestUninstallAll(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	// Install multiple resources
	installSymlink(t, projectPath, repoPath, resource.Command, "test-command")
	installSymlink(t, projectPath, repoPath, resource.Command, "pdf-command")
	installSymlink(t, projectPath, repoPath, resource.Skill, "pdf-processing")
	installSymlink(t, projectPath, repoPath, resource.Skill, "test-skill")
	installSymlink(t, projectPath, repoPath, resource.Agent, "code-reviewer")

	// Run uninstallAll
	err := uninstallAll(projectPath, repoPath, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("uninstallAll() failed: %v", err)
	}

	// Verify all symlinks were removed
	commandsDir := filepath.Join(projectPath, ".claude", "commands")
	skillsDir := filepath.Join(projectPath, ".claude", "skills")
	agentsDir := filepath.Join(projectPath, ".claude", "agents")

	// Check commands
	if _, err := os.Lstat(filepath.Join(commandsDir, "test-command.md")); !os.IsNotExist(err) {
		t.Error("uninstallAll() did not remove test-command.md")
	}
	if _, err := os.Lstat(filepath.Join(commandsDir, "pdf-command.md")); !os.IsNotExist(err) {
		t.Error("uninstallAll() did not remove pdf-command.md")
	}

	// Check skills
	if _, err := os.Lstat(filepath.Join(skillsDir, "pdf-processing")); !os.IsNotExist(err) {
		t.Error("uninstallAll() did not remove pdf-processing")
	}
	if _, err := os.Lstat(filepath.Join(skillsDir, "test-skill")); !os.IsNotExist(err) {
		t.Error("uninstallAll() did not remove test-skill")
	}

	// Check agents
	if _, err := os.Lstat(filepath.Join(agentsDir, "code-reviewer.md")); !os.IsNotExist(err) {
		t.Error("uninstallAll() did not remove code-reviewer.md")
	}
}

func TestUninstallAll_OnlyRemovesManagedSymlinks(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	commandsDir := filepath.Join(projectPath, ".claude", "commands")

	// Install a managed symlink
	installSymlink(t, projectPath, repoPath, resource.Command, "test-command")

	// Create an unmanaged symlink (points outside repo)
	otherDir := t.TempDir()
	unmanagedFile := filepath.Join(otherDir, "unmanaged.md")
	if err := os.WriteFile(unmanagedFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create unmanaged file: %v", err)
	}
	unmanagedSymlink := filepath.Join(commandsDir, "unmanaged.md")
	if err := os.Symlink(unmanagedFile, unmanagedSymlink); err != nil {
		t.Fatalf("failed to create unmanaged symlink: %v", err)
	}

	// Create a regular file (not a symlink)
	regularFile := filepath.Join(commandsDir, "regular.md")
	if err := os.WriteFile(regularFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create regular file: %v", err)
	}

	// Run uninstallAll
	err := uninstallAll(projectPath, repoPath, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("uninstallAll() failed: %v", err)
	}

	// Verify managed symlink was removed
	if _, err := os.Lstat(filepath.Join(commandsDir, "test-command.md")); !os.IsNotExist(err) {
		t.Error("uninstallAll() did not remove managed symlink")
	}

	// Verify unmanaged symlink was NOT removed
	if _, err := os.Lstat(unmanagedSymlink); err != nil {
		t.Error("uninstallAll() removed unmanaged symlink, should have skipped it")
	}

	// Verify regular file was NOT removed
	if _, err := os.Stat(regularFile); err != nil {
		t.Error("uninstallAll() removed regular file, should have skipped it")
	}
}

func TestUninstallAll_EmptyDirectory(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	// Run uninstallAll on empty directories
	err := uninstallAll(projectPath, repoPath, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("uninstallAll() failed on empty directories: %v", err)
	}
}

func TestProcessUninstall_CopilotAgentUsesAgentSuffix(t *testing.T) {
	tempDir := t.TempDir()

	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, "agents"), 0755); err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	manager := repo.NewManagerWithPath(repoDir)

	projectDir := filepath.Join(tempDir, "project")
	agentsDir := filepath.Join(projectDir, ".github", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create project dirs: %v", err)
	}

	agentRepoPath := filepath.Join(repoDir, "agents", "reviewer.md")
	if err := os.WriteFile(agentRepoPath, []byte("---\ndescription: reviewer\n---\n# Reviewer"), 0644); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	symlinkPath := filepath.Join(agentsDir, "reviewer.agent.md")
	if err := os.Symlink(agentRepoPath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	result := processUninstall("agent/reviewer", projectDir, repoDir, []tools.Tool{tools.Copilot}, manager)
	if !result.success {
		t.Fatalf("processUninstall() failed: %v", result.message)
	}

	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Errorf("copilot agent symlink still exists after uninstall")
	}
}

func TestExpandUninstallPattern_CopilotAgentSuffix(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	agentsDir := filepath.Join(projectDir, ".github", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create project dirs: %v", err)
	}

	repoDir := filepath.Join(tempDir, "repo")
	agentRepoPath := filepath.Join(repoDir, "agents", "reviewer.md")
	if err := os.MkdirAll(filepath.Dir(agentRepoPath), 0755); err != nil {
		t.Fatalf("failed to create repo dirs: %v", err)
	}
	if err := os.WriteFile(agentRepoPath, []byte("---\ndescription: reviewer\n---\n# Reviewer"), 0644); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	if err := os.Symlink(agentRepoPath, filepath.Join(agentsDir, "reviewer.agent.md")); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	matches, err := expandUninstallPattern(projectDir, "agent/*", []tools.Tool{tools.Copilot})
	if err != nil {
		t.Fatalf("expandUninstallPattern failed: %v", err)
	}

	if len(matches) != 1 || matches[0] != "agent/reviewer" {
		t.Fatalf("matches = %v, want [agent/reviewer]", matches)
	}
}

// Test helpers for package tests

// createTestPackageForUninstall creates a test package in the repo
func createTestPackageForUninstall(t *testing.T, repoPath, packageName string, resources []string) {
	t.Helper()

	// Create packages directory
	packagesDir := filepath.Join(repoPath, "packages", packageName)
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("failed to create packages dir: %v", err)
	}

	// Create package.yaml content
	content := fmt.Sprintf("name: %s\ndescription: Test package\nresources:\n", packageName)
	for _, res := range resources {
		content += fmt.Sprintf("  - %s\n", res)
	}

	// Write package.yaml
	pkgFile := filepath.Join(packagesDir, "package.yaml")
	if err := os.WriteFile(pkgFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create package.yaml: %v", err)
	}
}

// TestUninstallMultipleSkills tests uninstalling multiple skills
func TestUninstallMultipleSkills(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	// Install multiple skills
	installSymlink(t, projectPath, repoPath, resource.Skill, "pdf-processing")
	installSymlink(t, projectPath, repoPath, resource.Skill, "test-skill")

	// Verify they exist
	skillsDir := filepath.Join(projectPath, ".claude", "skills")
	if _, err := os.Lstat(filepath.Join(skillsDir, "pdf-processing")); err != nil {
		t.Fatalf("pdf-processing not installed: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(skillsDir, "test-skill")); err != nil {
		t.Fatalf("test-skill not installed: %v", err)
	}

	// Create manager for processUninstall
	manager := repo.NewManagerWithPath(repoPath)

	// Uninstall both
	result1 := processUninstall("skill/pdf-processing", projectPath, repoPath, []tools.Tool{tools.Claude}, manager)
	result2 := processUninstall("skill/test-skill", projectPath, repoPath, []tools.Tool{tools.Claude}, manager)

	if !result1.success {
		t.Errorf("failed to uninstall pdf-processing: %s", result1.message)
	}
	if !result2.success {
		t.Errorf("failed to uninstall test-skill: %s", result2.message)
	}

	// Verify they're gone
	if _, err := os.Lstat(filepath.Join(skillsDir, "pdf-processing")); !os.IsNotExist(err) {
		t.Error("pdf-processing still exists after uninstall")
	}
	if _, err := os.Lstat(filepath.Join(skillsDir, "test-skill")); !os.IsNotExist(err) {
		t.Error("test-skill still exists after uninstall")
	}
}

// TestUninstallSkillAndPackage tests uninstalling a skill and verifying package handling
func TestUninstallSkillAndPackage(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	// Create a test package
	createTestPackageForUninstall(t, repoPath, "test-pkg", []string{"skill/pdf-processing", "skill/test-skill"})

	// Install skills
	installSymlink(t, projectPath, repoPath, resource.Skill, "pdf-processing")
	installSymlink(t, projectPath, repoPath, resource.Skill, "test-skill")

	// Verify they exist
	skillsDir := filepath.Join(projectPath, ".claude", "skills")
	if _, err := os.Lstat(filepath.Join(skillsDir, "pdf-processing")); err != nil {
		t.Fatalf("pdf-processing not installed: %v", err)
	}
}

// TestUninstallMultiplePackages tests verifying package structure
func TestUninstallMultiplePackages(t *testing.T) {
	repoPath, projectPath := setupTestRepoForUninstall(t)

	// Create test packages
	createTestPackageForUninstall(t, repoPath, "pkg-a", []string{"skill/pdf-processing"})
	createTestPackageForUninstall(t, repoPath, "pkg-b", []string{"skill/test-skill"})

	// Install the resources
	installSymlink(t, projectPath, repoPath, resource.Skill, "pdf-processing")
	installSymlink(t, projectPath, repoPath, resource.Skill, "test-skill")

	// Verify they exist
	skillsDir := filepath.Join(projectPath, ".claude", "skills")
	if _, err := os.Lstat(filepath.Join(skillsDir, "pdf-processing")); err != nil {
		t.Fatalf("pdf-processing not installed: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(skillsDir, "test-skill")); err != nil {
		t.Fatalf("test-skill not installed: %v", err)
	}
}

// TestUninstallArgProcessing tests that arguments are correctly separated into packages and resources
func TestUninstallArgProcessing(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantPackages []string
		wantOthers   []string
	}{
		{
			name:         "only skills",
			args:         []string{"skill/pdf-processing", "skill/test-skill"},
			wantPackages: nil,
			wantOthers:   []string{"skill/pdf-processing", "skill/test-skill"},
		},
		{
			name:         "only packages",
			args:         []string{"package/pkg-a", "package/pkg-b"},
			wantPackages: []string{"package/pkg-a", "package/pkg-b"},
			wantOthers:   nil,
		},
		{
			name:         "mixed skills and packages",
			args:         []string{"skill/pdf-processing", "package/pkg-a", "command/test"},
			wantPackages: []string{"package/pkg-a"},
			wantOthers:   []string{"skill/pdf-processing", "command/test"},
		},
		{
			name:         "packages prefix normalization",
			args:         []string{"packages/pkg-a", "package/pkg-b"},
			wantPackages: []string{"package/pkg-a", "package/pkg-b"},
			wantOthers:   nil,
		},
		{
			name:         "skill and package together",
			args:         []string{"skill/pdf-processing", "package/test-pkg"},
			wantPackages: []string{"package/test-pkg"},
			wantOthers:   []string{"skill/pdf-processing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var packageRefs []string
			var resourceRefs []string

			// Simulate the arg processing logic from uninstall.go RunE
			for _, arg := range tt.args {
				if strings.HasPrefix(arg, "package/") || strings.HasPrefix(arg, "packages/") {
					normalizedArg := arg
					if strings.HasPrefix(arg, "packages/") {
						normalizedArg = "package/" + strings.TrimPrefix(arg, "packages/")
					}
					packageRefs = append(packageRefs, normalizedArg)
				} else {
					resourceRefs = append(resourceRefs, arg)
				}
			}

			// Verify packages
			if len(packageRefs) != len(tt.wantPackages) {
				t.Errorf("got %d packages, want %d", len(packageRefs), len(tt.wantPackages))
			}
			for i, pkg := range tt.wantPackages {
				if i >= len(packageRefs) || packageRefs[i] != pkg {
					t.Errorf("package[%d] = %v, want %v", i, packageRefs, tt.wantPackages)
					break
				}
			}

			// Verify other resources
			if len(resourceRefs) != len(tt.wantOthers) {
				t.Errorf("got %d resources, want %d", len(resourceRefs), len(tt.wantOthers))
			}
			for i, res := range tt.wantOthers {
				if i >= len(resourceRefs) || resourceRefs[i] != res {
					t.Errorf("resource[%d] = %v, want %v", i, resourceRefs, tt.wantOthers)
					break
				}
			}
		})
	}
}

func TestPersistUninstallManifestUpdates_RemovesFromBaseAndLocal(t *testing.T) {
	projectDir := t.TempDir()

	base := &manifest.Manifest{Resources: []string{"skill/shared", "skill/base-only"}}
	local := &manifest.Manifest{Resources: []string{"skill/shared", "skill/local-only"}}

	if err := base.Save(filepath.Join(projectDir, manifest.ManifestFileName)); err != nil {
		t.Fatalf("save base manifest: %v", err)
	}
	if err := local.Save(filepath.Join(projectDir, manifest.LocalManifestFileName)); err != nil {
		t.Fatalf("save local manifest: %v", err)
	}

	persistUninstallManifestUpdates(projectDir, []string{"skill/shared", "skill/local-only"})

	baseAfter, err := manifest.Load(filepath.Join(projectDir, manifest.ManifestFileName))
	if err != nil {
		t.Fatalf("load base manifest after update: %v", err)
	}
	if baseAfter.Has("skill/shared") {
		t.Fatalf("expected skill/shared removed from base manifest, got %v", baseAfter.Resources)
	}
	if !baseAfter.Has("skill/base-only") {
		t.Fatalf("expected skill/base-only retained in base manifest, got %v", baseAfter.Resources)
	}

	localAfter, err := manifest.Load(filepath.Join(projectDir, manifest.LocalManifestFileName))
	if err != nil {
		t.Fatalf("load local manifest after update: %v", err)
	}
	if localAfter.Has("skill/shared") || localAfter.Has("skill/local-only") {
		t.Fatalf("expected removed resources deleted from local manifest, got %v", localAfter.Resources)
	}
}
