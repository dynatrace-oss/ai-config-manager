package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
)

// TestRepoAdd_ManifestCommitted verifies that manifest changes are committed to git
// This is a regression test for bug ai-config-manager-8wr
func TestRepoAdd_ManifestCommitted(t *testing.T) {
	testDir := t.TempDir()
	repoDir := filepath.Join(testDir, "repo")

	// Set up environment
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Initialize repository
	_, err := runAimgr(t, "repo", "init")
	if err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Verify clean initial state
	gitStatus := func() string {
		cmd := exec.Command("git", "status", "--porcelain")
		cmd.Dir = repoDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to get git status: %v", err)
		}
		return strings.TrimSpace(string(output))
	}

	if status := gitStatus(); status != "" {
		t.Fatalf("Expected clean working tree after init, got: %s", status)
	}

	// Create test resource
	resourceDir := filepath.Join(testDir, "resources")
	createTestCommandInDir(t, resourceDir, "test-cmd", "Test command")

	// Add resource
	_, err = runAimgr(t, "repo", "add", "local:"+resourceDir, "--name", "test-source")
	if err != nil {
		t.Fatalf("Failed to add resource: %v", err)
	}

	// Verify working tree is clean (bug fix verification)
	if status := gitStatus(); status != "" {
		t.Errorf("Expected clean working tree after repo add, but got uncommitted changes:\n%s", status)
		t.Error("This indicates manifest files were not committed (bug ai-config-manager-8wr)")
	}

	// Verify we have the expected commits
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get git log: %v", err)
	}
	logOutput := string(output)

	// Should have: init commit, import commit, and manifest commit
	expectedCommits := []string{
		"aimgr: initialize repository",
		"aimgr: import 1 resource(s)",
		"aimgr: track source in manifest",
	}

	for _, expectedMsg := range expectedCommits {
		if !strings.Contains(logOutput, expectedMsg) {
			t.Errorf("Expected commit message %q not found in git log:\n%s", expectedMsg, logOutput)
		}
	}

	// Verify manifest commit contains the right files
	cmd = exec.Command("git", "show", "--stat", "--oneline", "HEAD")
	cmd.Dir = repoDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to show latest commit: %v", err)
	}
	commitShow := string(output)

	// Latest commit should be the manifest commit
	if !strings.Contains(commitShow, "aimgr: track source in manifest") {
		t.Errorf("Latest commit should be manifest tracking, got:\n%s", commitShow)
	}

	// Should include ai.repo.yaml and .metadata/sources.json
	if !strings.Contains(commitShow, "ai.repo.yaml") {
		t.Error("Manifest commit should include ai.repo.yaml")
	}
	if !strings.Contains(commitShow, ".metadata/sources.json") {
		t.Error("Manifest commit should include .metadata/sources.json")
	}
}

// TestRepoAdd_ManifestFailureHandling verifies graceful handling when manifest update fails
func TestRepoAdd_ManifestFailureHandling(t *testing.T) {
	testDir := t.TempDir()
	repoDir := filepath.Join(testDir, "repo")

	// Initialize repo programmatically
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Create test resource
	resourceDir := filepath.Join(testDir, "resources")
	createTestCommandInDir(t, resourceDir, "test-cmd", "Test command")

	// Make ai.repo.yaml read-only to cause manifest save to fail
	manifestPath := filepath.Join(repoDir, "ai.repo.yaml")
	if err := os.Chmod(manifestPath, 0444); err != nil {
		t.Fatalf("Failed to make manifest read-only: %v", err)
	}
	defer os.Chmod(manifestPath, 0644) // Cleanup

	// Try to add resource - should show warning but not fail
	t.Setenv("AIMGR_REPO_PATH", repoDir)
	output, err := runAimgr(t, "repo", "add", "local:"+resourceDir, "--name", "test-source")

	// The command might fail due to read-only manifest, which is expected
	if err == nil {
		// If it succeeded, at least verify the resource was added
		if !strings.Contains(output, "1 added") {
			t.Error("Expected resource to be added despite manifest warning")
		}
	} else {
		// If it failed, verify it's due to manifest issue
		if !strings.Contains(output, "Warning") && !strings.Contains(output, "manifest") {
			t.Errorf("Expected manifest-related warning, got: %s", output)
		}
	}
}
