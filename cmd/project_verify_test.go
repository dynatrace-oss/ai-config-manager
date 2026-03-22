package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

func TestScanProjectIssues(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(string, string) error
		expectedCount int
		expectedType  string
	}{
		{
			name: "detects broken symlinks",
			setupFunc: func(projectDir, repoDir string) error {
				claudeDir := filepath.Join(projectDir, ".claude", "commands")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					return err
				}
				// Create symlink to non-existent target
				target := filepath.Join(repoDir, "commands", "missing-cmd")
				return os.Symlink(target, filepath.Join(claudeDir, "missing-cmd"))
			},
			expectedCount: 1,
			expectedType:  "broken",
		},
		{
			name: "detects symlinks pointing to wrong repo",
			setupFunc: func(projectDir, repoDir string) error {
				claudeDir := filepath.Join(projectDir, ".claude", "commands")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					return err
				}
				// Create symlink to different repo (use a subdir of projectDir to avoid shared tmp issues)
				wrongRepo := filepath.Join(projectDir, "wrong-repo")
				wrongRepoCommands := filepath.Join(wrongRepo, "commands")
				if err := os.MkdirAll(wrongRepoCommands, 0755); err != nil {
					return err
				}
				target := filepath.Join(wrongRepoCommands, "test-cmd")
				if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(target, filepath.Join(claudeDir, "test-cmd"))
			},
			expectedCount: 1,
			expectedType:  "wrong-repo",
		},
		{
			name: "no issues with valid symlinks",
			setupFunc: func(projectDir, repoDir string) error {
				claudeDir := filepath.Join(projectDir, ".claude", "commands")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					return err
				}
				// Create valid symlink
				target := filepath.Join(repoDir, "commands", "test-cmd")
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(target, filepath.Join(claudeDir, "test-cmd"))
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			repoDir := t.TempDir()

			// Setup test scenario
			if err := tt.setupFunc(projectDir, repoDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			// Detect tools
			detectedTools, err := tools.DetectExistingTools(projectDir)
			if err != nil {
				t.Fatalf("Failed to detect tools: %v", err)
			}

			// Scan for issues
			issues, err := scanProjectIssues(projectDir, detectedTools, repoDir)
			if err != nil {
				t.Fatalf("scanProjectIssues failed: %v", err)
			}

			if len(issues) != tt.expectedCount {
				t.Errorf("Expected %d issues, got %d", tt.expectedCount, len(issues))
			}

			if tt.expectedCount > 0 && len(issues) > 0 {
				if issues[0].IssueType != tt.expectedType {
					t.Errorf("Issue type = %v, want %v", issues[0].IssueType, tt.expectedType)
				}
			}
		})
	}
}

func TestVerifyDirectory(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(string, string) error
		expectedCount int
	}{
		{
			name: "detects broken symlink",
			setupFunc: func(dir, repoPath string) error {
				target := filepath.Join(repoPath, "commands", "missing-cmd")
				return os.Symlink(target, filepath.Join(dir, "missing-cmd"))
			},
			expectedCount: 1,
		},
		{
			name: "valid symlink has no issues",
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
			name: "detects broken symlink in nested directory",
			setupFunc: func(dir, repoPath string) error {
				nsDir := filepath.Join(dir, "test-ns")
				if err := os.MkdirAll(nsDir, 0755); err != nil {
					return err
				}
				target := filepath.Join(repoPath, "commands", "test-ns", "broken-cmd.md")
				return os.Symlink(target, filepath.Join(nsDir, "broken-cmd.md"))
			},
			expectedCount: 1,
		},
		{
			name: "detects wrong-repo symlink in nested directory",
			setupFunc: func(dir, repoPath string) error {
				nsDir := filepath.Join(dir, "test-ns")
				if err := os.MkdirAll(nsDir, 0755); err != nil {
					return err
				}
				// Create a valid target outside the repo
				wrongDir := filepath.Join(dir, ".wrong-repo", "commands", "test-ns")
				if err := os.MkdirAll(wrongDir, 0755); err != nil {
					return err
				}
				wrongTarget := filepath.Join(wrongDir, "cmd.md")
				if err := os.WriteFile(wrongTarget, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(wrongTarget, filepath.Join(nsDir, "cmd.md"))
			},
			expectedCount: 1,
		},
		{
			name: "valid symlink in nested directory has no issues",
			setupFunc: func(dir, repoPath string) error {
				nsDir := filepath.Join(dir, "test-ns")
				if err := os.MkdirAll(nsDir, 0755); err != nil {
					return err
				}
				target := filepath.Join(repoPath, "commands", "test-ns", "good-cmd.md")
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(target, filepath.Join(nsDir, "good-cmd.md"))
			},
			expectedCount: 0,
		},
		{
			name: "ignores regular files in nested directory",
			setupFunc: func(dir, repoPath string) error {
				nsDir := filepath.Join(dir, "test-ns")
				if err := os.MkdirAll(nsDir, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(nsDir, "regular-file.md"), []byte("test"), 0644)
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			repoDir := t.TempDir()

			testDir := filepath.Join(dir, "commands")
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			// Setup test scenario
			if err := tt.setupFunc(testDir, repoDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			// Verify directory
			issues, err := verifyDirectory(testDir, tools.Claude, repoDir)
			if err != nil {
				t.Fatalf("verifyDirectory failed: %v", err)
			}

			if len(issues) != tt.expectedCount {
				t.Errorf("Expected %d issues, got %d", tt.expectedCount, len(issues))
			}
		})
	}
}

// TestVerifyDirectory_NestedCommands tests that verifyDirectory correctly
// recurses into subdirectories to find broken/valid nested command symlinks.
func TestVerifyDirectory_NestedCommands(t *testing.T) {
	tests := []struct {
		name             string
		setupFunc        func(string, string) error
		expectedCount    int
		expectedType     string
		expectedResource string
	}{
		{
			name: "detects broken nested symlink",
			setupFunc: func(dir, repoPath string) error {
				subDir := filepath.Join(dir, "api")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					return err
				}
				target := filepath.Join(repoPath, "commands", "api", "deploy.md")
				return os.Symlink(target, filepath.Join(subDir, "deploy.md"))
			},
			expectedCount:    1,
			expectedType:     "broken",
			expectedResource: "api/deploy",
		},
		{
			name: "valid nested symlink has no issues",
			setupFunc: func(dir, repoPath string) error {
				subDir := filepath.Join(dir, "api")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					return err
				}
				target := filepath.Join(repoPath, "commands", "api", "deploy.md")
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(target, filepath.Join(subDir, "deploy.md"))
			},
			expectedCount: 0,
		},
		{
			name: "detects wrong-repo nested symlink",
			setupFunc: func(dir, repoPath string) error {
				subDir := filepath.Join(dir, "api")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					return err
				}
				// Create a target outside the repo
				wrongTarget := filepath.Join(dir, "wrong-repo", "commands", "api", "deploy.md")
				if err := os.MkdirAll(filepath.Dir(wrongTarget), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(wrongTarget, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(wrongTarget, filepath.Join(subDir, "deploy.md"))
			},
			expectedCount:    1,
			expectedType:     "wrong-repo",
			expectedResource: "api/deploy",
		},
		{
			name: "ignores regular files in subdirectory",
			setupFunc: func(dir, repoPath string) error {
				subDir := filepath.Join(dir, "api")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(subDir, "not-a-symlink.md"), []byte("test"), 0644)
			},
			expectedCount: 0,
		},
		{
			name: "does not recurse deeper than one level",
			setupFunc: func(dir, repoPath string) error {
				deepDir := filepath.Join(dir, "level1", "level2")
				if err := os.MkdirAll(deepDir, 0755); err != nil {
					return err
				}
				target := filepath.Join(repoPath, "commands", "missing.md")
				return os.Symlink(target, filepath.Join(deepDir, "deep.md"))
			},
			expectedCount: 0, // Should NOT find the deeply nested symlink
		},
		{
			name: "detects multiple broken nested symlinks",
			setupFunc: func(dir, repoPath string) error {
				subDir := filepath.Join(dir, "api")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					return err
				}
				target1 := filepath.Join(repoPath, "commands", "api", "deploy.md")
				if err := os.Symlink(target1, filepath.Join(subDir, "deploy.md")); err != nil {
					return err
				}
				target2 := filepath.Join(repoPath, "commands", "api", "status.md")
				return os.Symlink(target2, filepath.Join(subDir, "status.md"))
			},
			expectedCount: 2,
			expectedType:  "broken",
		},
		{
			name: "mixes top-level and nested symlinks",
			setupFunc: func(dir, repoPath string) error {
				// Top-level broken symlink
				topTarget := filepath.Join(repoPath, "commands", "top-cmd")
				if err := os.Symlink(topTarget, filepath.Join(dir, "top-cmd")); err != nil {
					return err
				}
				// Nested broken symlink
				subDir := filepath.Join(dir, "api")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					return err
				}
				nestedTarget := filepath.Join(repoPath, "commands", "api", "deploy.md")
				return os.Symlink(nestedTarget, filepath.Join(subDir, "deploy.md"))
			},
			expectedCount: 2,
			expectedType:  "broken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			repoDir := t.TempDir()

			testDir := filepath.Join(dir, "commands")
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			if err := tt.setupFunc(testDir, repoDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			issues, err := verifyDirectory(testDir, tools.Claude, repoDir)
			if err != nil {
				t.Fatalf("verifyDirectory failed: %v", err)
			}

			if len(issues) != tt.expectedCount {
				t.Errorf("Expected %d issues, got %d: %+v", tt.expectedCount, len(issues), issues)
			}

			if tt.expectedCount > 0 && len(issues) > 0 {
				if tt.expectedType != "" && issues[0].IssueType != tt.expectedType {
					t.Errorf("Issue type = %v, want %v", issues[0].IssueType, tt.expectedType)
				}
				if tt.expectedResource != "" && issues[0].Resource != tt.expectedResource {
					t.Errorf("Resource = %v, want %v", issues[0].Resource, tt.expectedResource)
				}
			}
		})
	}
}

func TestCheckManifestSync(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(string, string) error
		expectedCount int
	}{
		{
			name: "detects resource in manifest but not installed",
			setupFunc: func(projectDir, repoDir string) error {
				// Create manifest with a resource
				m := &manifest.Manifest{
					Resources: []string{"skill/test-skill"},
				}
				manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
				return m.Save(manifestPath)
			},
			expectedCount: 1,
		},
		{
			name: "no issues when resource is installed",
			setupFunc: func(projectDir, repoDir string) error {
				// Create manifest
				m := &manifest.Manifest{
					Resources: []string{"skill/test-skill"},
				}
				manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
				if err := m.Save(manifestPath); err != nil {
					return err
				}

				// Create installed resource
				claudeDir := filepath.Join(projectDir, ".claude", "skills")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					return err
				}

				// Create target in repo
				target := filepath.Join(repoDir, "skills", "test-skill")
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("test"), 0644); err != nil {
					return err
				}

				// Create symlink
				return os.Symlink(target, filepath.Join(claudeDir, "test-skill"))
			},
			expectedCount: 0,
		},
		{
			name: "no issues when no manifest exists",
			setupFunc: func(projectDir, repoDir string) error {
				// Don't create manifest
				return nil
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			repoDir := t.TempDir()

			// Setup test scenario
			if err := tt.setupFunc(projectDir, repoDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			// Detect tools
			detectedTools, err := tools.DetectExistingTools(projectDir)
			if err != nil {
				t.Fatalf("Failed to detect tools: %v", err)
			}

			// Check manifest sync
			issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
			if err != nil {
				t.Fatalf("checkManifestSync failed: %v", err)
			}

			if len(issues) != tt.expectedCount {
				t.Errorf("Expected %d issues, got %d", tt.expectedCount, len(issues))
			}

			if tt.expectedCount > 0 && len(issues) > 0 {
				if issues[0].IssueType != "not-installed" {
					t.Errorf("Issue type = %v, want not-installed", issues[0].IssueType)
				}
			}
		})
	}
}

func TestProjectVerifyCommand(t *testing.T) {
	// Create temp directories
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

	// Create tool directory with valid symlink
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

	// Change to project directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Failed to change to project directory: %v", err)
	}

	// Run verify command
	err = projectVerifyCmd.RunE(projectVerifyCmd, []string{})
	if err != nil {
		t.Errorf("Verify command failed: %v", err)
	}
}

func TestDisplayVerifyIssues(t *testing.T) {
	issues := []VerifyIssue{
		{
			Resource:    "test-cmd",
			Tool:        "claude",
			IssueType:   "broken",
			Description: "Symlink target doesn't exist",
			Severity:    "error",
		},
		{
			Resource:    "test-skill",
			Tool:        "opencode",
			IssueType:   "wrong-repo",
			Description: "Points to wrong repo",
			Severity:    "warning",
		},
	}

	// Just verify it doesn't panic and returns no error
	err := displayVerifyIssues(issues, output.Table)
	if err != nil {
		t.Errorf("displayVerifyIssues failed: %v", err)
	}
}

func TestVerifyIssueTypes(t *testing.T) {
	issueTypes := []string{"broken", "wrong-repo", "not-installed", "undeclared", "unreadable"}

	for _, issueType := range issueTypes {
		t.Run(issueType, func(t *testing.T) {
			issue := VerifyIssue{
				Resource:    "test-resource",
				Tool:        "test-tool",
				IssueType:   issueType,
				Description: "Test issue",
			}

			if issue.IssueType != issueType {
				t.Errorf("IssueType = %v, want %v", issue.IssueType, issueType)
			}
		})
	}
}

// TestScanProjectIssues_NestedCommands verifies that scanProjectIssues
// detects broken symlinks inside subdirectories of the commands directory.
func TestScanProjectIssues_NestedCommands(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create commands directory with a nested broken symlink
	claudeDir := filepath.Join(projectDir, ".claude", "commands", "api")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create nested commands directory: %v", err)
	}
	target := filepath.Join(repoDir, "commands", "api", "deploy.md")
	if err := os.Symlink(target, filepath.Join(claudeDir, "deploy.md")); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	// Detect tools
	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	// Scan for issues
	issues, err := scanProjectIssues(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("scanProjectIssues failed: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].IssueType != "broken" {
		t.Errorf("Issue type = %v, want broken", issues[0].IssueType)
	}
	if issues[0].Resource != "api/deploy" {
		t.Errorf("Resource = %v, want api/deploy", issues[0].Resource)
	}
}

// TestProjectVerifyFixUsesRepairReconcile verifies that verify --fix uses the
// repair/reconcile flow to remove undeclared resources while still warning that
// --fix is deprecated.
func TestProjectVerifyFixUsesRepairReconcile(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	oldEnv := os.Getenv("AIMGR_REPO_PATH")
	defer func() {
		if oldEnv != "" {
			_ = os.Setenv("AIMGR_REPO_PATH", oldEnv)
		} else {
			_ = os.Unsetenv("AIMGR_REPO_PATH")
		}
	}()
	_ = os.Setenv("AIMGR_REPO_PATH", repoDir)

	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Empty manifest means any installed content in owned dirs is undeclared.
	m := &manifest.Manifest{Resources: []string{}}
	if err := m.Save(filepath.Join(projectDir, manifest.ManifestFileName)); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	commandsDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}
	undeclaredPath := filepath.Join(commandsDir, "orphan.md")
	if err := os.WriteFile(undeclaredPath, []byte("orphan"), 0644); err != nil {
		t.Fatalf("Failed to write undeclared file: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Failed to change to project directory: %v", err)
	}

	oldFix := verifyFixFlag
	verifyFixFlag = true
	defer func() { verifyFixFlag = oldFix }()

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err = projectVerifyCmd.RunE(projectVerifyCmd, []string{})

	w.Close()
	os.Stderr = oldStderr
	captured := make([]byte, 4096)
	n, _ := r.Read(captured)
	stderrOutput := string(captured[:n])

	if err != nil {
		t.Fatalf("verify --fix failed: %v", err)
	}
	if !strings.Contains(stderrOutput, "Warning: --fix is deprecated. Running 'aimgr repair' reconciliation.") {
		t.Errorf("Expected deprecation warning on stderr, got: %q", stderrOutput)
	}
	if _, statErr := os.Stat(undeclaredPath); !os.IsNotExist(statErr) {
		t.Fatalf("Expected undeclared path to be removed by reconcile, stat err: %v", statErr)
	}
}

// TestCheckManifestSync_PackageListsMissingResources verifies that when a
// package has missing resources, the issue description names each one.
func TestCheckManifestSync_PackageListsMissingResources(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create a package definition in the repo with 3 resources
	pkg := &resource.Package{
		Name:        "test-pkg",
		Description: "A test package",
		Resources:   []string{"skill/installed-skill", "agent/missing-agent", "command/missing-cmd"},
	}
	if err := resource.SavePackage(pkg, repoDir); err != nil {
		t.Fatalf("Failed to save package: %v", err)
	}

	// Create manifest referencing the package
	m := &manifest.Manifest{
		Resources: []string{"package/test-pkg"},
	}
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Create tool directory and install only one of the three resources
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	// Create the installed skill target in repo
	repoSkillDir := filepath.Join(repoDir, "skills", "installed-skill")
	if err := os.MkdirAll(repoSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create repo skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoSkillDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := os.Symlink(repoSkillDir, filepath.Join(skillsDir, "installed-skill")); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Also create agents dir (needed for opencode tool detection, but leave agent missing)
	agentsDir := filepath.Join(projectDir, ".opencode", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}

	// Also create commands dir (needed for opencode tool detection, but leave command missing)
	commandsDir := filepath.Join(projectDir, ".opencode", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}

	// Detect tools
	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	// Check manifest sync
	issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d: %+v", len(issues), issues)
	}

	issue := issues[0]
	if issue.Resource != "package/test-pkg" {
		t.Errorf("Expected resource 'package/test-pkg', got %q", issue.Resource)
	}
	if issue.IssueType != "not-installed" {
		t.Errorf("Expected issue type 'not-installed', got %q", issue.IssueType)
	}

	// Verify the description includes the count and the missing resource names
	desc := issue.Description
	if !strings.Contains(desc, "2 resource(s) not installed") {
		t.Errorf("Expected description to contain '2 resource(s) not installed', got: %s", desc)
	}
	if !strings.Contains(desc, "agent/missing-agent") {
		t.Errorf("Expected description to contain 'agent/missing-agent', got: %s", desc)
	}
	if !strings.Contains(desc, "command/missing-cmd") {
		t.Errorf("Expected description to contain 'command/missing-cmd', got: %s", desc)
	}
	// Verify installed resource is NOT in the missing list
	if strings.Contains(desc, "installed-skill") {
		t.Errorf("Description should NOT contain 'installed-skill' (it's installed), got: %s", desc)
	}
}

// TestCheckManifestSync_PackageAllInstalledNoIssue verifies that a package
// with all resources installed produces no issues.
func TestCheckManifestSync_PackageAllInstalledNoIssue(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create a package definition with one resource
	pkg := &resource.Package{
		Name:        "complete-pkg",
		Description: "A fully installed package",
		Resources:   []string{"skill/good-skill"},
	}
	if err := resource.SavePackage(pkg, repoDir); err != nil {
		t.Fatalf("Failed to save package: %v", err)
	}

	// Create manifest
	m := &manifest.Manifest{
		Resources: []string{"package/complete-pkg"},
	}
	if err := m.Save(filepath.Join(projectDir, manifest.ManifestFileName)); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Install the skill
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	repoSkillDir := filepath.Join(repoDir, "skills", "good-skill")
	if err := os.MkdirAll(repoSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create repo skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoSkillDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := os.Symlink(repoSkillDir, filepath.Join(skillsDir, "good-skill")); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues for fully installed package, got %d: %+v", len(issues), issues)
	}
}

// TestCheckManifestSync_DetectsBrokenSymlinkAsNotInstalled verifies that
// checkManifestSync treats a broken symlink as "not installed" since os.Stat
// correctly follows the symlink and fails when the target doesn't exist.
func TestCheckManifestSync_DetectsBrokenSymlinkAsNotInstalled(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create manifest referencing a skill
	m := &manifest.Manifest{
		Resources: []string{"skill/broken-skill"},
	}
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Create tool directory with a broken symlink (target doesn't exist)
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	brokenTarget := filepath.Join(repoDir, "skills", "broken-skill")
	if err := os.Symlink(brokenTarget, filepath.Join(skillsDir, "broken-skill")); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	// Detect tools
	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	// Check manifest sync — should detect the broken symlink as "not-installed"
	issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].IssueType != "not-installed" {
		t.Errorf("Expected issue type 'not-installed', got %q", issues[0].IssueType)
	}
	if issues[0].Resource != "skill/broken-skill" {
		t.Errorf("Expected resource 'skill/broken-skill', got %q", issues[0].Resource)
	}
}

// TestDeduplicateIssues verifies that manifest "not-installed" issues are
// dropped when Phase 1 already reported the same resource (e.g., as "broken").
func TestDeduplicateIssues(t *testing.T) {
	tests := []struct {
		name           string
		existing       []VerifyIssue
		manifestIssues []VerifyIssue
		expectedCount  int
	}{
		{
			name: "removes duplicate broken+not-installed for same resource",
			existing: []VerifyIssue{
				{Resource: "my-skill", Tool: "claude", IssueType: "broken", Severity: "error"},
			},
			manifestIssues: []VerifyIssue{
				{Resource: "skill/my-skill", Tool: "any", IssueType: "not-installed", Severity: "warning"},
			},
			expectedCount: 1, // Only the "broken" issue
		},
		{
			name:     "keeps non-duplicate manifest issues",
			existing: []VerifyIssue{},
			manifestIssues: []VerifyIssue{
				{Resource: "skill/other-skill", Tool: "any", IssueType: "not-installed", Severity: "warning"},
			},
			expectedCount: 1, // The "not-installed" issue is kept
		},
		{
			name: "keeps non-not-installed manifest issues even if resource matches",
			existing: []VerifyIssue{
				{Resource: "my-skill", Tool: "claude", IssueType: "broken", Severity: "error"},
			},
			manifestIssues: []VerifyIssue{
				{Resource: "skill/my-skill", Tool: "any", IssueType: "undeclared", Severity: "warning"},
			},
			expectedCount: 2, // Both kept (undeclared is not filtered)
		},
		{
			name: "no manifest issues means no change",
			existing: []VerifyIssue{
				{Resource: "cmd", Tool: "opencode", IssueType: "broken", Severity: "error"},
			},
			manifestIssues: []VerifyIssue{},
			expectedCount:  1,
		},
		{
			name:           "both empty",
			existing:       []VerifyIssue{},
			manifestIssues: []VerifyIssue{},
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateIssues(tt.existing, tt.manifestIssues)
			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d issues, got %d: %+v", tt.expectedCount, len(result), result)
			}
		})
	}
}

// TestBrokenSymlinkNoDuplicatesBetweenPhases verifies the end-to-end behavior:
// a broken symlink that is also in the manifest produces exactly one issue
// (the "broken" from Phase 1), not a duplicate "not-installed" from Phase 2.
func TestBrokenSymlinkNoDuplicatesBetweenPhases(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create manifest referencing a skill
	m := &manifest.Manifest{
		Resources: []string{"skill/test-skill"},
	}
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Create tool directory with a broken symlink
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	brokenTarget := filepath.Join(repoDir, "skills", "test-skill")
	if err := os.Symlink(brokenTarget, filepath.Join(skillsDir, "test-skill")); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	// Detect tools
	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	// Phase 1: scan for symlink issues
	phase1Issues, err := scanProjectIssues(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("scanProjectIssues failed: %v", err)
	}

	// Phase 2: check manifest sync
	phase2Issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	// Deduplicate
	allIssues := deduplicateIssues(phase1Issues, phase2Issues)

	// Should have exactly 1 issue (the "broken" from Phase 1)
	if len(allIssues) != 1 {
		t.Fatalf("Expected exactly 1 issue after dedup, got %d: %+v", len(allIssues), allIssues)
	}
	if allIssues[0].IssueType != "broken" {
		t.Errorf("Expected issue type 'broken', got %q", allIssues[0].IssueType)
	}
	if allIssues[0].Resource != "test-skill" {
		t.Errorf("Expected resource 'test-skill', got %q", allIssues[0].Resource)
	}
}

// TestCheckManifestSync_DetectsUndeclaredSkill verifies that checkManifestSync
// reports a skill installed on disk but not listed in ai.package.yaml.
func TestCheckManifestSync_DetectsUndeclaredSkill(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create an empty manifest (no resources declared)
	m := &manifest.Manifest{
		Resources: []string{},
	}
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Create an installed skill symlink pointing to the repo
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	repoSkillDir := filepath.Join(repoDir, "skills", "undeclared-skill")
	if err := os.MkdirAll(repoSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create repo skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoSkillDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := os.Symlink(repoSkillDir, filepath.Join(skillsDir, "undeclared-skill")); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("Expected 1 undeclared issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].IssueType != "undeclared" {
		t.Errorf("Expected issue type 'undeclared', got %q", issues[0].IssueType)
	}
	if issues[0].Resource != "undeclared-skill" {
		t.Errorf("Expected resource 'undeclared-skill', got %q", issues[0].Resource)
	}
	if issues[0].Severity != "warning" {
		t.Errorf("Expected severity 'warning', got %q", issues[0].Severity)
	}
}

// TestCheckManifestSync_DetectsUndeclaredCommand verifies that checkManifestSync
// reports a command installed on disk but not listed in ai.package.yaml.
func TestCheckManifestSync_DetectsUndeclaredCommand(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create an empty manifest
	m := &manifest.Manifest{
		Resources: []string{},
	}
	if err := m.Save(filepath.Join(projectDir, manifest.ManifestFileName)); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Create an installed command symlink
	commandsDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}
	repoCmd := filepath.Join(repoDir, "commands", "undeclared-cmd.md")
	if err := os.MkdirAll(filepath.Dir(repoCmd), 0755); err != nil {
		t.Fatalf("Failed to create repo cmd dir: %v", err)
	}
	if err := os.WriteFile(repoCmd, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}
	if err := os.Symlink(repoCmd, filepath.Join(commandsDir, "undeclared-cmd.md")); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("Expected 1 undeclared issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].IssueType != "undeclared" {
		t.Errorf("Expected issue type 'undeclared', got %q", issues[0].IssueType)
	}
	if issues[0].Resource != "undeclared-cmd.md" {
		t.Errorf("Expected resource 'undeclared-cmd.md', got %q", issues[0].Resource)
	}
}

// TestCheckManifestSync_DetectsUndeclaredNestedCommand verifies that undeclared detection
// works for namespaced commands in subdirectories (e.g., api/deploy).
func TestCheckManifestSync_DetectsUndeclaredNestedCommand(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create an empty manifest
	m := &manifest.Manifest{
		Resources: []string{},
	}
	if err := m.Save(filepath.Join(projectDir, manifest.ManifestFileName)); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Create a nested command symlink: .opencode/commands/api/deploy.md
	commandsDir := filepath.Join(projectDir, ".opencode", "commands", "api")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create nested commands dir: %v", err)
	}
	repoCmd := filepath.Join(repoDir, "commands", "api", "deploy.md")
	if err := os.MkdirAll(filepath.Dir(repoCmd), 0755); err != nil {
		t.Fatalf("Failed to create repo cmd dir: %v", err)
	}
	if err := os.WriteFile(repoCmd, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}
	if err := os.Symlink(repoCmd, filepath.Join(commandsDir, "deploy.md")); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	if len(issues) != 2 {
		t.Fatalf("Expected 2 undeclared issues (nested file + parent dir), got %d: %+v", len(issues), issues)
	}

	found := map[string]bool{}
	for _, issue := range issues {
		if issue.IssueType != "undeclared" {
			t.Errorf("Expected issue type 'undeclared', got %q", issue.IssueType)
		}
		found[issue.Resource] = true
	}
	if !found[filepath.ToSlash(filepath.Join("api", "deploy.md"))] {
		t.Errorf("Expected resource 'api/deploy.md' in issues, got %+v", issues)
	}
	if !found["api"] {
		t.Errorf("Expected resource 'api' in issues, got %+v", issues)
	}
}

// TestCheckManifestSync_NoUndeclaredWhenInManifest verifies that installed resources
// that ARE in the manifest are not reported as undeclared.
func TestCheckManifestSync_NoUndeclaredWhenInManifest(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create manifest declaring the skill
	m := &manifest.Manifest{
		Resources: []string{"skill/listed-skill"},
	}
	if err := m.Save(filepath.Join(projectDir, manifest.ManifestFileName)); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Create installed skill symlink
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	repoSkillDir := filepath.Join(repoDir, "skills", "listed-skill")
	if err := os.MkdirAll(repoSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create repo skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoSkillDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := os.Symlink(repoSkillDir, filepath.Join(skillsDir, "listed-skill")); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues when resource is in manifest, got %d: %+v", len(issues), issues)
	}
}

// TestCheckManifestSync_NoUndeclaredWithoutManifest verifies no false positives
// when no ai.package.yaml exists (returns nil, no undeclared scanning).
func TestCheckManifestSync_NoUndeclaredWithoutManifest(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create installed skill symlink — but NO manifest
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	repoSkillDir := filepath.Join(repoDir, "skills", "some-skill")
	if err := os.MkdirAll(repoSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create repo skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoSkillDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := os.Symlink(repoSkillDir, filepath.Join(skillsDir, "some-skill")); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues when no manifest exists, got %d: %+v", len(issues), issues)
	}
}

func TestCheckManifestSync_UsesLocalOverlayResources(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Base manifest does not include the local-only resource.
	if err := os.WriteFile(filepath.Join(projectDir, manifest.ManifestFileName), []byte("resources:\n  - skill/base-only\n"), 0644); err != nil {
		t.Fatalf("write base manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, manifest.LocalManifestFileName), []byte("resources:\n  - skill/local-only\n"), 0644); err != nil {
		t.Fatalf("write local manifest: %v", err)
	}

	// Install only the base skill so local overlay resource is reported as missing.
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}
	repoSkillDir := filepath.Join(repoDir, "skills", "base-only")
	if err := os.MkdirAll(repoSkillDir, 0755); err != nil {
		t.Fatalf("mkdir repo skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoSkillDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.Symlink(repoSkillDir, filepath.Join(skillsDir, "base-only")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("detect tools: %v", err)
	}

	issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for local overlay resource not installed, got %d: %+v", len(issues), issues)
	}
	if issues[0].Resource != "skill/local-only" {
		t.Fatalf("expected missing overlay resource issue for skill/local-only, got %+v", issues[0])
	}
	if issues[0].Description != "Listed in "+manifest.LocalManifestFileName+" but not installed" {
		t.Fatalf("expected overlay attribution in description, got %q", issues[0].Description)
	}
	if issues[0].Path != filepath.Join(projectDir, manifest.LocalManifestFileName) {
		t.Fatalf("expected issue path to local manifest, got %q", issues[0].Path)
	}
}

// TestCheckManifestSync_UndeclaredIncludesNonSymlinks verifies that regular files
// and directories in owned tool dirs are treated as undeclared content.
func TestCheckManifestSync_UndeclaredIncludesNonSymlinks(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create an empty manifest
	m := &manifest.Manifest{
		Resources: []string{},
	}
	if err := m.Save(filepath.Join(projectDir, manifest.ManifestFileName)); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Create a regular file and a regular directory in commands dir
	commandsDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "regular-file.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write regular file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(commandsDir, "regular-dir"), 0755); err != nil {
		t.Fatalf("Failed to create regular dir: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	if len(issues) != 2 {
		t.Fatalf("Expected 2 undeclared issues for non-symlinks, got %d: %+v", len(issues), issues)
	}

	found := map[string]bool{}
	for _, issue := range issues {
		if issue.IssueType != "undeclared" {
			t.Errorf("Expected issue type undeclared, got %q", issue.IssueType)
		}
		found[issue.Resource] = true
	}
	if !found["regular-file.md"] {
		t.Errorf("Expected undeclared regular-file.md issue, got: %+v", issues)
	}
	if !found["regular-dir"] {
		t.Errorf("Expected undeclared regular-dir issue, got: %+v", issues)
	}
}

// TestCheckManifestSync_UndeclaredReportedPerOwnedDir verifies that undeclared content
// installed in multiple tool directories is reported per owned directory.
func TestCheckManifestSync_UndeclaredReportedPerOwnedDir(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create an empty manifest
	m := &manifest.Manifest{
		Resources: []string{},
	}
	if err := m.Save(filepath.Join(projectDir, manifest.ManifestFileName)); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Create the same skill installed in both .opencode and .claude
	repoSkillDir := filepath.Join(repoDir, "skills", "shared-skill")
	if err := os.MkdirAll(repoSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create repo skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoSkillDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	for _, toolDir := range []string{".opencode/skills", ".claude/skills"} {
		skillsDir := filepath.Join(projectDir, toolDir)
		if err := os.MkdirAll(skillsDir, 0755); err != nil {
			t.Fatalf("Failed to create skills dir %s: %v", toolDir, err)
		}
		if err := os.Symlink(repoSkillDir, filepath.Join(skillsDir, "shared-skill")); err != nil {
			t.Fatalf("Failed to create symlink in %s: %v", toolDir, err)
		}
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	// Count undeclared issues — should be reported for each owned directory
	undeclaredCount := 0
	for _, issue := range issues {
		if issue.IssueType == "undeclared" {
			undeclaredCount++
		}
	}
	if undeclaredCount != 2 {
		t.Errorf("Expected 2 undeclared issues (one per owned dir), got %d: %+v", undeclaredCount, issues)
	}
}

// TestCheckManifestSync_UndeclaredPackageMembersNotReported verifies that resources
// installed as part of a package are not reported as undeclared when the package
// is declared in the manifest.
func TestCheckManifestSync_UndeclaredPackageMembersNotReported(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create a package with one skill member
	pkg := &resource.Package{
		Name:        "my-pkg",
		Description: "A test package",
		Resources:   []string{"skill/pkg-skill"},
	}
	if err := resource.SavePackage(pkg, repoDir); err != nil {
		t.Fatalf("Failed to save package: %v", err)
	}

	// Create manifest referencing the package (not the individual skill)
	m := &manifest.Manifest{
		Resources: []string{"package/my-pkg"},
	}
	if err := m.Save(filepath.Join(projectDir, manifest.ManifestFileName)); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Install the package member skill
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	repoSkillDir := filepath.Join(repoDir, "skills", "pkg-skill")
	if err := os.MkdirAll(repoSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create repo skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoSkillDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := os.Symlink(repoSkillDir, filepath.Join(skillsDir, "pkg-skill")); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	issues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	// The skill is a member of the declared package — it should NOT be undeclared
	for _, issue := range issues {
		if issue.IssueType == "undeclared" {
			t.Errorf("Package member should not be reported as undeclared: %+v", issue)
		}
	}
}

// TestVerifyFixDeprecationWarning verifies that using --fix with 'aimgr verify'
// prints a deprecation warning to stderr and still proceeds with fix behavior.
func TestVerifyFixDeprecationWarning(t *testing.T) {
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

	// Create tool directory with a broken symlink so there are issues to fix
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills directory: %v", err)
	}
	brokenSymlink := filepath.Join(skillsDir, "broken-skill")
	if err := os.Symlink("/nonexistent/target/skills/broken-skill", brokenSymlink); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
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

	// Set the fix flag
	oldFix := verifyFixFlag
	verifyFixFlag = true
	defer func() { verifyFixFlag = oldFix }()

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Run verify command (ignore the error — the fix logic may fail since skill is gone from repo)
	_ = projectVerifyCmd.RunE(projectVerifyCmd, []string{})

	// Restore stderr and read captured output
	w.Close()
	os.Stderr = oldStderr
	captured := make([]byte, 4096)
	n, _ := r.Read(captured)
	stderrOutput := string(captured[:n])

	// Verify deprecation warning was printed
	if !strings.Contains(stderrOutput, "Warning: --fix is deprecated. Running 'aimgr repair' reconciliation.") {
		t.Errorf("Expected deprecation warning on stderr, got: %q", stderrOutput)
	}
}

// TestFindUndeclaredResources tests the findUndeclaredResources helper directly.
func TestFindUndeclaredResources(t *testing.T) {
	tests := []struct {
		name          string
		resType       string
		manifest      map[string]bool
		setupFunc     func(string) error
		expectedCount int
		expectedName  string
	}{
		{
			name:     "detects undeclared command symlink",
			resType:  "command",
			manifest: map[string]bool{},
			setupFunc: func(dir string) error {
				target := filepath.Join(dir, ".target", "cmd.md")
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(target, filepath.Join(dir, "orphan-cmd.md"))
			},
			expectedCount: 1,
			expectedName:  "orphan-cmd",
		},
		{
			name:     "detects undeclared copilot agent symlink with .agent.md",
			resType:  "agent",
			manifest: map[string]bool{},
			setupFunc: func(dir string) error {
				target := filepath.Join(dir, ".target", "reviewer.md")
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(target, filepath.Join(dir, "reviewer.agent.md"))
			},
			expectedCount: 1,
			expectedName:  "reviewer",
		},
		{
			name:     "skips command in manifest",
			resType:  "command",
			manifest: map[string]bool{"command/listed-cmd": true},
			setupFunc: func(dir string) error {
				target := filepath.Join(dir, ".target", "cmd.md")
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(target, filepath.Join(dir, "listed-cmd.md"))
			},
			expectedCount: 0,
		},
		{
			name:     "detects undeclared nested command",
			resType:  "command",
			manifest: map[string]bool{},
			setupFunc: func(dir string) error {
				subDir := filepath.Join(dir, "api")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					return err
				}
				target := filepath.Join(dir, ".target", "deploy.md")
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
					return err
				}
				return os.Symlink(target, filepath.Join(subDir, "deploy.md"))
			},
			expectedCount: 1,
			expectedName:  "api/deploy",
		},
		{
			name:     "ignores regular files",
			resType:  "command",
			manifest: map[string]bool{},
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "not-a-symlink.md"), []byte("test"), 0644)
			},
			expectedCount: 0,
		},
		{
			name:     "handles nonexistent directory",
			resType:  "skill",
			manifest: map[string]bool{},
			setupFunc: func(dir string) error {
				// Remove the directory so it doesn't exist
				return os.RemoveAll(dir)
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			testDir := filepath.Join(dir, "scan-dir")
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}

			if err := tt.setupFunc(testDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			seen := make(map[string]bool)
			targetTool := tools.Claude
			if tt.resType == "agent" && strings.Contains(tt.name, "copilot") {
				targetTool = tools.Copilot
			}
			issues := findUndeclaredResources(testDir, tt.resType, targetTool, tt.manifest, seen)

			if len(issues) != tt.expectedCount {
				t.Errorf("Expected %d issues, got %d: %+v", tt.expectedCount, len(issues), issues)
			}

			if tt.expectedCount > 0 && len(issues) > 0 && tt.expectedName != "" {
				if issues[0].Resource != tt.expectedName {
					t.Errorf("Expected resource %q, got %q", tt.expectedName, issues[0].Resource)
				}
			}
		})
	}
}
