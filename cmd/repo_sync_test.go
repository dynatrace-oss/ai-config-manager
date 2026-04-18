//go:build integration

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	resmeta "github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/sourcemetadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/workspace"
)

type fakeWorkspaceManager struct {
	cachePath     string
	getOrCloneN   int
	updateN       int
	getOrCloneErr error
	updateErr     error
	gotCloneURL   string
	gotRef        string
}

func (f *fakeWorkspaceManager) GetOrClone(url string, ref string) (string, error) {
	f.getOrCloneN++
	f.gotCloneURL = url
	f.gotRef = ref
	if f.getOrCloneErr != nil {
		return "", f.getOrCloneErr
	}
	return f.cachePath, nil
}

func (f *fakeWorkspaceManager) Update(url string, ref string) error {
	f.updateN++
	f.gotCloneURL = url
	f.gotRef = ref
	return f.updateErr
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}

}

func createRemoteGitSource(t *testing.T) (remoteURL string, worktreePath string) {
	t.Helper()

	baseDir := t.TempDir()
	bareRepo := filepath.Join(baseDir, "remote.git")
	worktreePath = filepath.Join(baseDir, "worktree")

	runGit(t, baseDir, "init", "--bare", bareRepo)
	runGit(t, bareRepo, "symbolic-ref", "HEAD", "refs/heads/main")
	runGit(t, baseDir, "clone", bareRepo, worktreePath)
	runGit(t, worktreePath, "branch", "-M", "main")

	return bareRepo, worktreePath
}

func writeAndCommitRemoteCommand(t *testing.T, worktreePath, name, description string) {
	t.Helper()

	cmdDir := filepath.Join(worktreePath, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}

	content := fmt.Sprintf("---\ndescription: %s\n---\n# %s\n", description, name)
	cmdPath := filepath.Join(cmdDir, name+".md")
	if err := os.WriteFile(cmdPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write command: %v", err)
	}

	runGit(t, worktreePath, "add", ".")
	runGit(t, worktreePath, "-c", "user.name=Test User", "-c", "user.email=test@example.com", "commit", "-m", fmt.Sprintf("add %s", name))
	runGit(t, worktreePath, "push", "origin", "main")
}

func writeAndCommitRemotePluginDirectoryMarketplace(t *testing.T, worktreePath, marketplaceDirectory string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(worktreePath, "commands"), 0755); err != nil {
		t.Fatalf("failed to create loose commands dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(worktreePath, "plugins", "dt-github", "commands"), 0755); err != nil {
		t.Fatalf("failed to create plugin commands dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(worktreePath, marketplaceDirectory), 0755); err != nil {
		t.Fatalf("failed to create marketplace directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(worktreePath, "commands", "loose-command.md"), []byte("---\ndescription: loose\n---\n# loose-command\n"), 0644); err != nil {
		t.Fatalf("failed to write loose command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, "plugins", "dt-github", "commands", "plugin-command.md"), []byte("---\ndescription: plugin\n---\n# plugin-command\n"), 0644); err != nil {
		t.Fatalf("failed to write plugin command: %v", err)
	}

	marketplaceContent := `{
		"name": "plugin-manifest",
		"description": "plugin dir manifest",
		"plugins": [
			{
				"name": "dt-github",
				"description": "test plugin",
				"source": "plugins/dt-github"
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(worktreePath, marketplaceDirectory, "marketplace.json"), []byte(marketplaceContent), 0644); err != nil {
		t.Fatalf("failed to write marketplace manifest: %v", err)
	}

	runGit(t, worktreePath, "add", ".")
	runGit(t, worktreePath, "-c", "user.name=Test User", "-c", "user.email=test@example.com", "commit", "-m", "add plugin-dir marketplace")
	runGit(t, worktreePath, "push", "origin", "main")
}

// Test helper: create a minimal test source directory with resources
func createTestSource(t *testing.T) string {
	t.Helper()

	sourceDir := t.TempDir()

	// Create directory structure
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "skills"), 0755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "agents"), 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}

	// Create test commands
	testCommands := []struct {
		name    string
		content string
	}{
		{"sync-test-cmd", "Sync test command"},
		{"test-command", "Test command"},
		{"pdf-command", "PDF command"},
	}
	for _, cmd := range testCommands {
		content := fmt.Sprintf("---\ndescription: %s\n---\n# %s", cmd.content, cmd.name)
		path := filepath.Join(sourceDir, "commands", cmd.name+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create command %s: %v", cmd.name, err)
		}
	}

	// Create test skills
	testSkills := []struct {
		name    string
		content string
	}{
		{"sync-test-skill", "Sync test skill"},
		{"pdf-processing", "PDF processing skill"},
		{"image-processing", "Image processing skill"},
	}
	for _, skill := range testSkills {
		skillDir := filepath.Join(sourceDir, "skills", skill.name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("failed to create skill dir %s: %v", skill.name, err)
		}
		content := fmt.Sprintf("---\ndescription: %s\n---\n# %s", skill.content, skill.name)
		path := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create skill %s: %v", skill.name, err)
		}
	}

	// Create test agents
	testAgents := []struct {
		name    string
		content string
	}{
		{"sync-test-agent", "Sync test agent"},
		{"code-reviewer", "Code reviewer agent"},
	}
	for _, agent := range testAgents {
		content := fmt.Sprintf("---\ndescription: %s\n---\n# %s", agent.content, agent.name)
		path := filepath.Join(sourceDir, "agents", agent.name+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create agent %s: %v", agent.name, err)
		}
	}

	return sourceDir
}

func createNestedCommandTestSource(t *testing.T) string {
	t.Helper()

	sourceDir := t.TempDir()
	commandDir := filepath.Join(sourceDir, "commands", "opencode-coder")
	if err := os.MkdirAll(commandDir, 0755); err != nil {
		t.Fatalf("failed to create nested command dir: %v", err)
	}

	content := "---\ndescription: nested command\n---\n# status\n"
	path := filepath.Join(commandDir, "status.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create nested command: %v", err)
	}

	return sourceDir
}

func createSourceWithInvalidMarketplace(t *testing.T) string {
	t.Helper()

	sourceDir := t.TempDir()
	cmdDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}

	commandContent := "---\ndescription: Discovery command\n---\n# discovery-command\n"
	if err := os.WriteFile(filepath.Join(cmdDir, "discovery-command.md"), []byte(commandContent), 0644); err != nil {
		t.Fatalf("failed to write command: %v", err)
	}

	// Intentionally invalid JSON to test whether discovery mode attempts marketplace parsing.
	marketplaceContent := `{"name": "broken-marketplace", "plugins": [`
	if err := os.WriteFile(filepath.Join(sourceDir, "marketplace.json"), []byte(marketplaceContent), 0644); err != nil {
		t.Fatalf("failed to write marketplace.json: %v", err)
	}

	return sourceDir
}

func createSourceWithMalformedAgentAndValidCommand(t *testing.T) string {
	t.Helper()

	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "agents"), 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}

	badAgent := "---\ndescription: malformed agent\npermissions:\n  bash: \"*\": ask\n---\n# malformed\n"
	if err := os.WriteFile(filepath.Join(sourceDir, "agents", "broken-agent.md"), []byte(badAgent), 0644); err != nil {
		t.Fatalf("failed to write malformed agent: %v", err)
	}

	goodCommand := "---\ndescription: valid command\n---\n# valid-command\n"
	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "valid-command.md"), []byte(goodCommand), 0644); err != nil {
		t.Fatalf("failed to write valid command: %v", err)
	}

	return sourceDir
}

func createSourceWithMarketplaceAndLooseResources(t *testing.T) string {
	t.Helper()

	sourceDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "skills", "loose-skill"), 0755); err != nil {
		t.Fatalf("failed to create loose skills dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "plugins", "market-plugin", "commands"), 0755); err != nil {
		t.Fatalf("failed to create plugin commands dir: %v", err)
	}

	looseCommand := "---\ndescription: loose command\n---\n# loose-command\n"
	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "loose-command.md"), []byte(looseCommand), 0644); err != nil {
		t.Fatalf("failed to write loose command: %v", err)
	}

	looseSkill := "---\ndescription: loose skill\n---\n# loose-skill\n"
	if err := os.WriteFile(filepath.Join(sourceDir, "skills", "loose-skill", "SKILL.md"), []byte(looseSkill), 0644); err != nil {
		t.Fatalf("failed to write loose skill: %v", err)
	}

	plugCommand := "---\ndescription: plugin command\n---\n# plugin-command\n"
	if err := os.WriteFile(filepath.Join(sourceDir, "plugins", "market-plugin", "commands", "plugin-command.md"), []byte(plugCommand), 0644); err != nil {
		t.Fatalf("failed to write plugin command: %v", err)
	}

	marketplaceContent := `{
		"name": "hybrid-source",
		"description": "contains loose and marketplace resources",
		"plugins": [
			{
				"name": "market-plugin",
				"description": "plugin resources",
				"source": "plugins/market-plugin"
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(sourceDir, "marketplace.json"), []byte(marketplaceContent), 0644); err != nil {
		t.Fatalf("failed to write marketplace.json: %v", err)
	}

	return sourceDir
}

func createSourceWithMarketplaceNoResolvablePlugins(t *testing.T) string {
	t.Helper()

	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "fallback-command.md"), []byte("---\ndescription: fallback command\n---\n# fallback-command\n"), 0644); err != nil {
		t.Fatalf("failed to write fallback command: %v", err)
	}

	marketplaceContent := `{
		"name": "empty-marketplace",
		"description": "no resolvable plugins",
		"plugins": [
			{
				"name": "missing-plugin",
				"description": "missing source",
				"source": "plugins/does-not-exist"
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(sourceDir, "marketplace.json"), []byte(marketplaceContent), 0644); err != nil {
		t.Fatalf("failed to write marketplace.json: %v", err)
	}

	return sourceDir
}

func TestRunSync_MissingRepoFailsBeforeCreatingLockState(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)
	if err := os.RemoveAll(repoPath); err != nil {
		t.Fatalf("failed to remove repo path: %v", err)
	}

	err := runSync(syncCmd, []string{})
	if err == nil {
		t.Fatal("expected error for missing repo")
	}
	if !strings.Contains(err.Error(), "run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(repoPath, ".workspace")); !os.IsNotExist(statErr) {
		t.Fatalf("expected missing repo path to remain untouched, stat err: %v", statErr)
	}
}

// setupTestManifest creates a test repository with ai.repo.yaml
// Returns the repo path for verification and a cleanup function
func setupTestManifest(t *testing.T, sources []*repomanifest.Source) (string, func()) {
	t.Helper()

	// Create temp directory for repo
	repoPath := t.TempDir()

	// Create manifest
	manifest := &repomanifest.Manifest{
		Version: 1,
		Sources: sources,
	}

	// Save manifest to repo
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to save test manifest: %v", err)
	}

	// Use AIMGR_REPO_PATH to override repo location
	// This is the highest priority override in repo.NewManager()
	originalRepoPath := os.Getenv("AIMGR_REPO_PATH")
	os.Setenv("AIMGR_REPO_PATH", repoPath)

	cleanup := func() {
		if originalRepoPath != "" {
			os.Setenv("AIMGR_REPO_PATH", originalRepoPath)
		} else {
			_ = os.Unsetenv("AIMGR_REPO_PATH")
		}
	}

	return repoPath, cleanup
}

// verifySourceSynced checks that a source was successfully synced
// Verifies that last_synced timestamp was updated
func verifySourceSynced(t *testing.T, repoPath string, sourceName string) {
	t.Helper()

	// Load manifest
	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	// Find source
	_, found := manifest.GetSource(sourceName)
	if !found {
		t.Fatalf("source %s not found in manifest", sourceName)
	}

	// Load source metadata to check last_synced
	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load source metadata: %v", err)
	}

	state := metadata.Get(sourceName)
	if state == nil {
		t.Fatalf("source %s state not found in metadata", sourceName)
	}

	// Verify last_synced was updated
	if state.LastSynced.IsZero() {
		t.Errorf("source %s last_synced timestamp not updated", sourceName)
	}

	// Verify timestamp is recent (within last minute)
	if time.Since(state.LastSynced) > time.Minute {
		t.Errorf("source %s last_synced timestamp is too old: %v", sourceName, state.LastSynced)
	}

}

// Test helper: verify resources exist in repo
func verifyResourcesInRepo(t *testing.T, repoPath string, resourceType resource.ResourceType, names ...string) {
	t.Helper()

	for _, name := range names {
		var path string
		switch resourceType {
		case resource.Command:
			path = filepath.Join(repoPath, "commands", name+".md")
		case resource.Skill:
			path = filepath.Join(repoPath, "skills", name)
		case resource.Agent:
			path = filepath.Join(repoPath, "agents", name+".md")
		}

		if _, err := os.Stat(path); err != nil {
			t.Errorf("resource %s/%s not found in repo at %s: %v", resourceType, name, path, err)
		}
	}
}

// Test helper: verify resources do NOT exist in repo
func verifyResourcesNotInRepo(t *testing.T, repoPath string, resourceType resource.ResourceType, names ...string) {
	t.Helper()

	for _, name := range names {
		var path string
		switch resourceType {
		case resource.Command:
			path = filepath.Join(repoPath, "commands", name+".md")
		case resource.Skill:
			path = filepath.Join(repoPath, "skills", name)
		case resource.Agent:
			path = filepath.Join(repoPath, "agents", name+".md")
		}

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("resource %s/%s should not exist in repo at %s", resourceType, name, path)
		}
	}
}

func verifyPackagesInRepo(t *testing.T, repoPath string, names ...string) {
	t.Helper()

	for _, name := range names {
		path := filepath.Join(repoPath, "packages", name+".package.json")
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("package %s not found in repo at %s: %v", name, path, err)
		}
	}
}

func verifyPackagesNotInRepo(t *testing.T, repoPath string, names ...string) {
	t.Helper()

	for _, name := range names {
		path := filepath.Join(repoPath, "packages", name+".package.json")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("package %s should not exist in repo at %s", name, path)
		}
	}
}

// TestRunSync_SingleSource tests syncing from a single local source
func TestRunSync_SingleSource(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest with one source
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Verify resources were imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Verify source was marked as synced
	verifySourceSynced(t, repoPath, "test-source-1")
}

// TestRunSync_MultipleSources tests syncing from multiple sources
func TestRunSync_MultipleSources(t *testing.T) {
	source1 := createTestSource(t)
	source2 := t.TempDir()
	cmdDir2 := filepath.Join(source2, "commands")
	if err := os.MkdirAll(cmdDir2, 0755); err != nil {
		t.Fatalf("failed to create commands dir for source2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir2, "source2-only.md"), []byte("---\ndescription: source2 only\n---\n# source2-only"), 0644); err != nil {
		t.Fatalf("failed to create source2 command: %v", err)
	}

	// Create manifest with multiple sources
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
		{
			Name: "test-source-2",
			Path: source2,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Verify resources from both sources were imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Command, "source2-only")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Verify both sources were marked as synced
	verifySourceSynced(t, repoPath, "test-source-1")
	verifySourceSynced(t, repoPath, "test-source-2")
}

// TestRunSync_DryRun tests that --dry-run doesn't actually import
func TestRunSync_DryRun(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command with --dry-run
	syncDryRunFlag = true
	defer func() { syncDryRunFlag = false }()
	err := runSync(syncCmd, []string{})

	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Verify NO resources were actually imported (dry run)
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesNotInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesNotInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Verify last_synced was NOT updated (dry run)
	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	_, found := manifest.GetSource("test-source-1")
	if !found {
		t.Fatalf("source not found")
	}
	// Check that last_synced was not updated in metadata
	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load source metadata: %v", err)
	}
	state := metadata.Get("test-source-1")
	if state != nil && !state.LastSynced.IsZero() {
		t.Errorf("dry run should not update last_synced timestamp")
	}
}

// TestRunSync_SkipExisting tests that --skip-existing doesn't overwrite
func TestRunSync_SkipExisting(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Pre-populate repo with one resource (modified version)
	existingCmdDir := filepath.Join(repoPath, "commands")
	if err := os.MkdirAll(existingCmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	existingContent := "---\ndescription: EXISTING VERSION\n---\n# existing"
	existingPath := filepath.Join(existingCmdDir, "sync-test-cmd.md")
	if err := os.WriteFile(existingPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create existing resource: %v", err)
	}

	// Read original content
	originalData, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("failed to read existing resource: %v", err)
	}
	originalContent := string(originalData)

	// Run sync command with --skip-existing
	syncSkipExistingFlag = true
	defer func() { syncSkipExistingFlag = false }()
	err = runSync(syncCmd, []string{})

	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Verify existing resource was NOT overwritten
	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("failed to read resource after sync: %v", err)
	}
	newContent := string(data)

	if newContent != originalContent {
		t.Errorf("--skip-existing failed: existing resource was overwritten")
		t.Logf("Original: %s", originalContent)
		t.Logf("After sync: %s", newContent)
	}

	// Verify other resources were imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Verify source was marked as synced
	verifySourceSynced(t, repoPath, "test-source-1")
}

// TestRunSync_DefaultForce tests that by default, existing resources are overwritten
func TestRunSync_DefaultForce(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Pre-populate repo with one resource (modified version)
	existingCmdDir := filepath.Join(repoPath, "commands")
	if err := os.MkdirAll(existingCmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	existingContent := "---\ndescription: EXISTING VERSION\n---\n# existing"
	existingPath := filepath.Join(existingCmdDir, "sync-test-cmd.md")
	if err := os.WriteFile(existingPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to create existing resource: %v", err)
	}

	// Read original content
	originalData, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("failed to read existing resource: %v", err)
	}
	originalContent := string(originalData)

	// Run sync command (default force behavior)
	err = runSync(syncCmd, []string{})

	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Verify existing resource WAS overwritten (force is default)
	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("failed to read resource after sync: %v", err)
	}
	newContent := string(data)

	if newContent == originalContent {
		t.Errorf("default force failed: existing resource was not overwritten")
		t.Logf("Content unchanged: %s", newContent)
	}

	// Verify it contains the new content from source
	if !strings.Contains(newContent, "Sync test command") {
		t.Errorf("resource content doesn't match source after force update")
		t.Logf("Content: %s", newContent)
	}

	// Verify all resources were imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Verify source was marked as synced
	verifySourceSynced(t, repoPath, "test-source-1")
}

// TestRunSync_NoSources tests error when no sources configured
func TestRunSync_NoSources(t *testing.T) {
	// Create manifest with NO sources
	sources := []*repomanifest.Source{}
	_, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command
	err := runSync(syncCmd, []string{})

	// Should return error
	if err == nil {
		t.Fatal("expected error when no sources configured, got nil")
	}
	if got := getCommandExitCode(err); got != commandExitCodeOperationalFailure {
		t.Fatalf("exit code=%d want %d", got, commandExitCodeOperationalFailure)
	}
	var cmdErr *commandExitError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected commandExitError, got %T", err)
	}

	// Check error message
	expectedMsg := "no sync sources configured"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("error message doesn't contain expected text\nGot: %s\nWant substring: %s", err.Error(), expectedMsg)
	}
}

// TestRunSync_InvalidSource tests error handling for invalid sources
func TestRunSync_InvalidSource(t *testing.T) {
	// Create manifest with invalid source (non-existent path)
	sources := []*repomanifest.Source{
		{
			Name: "invalid-source",
			Path: "/nonexistent/path/that/does/not/exist",
		},
	}
	_, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command
	err := runSync(syncCmd, []string{})

	// Should return completed-with-findings error since all sources failed
	if err == nil {
		t.Fatal("expected error for invalid source, got nil")
	}
	if got := getCommandExitCode(err); got != commandExitCodeCompletedWithFindings {
		t.Fatalf("exit code=%d want %d", got, commandExitCodeCompletedWithFindings)
	}
	var cmdErr *commandExitError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected commandExitError, got %T", err)
	}

	// Check error message indicates failure
	if !strings.Contains(err.Error(), "all sources failed") {
		t.Errorf("error message doesn't indicate all sources failed\nGot: %s", err.Error())
	}
}

// TestRunSync_MixedValidInvalidSources tests partial success with mixed sources
func TestRunSync_MixedValidInvalidSources(t *testing.T) {
	validSource := createTestSource(t)

	// Create manifest with one valid and one invalid source
	sources := []*repomanifest.Source{
		{
			Name: "valid-source",
			Path: validSource,
		},
		{
			Name: "invalid-source",
			Path: "/nonexistent/path",
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command
	err := runSync(syncCmd, []string{})

	// Should return completed-with-findings error on partial source failure
	if err == nil {
		t.Fatal("expected completed-with-findings error for partial source failure, got nil")
	}
	if got := getCommandExitCode(err); got != commandExitCodeCompletedWithFindings {
		t.Fatalf("exit code=%d want %d", got, commandExitCodeCompletedWithFindings)
	}
	var cmdErr *commandExitError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected commandExitError, got %T", err)
	}

	// Verify resources from valid source were imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Verify valid source was marked as synced
	verifySourceSynced(t, repoPath, "valid-source")

	// Verify invalid source was NOT marked as synced
	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	_, found := manifest.GetSource("invalid-source")
	if !found {
		t.Fatalf("invalid-source not found in manifest")
	}
	// Check that invalid source has no last_synced in metadata
	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load source metadata: %v", err)
	}
	state := metadata.Get("invalid-source")
	if state != nil && !state.LastSynced.IsZero() {
		t.Errorf("invalid source should not have last_synced timestamp updated")
	}
}

// TestRunSync_UpdatesLastSynced tests that last_synced timestamps are updated
func TestRunSync_UpdatesLastSynced(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest with old last_synced timestamp
	oldTimestamp := time.Now().Add(-24 * time.Hour)
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Run sync command
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Load manifest and verify timestamp was updated
	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	_, found := manifest.GetSource("test-source-1")
	if !found {
		t.Fatalf("source not found")
	}

	// Verify timestamp was updated (should be recent, not old)
	// Check via metadata
	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	state := metadata.Get("test-source-1")
	if state == nil {
		t.Fatalf("state not found")
	}
	if state.LastSynced.Equal(oldTimestamp) {
		t.Errorf("last_synced timestamp was not updated")
	}
	if time.Since(state.LastSynced) > time.Minute {
		t.Errorf("last_synced timestamp is not recent: %v", state.LastSynced)
	}
}

// TestRunSync_PreservesManifest tests that manifest structure is preserved
func TestRunSync_PreservesManifest(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest with specific structure
	addedTime := time.Now().Add(-48 * time.Hour)
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Create metadata with specific added time
	metadata := &sourcemetadata.SourceMetadata{
		Version: 1,
		Sources: map[string]*sourcemetadata.SourceState{
			"test-source-1": {
				Added:      addedTime,
				LastSynced: time.Time{}, // Not yet synced
			},
		},
	}
	if err := metadata.Save(repoPath); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	// Run sync command
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Load manifest and verify structure is preserved
	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	// Verify version
	if manifest.Version != 1 {
		t.Errorf("manifest version changed: expected 1, got %d", manifest.Version)
	}

	// Verify source count
	if len(manifest.Sources) != 1 {
		t.Errorf("source count changed: expected 1, got %d", len(manifest.Sources))
	}

	// Verify source fields are preserved
	source := manifest.Sources[0]
	if source.Name != "test-source-1" {
		t.Errorf("source name changed: expected test-source-1, got %s", source.Name)
	}
	if source.Path != source1 {
		t.Errorf("source path changed: expected %s, got %s", source1, source.Path)
	}
	if source.GetMode() != "symlink" {
		t.Errorf("source mode changed: expected symlink, got %s", source.GetMode())
	}

	// Verify added timestamp is preserved in metadata
	loadedMetadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	state := loadedMetadata.Get("test-source-1")
	if state == nil {
		t.Fatalf("state not found")
	}
	if state.Added.Sub(addedTime).Abs() > time.Second {
		t.Errorf("added timestamp changed: expected %v, got %v", addedTime, state.Added)
	}

	// Verify last_synced was added
	if state.LastSynced.IsZero() {
		t.Errorf("last_synced timestamp was not added")
	}
}

// TestRunSync_WithIncludeFilter verifies that a source with include filters
// only imports matching resources and skips non-matching ones.
func TestRunSync_WithIncludeFilter(t *testing.T) {
	sourceDir := createTestSource(t)

	// Source has include: only import pdf-command and the pdf-processing skill
	sources := []*repomanifest.Source{
		{
			Name:    "filtered-source",
			Path:    sourceDir,
			ID:      "src-filter-test",
			Include: []string{"command/pdf-command", "skill/pdf-processing"},
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync with include filter failed: %v", err)
	}

	// Only the included resources should be imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "pdf-processing")

	// Non-matching resources should NOT be imported
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command")
	verifyResourcesNotInRepo(t, repoPath, resource.Skill, "sync-test-skill", "image-processing")
	verifyResourcesNotInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")
}

// TestRunSync_WithIncludeFilter_BackwardCompat verifies that a source without
// include filters imports all resources (backward compatibility).
func TestRunSync_WithIncludeFilter_BackwardCompat(t *testing.T) {
	sourceDir := createTestSource(t)

	// Source has NO include filter — all resources should be imported
	sources := []*repomanifest.Source{
		{
			Name: "unfiltered-source",
			Path: sourceDir,
			ID:   "src-no-filter",
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync without include filter failed: %v", err)
	}

	// All resources from the source should be imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")
}

// TestRunSync_IncludeFilterOrphanDetection verifies that a resource previously
// imported (included) is removed from the repo when it is removed from include
// and a sync is performed.
func TestRunSync_IncludeFilterOrphanDetection(t *testing.T) {
	sourceDir := createTestSource(t)

	// First sync: include both pdf-command and test-command
	sources := []*repomanifest.Source{
		{
			Name:    "shrinking-filter-source",
			Path:    sourceDir,
			ID:      "src-shrink-filter",
			Include: []string{"command/pdf-command", "command/test-command"},
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Both included commands should be in the repo
	verifyResourcesInRepo(t, repoPath, resource.Command, "pdf-command", "test-command")

	// Non-included resources should not be present
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "sync-test-cmd")
	verifyResourcesNotInRepo(t, repoPath, resource.Skill, "pdf-processing")

	// Now narrow the include to only pdf-command (remove test-command from include)
	sources[0].Include = []string{"command/pdf-command"}
	manifest := &repomanifest.Manifest{Version: 1, Sources: sources}
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to update manifest with narrowed include: %v", err)
	}

	// Second sync: test-command was previously imported but is now outside include
	syncPruneFlag = true
	defer func() { syncPruneFlag = false }()
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// pdf-command is still included — must remain in repo
	verifyResourcesInRepo(t, repoPath, resource.Command, "pdf-command")

	// test-command is no longer in include — must be removed as orphan
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "test-command")
}

func TestRunSync_DefaultDoesNotPruneWhenIncludeNarrows(t *testing.T) {
	sourceDir := createTestSource(t)

	sources := []*repomanifest.Source{{
		Name:    "non-prune-source",
		Path:    sourceDir,
		ID:      "src-non-prune",
		Include: []string{"command/pdf-command", "command/test-command"},
	}}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	verifyResourcesInRepo(t, repoPath, resource.Command, "pdf-command", "test-command")

	sources[0].Include = []string{"command/pdf-command"}
	manifest := &repomanifest.Manifest{Version: 1, Sources: sources}
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to save narrowed include manifest: %v", err)
	}

	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	verifyResourcesInRepo(t, repoPath, resource.Command, "pdf-command", "test-command")
}

// TestRunSync_MetadataCommitted verifies that metadata changes are committed to git
// This is a regression test for bug ai-config-manager-x7k
func TestRunSync_MetadataCommitted(t *testing.T) {
	source1 := createTestSource(t)

	// Create manifest with one source
	sources := []*repomanifest.Source{
		{
			Name: "test-source-1",
			Path: source1,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Initialize as git repository
	manager, err := repo.NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to initialize git repo: %v", err)
	}

	// Run sync command
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// Verify git status is clean (bug fix verification)
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get git status: %v", err)
	}

	status := string(output)
	if status != "" {
		t.Errorf("Expected clean working tree after repo sync, but got uncommitted changes:\n%s", status)
		t.Error("This indicates metadata changes were not committed (bug ai-config-manager-x7k)")
	}

	// Verify we have a commit for metadata update
	cmd = exec.Command("git", "-C", repoPath, "log", "--oneline")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get git log: %v", err)
	}

	logOutput := string(output)
	expectedCommitMsg := "aimgr: update sync timestamps"
	if !strings.Contains(logOutput, expectedCommitMsg) {
		t.Errorf("Expected commit message %q not found in git log:\n%s", expectedCommitMsg, logOutput)
	}

	// Verify the commit contains .metadata/sources.json
	cmd = exec.Command("git", "-C", repoPath, "show", "--stat", "--oneline", "HEAD")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to show latest commit: %v", err)
	}

	commitShow := string(output)
	if !strings.Contains(commitShow, ".metadata/sources.json") {
		t.Error("Metadata commit should include .metadata/sources.json")
	}
}

func TestRunSync_MetadataCommitIsScopedToSourcesMetadata(t *testing.T) {
	source1 := createTestSource(t)

	sources := []*repomanifest.Source{{
		Name: "test-source-1",
		Path: source1,
	}}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	manager, err := repo.NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to initialize git repo: %v", err)
	}

	unrelated := filepath.Join(repoPath, "sync-unrelated.txt")
	if err := os.WriteFile(unrelated, []byte("baseline\n"), 0644); err != nil {
		t.Fatalf("failed to write unrelated file: %v", err)
	}
	if err := manager.CommitChangesForPaths("test: add unrelated file", []string{"sync-unrelated.txt"}); err != nil {
		t.Fatalf("failed to commit unrelated baseline: %v", err)
	}
	if err := os.WriteFile(unrelated, []byte("baseline\nlocal change\n"), 0644); err != nil {
		t.Fatalf("failed to modify unrelated file: %v", err)
	}

	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	showCmd := exec.Command("git", "-C", repoPath, "show", "--name-only", "--pretty=format:", "HEAD")
	showOut, err := showCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to inspect latest commit: %v", err)
	}
	commitFiles := string(showOut)
	if !strings.Contains(commitFiles, ".metadata/sources.json") {
		t.Fatalf("expected sync metadata commit to include .metadata/sources.json, got:\n%s", commitFiles)
	}
	if strings.Contains(commitFiles, "sync-unrelated.txt") {
		t.Fatalf("sync metadata commit should not include unrelated file, got:\n%s", commitFiles)
	}

	statusCmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	statusOut, err := statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get git status: %v", err)
	}
	status := string(statusOut)
	if !strings.Contains(status, " M sync-unrelated.txt") {
		t.Fatalf("expected unrelated file to remain modified after sync, status:\n%s", status)
	}
}

// TestScanSourceResources_DiscoversAllTypes tests that scanSourceResources
// finds all four resource types (commands, skills, agents, packages)
func TestScanSourceResources_DiscoversAllTypes(t *testing.T) {
	sourceDir := createTestSource(t) // creates commands, skills, agents

	// Also add a package to the source
	pkgDir := filepath.Join(sourceDir, "packages")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create packages dir: %v", err)
	}
	pkgContent := `{"name": "test-pkg", "description": "A test package", "resources": ["command/sync-test-cmd"]}`
	if err := os.WriteFile(filepath.Join(pkgDir, "test-pkg.package.json"), []byte(pkgContent), 0644); err != nil {
		t.Fatalf("failed to create package file: %v", err)
	}

	result, err := scanSourceResources(sourceDir, repomanifest.DiscoveryModeAuto)
	if err != nil {
		t.Fatalf("scanSourceResources failed: %v", err)
	}

	// Verify commands found
	cmdSet := result[resource.Command]
	if cmdSet == nil {
		t.Fatal("no commands found")
	}
	expectedCmds := []string{"sync-test-cmd", "test-command", "pdf-command"}
	for _, name := range expectedCmds {
		if !cmdSet[name] {
			t.Errorf("command %q not found in scan results", name)
		}
	}

	// Verify skills found
	skillSet := result[resource.Skill]
	if skillSet == nil {
		t.Fatal("no skills found")
	}
	expectedSkills := []string{"sync-test-skill", "pdf-processing", "image-processing"}
	for _, name := range expectedSkills {
		if !skillSet[name] {
			t.Errorf("skill %q not found in scan results", name)
		}
	}

	// Verify agents found
	agentSet := result[resource.Agent]
	if agentSet == nil {
		t.Fatal("no agents found")
	}
	expectedAgents := []string{"sync-test-agent", "code-reviewer"}
	for _, name := range expectedAgents {
		if !agentSet[name] {
			t.Errorf("agent %q not found in scan results", name)
		}
	}

	// Verify packages found
	pkgSet := result[resource.PackageType]
	if pkgSet == nil {
		t.Fatal("no packages found")
	}
	if !pkgSet["test-pkg"] {
		t.Error("package 'test-pkg' not found in scan results")
	}
}

// TestScanSourceResources_EmptySource tests scanning an empty source directory
func TestScanSourceResources_EmptySource(t *testing.T) {
	emptyDir := t.TempDir()

	result, err := scanSourceResources(emptyDir, repomanifest.DiscoveryModeAuto)
	if err != nil {
		t.Fatalf("scanSourceResources failed on empty dir: %v", err)
	}

	// All type sets should be nil or empty
	for resType, typeSet := range result {
		if len(typeSet) > 0 {
			t.Errorf("expected no resources of type %s in empty dir, got %d", resType, len(typeSet))
		}
	}
}

// TestScanSourceResources_PartialTypes tests scanning a source with only some types
func TestScanSourceResources_PartialTypes(t *testing.T) {
	sourceDir := t.TempDir()

	// Create only commands (no skills, agents, or packages)
	cmdDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	content := "---\ndescription: A test command\n---\n# only-command"
	if err := os.WriteFile(filepath.Join(cmdDir, "only-command.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	result, err := scanSourceResources(sourceDir, repomanifest.DiscoveryModeAuto)
	if err != nil {
		t.Fatalf("scanSourceResources failed: %v", err)
	}

	// Verify commands found
	cmdSet := result[resource.Command]
	if cmdSet == nil || !cmdSet["only-command"] {
		t.Error("command 'only-command' not found in scan results")
	}

	// Verify other types NOT found
	if result[resource.Skill] != nil {
		t.Error("expected no skills, but found some")
	}
	if result[resource.Agent] != nil {
		t.Error("expected no agents, but found some")
	}
	if result[resource.PackageType] != nil {
		t.Error("expected no packages, but found some")
	}
}

// TestRunSync_DetectsRemovedResources tests that sync detects and removes resources
// deleted from a source between syncs (bmz1.2 detection + bmz1.3 removal)
func TestRunSync_DetectsRemovedResources(t *testing.T) {
	sourceDir := createTestSource(t)

	// Set up source with an ID so pre-sync inventory matches by ID
	sourceID := "src-test-detect-rm"
	sources := []*repomanifest.Source{
		{
			Name: "test-source",
			Path: sourceDir,
			ID:   sourceID,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// First sync: import all resources
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Verify all resources were imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Now remove a command and a skill from the source
	if err := os.Remove(filepath.Join(sourceDir, "commands", "pdf-command.md")); err != nil {
		t.Fatalf("failed to remove command from source: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(sourceDir, "skills", "image-processing")); err != nil {
		t.Fatalf("failed to remove skill from source: %v", err)
	}

	// Second sync: should detect AND remove the orphans (bmz1.3)
	syncPruneFlag = true
	defer func() { syncPruneFlag = false }()
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// Resources that were NOT removed from source should still be in repo
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// The removed resources should now be gone from the repo (bmz1.3 removes them)
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "pdf-command")
	verifyResourcesNotInRepo(t, repoPath, resource.Skill, "image-processing")
}

// TestRunSync_FailedSourceExcludedFromRemovalDetection tests that
// resources from failed sources are not flagged for removal
func TestRunSync_FailedSourceExcludedFromRemovalDetection(t *testing.T) {
	validSource := createTestSource(t)

	// Create manifest with one valid and one invalid source
	sources := []*repomanifest.Source{
		{
			Name: "valid-source",
			Path: validSource,
			ID:   "src-valid",
		},
		{
			Name: "will-fail-source",
			Path: "/nonexistent/path/that/does/not/exist",
			ID:   "src-will-fail",
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// First sync: only valid source succeeds
	err := runSync(syncCmd, []string{})
	if err == nil {
		t.Fatal("expected completed-with-findings error for partial source failure, got nil")
	}
	if got := getCommandExitCode(err); got != commandExitCodeCompletedWithFindings {
		t.Fatalf("first sync exit code=%d want %d", got, commandExitCodeCompletedWithFindings)
	}

	// Verify resources from valid source are imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")

	// Now remove a resource from the valid source
	if err := os.Remove(filepath.Join(validSource, "commands", "pdf-command.md")); err != nil {
		t.Fatalf("failed to remove command: %v", err)
	}

	// Second sync: should succeed without errors about the failed source
	// The failed source should NOT trigger removal detection
	syncPruneFlag = true
	defer func() { syncPruneFlag = false }()
	err = runSync(syncCmd, []string{})
	if err == nil {
		t.Fatal("expected completed-with-findings error for partial source failure, got nil")
	}
	if got := getCommandExitCode(err); got != commandExitCodeCompletedWithFindings {
		t.Fatalf("second sync exit code=%d want %d", got, commandExitCodeCompletedWithFindings)
	}

	// Resources from valid source that still exist should be present
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command")

	// pdf-command was removed from source, so sync should have removed it (bmz1.3)
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "pdf-command")
}

// TestRunSync_NoRemovalsWhenSourceUnchanged tests that no removals are
// detected when the source hasn't changed between syncs
func TestRunSync_NoRemovalsWhenSourceUnchanged(t *testing.T) {
	sourceDir := createTestSource(t)

	sources := []*repomanifest.Source{
		{
			Name: "stable-source",
			Path: sourceDir,
			ID:   "src-stable",
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// First sync
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Verify all resources imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")

	// Second sync without any changes — no removals should be detected
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// All resources should still be present
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")
	verifyResourcesInRepo(t, repoPath, resource.Skill, "sync-test-skill", "pdf-processing", "image-processing")
	verifyResourcesInRepo(t, repoPath, resource.Agent, "sync-test-agent", "code-reviewer")
}

// TestSyncSource_ReturnsSourcePath tests that syncSource returns the resolved
// source path for post-sync scanning
func TestSyncSource_ReturnsSourcePath(t *testing.T) {
	sourceDir := createTestSource(t)

	// Set up test repo
	sources := []*repomanifest.Source{
		{
			Name: "path-test-source",
			Path: sourceDir,
		},
	}
	_, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Create manager
	manager, err := repo.NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// syncSource should return the source path
	returnedPath, _, _, err := syncSource(sources[0], manager)
	if err != nil {
		t.Fatalf("syncSource failed: %v", err)
	}

	// Returned path should be the absolute path to the source
	absSourceDir, _ := filepath.Abs(sourceDir)
	if returnedPath != absSourceDir {
		t.Errorf("syncSource returned path %q, expected %q", returnedPath, absSourceDir)
	}
}

func TestSyncSource_RespectsDiscoveryMode(t *testing.T) {
	t.Run("generic mode skips marketplace parsing", func(t *testing.T) {
		sourceDir := createSourceWithInvalidMarketplace(t)

		sources := []*repomanifest.Source{{
			Name:      "generic-source",
			Path:      sourceDir,
			Discovery: repomanifest.DiscoveryModeGeneric,
		}}
		repoPath, cleanup := setupTestManifest(t, sources)
		defer cleanup()

		manager := repo.NewManagerWithPath(repoPath)
		_, _, _, err := syncSource(sources[0], manager)
		if err != nil {
			t.Fatalf("syncSource should ignore broken marketplace when discovery=generic: %v", err)
		}

		verifyResourcesInRepo(t, repoPath, resource.Command, "discovery-command")
	})

	t.Run("marketplace mode parses marketplace and fails on invalid file", func(t *testing.T) {
		sourceDir := createSourceWithInvalidMarketplace(t)

		sources := []*repomanifest.Source{{
			Name:      "marketplace-source",
			Path:      sourceDir,
			Discovery: repomanifest.DiscoveryModeMarketplace,
		}}
		repoPath, cleanup := setupTestManifest(t, sources)
		defer cleanup()

		manager := repo.NewManagerWithPath(repoPath)
		_, _, _, err := syncSource(sources[0], manager)
		if err == nil {
			t.Fatal("expected marketplace discovery parsing error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse marketplace") {
			t.Fatalf("expected marketplace parsing error, got: %v", err)
		}
	})
}

func TestSyncSource_DiscoveryMode_SelectsMarketplaceVsGenericResources(t *testing.T) {
	sourceDir := createSourceWithMarketplaceAndLooseResources(t)

	t.Run("auto mode imports marketplace resources only", func(t *testing.T) {
		sources := []*repomanifest.Source{{
			Name:      "auto-source",
			Path:      sourceDir,
			Discovery: repomanifest.DiscoveryModeAuto,
		}}
		repoPath, cleanup := setupTestManifest(t, sources)
		defer cleanup()

		manager := repo.NewManagerWithPath(repoPath)
		_, _, _, err := syncSource(sources[0], manager)
		if err != nil {
			t.Fatalf("syncSource failed: %v", err)
		}

		verifyResourcesInRepo(t, repoPath, resource.Command, "plugin-command")
		verifyResourcesNotInRepo(t, repoPath, resource.Command, "loose-command")
		verifyResourcesNotInRepo(t, repoPath, resource.Skill, "loose-skill")

		pkgPath := filepath.Join(repoPath, "packages", "market-plugin.package.json")
		if _, statErr := os.Stat(pkgPath); statErr != nil {
			t.Fatalf("expected marketplace package to exist: %v", statErr)
		}
	})

	t.Run("marketplace mode imports marketplace resources only", func(t *testing.T) {
		sources := []*repomanifest.Source{{
			Name:      "market-source",
			Path:      sourceDir,
			Discovery: repomanifest.DiscoveryModeMarketplace,
		}}
		repoPath, cleanup := setupTestManifest(t, sources)
		defer cleanup()

		manager := repo.NewManagerWithPath(repoPath)
		_, _, _, err := syncSource(sources[0], manager)
		if err != nil {
			t.Fatalf("syncSource failed: %v", err)
		}

		verifyResourcesInRepo(t, repoPath, resource.Command, "plugin-command")
		verifyResourcesNotInRepo(t, repoPath, resource.Command, "loose-command")
		verifyResourcesNotInRepo(t, repoPath, resource.Skill, "loose-skill")
	})

	t.Run("generic mode ignores marketplace and imports loose resources", func(t *testing.T) {
		sources := []*repomanifest.Source{{
			Name:      "generic-source",
			Path:      sourceDir,
			Discovery: repomanifest.DiscoveryModeGeneric,
		}}
		repoPath, cleanup := setupTestManifest(t, sources)
		defer cleanup()

		manager := repo.NewManagerWithPath(repoPath)
		_, _, _, err := syncSource(sources[0], manager)
		if err != nil {
			t.Fatalf("syncSource failed: %v", err)
		}

		verifyResourcesInRepo(t, repoPath, resource.Command, "loose-command", "plugin-command")
		verifyResourcesInRepo(t, repoPath, resource.Skill, "loose-skill")
	})
}

func TestSyncSource_DiscoveryMode_ZeroResolvableMarketplaceErrors(t *testing.T) {
	sourceDir := createSourceWithMarketplaceNoResolvablePlugins(t)

	t.Run("marketplace mode errors", func(t *testing.T) {
		sources := []*repomanifest.Source{{
			Name:      "market-empty",
			Path:      sourceDir,
			Discovery: repomanifest.DiscoveryModeMarketplace,
		}}
		repoPath, cleanup := setupTestManifest(t, sources)
		defer cleanup()

		manager := repo.NewManagerWithPath(repoPath)
		_, _, _, err := syncSource(sources[0], manager)
		if err == nil {
			t.Fatal("expected zero-resolvable marketplace error")
		}
		if !strings.Contains(err.Error(), "no plugin resources were resolvable") {
			t.Fatalf("expected no-resolvable error, got: %v", err)
		}
	})

	t.Run("auto mode errors when marketplace is present but empty", func(t *testing.T) {
		sources := []*repomanifest.Source{{
			Name:      "auto-empty",
			Path:      sourceDir,
			Discovery: repomanifest.DiscoveryModeAuto,
		}}
		repoPath, cleanup := setupTestManifest(t, sources)
		defer cleanup()

		manager := repo.NewManagerWithPath(repoPath)
		_, _, _, err := syncSource(sources[0], manager)
		if err == nil {
			t.Fatal("expected zero-resolvable marketplace error")
		}
		if !strings.Contains(err.Error(), "no plugin resources were resolvable") {
			t.Fatalf("expected no-resolvable error, got: %v", err)
		}
	})
}

func TestSyncSource_DirectPluginDirectoryMarketplaceFileInputs(t *testing.T) {
	t.Run("local direct marketplace files", func(t *testing.T) {
		tests := []struct {
			name                 string
			marketplaceDirectory string
		}{
			{name: "claude-plugin manifest", marketplaceDirectory: ".claude-plugin"},
			{name: "opencode-plugin manifest", marketplaceDirectory: ".opencode-plugin"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sourceDir := createSourceWithPluginDirectoryMarketplace(t, tt.marketplaceDirectory)
				sourcePath := filepath.Join(sourceDir, tt.marketplaceDirectory, "marketplace.json")

				sources := []*repomanifest.Source{{
					Name:      "local-file-source",
					Path:      sourcePath,
					Discovery: repomanifest.DiscoveryModeAuto,
				}}

				repoPath, cleanup := setupTestManifest(t, sources)
				defer cleanup()

				manager := repo.NewManagerWithPath(repoPath)
				_, _, _, err := syncSource(sources[0], manager)
				if err != nil {
					t.Fatalf("syncSource failed for %s: %v", sourcePath, err)
				}

				verifyResourcesInRepo(t, repoPath, resource.Command, "plugin-command")
				verifyResourcesNotInRepo(t, repoPath, resource.Command, "loose-command")
			})
		}
	})

	t.Run("remote repo-backed marketplace files", func(t *testing.T) {
		tests := []struct {
			name                 string
			marketplaceDirectory string
			subpath              string
		}{
			{name: "claude-plugin manifest", marketplaceDirectory: ".claude-plugin", subpath: ".claude-plugin/marketplace.json"},
			{name: "opencode-plugin manifest", marketplaceDirectory: ".opencode-plugin", subpath: ".opencode-plugin/marketplace.json"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				remoteOrigin, worktreePath := createRemoteGitSource(t)
				writeAndCommitRemotePluginDirectoryMarketplace(t, worktreePath, tt.marketplaceDirectory)

				remoteURL := "https://example.com/test/remote.git"
				sources := []*repomanifest.Source{{
					Name:      "remote-file-source",
					URL:       remoteURL,
					Ref:       "main",
					Subpath:   tt.subpath,
					Discovery: repomanifest.DiscoveryModeAuto,
				}}

				repoPath, cleanup := setupTestManifest(t, sources)
				defer cleanup()

				cacheRepoPath := filepath.Join(repoPath, ".workspace", workspace.ComputeHash(remoteURL))
				if err := os.MkdirAll(filepath.Dir(cacheRepoPath), 0755); err != nil {
					t.Fatalf("failed to create workspace dir: %v", err)
				}
				runGit(t, repoPath, "clone", "-b", "main", remoteOrigin, cacheRepoPath)
				runGit(t, cacheRepoPath, "checkout", "main")

				manager := repo.NewManagerWithPath(repoPath)
				_, _, _, err := syncSource(sources[0], manager)
				if err != nil {
					t.Fatalf("syncSource failed for remote subpath %q: %v", tt.subpath, err)
				}

				verifyResourcesInRepo(t, repoPath, resource.Command, "plugin-command")
				verifyResourcesNotInRepo(t, repoPath, resource.Command, "loose-command")
			})
		}
	})
}

// TestRunSync_DryRunDoesNotRemoveOrphans tests that --dry-run mode detects
// orphaned resources but does NOT actually remove them from the repo
func TestRunSync_DryRunDoesNotRemoveOrphans(t *testing.T) {
	sourceDir := createTestSource(t)

	sourceID := "src-dryrun-rm"
	sources := []*repomanifest.Source{
		{
			Name: "dryrun-source",
			Path: sourceDir,
			ID:   sourceID,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// First sync: import all resources (non-dry-run)
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Verify all resources imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "sync-test-cmd", "test-command", "pdf-command")

	// Remove a command from the source
	if err := os.Remove(filepath.Join(sourceDir, "commands", "pdf-command.md")); err != nil {
		t.Fatalf("failed to remove command from source: %v", err)
	}

	// Second sync with --dry-run --prune: should NOT remove the orphan
	syncDryRunFlag = true
	syncPruneFlag = true
	defer func() {
		syncDryRunFlag = false
		syncPruneFlag = false
	}()
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("dry-run sync failed: %v", err)
	}

	// pdf-command should still exist as a dangling symlink in the repo
	// (dry-run should not remove it). Use Lstat to check the symlink entry itself.
	danglingCmd := filepath.Join(repoPath, "commands", "pdf-command.md")
	if _, err := os.Lstat(danglingCmd); err != nil {
		t.Errorf("dry-run should NOT have removed pdf-command, but it's gone: %v", err)
	}
}

func TestRunSync_DryRunPruneShowsPlannedCleanup(t *testing.T) {
	sourceDir := createTestSource(t)

	sources := []*repomanifest.Source{{
		Name: "dryrun-preview-source",
		Path: sourceDir,
		ID:   "src-dryrun-preview",
	}}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	if err := os.Remove(filepath.Join(sourceDir, "commands", "pdf-command.md")); err != nil {
		t.Fatalf("failed to remove command from source: %v", err)
	}

	syncDryRunFlag = true
	syncPruneFlag = true
	defer func() {
		syncDryRunFlag = false
		syncPruneFlag = false
	}()

	output := captureOutput(t, func() {
		if err := runSync(syncCmd, []string{}); err != nil {
			t.Fatalf("dry-run prune sync failed: %v", err)
		}
	})

	if !strings.Contains(output.Stdout, "Would prune") {
		t.Fatalf("expected dry-run prune output to include planned prune actions, got:\n%s", output.Stdout)
	}

	if _, err := os.Lstat(filepath.Join(repoPath, "commands", "pdf-command.md")); err != nil {
		t.Fatalf("dry-run prune should not mutate repo, expected command to remain: %v", err)
	}
}

func TestRunSync_PruneRemovesStalePackages(t *testing.T) {
	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}

	keepCmdContent := "---\ndescription: keep cmd\n---\n# keep-cmd\n"
	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "keep-cmd.md"), []byte(keepCmdContent), 0644); err != nil {
		t.Fatalf("failed to write keep command: %v", err)
	}

	sources := []*repomanifest.Source{{
		Name: "package-source",
		Path: sourceDir,
		ID:   "src-package-prune",
	}}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Seed stale source-owned package state in repo (simulates state left behind
	// after source include/subpath/discovery changes).
	stalePkgPath := filepath.Join(repoPath, "packages", "source-bundle.package.json")
	stalePkgContent := `{"name":"source-bundle","description":"stale","resources":["command/missing-cmd"]}`
	if err := os.WriteFile(stalePkgPath, []byte(stalePkgContent), 0644); err != nil {
		t.Fatalf("failed to seed stale package: %v", err)
	}
	if err := resmeta.SavePackageMetadata(&resmeta.PackageMetadata{
		Name:          "source-bundle",
		SourceType:    "local",
		SourceURL:     "file://" + sourceDir,
		SourceName:    sources[0].Name,
		SourceID:      repomanifest.GenerateSourceID(&repomanifest.Source{Path: sourceDir}),
		ResourceCount: 1,
	}, repoPath); err != nil {
		t.Fatalf("failed to seed stale package metadata: %v", err)
	}

	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("sync without prune failed: %v", err)
	}
	verifyPackagesInRepo(t, repoPath, "source-bundle")

	manager := repo.NewManagerWithPath(repoPath)
	verifyBeforePrune, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository before prune failed: %v", err)
	}
	if len(verifyBeforePrune.PackagesWithMissingRefs) == 0 {
		t.Fatalf("expected stale package to trigger repo verify findings before prune")
	}

	syncPruneFlag = true
	defer func() { syncPruneFlag = false }()
	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("sync with prune failed: %v", err)
	}

	verifyPackagesNotInRepo(t, repoPath, "source-bundle")

	verifyAfterPrune, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository after prune failed: %v", err)
	}
	if len(verifyAfterPrune.PackagesWithMissingRefs) != 0 {
		t.Fatalf("expected prune to clear stale package verify findings, got %#v", verifyAfterPrune.PackagesWithMissingRefs)
	}
}

func TestRunSync_PruneRemovesStalePackagesWithLegacySourceNameMetadata(t *testing.T) {
	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}

	keepCmdContent := "---\ndescription: keep cmd\n---\n# keep-cmd\n"
	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "keep-cmd.md"), []byte(keepCmdContent), 0644); err != nil {
		t.Fatalf("failed to write keep command: %v", err)
	}

	sources := []*repomanifest.Source{{
		Name: "package-source",
		Path: sourceDir,
		ID:   "src-package-prune",
	}}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Seed stale source-owned package state with legacy metadata that only sets
	// source_name (no source_id).
	stalePkgPath := filepath.Join(repoPath, "packages", "source-bundle.package.json")
	stalePkgContent := `{"name":"source-bundle","description":"stale","resources":["command/missing-cmd"]}`
	if err := os.WriteFile(stalePkgPath, []byte(stalePkgContent), 0644); err != nil {
		t.Fatalf("failed to seed stale package: %v", err)
	}
	if err := resmeta.SavePackageMetadata(&resmeta.PackageMetadata{
		Name:          "source-bundle",
		SourceType:    "local",
		SourceURL:     "file://" + sourceDir,
		SourceName:    sources[0].Name,
		ResourceCount: 1,
	}, repoPath); err != nil {
		t.Fatalf("failed to seed stale package metadata: %v", err)
	}

	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("sync without prune failed: %v", err)
	}
	verifyPackagesInRepo(t, repoPath, "source-bundle")

	manager := repo.NewManagerWithPath(repoPath)
	verifyBeforePrune, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository before prune failed: %v", err)
	}
	if len(verifyBeforePrune.PackagesWithMissingRefs) == 0 {
		t.Fatalf("expected stale package to trigger repo verify findings before prune")
	}

	syncPruneFlag = true
	defer func() { syncPruneFlag = false }()
	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("sync with prune failed: %v", err)
	}

	verifyPackagesNotInRepo(t, repoPath, "source-bundle")

	verifyAfterPrune, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("verifyRepository after prune failed: %v", err)
	}
	if len(verifyAfterPrune.PackagesWithMissingRefs) != 0 {
		t.Fatalf("expected prune to clear stale package verify findings, got %#v", verifyAfterPrune.PackagesWithMissingRefs)
	}
}

func TestPrepareRemoteSourcePath_RefreshesCachedRepo(t *testing.T) {
	wsMgr := &fakeWorkspaceManager{cachePath: "/tmp/cache"}

	path, err := prepareRemoteSourcePath(wsMgr, "https://example.com/repo", "main")
	if err != nil {
		t.Fatalf("prepareRemoteSourcePath failed: %v", err)
	}

	if path != "/tmp/cache" {
		t.Fatalf("prepareRemoteSourcePath returned %q, want %q", path, "/tmp/cache")
	}

	if wsMgr.getOrCloneN != 1 {
		t.Fatalf("GetOrClone called %d times, want 1", wsMgr.getOrCloneN)
	}

	if wsMgr.updateN != 1 {
		t.Fatalf("Update called %d times, want 1", wsMgr.updateN)
	}

	if wsMgr.gotCloneURL != "https://example.com/repo" {
		t.Fatalf("clone URL = %q, want %q", wsMgr.gotCloneURL, "https://example.com/repo")
	}

	if wsMgr.gotRef != "main" {
		t.Fatalf("ref = %q, want %q", wsMgr.gotRef, "main")
	}
}

func TestPrepareRemoteSourcePath_FailsWhenUpdateFails(t *testing.T) {
	wsMgr := &fakeWorkspaceManager{
		cachePath: "/tmp/cache",
		updateErr: fmt.Errorf("fetch failed"),
	}

	_, err := prepareRemoteSourcePath(wsMgr, "https://example.com/repo", "main")
	if err == nil {
		t.Fatal("expected error when update fails")
	}

	if !strings.Contains(err.Error(), "failed to update cached repository") {
		t.Fatalf("unexpected error: %v", err)
	}

	if wsMgr.updateN != 1 {
		t.Fatalf("Update called %d times, want 1", wsMgr.updateN)
	}
}

func TestParsedRemoteSourceForManifestEntry_UsesManifestRefAndSubpath(t *testing.T) {
	src := &repomanifest.Source{
		URL:     "https://github.com/example/tools",
		Ref:     "main",
		Subpath: ".claude-plugin/marketplace.json",
	}

	parsed, err := parsedRemoteSourceForManifestEntry(src)
	if err != nil {
		t.Fatalf("parsedRemoteSourceForManifestEntry failed: %v", err)
	}

	if parsed.URL != "https://github.com/example/tools" {
		t.Fatalf("parsed URL = %q, want %q", parsed.URL, "https://github.com/example/tools")
	}
	if parsed.Ref != "main" {
		t.Fatalf("parsed ref = %q, want %q", parsed.Ref, "main")
	}
	if parsed.Subpath != ".claude-plugin/marketplace.json" {
		t.Fatalf("parsed subpath = %q, want %q", parsed.Subpath, ".claude-plugin/marketplace.json")
	}
}

func TestParsedRemoteSourceForManifestEntry_StoredFieldsOverrideInlineURLCoordinates(t *testing.T) {
	src := &repomanifest.Source{
		URL:     "https://github.com/example/tools/tree/main/legacy-path",
		Ref:     "release/v2",
		Subpath: "explicit/resources",
	}

	parsed, err := parsedRemoteSourceForManifestEntry(src)
	if err != nil {
		t.Fatalf("parsedRemoteSourceForManifestEntry failed: %v", err)
	}

	if parsed.URL != "https://github.com/example/tools" {
		t.Fatalf("parsed URL = %q, want %q", parsed.URL, "https://github.com/example/tools")
	}
	if parsed.Ref != "release/v2" {
		t.Fatalf("parsed ref = %q, want %q", parsed.Ref, "release/v2")
	}
	if parsed.Subpath != "explicit/resources" {
		t.Fatalf("parsed subpath = %q, want %q", parsed.Subpath, "explicit/resources")
	}
}

func TestParsedRemoteSourceForManifestEntry_LegacyInlineCoordinatesRemainSupportedWhenManifestFieldsEmpty(t *testing.T) {
	src := &repomanifest.Source{
		URL: "https://github.com/example/tools/tree/main/resources",
	}

	parsed, err := parsedRemoteSourceForManifestEntry(src)
	if err != nil {
		t.Fatalf("parsedRemoteSourceForManifestEntry failed: %v", err)
	}

	if parsed.URL != "https://github.com/example/tools" {
		t.Fatalf("parsed URL = %q, want %q", parsed.URL, "https://github.com/example/tools")
	}
	if parsed.Ref != "main" {
		t.Fatalf("parsed ref = %q, want %q", parsed.Ref, "main")
	}
	if parsed.Subpath != "resources" {
		t.Fatalf("parsed subpath = %q, want %q", parsed.Subpath, "resources")
	}
}

func TestParsedRemoteSourceForManifestEntry_RejectsUnsupportedStandaloneManifestURL(t *testing.T) {
	src := &repomanifest.Source{
		URL: "https://example.com/marketplace.json",
	}

	_, err := parsedRemoteSourceForManifestEntry(src)
	if err == nil {
		t.Fatal("expected error for unsupported standalone manifest URL")
	}
	if !strings.Contains(err.Error(), "standalone remote manifest fetching is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSync_RemoteSourceRefreshesStaleCache(t *testing.T) {
	remoteOrigin, worktreePath := createRemoteGitSource(t)
	writeAndCommitRemoteCommand(t, worktreePath, "remote-command", "version one")

	remoteURL := "https://example.com/test/remote.git"

	sources := []*repomanifest.Source{{
		Name: "remote-source",
		URL:  remoteURL,
	}}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	cacheRepoPath := filepath.Join(repoPath, ".workspace", workspace.ComputeHash(remoteURL))
	if err := os.MkdirAll(filepath.Dir(cacheRepoPath), 0755); err != nil {
		t.Fatalf("failed to create workspace dir: %v", err)
	}
	runGit(t, repoPath, "clone", "-b", "main", remoteOrigin, cacheRepoPath)
	runGit(t, cacheRepoPath, "checkout", "main")

	writeAndCommitRemoteCommand(t, worktreePath, "remote-command", "version two")

	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	cmdPath := filepath.Join(repoPath, "commands", "remote-command.md")
	data, err := os.ReadFile(cmdPath)
	if err != nil {
		t.Fatalf("failed to read synced command: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "version two") {
		t.Fatalf("expected synced command to contain refreshed content, got:\n%s", content)
	}
}

func TestRunSync_RemoteSourceTrickyURLStableAcrossSyncAndPrune(t *testing.T) {
	remoteOrigin, worktreePath := createRemoteGitSource(t)
	writeAndCommitRemoteCommand(t, worktreePath, "remote-command", "version one")

	trickyURL := "https://example.com/test/remote.git/"
	canonicalURL := "https://example.com/test/remote"

	sources := []*repomanifest.Source{{
		Name: "remote-source",
		URL:  trickyURL,
	}}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	workspaceManager, err := workspace.NewManager(repoPath)
	if err != nil {
		t.Fatalf("failed to create workspace manager: %v", err)
	}

	cacheRepoPath := filepath.Join(repoPath, ".workspace", workspace.ComputeHash(canonicalURL))
	if err := os.MkdirAll(filepath.Dir(cacheRepoPath), 0755); err != nil {
		t.Fatalf("failed to create workspace dir: %v", err)
	}
	runGit(t, repoPath, "clone", "-b", "main", remoteOrigin, cacheRepoPath)

	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	if workspace.ComputeHash(trickyURL) != workspace.ComputeHash(canonicalURL) {
		t.Fatalf("expected tricky and canonical URL to resolve to same cache hash")
	}

	manager := repo.NewManagerWithPath(repoPath)
	unreferenced, err := findUnreferencedCaches(manager, workspaceManager)
	if err != nil {
		t.Fatalf("findUnreferencedCaches failed: %v", err)
	}
	if len(unreferenced) != 0 {
		t.Fatalf("expected no unreferenced caches, got %#v", unreferenced)
	}

	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load source metadata: %v", err)
	}
	state := metadata.Get("remote-source")
	if state == nil {
		t.Fatalf("expected source metadata for remote-source")
	}
	if state.LastSynced.IsZero() {
		t.Fatalf("expected non-zero last_synced for remote-source")
	}

	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	src, ok := manifest.GetSource("remote-source")
	if !ok {
		t.Fatalf("expected remote-source in manifest")
	}
	if src.ID == "" {
		t.Fatalf("expected source ID to be set for remote-source")
	}
}

func TestPrintSyncOutputTable_CompactIncludesRemovedCounts(t *testing.T) {
	so := &syncOutput{
		Sources: []sourceSyncResult{
			{
				Name:         "anthropics",
				Mode:         "remote",
				RemovedCount: 2,
				Result: &output.BulkOperationResult{
					Added:   []output.ResourceResult{{Name: "new-skill", Type: "skill"}},
					Updated: []output.ResourceResult{{Name: "old-skill", Type: "skill"}},
				},
			},
		},
		Summary:  syncSummary{SourcesTotal: 1, SourcesSynced: 1, ResourcesAdded: 1, ResourcesUpdated: 1, ResourcesRemoved: 2},
		Warnings: []string{"removed resources may have active project installations; run 'aimgr repair' in affected projects if needed"},
	}

	output := captureOutput(t, func() {
		printSyncOutputTable(so, false)
	})

	if !strings.Contains(output.Stdout, "✓ anthropics (remote)") {
		t.Fatalf("expected compact source line, got:\n%s", output.Stdout)
	}
	if !strings.Contains(output.Stdout, "1 added, 1 updated, 2 removed") {
		t.Fatalf("expected removed count in compact output, got:\n%s", output.Stdout)
	}
	if !strings.Contains(output.Stdout, "warnings: 1") {
		t.Fatalf("expected compact warning summary, got:\n%s", output.Stdout)
	}
	if output.Stderr != "" {
		t.Fatalf("expected no stderr output, got:\n%s", output.Stderr)
	}
}

func TestResolveSourceDisplayName_PrefersManifestName(t *testing.T) {
	displayNames := buildSourceDisplayNames([]*repomanifest.Source{{
		Name: "open-coder",
		ID:   "src-c7b86a375511",
	}})

	got := resolveSourceDisplayName("src-c7b86a375511", displayNames)
	if got != "open-coder" {
		t.Fatalf("resolveSourceDisplayName() = %q, want %q", got, "open-coder")
	}
}

func TestResolveSourceDisplayName_FallsBackToSourceKey(t *testing.T) {
	got := resolveSourceDisplayName("src-unknown", map[string]string{})
	if got != "src-unknown" {
		t.Fatalf("resolveSourceDisplayName() = %q, want %q", got, "src-unknown")
	}
}

func TestCollectResourcesBySource_UsesMetadataNameForNestedCommands(t *testing.T) {
	repoPath := t.TempDir()
	mgr := repo.NewManagerWithPath(repoPath)
	if err := mgr.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	sourceDir := createNestedCommandTestSource(t)
	commandPath := filepath.Join(sourceDir, "commands", "opencode-coder", "status.md")
	if err := mgr.AddCommand(commandPath, "file://"+sourceDir, "local"); err != nil {
		t.Fatalf("failed to add nested command: %v", err)
	}

	metaPath := filepath.Join(repoPath, ".metadata", "commands", "opencode-coder-status-metadata.json")
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("expected nested metadata file to exist: %v", err)
	}

	bySource, err := collectResourcesBySource(repoPath)
	if err != nil {
		t.Fatalf("collectResourcesBySource() error = %v", err)
	}

	if len(bySource) != 1 {
		t.Fatalf("expected exactly 1 source entry, got %#v", bySource)
	}

	var resources []resourceInfo
	for _, res := range bySource {
		resources = res
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	if resources[0].Name != "opencode-coder/status" {
		t.Fatalf("resource name = %q, want %q", resources[0].Name, "opencode-coder/status")
	}
}

func TestRunSync_NestedCommandsRemainTrackedAcrossRepeatedSyncs(t *testing.T) {
	sourceDir := createNestedCommandTestSource(t)
	sources := []*repomanifest.Source{{
		Name: "open-coder",
		Path: sourceDir,
	}}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	metaPath := filepath.Join(repoPath, ".metadata", "commands", "opencode-coder-status-metadata.json")
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("expected metadata to remain after repeated syncs: %v", err)
	}

	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load source metadata: %v", err)
	}

	sourceDisplayNames := buildSourceDisplayNames(manifest.Sources)
	preSyncResources, err := collectResourcesBySource(repoPath)
	if err != nil {
		t.Fatalf("failed to collect pre-sync resources: %v", err)
	}

	removed, warnings := detectRemovedForSource(manifest.Sources[0], sourceDir, repoPath, preSyncResources)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(removed) != 0 {
		t.Fatalf("expected no removed resources, got %#v", removed)
	}

	state, ok := metadata.Sources["open-coder"]
	if !ok {
		t.Fatalf("expected source metadata for open-coder")
	}
	if state.LastSynced.IsZero() {
		t.Fatalf("expected open-coder source to have a last_synced timestamp")
	}

	if sourceDisplayNames[manifest.Sources[0].Name] != "open-coder" {
		t.Fatalf("expected display name for source %q", manifest.Sources[0].Name)
	}
}

func TestRemoveOrphanedResources_UsesFriendlySourceNamesInHumanOutput(t *testing.T) {
	repoPath := t.TempDir()
	manager := repo.NewManagerWithPath(repoPath)

	cmdDir := filepath.Join(repoPath, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	cmdPath := filepath.Join(cmdDir, "demo.md")
	if err := os.WriteFile(cmdPath, []byte("---\ndescription: demo\n---\n# demo"), 0644); err != nil {
		t.Fatalf("failed to write command file: %v", err)
	}

	output := captureOutput(t, func() {
		removed, warnings := removeOrphanedResources(
			manager,
			map[string][]resourceInfo{"src-c7b86a375511": {{Name: "demo", Type: resource.Command}}},
			map[string]string{"src-c7b86a375511": "open-coder"},
			syncOutputMode{format: output.Table},
		)
		if len(removed) != 1 {
			t.Fatalf("expected 1 removed resource, got %d", len(removed))
		}
		if len(warnings) != 0 {
			t.Fatalf("expected human-mode warnings to be printed instead of returned, got %v", warnings)
		}
	})

	if !strings.Contains(output.Stdout, "from open-coder") {
		t.Fatalf("expected friendly source name in output, got:\n%s", output.Stdout)
	}
	if strings.Contains(output.Stdout, "src-c7b86a375511") {
		t.Fatalf("expected source id to be hidden when name available, got:\n%s", output.Stdout)
	}
	if output.Stderr != "" {
		t.Fatalf("expected no stderr output, got:\n%s", output.Stderr)
	}
}

func TestRunSync_JSONOutputIsSingleDocument(t *testing.T) {
	source1 := createTestSource(t)
	sources := []*repomanifest.Source{{Name: "test-source-1", Path: source1}}
	_, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	oldFormat := syncFormatFlag
	oldVerbose := syncVerboseFlag
	syncFormatFlag = "json"
	syncVerboseFlag = false
	defer func() {
		syncFormatFlag = oldFormat
		syncVerboseFlag = oldVerbose
	}()

	var runErr error
	output := captureOutput(t, func() {
		runErr = runSync(syncCmd, []string{})
	})
	if runErr != nil {
		t.Fatalf("runSync failed: %v", runErr)
	}
	if output.Stderr != "" {
		t.Fatalf("expected no stderr output in json mode, got:\n%s", output.Stderr)
	}
	if strings.Contains(output.Stdout, "Syncing from") {
		t.Fatalf("expected no human progress output in json mode, got:\n%s", output.Stdout)
	}

	var got syncOutput
	if err := json.Unmarshal([]byte(output.Stdout), &got); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\noutput:\n%s", err, output.Stdout)
	}
	if got.Summary.SourcesTotal != 1 {
		t.Fatalf("expected one source in json summary, got %+v", got.Summary)
	}
	if len(got.Sources) != 1 {
		t.Fatalf("expected one source result, got %d", len(got.Sources))
	}
}

func TestRunSync_TableOutputPrintsDiscoveryIssuesForMalformedResources(t *testing.T) {
	sourceDir := createSourceWithMalformedAgentAndValidCommand(t)
	repoPath, cleanup := setupTestManifest(t, []*repomanifest.Source{{Name: "broken-source", Path: sourceDir}})
	defer cleanup()

	output := captureOutput(t, func() {
		if err := runSync(syncCmd, []string{}); err != nil {
			t.Fatalf("runSync failed: %v", err)
		}
	})

	if !strings.Contains(output.Stdout, "Discovery Issues for source 'broken-source' (1)") {
		t.Fatalf("expected discovery issue header in sync output, got:\n%s", output.Stdout)
	}
	if !strings.Contains(output.Stdout, "agents/broken-agent.md") {
		t.Fatalf("expected malformed agent path in sync output, got:\n%s", output.Stdout)
	}
	if !strings.Contains(output.Stdout, "Some files were skipped during discovery") {
		t.Fatalf("expected discovery tip in sync output, got:\n%s", output.Stdout)
	}
	if output.Stderr != "" {
		t.Fatalf("expected no stderr output, got:\n%s", output.Stderr)
	}

	verifyResourcesInRepo(t, repoPath, resource.Command, "valid-command")
	verifyResourcesNotInRepo(t, repoPath, resource.Agent, "broken-agent")
}

func TestRunSync_JSONOutputIncludesDiscoveryIssues(t *testing.T) {
	sourceDir := createSourceWithMalformedAgentAndValidCommand(t)
	_, cleanup := setupTestManifest(t, []*repomanifest.Source{{Name: "broken-source", Path: sourceDir}})
	defer cleanup()

	oldFormat := syncFormatFlag
	oldVerbose := syncVerboseFlag
	syncFormatFlag = "json"
	syncVerboseFlag = false
	defer func() {
		syncFormatFlag = oldFormat
		syncVerboseFlag = oldVerbose
	}()

	var runErr error
	output := captureOutput(t, func() {
		runErr = runSync(syncCmd, []string{})
	})
	if runErr != nil {
		t.Fatalf("runSync failed: %v", runErr)
	}

	var got syncOutput
	if err := json.Unmarshal([]byte(output.Stdout), &got); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v\noutput:\n%s", err, output.Stdout)
	}
	if len(got.Sources) != 1 {
		t.Fatalf("expected one source result, got %d", len(got.Sources))
	}
	if len(got.Sources[0].DiscoveryIssues) != 1 {
		t.Fatalf("expected one discovery issue, got %+v", got.Sources[0].DiscoveryIssues)
	}
	if got.Sources[0].DiscoveryIssues[0].DisplayPath != "agents/broken-agent.md" {
		t.Fatalf("unexpected discovery issue path: %+v", got.Sources[0].DiscoveryIssues[0])
	}
	if len(got.Warnings) == 0 {
		t.Fatalf("expected top-level warnings in sync JSON output, got %+v", got)
	}
}

func TestRunSync_TableModePrintsImmediateStartupBanner(t *testing.T) {
	source1 := createTestSource(t)
	sources := []*repomanifest.Source{{Name: "test-source-1", Path: source1}}
	_, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	oldFormat := syncFormatFlag
	oldVerbose := syncVerboseFlag
	syncFormatFlag = "table"
	syncVerboseFlag = false
	defer func() {
		syncFormatFlag = oldFormat
		syncVerboseFlag = oldVerbose
	}()

	output := captureOutput(t, func() {
		if err := runSync(syncCmd, []string{}); err != nil {
			t.Fatalf("runSync failed: %v", err)
		}
	})

	if !strings.HasPrefix(output.Stdout, "Syncing from 1 configured source(s)...") {
		t.Fatalf("expected immediate startup banner at beginning of output, got:\n%s", output.Stdout)
	}
	if output.Stderr != "" {
		t.Fatalf("expected no stderr output, got:\n%s", output.Stderr)
	}
}

// TestRunSync_RemovesOrphansWithGitCommit tests that orphan removals are
// committed to git during sync
func TestRunSync_RemovesOrphansWithGitCommit(t *testing.T) {
	sourceDir := createTestSource(t)

	sourceID := "src-git-rm"
	sources := []*repomanifest.Source{
		{
			Name: "git-rm-source",
			Path: sourceDir,
			ID:   sourceID,
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// Initialize as git repo
	manager, err := repo.NewManager()
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// First sync: import all resources
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Remove resources from source
	if err := os.Remove(filepath.Join(sourceDir, "commands", "pdf-command.md")); err != nil {
		t.Fatalf("failed to remove command: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(sourceDir, "skills", "image-processing")); err != nil {
		t.Fatalf("failed to remove skill: %v", err)
	}

	// Second sync: should remove orphans and commit
	syncPruneFlag = true
	defer func() { syncPruneFlag = false }()
	err = runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// Verify resources were removed
	verifyResourcesNotInRepo(t, repoPath, resource.Command, "pdf-command")
	verifyResourcesNotInRepo(t, repoPath, resource.Skill, "image-processing")

	// Verify git status is clean
	gitCmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	output, err := gitCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get git status: %v", err)
	}
	status := string(output)
	if status != "" {
		t.Errorf("expected clean git status after sync with removals, got:\n%s", status)
	}

	// Verify git log contains removal commits (Manager.Remove commits each individually)
	gitCmd = exec.Command("git", "-C", repoPath, "log", "--oneline")
	output, err = gitCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get git log: %v", err)
	}
	logOutput := string(output)
	if !strings.Contains(logOutput, "aimgr: remove command: pdf-command") {
		t.Errorf("expected removal commit for pdf-command in git log, got:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "aimgr: remove skill: image-processing") {
		t.Errorf("expected removal commit for image-processing in git log, got:\n%s", logOutput)
	}
}

// TestRunSync_RemovalOnlyForSuccessfulSources verifies that resources from
// failed sources are NOT removed even if they appear orphaned
func TestRunSync_RemovalOnlyForSuccessfulSources(t *testing.T) {
	// Create two separate source directories
	sourceA := createTestSource(t)
	sourceB := t.TempDir()

	// Source B has a unique command
	cmdDir := filepath.Join(sourceB, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	content := "---\ndescription: Source B only command\n---\n# source-b-cmd"
	if err := os.WriteFile(filepath.Join(cmdDir, "source-b-cmd.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	sources := []*repomanifest.Source{
		{
			Name: "source-a",
			Path: sourceA,
			ID:   "src-a",
		},
		{
			Name: "source-b",
			Path: sourceB,
			ID:   "src-b",
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// First sync: both sources succeed
	err := runSync(syncCmd, []string{})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Verify source B's resource is imported
	verifyResourcesInRepo(t, repoPath, resource.Command, "source-b-cmd")

	// Now make source B's path invalid (simulating a failed source)
	sources[1].Path = "/nonexistent/source-b-path"
	manifest := &repomanifest.Manifest{Version: 1, Sources: sources}
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to save updated manifest: %v", err)
	}

	// Second sync: source A succeeds, source B fails
	// Source B's resources should NOT be removed because source B failed
	syncPruneFlag = true
	defer func() { syncPruneFlag = false }()
	err = runSync(syncCmd, []string{})
	if err == nil {
		t.Fatal("expected completed-with-findings error for partial source failure, got nil")
	}
	if got := getCommandExitCode(err); got != commandExitCodeCompletedWithFindings {
		t.Fatalf("second sync exit code=%d want %d", got, commandExitCodeCompletedWithFindings)
	}

	// Source B's resource should still exist (not removed because source failed)
	// Note: since it was symlinked from the original source B path, the symlink
	// target still exists. Use Lstat to be safe.
	sourceBCmd := filepath.Join(repoPath, "commands", "source-b-cmd.md")
	if _, err := os.Lstat(sourceBCmd); err != nil {
		t.Errorf("source B's resource should NOT have been removed (source B failed to sync): %v", err)
	}
}

// TestRunSync_CrossSourceNameCollision tests that when two sources provide a
// resource with the same name, orphan cleanup does not incorrectly remove the
// resource that was overwritten by the other source (bmz1.7).
//
// Scenario:
//  1. Source A has "shared-cmd" and "cmd-a"
//  2. Source B has "shared-cmd" and "cmd-b"
//  3. After first sync, "shared-cmd" metadata points to Source B (last-write-wins)
//  4. "shared-cmd" is then removed from Source A
//  5. On second sync, orphan detection for Source A sees "shared-cmd" in pre-sync
//     inventory but not in the source. Without bmz1.7, it would remove it.
//  6. With bmz1.7: metadata re-check sees "shared-cmd" now belongs to Source B,
//     so it skips removal. The resource survives.
func TestRunSync_CrossSourceNameCollision(t *testing.T) {
	// Source A: has "shared-cmd" and "cmd-a"
	sourceA := t.TempDir()
	cmdDirA := filepath.Join(sourceA, "commands")
	if err := os.MkdirAll(cmdDirA, 0755); err != nil {
		t.Fatalf("failed to create commands dir for source A: %v", err)
	}
	sharedContent := "---\ndescription: Shared command from A\n---\n# shared-cmd (A)"
	if err := os.WriteFile(filepath.Join(cmdDirA, "shared-cmd.md"), []byte(sharedContent), 0644); err != nil {
		t.Fatalf("failed to create shared-cmd in source A: %v", err)
	}
	cmdAContent := "---\ndescription: Source A only\n---\n# cmd-a"
	if err := os.WriteFile(filepath.Join(cmdDirA, "cmd-a.md"), []byte(cmdAContent), 0644); err != nil {
		t.Fatalf("failed to create cmd-a: %v", err)
	}

	// Source B: has "shared-cmd" and "cmd-b"
	sourceB := t.TempDir()
	cmdDirB := filepath.Join(sourceB, "commands")
	if err := os.MkdirAll(cmdDirB, 0755); err != nil {
		t.Fatalf("failed to create commands dir for source B: %v", err)
	}
	sharedContentB := "---\ndescription: Shared command from B\n---\n# shared-cmd (B)"
	if err := os.WriteFile(filepath.Join(cmdDirB, "shared-cmd.md"), []byte(sharedContentB), 0644); err != nil {
		t.Fatalf("failed to create shared-cmd in source B: %v", err)
	}
	cmdBContent := "---\ndescription: Source B only\n---\n# cmd-b"
	if err := os.WriteFile(filepath.Join(cmdDirB, "cmd-b.md"), []byte(cmdBContent), 0644); err != nil {
		t.Fatalf("failed to create cmd-b: %v", err)
	}

	// Sources: A is synced first, then B overwrites "shared-cmd"
	sources := []*repomanifest.Source{
		{
			Name: "source-a",
			Path: sourceA,
			ID:   "src-collision-a",
		},
		{
			Name: "source-b",
			Path: sourceB,
			ID:   "src-collision-b",
		},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	// First sync should be rejected: same resource name from different canonical sources.
	err := runSync(syncCmd, []string{})
	if err == nil {
		t.Fatal("expected sync collision error, got nil")
	}
	for _, expected := range []string{"command/shared-cmd", "source-a", "source-b", "sync rejected"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected collision error to contain %q, got: %v", expected, err)
		}
	}

	if _, statErr := os.Stat(filepath.Join(repoPath, "commands", "shared-cmd.md")); !os.IsNotExist(statErr) {
		t.Fatalf("rejected sync must not import shared-cmd, stat err: %v", statErr)
	}
}

func TestRunSync_RejectsConflictingResourceNamesAcrossDifferentSources(t *testing.T) {
	sourceA := t.TempDir()
	sourceB := t.TempDir()

	cmdDirA := filepath.Join(sourceA, "commands")
	if err := os.MkdirAll(cmdDirA, 0755); err != nil {
		t.Fatalf("failed to create commands dir for source A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDirA, "shared-cmd.md"), []byte("---\ndescription: shared from A\n---\n# shared-cmd"), 0644); err != nil {
		t.Fatalf("failed to create command in source A: %v", err)
	}

	cmdDirB := filepath.Join(sourceB, "commands")
	if err := os.MkdirAll(cmdDirB, 0755); err != nil {
		t.Fatalf("failed to create commands dir for source B: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDirB, "shared-cmd.md"), []byte("---\ndescription: shared from B\n---\n# shared-cmd"), 0644); err != nil {
		t.Fatalf("failed to create command in source B: %v", err)
	}

	sources := []*repomanifest.Source{
		{Name: "source-a", Path: sourceA},
		{Name: "source-b", Path: sourceB},
	}
	repoPath, cleanup := setupTestManifest(t, sources)
	defer cleanup()

	err := runSync(syncCmd, []string{})
	if err == nil {
		t.Fatal("expected sync collision error, got nil")
	}
	for _, expected := range []string{"sync rejected", "command/shared-cmd", "source-a", "source-b", "include filters"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected error to contain %q, got: %v", expected, err)
		}
	}

	manifest, loadErr := repomanifest.Load(repoPath)
	if loadErr != nil {
		t.Fatalf("failed to load manifest after rejected sync: %v", loadErr)
	}
	if len(manifest.Sources) != 2 {
		t.Fatalf("rejected sync must not alter manifest sources, got %d", len(manifest.Sources))
	}

	if _, statErr := os.Stat(filepath.Join(repoPath, "commands", "shared-cmd.md")); !os.IsNotExist(statErr) {
		t.Fatalf("rejected sync must not import conflicting resource, stat err: %v", statErr)
	}
}

func TestCanonicalSourceID_UsesOriginalRemoteForOverriddenSource(t *testing.T) {
	overridden := &repomanifest.Source{
		Name:                "team-tools",
		Path:                "/tmp/local/tools",
		OverrideOriginalURL: "https://github.com/example/tools.git",
	}

	remote := &repomanifest.Source{
		Name: "team-tools",
		URL:  "https://github.com/example/tools",
	}

	if got, want := canonicalSourceID(overridden), canonicalSourceID(remote); got != want {
		t.Fatalf("canonicalSourceID mismatch for override vs remote: got %q want %q", got, want)
	}
}

func TestDetectRemovedForSource_OverrideKeepsCanonicalSourceKey(t *testing.T) {
	repoPath := t.TempDir()
	sourceDir := t.TempDir()
	cmdDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "shared-cmd.md"), []byte("---\ndescription: shared\n---\n# shared-cmd"), 0644); err != nil {
		t.Fatalf("failed to create command file: %v", err)
	}

	src := &repomanifest.Source{
		Name:                "team-tools",
		Path:                sourceDir,
		OverrideOriginalURL: "https://github.com/example/tools",
	}

	canonicalID := canonicalSourceID(src)
	if canonicalID == "" {
		t.Fatalf("expected canonical source ID")
	}

	preSync := map[string][]resourceInfo{
		canonicalID: {{Name: "shared-cmd", Type: resource.Command}},
	}

	removed, warnings := detectRemovedForSource(src, sourceDir, repoPath, preSync)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings when resource is still present in source, got: %v", warnings)
	}
	if len(removed) != 0 {
		t.Fatalf("expected no removed resources; canonical source key should match override identity, got %#v", removed)
	}
}

func TestDetectRemovedForSource_OverrideSubpathUsesStableCanonicalKey(t *testing.T) {
	repoPath := t.TempDir()
	sourceDir := t.TempDir()
	cmdDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "subpath-cmd.md"), []byte("---\ndescription: subpath\n---\n# subpath-cmd"), 0644); err != nil {
		t.Fatalf("failed to create command file: %v", err)
	}

	src := &repomanifest.Source{
		Name:                    "team-tools",
		Path:                    sourceDir,
		OverrideOriginalURL:     "https://github.com/example/tools",
		OverrideOriginalRef:     "main",
		OverrideOriginalSubpath: "resources",
	}

	canonicalID := canonicalSourceID(src)
	if canonicalID == "" {
		t.Fatalf("expected canonical source ID")
	}

	preSync := map[string][]resourceInfo{
		canonicalID: {{Name: "subpath-cmd", Type: resource.Command}},
	}

	removed, warnings := detectRemovedForSource(src, sourceDir, repoPath, preSync)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings when resource is still present in source, got: %v", warnings)
	}
	if len(removed) != 0 {
		t.Fatalf("expected no removals with stable canonical key, got %#v", removed)
	}
}

func TestRemapLegacyRemoteSourceIDs_MigratesPreSyncInventoryKeyForSubpathSource(t *testing.T) {
	src := &repomanifest.Source{
		Name:    "remote-subpath",
		URL:     "https://github.com/example/tools",
		Subpath: "resources",
		ID:      repomanifest.GenerateSourceID(&repomanifest.Source{URL: "https://github.com/example/tools", Subpath: "resources"}),
	}
	legacyID := repomanifest.GenerateSourceID(&repomanifest.Source{URL: src.URL})
	canonicalID := canonicalSourceID(src)
	if legacyID == canonicalID {
		t.Fatalf("test precondition failed: legacy and canonical IDs must differ for non-empty subpath")
	}

	preSync := map[string][]resourceInfo{
		legacyID: {{Name: "legacy-cmd", Type: resource.Command}},
	}

	manifest := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{src}}
	remapLegacyRemoteSourceIDs(preSync, manifest)

	if _, exists := preSync[legacyID]; exists {
		t.Fatalf("expected legacy inventory key %q to be removed", legacyID)
	}
	if len(preSync[canonicalID]) != 1 || preSync[canonicalID][0].Name != "legacy-cmd" {
		t.Fatalf("expected resources to be remapped to canonical key %q, got %#v", canonicalID, preSync)
	}
}

func TestDetectRemovedForSource_SyncAfterUpgradeUsesRemappedLegacyRemoteID(t *testing.T) {
	repoPath := t.TempDir()
	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create source commands dir: %v", err)
	}

	src := &repomanifest.Source{
		Name:    "remote-subpath",
		URL:     "https://github.com/example/tools",
		Subpath: "resources",
		ID:      repomanifest.GenerateSourceID(&repomanifest.Source{URL: "https://github.com/example/tools", Subpath: "resources"}),
	}
	legacyID := repomanifest.GenerateSourceID(&repomanifest.Source{URL: src.URL})
	canonicalID := canonicalSourceID(src)

	if err := resmeta.Save(&resmeta.ResourceMetadata{
		Name:       "legacy-cmd",
		Type:       resource.Command,
		SourceID:   legacyID,
		SourceName: src.Name,
	}, repoPath, src.Name); err != nil {
		t.Fatalf("failed to seed legacy metadata: %v", err)
	}

	preSync := map[string][]resourceInfo{
		legacyID: {{Name: "legacy-cmd", Type: resource.Command}},
	}
	manifest := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{src}}
	remapLegacyRemoteSourceIDs(preSync, manifest)

	removed, warnings := detectRemovedForSource(src, sourceDir, repoPath, preSync)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(removed) != 1 || removed[0].Name != "legacy-cmd" || removed[0].Type != resource.Command {
		t.Fatalf("expected legacy resource removal after remap under canonical key %q, got %#v", canonicalID, removed)
	}
}

func TestDetectSyncResourceCollisions_RespectsDiscoveryMode(t *testing.T) {
	sourceA := createSourceWithMarketplaceAndLooseResources(t)

	sourceB := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceB, "commands"), 0755); err != nil {
		t.Fatalf("failed to create sourceB commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceB, "commands", "loose-command.md"), []byte("---\ndescription: duplicate loose\n---\n# loose-command\n"), 0644); err != nil {
		t.Fatalf("failed to create sourceB command: %v", err)
	}

	manifest := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{
		{Name: "marketplace-source", Path: sourceA, Discovery: repomanifest.DiscoveryModeMarketplace},
		{Name: "generic-source", Path: sourceB, Discovery: repomanifest.DiscoveryModeGeneric},
	}}

	manager := repo.NewManagerWithPath(t.TempDir())
	if err := detectSyncResourceCollisions(manifest, manager); err != nil {
		t.Fatalf("expected no collision because marketplace mode excludes loose resources, got: %v", err)
	}
}

func TestDetectRemovedForSource_RespectsMarketplaceDiscoveryMode(t *testing.T) {
	sourceDir := createSourceWithMarketplaceAndLooseResources(t)
	repoPath := t.TempDir()

	src := &repomanifest.Source{
		Name:      "hybrid-source",
		Path:      sourceDir,
		Discovery: repomanifest.DiscoveryModeMarketplace,
	}

	preSync := map[string][]resourceInfo{
		canonicalSourceID(src): {
			{Name: "loose-command", Type: resource.Command},
			{Name: "plugin-command", Type: resource.Command},
		},
	}

	if err := resmeta.Save(&resmeta.ResourceMetadata{
		Name:       "loose-command",
		Type:       resource.Command,
		SourceID:   canonicalSourceID(src),
		SourceName: src.Name,
	}, repoPath, src.Name); err != nil {
		t.Fatalf("failed to seed metadata for loose-command: %v", err)
	}

	removed, warnings := detectRemovedForSource(src, sourceDir, repoPath, preSync)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	removedSet := map[string]bool{}
	for _, res := range removed {
		removedSet[string(res.Type)+"/"+res.Name] = true
	}

	if !removedSet["command/loose-command"] {
		t.Fatalf("expected loose-command to be marked removed in marketplace mode; removed=%#v", removed)
	}
	if removedSet["command/plugin-command"] {
		t.Fatalf("did not expect plugin-command to be marked removed; removed=%#v", removed)
	}
}

func TestResourceBelongsToSource_PackageUsesPackageMetadata(t *testing.T) {
	repoPath := t.TempDir()

	if err := resmeta.SavePackageMetadata(&resmeta.PackageMetadata{
		Name:       "team-bundle",
		SourceName: "team-source",
		SourceID:   "src-team",
	}, repoPath); err != nil {
		t.Fatalf("failed to save package metadata: %v", err)
	}

	if !resourceBelongsToSource("team-bundle", resource.PackageType, "src-team", "team-source", repoPath) {
		t.Fatalf("expected package to match by source ID")
	}
	if !resourceBelongsToSource("team-bundle", resource.PackageType, "team-source", "team-source", repoPath) {
		t.Fatalf("expected package to match by source name")
	}
	if resourceBelongsToSource("team-bundle", resource.PackageType, "src-other", "other-source", repoPath) {
		t.Fatalf("did not expect package to match different source")
	}
}

func TestPreSyncResourcesForSource_IncludesLegacySourceNameEntries(t *testing.T) {
	preSyncResources := map[string][]resourceInfo{
		"src-package-prune": {
			{Name: "id-owned", Type: resource.Command},
			{Name: "shared", Type: resource.PackageType},
		},
		"package-source": {
			{Name: "name-owned", Type: resource.PackageType},
			{Name: "shared", Type: resource.PackageType},
		},
	}

	merged := preSyncResourcesForSource(preSyncResources, "src-package-prune", "package-source")

	got := make(map[string]bool, len(merged))
	for _, res := range merged {
		got[string(res.Type)+"/"+res.Name] = true
	}

	for _, expected := range []string{"command/id-owned", "package/name-owned", "package/shared"} {
		if !got[expected] {
			t.Fatalf("expected merged set to contain %q, got %#v", expected, got)
		}
	}
	if len(got) != 3 {
		t.Fatalf("expected deduplicated merged set of size 3, got %d (%#v)", len(got), got)
	}
}
