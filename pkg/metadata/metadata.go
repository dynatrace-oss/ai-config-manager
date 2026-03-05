package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// ResourceMetadata tracks source information and timestamps for a resource.
// Metadata is stored in JSON format in the centralized .metadata/ directory.
//
// Example locations:
//   - Commands: ~/.local/share/ai-config/repo/.metadata/commands/mycmd-metadata.json
//   - Skills:   ~/.local/share/ai-config/repo/.metadata/skills/myskill-metadata.json
//   - Agents:   ~/.local/share/ai-config/repo/.metadata/agents/myagent-metadata.json
type ResourceMetadata struct {
	Name           string                `json:"name"`                  // Resource name
	Type           resource.ResourceType `json:"type"`                  // Resource type (command, skill, agent)
	SourceType     string                `json:"source_type"`           // Source type: "github", "local", "file"
	SourceURL      string                `json:"source_url"`            // Source URL or file path
	SourceName     string                `json:"source_name,omitempty"` // Source name from ai.repo.yaml or derived from URL/path
	SourceID       string                `json:"source_id,omitempty"`   // Source ID from ai.repo.yaml (hash-based)
	Ref            string                `json:"ref,omitempty"`         // Git ref (branch/tag/commit), defaults to "main" if empty
	FirstInstalled time.Time             `json:"first_installed"`       // When resource was first added
	LastUpdated    time.Time             `json:"last_updated"`          // When resource was last updated
}

// Save writes metadata to a JSON file in the .metadata/ directory.
// Metadata is stored in a centralized location separate from resource files.
// The sourceName parameter identifies which source the resource came from (optional, can be empty for backward compatibility).
func Save(metadata *ResourceMetadata, repoPath, sourceName string) error {
	if metadata == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	// Set source name if provided
	if sourceName != "" {
		metadata.SourceName = sourceName
	}

	metadataPath := GetMetadataPath(metadata.Name, metadata.Type, repoPath)

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(metadataPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Marshal to JSON with pretty printing
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Write to file
	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// Load reads metadata from a JSON file in the .metadata/ directory.
// Returns an error if the metadata file doesn't exist or can't be parsed.
// Handles legacy metadata files that don't have the source_name field (backward compatible).
func Load(name string, resourceType resource.ResourceType, repoPath string) (*ResourceMetadata, error) {
	metadataPath := GetMetadataPath(name, resourceType, repoPath)

	// Read file
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("metadata file not found: %s", metadataPath)
		}
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	// Unmarshal JSON
	var metadata ResourceMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

// HasSource checks if a resource has the specified source identifier.
// The sourceIdentifier is matched against SourceID first (if present), then falls back
// to matching against SourceName for backward compatibility with resources that
// don't yet have a SourceID set.
// Returns false if the metadata doesn't exist or if neither field matches.
func HasSource(name string, resourceType resource.ResourceType, sourceIdentifier, repoPath string) bool {
	metadata, err := Load(name, resourceType, repoPath)
	if err != nil {
		return false
	}
	// Check SourceID first
	if metadata.SourceID != "" && metadata.SourceID == sourceIdentifier {
		return true
	}
	// Fall back to SourceName matching
	return metadata.SourceName != "" && metadata.SourceName == sourceIdentifier
}

// DeriveSourceName derives a human-readable source name from a sourceURL.
// This is used when an explicit source name is not provided.
// Examples:
//   - "gh:owner/repo" -> "owner-repo"
//   - "https://github.com/owner/repo" -> "owner-repo"
//   - "file:///home/user/resources" -> "resources"
//   - "/home/user/resources" -> "resources"
func DeriveSourceName(sourceURL string) string {
	if sourceURL == "" {
		return "unknown"
	}

	// Handle gh: prefix
	if strings.HasPrefix(sourceURL, "gh:") {
		parts := strings.Split(strings.TrimPrefix(sourceURL, "gh:"), "/")
		if len(parts) >= 2 {
			return parts[0] + "-" + parts[1]
		}
		return strings.ReplaceAll(strings.TrimPrefix(sourceURL, "gh:"), "/", "-")
	}

	// Handle file:// URLs
	if strings.HasPrefix(sourceURL, "file://") {
		path := strings.TrimPrefix(sourceURL, "file://")
		return filepath.Base(path)
	}

	// Handle https://github.com URLs
	if strings.HasPrefix(sourceURL, "https://github.com/") {
		parts := strings.Split(strings.TrimPrefix(sourceURL, "https://github.com/"), "/")
		if len(parts) >= 2 {
			return parts[0] + "-" + parts[1]
		}
	}

	// Handle other URLs
	if strings.HasPrefix(sourceURL, "http://") || strings.HasPrefix(sourceURL, "https://") {
		// Extract domain or last path component
		parts := strings.Split(sourceURL, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Handle local file paths
	if strings.HasPrefix(sourceURL, "/") || strings.HasPrefix(sourceURL, "./") || strings.HasPrefix(sourceURL, "../") {
		return filepath.Base(sourceURL)
	}

	// Fallback: sanitize the URL by replacing slashes with hyphens
	return strings.ReplaceAll(sourceURL, "/", "-")
}

// GetMetadataPath returns the path to the metadata file for a resource.
//
// Metadata is stored in a centralized .metadata/ directory with the pattern:
//
//	<repoPath>/.metadata/<type>s/<name-with-slashes-escaped>-metadata.json
//
// For nested resources, slashes in names are replaced with hyphens to create flat filenames.
//
// Examples:
//   - flat command: ~/.local/share/ai-config/repo/.metadata/commands/mycmd-metadata.json
//   - nested command: ~/.local/share/ai-config/repo/.metadata/commands/api-deploy-metadata.json (from name "api/deploy")
//   - skill: ~/.local/share/ai-config/repo/.metadata/skills/myskill-metadata.json
//   - agent: ~/.local/share/ai-config/repo/.metadata/agents/myagent-metadata.json
func GetMetadataPath(name string, resourceType resource.ResourceType, repoPath string) string {
	// Get the resource type directory (commands, skills, agents)
	typeDir := string(resourceType) + "s"

	// Escape slashes in name for flat filename (e.g., "api/deploy" -> "api-deploy")
	escapedName := strings.ReplaceAll(name, "/", "-")

	// Build filename: <name>-metadata.json
	filename := fmt.Sprintf("%s-metadata.json", escapedName)

	return filepath.Join(repoPath, ".metadata", typeDir, filename)
}

// SavePackageMetadata writes package metadata to a JSON file in the .metadata/packages/ directory.
func SavePackageMetadata(metadata *PackageMetadata, repoPath string) error {
	if metadata == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	metadataPath := GetPackageMetadataPath(metadata.Name, repoPath)

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(metadataPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Marshal to JSON with pretty printing
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Write to file
	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// LoadPackageMetadata reads package metadata from a JSON file in the .metadata/packages/ directory.
func LoadPackageMetadata(name, repoPath string) (*PackageMetadata, error) {
	metadataPath := GetPackageMetadataPath(name, repoPath)

	// Read file
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("metadata file not found: %s", metadataPath)
		}
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	// Unmarshal JSON
	var metadata PackageMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

// CreatePackageMetadata creates a new PackageMetadata with timestamps set.
func CreatePackageMetadata(name, sourceType, sourceURL string, resourceCount int) *PackageMetadata {
	now := time.Now()
	return &PackageMetadata{
		Name:          name,
		SourceType:    sourceType,
		SourceURL:     sourceURL,
		FirstAdded:    now,
		LastUpdated:   now,
		ResourceCount: resourceCount,
	}
}

// GetPackageMetadataPath returns the path to the metadata file for a package.
// Pattern: <repoPath>/.metadata/packages/<name>-metadata.json
func GetPackageMetadataPath(name, repoPath string) string {
	filename := fmt.Sprintf("%s-metadata.json", name)
	return filepath.Join(repoPath, ".metadata", "packages", filename)
}

// PackageMetadata tracks source information and timestamps for a package.
type PackageMetadata struct {
	Name           string    `json:"name"`
	SourceType     string    `json:"source_type"`
	SourceURL      string    `json:"source_url,omitempty"`
	SourceName     string    `json:"source_name,omitempty"`
	SourceID       string    `json:"source_id,omitempty"`
	SourceRef      string    `json:"source_ref,omitempty"`
	FirstAdded     time.Time `json:"first_added"`
	LastUpdated    time.Time `json:"last_updated"`
	ResourceCount  int       `json:"resource_count"`
	OriginalFormat string    `json:"original_format,omitempty"`
}
