package repo

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/logging"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
)

// Manager manages the AI resources repository
type Manager struct {
	repoPath string
	logger   *slog.Logger
	logLevel slog.Level
}

// NewManager creates a new repository manager
// Repository path determined by 3-level precedence:
// 1. AIMGR_REPO_PATH environment variable (highest priority)
// 2. repo.path from config file (~/.config/aimgr/aimgr.yaml)
// 3. XDG default (~/.local/share/ai-config/repo/)
func NewManager() (*Manager, error) {
	var repoPath string

	// Priority 1: Check for environment variable override first
	repoPath = os.Getenv("AIMGR_REPO_PATH")
	if repoPath != "" {
		m := &Manager{
			repoPath: repoPath,
			logLevel: slog.LevelInfo, // Default to Info
		}
		m.initLogger()
		return m, nil
	}

	// Priority 2: Check config file for repo.path
	cfg, err := config.LoadGlobal()
	if err == nil && cfg.Repo.Path != "" {
		// Config loaded successfully and has repo.path set
		// Path is already validated and expanded by config.Validate()
		m := &Manager{
			repoPath: cfg.Repo.Path,
			logLevel: slog.LevelInfo, // Default to Info
		}
		m.initLogger()
		return m, nil
	}

	// Priority 3: Fall back to XDG default
	// Ignore config errors - user may not have config file
	repoPath = filepath.Join(xdg.DataHome, "ai-config", "repo")
	m := &Manager{
		repoPath: repoPath,
		logLevel: slog.LevelInfo, // Default to Info
	}
	m.initLogger()
	return m, nil
}

// NewManagerWithPath creates a manager with a custom repository path (for testing)
func NewManagerWithPath(repoPath string) *Manager {
	m := &Manager{
		repoPath: repoPath,
		logLevel: slog.LevelDebug, // Tests use Debug level
	}
	m.initLogger()
	return m
}

// SetLogLevel sets the log level and reinitializes the logger.
// This should be called after creating the manager and before using it.
func (m *Manager) SetLogLevel(level slog.Level) {
	m.logLevel = level
	m.initLogger()
}

// initLogger initializes the logger for this Manager.
// If logger creation fails, logs a warning to stderr but continues.
// Manager can operate without logging (graceful degradation).
func (m *Manager) initLogger() {
	logger, err := logging.NewRepoLogger(m.repoPath, m.logLevel)
	if err != nil {
		// Log warning to stderr but don't fail
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize logger: %v\n", err)
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

// Init initializes the repository directory structure and git repository.
// This is idempotent - safe to call multiple times.
//
//nolint:gocyclo // Sequential initialization of directory structure, git repo, gitignore, and initial commit; must remain atomic for idempotency.
func (m *Manager) Init() error {
	// Create main repo directory
	if m.logger != nil {
		m.logger.Debug("creating repo directory",
			"path", m.repoPath,
			"permissions", "0755",
		)
	}
	if err := os.MkdirAll(m.repoPath, 0755); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to create repo directory",
				"path", m.repoPath,
				"error", err.Error(),
			)
		}
		return fmt.Errorf("failed to create repo directory: %w", err)
	}

	// Create commands subdirectory
	commandsPath := filepath.Join(m.repoPath, "commands")
	if m.logger != nil {
		m.logger.Debug("creating commands directory",
			"path", commandsPath,
			"permissions", "0755",
		)
	}
	if err := os.MkdirAll(commandsPath, 0755); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to create commands directory",
				"path", commandsPath,
				"error", err.Error(),
			)
		}
		return fmt.Errorf("failed to create commands directory: %w", err)
	}

	// Create skills subdirectory
	skillsPath := filepath.Join(m.repoPath, "skills")
	if m.logger != nil {
		m.logger.Debug("creating skills directory",
			"path", skillsPath,
			"permissions", "0755",
		)
	}
	if err := os.MkdirAll(skillsPath, 0755); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to create skills directory",
				"path", skillsPath,
				"error", err.Error(),
			)
		}
		return fmt.Errorf("failed to create skills directory: %w", err)
	}

	// Create agents subdirectory
	agentsPath := filepath.Join(m.repoPath, "agents")
	if m.logger != nil {
		m.logger.Debug("creating agents directory",
			"path", agentsPath,
			"permissions", "0755",
		)
	}
	if err := os.MkdirAll(agentsPath, 0755); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to create agents directory",
				"path", agentsPath,
				"error", err.Error(),
			)
		}
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	// Create packages subdirectory
	packagesPath := filepath.Join(m.repoPath, "packages")
	if m.logger != nil {
		m.logger.Debug("creating packages directory",
			"path", packagesPath,
			"permissions", "0755",
		)
	}
	if err := os.MkdirAll(packagesPath, 0755); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to create packages directory",
				"path", packagesPath,
				"error", err.Error(),
			)
		}
		return fmt.Errorf("failed to create packages directory: %w", err)
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
		// Log that manifest was created (helpful for upgrades from pre-manifest versions)
		fmt.Fprintf(os.Stderr, "Created ai.repo.yaml\n")
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
