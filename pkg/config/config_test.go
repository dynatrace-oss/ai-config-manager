package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

func TestLoad_NoConfigFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Load config from directory without config file
	_, err = Load(tmpDir)

	// Should return error when no config file exists
	if err == nil {
		t.Fatalf("Load() expected error when no config file exists, got nil")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	tests := []struct {
		name          string
		configYAML    string
		wantTargets   []string
		wantToolTypes []tools.Tool
	}{
		{
			name:          "new format: single target",
			configYAML:    "install:\n  targets: [claude]\n",
			wantTargets:   []string{"claude"},
			wantToolTypes: []tools.Tool{tools.Claude},
		},
		{
			name:          "new format: multiple targets",
			configYAML:    "install:\n  targets: [claude, opencode]\n",
			wantTargets:   []string{"claude", "opencode"},
			wantToolTypes: []tools.Tool{tools.Claude, tools.OpenCode},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "config-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Write config file
			configPath := filepath.Join(tmpDir, DefaultConfigFileName)
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			// Load config
			config, err := Load(tmpDir)
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}

			// Check install targets
			if len(config.Install.Targets) != len(tt.wantTargets) {
				t.Errorf("Load().Install.Targets length = %v, want %v", len(config.Install.Targets), len(tt.wantTargets))
			}
			for i, target := range tt.wantTargets {
				if i >= len(config.Install.Targets) || config.Install.Targets[i] != target {
					t.Errorf("Load().Install.Targets[%d] = %v, want %v", i, config.Install.Targets, tt.wantTargets)
					break
				}
			}

			// Check GetDefaultTargets
			toolTypes, err := config.GetDefaultTargets()
			if err != nil {
				t.Fatalf("GetDefaultTargets() error = %v, want nil", err)
			}
			if len(toolTypes) != len(tt.wantToolTypes) {
				t.Errorf("GetDefaultTargets() length = %v, want %v", len(toolTypes), len(tt.wantToolTypes))
			}
			for i, wantType := range tt.wantToolTypes {
				if i >= len(toolTypes) || toolTypes[i] != wantType {
					t.Errorf("GetDefaultTargets()[%d] = %v, want %v", i, toolTypes[i], wantType)
				}
			}
		})
	}
}

func TestLoad_MissingTargets(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
	}{
		{
			name:       "old format: claude (no auto-migration)",
			configYAML: "default-tool: claude\n",
		},
		{
			name:       "old format: opencode (no auto-migration)",
			configYAML: "default-tool: opencode\n",
		},
		{
			name:       "empty config requires targets",
			configYAML: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "config-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Write config file
			configPath := filepath.Join(tmpDir, DefaultConfigFileName)
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			// Load config - should fail because install.targets is required
			_, err = Load(tmpDir)
			if err == nil {
				t.Errorf("Load() expected error for missing install.targets, got nil")
			}
		})
	}
}

func TestLoad_InvalidTool(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
	}{
		{
			name:       "invalid tool name in new format",
			configYAML: "install:\n  targets: [invalid]\n",
		},
		{
			name:       "unsupported tool in new format",
			configYAML: "install:\n  targets: [cursor]\n",
		},
		{
			name:       "invalid tool name in old format",
			configYAML: "default-tool: invalid\n",
		},
		{
			name:       "unsupported tool in old format",
			configYAML: "default-tool: cursor\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "config-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Write config file
			configPath := filepath.Join(tmpDir, DefaultConfigFileName)
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			// Load config - should fail validation
			_, err = Load(tmpDir)
			if err == nil {
				t.Errorf("Load() expected error for invalid tool, got nil")
			}
		})
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write malformed YAML
	configPath := filepath.Join(tmpDir, DefaultConfigFileName)
	malformedYAML := "default-tool: [invalid yaml\n"
	if err := os.WriteFile(configPath, []byte(malformedYAML), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Load config - should fail parsing
	_, err = Load(tmpDir)
	if err == nil {
		t.Errorf("Load() expected error for malformed YAML, got nil")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantError bool
	}{
		{
			name: "valid single target",
			config: Config{
				Install: InstallConfig{
					Targets: []string{"claude"},
				},
			},
			wantError: false,
		},
		{
			name: "valid multiple targets",
			config: Config{
				Install: InstallConfig{
					Targets: []string{"claude", "opencode", "copilot"},
				},
			},
			wantError: false,
		},
		{
			name: "empty is valid (uses default)",
			config: Config{
				Install: InstallConfig{
					Targets: []string{},
				},
			},
			wantError: false,
		},
		{
			name: "invalid tool in targets",
			config: Config{
				Install: InstallConfig{
					Targets: []string{"invalid"},
				},
			},
			wantError: true,
		},
		{
			name: "mixed valid and invalid tools",
			config: Config{
				Install: InstallConfig{
					Targets: []string{"claude", "invalid"},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError && err == nil {
				t.Errorf("Validate() expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestGetDefaultTargets(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantTools []tools.Tool
		wantError bool
	}{
		{
			name: "single target",
			config: Config{
				Install: InstallConfig{
					Targets: []string{"claude"},
				},
			},
			wantTools: []tools.Tool{tools.Claude},
			wantError: false,
		},
		{
			name: "multiple targets",
			config: Config{
				Install: InstallConfig{
					Targets: []string{"claude", "opencode"},
				},
			},
			wantTools: []tools.Tool{tools.Claude, tools.OpenCode},
			wantError: false,
		},
		{
			name: "all three tools",
			config: Config{
				Install: InstallConfig{
					Targets: []string{"claude", "opencode", "copilot"},
				},
			},
			wantTools: []tools.Tool{tools.Claude, tools.OpenCode, tools.Copilot},
			wantError: false,
		},
		{
			name: "empty targets returns error",
			config: Config{
				Install: InstallConfig{
					Targets: []string{},
				},
			},
			wantTools: nil,
			wantError: true,
		},
		{
			name: "invalid tool",
			config: Config{
				Install: InstallConfig{
					Targets: []string{"invalid"},
				},
			},
			wantTools: nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolsList, err := tt.config.GetDefaultTargets()
			if tt.wantError {
				if err == nil {
					t.Errorf("GetDefaultTargets() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("GetDefaultTargets() unexpected error: %v", err)
			}
			if len(toolsList) != len(tt.wantTools) {
				t.Errorf("GetDefaultTargets() length = %v, want %v", len(toolsList), len(tt.wantTools))
				return
			}
			for i, wantTool := range tt.wantTools {
				if toolsList[i] != wantTool {
					t.Errorf("GetDefaultTargets()[%d] = %v, want %v", i, toolsList[i], wantTool)
				}
			}
		})
	}
}

func TestGetConfigPath(t *testing.T) {
	// Test that GetConfigPath returns a valid path
	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath() error = %v, want nil", err)
	}

	// Path should end with aimgr/aimgr.yaml
	if !filepath.IsAbs(path) {
		t.Errorf("GetConfigPath() = %v, want absolute path", path)
	}

	// Path should contain .config/aimgr/aimgr.yaml
	if !filepath.IsAbs(path) || filepath.Base(path) != DefaultConfigFileName {
		t.Errorf("GetConfigPath() = %v, want path ending with %v", path, DefaultConfigFileName)
	}
}

func TestMigrateConfig(t *testing.T) {
	// Create temp directory for old config
	oldDir := t.TempDir()
	oldPath := filepath.Join(oldDir, OldConfigFileName)

	// Create temp directory for new config
	newDir := t.TempDir()
	newPath := filepath.Join(newDir, "aimgr", DefaultConfigFileName)

	// Write old config
	oldConfigData := []byte("default-tool: opencode\n")
	if err := os.WriteFile(oldPath, oldConfigData, 0644); err != nil {
		t.Fatalf("failed to write old config: %v", err)
	}

	// Migrate
	if err := migrateConfig(oldPath, newPath); err != nil {
		t.Fatalf("migrateConfig() error = %v, want nil", err)
	}

	// Check new config exists
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Errorf("new config file does not exist at %v", newPath)
	}

	// Check old config still exists
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		t.Errorf("old config file should still exist at %v", oldPath)
	}

	// Verify content
	newData, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("failed to read new config: %v", err)
	}
	if string(newData) != string(oldConfigData) {
		t.Errorf("new config content = %v, want %v", string(newData), string(oldConfigData))
	}
}

func TestLoadGlobal_NoConfig(t *testing.T) {
	// Create temp directory for isolated test environment
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override XDG_CONFIG_HOME to use temp directory
	// This isolates the test from the actual user's config
	oldXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer func() {
		os.Setenv("XDG_CONFIG_HOME", oldXDGConfigHome)
		xdg.Reload() // Restore original paths
	}()

	// Also override HOME to prevent fallback to ~/.ai-repo.yaml
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Reload XDG paths to pick up new environment variables
	xdg.Reload()

	// LoadGlobal should return default config when no config exists
	cfg, err := LoadGlobal()

	// Should not return error - returns default config instead
	if err != nil {
		t.Fatalf("LoadGlobal() unexpected error: %v", err)
	}

	// Verify default config has expected values
	if len(cfg.Install.Targets) != 1 || cfg.Install.Targets[0] != "claude" {
		t.Errorf("LoadGlobal() default config should have claude as target, got %v", cfg.Install.Targets)
	}
}

func TestValidate_RepoPath(t *testing.T) {
	tests := []struct {
		name      string
		repoPath  string
		wantError bool
		checkPath func(t *testing.T, path string)
	}{
		{
			name:      "empty path is valid",
			repoPath:  "",
			wantError: false,
		},
		{
			name:      "absolute path unchanged",
			repoPath:  "/home/user/custom-repo",
			wantError: false,
			checkPath: func(t *testing.T, path string) {
				if path != "/home/user/custom-repo" {
					t.Errorf("path = %q, want /home/user/custom-repo", path)
				}
			},
		},
		{
			name:      "tilde expansion",
			repoPath:  "~/my-repo",
			wantError: false,
			checkPath: func(t *testing.T, path string) {
				if contains(path, "~") {
					t.Errorf("path still contains ~: %q", path)
				}
				if !filepath.IsAbs(path) {
					t.Errorf("path not absolute after expansion: %q", path)
				}
			},
		},
		{
			name:      "relative path converted to absolute",
			repoPath:  "relative/path",
			wantError: false,
			checkPath: func(t *testing.T, path string) {
				if !filepath.IsAbs(path) {
					t.Errorf("path not absolute: %q", path)
				}
			},
		},
		{
			name:      "path with dot cleaning",
			repoPath:  "/home/user/./repo",
			wantError: false,
			checkPath: func(t *testing.T, path string) {
				if contains(path, "/.") {
					t.Errorf("path still contains /.: %q", path)
				}
				if path != "/home/user/repo" {
					t.Errorf("path = %q, want /home/user/repo", path)
				}
			},
		},
		{
			name:      "path with double-dot cleaning",
			repoPath:  "/home/user/foo/../repo",
			wantError: false,
			checkPath: func(t *testing.T, path string) {
				if contains(path, "..") {
					t.Errorf("path still contains ..: %q", path)
				}
				if path != "/home/user/repo" {
					t.Errorf("path = %q, want /home/user/repo", path)
				}
			},
		},
		{
			name:      "tilde with subdirectory",
			repoPath:  "~/projects/ai-repo",
			wantError: false,
			checkPath: func(t *testing.T, path string) {
				if contains(path, "~") {
					t.Errorf("path still contains ~: %q", path)
				}
				if !filepath.IsAbs(path) {
					t.Errorf("path not absolute after expansion: %q", path)
				}
				if !contains(path, "projects/ai-repo") && !contains(path, "projects\\ai-repo") {
					t.Errorf("path = %q, want to contain projects/ai-repo", path)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Install: InstallConfig{Targets: []string{"claude"}},
				Repo:    RepoConfig{Path: tt.repoPath},
			}

			err := cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}

			if !tt.wantError && tt.checkPath != nil {
				tt.checkPath(t, cfg.Repo.Path)
			}
		})
	}
}

func TestLoad_WithRepoPath(t *testing.T) {
	tests := []struct {
		name      string
		repoPath  string
		checkPath func(t *testing.T, path string)
	}{
		{
			name:     "absolute path",
			repoPath: "/custom/repo/path",
			checkPath: func(t *testing.T, path string) {
				if path != "/custom/repo/path" {
					t.Errorf("Repo.Path = %q, want /custom/repo/path", path)
				}
			},
		},
		{
			name:     "tilde expansion",
			repoPath: "~/custom-repo",
			checkPath: func(t *testing.T, path string) {
				if contains(path, "~") {
					t.Errorf("Repo.Path still contains ~: %q", path)
				}
				if !filepath.IsAbs(path) {
					t.Errorf("Repo.Path not absolute: %q", path)
				}
			},
		},
		{
			name:     "relative path",
			repoPath: "my-repo",
			checkPath: func(t *testing.T, path string) {
				if !filepath.IsAbs(path) {
					t.Errorf("Repo.Path not absolute: %q", path)
				}
			},
		},
		{
			name:     "path with dots",
			repoPath: "/tmp/../home/user/repo",
			checkPath: func(t *testing.T, path string) {
				if contains(path, "..") {
					t.Errorf("Repo.Path still contains ..: %q", path)
				}
				if path != "/home/user/repo" {
					t.Errorf("Repo.Path = %q, want /home/user/repo", path)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			configYAML := "install:\n  targets: [claude]\nrepo:\n  path: " + tt.repoPath + "\n"

			configPath := filepath.Join(tmpDir, DefaultConfigFileName)
			if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			cfg, err := Load(tmpDir)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if tt.checkPath != nil {
				tt.checkPath(t, cfg.Repo.Path)
			}
		})
	}
}

func TestLoad_WithEmptyRepoPath(t *testing.T) {
	tmpDir := t.TempDir()

	configYAML := `install:
  targets: [claude]
repo:
  path: ""
`

	configPath := filepath.Join(tmpDir, DefaultConfigFileName)
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Empty path should remain empty
	if cfg.Repo.Path != "" {
		t.Errorf("Repo.Path = %q, want empty string", cfg.Repo.Path)
	}
}

func TestLoad_WithoutRepoSection(t *testing.T) {
	tmpDir := t.TempDir()

	configYAML := `install:
  targets: [claude]
`

	configPath := filepath.Join(tmpDir, DefaultConfigFileName)
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Repo path should be empty when section is omitted
	if cfg.Repo.Path != "" {
		t.Errorf("Repo.Path = %q, want empty string", cfg.Repo.Path)
	}
}

func TestExpandEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "simple variable",
			input:    "path: ${HOME}/repo",
			envVars:  map[string]string{"HOME": "/home/user"},
			expected: "path: /home/user/repo",
		},
		{
			name:     "variable with default - var set",
			input:    "path: ${REPO_PATH:-/default/path}",
			envVars:  map[string]string{"REPO_PATH": "/custom/path"},
			expected: "path: /custom/path",
		},
		{
			name:     "variable with default - var unset",
			input:    "path: ${REPO_PATH:-/default/path}",
			envVars:  map[string]string{},
			expected: "path: /default/path",
		},
		{
			name:     "variable with default - var empty",
			input:    "path: ${REPO_PATH:-/default/path}",
			envVars:  map[string]string{"REPO_PATH": ""},
			expected: "path: /default/path",
		},
		{
			name:     "simple variable - unset becomes empty",
			input:    "path: ${MISSING_VAR}",
			envVars:  map[string]string{},
			expected: "path: ",
		},
		{
			name:     "multiple variables",
			input:    "url: ${PROTOCOL}://${HOST}:${PORT}",
			envVars:  map[string]string{"PROTOCOL": "https", "HOST": "example.com", "PORT": "8080"},
			expected: "url: https://example.com:8080",
		},
		{
			name:     "mixed with and without defaults",
			input:    "${VAR1}:${VAR2:-default2}:${VAR3}",
			envVars:  map[string]string{"VAR1": "value1", "VAR3": "value3"},
			expected: "value1:default2:value3",
		},
		{
			name:     "no variables",
			input:    "plain text without variables",
			envVars:  map[string]string{},
			expected: "plain text without variables",
		},
		{
			name:     "variable name with underscore",
			input:    "${MY_VAR_NAME}",
			envVars:  map[string]string{"MY_VAR_NAME": "test"},
			expected: "test",
		},
		{
			name:     "variable name with numbers",
			input:    "${VAR_123}",
			envVars:  map[string]string{"VAR_123": "test"},
			expected: "test",
		},
		{
			name:     "default with special characters",
			input:    "${VAR:-/path/to/default}",
			envVars:  map[string]string{},
			expected: "/path/to/default",
		},
		{
			name:     "default with spaces",
			input:    "${VAR:-default value}",
			envVars:  map[string]string{},
			expected: "default value",
		},
		{
			name:     "tilde in default",
			input:    "${REPO_PATH:-~/.local/share/repo}",
			envVars:  map[string]string{},
			expected: "~/.local/share/repo",
		},
		{
			name:     "empty default",
			input:    "${VAR:-}",
			envVars:  map[string]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Clear any vars not in the map
			// (t.Setenv handles cleanup automatically)

			result := expandEnvVars(tt.input)
			if result != tt.expected {
				t.Errorf("expandEnvVars() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestLoad_WithEnvVarExpansion(t *testing.T) {
	tests := []struct {
		name         string
		configYAML   string
		envVars      map[string]string
		wantRepoPath string
		wantError    bool
	}{
		{
			name: "expand env var in repo path",
			configYAML: `install:
  targets: [claude]
repo:
  path: ${CUSTOM_REPO}`,
			envVars:      map[string]string{"CUSTOM_REPO": "/tmp/test-repo"},
			wantRepoPath: "/tmp/test-repo",
			wantError:    false,
		},
		{
			name: "expand env var with default - var set",
			configYAML: `install:
  targets: [claude]
repo:
  path: ${CUSTOM_REPO:-/default/repo}`,
			envVars:      map[string]string{"CUSTOM_REPO": "/actual/repo"},
			wantRepoPath: "/actual/repo",
			wantError:    false,
		},
		{
			name: "expand env var with default - var unset",
			configYAML: `install:
  targets: [claude]
repo:
  path: ${CUSTOM_REPO:-/default/repo}`,
			envVars:      map[string]string{},
			wantRepoPath: "/default/repo",
			wantError:    false,
		},
		{
			name: "no env vars - static config",
			configYAML: `install:
  targets: [claude]
repo:
  path: /static/path`,
			envVars:      map[string]string{},
			wantRepoPath: "/static/path",
			wantError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Write config file
			configPath := filepath.Join(tmpDir, DefaultConfigFileName)
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			// Load config
			cfg, err := Load(tmpDir)
			if (err != nil) != tt.wantError {
				t.Errorf("Load() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if tt.wantError {
				return
			}

			// Verify repo path if specified
			if tt.wantRepoPath != "" {
				if cfg.Repo.Path != tt.wantRepoPath {
					t.Errorf("Repo.Path = %q, want %q", cfg.Repo.Path, tt.wantRepoPath)
				}
			}
		})
	}
}

func TestLoadGlobal_WithEnvVarExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	xdgConfigDir := filepath.Join(tmpDir, "config")

	t.Setenv("XDG_CONFIG_HOME", xdgConfigDir)
	t.Setenv("TEST_REPO_PATH", "/test/repo/path")

	// Create config directory
	configDir := filepath.Join(xdgConfigDir, "aimgr")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configYAML := `install:
  targets: [claude]
repo:
  path: ${TEST_REPO_PATH:-/default/path}`

	configPath := filepath.Join(configDir, DefaultConfigFileName)
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Reload XDG paths
	xdg.Reload()

	// Load global config
	cfg, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal() error = %v", err)
	}

	// Verify expansion worked
	if cfg.Repo.Path != "/test/repo/path" {
		t.Errorf("Repo.Path = %q, want /test/repo/path", cfg.Repo.Path)
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test TypeMappings methods
func TestTypeMappings_GetMapping(t *testing.T) {
	tm := TypeMappings{
		Skill: FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
					"claude":   "claude-sonnet-4",
				},
			},
		},
		Agent: FieldMappings{
			"model": {
				"gpt-4": {
					"opencode": "langdock/gpt-4",
				},
			},
		},
	}

	tests := []struct {
		name         string
		resourceType resource.ResourceType
		fieldName    string
		logicalValue string
		toolName     string
		wantValue    string
		wantFound    bool
	}{
		{
			name:         "skill model opencode mapping exists",
			resourceType: resource.Skill,
			fieldName:    "model",
			logicalValue: "sonnet-4.5",
			toolName:     "opencode",
			wantValue:    "langdock/claude-sonnet-4-5",
			wantFound:    true,
		},
		{
			name:         "skill model claude mapping exists",
			resourceType: resource.Skill,
			fieldName:    "model",
			logicalValue: "sonnet-4.5",
			toolName:     "claude",
			wantValue:    "claude-sonnet-4",
			wantFound:    true,
		},
		{
			name:         "skill model windsurf mapping not found",
			resourceType: resource.Skill,
			fieldName:    "model",
			logicalValue: "sonnet-4.5",
			toolName:     "windsurf",
			wantValue:    "",
			wantFound:    false,
		},
		{
			name:         "agent model mapping exists",
			resourceType: resource.Agent,
			fieldName:    "model",
			logicalValue: "gpt-4",
			toolName:     "opencode",
			wantValue:    "langdock/gpt-4",
			wantFound:    true,
		},
		{
			name:         "command has no mappings",
			resourceType: resource.Command,
			fieldName:    "model",
			logicalValue: "sonnet-4.5",
			toolName:     "opencode",
			wantValue:    "",
			wantFound:    false,
		},
		{
			name:         "unknown resource type",
			resourceType: resource.PackageType,
			fieldName:    "model",
			logicalValue: "sonnet-4.5",
			toolName:     "opencode",
			wantValue:    "",
			wantFound:    false,
		},
		{
			name:         "unknown field name",
			resourceType: resource.Skill,
			fieldName:    "unknown",
			logicalValue: "sonnet-4.5",
			toolName:     "opencode",
			wantValue:    "",
			wantFound:    false,
		},
		{
			name:         "unknown logical value",
			resourceType: resource.Skill,
			fieldName:    "model",
			logicalValue: "unknown-model",
			toolName:     "opencode",
			wantValue:    "",
			wantFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotFound := tm.GetMapping(tt.resourceType, tt.fieldName, tt.logicalValue, tt.toolName)
			if gotValue != tt.wantValue {
				t.Errorf("GetMapping() value = %q, want %q", gotValue, tt.wantValue)
			}
			if gotFound != tt.wantFound {
				t.Errorf("GetMapping() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

func TestTypeMappings_GetMappingWithNull(t *testing.T) {
	tm := TypeMappings{
		Skill: FieldMappings{
			"model": {
				"sonnet-4.5": {
					"opencode": "langdock/claude-sonnet-4-5",
				},
				"null": {
					"opencode": "langdock/default-model",
				},
			},
		},
	}

	tests := []struct {
		name         string
		resourceType resource.ResourceType
		fieldName    string
		logicalValue string
		toolName     string
		wantValue    string
		wantFound    bool
	}{
		{
			name:         "non-empty value uses direct mapping",
			resourceType: resource.Skill,
			fieldName:    "model",
			logicalValue: "sonnet-4.5",
			toolName:     "opencode",
			wantValue:    "langdock/claude-sonnet-4-5",
			wantFound:    true,
		},
		{
			name:         "empty value uses null mapping",
			resourceType: resource.Skill,
			fieldName:    "model",
			logicalValue: "",
			toolName:     "opencode",
			wantValue:    "langdock/default-model",
			wantFound:    true,
		},
		{
			name:         "empty value with no null mapping",
			resourceType: resource.Skill,
			fieldName:    "model",
			logicalValue: "",
			toolName:     "claude",
			wantValue:    "",
			wantFound:    false,
		},
		{
			name:         "non-empty unknown value not found",
			resourceType: resource.Skill,
			fieldName:    "model",
			logicalValue: "unknown-model",
			toolName:     "opencode",
			wantValue:    "",
			wantFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotFound := tm.GetMappingWithNull(tt.resourceType, tt.fieldName, tt.logicalValue, tt.toolName)
			if gotValue != tt.wantValue {
				t.Errorf("GetMappingWithNull() value = %q, want %q", gotValue, tt.wantValue)
			}
			if gotFound != tt.wantFound {
				t.Errorf("GetMappingWithNull() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

func TestTypeMappings_GetToolsWithMappings(t *testing.T) {
	tests := []struct {
		name      string
		mappings  TypeMappings
		wantTools []string
	}{
		{
			name:      "empty mappings",
			mappings:  TypeMappings{},
			wantTools: []string{},
		},
		{
			name: "single tool in skill",
			mappings: TypeMappings{
				Skill: FieldMappings{
					"model": {
						"sonnet-4.5": {
							"opencode": "value",
						},
					},
				},
			},
			wantTools: []string{"opencode"},
		},
		{
			name: "multiple tools across types",
			mappings: TypeMappings{
				Skill: FieldMappings{
					"model": {
						"sonnet-4.5": {
							"opencode": "value1",
							"claude":   "value2",
						},
					},
				},
				Agent: FieldMappings{
					"model": {
						"gpt-4": {
							"windsurf": "value3",
						},
					},
				},
			},
			wantTools: []string{"claude", "opencode", "windsurf"},
		},
		{
			name: "deduplicates tools",
			mappings: TypeMappings{
				Skill: FieldMappings{
					"field1": {
						"value1": {"opencode": "mapped1"},
					},
					"field2": {
						"value2": {"opencode": "mapped2"},
					},
				},
			},
			wantTools: []string{"opencode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTools := tt.mappings.GetToolsWithMappings()
			if len(gotTools) != len(tt.wantTools) {
				t.Errorf("GetToolsWithMappings() = %v, want %v", gotTools, tt.wantTools)
				return
			}
			for i, want := range tt.wantTools {
				if gotTools[i] != want {
					t.Errorf("GetToolsWithMappings()[%d] = %q, want %q", i, gotTools[i], want)
				}
			}
		})
	}
}

func TestTypeMappings_HasAny(t *testing.T) {
	tests := []struct {
		name     string
		mappings TypeMappings
		want     bool
	}{
		{
			name:     "empty mappings",
			mappings: TypeMappings{},
			want:     false,
		},
		{
			name: "skill has mappings",
			mappings: TypeMappings{
				Skill: FieldMappings{"model": {"value": {"tool": "mapped"}}},
			},
			want: true,
		},
		{
			name: "agent has mappings",
			mappings: TypeMappings{
				Agent: FieldMappings{"model": {"value": {"tool": "mapped"}}},
			},
			want: true,
		},
		{
			name: "command has mappings",
			mappings: TypeMappings{
				Command: FieldMappings{"model": {"value": {"tool": "mapped"}}},
			},
			want: true,
		},
		{
			name: "nil field mappings",
			mappings: TypeMappings{
				Skill:   nil,
				Agent:   nil,
				Command: nil,
			},
			want: false,
		},
		{
			name: "empty field mappings",
			mappings: TypeMappings{
				Skill:   FieldMappings{},
				Agent:   FieldMappings{},
				Command: FieldMappings{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mappings.HasAny(); got != tt.want {
				t.Errorf("HasAny() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad_WithMappings(t *testing.T) {
	tests := []struct {
		name          string
		configYAML    string
		wantSkillMap  bool
		wantAgentMap  bool
		wantHasAny    bool
		checkMappings func(t *testing.T, cfg *Config)
	}{
		{
			name: "config with mappings section",
			configYAML: `install:
  targets: [claude]
mappings:
  skill:
    model:
      sonnet-4.5:
        opencode: "langdock/claude-sonnet-4-5"
        claude: "claude-sonnet-4"
`,
			wantSkillMap: true,
			wantAgentMap: false,
			wantHasAny:   true,
			checkMappings: func(t *testing.T, cfg *Config) {
				val, found := cfg.Mappings.GetMapping(resource.Skill, "model", "sonnet-4.5", "opencode")
				if !found || val != "langdock/claude-sonnet-4-5" {
					t.Errorf("expected opencode mapping, got value=%q found=%v", val, found)
				}
			},
		},
		{
			name: "config without mappings section (backwards compatible)",
			configYAML: `install:
  targets: [claude]
`,
			wantSkillMap: false,
			wantAgentMap: false,
			wantHasAny:   false,
		},
		{
			name: "config with empty mappings section",
			configYAML: `install:
  targets: [claude]
mappings: {}
`,
			wantSkillMap: false,
			wantAgentMap: false,
			wantHasAny:   false,
		},
		{
			name: "config with null mapping",
			configYAML: `install:
  targets: [claude]
mappings:
  skill:
    model:
      "null":
        opencode: "langdock/default-model"
`,
			wantSkillMap: true,
			wantHasAny:   true,
			checkMappings: func(t *testing.T, cfg *Config) {
				val, found := cfg.Mappings.GetMappingWithNull(resource.Skill, "model", "", "opencode")
				if !found || val != "langdock/default-model" {
					t.Errorf("expected null mapping, got value=%q found=%v", val, found)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			configPath := filepath.Join(tmpDir, DefaultConfigFileName)
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			cfg, err := Load(tmpDir)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			// Check HasAny
			if got := cfg.Mappings.HasAny(); got != tt.wantHasAny {
				t.Errorf("HasAny() = %v, want %v", got, tt.wantHasAny)
			}

			// Check specific mapping presence
			hasSkillMap := len(cfg.Mappings.Skill) > 0
			if hasSkillMap != tt.wantSkillMap {
				t.Errorf("Skill mappings present = %v, want %v", hasSkillMap, tt.wantSkillMap)
			}

			hasAgentMap := len(cfg.Mappings.Agent) > 0
			if hasAgentMap != tt.wantAgentMap {
				t.Errorf("Agent mappings present = %v, want %v", hasAgentMap, tt.wantAgentMap)
			}

			// Run custom mapping checks
			if tt.checkMappings != nil {
				tt.checkMappings(t, cfg)
			}
		})
	}
}

func TestValidate_MappingsWarnsUnknownTools(t *testing.T) {
	// This test verifies that unknown tool names produce a warning but don't error
	cfg := &Config{
		Install: InstallConfig{Targets: []string{"claude"}},
		Mappings: TypeMappings{
			Skill: FieldMappings{
				"model": {
					"value": {
						"unknowntool": "mapped-value",
						"opencode":    "known-value",
					},
				},
			},
		},
	}

	// Validate should succeed (warning only, no error)
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() should not error for unknown tools, got: %v", err)
	}
}
