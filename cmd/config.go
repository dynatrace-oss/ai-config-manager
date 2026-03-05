package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and manage aimgr configuration",
	Long: `View and manage aimgr configuration settings.

Configuration is stored in ~/.config/aimgr/aimgr.yaml and controls default
behavior for aimgr commands.`,
}

// configGetCmd represents the config get command
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Get a configuration value with precedence handling.

Available keys:
  install.targets - The default AI tools to install to

Precedence (highest to lowest):
  1. ai.package.yaml install.targets (current directory)
  2. ~/.config/aimgr/aimgr.yaml install.targets (global config)

The source of the value is displayed in the output.

Example:
  aimgr config get install.targets`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeConfigKeys,
	RunE:              configGet,
}

// configSetCmd represents the config set command
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value.

Available keys:
  install.targets - The default AI tools to install to (comma-separated)

Valid tools:
  claude   - Claude Code (supports commands, skills, and agents)
  opencode - OpenCode (supports commands, skills, and agents)
  copilot  - GitHub Copilot (supports skills only)

Examples:
  aimgr config set install.targets claude
  aimgr config set install.targets claude,opencode
  aimgr config set install.targets claude,opencode,copilot`,
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: completeConfigSetArgs,
	RunE:              configSet,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}

func configGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	// Only support install.targets now
	if key != "install.targets" {
		return fmt.Errorf("unknown config key: %s (available: install.targets)", key)
	}

	// Check for ai.package.yaml in current directory (highest precedence)
	manifestPath := filepath.Join(".", manifest.ManifestFileName)
	var targets []string
	var source string

	if manifest.Exists(manifestPath) {
		m, err := manifest.Load(manifestPath)
		if err != nil {
			return fmt.Errorf("loading manifest: %w", err)
		}

		if len(m.Install.Targets) > 0 {
			targets = m.Install.Targets
			absManifestPath, _ := filepath.Abs(manifestPath)
			source = absManifestPath
		}
	}

	// Fall back to global config if no manifest or manifest has no targets
	if source == "" {
		cfg, err := config.LoadGlobal()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		targets = cfg.Install.Targets
		source, _ = config.GetConfigPath()
	}

	// Display source
	fmt.Fprintf(os.Stderr, "Source: %s\n", source)

	// Print value
	if len(targets) == 0 {
		fmt.Printf("install.targets: []\n")
	} else if len(targets) == 1 {
		fmt.Printf("install.targets: %s\n", targets[0])
	} else {
		fmt.Printf("install.targets:\n")
		for _, target := range targets {
			fmt.Printf("  - %s\n", target)
		}
	}
	return nil
}

func configSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	if key != "install.targets" {
		return fmt.Errorf("unknown config key: %s (available: install.targets)", key)
	}

	// Parse comma-separated values
	var targets []string
	if value != "" {
		for _, target := range splitAndTrim(value) {
			// Validate tool name
			if _, err := tools.ParseTool(target); err != nil {
				return fmt.Errorf("invalid tool '%s': %w\nValid tools: claude, opencode, copilot", target, err)
			}
			targets = append(targets, target)
		}
	}

	// Load existing config (or create new)
	cfg, err := config.LoadGlobal()
	if err != nil {
		// If config doesn't exist or has issues, create a new one
		cfg = &config.Config{}
	}

	// Update value
	cfg.Install.Targets = targets

	// Get config path
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("getting config path: %w", err)
	}

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Save config
	if err := saveConfig(configPath, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Using config file: %s\n", configPath)
	if len(targets) == 1 {
		fmt.Printf("✓ Set install.targets to '%s'\n", targets[0])
	} else {
		fmt.Printf("✓ Set install.targets to [%s]\n", value)
	}
	return nil
}

func saveConfig(path string, cfg *config.Config) error {
	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// splitAndTrim splits a comma-separated string and trims whitespace from each part
func splitAndTrim(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := []string{}
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}
