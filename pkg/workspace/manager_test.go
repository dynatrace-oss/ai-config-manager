package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repolock"
)

// TestNormalizeURL verifies URL normalization logic
func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic URL unchanged",
			input:    "https://github.com/anthropics/skills",
			expected: "https://github.com/anthropics/skills",
		},
		{
			name:     "uppercase converted to lowercase",
			input:    "https://GitHub.com/Anthropics/Skills",
			expected: "https://github.com/anthropics/skills",
		},
		{
			name:     "trailing slash removed",
			input:    "https://github.com/anthropics/skills/",
			expected: "https://github.com/anthropics/skills",
		},
		{
			name:     ".git suffix removed",
			input:    "https://github.com/anthropics/skills.git",
			expected: "https://github.com/anthropics/skills",
		},
		{
			name:     ".git slash removed",
			input:    "https://github.com/anthropics/skills.git/",
			expected: "https://github.com/anthropics/skills",
		},
		{
			name:     "slash .git removed",
			input:    "https://github.com/anthropics/skills/.git",
			expected: "https://github.com/anthropics/skills",
		},
		{
			name:     "multiple trailing slashes removed",
			input:    "https://github.com/anthropics/skills///",
			expected: "https://github.com/anthropics/skills",
		},
		{
			name:     "multiple transformations",
			input:    "  https://GitHub.com/Anthropics/Skills.git/  ",
			expected: "https://github.com/anthropics/skills",
		},
		{
			name:     "whitespace trimmed",
			input:    "  https://github.com/anthropics/skills  ",
			expected: "https://github.com/anthropics/skills",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeURL(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestComputeHash verifies hash computation consistency
func TestComputeHash(t *testing.T) {
	// Same URL should always produce same hash
	url1 := "https://github.com/anthropics/skills"
	hash1 := computeHash(url1)
	hash2 := computeHash(url1)

	if hash1 != hash2 {
		t.Errorf("computeHash produced inconsistent results: %s != %s", hash1, hash2)
	}

	// Different variations of same URL should produce same hash (after normalization)
	variations := []string{
		"https://github.com/anthropics/skills",
		"https://GitHub.com/Anthropics/Skills",
		"https://github.com/anthropics/skills/",
		"https://github.com/anthropics/skills.git",
		"https://github.com/anthropics/skills.git/",
		"https://github.com/anthropics/skills/.git",
		"https://github.com/anthropics/skills///",
		"  https://github.com/anthropics/skills  ",
	}

	expectedHash := computeHash(variations[0])
	for _, variation := range variations {
		hash := computeHash(variation)
		if hash != expectedHash {
			t.Errorf("computeHash(%q) = %s; want %s", variation, hash, expectedHash)
		}
	}

	// Different URLs should produce different hashes
	url2 := "https://github.com/different/repo"
	hashDifferent := computeHash(url2)
	if hashDifferent == hash1 {
		t.Errorf("computeHash produced same hash for different URLs")
	}

	// Hash should be 64 characters (SHA256 hex encoded)
	if len(hash1) != 64 {
		t.Errorf("computeHash produced hash of length %d; want 64", len(hash1))
	}
}

// TestNewManager verifies Manager creation
func TestNewManager(t *testing.T) {
	repoPath := "/tmp/test-repo"
	mgr, err := NewManager(repoPath)

	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	expectedWorkspace := filepath.Join(repoPath, ".workspace")
	if mgr.workspaceDir != expectedWorkspace {
		t.Errorf("Manager.workspaceDir = %q; want %q", mgr.workspaceDir, expectedWorkspace)
	}
}

// TestInit verifies workspace directory initialization
func TestInit(t *testing.T) {
	// Use temporary directory for testing
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	mgr, err := NewManager(repoPath)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Workspace should not exist yet
	if _, err := os.Stat(mgr.workspaceDir); err == nil {
		t.Errorf("workspace directory should not exist before Init()")
	}

	// Initialize
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Workspace should now exist
	info, err := os.Stat(mgr.workspaceDir)
	if err != nil {
		t.Errorf("workspace directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("workspace path exists but is not a directory")
	}

	// Should be idempotent
	if err := mgr.Init(); err != nil {
		t.Errorf("Init should be idempotent but failed on second call: %v", err)
	}
}

// TestGetCachePath verifies cache path computation
func TestGetCachePath(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	url := "https://github.com/anthropics/skills"
	cachePath := mgr.getCachePath(url)

	// Should be under workspace directory
	expectedPrefix := filepath.Join(tmpDir, ".workspace")
	if !strings.HasPrefix(cachePath, expectedPrefix) {
		t.Errorf("getCachePath returned path outside workspace: %s", cachePath)
	}

	// Should end with hash
	hash := computeHash(url)
	expectedPath := filepath.Join(expectedPrefix, hash)
	if cachePath != expectedPath {
		t.Errorf("getCachePath = %s; want %s", cachePath, expectedPath)
	}
}

// TestIsValidCache verifies cache validation logic
func TestIsValidCache(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	tests := []struct {
		name     string
		setup    func(string)
		expected bool
	}{
		{
			name:     "non-existent directory",
			setup:    func(path string) {},
			expected: false,
		},
		{
			name: "directory without .git",
			setup: func(path string) {
				os.MkdirAll(path, 0755)
			},
			expected: false,
		},
		{
			name: "directory with .git file (not directory)",
			setup: func(path string) {
				os.MkdirAll(path, 0755)
				os.WriteFile(filepath.Join(path, ".git"), []byte("gitdir: ../repo/.git"), 0644)
			},
			expected: false,
		},
		{
			name: "valid cache with .git directory",
			setup: func(path string) {
				os.MkdirAll(path, 0755)
				os.MkdirAll(filepath.Join(path, ".git"), 0755)
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cachePath := filepath.Join(tmpDir, tt.name)
			tt.setup(cachePath)

			result := mgr.isValidCache(cachePath)
			if result != tt.expected {
				t.Errorf("isValidCache = %v; want %v", result, tt.expected)
			}
		})
	}
}

// TestMetadataOperations verifies metadata loading and saving
func TestMetadataOperations(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Load metadata (should create empty if not exists)
	metadata, err := mgr.loadMetadata()
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}

	if metadata.Version != "1.0" {
		t.Errorf("metadata.Version = %q; want %q", metadata.Version, "1.0")
	}

	if len(metadata.Caches) != 0 {
		t.Errorf("new metadata should have empty Caches map")
	}

	// Add an entry
	url := "https://github.com/anthropics/skills"
	hash := computeHash(url)
	metadata.Caches[hash] = CacheEntry{
		URL: normalizeURL(url),
		Ref: "main",
	}

	// Save metadata
	if err := mgr.saveMetadata(metadata); err != nil {
		t.Fatalf("saveMetadata failed: %v", err)
	}

	// Load again and verify
	loaded, err := mgr.loadMetadata()
	if err != nil {
		t.Fatalf("loadMetadata after save failed: %v", err)
	}

	if len(loaded.Caches) != 1 {
		t.Errorf("loaded metadata has %d caches; want 1", len(loaded.Caches))
	}

	entry, exists := loaded.Caches[hash]
	if !exists {
		t.Errorf("cache entry not found after reload")
	}

	if entry.URL != normalizeURL(url) {
		t.Errorf("entry.URL = %q; want %q", entry.URL, normalizeURL(url))
	}

	if entry.Ref != "main" {
		t.Errorf("entry.Ref = %q; want %q", entry.Ref, "main")
	}
}

// TestUpdateMetadataEntry verifies metadata entry updates
func TestUpdateMetadataEntry(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	url := "https://github.com/anthropics/skills"
	ref := "main"

	// Add new entry
	if err := mgr.updateMetadataEntryForHash(url, ref, "clone", computeHash(url)); err != nil {
		t.Fatalf("updateMetadataEntry failed: %v", err)
	}

	// Verify entry was added
	metadata, _ := mgr.loadMetadata()
	hash := computeHash(url)
	entry, exists := metadata.Caches[hash]

	if !exists {
		t.Fatalf("cache entry not created")
	}

	if entry.URL != normalizeURL(url) {
		t.Errorf("entry.URL = %q; want %q", entry.URL, normalizeURL(url))
	}

	if entry.Ref != ref {
		t.Errorf("entry.Ref = %q; want %q", entry.Ref, ref)
	}

	if entry.LastAccessed.IsZero() {
		t.Errorf("entry.LastAccessed not set")
	}

	if entry.LastUpdated.IsZero() {
		t.Errorf("entry.LastUpdated not set for clone operation")
	}

	firstUpdated := entry.LastUpdated

	// Update with access only (no update)
	if err := mgr.updateMetadataEntryForHash(url, ref, "access", computeHash(url)); err != nil {
		t.Fatalf("updateMetadataEntry (access) failed: %v", err)
	}

	metadata, _ = mgr.loadMetadata()
	entry = metadata.Caches[hash]

	// LastUpdated should not change for access-only
	if entry.LastUpdated != firstUpdated {
		t.Errorf("LastUpdated changed on access-only operation")
	}
}

// Integration tests moved to manager_integration_test.go

// TestGetOrClone_EmptyInputs tests error handling for empty inputs
func TestGetOrClone_EmptyInputs(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	tests := []struct {
		name        string
		url         string
		ref         string
		wantErr     bool
		checkErrMsg string // Optional: check that error is about validation, not clone failure
	}{
		{
			name:        "empty URL",
			url:         "",
			ref:         "main",
			wantErr:     true,
			checkErrMsg: "url cannot be empty",
		},
		{
			name:        "both empty",
			url:         "",
			ref:         "",
			wantErr:     true,
			checkErrMsg: "url cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mgr.GetOrClone(tt.url, tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetOrClone() error = %v; wantErr %v", err, tt.wantErr)
			}
			// If we want to check specific error message (for validation errors)
			if tt.checkErrMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.checkErrMsg) {
					t.Errorf("GetOrClone() error = %v; want error containing %q", err, tt.checkErrMsg)
				}
			}
		})
	}
}

// TestUpdate_EmptyInputs tests error handling for empty inputs
func TestUpdate_EmptyInputs(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	tests := []struct {
		name    string
		url     string
		ref     string
		wantErr bool
	}{
		{
			name:    "empty URL",
			url:     "",
			ref:     "main",
			wantErr: true,
		},
		{
			name:    "empty ref allowed (updates current branch)",
			url:     "https://github.com/test/repo",
			ref:     "",
			wantErr: true, // Still errors because cache doesn't exist
		},
		{
			name:    "both empty",
			url:     "",
			ref:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.Update(tt.url, tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v; wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestUpdate_CacheNotExists tests error when cache doesn't exist
func TestUpdate_CacheNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Try to update a repo that was never cloned
	err := mgr.Update("https://github.com/test/repo", "main")
	if err == nil {
		t.Errorf("Update should fail when cache doesn't exist")
	}

	// Error should mention using GetOrClone first
	if !strings.Contains(err.Error(), "GetOrClone") {
		t.Errorf("Error should suggest using GetOrClone first, got: %v", err)
	}
}

// TestListCached_Empty verifies ListCached with no caches
func TestListCached_Empty(t *testing.T) {
	tempDir := t.TempDir()
	mgr, err := NewManager(tempDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Initialize workspace
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// List caches (should be empty)
	urls, err := mgr.ListCached()
	if err != nil {
		t.Fatalf("ListCached failed: %v", err)
	}

	if len(urls) != 0 {
		t.Errorf("expected 0 cached URLs, got %d", len(urls))
	}
}

// TestRemove_NonExistent verifies removing a non-existent cache returns error
func TestRemove_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	mgr, err := NewManager(tempDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Try to remove non-existent cache
	err = mgr.Remove("https://github.com/nonexistent/repo")
	if err == nil {
		t.Error("Remove should return error for non-existent cache")
	}
}

// TestPrune verifies pruning unreferenced caches
func TestPrune(t *testing.T) {
	// This is a unit test that doesn't need real Git repos
	tempDir := t.TempDir()
	mgr, err := NewManager(tempDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create fake cache directories manually
	workspaceDir := filepath.Join(tempDir, ".workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	testURL1 := "https://github.com/test/repo1"
	testURL2 := "https://github.com/test/repo2"

	hash1 := computeHash(testURL1)
	hash2 := computeHash(testURL2)

	cache1 := filepath.Join(workspaceDir, hash1)
	cache2 := filepath.Join(workspaceDir, hash2)

	// Create fake .git directories
	if err := os.MkdirAll(filepath.Join(cache1, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cache2, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create metadata pointing to both repos
	metadata := &CacheMetadata{
		Version: "1.0",
		Caches: map[string]CacheEntry{
			hash1: {
				URL: normalizeURL(testURL1),
				Ref: "main",
			},
			hash2: {
				URL: normalizeURL(testURL2),
				Ref: "main",
			},
		},
	}
	if err := mgr.saveMetadata(metadata); err != nil {
		t.Fatal(err)
	}

	// Verify both caches exist
	urls, err := mgr.ListCached()
	if err != nil {
		t.Fatalf("ListCached failed: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected 2 cached URLs, got %d", len(urls))
	}

	// Prune with only URL1 referenced
	removed, err := mgr.Prune([]string{testURL1})
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	// Verify URL2 was removed
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed URL, got %d", len(removed))
	}

	normalizedURL2 := normalizeURL(testURL2)
	if removed[0] != normalizedURL2 {
		t.Errorf("expected removed URL %s, got %s", normalizedURL2, removed[0])
	}

	// Verify only URL1 remains cached
	urls, err = mgr.ListCached()
	if err != nil {
		t.Fatalf("ListCached failed: %v", err)
	}
	if len(urls) != 1 {
		t.Errorf("expected 1 cached URL after prune, got %d", len(urls))
	}
	if urls[0] != normalizeURL(testURL1) {
		t.Errorf("expected remaining URL %s, got %s", normalizeURL(testURL1), urls[0])
	}
}

func TestGetOrClone_TimesOutWhenCacheLockHeldByAnotherProcess(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	mgr.lockAcquireTimeout = 120 * time.Millisecond
	url := "https://github.com/test/locked-repo"
	lockPath := mgr.locks.CacheLockPath(computeHash(url))

	cmd := startWorkspaceLockHelper(t, lockPath)
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	_, err = mgr.GetOrClone(url, "main")
	if err == nil {
		t.Fatalf("GetOrClone expected lock acquisition error")
	}

	var timeoutErr *repolock.AcquireTimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("expected AcquireTimeoutError, got: %v", err)
	}
}

func TestUpdate_TimesOutWhenCacheLockHeldByAnotherProcess(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	mgr.lockAcquireTimeout = 120 * time.Millisecond
	url := "https://github.com/test/locked-update"
	cachePath := mgr.getCachePath(url)
	if err := os.MkdirAll(filepath.Join(cachePath, ".git"), 0755); err != nil {
		t.Fatalf("failed to create fake cache: %v", err)
	}

	lockPath := mgr.locks.CacheLockPath(computeHash(url))
	cmd := startWorkspaceLockHelper(t, lockPath)
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	err = mgr.Update(url, "main")
	if err == nil {
		t.Fatalf("Update expected lock acquisition error")
	}

	var timeoutErr *repolock.AcquireTimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("expected AcquireTimeoutError, got: %v", err)
	}
}

func TestPrune_RemovesUnlockedCacheWhileLockedCacheContended(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	mgr.lockAcquireTimeout = 120 * time.Millisecond

	lockedURL := "https://github.com/test/prune-locked"
	openURL := "https://github.com/test/prune-open"

	lockedCache := mgr.getCachePath(lockedURL)
	openCache := mgr.getCachePath(openURL)
	if err := os.MkdirAll(filepath.Join(lockedCache, ".git"), 0755); err != nil {
		t.Fatalf("failed to create locked fake cache: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(openCache, ".git"), 0755); err != nil {
		t.Fatalf("failed to create open fake cache: %v", err)
	}

	metadata := &CacheMetadata{
		Version: "1.0",
		Caches: map[string]CacheEntry{
			computeHash(lockedURL): {
				URL: normalizeURL(lockedURL),
				Ref: "main",
			},
			computeHash(openURL): {
				URL: normalizeURL(openURL),
				Ref: "main",
			},
		},
	}
	if err := mgr.saveMetadata(metadata); err != nil {
		t.Fatalf("saveMetadata failed: %v", err)
	}

	lockPath := mgr.locks.CacheLockPath(computeHash(lockedURL))
	cmd := startWorkspaceLockHelper(t, lockPath)
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	removed, err := mgr.Prune(nil)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	if len(removed) != 1 || removed[0] != normalizeURL(openURL) {
		t.Fatalf("expected only %s to be pruned, got %v", normalizeURL(openURL), removed)
	}

	if _, err := os.Stat(openCache); !os.IsNotExist(err) {
		t.Fatalf("expected open cache to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(lockedCache); err != nil {
		t.Fatalf("expected locked cache to remain, stat err=%v", err)
	}
}

func TestGetOrClone_CompatibilityUsesLegacyCacheHashWhenPresent(t *testing.T) {
	remoteURL := createLocalGitRemoteForWorkspaceTest(t)
	trickyURL := remoteURL + ".git/"

	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	legacyPath := filepath.Join(tmpDir, ".workspace", legacyComputeHash(trickyURL))
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0755); err != nil {
		t.Fatalf("failed to create workspace parent: %v", err)
	}

	cmd := exec.Command("git", "clone", "-b", "main", remoteURL, legacyPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to seed legacy cache: %v\n%s", err, string(output))
	}

	cachePath, err := mgr.GetOrClone(trickyURL, "main")
	if err != nil {
		t.Fatalf("GetOrClone failed: %v", err)
	}

	if cachePath != legacyPath {
		t.Fatalf("expected legacy cache path %q, got %q", legacyPath, cachePath)
	}

	modernPath := filepath.Join(tmpDir, ".workspace", computeHash(trickyURL))
	if legacyPath == modernPath {
		t.Fatalf("test requires legacy and modern cache paths to differ")
	}
	if _, err := os.Stat(modernPath); err == nil {
		t.Fatalf("expected modern cache path to not be created when legacy cache exists")
	}
}

func TestRemove_CompatibilityRemovesLegacyCacheAndMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	trickyURL := "https://github.com/test/legacy.git/"
	legacyHash := legacyComputeHash(trickyURL)
	legacyPath := filepath.Join(tmpDir, ".workspace", legacyHash)
	if err := os.MkdirAll(filepath.Join(legacyPath, ".git"), 0755); err != nil {
		t.Fatalf("failed to create legacy cache dir: %v", err)
	}

	metadata := &CacheMetadata{
		Version: "1.0",
		Caches: map[string]CacheEntry{
			legacyHash: {
				URL: trickyURL,
				Ref: "main",
			},
		},
	}
	if err := mgr.saveMetadata(metadata); err != nil {
		t.Fatalf("saveMetadata failed: %v", err)
	}

	if err := mgr.Remove(trickyURL); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy cache path to be removed, stat err=%v", err)
	}

	loaded, err := mgr.loadMetadata()
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}
	if _, exists := loaded.Caches[legacyHash]; exists {
		t.Fatalf("expected legacy metadata entry to be removed")
	}
}

func TestLoadMetadata_CompatibilityPreservesLegacyNormalizedURL(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	legacyURL := "https://github.com/test/legacy.git"
	legacyHash := computeHash(legacyURL)

	legacyMetadata := `{
  "version": "1.0",
  "caches": {
    "` + legacyHash + `": {
      "url": "` + legacyURL + `",
      "last_accessed": "2026-01-01T00:00:00Z",
      "last_updated": "2026-01-01T00:00:00Z",
      "ref": "main"
    }
  }
}`

	metadataPath := filepath.Join(tmpDir, ".workspace", ".cache-metadata.json")
	if err := os.WriteFile(metadataPath, []byte(legacyMetadata), 0644); err != nil {
		t.Fatalf("failed to write legacy metadata: %v", err)
	}

	metadata, err := mgr.loadMetadata()
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}

	entry, ok := metadata.Caches[legacyHash]
	if !ok {
		t.Fatalf("expected legacy cache entry to remain addressable by legacy hash")
	}
	if entry.URL != legacyURL {
		t.Fatalf("expected legacy URL to remain unchanged, got %q", entry.URL)
	}
}

func TestConcurrentMetadataUpdates_DoNotLoseEntries(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	const count = 8
	urls := make([]string, 0, count)
	for i := 0; i < count; i++ {
		url := fmt.Sprintf("https://github.com/test/metadata-%d", i)
		urls = append(urls, url)
	}

	cmds := make([]*exec.Cmd, 0, len(urls))
	for _, u := range urls {
		// #nosec G702 -- os.Args[0] is the current test binary path.
		cmd := exec.Command(os.Args[0], "-test.run=TestHelperWorkspaceMetadataUpdate")
		cmd.Env = append(
			os.Environ(),
			"AIMGR_TEST_WORKSPACE_HELPER_MODE=metadata-update",
			"AIMGR_TEST_WORKSPACE_REPO_PATH="+tmpDir,
			"AIMGR_TEST_WORKSPACE_METADATA_URL="+u,
		)
		if err := cmd.Start(); err != nil {
			t.Fatalf("failed to start metadata helper for %s: %v", u, err)
		}
		cmds = append(cmds, cmd)
	}

	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("metadata helper failed: %v", err)
		}
	}

	metadata, err := mgr.loadMetadata()
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}

	if len(metadata.Caches) != count {
		t.Fatalf("expected %d metadata entries, got %d", count, len(metadata.Caches))
	}

	gotURLs := make([]string, 0, len(metadata.Caches))
	for _, entry := range metadata.Caches {
		gotURLs = append(gotURLs, entry.URL)
	}
	sort.Strings(gotURLs)

	wantURLs := make([]string, 0, len(urls))
	for _, u := range urls {
		wantURLs = append(wantURLs, normalizeURL(u))
	}
	sort.Strings(wantURLs)

	for i := range wantURLs {
		if gotURLs[i] != wantURLs[i] {
			t.Fatalf("metadata URL mismatch at %d: got %s want %s", i, gotURLs[i], wantURLs[i])
		}
	}
}

func TestUpdateMetadataEntry_TimesOutWhenMetadataLockHeldByAnotherProcess(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	mgr.lockAcquireTimeout = 120 * time.Millisecond
	lockPath := mgr.locks.WorkspaceMetadataLockPath()

	cmd := startWorkspaceLockHelper(t, lockPath)
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	err = mgr.updateMetadataEntry("https://github.com/test/metadata-locked", "clone")
	if err == nil {
		t.Fatalf("updateMetadataEntry expected lock acquisition error")
	}

	var timeoutErr *repolock.AcquireTimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("expected AcquireTimeoutError, got: %v", err)
	}
}

func TestGetOrClone_BlocksUntilConcurrentCloneReleasesCacheLock(t *testing.T) {
	repoPath := t.TempDir()
	mgr, err := NewManager(repoPath)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	url := createLocalGitRemoteForWorkspaceTest(t)
	signalDir := t.TempDir()

	// #nosec G702 -- os.Args[0] is the current test binary path.
	first := exec.Command(os.Args[0], "-test.run=TestHelperWorkspaceConcurrentOperation")
	first.Env = append(os.Environ(),
		"AIMGR_TEST_WORKSPACE_HELPER_MODE=workspace-op",
		"AIMGR_TEST_WORKSPACE_REPO_PATH="+repoPath,
		"AIMGR_TEST_WORKSPACE_OPERATION=get-or-clone",
		"AIMGR_TEST_WORKSPACE_URL="+url,
		"AIMGR_TEST_WORKSPACE_REF=main",
		"AIMGR_TEST_WORKSPACE_HOLD_OP=clone",
		"AIMGR_TEST_WORKSPACE_HOLD_SIGNAL_DIR="+signalDir,
		"AIMGR_TEST_WORKSPACE_HOLD_RELEASE_PATH="+filepath.Join(signalDir, "clone.release"),
		"AIMGR_TEST_WORKSPACE_SIGNAL_DIR="+signalDir,
	)
	if err := first.Start(); err != nil {
		t.Fatalf("failed to start first helper: %v", err)
	}
	firstDone := make(chan error, 1)
	go func() { firstDone <- first.Wait() }()

	waitForWorkspaceMarker(t, filepath.Join(signalDir, "clone.ready"), 5*time.Second)

	// #nosec G702 -- os.Args[0] is the current test binary path.
	second := exec.Command(os.Args[0], "-test.run=TestHelperWorkspaceConcurrentOperation")
	second.Env = append(os.Environ(),
		"AIMGR_TEST_WORKSPACE_HELPER_MODE=workspace-op",
		"AIMGR_TEST_WORKSPACE_REPO_PATH="+repoPath,
		"AIMGR_TEST_WORKSPACE_OPERATION=get-or-clone",
		"AIMGR_TEST_WORKSPACE_URL="+url,
		"AIMGR_TEST_WORKSPACE_REF=main",
	)
	if err := second.Start(); err != nil {
		t.Fatalf("failed to start second helper: %v", err)
	}
	secondDone := make(chan error, 1)
	go func() { secondDone <- second.Wait() }()

	select {
	case err := <-secondDone:
		t.Fatalf("second helper exited before lock release, expected blocking: %v", err)
	default:
	}

	if err := os.WriteFile(filepath.Join(signalDir, "clone.release"), []byte("release"), 0644); err != nil {
		t.Fatalf("failed to release first helper: %v", err)
	}

	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first helper failed: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("timeout waiting for first helper completion")
	}

	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("second helper failed: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("timeout waiting for second helper completion")
	}

	metadata, err := mgr.loadMetadata()
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}
	if len(metadata.Caches) != 1 {
		t.Fatalf("expected exactly one cache metadata entry, got %d", len(metadata.Caches))
	}

	cachePath := mgr.getCachePath(url)
	if !mgr.isValidCache(cachePath) {
		t.Fatalf("expected valid cache after concurrent clone operations: %s", cachePath)
	}
}

func waitForWorkspaceMarker(t *testing.T, markerPath string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(markerPath); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for marker: %s", markerPath)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func createLocalGitRemoteForWorkspaceTest(t *testing.T) string {
	t.Helper()

	seedRepo := filepath.Join(t.TempDir(), "seed")
	if err := os.MkdirAll(seedRepo, 0755); err != nil {
		t.Fatalf("failed to create seed repo: %v", err)
	}

	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
		}
	}

	runGit(seedRepo, "init")
	if err := os.WriteFile(filepath.Join(seedRepo, "README.md"), []byte("workspace lock test\n"), 0644); err != nil {
		t.Fatalf("failed to write seed file: %v", err)
	}
	runGit(seedRepo, "add", "README.md")
	runGit(seedRepo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")
	runGit(seedRepo, "branch", "-M", "main")

	baredir := filepath.Join(t.TempDir(), "remote.git")
	cmd := exec.Command("git", "clone", "--bare", seedRepo, baredir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create bare remote: %v\n%s", err, string(output))
	}

	return baredir
}

func TestHelperWorkspaceMetadataUpdate(t *testing.T) {
	if os.Getenv("AIMGR_TEST_WORKSPACE_HELPER_MODE") != "metadata-update" {
		return
	}

	repoPath := os.Getenv("AIMGR_TEST_WORKSPACE_REPO_PATH")
	url := os.Getenv("AIMGR_TEST_WORKSPACE_METADATA_URL")
	if repoPath == "" || url == "" {
		os.Exit(2)
	}

	mgr, err := NewManager(repoPath)
	if err != nil {
		os.Exit(3)
	}
	if err := mgr.Init(); err != nil {
		os.Exit(4)
	}
	if err := mgr.updateMetadataEntry(url, "clone"); err != nil {
		os.Exit(5)
	}

	os.Exit(0)
}

func TestHelperWorkspaceConcurrentOperation(t *testing.T) {
	if os.Getenv("AIMGR_TEST_WORKSPACE_HELPER_MODE") != "workspace-op" {
		return
	}

	repoPath := os.Getenv("AIMGR_TEST_WORKSPACE_REPO_PATH")
	op := os.Getenv("AIMGR_TEST_WORKSPACE_OPERATION")
	url := os.Getenv("AIMGR_TEST_WORKSPACE_URL")
	ref := os.Getenv("AIMGR_TEST_WORKSPACE_REF")
	if repoPath == "" || op == "" || url == "" {
		os.Exit(2)
	}

	if holdOp := os.Getenv("AIMGR_TEST_WORKSPACE_HOLD_OP"); holdOp != "" {
		_ = os.Setenv("AIMGR_TEST_WORKSPACE_HOLD_OP", holdOp)
	}
	if signalDir := os.Getenv("AIMGR_TEST_WORKSPACE_HOLD_SIGNAL_DIR"); signalDir != "" {
		_ = os.Setenv("AIMGR_TEST_WORKSPACE_SIGNAL_DIR", signalDir)
	}

	mgr, err := NewManager(repoPath)
	if err != nil {
		os.Exit(3)
	}
	if err := mgr.Init(); err != nil {
		os.Exit(4)
	}

	switch op {
	case "get-or-clone":
		if _, err := mgr.GetOrClone(url, ref); err != nil {
			os.Exit(5)
		}
	default:
		os.Exit(6)
	}

	os.Exit(0)
}

func TestHelperWorkspaceAcquireLockAndWait(t *testing.T) {
	if os.Getenv("AIMGR_TEST_WORKSPACE_HELPER_MODE") != "acquire-wait" {
		return
	}

	path := os.Getenv("AIMGR_TEST_WORKSPACE_HELPER_LOCK_PATH")
	if path == "" {
		os.Exit(2)
	}
	readyPath := os.Getenv("AIMGR_TEST_WORKSPACE_HELPER_READY_PATH")
	if readyPath == "" {
		os.Exit(4)
	}

	lock, err := repolock.Acquire(context.Background(), path, time.Second)
	if err != nil {
		os.Exit(3)
	}
	// #nosec G703 -- readyPath is a test-only marker path controlled by this test process.
	if err := os.WriteFile(readyPath, []byte("ready"), 0644); err != nil {
		os.Exit(5)
	}

	defer func() {
		_ = lock.Unlock()
	}()
	for {
		time.Sleep(time.Second)
	}
}

func startWorkspaceLockHelper(t *testing.T, lockPath string) *exec.Cmd {
	t.Helper()

	readyPath := filepath.Join(t.TempDir(), "ready")
	// #nosec G702 -- os.Args[0] is the current test binary path.
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperWorkspaceAcquireLockAndWait")
	cmd.Env = append(
		os.Environ(),
		"AIMGR_TEST_WORKSPACE_HELPER_LOCK_PATH="+lockPath,
		"AIMGR_TEST_WORKSPACE_HELPER_READY_PATH="+readyPath,
		"AIMGR_TEST_WORKSPACE_HELPER_MODE=acquire-wait",
	)

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			return cmd
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("helper did not signal readiness")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
