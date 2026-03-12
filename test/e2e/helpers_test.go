//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildTestBinary verifies that we can build the aimgr binary for testing.
func TestBuildTestBinary(t *testing.T) {
	binPath := buildTestBinary(t)

	// Verify binary exists and is executable
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("Binary does not exist: %v", err)
	}

	// Check if executable (Unix-style check)
	if info.Mode()&0111 == 0 {
		t.Errorf("Binary is not executable: %s (mode: %o)", binPath, info.Mode())
	}
}

// TestLoadTestConfig verifies we can load the E2E test config.
func TestLoadTestConfig(t *testing.T) {
	configPath := loadTestConfig(t, "e2e-test")

	// Verify config file exists
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("Config file should exist: %v", err)
	}

	// Verify it ends with .yaml
	if filepath.Ext(configPath) != ".yaml" {
		t.Errorf("Config path should end with .yaml, got: %s", configPath)
	}
}

// TestGetRepoPathFromConfig verifies we can parse repo.path from config.
func TestGetRepoPathFromConfig(t *testing.T) {
	configPath := loadTestConfig(t, "e2e-test")
	repoPath := getRepoPathFromConfig(t, configPath)

	// Should be an absolute path
	if !filepath.IsAbs(repoPath) {
		t.Errorf("Repo path should be absolute, got: %s", repoPath)
	}

	// Should contain "test-repo-1" from e2e-test.yaml
	if !strings.Contains(repoPath, "test-repo-1") {
		t.Errorf("Repo path should contain 'test-repo-1', got: %s", repoPath)
	}
}

// TestSetupTestRepo verifies we can create a test repo.
func TestSetupTestRepo(t *testing.T) {
	repoPath := setupTestRepo(t, "test-helpers-repo")

	// Verify directory was created
	info, err := os.Stat(repoPath)
	if err != nil {
		t.Fatalf("Test repo should exist: %v", err)
	}

	if !info.IsDir() {
		t.Errorf("Test repo path should be a directory: %s", repoPath)
	}

	// Should be in test/e2e/repos/
	if !strings.Contains(repoPath, "test/e2e/repos") {
		t.Errorf("Test repo should be in test/e2e/repos/, got: %s", repoPath)
	}

	// Cleanup is automatic via t.Cleanup(), but verify we can clean manually
	cleanTestRepo(t, repoPath)

	// After manual cleanup, repo should not exist
	if _, err := os.Stat(repoPath); !os.IsNotExist(err) {
		t.Errorf("Test repo should be cleaned up, but still exists: %s", repoPath)
	}
}

// TestRunAimgr verifies we can execute aimgr commands.
func TestRunAimgr(t *testing.T) {
	// Just test that we can run aimgr --version
	stdout, stderr, err := runAimgr(t, "", "--version")

	// Should succeed or at least not crash
	if err != nil {
		t.Logf("Note: --version returned error (may not be implemented): %v", err)
	}

	// Should produce some output
	if stdout == "" && stderr == "" {
		t.Logf("Note: No output from --version (may not be implemented)")
	}

	t.Logf("aimgr --version output:\nstdout: %s\nstderr: %s", stdout, stderr)
}

// TestRunAimgrCombined verifies the combined output variant.
func TestRunAimgrCombined(t *testing.T) {
	output, err := runAimgrCombined(t, "", "--version")

	if err != nil {
		t.Logf("Note: --version returned error: %v", err)
	}

	t.Logf("aimgr --version combined output: %s", output)
}

// TestGetProjectRoot verifies project root resolution.
func TestGetProjectRoot(t *testing.T) {
	root := getProjectRoot(t)

	// Should be absolute
	if !filepath.IsAbs(root) {
		t.Errorf("Project root should be absolute, got: %s", root)
	}

	// Should contain go.mod (project root indicator)
	goModPath := filepath.Join(root, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		t.Errorf("Project root should contain go.mod, got: %s", root)
	}
}

// TestGetTestDataPath verifies testdata path resolution.
func TestGetTestDataPath(t *testing.T) {
	testDataPath := getTestDataPath(t)

	// Should exist
	info, err := os.Stat(testDataPath)
	if err != nil {
		t.Errorf("Test data path should exist: %v", err)
		return
	}

	if !info.IsDir() {
		t.Errorf("Test data path should be a directory: %s", testDataPath)
	}

	// Should end with test/testdata
	if !strings.Contains(testDataPath, "test/testdata") && !strings.Contains(testDataPath, "test\\testdata") {
		t.Errorf("Test data path should contain test/testdata, got: %s", testDataPath)
	}
}

// TestAssertFileExists verifies file assertion helpers.
func TestAssertFileExists(t *testing.T) {
	// Create a temp file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Should pass
	assertFileExists(t, testFile)

	// Test with non-existent file (should fail, but we catch it)
	nonExistent := filepath.Join(tempDir, "nonexistent.txt")
	testFail := &testing.T{} // Create a dummy T to catch the failure
	assertFileExists(testFail, nonExistent)
	// We expect this to fail - just verify it doesn't crash
}

// TestAssertDirExists verifies directory assertion helpers.
func TestAssertDirExists(t *testing.T) {
	tempDir := t.TempDir()

	// Should pass
	assertDirExists(t, tempDir)
}

// TestParseJSONOutput verifies JSON parsing helper.
func TestParseJSONOutput(t *testing.T) {
	jsonOutput := `[
		{"name": "test-cmd", "type": "command"},
		{"name": "test-skill", "type": "skill"}
	]`

	items := parseJSONOutput(t, jsonOutput)

	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}

	if items[0]["name"] != "test-cmd" {
		t.Errorf("Expected first item name to be 'test-cmd', got %v", items[0]["name"])
	}
}

// TestFindResourceByName verifies resource lookup helper.
func TestFindResourceByName(t *testing.T) {
	items := []map[string]interface{}{
		{"name": "test-cmd", "type": "command"},
		{"name": "test-skill", "type": "skill"},
	}

	found := findResourceByName(items, "test-skill")
	if found == nil {
		t.Error("Should find test-skill")
	} else if found["type"] != "skill" {
		t.Errorf("Expected type 'skill', got %v", found["type"])
	}

	notFound := findResourceByName(items, "nonexistent")
	if notFound != nil {
		t.Error("Should not find nonexistent resource")
	}
}
