package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"gopkg.in/yaml.v3"
)

// TestSaveConfig tests the saveConfig helper function
func TestSaveConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.Config
		expectError bool
	}{
		{
			name: "save valid config with single target",
			cfg: &config.Config{
				Install: config.InstallConfig{
					Targets: []string{"claude"},
				},
			},
			expectError: false,
		},
		{
			name: "save valid config with multiple targets",
			cfg: &config.Config{
				Install: config.InstallConfig{
					Targets: []string{"claude", "opencode", "copilot"},
				},
			},
			expectError: false,
		},
		{
			name: "save empty config",
			cfg: &config.Config{
				Install: config.InstallConfig{
					Targets: []string{},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "aimgr.yaml")

			// Run saveConfig
			err := saveConfig(configPath, tt.cfg)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify file was created
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Fatal("Config file was not created")
			}

			// Load and verify config
			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("Failed to read config file: %v", err)
			}

			var actualConfig config.Config
			if err := yaml.Unmarshal(data, &actualConfig); err != nil {
				t.Fatalf("Failed to parse config file: %v", err)
			}

			// Compare targets
			if len(actualConfig.Install.Targets) != len(tt.cfg.Install.Targets) {
				t.Fatalf("Expected %d targets, got %d", len(tt.cfg.Install.Targets), len(actualConfig.Install.Targets))
			}

			for i, target := range tt.cfg.Install.Targets {
				if actualConfig.Install.Targets[i] != target {
					t.Errorf("Expected target[%d] to be '%s', got '%s'", i, target, actualConfig.Install.Targets[i])
				}
			}
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single value",
			input:    "claude",
			expected: []string{"claude"},
		},
		{
			name:     "multiple values",
			input:    "claude,opencode",
			expected: []string{"claude", "opencode"},
		},
		{
			name:     "values with spaces",
			input:    "claude, opencode, copilot",
			expected: []string{"claude", "opencode", "copilot"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "trailing comma",
			input:    "claude,opencode,",
			expected: []string{"claude", "opencode"},
		},
		{
			name:     "leading comma",
			input:    ",claude,opencode",
			expected: []string{"claude", "opencode"},
		},
		{
			name:     "extra spaces",
			input:    "  claude  ,  opencode  ",
			expected: []string{"claude", "opencode"},
		},
		{
			name:     "consecutive commas",
			input:    "claude,,opencode",
			expected: []string{"claude", "opencode"},
		},
		{
			name:     "all three tools",
			input:    "claude,opencode,copilot",
			expected: []string{"claude", "opencode", "copilot"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndTrim(tt.input)

			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d parts, got %d: %v", len(tt.expected), len(result), result)
			}

			for i, part := range result {
				if part != tt.expected[i] {
					t.Errorf("Expected part[%d] to be '%s', got '%s'", i, tt.expected[i], part)
				}
			}
		})
	}
}
