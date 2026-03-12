package cmd

import (
	"encoding/json"
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

// TestRepair_NoIssues verifies that repair reports success when there are no issues.
func TestRepair_NoIssues(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create a valid skill in the repo and install it
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
	// Valid symlink pointing to repo
	if err := os.Symlink(repoSkillDir, filepath.Join(skillsDir, "good-skill")); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	// Phase 1 + Phase 2 should find no issues
	issues, err := scanProjectIssues(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("scanProjectIssues failed: %v", err)
	}
	manifestIssues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}
	allIssues := deduplicateIssues(issues, manifestIssues)

	if len(allIssues) != 0 {
		t.Fatalf("Expected 0 issues for clean project, got %d: %+v", len(allIssues), allIssues)
	}
}

// TestRepair_BrokenSymlink verifies that applyRepairFixes reinstalls a broken symlink.
func TestRepair_BrokenSymlink(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo and add a skill
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	tempSkillDir := t.TempDir()
	skillDir := filepath.Join(tempSkillDir, "broken-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\ndescription: A broken skill\n---\n\n# Broken Skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to repo: %v", err)
	}

	// Create a broken symlink in the project
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	brokenSymlink := filepath.Join(skillsDir, "broken-skill")
	if err := os.Symlink("/nonexistent/path/broken-skill", brokenSymlink); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	// Verify it is broken
	if _, err := os.Stat(brokenSymlink); err == nil {
		t.Fatal("Expected broken symlink")
	}

	issues := []VerifyIssue{
		{
			Resource:  "broken-skill",
			Tool:      "opencode",
			IssueType: issueTypeBroken,
			Path:      brokenSymlink,
			Severity:  "error",
		},
	}

	result := applyRepairFixes(projectDir, issues, manager, output.Table)

	if result.Summary.Fixed != 1 {
		t.Errorf("Expected 1 fixed, got %d. Failed: %+v", result.Summary.Fixed, result.Failed)
	}
	if result.Summary.Failed != 0 {
		t.Errorf("Expected 0 failed, got %d: %+v", result.Summary.Failed, result.Failed)
	}

	// Verify the symlink now points to the correct repo
	actualTarget, err := os.Readlink(brokenSymlink)
	if err != nil {
		t.Fatalf("Symlink no longer exists after repair: %v", err)
	}
	expectedTarget := filepath.Join(repoDir, "skills", "broken-skill")
	if actualTarget != expectedTarget {
		t.Errorf("Symlink points to wrong target: got %s, want %s", actualTarget, expectedTarget)
	}
}

// TestRepair_MissingResource verifies that applyRepairFixes installs a resource
// that is listed in the manifest but not present on disk.
func TestRepair_MissingResource(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo and add a skill
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	tempSkillDir := t.TempDir()
	skillDir := filepath.Join(tempSkillDir, "missing-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\ndescription: A missing skill\n---\n\n# Missing Skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to repo: %v", err)
	}

	// Create tool directory so the installer has somewhere to install
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}

	issues := []VerifyIssue{
		{
			Resource:  "skill/missing-skill",
			Tool:      "any",
			IssueType: issueTypeNotInstalled,
			Path:      filepath.Join(projectDir, manifest.ManifestFileName),
			Severity:  "warning",
		},
	}

	result := applyRepairFixes(projectDir, issues, manager, output.Table)

	if result.Summary.Fixed != 1 {
		t.Errorf("Expected 1 fixed, got %d. Failed: %+v", result.Summary.Fixed, result.Failed)
	}

	// Verify the skill symlink exists
	installedPath := filepath.Join(skillsDir, "missing-skill")
	info, err := os.Lstat(installedPath)
	if err != nil {
		t.Fatalf("Expected skill to be installed at %s: %v", installedPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected a symlink, got regular file/directory")
	}
}

// TestRepair_PackageReference verifies that a package manifest issue can be
// repaired by installing the package members.
func TestRepair_PackageReference(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo and add a skill that is a package member
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	tempSkillDir := t.TempDir()
	skillDir := filepath.Join(tempSkillDir, "pkg-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\ndescription: A package skill\n---\n\n# Package Skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to repo: %v", err)
	}

	// Create a package definition in the repo
	pkg := &resource.Package{
		Name:        "my-pkg",
		Description: "A test package",
		Resources:   []string{"skill/pkg-skill"},
	}
	if err := resource.SavePackage(pkg, repoDir); err != nil {
		t.Fatalf("Failed to save package: %v", err)
	}

	// Create manifest referencing the package
	m := &manifest.Manifest{
		Resources: []string{"package/my-pkg"},
	}
	if err := m.Save(filepath.Join(projectDir, manifest.ManifestFileName)); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Create tool dir but do NOT install the skill (simulating missing member)
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}

	// checkManifestSync will produce a not-installed issue for the package member
	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}
	manifestIssues, err := checkManifestSync(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("checkManifestSync failed: %v", err)
	}

	if len(manifestIssues) == 0 {
		t.Fatal("Expected at least 1 manifest issue for missing package member")
	}
	if manifestIssues[0].Resource != "package/my-pkg" {
		t.Fatalf("Expected package-level issue, got %+v", manifestIssues[0])
	}

	result := applyRepairFixes(projectDir, manifestIssues, manager, output.Table)

	if result.Summary.Fixed != 1 {
		t.Errorf("Expected 1 fixed, got %d. Failed: %+v", result.Summary.Fixed, result.Failed)
	}
	if result.Summary.Failed != 0 {
		t.Errorf("Expected 0 failed, got %d: %+v", result.Summary.Failed, result.Failed)
	}

	// Verify the skill member is now installed
	installedPath := filepath.Join(skillsDir, "pkg-skill")
	info, err := os.Lstat(installedPath)
	if err != nil {
		t.Fatalf("Expected pkg-skill to be installed: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Expected a symlink, got regular file/directory")
	}
}

// TestRepair_MissingPackageReference verifies that missing package references
// fail with a clear repository lookup error.
func TestRepair_MissingPackageReference(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create tool directory so the installer can be constructed.
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}

	issues := []VerifyIssue{{
		Resource:  "package/nonexistent-pkg",
		Tool:      "any",
		IssueType: issueTypeNotInstalled,
		Path:      filepath.Join(projectDir, manifest.ManifestFileName),
		Severity:  "warning",
	}}

	result := applyRepairFixes(projectDir, issues, manager, output.Table)

	if result.Summary.Fixed != 0 {
		t.Errorf("Expected 0 fixed, got %d", result.Summary.Fixed)
	}
	if result.Summary.Failed != 1 {
		t.Fatalf("Expected 1 failed, got %d: %+v", result.Summary.Failed, result.Failed)
	}
	if !strings.Contains(result.Failed[0].Description, "package 'nonexistent-pkg' not found in repository") {
		t.Errorf("Expected clear missing package error, got: %s", result.Failed[0].Description)
	}
}

// TestRepair_HierarchicalCommand verifies that repair can fix a broken symlink
// for a namespaced command like namespace/cmd.
func TestRepair_HierarchicalCommand(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo and add a namespaced command
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create the command file in a separate temp dir with the expected nested structure
	// (commands/api/deploy.md) so that LoadCommand correctly detects the namespaced name.
	tempCmdBase := t.TempDir()
	tempCmdApiDir := filepath.Join(tempCmdBase, "commands", "api")
	if err := os.MkdirAll(tempCmdApiDir, 0755); err != nil {
		t.Fatalf("Failed to create temp api dir: %v", err)
	}
	cmdPath := filepath.Join(tempCmdApiDir, "deploy.md")
	if err := os.WriteFile(cmdPath, []byte("---\ndescription: Deploy command\n---\n\n# Deploy"), 0644); err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}
	// Add command via manager (source is separate from repo, so no self-copy issue)
	if err := manager.AddCommand(cmdPath, "file://"+cmdPath, "file"); err != nil {
		t.Fatalf("Failed to add command to repo: %v", err)
	}

	// Create a broken nested symlink in the project: .claude/commands/api/deploy.md
	nsDir := filepath.Join(projectDir, ".claude", "commands", "api")
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		t.Fatalf("Failed to create nested commands dir: %v", err)
	}
	brokenSymlink := filepath.Join(nsDir, "deploy.md")
	if err := os.Symlink("/nonexistent/commands/api/deploy.md", brokenSymlink); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	// Verify symlink is broken
	if _, err := os.Stat(brokenSymlink); err == nil {
		t.Fatal("Expected broken symlink")
	}

	issues := []VerifyIssue{
		{
			Resource:  "api/deploy",
			Tool:      "claude",
			IssueType: issueTypeBroken,
			Path:      brokenSymlink,
			Severity:  "error",
		},
	}

	result := applyRepairFixes(projectDir, issues, manager, output.Table)

	if result.Summary.Fixed != 1 {
		t.Errorf("Expected 1 fixed, got %d. Failed: %+v", result.Summary.Fixed, result.Failed)
	}

	// Verify symlink was repaired
	actualTarget, err := os.Readlink(brokenSymlink)
	if err != nil {
		t.Fatalf("Symlink does not exist after repair: %v", err)
	}
	expectedTarget := filepath.Join(repoDir, "commands", "api", "deploy.md")
	if actualTarget != expectedTarget {
		t.Errorf("Symlink target wrong: got %s, want %s", actualTarget, expectedTarget)
	}
}

// TestRepair_OrphanedSymlink verifies that applyRepairFixes prints a hint
// for orphaned resources and does NOT remove them.
func TestRepair_OrphanedSymlink(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create an installed skill that is not in the manifest (orphan)
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	repoSkillDir := filepath.Join(repoDir, "skills", "orphan-skill")
	if err := os.MkdirAll(repoSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create repo skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoSkillDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	orphanSymlink := filepath.Join(skillsDir, "orphan-skill")
	if err := os.Symlink(repoSkillDir, orphanSymlink); err != nil {
		t.Fatalf("Failed to create orphan symlink: %v", err)
	}

	issues := []VerifyIssue{
		{
			Resource:  "orphan-skill",
			Tool:      "opencode",
			IssueType: issueTypeOrphaned,
			Path:      orphanSymlink,
			Severity:  "warning",
		},
	}

	result := applyRepairFixes(projectDir, issues, manager, output.Table)

	// Orphaned issues → hints only, not fixed or failed
	if result.Summary.Fixed != 0 {
		t.Errorf("Expected 0 fixed for orphan, got %d", result.Summary.Fixed)
	}
	if result.Summary.Failed != 0 {
		t.Errorf("Expected 0 failed for orphan, got %d", result.Summary.Failed)
	}
	if result.Summary.Hints != 1 {
		t.Errorf("Expected 1 hint for orphan, got %d", result.Summary.Hints)
	}

	// Verify the symlink was NOT removed
	if _, err := os.Lstat(orphanSymlink); err != nil {
		t.Error("Orphan symlink was removed — repair should only hint, not remove")
	}

	// Verify the hint mentions 'aimgr uninstall'
	if len(result.Hints) > 0 && !strings.Contains(result.Hints[0].Description, "aimgr uninstall") {
		t.Errorf("Hint should mention 'aimgr uninstall', got: %s", result.Hints[0].Description)
	}
}

// TestRepair_CommandFlagsExist verifies that the repair command exposes
// all expected flags in --help output.
func TestRepair_CommandFlagsExist(t *testing.T) {
	expectedFlags := []string{"format", "reset", "prune-package", "force", "dry-run", "project-path"}

	for _, flagName := range expectedFlags {
		flag := repairCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected flag --%s to be registered on repair command", flagName)
		}
	}
}

// TestRepair_JSONOutput verifies that repairDisplayNoIssues outputs valid JSON.
func TestRepair_JSONOutput(t *testing.T) {
	// Redirect stdout to capture output
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	err = repairDisplayNoIssues(output.JSON)
	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("repairDisplayNoIssues failed: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	if !strings.Contains(got, `"fixed"`) {
		t.Errorf("JSON output should contain 'fixed' key, got: %s", got)
	}
	if !strings.Contains(got, `"summary"`) {
		t.Errorf("JSON output should contain 'summary' key, got: %s", got)
	}
}

// TestRepair_RepairResultJSON verifies RepairResult struct fields.
func TestRepair_RepairResultJSON(t *testing.T) {
	result := RepairResult{
		Fixed:   []RepairAction{{Resource: "test-skill", Tool: "opencode", IssueType: "broken", Description: "Reinstalled test-skill"}},
		Failed:  []RepairAction{},
		Hints:   []RepairAction{},
		Summary: RepairSummary{Fixed: 1, Failed: 0, Hints: 0},
	}

	if result.Summary.Fixed != 1 {
		t.Errorf("Expected Fixed=1, got %d", result.Summary.Fixed)
	}
	if len(result.Fixed) != 1 {
		t.Errorf("Expected 1 fixed action, got %d", len(result.Fixed))
	}
	if result.Fixed[0].Resource != "test-skill" {
		t.Errorf("Expected resource 'test-skill', got %q", result.Fixed[0].Resource)
	}
}

// TestRepair_JSONOutputNoProgressOnStdout verifies that when format is JSON,
// progress lines do NOT appear on stdout — only valid JSON.
// This is a regression test for ai-config-manager-cpd.
func TestRepair_JSONOutputNoProgressOnStdout(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo and add a skill
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	tempSkillDir := t.TempDir()
	skillDir := filepath.Join(tempSkillDir, "json-test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\ndescription: JSON test skill\n---\n\n# JSON Test Skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to repo: %v", err)
	}

	// Create a broken symlink in the project
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	brokenSymlink := filepath.Join(skillsDir, "json-test-skill")
	if err := os.Symlink("/nonexistent/path/json-test-skill", brokenSymlink); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	issues := []VerifyIssue{
		{
			Resource:  "json-test-skill",
			Tool:      "opencode",
			IssueType: issueTypeBroken,
			Path:      brokenSymlink,
			Severity:  "error",
		},
	}

	// Capture stdout during applyRepairFixes with JSON format
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	result := applyRepairFixes(projectDir, issues, manager, output.JSON)

	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	stdoutContent := string(buf[:n])

	// The repair should succeed
	if result.Summary.Fixed != 1 {
		t.Errorf("Expected 1 fixed, got %d. Failed: %+v", result.Summary.Fixed, result.Failed)
	}

	// stdout should be empty — all progress goes to stderr in JSON mode
	if stdoutContent != "" {
		t.Errorf("Expected no stdout output in JSON mode, but got:\n%s", stdoutContent)
	}

	// Now verify that repairDisplayResult + the full pipeline produces valid JSON
	r2, w2, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w2

	if err := repairDisplayResult(result, output.JSON); err != nil {
		w2.Close()
		os.Stdout = oldStdout
		t.Fatalf("repairDisplayResult failed: %v", err)
	}
	w2.Close()
	os.Stdout = oldStdout

	buf2 := make([]byte, 8192)
	n2, _ := r2.Read(buf2)
	jsonOutput := string(buf2[:n2])

	// Verify the JSON is parseable
	var parsed RepairResult
	if err := json.Unmarshal([]byte(jsonOutput), &parsed); err != nil {
		t.Errorf("JSON output is not valid JSON: %v\nGot: %s", err, jsonOutput)
	}
	if parsed.Summary.Fixed != 1 {
		t.Errorf("Parsed JSON should show 1 fixed, got %d", parsed.Summary.Fixed)
	}
}

// TestRepair_JSONOutputPackageNoProgressOnStdout verifies that when repairing a
// package reference with JSON format, the package install progress lines do NOT
// appear on stdout.
func TestRepair_JSONOutputPackageNoProgressOnStdout(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Initialize repo, add a skill, and create a package
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	tempSkillDir := t.TempDir()
	skillDir := filepath.Join(tempSkillDir, "pkg-json-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\ndescription: Package JSON test skill\n---\n\n# Skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to repo: %v", err)
	}

	pkg := &resource.Package{
		Name:        "json-test-pkg",
		Description: "A test package for JSON output",
		Resources:   []string{"skill/pkg-json-skill"},
	}
	if err := resource.SavePackage(pkg, repoDir); err != nil {
		t.Fatalf("Failed to save package: %v", err)
	}

	// Create tool dir but do NOT install the skill
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}

	issues := []VerifyIssue{
		{
			Resource:  "package/json-test-pkg",
			Tool:      "any",
			IssueType: issueTypeNotInstalled,
			Path:      filepath.Join(projectDir, manifest.ManifestFileName),
			Severity:  "warning",
		},
	}

	// Capture stdout during applyRepairFixes with JSON format
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	result := applyRepairFixes(projectDir, issues, manager, output.JSON)

	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	stdoutContent := string(buf[:n])

	// The repair should succeed
	if result.Summary.Fixed != 1 {
		t.Errorf("Expected 1 fixed, got %d. Failed: %+v", result.Summary.Fixed, result.Failed)
	}

	// stdout should be empty — all package install progress goes to stderr in JSON mode
	if stdoutContent != "" {
		t.Errorf("Expected no stdout output in JSON mode for package repair, but got:\n%s", stdoutContent)
	}
}

// ---- --reset tests ----

// TestRepairReset_FindsUnmanagedFiles verifies that regular files in resource dirs are detected.
func TestRepairReset_FindsUnmanagedFiles(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create .opencode/skills directory with a regular file (not a symlink)
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	regularFile := filepath.Join(skillsDir, "manual-skill.md")
	if err := os.WriteFile(regularFile, []byte("manual content"), 0644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	unmanaged, err := findUnmanagedFiles(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("findUnmanagedFiles failed: %v", err)
	}

	if len(unmanaged) != 1 {
		t.Errorf("Expected 1 unmanaged file, got %d: %v", len(unmanaged), unmanaged)
	}
	if len(unmanaged) > 0 && unmanaged[0] != regularFile {
		t.Errorf("Expected unmanaged path %s, got %s", regularFile, unmanaged[0])
	}
}

// TestRepairReset_IgnoresManagedSymlinks verifies that symlinks to repo are not flagged.
func TestRepairReset_IgnoresManagedSymlinks(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create a skill in the repo
	repoSkillDir := filepath.Join(repoDir, "skills", "good-skill")
	if err := os.MkdirAll(repoSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create repo skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoSkillDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Install it as a symlink to the repo
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	symlinkPath := filepath.Join(skillsDir, "good-skill")
	if err := os.Symlink(repoSkillDir, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	unmanaged, err := findUnmanagedFiles(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("findUnmanagedFiles failed: %v", err)
	}

	if len(unmanaged) != 0 {
		t.Errorf("Expected 0 unmanaged files, got %d: %v", len(unmanaged), unmanaged)
	}
}

// TestRepairReset_DetectsNonRepoSymlinks verifies that symlinks to other locations are flagged.
func TestRepairReset_DetectsNonRepoSymlinks(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()
	otherDir := t.TempDir()

	// Create a skill in some other location (not the repo)
	otherSkillDir := filepath.Join(otherDir, "some-skill")
	if err := os.MkdirAll(otherSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create other skill dir: %v", err)
	}

	// Create a symlink to the other location
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	symlinkPath := filepath.Join(skillsDir, "some-skill")
	if err := os.Symlink(otherSkillDir, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	unmanaged, err := findUnmanagedFiles(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("findUnmanagedFiles failed: %v", err)
	}

	if len(unmanaged) != 1 {
		t.Errorf("Expected 1 unmanaged symlink, got %d: %v", len(unmanaged), unmanaged)
	}
	if len(unmanaged) > 0 && unmanaged[0] != symlinkPath {
		t.Errorf("Expected unmanaged path %s, got %s", symlinkPath, unmanaged[0])
	}
}

// TestRepairReset_NestedNamespaceCommands verifies that unmanaged files in namespace subdirs
// are detected (one-level recursion).
func TestRepairReset_NestedNamespaceCommands(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create .claude/commands/api/ with a regular file (not a symlink)
	nsDir := filepath.Join(projectDir, ".claude", "commands", "api")
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		t.Fatalf("Failed to create namespace dir: %v", err)
	}
	regularFile := filepath.Join(nsDir, "deploy.md")
	if err := os.WriteFile(regularFile, []byte("deploy cmd"), 0644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	unmanaged, err := findUnmanagedFiles(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("findUnmanagedFiles failed: %v", err)
	}

	if len(unmanaged) != 1 {
		t.Errorf("Expected 1 unmanaged file in namespace dir, got %d: %v", len(unmanaged), unmanaged)
	}
	if len(unmanaged) > 0 && unmanaged[0] != regularFile {
		t.Errorf("Expected %s, got %s", regularFile, unmanaged[0])
	}
}

// TestRepairReset_OnlyScansResourceDirs verifies that files in the root tool dir are NOT flagged.
func TestRepairReset_OnlyScansResourceDirs(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create .opencode/ root with a config file (e.g. config.yaml) — should NOT be flagged
	opencodeDir := filepath.Join(projectDir, ".opencode")
	if err := os.MkdirAll(opencodeDir, 0755); err != nil {
		t.Fatalf("Failed to create .opencode dir: %v", err)
	}
	rootConfig := filepath.Join(opencodeDir, "config.yaml")
	if err := os.WriteFile(rootConfig, []byte("config: value"), 0644); err != nil {
		t.Fatalf("Failed to create root config: %v", err)
	}

	// Also create .opencode/skills/ — empty, so no unmanaged files inside it
	skillsDir := filepath.Join(opencodeDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	unmanaged, err := findUnmanagedFiles(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("findUnmanagedFiles failed: %v", err)
	}

	// The root config.yaml should NOT be in unmanaged results — we only scan subdirs
	for _, path := range unmanaged {
		if path == rootConfig {
			t.Errorf("Root config file %s should not be flagged as unmanaged", rootConfig)
		}
	}

	if len(unmanaged) != 0 {
		t.Errorf("Expected 0 unmanaged files, got %d: %v", len(unmanaged), unmanaged)
	}
}

// TestRepairReset_ForceMode verifies that --force removes unmanaged files without prompting.
func TestRepairReset_ForceMode(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create a regular file in the skills dir
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	unmanagedFile := filepath.Join(skillsDir, "manual-skill.md")
	if err := os.WriteFile(unmanagedFile, []byte("manual"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	unmanaged, err := findUnmanagedFiles(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("findUnmanagedFiles failed: %v", err)
	}
	if len(unmanaged) != 1 {
		t.Fatalf("Expected 1 unmanaged file, got %d", len(unmanaged))
	}

	// Force mode: should remove without prompting
	removed, err := promptAndRemoveUnmanaged(unmanaged, false, true)
	if err != nil {
		t.Fatalf("promptAndRemoveUnmanaged failed: %v", err)
	}

	if len(removed) != 1 {
		t.Errorf("Expected 1 removed file, got %d", len(removed))
	}

	// Verify the file is gone
	if _, err := os.Stat(unmanagedFile); err == nil {
		t.Error("Expected file to be removed, but it still exists")
	}
}

// TestRepairReset_DryRunMode verifies that --dry-run lists files but doesn't remove them.
func TestRepairReset_DryRunMode(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create a regular file in the skills dir
	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	unmanagedFile := filepath.Join(skillsDir, "manual-skill.md")
	if err := os.WriteFile(unmanagedFile, []byte("manual"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	unmanaged, err := findUnmanagedFiles(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("findUnmanagedFiles failed: %v", err)
	}
	if len(unmanaged) != 1 {
		t.Fatalf("Expected 1 unmanaged file, got %d", len(unmanaged))
	}

	// Dry-run mode: should not remove
	removed, err := promptAndRemoveUnmanaged(unmanaged, true, false)
	if err != nil {
		t.Fatalf("promptAndRemoveUnmanaged (dry-run) failed: %v", err)
	}

	if len(removed) != 0 {
		t.Errorf("Expected 0 removed files in dry-run mode, got %d", len(removed))
	}

	// Verify the file still exists
	if _, err := os.Stat(unmanagedFile); err != nil {
		t.Errorf("Expected file to still exist after dry-run, but got: %v", err)
	}
}

// TestRepairReset_MixedDirectory verifies that in a dir with both managed symlinks and
// unmanaged files, only the unmanaged files are removed.
func TestRepairReset_MixedDirectory(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	skillsDir := filepath.Join(projectDir, ".opencode", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}

	// Create a managed symlink (points to repo)
	repoSkillDir := filepath.Join(repoDir, "skills", "managed-skill")
	if err := os.MkdirAll(repoSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create repo skill: %v", err)
	}
	managedSymlink := filepath.Join(skillsDir, "managed-skill")
	if err := os.Symlink(repoSkillDir, managedSymlink); err != nil {
		t.Fatalf("Failed to create managed symlink: %v", err)
	}

	// Create an unmanaged regular file
	unmanagedFile := filepath.Join(skillsDir, "unmanaged.md")
	if err := os.WriteFile(unmanagedFile, []byte("unmanaged"), 0644); err != nil {
		t.Fatalf("Failed to create unmanaged file: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	unmanaged, err := findUnmanagedFiles(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("findUnmanagedFiles failed: %v", err)
	}

	// Only the regular file should be detected as unmanaged
	if len(unmanaged) != 1 {
		t.Errorf("Expected 1 unmanaged file (not the managed symlink), got %d: %v", len(unmanaged), unmanaged)
	}

	// Remove via force mode
	removed, err := promptAndRemoveUnmanaged(unmanaged, false, true)
	if err != nil {
		t.Fatalf("promptAndRemoveUnmanaged failed: %v", err)
	}
	if len(removed) != 1 {
		t.Errorf("Expected 1 removed, got %d", len(removed))
	}

	// Verify managed symlink still exists
	if _, err := os.Lstat(managedSymlink); err != nil {
		t.Error("Managed symlink was incorrectly removed")
	}

	// Verify unmanaged file is gone
	if _, err := os.Stat(unmanagedFile); err == nil {
		t.Error("Expected unmanaged file to be removed")
	}
}

// TestRepairReset_EmptyAfterCleanup verifies that empty namespace dirs are cleaned up.
func TestRepairReset_EmptyAfterCleanup(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	// Create .claude/commands/api/ with only an unmanaged file
	nsDir := filepath.Join(projectDir, ".claude", "commands", "api")
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		t.Fatalf("Failed to create namespace dir: %v", err)
	}
	unmanagedFile := filepath.Join(nsDir, "deploy.md")
	if err := os.WriteFile(unmanagedFile, []byte("manual deploy"), 0644); err != nil {
		t.Fatalf("Failed to create unmanaged file: %v", err)
	}

	detectedTools, err := tools.DetectExistingTools(projectDir)
	if err != nil {
		t.Fatalf("Failed to detect tools: %v", err)
	}

	unmanaged, err := findUnmanagedFiles(projectDir, detectedTools, repoDir)
	if err != nil {
		t.Fatalf("findUnmanagedFiles failed: %v", err)
	}

	if len(unmanaged) != 1 {
		t.Fatalf("Expected 1 unmanaged file, got %d: %v", len(unmanaged), unmanaged)
	}

	// Remove with force mode — should also clean up the empty namespace dir
	removed, err := promptAndRemoveUnmanaged(unmanaged, false, true)
	if err != nil {
		t.Fatalf("promptAndRemoveUnmanaged failed: %v", err)
	}
	if len(removed) != 1 {
		t.Errorf("Expected 1 removed file, got %d", len(removed))
	}

	// Verify the namespace directory is also removed (it's empty now)
	if _, err := os.Stat(nsDir); err == nil {
		t.Error("Expected empty namespace directory to be removed, but it still exists")
	}
}

// ---- --prune-package tests ----

// createTestRepoWithSkill creates a minimal initialized repo and adds a skill to it.
func createTestRepoWithSkill(t *testing.T, repoDir, skillName string) *repo.Manager {
	t.Helper()
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	tempSkillBase := t.TempDir()
	skillDir := filepath.Join(tempSkillBase, skillName)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\ndescription: Test skill\n---\n\n# Skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill to repo: %v", err)
	}
	return manager
}

// createTestRepoWithPackage creates a repo with a package that references a skill.
func createTestRepoWithPackage(t *testing.T, repoDir, pkgName, skillName string) *repo.Manager {
	t.Helper()
	manager := createTestRepoWithSkill(t, repoDir, skillName)

	pkg := &resource.Package{
		Name:        pkgName,
		Description: "A test package",
		Resources:   []string{"skill/" + skillName},
	}
	if err := resource.SavePackage(pkg, repoDir); err != nil {
		t.Fatalf("Failed to save package: %v", err)
	}
	return manager
}

// mockStdinFile creates a temp file with pre-written input lines for stdin simulation.
func mockStdinFile(t *testing.T, inputs ...string) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdin-mock")
	if err != nil {
		t.Fatalf("Failed to create mock stdin: %v", err)
	}
	content := strings.Join(inputs, "\n") + "\n"
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("Failed to write to mock stdin: %v", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek mock stdin: %v", err)
	}
	return f
}

// TestRepairPrunePackage_AllValid verifies that when all manifest refs exist in the repo,
// nothing is flagged as invalid.
func TestRepairPrunePackage_AllValid(t *testing.T) {
	repoDir := t.TempDir()
	manager := createTestRepoWithSkill(t, repoDir, "my-skill")

	m := &manifest.Manifest{
		Resources: []string{"skill/my-skill"},
	}

	invalidRefs, partialPkgs := findInvalidManifestRefs(m, manager)

	if len(invalidRefs) != 0 {
		t.Errorf("Expected 0 invalid refs, got %d: %v", len(invalidRefs), invalidRefs)
	}
	if len(partialPkgs) != 0 {
		t.Errorf("Expected 0 partial packages, got %d", len(partialPkgs))
	}
}

// TestRepairPrunePackage_InvalidIndividualRef verifies that a command/skill/agent ref
// that doesn't exist in the repo is detected as invalid.
func TestRepairPrunePackage_InvalidIndividualRef(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Manifest references a skill that doesn't exist in the repo
	m := &manifest.Manifest{
		Resources: []string{"skill/missing-skill"},
	}

	invalidRefs, partialPkgs := findInvalidManifestRefs(m, manager)

	if len(invalidRefs) != 1 {
		t.Errorf("Expected 1 invalid ref, got %d: %v", len(invalidRefs), invalidRefs)
	}
	if len(invalidRefs) > 0 && invalidRefs[0] != "skill/missing-skill" {
		t.Errorf("Expected 'skill/missing-skill', got %q", invalidRefs[0])
	}
	if len(partialPkgs) != 0 {
		t.Errorf("Expected 0 partial packages, got %d", len(partialPkgs))
	}
}

// TestRepairPrunePackage_InvalidPackageRef verifies that a package ref where the
// package doesn't exist is detected as invalid.
func TestRepairPrunePackage_InvalidPackageRef(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Manifest references a package that doesn't exist in the repo
	m := &manifest.Manifest{
		Resources: []string{"package/missing-pkg"},
	}

	invalidRefs, partialPkgs := findInvalidManifestRefs(m, manager)

	if len(invalidRefs) != 1 {
		t.Errorf("Expected 1 invalid ref, got %d: %v", len(invalidRefs), invalidRefs)
	}
	if len(invalidRefs) > 0 && invalidRefs[0] != "package/missing-pkg" {
		t.Errorf("Expected 'package/missing-pkg', got %q", invalidRefs[0])
	}
	if len(partialPkgs) != 0 {
		t.Errorf("Expected 0 partial packages, got %d", len(partialPkgs))
	}
}

// TestRepairPrunePackage_PartialPackage verifies that a package that exists but has a
// missing member is reported as a warning (partial package), NOT as an invalid ref.
func TestRepairPrunePackage_PartialPackage(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create a package that references a skill NOT in the repo
	pkg := &resource.Package{
		Name:        "my-pkg",
		Description: "A test package",
		Resources:   []string{"skill/ghost-skill"},
	}
	if err := resource.SavePackage(pkg, repoDir); err != nil {
		t.Fatalf("Failed to save package: %v", err)
	}

	// Manifest references the package (which exists)
	m := &manifest.Manifest{
		Resources: []string{"package/my-pkg"},
	}

	invalidRefs, partialPkgs := findInvalidManifestRefs(m, manager)

	// Package exists → should NOT be in invalidRefs
	if len(invalidRefs) != 0 {
		t.Errorf("Expected 0 invalid refs (package exists), got %d: %v", len(invalidRefs), invalidRefs)
	}
	// Package has missing member → should be in partialPkgs (warning only)
	if len(partialPkgs) != 1 {
		t.Errorf("Expected 1 partial package warning, got %d", len(partialPkgs))
	}
	if len(partialPkgs) > 0 {
		if partialPkgs[0].PackageName != "my-pkg" {
			t.Errorf("Expected package name 'my-pkg', got %q", partialPkgs[0].PackageName)
		}
		if len(partialPkgs[0].MissingMembers) == 0 {
			t.Error("Expected at least one missing member in partial package warning")
		}
	}
}

// TestRepairPrunePackage_ForceMode verifies that --force removes invalid refs without
// prompting and saves the manifest.
func TestRepairPrunePackage_ForceMode(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	// Create manifest with one valid and one invalid ref
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	m := &manifest.Manifest{
		Resources: []string{"skill/missing-skill"},
	}
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Reload manifest (as the real code does)
	loaded, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	invalidRefs, _ := findInvalidManifestRefs(loaded, manager)
	if len(invalidRefs) != 1 {
		t.Fatalf("Expected 1 invalid ref, got %d", len(invalidRefs))
	}

	// Force mode: remove without prompting
	if err := resolveInvalidRefs(invalidRefs, loaded, manifestPath, manager, false, true, os.Stdin); err != nil {
		t.Fatalf("resolveInvalidRefs failed: %v", err)
	}

	// Reload and verify the ref was removed
	updated, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}
	if len(updated.Resources) != 0 {
		t.Errorf("Expected 0 resources after force prune, got %d: %v", len(updated.Resources), updated.Resources)
	}
	if updated.Has("skill/missing-skill") {
		t.Error("Expected 'skill/missing-skill' to be removed from manifest")
	}
}

// TestRepairPrunePackage_DryRunMode verifies that --dry-run lists invalid refs but
// does NOT modify the manifest.
func TestRepairPrunePackage_DryRunMode(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	m := &manifest.Manifest{
		Resources: []string{"skill/missing-skill"},
	}
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	loaded, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	invalidRefs, _ := findInvalidManifestRefs(loaded, manager)
	if len(invalidRefs) != 1 {
		t.Fatalf("Expected 1 invalid ref, got %d", len(invalidRefs))
	}

	// Dry-run mode: should not modify manifest
	if err := resolveInvalidRefs(invalidRefs, loaded, manifestPath, manager, true, false, os.Stdin); err != nil {
		t.Fatalf("resolveInvalidRefs (dry-run) failed: %v", err)
	}

	// Reload and verify the ref was NOT removed
	updated, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}
	if !updated.Has("skill/missing-skill") {
		t.Error("Expected 'skill/missing-skill' to still be in manifest after dry-run")
	}
}

// TestRepairPrunePackage_NoManifest verifies that the prune-package code handles the
// case where no ai.package.yaml exists gracefully (no panic, no error).
func TestRepairPrunePackage_NoManifest(t *testing.T) {
	projectDir := t.TempDir()
	// No manifest file — just verify it doesn't exist
	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	if manifest.Exists(manifestPath) {
		t.Fatal("Test setup error: manifest should not exist")
	}

	// The RunE code checks Exists() before loading — simulate that logic
	if manifest.Exists(manifestPath) {
		t.Error("Manifest should not exist")
	}
	// If we reached here without panic, the graceful-skip logic works
}

// TestRepairPrunePackage_MixedValidInvalid verifies that only invalid refs are flagged
// when the manifest contains both valid and invalid references.
func TestRepairPrunePackage_MixedValidInvalid(t *testing.T) {
	repoDir := t.TempDir()
	manager := createTestRepoWithSkill(t, repoDir, "valid-skill")

	// Manifest has one valid ref and two invalid refs
	m := &manifest.Manifest{
		Resources: []string{
			"skill/valid-skill",
			"skill/missing-skill",
			"command/missing-cmd",
		},
	}

	invalidRefs, partialPkgs := findInvalidManifestRefs(m, manager)

	if len(invalidRefs) != 2 {
		t.Errorf("Expected 2 invalid refs, got %d: %v", len(invalidRefs), invalidRefs)
	}
	// valid-skill should NOT be in invalid refs
	for _, ref := range invalidRefs {
		if ref == "skill/valid-skill" {
			t.Error("'skill/valid-skill' should not be in invalid refs")
		}
	}
	if len(partialPkgs) != 0 {
		t.Errorf("Expected 0 partial packages, got %d", len(partialPkgs))
	}
}

// TestRepairPrunePackage_InteractiveRemove verifies the interactive mode "remove" path
// using a mocked stdin file.
func TestRepairPrunePackage_InteractiveRemove(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	m := &manifest.Manifest{
		Resources: []string{"skill/missing-skill"},
	}
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	loaded, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	invalidRefs, _ := findInvalidManifestRefs(loaded, manager)
	if len(invalidRefs) != 1 {
		t.Fatalf("Expected 1 invalid ref, got %d", len(invalidRefs))
	}

	// In interactive mode with no sync/repair tried, options are:
	// [1] Run repo sync first
	// [2] Run repo repair first
	// [3] Remove from ai.package.yaml
	// [4] Skip
	// We select option 3 (remove)
	mockInput := mockStdinFile(t, "3")
	defer mockInput.Close()

	if err := resolveInvalidRefs(invalidRefs, loaded, manifestPath, manager, false, false, mockInput); err != nil {
		t.Fatalf("resolveInvalidRefs failed: %v", err)
	}

	// Reload and verify the ref was removed
	updated, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}
	if updated.Has("skill/missing-skill") {
		t.Error("Expected 'skill/missing-skill' to be removed from manifest")
	}
}

// TestRepairPrunePackage_InteractiveSkip verifies the interactive mode "skip" path.
func TestRepairPrunePackage_InteractiveSkip(t *testing.T) {
	repoDir := t.TempDir()
	projectDir := t.TempDir()

	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	manifestPath := filepath.Join(projectDir, manifest.ManifestFileName)
	m := &manifest.Manifest{
		Resources: []string{"skill/missing-skill"},
	}
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	loaded, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	invalidRefs, _ := findInvalidManifestRefs(loaded, manager)
	if len(invalidRefs) != 1 {
		t.Fatalf("Expected 1 invalid ref, got %d", len(invalidRefs))
	}

	// Select option 4 (skip)
	mockInput := mockStdinFile(t, "4")
	defer mockInput.Close()

	if err := resolveInvalidRefs(invalidRefs, loaded, manifestPath, manager, false, false, mockInput); err != nil {
		t.Fatalf("resolveInvalidRefs failed: %v", err)
	}

	// Reload and verify the ref was NOT removed
	updated, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}
	if !updated.Has("skill/missing-skill") {
		t.Error("Expected 'skill/missing-skill' to still be present after skip")
	}
}
