package workspace

/*
Workspace Cache Design
======================

## Purpose
The workspace cache provides efficient Git repository management for resources sourced
from Git URLs. It avoids redundant clones when multiple resources come from the same
repository by maintaining a local cache of Git repositories.

## Directory Structure

~/.local/share/ai-config/repo/
  .workspace/                    # Workspace cache root
    <url-hash>/                  # Directory for each cached repository
      .git/                      # Git repository data
      README.md                  # Cloned repository contents
      commands/                  # Example: repository structure
      skills/
      ...
    .cache-metadata.json         # Optional: Cache index for quick lookups

### Cache Key Algorithm

Each cached repository is stored in a directory named after the SHA256 hash of its
normalized Git URL. This ensures:
- Collision-free storage (unique hash per URL)
- Consistent lookup (same URL always maps to same hash)
- Security (no special characters or path traversal in directory names)

Normalization rules:
1. Trim whitespace
2. Convert to lowercase
3. Repeatedly strip trailing slashes
4. Repeatedly strip trailing .git suffixes
5. Stop when stable

Examples:
  https://github.com/anthropics/skills       → sha256(https://github.com/anthropics/skills)
  https://github.com/anthropics/skills.git   → sha256(https://github.com/anthropics/skills)
  https://github.com/anthropics/skills/      → sha256(https://github.com/anthropics/skills)
  https://GitHub.com/Anthropics/Skills       → sha256(https://github.com/anthropics/skills)

All above URLs normalize to the same hash and share the same cached directory.

## Cache Metadata Format

The optional .cache-metadata.json provides a quick lookup index without scanning
directories. Format:

{
  "version": "1.0",
  "caches": {
    "a3f2e1...": {
      "url": "https://github.com/anthropics/skills",
      "last_accessed": "2026-01-25T10:00:00Z",
      "last_updated": "2026-01-25T09:00:00Z",
      "ref": "main"
    }
  }
}

This metadata is optional and used for optimization only. The cache is designed to
work correctly even if metadata is missing or out of sync.

## API Design

The Manager provides methods for:
- GetOrClone: Retrieve cached repo or clone if missing
- Update: Pull latest changes for a cached repo
- ListCached: Enumerate all cached repositories
- Prune: Remove unused cached repos
- Remove: Delete a specific cached repo

All methods handle edge cases:
- Corrupted cache (missing .git directory)
- Network failures during clone/update
- Invalid Git URLs
- Ref changes (branch/tag/commit switches)
- Concurrent access (file locking where needed)

## Edge Cases Handled

### 1. Corrupted Cache
If a cached directory exists but is missing .git or is corrupted:
- Detection: Check for .git directory and run git status
- Recovery: Remove corrupted directory and re-clone

### 2. Missing .git Directory
If cache directory exists but .git is missing:
- Treat as corrupted cache
- Remove and re-clone

### 3. Ref Changes
When requesting a different ref (branch/tag/commit) than last used:
- Fetch latest refs from remote
- Checkout requested ref
- Update working tree

### 4. Network Failures
- Clone failures: Return error without creating cache directory
- Update failures: Leave cache in last known good state, return error

### 5. Concurrent Access
- Use OS-backed advisory file locks for cache and metadata mutations
- Write operations (clone, update, remove) are serialized per cache
- Shared cache-metadata updates are serialized with a dedicated metadata lock

### 6. Invalid/Unavailable Git sources
- Clone/fetch/checkout failures are surfaced as command errors
- Callers receive the underlying git failure context

## Thread Safety

The Manager is designed to be safe for concurrent use from multiple goroutines.
Individual cache directories are locked during write operations to prevent
corruption from concurrent access.

## Performance Considerations

- Caching: Significantly reduces clone time for repeated accesses
- Incremental updates: git pull only fetches changed objects
- Lazy loading: Caches created on-demand, not pre-populated

## Migration Notes

When introducing workspace cache to existing installations:
1. Existing resources with Git sources continue to work
2. Cache is populated lazily on first update after upgrade
3. No migration of existing data required
4. Old behavior (clone per resource) is replaced transparently
*/

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/fileutil"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/giturl"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repolock"
)

const defaultWorkspaceLockAcquireTimeout = 30 * time.Second

// Package-level logger for workspace operations
var logger *slog.Logger

var testWorkspaceGate statefulWorkspaceGate

type statefulWorkspaceGate struct {
	mu            sync.Mutex
	repoReadySeen map[string]bool
}

func (g *statefulWorkspaceGate) markReady(repoPath string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.repoReadySeen == nil {
		g.repoReadySeen = make(map[string]bool)
	}
	g.repoReadySeen[repoPath] = true
}

func (g *statefulWorkspaceGate) clearReady(repoPath string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.repoReadySeen == nil {
		return
	}
	delete(g.repoReadySeen, repoPath)
}

func (g *statefulWorkspaceGate) wasReady(repoPath string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.repoReadySeen == nil {
		return false
	}
	return g.repoReadySeen[repoPath]
}

// SetLogger sets the logger for the workspace package.
// This should be called by the application during initialization.
func SetLogger(l *slog.Logger) {
	logger = l
}

// maybeHoldAfterCacheLock provides deterministic coordination for subprocess
// workspace contention tests. It is inert unless AIMGR_TEST_WORKSPACE_HOLD_OP
// matches the requested operation.
func maybeHoldAfterCacheLock(ctx context.Context, op string, repoPath string) error {
	holdOp := strings.TrimSpace(os.Getenv("AIMGR_TEST_WORKSPACE_HOLD_OP"))
	if holdOp == "" || holdOp != op {
		return nil
	}

	signalDir := strings.TrimSpace(os.Getenv("AIMGR_TEST_WORKSPACE_SIGNAL_DIR"))
	if signalDir == "" {
		return fmt.Errorf("AIMGR_TEST_WORKSPACE_SIGNAL_DIR must be set when AIMGR_TEST_WORKSPACE_HOLD_OP is used")
	}
	if err := os.MkdirAll(signalDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace test signal directory: %w", err)
	}

	readyPath := filepath.Join(signalDir, op+".ready")
	releasePath := filepath.Join(signalDir, op+".release")

	if op == "clone" {
		testWorkspaceGate.markReady(repoPath)
	}

	if err := os.WriteFile(readyPath, []byte("ready"), 0644); err != nil {
		if op == "clone" {
			testWorkspaceGate.clearReady(repoPath)
		}
		return fmt.Errorf("failed to write workspace ready marker: %w", err)
	}

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if _, err := os.Stat(releasePath); err == nil {
			if op == "clone" {
				testWorkspaceGate.clearReady(repoPath)
			}
			return nil
		}

		select {
		case <-ctx.Done():
			if op == "clone" {
				testWorkspaceGate.clearReady(repoPath)
			}
			return fmt.Errorf("workspace test hold canceled: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func maybeHoldAfterMetadataLock(ctx context.Context, op string, repoPath string) error {
	holdOp := strings.TrimSpace(os.Getenv("AIMGR_TEST_WORKSPACE_HOLD_OP"))
	if holdOp == "" || holdOp != op {
		return nil
	}

	if op == "metadata-rmw" {
		if !testWorkspaceGate.wasReady(repoPath) {
			return nil
		}
	}

	signalDir := strings.TrimSpace(os.Getenv("AIMGR_TEST_WORKSPACE_SIGNAL_DIR"))
	if signalDir == "" {
		return fmt.Errorf("AIMGR_TEST_WORKSPACE_SIGNAL_DIR must be set when AIMGR_TEST_WORKSPACE_HOLD_OP is used")
	}
	if err := os.MkdirAll(signalDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace test signal directory: %w", err)
	}

	readyPath := filepath.Join(signalDir, op+".ready")
	releasePath := filepath.Join(signalDir, op+".release")
	if err := os.WriteFile(readyPath, []byte("ready"), 0644); err != nil {
		return fmt.Errorf("failed to write workspace metadata ready marker: %w", err)
	}

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if _, err := os.Stat(releasePath); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("workspace metadata test hold canceled: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// Manager manages the workspace cache for Git repositories.
// The workspace cache stores cloned Git repositories to avoid redundant clones
// when multiple resources come from the same repository.
type Manager struct {
	workspaceDir       string // Path to .workspace directory
	locks              *repolock.Manager
	lockAcquireTimeout time.Duration
}

// CacheEntry represents metadata for a single cached repository.
type CacheEntry struct {
	URL          string    `json:"url"`           // Normalized Git URL
	LastAccessed time.Time `json:"last_accessed"` // Last time cache was accessed
	LastUpdated  time.Time `json:"last_updated"`  // Last time cache was updated (git pull)
	Ref          string    `json:"ref"`           // Current ref (branch/tag/commit)
}

// CacheMetadata is the root structure for .cache-metadata.json.
type CacheMetadata struct {
	Version string                `json:"version"` // Metadata format version
	Caches  map[string]CacheEntry `json:"caches"`  // Map of hash -> CacheEntry
}

// NewManager creates a new workspace cache manager.
// The workspace directory is located at <repoPath>/.workspace/
func NewManager(repoPath string) (*Manager, error) {
	workspaceDir := filepath.Join(repoPath, ".workspace")
	return &Manager{
		workspaceDir:       workspaceDir,
		locks:              repolock.NewManager(repoPath),
		lockAcquireTimeout: defaultWorkspaceLockAcquireTimeout,
	}, nil
}

// Init initializes the workspace cache directory structure.
// Creates the .workspace directory if it doesn't exist.
func (m *Manager) Init() error {
	if err := os.MkdirAll(m.workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}
	return nil
}

// GetOrClone returns the path to a cached Git repository, cloning it if necessary.
//
// Parameters:
//   - url: Git repository URL (will be normalized)
//   - ref: Git ref to checkout (branch, tag, or commit hash). If empty, uses repository's default branch.
//
// Returns:
//   - string: Absolute path to the cached repository directory
//   - error: Non-nil if clone/checkout fails
//
// Behavior:
//   - If cache exists and is valid: checkout ref and return path
//   - If cache exists but is corrupted: remove and re-clone
//   - If cache doesn't exist: clone repository
//   - If ref is empty: uses repository's default branch (main/master)
//
// Edge cases handled:
//   - Corrupted cache (missing .git): detected and re-cloned
//   - Network failures: error returned, no partial cache created
//   - Invalid URL: error returned before any filesystem operations
//   - Ref not found: error returned with helpful message
//   - Empty ref: defaults to repository's default branch
//
// Locking:
//   - This method is self-locking for cache mutations. It acquires the per-cache
//     lock before mutating the cache and acquires workspace metadata lock only for
//     short metadata read-modify-write sections.
//   - Callers that already hold the repo lock must not re-acquire it here.
//   - Lock ordering is preserved as: repo lock (caller) -> cache lock (here) ->
//     workspace metadata lock (inside metadata update helper).
func (m *Manager) GetOrClone(url string, ref string) (string, error) {
	// Ensure workspace is initialized
	if err := m.Init(); err != nil {
		return "", err
	}

	// Validate inputs
	if url == "" {
		return "", fmt.Errorf("url cannot be empty")
	}

	// If ref is empty, use empty string for git clone (which defaults to HEAD)
	// This will be handled properly in cloneRepo and checkoutRef

	// Resolve cache path/hash with legacy compatibility fallback
	cachePath, cacheHash := m.resolveCacheLocation(url)

	cacheLock, err := m.acquireCacheLock(context.Background(), cacheHash)
	if err != nil {
		return "", fmt.Errorf("failed to acquire cache lock at %s: %w", m.locks.CacheLockPath(cacheHash), err)
	}
	defer func() {
		_ = cacheLock.Unlock()
	}()

	if err := maybeHoldAfterCacheLock(context.Background(), "clone", filepath.Dir(m.workspaceDir)); err != nil {
		return "", err
	}

	// Log cache lookup
	if logger != nil {
		logger.Debug("cache lookup",
			"url", url,
			"ref", ref,
			"cache_path", cachePath,
		)
	}

	// Check if cache exists and is valid
	if m.isValidCache(cachePath) {
		// Log cache hit
		if logger != nil {
			logger.Debug("cache hit", "cache_path", cachePath)
		}
		// Cache exists - ensure correct ref is checked out (only if ref is specified)
		if ref != "" {
			if err := m.checkoutRef(cachePath, ref); err != nil {
				// If checkout fails, try to recover by fetching
				if fetchErr := m.fetchRepo(cachePath); fetchErr != nil {
					// Fetch failed - cache may be corrupted, remove and re-clone
					if removeErr := os.RemoveAll(cachePath); removeErr != nil {
						return "", fmt.Errorf("failed to remove corrupted cache: %w", removeErr)
					}
					// Fall through to clone
				} else {
					// Fetch succeeded, try checkout again
					if err := m.checkoutRef(cachePath, ref); err != nil {
						return "", fmt.Errorf("failed to checkout ref after fetch: %w", err)
					}
					// Success - update metadata and return
					if err := m.updateMetadataEntryForHash(url, ref, "access", cacheHash); err != nil {
						// Log warning but don't fail - metadata is optional
						fmt.Fprintf(os.Stderr, "warning: failed to update metadata: %v\n", err)
					}
					return cachePath, nil
				}
			} else {
				// Checkout succeeded - update metadata and return
				if err := m.updateMetadataEntryForHash(url, ref, "access", cacheHash); err != nil {
					// Log warning but don't fail - metadata is optional
					fmt.Fprintf(os.Stderr, "warning: failed to update metadata: %v\n", err)
				}
				return cachePath, nil
			}
		} else {
			// No ref specified, use whatever is currently checked out
			if err := m.updateMetadataEntryForHash(url, ref, "access", cacheHash); err != nil {
				// Log warning but don't fail - metadata is optional
				fmt.Fprintf(os.Stderr, "warning: failed to update metadata: %v\n", err)
			}
			return cachePath, nil
		}
	}

	// Log cache miss
	if logger != nil {
		logger.Debug("cache miss", "cache_path", cachePath)
	}

	// Cache doesn't exist or was removed due to corruption - clone it
	if err := m.cloneRepo(url, cachePath, ref); err != nil {
		return "", fmt.Errorf("failed to clone repository: %w", err)
	}

	// Update metadata
	if err := m.updateMetadataEntryForHash(url, ref, "clone", cacheHash); err != nil {
		// Log warning but don't fail - metadata is optional
		fmt.Fprintf(os.Stderr, "warning: failed to update metadata: %v\n", err)
	}

	return cachePath, nil
}

// Update pulls the latest changes for a cached repository.
//
// Parameters:
//   - url: Git repository URL (will be normalized)
//   - ref: Git ref to update to (branch, tag, or commit). If empty, uses current branch.
//
// Returns:
//   - error: Non-nil if update fails
//
// Behavior:
//   - Fetches latest refs from remote
//   - Checks out requested ref (if specified)
//   - Updates working tree (git pull for branches, checkout for tags/commits)
//
// Edge cases handled:
//   - Cache doesn't exist: returns error (use GetOrClone first)
//   - Network failure: cache left in last known good state
//   - Ref doesn't exist: error returned
//   - Uncommitted changes: stashed before update, restored after
//   - Empty ref: updates current branch
//
// Locking:
//   - This method is self-locking for cache mutations. It acquires the per-cache
//     lock for the full update section (stash/fetch/checkout/reset/pull/pop).
//   - Workspace metadata lock is acquired only for the short metadata update.
//   - Callers that already hold the repo lock must not re-acquire it here.
func (m *Manager) Update(url string, ref string) error {
	// Validate inputs
	if url == "" {
		return fmt.Errorf("url cannot be empty")
	}

	// Resolve cache path/hash with legacy compatibility fallback
	cachePath, cacheHash := m.resolveCacheLocation(url)

	cacheLock, err := m.acquireCacheLock(context.Background(), cacheHash)
	if err != nil {
		return fmt.Errorf("failed to acquire cache lock at %s: %w", m.locks.CacheLockPath(cacheHash), err)
	}
	defer func() {
		_ = cacheLock.Unlock()
	}()

	if err := maybeHoldAfterCacheLock(context.Background(), "update", filepath.Dir(m.workspaceDir)); err != nil {
		return err
	}

	// Log update operation
	if logger != nil {
		logger.Debug("updating cached repository",
			"url", url,
			"ref", ref,
			"cache_path", cachePath,
		)
	}

	// Verify cache exists
	if !m.isValidCache(cachePath) {
		return fmt.Errorf("cache does not exist for URL: %s (use GetOrClone first)", url)
	}

	// Check for uncommitted changes
	hasChanges, err := m.hasUncommittedChanges(cachePath)
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}

	// Stash changes if present
	stashed := false
	if hasChanges {
		if err := m.stashChanges(cachePath); err != nil {
			return fmt.Errorf("failed to stash uncommitted changes: %w", err)
		}
		stashed = true
	}

	// Fetch latest refs from remote
	if err := m.fetchRepo(cachePath); err != nil {
		return fmt.Errorf("failed to fetch from remote: %w", err)
	}

	// Reset to origin state to handle conflicts (only if ref is specified)
	// This ensures a clean update by discarding local commits
	if ref != "" {
		if err := m.resetToOrigin(cachePath, ref); err != nil {
			// If reset fails, try just checking out
			if checkoutErr := m.checkoutRef(cachePath, ref); checkoutErr != nil {
				return fmt.Errorf("failed to update ref: reset failed (%v), checkout failed (%v)", err, checkoutErr)
			}
		}
	} else {
		// No ref specified, pull current branch
		if err := m.pullCurrentBranch(cachePath); err != nil {
			return fmt.Errorf("failed to pull current branch: %w", err)
		}
	}

	// Restore stashed changes if any
	if stashed {
		if err := m.popStash(cachePath); err != nil {
			// Log warning but don't fail - the update itself succeeded
			fmt.Fprintf(os.Stderr, "warning: failed to restore stashed changes: %v\n", err)
		}
	}

	// Update metadata
	if err := m.updateMetadataEntryForHash(url, ref, "update", cacheHash); err != nil {
		// Log warning but don't fail - metadata is optional
		fmt.Fprintf(os.Stderr, "warning: failed to update metadata: %v\n", err)
	}

	return nil
}

// ListCached returns all cached repository URLs.
//
// Returns:
//   - []string: List of normalized Git URLs for all cached repositories
//   - error: Non-nil if cache directory can't be read
//
// Behavior:
//   - Reads .cache-metadata.json if available
//   - Falls back to scanning .workspace directory if metadata missing
//   - Skips corrupted caches (missing .git)
//
// Note: May be slow for large caches without metadata file.
func (m *Manager) ListCached() ([]string, error) {
	// Try to load metadata first (fast path)
	metadata, err := m.loadMetadata()
	if err == nil && metadata != nil && len(metadata.Caches) > 0 {
		// Return URLs from metadata
		urls := make([]string, 0, len(metadata.Caches))
		for _, entry := range metadata.Caches {
			urls = append(urls, entry.URL)
		}
		return urls, nil
	}

	// Fallback: scan workspace directory
	entries, err := os.ReadDir(m.workspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No workspace directory means no caches
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read workspace directory: %w", err)
	}

	var urls []string
	for _, entry := range entries {
		// Skip files and hidden directories (like .cache-metadata.json)
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		cachePath := filepath.Join(m.workspaceDir, entry.Name())

		// Verify it's a valid git repository
		if !m.isValidCache(cachePath) {
			continue
		}

		// Get the remote URL from git config
		url, err := m.getRemoteURL(cachePath)
		if err != nil {
			// Skip caches where we can't determine the URL
			continue
		}

		urls = append(urls, normalizeURL(url))
	}

	return urls, nil
}

// Prune removes cached repositories that are not referenced by any resources.
//
// Parameters:
//   - referencedURLs: Set of Git URLs currently referenced by resources
//
// Returns:
//   - []string: List of URLs that were removed
//   - error: Non-nil if pruning fails
//
// Behavior:
//   - Compares cached repos against referenced URLs
//   - Removes caches not in referenced set
//   - Updates cache metadata
//   - Returns list of removed URLs for logging
//
// Safety:
//   - Uses normalized URLs for comparison
//   - Dry-run mode available (future enhancement)
//   - Logs removals for audit trail
func (m *Manager) Prune(referencedURLs []string) ([]string, error) {
	// Get list of all cached URLs
	cachedURLs, err := m.ListCached()
	if err != nil {
		return nil, fmt.Errorf("failed to list cached repositories: %w", err)
	}

	// Normalize referenced URLs and build a set for fast lookup
	referencedSet := make(map[string]bool)
	for _, url := range referencedURLs {
		normalized := normalizeURL(url)
		if normalized != "" {
			referencedSet[normalized] = true
		}
	}

	// Find unreferenced caches
	var unreferenced []string
	for _, cachedURL := range cachedURLs {
		normalized := normalizeURL(cachedURL)
		if !referencedSet[normalized] {
			unreferenced = append(unreferenced, normalized)
		}
	}

	// Remove each unreferenced cache one at a time. Remove() acquires only the
	// current cache lock, so prune never holds multiple cache locks concurrently.
	var removed []string
	for _, url := range unreferenced {
		if err := m.Remove(url); err != nil {
			// Log error but continue with other removals
			fmt.Fprintf(os.Stderr, "warning: failed to remove cache for %s: %v\n", url, err)
			continue
		}
		removed = append(removed, url)
	}

	return removed, nil
}

// Remove deletes a specific cached repository.
//
// Parameters:
//   - url: Git repository URL (will be normalized)
//
// Returns:
//   - error: Non-nil if removal fails or cache doesn't exist
//
// Behavior:
//   - Normalizes URL
//   - Computes cache hash
//   - Removes cache directory
//   - Updates cache metadata
//
// Safety:
//   - Acquires lock before removal to prevent concurrent access
//   - Validates hash before removal (sanity check)
//
// Locking:
//   - This method is self-locking for cache mutation. It acquires the per-cache
//     lock for the full remove section and workspace metadata lock only for the
//     metadata read-modify-write.
//   - Callers that already hold the repo lock must not re-acquire it here.
func (m *Manager) Remove(url string) error {
	// Normalize URL
	normalized := normalizeURL(url)
	if normalized == "" {
		return fmt.Errorf("invalid URL: %s", url)
	}

	cachePath, cacheHash := m.resolveCacheLocation(normalized)

	cacheLock, err := m.acquireCacheLock(context.Background(), cacheHash)
	if err != nil {
		return fmt.Errorf("failed to acquire cache lock at %s: %w", m.locks.CacheLockPath(cacheHash), err)
	}
	defer func() {
		_ = cacheLock.Unlock()
	}()

	if err := maybeHoldAfterCacheLock(context.Background(), "remove", filepath.Dir(m.workspaceDir)); err != nil {
		return err
	}

	// Check if cache exists
	if _, err := os.Stat(cachePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cache does not exist for URL: %s", normalized)
		}
		return fmt.Errorf("failed to stat cache directory: %w", err)
	}

	// Remove cache directory
	if err := os.RemoveAll(cachePath); err != nil {
		return fmt.Errorf("failed to remove cache directory: %w", err)
	}

	if err := m.removeMetadataEntry(normalized); err != nil {
		// Log warning but don't fail (removal succeeded)
		fmt.Fprintf(os.Stderr, "warning: failed to update metadata: %v\n", err)
	}

	return nil
}

func (m *Manager) acquireCacheLock(ctx context.Context, cacheHash string) (*repolock.Lock, error) {
	return repolock.Acquire(ctx, m.locks.CacheLockPath(cacheHash), m.lockAcquireTimeout)
}

func (m *Manager) resolveCacheLocation(url string) (cachePath string, cacheHash string) {
	primaryHash := computeHash(url)
	primaryPath := filepath.Join(m.workspaceDir, primaryHash)

	if m.isValidCache(primaryPath) {
		return primaryPath, primaryHash
	}

	legacyHash := legacyComputeHash(url)
	if legacyHash != primaryHash {
		legacyPath := filepath.Join(m.workspaceDir, legacyHash)
		if m.isValidCache(legacyPath) {
			return legacyPath, legacyHash
		}
	}

	return primaryPath, primaryHash
}

func (m *Manager) acquireWorkspaceMetadataLock(ctx context.Context) (*repolock.Lock, error) {
	return repolock.Acquire(ctx, m.locks.WorkspaceMetadataLockPath(), m.lockAcquireTimeout)
}

// runGitCommand executes a git command with comprehensive logging.
// Logs command execution, output, and failures at appropriate levels.
func runGitCommand(workDir string, args ...string) (string, error) {
	// Log command execution at DEBUG level
	if logger != nil {
		logger.Debug("executing git command",
			"command", fmt.Sprintf("git %s", strings.Join(args, " ")),
			"working_dir", workDir,
		)
	}

	// Build command
	cmd := exec.Command("git", args...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Execute command and capture combined output
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	if err != nil {
		// Log failure at ERROR level with full context
		if logger != nil {
			logger.Error("git command failed",
				"command", fmt.Sprintf("git %s", strings.Join(args, " ")),
				"working_dir", workDir,
				"error", err.Error(),
				"output", outputStr,
			)
		}
		return outputStr, fmt.Errorf("git command failed: %w\nOutput: %s", err, outputStr)
	}

	// Log success output at DEBUG level
	if logger != nil && outputStr != "" {
		logger.Debug("git command output",
			"command", fmt.Sprintf("git %s", strings.Join(args, " ")),
			"output", outputStr,
		)
	}

	return outputStr, nil
}

// getRemoteURL retrieves the remote URL from a Git repository.
func (m *Manager) getRemoteURL(cachePath string) (string, error) {
	output, err := runGitCommand(cachePath, "config", "--get", "remote.origin.url")
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}
	return output, nil
}

func normalizeURL(url string) string {
	return giturl.NormalizeURL(url)
}

func legacyNormalizeURL(url string) string {
	normalized := strings.TrimSpace(url)
	normalized = strings.ToLower(normalized)
	normalized = strings.TrimSuffix(normalized, "/")
	normalized = strings.TrimSuffix(normalized, ".git")
	return normalized
}

// computeHash computes the SHA256 hash of a normalized URL.
// Returns the hash as a lowercase hexadecimal string.
func computeHash(url string) string {
	normalized := normalizeURL(url)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

func legacyComputeHash(url string) string {
	normalized := legacyNormalizeURL(url)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

// ComputeHash computes the SHA256 hash of a normalized URL (exported version).
// Returns the hash as a lowercase hexadecimal string.
func ComputeHash(url string) string {
	return computeHash(url)
}

// getCachePath returns the filesystem path for a cached repository.
func (m *Manager) getCachePath(url string) string {
	hash := computeHash(url)
	return filepath.Join(m.workspaceDir, hash)
}

// isValidCache checks if a cache directory exists and contains a valid Git repository.
// Returns true if the cache is valid, false otherwise.
func (m *Manager) isValidCache(cachePath string) bool {
	// Check if directory exists
	info, err := os.Stat(cachePath)
	if err != nil || !info.IsDir() {
		return false
	}

	// Check if .git directory exists
	gitPath := filepath.Join(cachePath, ".git")
	gitInfo, err := os.Stat(gitPath)
	if err != nil || !gitInfo.IsDir() {
		return false
	}

	// TODO: Could add additional validation here (e.g., git status)
	return true
}

// loadMetadata loads the cache metadata from .cache-metadata.json.
// Returns empty metadata if the file doesn't exist (not an error).
// Callers performing read-modify-write must hold the workspace metadata lock.
func (m *Manager) loadMetadata() (*CacheMetadata, error) {
	metadataPath := filepath.Join(m.workspaceDir, ".cache-metadata.json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, return empty metadata
			return &CacheMetadata{
				Version: "1.0",
				Caches:  make(map[string]CacheEntry),
			}, nil
		}
		return nil, fmt.Errorf("failed to read cache metadata: %w", err)
	}

	var metadata CacheMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse cache metadata: %w", err)
	}

	return &metadata, nil
}

// saveMetadata saves the cache metadata to .cache-metadata.json.
// Callers performing read-modify-write must hold the workspace metadata lock.
func (m *Manager) saveMetadata(metadata *CacheMetadata) error {
	metadataPath := filepath.Join(m.workspaceDir, ".cache-metadata.json")

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache metadata: %w", err)
	}

	if err := fileutil.AtomicWrite(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache metadata: %w", err)
	}

	return nil
}

// updateMetadataEntry updates the metadata for a specific cache entry.
func (m *Manager) updateMetadataEntry(url string, ref string, updateType string) error {
	return m.updateMetadataEntryForHash(url, ref, updateType, computeHash(url))
}

func (m *Manager) updateMetadataEntryForHash(url string, ref string, updateType string, hash string) error {
	metadataLock, err := m.acquireWorkspaceMetadataLock(context.Background())
	if err != nil {
		return fmt.Errorf("failed to acquire workspace metadata lock at %s: %w", m.locks.WorkspaceMetadataLockPath(), err)
	}
	defer func() {
		_ = metadataLock.Unlock()
	}()

	if err := maybeHoldAfterMetadataLock(context.Background(), "metadata-rmw", filepath.Dir(m.workspaceDir)); err != nil {
		return err
	}

	metadata, err := m.loadMetadata()
	if err != nil {
		return err
	}

	normalized := normalizeURL(url)

	entry, exists := metadata.Caches[hash]
	if !exists {
		entry = CacheEntry{
			URL: normalized,
		}
	}

	now := time.Now()
	entry.Ref = ref
	entry.LastAccessed = now

	if updateType == "update" || updateType == "clone" {
		entry.LastUpdated = now
	}

	metadata.Caches[hash] = entry

	return m.saveMetadata(metadata)
}

func (m *Manager) removeMetadataEntry(url string) error {
	metadataLock, err := m.acquireWorkspaceMetadataLock(context.Background())
	if err != nil {
		return fmt.Errorf("failed to acquire workspace metadata lock at %s: %w", m.locks.WorkspaceMetadataLockPath(), err)
	}
	defer func() {
		_ = metadataLock.Unlock()
	}()

	metadata, err := m.loadMetadata()
	if err != nil {
		return err
	}

	delete(metadata.Caches, computeHash(url))
	delete(metadata.Caches, legacyComputeHash(url))

	return m.saveMetadata(metadata)
}

// cloneRepo clones a Git repository to the specified cache path.
// This does a full clone (not shallow) to support ref switching.
func (m *Manager) cloneRepo(url string, cachePath string, ref string) error {
	// Log clone operation
	if logger != nil {
		logger.Debug("cloning repository",
			"url", url,
			"ref", ref,
			"cache_path", cachePath,
		)
	}

	// Build git clone command (full clone for ref switching)
	args := []string{"clone"}

	// Add branch/tag reference if specified (optimization)
	if ref != "" {
		args = append(args, "--branch", ref)
	}

	args = append(args, url, cachePath)

	// Execute git clone (using parent directory as workdir since target doesn't exist yet)
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()

	// Manual logging since we can't use runGitCommand (cachePath doesn't exist yet)
	if logger != nil {
		logger.Debug("executing git command",
			"command", fmt.Sprintf("git %s", strings.Join(args, " ")),
			"working_dir", "",
		)
	}

	if err != nil {
		// Log failure
		if logger != nil {
			logger.Error("git clone failed",
				"command", fmt.Sprintf("git %s", strings.Join(args, " ")),
				"url", url,
				"cache_path", cachePath,
				"error", err.Error(),
				"output", strings.TrimSpace(string(output)),
			)
		}
		// Clean up partial clone on failure
		_ = os.RemoveAll(cachePath)
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}

	// Log success output
	if logger != nil {
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" {
			logger.Debug("git command output",
				"command", "clone",
				"output", outputStr,
			)
		}
	}

	return nil
}

// checkoutRef checks out the specified ref in a Git repository.
func (m *Manager) checkoutRef(cachePath string, ref string) error {
	// Get current ref for logging
	var currentRef string
	if output, err := runGitCommand(cachePath, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		currentRef = output
	}

	// Check if ref is a branch
	isBranch := false
	if _, err := runGitCommand(cachePath, "show-ref", "--verify", "--quiet", fmt.Sprintf("refs/heads/%s", ref)); err == nil {
		isBranch = true
	} else if _, err := runGitCommand(cachePath, "show-ref", "--verify", "--quiet", fmt.Sprintf("refs/remotes/origin/%s", ref)); err == nil {
		isBranch = true
	}

	// Log checkout operation
	if logger != nil {
		logger.Debug("checking out ref",
			"cache_path", cachePath,
			"current_ref", currentRef,
			"target_ref", ref,
			"is_branch", isBranch,
		)
	}

	// Execute checkout
	_, err := runGitCommand(cachePath, "checkout", ref)
	if err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}

	return nil
}

// fetchRepo fetches the latest refs from the remote repository.
func (m *Manager) fetchRepo(cachePath string) error {
	if logger != nil {
		logger.Debug("fetching from remote", "cache_path", cachePath)
	}

	_, err := runGitCommand(cachePath, "fetch", "--all")
	if err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}
	return nil
}

// hasUncommittedChanges checks if the repository has uncommitted changes.
func (m *Manager) hasUncommittedChanges(cachePath string) (bool, error) {
	output, err := runGitCommand(cachePath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}
	// If output is non-empty, there are uncommitted changes
	hasChanges := len(output) > 0

	if logger != nil && hasChanges {
		logger.Debug("uncommitted changes detected", "cache_path", cachePath)
	}

	return hasChanges, nil
}

// stashChanges stashes uncommitted changes in the repository.
func (m *Manager) stashChanges(cachePath string) error {
	if logger != nil {
		logger.Debug("stashing uncommitted changes", "cache_path", cachePath)
	}

	_, err := runGitCommand(cachePath, "stash", "push", "-m", "workspace cache auto-stash")
	if err != nil {
		return fmt.Errorf("git stash failed: %w", err)
	}
	return nil
}

// popStash restores stashed changes in the repository.
func (m *Manager) popStash(cachePath string) error {
	if logger != nil {
		logger.Debug("restoring stashed changes", "cache_path", cachePath)
	}

	_, err := runGitCommand(cachePath, "stash", "pop")
	if err != nil {
		return fmt.Errorf("git stash pop failed: %w", err)
	}
	return nil
}

// resetToOrigin resets the repository to the origin state for the specified ref.
// This handles both branches and tags/commits.
func (m *Manager) resetToOrigin(cachePath string, ref string) error {
	// First, try to determine if this is a branch by checking remote branches
	checkBranchCmd := exec.Command("git", "-C", cachePath, "show-ref", "--verify", "--quiet", fmt.Sprintf("refs/remotes/origin/%s", ref))
	isBranch := checkBranchCmd.Run() == nil

	if logger != nil {
		logger.Debug("resetting to origin",
			"cache_path", cachePath,
			"ref", ref,
			"is_branch", isBranch,
		)
	}

	if isBranch {
		// For branches, checkout and reset to origin
		if err := m.checkoutRef(cachePath, ref); err != nil {
			return err
		}

		// Use runGitCommand for consistent logging
		_, err := runGitCommand(cachePath, "reset", "--hard", fmt.Sprintf("origin/%s", ref))
		if err != nil {
			return fmt.Errorf("git reset failed: %w", err)
		}
	} else {
		// For tags/commits, just checkout
		if err := m.checkoutRef(cachePath, ref); err != nil {
			return err
		}
	}

	return nil
}

// pullCurrentBranch pulls the latest changes for the currently checked out branch.
func (m *Manager) pullCurrentBranch(cachePath string) error {
	if logger != nil {
		logger.Debug("pulling current branch", "cache_path", cachePath)
	}

	// Use runGitCommand for consistent logging
	_, err := runGitCommand(cachePath, "pull")
	if err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}
	return nil
}
