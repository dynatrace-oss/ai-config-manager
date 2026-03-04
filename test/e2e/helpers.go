//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

// E2E test configuration
type E2EConfig struct {
	Install struct {
		Targets []string `yaml:"targets"`
	} `yaml:"install"`
	Repo struct {
		Path string `yaml:"path"`
	} `yaml:"repo"`
	Sync struct {
		Sources []struct {
			URL string `yaml:"url"`
		} `yaml:"sources"`
	} `yaml:"sync"`
}

var (
	// binaryPath caches the built binary path
	binaryPath string
	// buildOnce ensures we only build the binary once across all tests
	buildOnce sync.Once
	// buildErr stores any error from building
	buildErr error
)

// buildTestBinary builds the aimgr binary for testing and caches it.
// The binary is built once and reused across all E2E tests for performance.
// Returns the absolute path to the built binary.
func buildTestBinary(t *testing.T) string {
	t.Helper()

	buildOnce.Do(func() {
		// Build in a temp directory to avoid conflicts
		tempDir, err := os.MkdirTemp("", "aimgr-e2e-*")
		if err != nil {
			buildErr = fmt.Errorf("failed to create temp dir: %w", err)
			return
		}

		binPath := filepath.Join(tempDir, "aimgr")

		// Get project root (test/e2e -> test -> project root)
		projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			buildErr = fmt.Errorf("failed to get project root: %w", err)
			return
		}

		// Build the binary
		cmd := exec.Command("go", "build", "-o", binPath, "./cmd/aimgr/")
		cmd.Dir = projectRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("failed to build binary: %w\nOutput: %s", err, output)
			return
		}

		binaryPath = binPath
	})

	if buildErr != nil {
		t.Fatalf("Failed to build test binary: %v", buildErr)
	}

	return binaryPath
}

// runAimgr runs the aimgr CLI with the specified config and arguments.
// Returns stdout, stderr, and error.
//
// Example:
//
//	stdout, stderr, err := runAimgr(t, configPath, "repo", "list", "--format=json")
func runAimgr(t *testing.T, configPath string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	return runAimgrWithEnv(t, configPath, nil, args...)
}

// runAimgrWithEnv runs the aimgr CLI with the specified config, environment, and arguments.
// Returns stdout, stderr, and error.
//
// Example:
//
//	env := map[string]string{"AIMGR_REPO_PATH": "/tmp/repo"}
//	stdout, stderr, err := runAimgrWithEnv(t, configPath, env, "repo", "list")
func runAimgrWithEnv(t *testing.T, configPath string, env map[string]string, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	binPath := buildTestBinary(t)

	// Prepend --config flag if configPath provided
	var fullArgs []string
	if configPath != "" {
		fullArgs = append(fullArgs, "--config", configPath)
	}
	fullArgs = append(fullArgs, args...)

	cmd := exec.Command(binPath, fullArgs...)

	// Set environment variables
	if len(env) > 0 {
		cmd.Env = os.Environ() // Start with current environment
		for key, value := range env {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}

	// Capture stdout and stderr separately
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// runAimgrCombined runs aimgr and returns combined output (stdout + stderr).
// Useful when you don't need to distinguish between stdout and stderr.
//
// Example:
//
//	output, err := runAimgrCombined(t, configPath, "repo", "add", sourcePath)
func runAimgrCombined(t *testing.T, configPath string, args ...string) (string, error) {
	t.Helper()

	binPath := buildTestBinary(t)

	var fullArgs []string
	if configPath != "" {
		fullArgs = append(fullArgs, "--config", configPath)
	}
	fullArgs = append(fullArgs, args...)

	cmd := exec.Command(binPath, fullArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// setupTestRepo creates a test repository directory and returns its absolute path.
// The repo is created within test/e2e/repos/ and uses the provided name.
// Cleanup is handled automatically via t.Cleanup().
//
// Example:
//
//	repoPath := setupTestRepo(t, "test-sync-repo")
func setupTestRepo(t *testing.T, repoName string) string {
	t.Helper()

	// Get absolute path to test/e2e/repos/
	e2eDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("Failed to get E2E directory: %v", err)
	}

	reposDir := filepath.Join(e2eDir, "repos")
	repoPath := filepath.Join(reposDir, repoName)

	// Create the repo directory
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create test repo %s: %v", repoPath, err)
	}

	// Register cleanup
	t.Cleanup(func() {
		cleanTestRepo(t, repoPath)
	})

	return repoPath
}

// cleanTestRepo removes a test repository directory.
// This is typically called via t.Cleanup(), but can be called manually if needed.
func cleanTestRepo(t *testing.T, repoPath string) {
	t.Helper()

	if err := os.RemoveAll(repoPath); err != nil {
		t.Logf("Warning: failed to clean test repo %s: %v", repoPath, err)
	}
}

// loadTestConfig loads a config file from test/e2e/configs/ and returns its absolute path.
// The name parameter should not include the .yaml extension.
//
// Example:
//
//	configPath := loadTestConfig(t, "e2e-test")
//	// Returns: /abs/path/to/test/e2e/configs/e2e-test.yaml
func loadTestConfig(t *testing.T, name string) string {
	t.Helper()

	e2eDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("Failed to get E2E directory: %v", err)
	}

	configPath := filepath.Join(e2eDir, "configs", name+".yaml")

	// Verify config exists
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("Config file not found: %s", configPath)
	}

	return configPath
}

// getRepoPathFromConfig parses the repo.path from a config file.
// Returns the absolute path to the repository directory.
//
// Example:
//
//	repoPath := getRepoPathFromConfig(t, configPath)
func getRepoPathFromConfig(t *testing.T, configPath string) string {
	t.Helper()

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config %s: %v", configPath, err)
	}

	var config E2EConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config %s: %v", configPath, err)
	}

	if config.Repo.Path == "" {
		t.Fatalf("Config %s missing repo.path", configPath)
	}

	// Resolve relative paths from config file location
	repoPath := config.Repo.Path
	if !filepath.IsAbs(repoPath) {
		configDir := filepath.Dir(configPath)
		repoPath = filepath.Join(configDir, repoPath)
	}

	// Return absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		t.Fatalf("Failed to resolve repo path: %v", err)
	}

	return absPath
}

// assertFileExists asserts that a file exists at the given path.
func assertFileExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("File does not exist: %s", path)
		} else {
			t.Errorf("Failed to stat file %s: %v", path, err)
		}
		return
	}

	if info.IsDir() {
		t.Errorf("Path is a directory, not a file: %s", path)
	}
}

// assertDirExists asserts that a directory exists at the given path.
func assertDirExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("Directory does not exist: %s", path)
		} else {
			t.Errorf("Failed to stat directory %s: %v", path, err)
		}
		return
	}

	if !info.IsDir() {
		t.Errorf("Path is not a directory: %s", path)
	}
}

// assertDirContains asserts that a directory contains the expected files.
// expectedFiles should be relative paths from the directory.
//
// Example:
//
//	assertDirContains(t, "/path/to/repo/commands", []string{"test-cmd.md", "api/deploy.md"})
func assertDirContains(t *testing.T, dir string, expectedFiles []string) {
	t.Helper()

	for _, expectedFile := range expectedFiles {
		filePath := filepath.Join(dir, expectedFile)
		assertFileExists(t, filePath)
	}
}

// assertSymlinkExists asserts that a symlink exists at the given path.
func assertSymlinkExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Lstat(path) // Use Lstat to not follow symlinks
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("Symlink does not exist: %s", path)
		} else {
			t.Errorf("Failed to stat symlink %s: %v", path, err)
		}
		return
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Path is not a symlink: %s", path)
	}
}

// assertSymlinkTarget asserts that a symlink points to the expected target.
func assertSymlinkTarget(t *testing.T, symlinkPath, expectedTarget string) {
	t.Helper()

	assertSymlinkExists(t, symlinkPath)

	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink %s: %v", symlinkPath, err)
	}

	// Resolve both paths to absolute for comparison
	absTarget, err := filepath.Abs(target)
	if err != nil {
		t.Fatalf("Failed to resolve symlink target %s: %v", target, err)
	}

	absExpected, err := filepath.Abs(expectedTarget)
	if err != nil {
		t.Fatalf("Failed to resolve expected target %s: %v", expectedTarget, err)
	}

	if absTarget != absExpected {
		t.Errorf("Symlink target mismatch:\n  got:      %s\n  expected: %s", absTarget, absExpected)
	}
}

// parseJSONOutput parses JSON output from aimgr --format=json.
// Returns a slice of generic maps for flexible assertions.
//
// Example:
//
//	stdout, _, err := runAimgr(t, configPath, "repo", "list", "--format=json")
//	items := parseJSONOutput(t, stdout)
//	if len(items) != 3 {
//	    t.Errorf("expected 3 resources, got %d", len(items))
//	}
func parseJSONOutput(t *testing.T, output string) []map[string]interface{} {
	t.Helper()

	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &items); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	return items
}

// findResourceByName finds a resource in JSON output by name field.
// Returns the resource map or nil if not found.
func findResourceByName(items []map[string]interface{}, name string) map[string]interface{} {
	for _, item := range items {
		if itemName, ok := item["name"].(string); ok && itemName == name {
			return item
		}
	}
	return nil
}

// getProjectRoot returns the absolute path to the project root directory.
// This is useful for referencing test fixtures in test/testdata/.
func getProjectRoot(t *testing.T) string {
	t.Helper()

	// test/e2e -> test -> project root
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	return root
}

// getTestDataPath returns the absolute path to test/testdata/.
func getTestDataPath(t *testing.T) string {
	t.Helper()

	return filepath.Join(getProjectRoot(t), "test", "testdata")
}

// getTestRepoPath returns the absolute path to a fixture repo in test/testdata/repos/.
func getTestRepoPath(t *testing.T, repoName string) string {
	t.Helper()

	repoPath := filepath.Join(getTestDataPath(t), "repos", repoName)

	// Verify fixture exists
	if _, err := os.Stat(repoPath); err != nil {
		t.Fatalf("Test fixture repo not found: %s", repoPath)
	}

	return repoPath
}

// setupTestRepoWithSources creates ai.repo.yaml in the test repository with realistic GitHub sources.
// This configures the sources that repo sync will use to populate the repository.
//
// The sources include:
//   - ai-config-examples (uses local path to examples directory)
//
// Note: Using local `path:` source instead of remote `url:` to avoid network dependencies in E2E tests.
// The path source is a realistic production configuration for local development.
//
// Example:
//
//	repoPath := getRepoPathFromConfig(t, configPath)
//	setupTestRepoWithSources(t, repoPath)
//	// Now repo sync will use sources from ai.repo.yaml
func setupTestRepoWithSources(t *testing.T, repoPath string) {
	t.Helper()

	// Ensure repo directory exists
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}

	// Get absolute path to examples directory
	projectRoot := getProjectRoot(t)
	examplesPath := filepath.Join(projectRoot, "examples")

	// Create ai.repo.yaml with local path source
	// Using path instead of url for reliable E2E testing without network dependencies
	aiRepoYAML := fmt.Sprintf(`version: 1
sources:
  - name: ai-config-examples
    path: %s
`, examplesPath)

	aiRepoPath := filepath.Join(repoPath, "ai.repo.yaml")
	if err := os.WriteFile(aiRepoPath, []byte(aiRepoYAML), 0644); err != nil {
		t.Fatalf("Failed to write ai.repo.yaml: %v", err)
	}

	t.Logf("Created ai.repo.yaml at %s", aiRepoPath)
}
