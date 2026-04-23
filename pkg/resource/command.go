package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CommandResource represents a command resource
type CommandResource struct {
	Resource
	Agent        string   `yaml:"agent,omitempty"`
	Model        string   `yaml:"model,omitempty"`
	AllowedTools []string `yaml:"allowed-tools,omitempty"`
	Content      string   `yaml:"-"` // The markdown content
}

// autoDetectCommandsBase finds the commands/ directory containing the given file
// Returns empty string if no commands/ directory is found in the path
func autoDetectCommandsBase(filePath string) string {
	// Clean and normalize path
	cleanPath := filepath.Clean(filePath)

	// Walk up the path looking for a "commands" directory
	dir := filepath.Dir(cleanPath)
	for {
		// Check if current directory is named "commands"
		if filepath.Base(dir) == "commands" {
			return dir
		}

		// Check if parent is "commands" (we're one level nested)
		parent := filepath.Dir(dir)
		if filepath.Base(parent) == "commands" {
			return parent
		}

		// Stop at filesystem root or after reasonable depth
		if dir == "." || dir == "/" || strings.Count(dir, string(filepath.Separator)) < 2 {
			break
		}

		dir = parent
	}

	return "" // Not found
}

// LoadCommand loads a command resource from a markdown file
// Automatically detects the commands/ base directory and preserves nested structure
// Returns error if file is not in a commands/ directory
func LoadCommand(filePath string) (*Resource, error) {
	basePath := autoDetectCommandsBase(filePath)

	if basePath == "" {
		return nil, fmt.Errorf("command file must be in a 'commands/' directory (e.g., 'commands/', '.claude/commands/', '.opencode/commands/'): %s", filePath)
	}

	return LoadCommandWithBase(filePath, basePath)
}

// LoadCommandWithBase loads a command resource and calculates nested name if basePath is provided.
//
// This is the canonical explicit/testable loader path for callers that already
// know the commands base directory. Use LoadCommand as a convenience wrapper
// when auto-detecting the nearest commands/ directory is desired.
//
// basePath should be the directory to calculate relative paths from (e.g., "commands/")
// If basePath is provided and file is nested, Name will contain the full nested path (e.g., "api/deploy")
// If basePath is empty, Name will be just the basename (backward compatibility)
func LoadCommandWithBase(filePath string, basePath string) (*Resource, error) {
	// Validate it's a .md file
	if filepath.Ext(filePath) != ".md" {
		return nil, WrapLoadError(filePath, Command, fmt.Errorf("command must be a .md file"))
	}

	// Check file exists
	if _, err := os.Stat(filePath); err != nil {
		return nil, WrapLoadError(filePath, Command, fmt.Errorf("file does not exist: %w", err))
	}

	// Extract basename from filename (without .md extension)
	name := strings.TrimSuffix(filepath.Base(filePath), ".md")

	// Parse frontmatter
	frontmatter, _, err := ParseFrontmatter(filePath)
	if err != nil {
		return nil, NewValidationError(filePath, "command", name, "frontmatter", err)
	}

	// Calculate relative path if basePath is provided, and use it as Name for nested commands
	if basePath != "" {
		// Clean paths for consistent comparison
		cleanFilePath := filepath.Clean(filePath)
		cleanBasePath := filepath.Clean(basePath)

		// Get relative path from basePath to filePath
		relPath, err := filepath.Rel(cleanBasePath, cleanFilePath)
		if err == nil && !strings.HasPrefix(relPath, "..") {
			// Remove the .md extension from relative path
			relativePath := strings.TrimSuffix(relPath, ".md")
			// Use relative path as name for nested commands
			if relativePath != "" && relativePath != name {
				name = relativePath
			}
		}
	}

	// Build resource
	resource := &Resource{
		Name:        name, // Now contains full nested path (e.g., "opencode-coder/doctor") or basename
		Type:        Command,
		Description: frontmatter.GetString("description"),
		Version:     frontmatter.GetString("version"),
		Author:      frontmatter.GetString("author"),
		License:     frontmatter.GetString("license"),
		Path:        filePath,
		Metadata:    frontmatter.GetMap("metadata"),
	}

	// Validate
	if err := resource.Validate(); err != nil {
		return nil, NewValidationError(filePath, "command", name, "", err)
	}

	return resource, nil
}

// LoadCommandResource loads a command resource with full details including content
// For flat commands without nested structure
func LoadCommandResource(filePath string) (*CommandResource, error) {
	return LoadCommandResourceWithBase(filePath, "")
}

// LoadCommandResourceWithBase loads a command resource with full details and calculates nested name.
//
// This is the canonical explicit/testable loader path for callers that already
// know the commands base directory. Use LoadCommandResource as a convenience
// wrapper when no explicit base is needed.
func LoadCommandResourceWithBase(filePath string, basePath string) (*CommandResource, error) {
	// Validate it's a .md file
	if filepath.Ext(filePath) != ".md" {
		return nil, fmt.Errorf("command must be a .md file")
	}

	// Check file exists
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("file does not exist: %w", err)
	}

	// Parse frontmatter and content
	frontmatter, content, err := ParseFrontmatter(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Extract basename from filename (without .md extension)
	name := strings.TrimSuffix(filepath.Base(filePath), ".md")

	// Calculate relative path if basePath is provided, and use it as Name for nested commands
	if basePath != "" {
		cleanFilePath := filepath.Clean(filePath)
		cleanBasePath := filepath.Clean(basePath)

		relPath, err := filepath.Rel(cleanBasePath, cleanFilePath)
		if err == nil && !strings.HasPrefix(relPath, "..") {
			relativePath := strings.TrimSuffix(relPath, ".md")
			// Use relative path as name for nested commands
			if relativePath != "" && relativePath != name {
				name = relativePath
			}
		}
	}

	// Build command resource
	cmd := &CommandResource{
		Resource: Resource{
			Name:        name, // Now contains full nested path (e.g., "opencode-coder/doctor") or basename
			Type:        Command,
			Description: frontmatter.GetString("description"),
			Version:     frontmatter.GetString("version"),
			Author:      frontmatter.GetString("author"),
			License:     frontmatter.GetString("license"),
			Path:        filePath,
			Metadata:    frontmatter.GetMap("metadata"),
		},
		Agent:        frontmatter.GetString("agent"),
		Model:        frontmatter.GetString("model"),
		AllowedTools: frontmatter.GetStringSlice("allowed-tools"),
		Content:      content,
	}

	// Validate
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return cmd, nil
}

// ValidateCommand validates a command resource structure
func ValidateCommand(filePath string) error {
	_, err := LoadCommand(filePath)
	return err
}

// NewCommandResource creates a new command resource
func NewCommandResource(name, description string) *CommandResource {
	return &CommandResource{
		Resource: Resource{
			Name:        name,
			Type:        Command,
			Description: description,
			Metadata:    make(map[string]string),
		},
	}
}

// WriteCommand writes a command resource to a file
func WriteCommand(cmd *CommandResource, filePath string) error { //nolint:dupl // command and agent Write share structure but have different type-specific fields
	// Build frontmatter
	frontmatter := Frontmatter{
		"description": cmd.Description,
	}

	if cmd.Agent != "" {
		frontmatter["agent"] = cmd.Agent
	}
	if cmd.Model != "" {
		frontmatter["model"] = cmd.Model
	}
	if len(cmd.AllowedTools) > 0 {
		frontmatter["allowed-tools"] = cmd.AllowedTools
	}
	if cmd.Version != "" {
		frontmatter["version"] = cmd.Version
	}
	if cmd.Author != "" {
		frontmatter["author"] = cmd.Author
	}
	if cmd.License != "" {
		frontmatter["license"] = cmd.License
	}
	if len(cmd.Metadata) > 0 {
		frontmatter["metadata"] = cmd.Metadata
	}

	return WriteFrontmatter(filePath, frontmatter, cmd.Content)
}
