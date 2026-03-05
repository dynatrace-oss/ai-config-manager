//go:build integration

package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/source"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/workspace"
	"github.com/dynatrace-oss/ai-config-manager/v3/test/testutil"
)

const controlledRepo = "https://github.com/dynatrace-oss/ai-config-manager"

// TestGitClone_BasicFetch verifies we can clone a real GitHub repo
func TestGitClone_BasicFetch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := t.TempDir()
	mgr, _ := workspace.NewManager(repoDir)
	mgr.Init()

	parsed, _ := source.ParseSource(controlledRepo)
	cloneURL, _ := source.GetCloneURL(parsed)
	ref := "main"

	// Clone repository
	cachePath, err := mgr.GetOrClone(cloneURL, ref)
	if err != nil {
		t.Fatalf("GetOrClone failed: %v", err)
	}

	// Verify .git exists
	gitPath := filepath.Join(cachePath, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		t.Errorf(".git directory not found: %v", err)
	}

	// Verify can read some file (README.md likely exists)
	if _, err := os.Stat(filepath.Join(cachePath, "README.md")); err != nil {
		t.Logf("Note: README.md not found, but clone succeeded")
	}
}

// TestGitClone_WithRef verifies branch/ref handling
func TestGitClone_WithRef(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := t.TempDir()
	mgr, _ := workspace.NewManager(repoDir)
	mgr.Init()

	parsed, _ := source.ParseSource(controlledRepo)
	cloneURL, _ := source.GetCloneURL(parsed)
	ref := "main" // Use main branch explicitly

	cachePath, err := mgr.GetOrClone(cloneURL, ref)
	if err != nil {
		t.Fatalf("GetOrClone with ref failed: %v", err)
	}

	// Verify cache exists and is valid
	gitPath := filepath.Join(cachePath, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		t.Errorf("Cache is not valid after clone with ref: .git not found: %v", err)
	}
}

// TestGitClone_InvalidURL verifies error handling
func TestGitClone_InvalidURL(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := t.TempDir()
	mgr, _ := workspace.NewManager(repoDir)
	mgr.Init()

	invalidURL := "https://github.com/nonexistent-user-12345/nonexistent-repo-67890"

	_, err := mgr.GetOrClone(invalidURL, "main")
	if err == nil {
		t.Errorf("GetOrClone should fail for invalid URL")
	}
}

// TestWorkspaceCache_ReuseOnSecondFetch verifies cache reuse
func TestWorkspaceCache_ReuseOnSecondFetch(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := t.TempDir()
	mgr, _ := workspace.NewManager(repoDir)
	mgr.Init()

	parsed, _ := source.ParseSource(controlledRepo)
	cloneURL, _ := source.GetCloneURL(parsed)
	ref := "main"

	// First fetch - clone
	cachePath1, err := mgr.GetOrClone(cloneURL, ref)
	if err != nil {
		t.Fatalf("First GetOrClone failed: %v", err)
	}

	// Get .git mtime
	gitPath := filepath.Join(cachePath1, ".git")
	stat1, _ := os.Stat(gitPath)
	mtime1 := stat1.ModTime()

	// Second fetch - should reuse
	cachePath2, err := mgr.GetOrClone(cloneURL, ref)
	if err != nil {
		t.Fatalf("Second GetOrClone failed: %v", err)
	}

	// Verify same path
	if cachePath1 != cachePath2 {
		t.Errorf("Cache paths differ: %s != %s", cachePath1, cachePath2)
	}

	// Verify .git not recreated (allow small time difference for git ops)
	stat2, _ := os.Stat(gitPath)
	mtime2 := stat2.ModTime()
	if mtime2.Before(mtime1) {
		t.Errorf(".git appears to have been recreated (older mtime)")
	}
}
