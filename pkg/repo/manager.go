package repo

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/logging"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repolock"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
)

// Manager manages the AI resources repository
type Manager struct {
	repoPath string
	logger   *slog.Logger
	logLevel slog.Level
	locks    *repolock.Manager
}

// NewManager creates a new repository manager
// Repository path determined by 3-level precedence:
// 1. AIMGR_REPO_PATH environment variable (highest priority)
// 2. repo.path from config file (~/.config/aimgr/aimgr.yaml)
// 3. XDG default (~/.local/share/ai-config/repo/)
func NewManager() (*Manager, error) {
	repoPath := ResolveRepoPath()
	m := &Manager{
		repoPath: repoPath,
		logLevel: slog.LevelInfo, // Default to Info
		locks:    repolock.NewManager(repoPath),
	}
	m.initLogger()
	return m, nil
}

// ResolveRepoPath determines the effective repository path using the same
// precedence as NewManager, but without initializing a Manager or logger.
//
// Precedence:
// 1. AIMGR_REPO_PATH environment variable
// 2. repo.path from global config
// 3. XDG default (~/.local/share/ai-config/repo)
func ResolveRepoPath() string {
	if repoPath := os.Getenv("AIMGR_REPO_PATH"); repoPath != "" {
		return repoPath
	}

	// Ignore config loading errors to match NewManager behavior.
	if cfg, err := config.LoadGlobal(); err == nil && cfg.Repo.Path != "" {
		return cfg.Repo.Path
	}

	return filepath.Join(xdg.DataHome, "ai-config", "repo")
}

// NewManagerWithPath creates a manager with a custom repository path (for testing)
func NewManagerWithPath(repoPath string) *Manager {
	m := &Manager{
		repoPath: repoPath,
		logLevel: slog.LevelDebug, // Tests use Debug level
		locks:    repolock.NewManager(repoPath),
	}
	m.initLogger()
	return m
}

func repoLockAcquireTimeout() time.Duration {
	timeout := 30 * time.Second

	override := strings.TrimSpace(os.Getenv("AIMGR_TEST_REPO_LOCK_TIMEOUT"))
	if override == "" {
		return timeout
	}

	parsed, err := time.ParseDuration(override)
	if err != nil || parsed <= 0 {
		return timeout
	}

	return parsed
}

// SetLogLevel sets the log level and reinitializes the logger.
// This should be called after creating the manager and before using it.
func (m *Manager) SetLogLevel(level slog.Level) {
	m.logLevel = level
	m.initLogger()
}

// initLogger initializes the logger for this Manager.
// If logger creation fails, it falls back to nil logger and continues.
// Manager can operate without logging (graceful degradation).
func (m *Manager) initLogger() {
	logger, err := logging.NewRepoLogger(m.repoPath, m.logLevel)
	if err != nil {
		m.logger = nil
		return
	}
	m.logger = logger
}

// GetLogger returns the logger for this Manager, or nil if logger creation failed.
func (m *Manager) GetLogger() *slog.Logger {
	return m.logger
}

// GetRepoPath returns the repository root path
func (m *Manager) GetRepoPath() string {
	return m.repoPath
}

// Locking conventions for repo mutation (foundational contract):
//
// - Lock ordering is always: repo lock (read or write) -> cache lock (exclusive)
//   -> workspace metadata lock (exclusive).
// - Repo lock is non-reentrant by design. Same-process reacquisition of the same
//   path fails with ErrNonReentrantLock, including read->write and write->read
//   transition attempts.
// - Top-level CLI command handlers should hold the outer repo lock and call
//   internal helpers (including Init) inside that critical section.

// AcquireRepoReadLock acquires the repository-wide shared/read lock with default
// CLI semantics: blocking up to 30s and honoring context cancellation.
func (m *Manager) AcquireRepoReadLock(ctx context.Context) (*repolock.Lock, error) {
	return repolock.AcquireShared(ctx, m.locks.RepoLockPath(), repoLockAcquireTimeout())
}

// AcquireRepoReadLockWithTimeout acquires the repository-wide shared/read lock
// with a caller-provided timeout.
func (m *Manager) AcquireRepoReadLockWithTimeout(ctx context.Context, timeout time.Duration) (*repolock.Lock, error) {
	return repolock.AcquireShared(ctx, m.locks.RepoLockPath(), timeout)
}

// TryAcquireRepoReadLock attempts to acquire the repository-wide shared/read
// lock without waiting.
func (m *Manager) TryAcquireRepoReadLock() (*repolock.Lock, bool, error) {
	return m.locks.TryAcquireRepoReadLock()
}

// AcquireRepoWriteLock acquires the repository-wide exclusive/write lock with
// default CLI semantics: blocking up to 30s and honoring context cancellation.
func (m *Manager) AcquireRepoWriteLock(ctx context.Context) (*repolock.Lock, error) {
	return repolock.AcquireExclusive(ctx, m.locks.RepoLockPath(), repoLockAcquireTimeout())
}

// AcquireRepoWriteLockWithTimeout acquires the repository-wide exclusive/write
// lock with a caller-provided timeout.
func (m *Manager) AcquireRepoWriteLockWithTimeout(ctx context.Context, timeout time.Duration) (*repolock.Lock, error) {
	return repolock.AcquireExclusive(ctx, m.locks.RepoLockPath(), timeout)
}

// TryAcquireRepoWriteLock attempts to acquire the repository-wide
// exclusive/write lock without waiting.
func (m *Manager) TryAcquireRepoWriteLock() (*repolock.Lock, bool, error) {
	return m.locks.TryAcquireRepoWriteLock()
}

// AcquireWorkspaceMetadataLock acquires the exclusive workspace metadata lock.
func (m *Manager) AcquireWorkspaceMetadataLock(ctx context.Context) (*repolock.Lock, error) {
	return m.locks.AcquireWorkspaceMetadataLock(ctx)
}

// AcquireCacheLock acquires the per-cache lock for a specific cache hash.
func (m *Manager) AcquireCacheLock(ctx context.Context, cacheHash string) (*repolock.Lock, error) {
	return m.locks.AcquireCacheLock(ctx, cacheHash)
}

// RepoLockPath returns the repo lock file path under .workspace/locks.
func (m *Manager) RepoLockPath() string {
	return m.locks.RepoLockPath()
}

// WorkspaceMetadataLockPath returns the workspace metadata lock file path.
func (m *Manager) WorkspaceMetadataLockPath() string {
	return m.locks.WorkspaceMetadataLockPath()
}

// CacheLockPath returns the per-cache lock path for the given cache hash.
func (m *Manager) CacheLockPath(cacheHash string) string {
	return m.locks.CacheLockPath(cacheHash)
}

// Init initializes the repository directory structure and git repository.
// This is idempotent - safe to call multiple times.
//
//nolint:gocyclo // Sequential initialization of directory structure, git repo, gitignore, and initial commit; must remain atomic for idempotency.
func (m *Manager) Init() error {
	// Create repo directory and standard subdirectories
	dirs := []struct {
		path  string
		label string
	}{
		{m.repoPath, "repo"},
		{filepath.Join(m.repoPath, "commands"), "commands"},
		{filepath.Join(m.repoPath, "skills"), "skills"},
		{filepath.Join(m.repoPath, "agents"), "agents"},
		{filepath.Join(m.repoPath, "packages"), "packages"},
	}
	for _, d := range dirs {
		if m.logger != nil {
			m.logger.Debug("creating "+d.label+" directory",
				"path", d.path,
				"permissions", "0755",
			)
		}
		if err := os.MkdirAll(d.path, 0755); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to create "+d.label+" directory",
					"path", d.path,
					"error", err.Error(),
				)
			}
			return fmt.Errorf("failed to create %s directory: %w", d.label, err)
		}
	}

	// Initialize git repository if not already initialized
	gitDir := filepath.Join(m.repoPath, ".git")
	alreadyGit := false
	if _, err := os.Stat(gitDir); err == nil {
		alreadyGit = true
	}

	if !alreadyGit {
		gitCmd := exec.Command("git", "init")
		gitCmd.Dir = m.repoPath
		if output, err := gitCmd.CombinedOutput(); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to initialize git repository",
					"path", m.repoPath,
					"error", err.Error(),
					"output", string(output),
				)
			}
			return fmt.Errorf("failed to initialize git repository: %w\nOutput: %s", err, output)
		}
	}

	// Log repo initialization
	if m.logger != nil {
		m.logger.Info("repo init",
			"path", m.repoPath,
		)
	}

	// Create ai.repo.yaml if it doesn't exist
	// NOTE: This handles the upgrade path for existing users. When upgrading from a version
	// without ai.repo.yaml, Init() will automatically create an empty manifest on first run.
	// This is idempotent and safe - the file is only created if missing.
	// TODO(release): Document in migration guide that ai.repo.yaml is auto-created on upgrade.
	manifestPath := filepath.Join(m.repoPath, repomanifest.ManifestFileName)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		// Create empty manifest
		manifest := &repomanifest.Manifest{
			Version: 1,
			Sources: []*repomanifest.Source{},
		}
		if err := manifest.Save(m.repoPath); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to create ai.repo.yaml",
					"path", manifestPath,
					"error", err.Error(),
				)
			}
			return fmt.Errorf("failed to create ai.repo.yaml: %w", err)
		}
		// Keep CLI success messaging on stdout; avoid stderr output during successful init.
	} else if err != nil {
		return fmt.Errorf("failed to check ai.repo.yaml: %w", err)
	}
	// If file exists, do nothing (idempotent)

	// Create/update .gitignore (idempotent)
	gitignorePath := filepath.Join(m.repoPath, ".gitignore")
	gitignoreContent := `# aimgr workspace cache (Git clones for remote sources)
.workspace/

# Log files
logs/
*.log

# macOS
.DS_Store

# Editor files
*.swp
*.swo
*~
.vscode/
.idea/
`

	if _, err := os.Stat(gitignorePath); err == nil {
		// .gitignore exists - check if it contains .workspace/
		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			if m.logger != nil {
				m.logger.Error("failed to read .gitignore",
					"path", gitignorePath,
					"error", err.Error(),
				)
			}
			return fmt.Errorf("failed to read .gitignore: %w", err)
		}

		// If .workspace/ is not in .gitignore, append it
		if !strings.Contains(string(content), ".workspace/") &&
			!strings.Contains(string(content), ".workspace") {
			f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				if m.logger != nil {
					m.logger.Error("failed to open .gitignore for append",
						"path", gitignorePath,
						"error", err.Error(),
					)
				}
				return fmt.Errorf("failed to open .gitignore for append: %w", err)
			}
			defer f.Close()

			if _, err := f.WriteString("\n" + gitignoreContent); err != nil {
				if m.logger != nil {
					m.logger.Error("failed to append to .gitignore",
						"path", gitignorePath,
						"error", err.Error(),
					)
				}
				return fmt.Errorf("failed to append to .gitignore: %w", err)
			}
		}
	} else {
		// Create new .gitignore
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to create .gitignore",
					"path", gitignorePath,
					"error", err.Error(),
				)
			}
			return fmt.Errorf("failed to create .gitignore: %w", err)
		}
	}

	// Initial commit if git was just initialized
	if !alreadyGit {
		// Add .gitignore and ai.repo.yaml
		// Don't fail on add error - might not have anything to add
		addCmd := exec.Command("git", "add", ".gitignore", repomanifest.ManifestFileName)
		addCmd.Dir = m.repoPath
		_, _ = addCmd.CombinedOutput()

		// Create initial commit
		commitCmd := exec.Command("git", "commit", "-m", "aimgr: initialize repository")
		commitCmd.Dir = m.repoPath
		// Don't fail on commit error - might not have anything to commit
		_, _ = commitCmd.CombinedOutput()
	}

	return nil
}
