package resource

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Package represents a collection of resources that can be installed together.
// Packages are stored as JSON files in the repo/packages/ directory.
type Package struct {
	Name        string   `json:"name"`        // Package name (must match filename without .package.json)
	Description string   `json:"description"` // Human-readable description
	Resources   []string `json:"resources"`   // Array of resource references in "type/name" format
}

// PackageMetadata tracks source information and timestamps for a package.
// Metadata is stored in JSON format in the .metadata/packages/ directory.
type PackageMetadata struct {
	Name           string    `json:"name"`                      // Package name
	SourceType     string    `json:"source_type"`               // Source type: "github", "local", "manual", "claude-plugin"
	SourceURL      string    `json:"source_url,omitempty"`      // Source URL (null for manual)
	SourceRef      string    `json:"source_ref,omitempty"`      // Git ref/branch
	FirstAdded     time.Time `json:"first_added"`               // When package was first added
	LastUpdated    time.Time `json:"last_updated"`              // When package was last updated
	ResourceCount  int       `json:"resource_count"`            // Number of resources in package
	OriginalFormat string    `json:"original_format,omitempty"` // Original format if converted (e.g., "claude-plugin")
}

// ParseResourceReference parses a resource reference in "type/name" format.
// Returns the resource type, name, and error if invalid.
//
// Valid formats:
//   - "command/name"
//   - "skill/name"
//   - "agent/name"
//
// Examples:
//
//	ParseResourceReference("command/test") // => Command, "test", nil
//	ParseResourceReference("skill/pdf")    // => Skill, "pdf", nil
//	ParseResourceReference("invalid")      // => "", "", error
func ParseResourceReference(ref string) (ResourceType, string, error) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid resource format: %q (expected type/name)", ref)
	}

	typeStr, name := parts[0], parts[1]

	var resourceType ResourceType
	switch typeStr {
	case "command":
		resourceType = Command
	case "skill":
		resourceType = Skill
	case "agent":
		resourceType = Agent
	default:
		return "", "", fmt.Errorf("invalid resource type: %q (expected command/skill/agent)", typeStr)
	}

	if name == "" {
		return "", "", fmt.Errorf("resource name cannot be empty in: %q", ref)
	}

	return resourceType, name, nil
}

// LoadPackage loads a package from a .package.json file.
// Returns error if file doesn't exist, is invalid JSON, is missing required fields,
// or contains invalid resource references.
func LoadPackage(filePath string) (*Package, error) {
	return loadPackage(filePath, true)
}

// LoadPackageLenient loads a package from a .package.json file while skipping
// resource-reference validation. This is intended for repository inspection
// flows that still need to surface broken package references to users.
func LoadPackageLenient(filePath string) (*Package, error) {
	return loadPackage(filePath, false)
}

func loadPackage(filePath string, validateRefs bool) (*Package, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package file: %w", err)
	}

	var pkg Package
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse package JSON: %w", err)
	}

	// Validate required fields
	if pkg.Name == "" {
		return nil, fmt.Errorf("package name is required")
	}
	if pkg.Description == "" {
		return nil, fmt.Errorf("package description is required")
	}

	// Validate package name follows agentskills.io rules
	if err := ValidateName(pkg.Name); err != nil {
		return nil, fmt.Errorf("invalid package name: %w", err)
	}

	if validateRefs {
		for i, ref := range pkg.Resources {
			if _, _, err := ParseResourceReference(ref); err != nil {
				return nil, fmt.Errorf("invalid package resource reference at index %d: %w", i, err)
			}
		}
	}

	return &pkg, nil
}

// SavePackage saves a package to a .package.json file in the repo/packages/ directory.
// Creates the packages/ directory if it doesn't exist.
func SavePackage(pkg *Package, repoPath string) error {
	if pkg == nil {
		return fmt.Errorf("package cannot be nil")
	}

	// Validate package
	if pkg.Name == "" {
		return fmt.Errorf("package name is required")
	}
	if pkg.Description == "" {
		return fmt.Errorf("package description is required")
	}

	// Create packages directory if needed
	packagesDir := filepath.Join(repoPath, "packages")
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		return fmt.Errorf("failed to create packages directory: %w", err)
	}

	// Build file path
	filename := fmt.Sprintf("%s.package.json", pkg.Name)
	filePath := filepath.Join(packagesDir, filename)

	// Marshal to JSON with pretty printing
	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal package: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write package file: %w", err)
	}

	return nil
}

// GetPackagePath returns the path to a package file in the repo.
func GetPackagePath(packageName, repoPath string) string {
	return filepath.Join(repoPath, "packages", fmt.Sprintf("%s.package.json", packageName))
}
