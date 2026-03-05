package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/logging"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
)

func TestRootCommand(t *testing.T) {
	// Verify command metadata
	if rootCmd.Use != "aimgr" {
		t.Errorf("Expected Use 'aimgr', got %s", rootCmd.Use)
	}

	if rootCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if rootCmd.Long == "" {
		t.Error("Long description should not be empty")
	}
}

func TestRootCommandHasExpectedSubcommands(t *testing.T) {
	// Verify root command has expected subcommands
	expectedSubcommands := []string{
		"init",
		"install",
		"uninstall",
		"list",
		"clean",
		"verify",
		"repo",
		"config",
		"completion",
	}

	for _, expectedCmd := range expectedSubcommands {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == expectedCmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected subcommand %s not found", expectedCmd)
		}
	}
}

func TestRootCommandFlags(t *testing.T) {
	// Verify persistent flags exist
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Error("Expected --config flag to exist")
	}

	logLevelFlag := rootCmd.PersistentFlags().Lookup("log-level")
	if logLevelFlag == nil {
		t.Error("Expected --log-level flag to exist")
	}

	// Verify local flags
	versionFlag := rootCmd.Flags().Lookup("version")
	if versionFlag == nil {
		t.Error("Expected --version flag to exist")
	}
}

func TestRootCommandVersionFlag(t *testing.T) {
	// Test that version flag is properly configured
	versionFlag := rootCmd.Flags().Lookup("version")
	if versionFlag == nil {
		t.Fatal("Expected --version flag to exist")
	}

	if versionFlag.Shorthand != "v" {
		t.Errorf("Expected version flag shorthand 'v', got %s", versionFlag.Shorthand)
	}
}

func TestGetLogLevel(t *testing.T) {
	// Save original value
	originalLogLevel := logLevel
	defer func() { logLevel = originalLogLevel }()

	// Test getting log level
	testLevels := []string{"debug", "info", "warn", "error"}

	for _, level := range testLevels {
		logLevel = level
		got := GetLogLevel()
		if got != level {
			t.Errorf("GetLogLevel() = %v, want %v", got, level)
		}
	}
}

func TestNewManagerWithLogLevel(t *testing.T) {
	// Create temp directory for test repository
	repoDir := t.TempDir()

	// Set AIMGR_REPO_PATH to use test directory
	oldEnv := os.Getenv("AIMGR_REPO_PATH")
	defer func() {
		if oldEnv != "" {
			_ = os.Setenv("AIMGR_REPO_PATH", oldEnv)
		} else {
			_ = os.Unsetenv("AIMGR_REPO_PATH")
		}
	}()
	_ = os.Setenv("AIMGR_REPO_PATH", repoDir)

	// Initialize repo first
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Save original log level
	originalLogLevel := logLevel
	defer func() { logLevel = originalLogLevel }()

	tests := []struct {
		name        string
		logLevel    string
		expectError bool
	}{
		{
			name:        "valid log level info",
			logLevel:    "info",
			expectError: false,
		},
		{
			name:        "valid log level debug",
			logLevel:    "debug",
			expectError: false,
		},
		{
			name:        "valid log level warn",
			logLevel:    "warn",
			expectError: false,
		},
		{
			name:        "valid log level error",
			logLevel:    "error",
			expectError: false,
		},
		{
			name:        "invalid log level",
			logLevel:    "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logLevel = tt.logLevel

			mgr, err := NewManagerWithLogLevel()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if mgr == nil {
				t.Error("Expected manager to be created")
			}
		})
	}
}

func TestNewManagerWithLogLevelSetsLogger(t *testing.T) {
	// Create temp directory for test repository
	repoDir := t.TempDir()

	// Set AIMGR_REPO_PATH to use test directory
	oldEnv := os.Getenv("AIMGR_REPO_PATH")
	defer func() {
		if oldEnv != "" {
			_ = os.Setenv("AIMGR_REPO_PATH", oldEnv)
		} else {
			_ = os.Unsetenv("AIMGR_REPO_PATH")
		}
	}()
	_ = os.Setenv("AIMGR_REPO_PATH", repoDir)

	// Initialize repo first
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Save original log level
	originalLogLevel := logLevel
	defer func() { logLevel = originalLogLevel }()

	logLevel = "info"

	mgr, err := NewManagerWithLogLevel()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Verify logger was set
	logger := mgr.GetLogger()
	if logger == nil {
		t.Error("Expected logger to be set on manager")
	}
}

func TestRootCommandDocumentation(t *testing.T) {
	// Verify long description mentions key concepts
	expectedMentions := []string{
		"AI resources",
		"commands",
		"skills",
	}

	for _, mention := range expectedMentions {
		if !strings.Contains(rootCmd.Long, mention) {
			t.Errorf("Long description should mention '%s'", mention)
		}
	}
}

func TestLogLevelParsing(t *testing.T) {
	tests := []struct {
		name        string
		level       string
		expectError bool
	}{
		{
			name:        "debug level",
			level:       "debug",
			expectError: false,
		},
		{
			name:        "info level",
			level:       "info",
			expectError: false,
		},
		{
			name:        "warn level",
			level:       "warn",
			expectError: false,
		},
		{
			name:        "error level",
			level:       "error",
			expectError: false,
		},
		{
			name:        "invalid level",
			level:       "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := logging.ParseLogLevel(tt.level)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
