package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/adrg/xdg"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	// DefaultConfigFileName is the name of the config file (without leading dot for XDG)
	DefaultConfigFileName = "aimgr.yaml"
	// OldConfigFileName is the legacy config file name in home directory (ai-repo for migration)
	OldConfigFileName = ".ai-repo.yaml"
)

// envVarPattern matches Docker Compose-style environment variable syntax.
// Matches ${VAR} or ${VAR:-default} with whitelisted variable names.
// Variable names must start with a letter or underscore and contain only
// alphanumeric characters or underscores.
var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)

// expandEnvVars expands environment variables in a string using Docker Compose-style syntax.
//
// Supported syntax:
//   - ${VAR}          - Expands to the value of VAR, or empty string if unset
//   - ${VAR:-default} - Expands to the value of VAR, or "default" if unset/empty
//
// Variable names must start with a letter or underscore and contain only
// alphanumeric characters or underscores (matching pattern [A-Za-z_][A-Za-z0-9_]*).
//
// Examples:
//
//	expandEnvVars("${HOME}/config")           → "/home/user/config"
//	expandEnvVars("${UNSET_VAR}")             → ""
//	expandEnvVars("${UNSET_VAR:-fallback}")   → "fallback"
//	expandEnvVars("${HOME:-/default}/config") → "/home/user/config"
//
// If a variable reference cannot be parsed (malformed syntax), the original
// text is preserved for safety.
func expandEnvVars(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract submatches: [full_match, var_name, optional_default_with_:-, default_value]
		submatches := envVarPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			// Should never happen with valid regex, but be safe
			return match
		}

		varName := submatches[1]
		value := os.Getenv(varName)

		// If variable is unset or empty, use default if provided
		if value == "" && len(submatches) >= 4 {
			// submatches[3] contains the default value (text after :-)
			return submatches[3]
		}

		return value
	})
}

// FieldMappings maps logical values to tool-specific values
// Structure: field_name -> logical_value -> tool_name -> actual_value
// Special key "null" maps missing/empty values
type FieldMappings map[string]map[string]map[string]string

// TypeMappings holds field mappings for each resource type
type TypeMappings struct {
	Skill   FieldMappings `yaml:"skill,omitempty"`
	Agent   FieldMappings `yaml:"agent,omitempty"`
	Command FieldMappings `yaml:"command,omitempty"`
}

// Config represents the application configuration
type Config struct {
	// Install configuration for default installation targets
	Install InstallConfig `yaml:"install"`

	// Repo configuration for repository settings
	Repo RepoConfig `yaml:"repo"`

	// Mappings holds tool-specific field transformations for each resource type
	Mappings TypeMappings `yaml:"mappings,omitempty"`
}

// InstallConfig holds installation-related configuration
type InstallConfig struct {
	// Targets specifies which AI tools to install to by default
	// Valid values: claude, opencode, copilot
	Targets []string `yaml:"targets"`
}

// RepoConfig holds repository-related configuration
type RepoConfig struct {
	// Path is an optional custom repository path
	Path string `yaml:"path,omitempty"`
}

// GetConfigPath returns the path to the config file in XDG config directory
// Returns ~/.config/aimgr/aimgr.yaml
func GetConfigPath() (string, error) {
	configDir := filepath.Join(xdg.ConfigHome, "aimgr")
	return filepath.Join(configDir, DefaultConfigFileName), nil
}

// getOldConfigPath returns the path to the legacy config file in home directory
// Returns ~/.ai-repo.yaml
func getOldConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, OldConfigFileName), nil
}

// Load loads the configuration from file in the specified directory
// If no config file exists, returns default configuration
// For backward compatibility, checks old location (~/.ai-repo.yaml) and migrates if found
func Load(projectPath string) (*Config, error) {
	configPath := filepath.Join(projectPath, DefaultConfigFileName)

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// No config file - return error
		return nil, fmt.Errorf("no config file found at: %s\n\n"+
			"Expected format:\n"+
			"  install:\n"+
			"    targets:\n"+
			"      - claude\n"+
			"      - opencode", configPath)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Expand environment variables
	expanded := expandEnvVars(string(data))

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Require install.targets
	if len(config.Install.Targets) == 0 {
		return nil, fmt.Errorf("install.targets is required\n\n"+
			"Expected format:\n"+
			"  install:\n"+
			"    targets:\n"+
			"      - claude\n"+
			"      - opencode\n\n"+
			"Config location: %s", configPath)
	}

	// Validate
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// LoadGlobal loads the global configuration from XDG config directory
// Checks ~/.config/aimgr/aimgr.yaml and falls back to ~/.ai-repo.yaml
// Automatically migrates from old location if found
// If no config exists, returns a default config with "claude" as the default target
func LoadGlobal() (*Config, error) {
	var configPath string
	var err error

	// Priority 1: Check if Viper has a config file set (via --config flag)
	viperConfigFile := viper.ConfigFileUsed()
	if viperConfigFile != "" {
		configPath = viperConfigFile
	} else {
		// Priority 2: Use default config path
		configPath, err = GetConfigPath()
		if err != nil {
			return nil, fmt.Errorf("getting config path: %w", err)
		}
	}

	// Check if new config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Check for old config location
		oldPath, err := getOldConfigPath()
		if err != nil {
			return nil, fmt.Errorf("getting old config path: %w", err)
		}

		if _, err := os.Stat(oldPath); err == nil {
			// Old config exists - migrate it
			if err := migrateConfig(oldPath, configPath); err != nil {
				return nil, fmt.Errorf("migrating config: %w", err)
			}
			fmt.Fprintf(os.Stderr, "⚠️  Migrated config from %s to %s\n", oldPath, configPath)
			fmt.Fprintf(os.Stderr, "   Old config file left intact for safety\n")
		} else {
			// No config exists - return default config
			defaultConfig := &Config{
				Install: InstallConfig{
					Targets: []string{"claude"},
				},
				Repo: RepoConfig{
					Path: "",
				},
			}
			return defaultConfig, nil
		}
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Expand environment variables
	expanded := expandEnvVars(string(data))

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal([]byte(expanded), &config); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Provide default if install.targets is empty
	if len(config.Install.Targets) == 0 {
		config.Install.Targets = []string{"claude"}
	}

	// Validate
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// migrateConfig copies config from old location to new XDG location
func migrateConfig(oldPath, newPath string) error {
	// Read old config
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return fmt.Errorf("reading old config: %w", err)
	}

	// Ensure new config directory exists
	newDir := filepath.Dir(newPath)
	if err := os.MkdirAll(newDir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Write to new location
	if err := os.WriteFile(newPath, data, 0644); err != nil {
		return fmt.Errorf("writing new config: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate install targets
	for _, target := range c.Install.Targets {
		if _, err := tools.ParseTool(target); err != nil {
			return fmt.Errorf("install.targets: invalid tool '%s': %w", target, err)
		}
	}

	// Validate repo path if provided
	if c.Repo.Path != "" {
		// Expand ~ to home directory
		if strings.HasPrefix(c.Repo.Path, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("repo.path: cannot expand ~: %w", err)
			}
			c.Repo.Path = filepath.Join(home, c.Repo.Path[2:])
		}

		// Convert to absolute path if relative
		if !filepath.IsAbs(c.Repo.Path) {
			absPath, err := filepath.Abs(c.Repo.Path)
			if err != nil {
				return fmt.Errorf("repo.path: cannot convert to absolute path: %w", err)
			}
			c.Repo.Path = absPath
		}

		// Clean the path
		c.Repo.Path = filepath.Clean(c.Repo.Path)
	}

	// Validate mappings - warn about unknown tool names but don't error
	if c.Mappings.HasAny() {
		c.validateMappingsToolNames()
	}

	return nil
}

// validateMappingsToolNames checks if tool names in mappings are known
// and logs warnings for unknown tools (allows future tools)
func (c *Config) validateMappingsToolNames() {
	unknownTools := make(map[string]struct{})

	// Check each field mappings for unknown tools
	checkFieldMappings := func(fm FieldMappings) {
		for _, valueMappings := range fm {
			for _, toolMappings := range valueMappings {
				for toolName := range toolMappings {
					if _, err := tools.ParseTool(toolName); err != nil {
						unknownTools[toolName] = struct{}{}
					}
				}
			}
		}
	}

	checkFieldMappings(c.Mappings.Skill)
	checkFieldMappings(c.Mappings.Agent)
	checkFieldMappings(c.Mappings.Command)

	// Log warnings for unknown tools
	for toolName := range unknownTools {
		fmt.Fprintf(os.Stderr, "⚠️  Warning: mappings contains unknown tool '%s' (known: claude, opencode, copilot, windsurf)\n", toolName)
	}
}

// GetDefaultTargets returns the configured default installation targets
func (c *Config) GetDefaultTargets() ([]tools.Tool, error) {
	targetStrs := c.Install.Targets
	if len(targetStrs) == 0 {
		return nil, fmt.Errorf("install.targets is not configured")
	}

	targets := make([]tools.Tool, 0, len(targetStrs))
	for _, targetStr := range targetStrs {
		tool, err := tools.ParseTool(targetStr)
		if err != nil {
			return nil, fmt.Errorf("invalid target '%s': %w", targetStr, err)
		}
		targets = append(targets, tool)
	}

	return targets, nil
}

// GetMapping returns the tool-specific value for a field
// Returns (mappedValue, true) if mapping exists
// Returns ("", false) if no mapping exists
func (tm *TypeMappings) GetMapping(resourceType resource.ResourceType, fieldName string, logicalValue string, toolName string) (string, bool) {
	var fieldMappings FieldMappings
	switch resourceType {
	case resource.Skill:
		fieldMappings = tm.Skill
	case resource.Agent:
		fieldMappings = tm.Agent
	case resource.Command:
		fieldMappings = tm.Command
	default:
		return "", false
	}

	if fieldMappings == nil {
		return "", false
	}

	valueMappings, ok := fieldMappings[fieldName]
	if !ok {
		return "", false
	}

	toolMappings, ok := valueMappings[logicalValue]
	if !ok {
		return "", false
	}

	actualValue, ok := toolMappings[toolName]
	if !ok {
		return "", false
	}

	return actualValue, true
}

// GetMappingWithNull handles null mapping for missing values
// If logicalValue is empty, checks for "null" key mapping
func (tm *TypeMappings) GetMappingWithNull(resourceType resource.ResourceType, fieldName string, logicalValue string, toolName string) (string, bool) {
	// First try the actual value
	if logicalValue != "" {
		if val, ok := tm.GetMapping(resourceType, fieldName, logicalValue, toolName); ok {
			return val, true
		}
	}

	// If empty or not found, try null mapping
	if logicalValue == "" {
		return tm.GetMapping(resourceType, fieldName, "null", toolName)
	}

	return "", false
}

// GetToolsWithMappings returns list of tools that have any mappings defined
func (tm *TypeMappings) GetToolsWithMappings() []string {
	toolSet := make(map[string]struct{})

	// Collect tools from all field mappings
	collectTools := func(fm FieldMappings) {
		for _, valueMappings := range fm {
			for _, toolMappings := range valueMappings {
				for toolName := range toolMappings {
					toolSet[toolName] = struct{}{}
				}
			}
		}
	}

	collectTools(tm.Skill)
	collectTools(tm.Agent)
	collectTools(tm.Command)

	// Convert set to sorted slice
	result := make([]string, 0, len(toolSet))
	for toolName := range toolSet {
		result = append(result, toolName)
	}
	sort.Strings(result)

	return result
}

// HasAny returns true if any mappings are defined
func (tm *TypeMappings) HasAny() bool {
	return len(tm.Skill) > 0 || len(tm.Agent) > 0 || len(tm.Command) > 0
}
