package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

func writeGlobalConfig(t *testing.T, xdgConfigHome string, content string) string {
	t.Helper()

	configDir := filepath.Join(xdgConfigHome, "aimgr")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	configPath := filepath.Join(configDir, "aimgr.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	return configPath
}

func setupTestRepo(t *testing.T) *repo.Manager {
	t.Helper()

	// Create temp repo
	tmpDir := t.TempDir()
	manager := repo.NewManagerWithPath(tmpDir)

	// Add a test command
	testCmd := filepath.Join(tmpDir, "test-cmd.md")
	cmdContent := `---
description: A test command
---

# Test Command
`
	if err := os.WriteFile(testCmd, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}
	if err := manager.AddCommand(testCmd, "file://"+testCmd, "file"); err != nil {
		t.Fatalf("Failed to add command to repo: %v", err)
	}

	// Add a test skill
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	skillContent := `---
name: test-skill
description: A test skill
---

# Test Skill
`
	if err := os.WriteFile(skillMdPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to create SKILL.md: %v", err)
	}
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to repo: %v", err)
	}

	return manager
}

func TestNewInstaller(t *testing.T) {
	tmpDir := t.TempDir()

	installer, err := NewInstaller(tmpDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	if installer.projectPath != tmpDir {
		t.Errorf("projectPath = %v, want %v", installer.projectPath, tmpDir)
	}

	// Should have Claude as target when no existing dirs
	targets := installer.GetTargetTools()
	if len(targets) != 1 || targets[0] != tools.Claude {
		t.Errorf("targetTools = %v, want [Claude]", targets)
	}
}

func TestDetectInstallTargets(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(string) error
		defaultTools  []tools.Tool
		expectedTools []tools.Tool
	}{
		{
			name:          "no existing folders - uses default",
			setupFunc:     func(dir string) error { return nil },
			defaultTools:  []tools.Tool{tools.Claude},
			expectedTools: []tools.Tool{tools.Claude},
		},
		{
			name: "one existing folder - claude",
			setupFunc: func(dir string) error {
				return os.MkdirAll(filepath.Join(dir, ".claude"), 0755)
			},
			defaultTools:  []tools.Tool{tools.OpenCode},
			expectedTools: []tools.Tool{tools.Claude},
		},
		{
			name: "one existing folder - opencode",
			setupFunc: func(dir string) error {
				return os.MkdirAll(filepath.Join(dir, ".opencode"), 0755)
			},
			defaultTools:  []tools.Tool{tools.Claude},
			expectedTools: []tools.Tool{tools.OpenCode},
		},
		{
			name: "multiple existing folders",
			setupFunc: func(dir string) error {
				if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0755); err != nil {
					return err
				}
				return os.MkdirAll(filepath.Join(dir, ".opencode"), 0755)
			},
			defaultTools:  []tools.Tool{tools.Copilot},
			expectedTools: []tools.Tool{tools.Claude, tools.OpenCode},
		},
		{
			name: "all three tools",
			setupFunc: func(dir string) error {
				if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0755); err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Join(dir, ".opencode"), 0755); err != nil {
					return err
				}
				return os.MkdirAll(filepath.Join(dir, ".github", "skills"), 0755)
			},
			defaultTools:  []tools.Tool{tools.Claude},
			expectedTools: []tools.Tool{tools.Claude, tools.OpenCode, tools.Copilot},
		},
		{
			name:          "no existing folders - multiple defaults",
			setupFunc:     func(dir string) error { return nil },
			defaultTools:  []tools.Tool{tools.Claude, tools.OpenCode},
			expectedTools: []tools.Tool{tools.Claude, tools.OpenCode},
		},
		{
			name: "manifest targets override global config",
			setupFunc: func(dir string) error {
				// Create ai.package.yaml with install.targets
				manifestContent := `resources: []
install:
  targets:
    - opencode
    - copilot
`
				return os.WriteFile(filepath.Join(dir, "ai.package.yaml"), []byte(manifestContent), 0644)
			},
			defaultTools:  []tools.Tool{tools.Claude},
			expectedTools: []tools.Tool{tools.OpenCode, tools.Copilot},
		},
		{
			name: "existing tools override manifest targets",
			setupFunc: func(dir string) error {
				// Create .claude directory
				if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0755); err != nil {
					return err
				}
				// Create ai.package.yaml with different targets
				manifestContent := `resources: []
install:
  targets:
    - opencode
`
				return os.WriteFile(filepath.Join(dir, "ai.package.yaml"), []byte(manifestContent), 0644)
			},
			defaultTools:  []tools.Tool{tools.Copilot},
			expectedTools: []tools.Tool{tools.Claude},
		},
		{
			name: "manifest with no targets uses defaults",
			setupFunc: func(dir string) error {
				// Create ai.package.yaml without install.targets
				manifestContent := `resources: []
`
				return os.WriteFile(filepath.Join(dir, "ai.package.yaml"), []byte(manifestContent), 0644)
			},
			defaultTools:  []tools.Tool{tools.Claude},
			expectedTools: []tools.Tool{tools.Claude},
		},
		{
			name: "local overlay targets extend base targets",
			setupFunc: func(dir string) error {
				base := `resources: []
install:
  targets:
    - claude
`
				local := `resources: []
install:
  targets:
    - opencode
`
				if err := os.WriteFile(filepath.Join(dir, "ai.package.yaml"), []byte(base), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "ai.package.local.yaml"), []byte(local), 0644)
			},
			defaultTools:  []tools.Tool{tools.Copilot},
			expectedTools: []tools.Tool{tools.Claude, tools.OpenCode},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := tt.setupFunc(tmpDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			targets, err := DetectInstallTargets(tmpDir, tt.defaultTools)
			if err != nil {
				t.Fatalf("DetectInstallTargets() error = %v", err)
			}

			if len(targets) != len(tt.expectedTools) {
				t.Errorf("DetectInstallTargets() returned %d tools, want %d", len(targets), len(tt.expectedTools))
				t.Errorf("Got: %v, Want: %v", targets, tt.expectedTools)
				return
			}

			// Check each expected tool is present
			for _, expected := range tt.expectedTools {
				found := false
				for _, target := range targets {
					if target == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected tool %v not found in targets %v", expected, targets)
				}
			}
		})
	}
}

func TestInstallCommand(t *testing.T) {
	manager := setupTestRepo(t)
	projectDir := t.TempDir()

	installer, err := NewInstaller(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Install command
	err = installer.InstallCommand("test-cmd", manager)
	if err != nil {
		t.Fatalf("InstallCommand() error = %v", err)
	}

	// Verify symlink was created in .claude/commands
	symlinkPath := filepath.Join(projectDir, ".claude", "commands", "test-cmd.md")
	info, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("Symlink not created: %v", err)
	}

	// Verify it's a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected symlink, got regular file")
	}

	// Verify symlink target is correct
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	expectedTarget := manager.GetPath("test-cmd", resource.Command)
	if target != expectedTarget {
		t.Errorf("Symlink target = %v, want %v", target, expectedTarget)
	}
}

func TestInstallCommandMultipleTools(t *testing.T) {
	manager := setupTestRepo(t)
	projectDir := t.TempDir()

	// Create both .claude and .opencode directories
	os.MkdirAll(filepath.Join(projectDir, ".claude"), 0755)
	os.MkdirAll(filepath.Join(projectDir, ".opencode"), 0755)

	installer, err := NewInstaller(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Should target both tools
	targets := installer.GetTargetTools()
	if len(targets) != 2 {
		t.Fatalf("Expected 2 target tools, got %d", len(targets))
	}

	// Install command
	err = installer.InstallCommand("test-cmd", manager)
	if err != nil {
		t.Fatalf("InstallCommand() error = %v", err)
	}

	// Verify symlink was created in both directories
	claudeSymlink := filepath.Join(projectDir, ".claude", "commands", "test-cmd.md")
	opencodeSymlink := filepath.Join(projectDir, ".opencode", "commands", "test-cmd.md")

	for _, path := range []string{claudeSymlink, opencodeSymlink} {
		info, err := os.Lstat(path)
		if err != nil {
			t.Errorf("Symlink not created at %s: %v", path, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("Expected symlink at %s, got regular file", path)
		}
	}
}

func TestInstallSkill(t *testing.T) {
	manager := setupTestRepo(t)
	projectDir := t.TempDir()

	installer, err := NewInstaller(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Install skill
	err = installer.InstallSkill("test-skill", manager)
	if err != nil {
		t.Fatalf("InstallSkill() error = %v", err)
	}

	// Verify symlink was created in .claude/skills
	symlinkPath := filepath.Join(projectDir, ".claude", "skills", "test-skill")
	info, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("Symlink not created: %v", err)
	}

	// Verify it's a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected symlink, got regular directory")
	}

	// Verify symlink target is correct
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	expectedTarget := manager.GetPath("test-skill", resource.Skill)
	if target != expectedTarget {
		t.Errorf("Symlink target = %v, want %v", target, expectedTarget)
	}
}

func TestUninstall(t *testing.T) {
	manager := setupTestRepo(t)
	projectDir := t.TempDir()

	installer, err := NewInstaller(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Install command
	if err := installer.InstallCommand("test-cmd", manager); err != nil {
		t.Fatalf("InstallCommand() error = %v", err)
	}

	// Verify it's installed
	if !installer.IsInstalled("test-cmd", resource.Command) {
		t.Fatal("Command should be installed")
	}

	// Uninstall
	err = installer.Uninstall("test-cmd", resource.Command, manager)
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	// Verify it's uninstalled
	if installer.IsInstalled("test-cmd", resource.Command) {
		t.Error("Command should be uninstalled")
	}
}

func TestUninstallNotInstalled(t *testing.T) {
	manager := setupTestRepo(t)
	projectDir := t.TempDir()

	installer, err := NewInstaller(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Try to uninstall non-existent resource
	err = installer.Uninstall("nonexistent", resource.Command, manager)
	if err == nil {
		t.Error("Uninstall() expected error for non-installed resource, got nil")
	}
}

func TestList(t *testing.T) {
	manager := setupTestRepo(t)
	projectDir := t.TempDir()

	installer, err := NewInstaller(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Initially empty
	resources, err := installer.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("List() returned %d resources, want 0", len(resources))
	}

	// Install command and skill
	if err := installer.InstallCommand("test-cmd", manager); err != nil {
		t.Fatalf("InstallCommand() error = %v", err)
	}
	if err := installer.InstallSkill("test-skill", manager); err != nil {
		t.Fatalf("InstallSkill() error = %v", err)
	}

	// List should return 2 resources
	resources, err = installer.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(resources) != 2 {
		t.Errorf("List() returned %d resources, want 2", len(resources))
	}

	// Verify types
	commandFound := false
	skillFound := false
	for _, res := range resources {
		if res.Type == resource.Command && res.Name == "test-cmd" {
			commandFound = true
		}
		if res.Type == resource.Skill && res.Name == "test-skill" {
			skillFound = true
		}
	}

	if !commandFound {
		t.Error("List() did not return installed command")
	}
	if !skillFound {
		t.Error("List() did not return installed skill")
	}
}

func TestIsInstalled(t *testing.T) {
	manager := setupTestRepo(t)
	projectDir := t.TempDir()

	installer, err := NewInstaller(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("NewInstaller() error = %v", err)
	}

	// Not installed initially
	if installer.IsInstalled("test-cmd", resource.Command) {
		t.Error("IsInstalled() returned true for non-installed command")
	}

	// Install command
	if err := installer.InstallCommand("test-cmd", manager); err != nil {
		t.Fatalf("InstallCommand() error = %v", err)
	}

	// Should be installed now
	if !installer.IsInstalled("test-cmd", resource.Command) {
		t.Error("IsInstalled() returned false for installed command")
	}

	// Other resource should not be installed
	if installer.IsInstalled("test-skill", resource.Skill) {
		t.Error("IsInstalled() returned true for non-installed skill")
	}
}

func TestNewInstallerWithTargets(t *testing.T) {
	projectDir := t.TempDir()

	tests := []struct {
		name            string
		targets         []tools.Tool
		wantTargetCount int
	}{
		{
			name:            "single target - claude",
			targets:         []tools.Tool{tools.Claude},
			wantTargetCount: 1,
		},
		{
			name:            "multiple targets - claude and opencode",
			targets:         []tools.Tool{tools.Claude, tools.OpenCode},
			wantTargetCount: 2,
		},
		{
			name:            "all three targets",
			targets:         []tools.Tool{tools.Claude, tools.OpenCode, tools.Copilot},
			wantTargetCount: 3,
		},
		{
			name:            "empty targets",
			targets:         []tools.Tool{},
			wantTargetCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer, err := NewInstallerWithTargets(projectDir, tt.targets)
			if err != nil {
				t.Fatalf("NewInstallerWithTargets() error = %v", err)
			}

			if installer == nil {
				t.Fatal("NewInstallerWithTargets() returned nil installer")
			}

			gotTargets := installer.GetTargetTools()
			if len(gotTargets) != tt.wantTargetCount {
				t.Errorf("NewInstallerWithTargets() got %d targets, want %d", len(gotTargets), tt.wantTargetCount)
			}

			// Verify targets match
			for i, target := range tt.targets {
				if i < len(gotTargets) && gotTargets[i] != target {
					t.Errorf("Target[%d] = %v, want %v", i, gotTargets[i], target)
				}
			}
		})
	}
}

func TestExplicitTargetOverridesDetection(t *testing.T) {
	manager := setupTestRepo(t)
	projectDir := t.TempDir()

	// Create .claude directory (normally would be auto-detected)
	claudeDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create .opencode directory (normally would be auto-detected)
	opencodeDir := filepath.Join(projectDir, ".opencode", "commands")
	if err := os.MkdirAll(opencodeDir, 0755); err != nil {
		t.Fatalf("Failed to create .opencode directory: %v", err)
	}

	// Use explicit target (OpenCode only) - should override auto-detection
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.OpenCode})
	if err != nil {
		t.Fatalf("NewInstallerWithTargets() error = %v", err)
	}

	targets := installer.GetTargetTools()
	if len(targets) != 1 {
		t.Fatalf("Expected 1 target, got %d", len(targets))
	}

	if targets[0] != tools.OpenCode {
		t.Errorf("Expected target OpenCode, got %v", targets[0])
	}

	// Install command - should only go to OpenCode
	if err := installer.InstallCommand("test-cmd", manager); err != nil {
		t.Fatalf("InstallCommand() error = %v", err)
	}

	// Verify installed only in OpenCode
	opencodeInstall := filepath.Join(projectDir, ".opencode", "commands", "test-cmd.md")
	if _, err := os.Lstat(opencodeInstall); err != nil {
		t.Errorf("Command not installed in OpenCode: %v", err)
	}

	// Verify NOT installed in Claude
	claudeInstall := filepath.Join(projectDir, ".claude", "commands", "test-cmd.md")
	if _, err := os.Lstat(claudeInstall); err == nil {
		t.Error("Command should not be installed in Claude when using explicit target")
	}
}

func TestInstallSkillToCopilot(t *testing.T) {
	manager := setupTestRepo(t)
	projectDir := t.TempDir()

	// Create installer targeting Copilot/VSCode
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.Copilot})
	if err != nil {
		t.Fatalf("NewInstallerWithTargets() error = %v", err)
	}

	// Install skill
	err = installer.InstallSkill("test-skill", manager)
	if err != nil {
		t.Fatalf("InstallSkill() error = %v", err)
	}

	// Verify .github directory was created
	githubDir := filepath.Join(projectDir, ".github")
	info, err := os.Stat(githubDir)
	if err != nil {
		t.Fatalf(".github directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error(".github should be a directory")
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf(".github directory permissions = %o, want 0755", info.Mode().Perm())
	}

	// Verify skills directory was created
	skillsDir := filepath.Join(projectDir, ".github", "skills")
	info, err = os.Stat(skillsDir)
	if err != nil {
		t.Fatalf(".github/skills directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error(".github/skills should be a directory")
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf(".github/skills directory permissions = %o, want 0755", info.Mode().Perm())
	}

	// Verify symlink was created in .github/skills
	symlinkPath := filepath.Join(projectDir, ".github", "skills", "test-skill")
	info, err = os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("Symlink not created: %v", err)
	}

	// Verify it's a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected symlink, got regular directory")
	}

	// Verify symlink target is correct
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	expectedTarget := manager.GetPath("test-skill", resource.Skill)
	if target != expectedTarget {
		t.Errorf("Symlink target = %v, want %v", target, expectedTarget)
	}
}

func TestInstallSkillMultipleToolsIncludingCopilot(t *testing.T) {
	manager := setupTestRepo(t)
	projectDir := t.TempDir()

	// Create installer targeting all three tools
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.Claude, tools.OpenCode, tools.Copilot})
	if err != nil {
		t.Fatalf("NewInstallerWithTargets() error = %v", err)
	}

	// Install skill
	err = installer.InstallSkill("test-skill", manager)
	if err != nil {
		t.Fatalf("InstallSkill() error = %v", err)
	}

	// Verify symlink was created in all three directories
	claudeSymlink := filepath.Join(projectDir, ".claude", "skills", "test-skill")
	opencodeSymlink := filepath.Join(projectDir, ".opencode", "skills", "test-skill")
	copilotSymlink := filepath.Join(projectDir, ".github", "skills", "test-skill")

	for _, path := range []string{claudeSymlink, opencodeSymlink, copilotSymlink} {
		info, err := os.Lstat(path)
		if err != nil {
			t.Errorf("Symlink not created at %s: %v", path, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("Expected symlink at %s, got regular directory", path)
		}
	}
}

func TestInstallAgentToCopilotFromCopilotArtifactSource(t *testing.T) {
	tmpRepo := t.TempDir()
	manager := repo.NewManagerWithPath(tmpRepo)
	if err := manager.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	agentFile := filepath.Join(tmpRepo, "test-agent.agent.md")
	agentContent := `---
description: Copilot agent suffix regression test
---

# Test Agent
`
	if err := os.WriteFile(agentFile, []byte(agentContent), 0644); err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}
	if err := manager.AddAgent(agentFile, "file://"+agentFile, "file"); err != nil {
		t.Fatalf("AddAgent() error = %v", err)
	}

	projectDir := t.TempDir()
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.Copilot})
	if err != nil {
		t.Fatalf("NewInstallerWithTargets() error = %v", err)
	}

	if err := installer.InstallAgent("test-agent", manager); err != nil {
		t.Fatalf("InstallAgent() error = %v", err)
	}

	installedPath := filepath.Join(projectDir, ".github", "agents", "test-agent.agent.md")
	if _, err := os.Lstat(installedPath); err != nil {
		t.Fatalf("expected Copilot agent symlink at %s: %v", installedPath, err)
	}

	duplicatePath := filepath.Join(projectDir, ".github", "agents", "test-agent.agent.agent.md")
	if _, err := os.Lstat(duplicatePath); err == nil {
		t.Fatalf("unexpected duplicated Copilot agent suffix at %s", duplicatePath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking duplicate Copilot path: %v", err)
	}
}

func TestInstallCommandCopilotReturnsUnsupportedError(t *testing.T) {
	manager := setupTestRepo(t)
	projectDir := t.TempDir()

	// Create installer targeting Copilot (doesn't support commands)
	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.Copilot})
	if err != nil {
		t.Fatalf("NewInstallerWithTargets() error = %v", err)
	}

	// Try to install command - should fail clearly
	err = installer.InstallCommand("test-cmd", manager)
	if err == nil {
		t.Fatal("InstallCommand() expected error for unsupported copilot command install, got nil")
	}

	if !strings.Contains(err.Error(), "not supported") || !strings.Contains(err.Error(), "copilot") {
		t.Fatalf("InstallCommand() error = %v, want message mentioning unsupported copilot target", err)
	}

	// Verify no .github/commands directory was created
	githubCommandsDir := filepath.Join(projectDir, ".github", "commands")
	if _, err := os.Stat(githubCommandsDir); err == nil {
		t.Error("Copilot should not create commands directory (commands not supported)")
	}
}

func TestGetSymlinkSourcePrefersModificationPath(t *testing.T) {
	repoPath := t.TempDir()
	projectDir := t.TempDir()

	xdgConfigHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
	writeGlobalConfig(t, xdgConfigHome, "install:\n  targets:\n    - claude\n")

	res := &resource.Resource{
		Name: "test-cmd",
		Type: resource.Command,
		Path: filepath.Join(repoPath, "commands", "test-cmd.md"),
	}

	modPath := filepath.Join(repoPath, ".modifications", "claude", "commands", "test-cmd.md")
	if err := os.MkdirAll(filepath.Dir(modPath), 0755); err != nil {
		t.Fatalf("Failed to create modification directory: %v", err)
	}
	if err := os.WriteFile(modPath, []byte("# modified"), 0644); err != nil {
		t.Fatalf("Failed to create modification file: %v", err)
	}

	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("NewInstallerWithTargets() error = %v", err)
	}

	got := installer.getSymlinkSource(res, tools.Claude, repoPath)
	if got != modPath {
		t.Fatalf("getSymlinkSource() = %q, want modification path %q", got, modPath)
	}
}

func TestGetSymlinkSourceFallsBackOnConfigLoadFailure(t *testing.T) {
	repoPath := t.TempDir()
	projectDir := t.TempDir()

	res := &resource.Resource{
		Name: "test-cmd",
		Type: resource.Command,
		Path: filepath.Join(repoPath, "commands", "test-cmd.md"),
	}

	modPath := filepath.Join(repoPath, ".modifications", "claude", "commands", "test-cmd.md")
	if err := os.MkdirAll(filepath.Dir(modPath), 0755); err != nil {
		t.Fatalf("Failed to create modification directory: %v", err)
	}
	if err := os.WriteFile(modPath, []byte("# modified"), 0644); err != nil {
		t.Fatalf("Failed to create modification file: %v", err)
	}

	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("NewInstallerWithTargets() error = %v", err)
	}
	installer.globalConfigLoaded = true
	installer.globalConfigErr = os.ErrPermission

	got := installer.getSymlinkSource(res, tools.Claude, repoPath)
	if got != res.Path {
		t.Fatalf("getSymlinkSource() = %q, want original path %q on config error", got, res.Path)
	}
}

func TestGetSymlinkSourceCachesGlobalConfigPerInstaller(t *testing.T) {
	repoPath := t.TempDir()
	projectDir := t.TempDir()

	xdgConfigHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
	configPath := writeGlobalConfig(t, xdgConfigHome, "install:\n  targets:\n    - claude\n")

	res := &resource.Resource{
		Name: "test-cmd",
		Type: resource.Command,
		Path: filepath.Join(repoPath, "commands", "test-cmd.md"),
	}

	modPath := filepath.Join(repoPath, ".modifications", "claude", "commands", "test-cmd.md")
	if err := os.MkdirAll(filepath.Dir(modPath), 0755); err != nil {
		t.Fatalf("Failed to create modification directory: %v", err)
	}
	if err := os.WriteFile(modPath, []byte("# modified"), 0644); err != nil {
		t.Fatalf("Failed to create modification file: %v", err)
	}

	installer, err := NewInstallerWithTargets(projectDir, []tools.Tool{tools.Claude})
	if err != nil {
		t.Fatalf("NewInstallerWithTargets() error = %v", err)
	}

	first := installer.getSymlinkSource(res, tools.Claude, repoPath)
	if first != modPath {
		t.Fatalf("first getSymlinkSource() = %q, want %q", first, modPath)
	}

	if err := os.WriteFile(configPath, []byte("install: [invalid"), 0644); err != nil {
		t.Fatalf("Failed to corrupt config file: %v", err)
	}

	second := installer.getSymlinkSource(res, tools.Claude, repoPath)
	if second != modPath {
		t.Fatalf("second getSymlinkSource() = %q, want cached modification path %q", second, modPath)
	}
}
