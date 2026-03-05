package marketplace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// PackageInfo contains a generated package and its source directory
type PackageInfo struct {
	Package    *resource.Package
	SourcePath string // Absolute path to plugin source directory
}

// GeneratePackages generates aimgr packages from marketplace plugin entries.
// It discovers resources in each plugin's source directory and creates a package
// for each plugin with resource references in type/name format.
//
// basePath is the directory containing the marketplace.json file (used to resolve
// relative plugin source paths).
//
// Returns an array of generated package info. Plugins with no resources are skipped.
func GeneratePackages(marketplace *MarketplaceConfig, basePath string) ([]*PackageInfo, error) {
	if marketplace == nil {
		return nil, fmt.Errorf("marketplace config cannot be nil")
	}

	if basePath == "" {
		return nil, fmt.Errorf("basePath cannot be empty")
	}

	// Verify basePath exists
	if info, err := os.Stat(basePath); err != nil {
		return nil, fmt.Errorf("basePath does not exist: %w", err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("basePath is not a directory: %s", basePath)
	}

	var packages []*PackageInfo

	for i, plugin := range marketplace.Plugins {
		// Resolve plugin source path (relative to basePath)
		sourcePath := plugin.Source
		if !filepath.IsAbs(sourcePath) {
			sourcePath = filepath.Join(basePath, plugin.Source)
		}

		// Check if source directory exists
		if info, err := os.Stat(sourcePath); err != nil {
			// Skip plugins with missing source directories (could be placeholders)
			continue
		} else if !info.IsDir() {
			return nil, fmt.Errorf("plugin %d (%s): source is not a directory: %s", i, plugin.Name, sourcePath)
		}

		// Sanitize plugin name for aimgr naming rules
		packageName := sanitizeName(plugin.Name)
		if packageName == "" {
			return nil, fmt.Errorf("plugin %d: name %q cannot be sanitized to valid aimgr name", i, plugin.Name)
		}

		// Validate the sanitized name
		if err := resource.ValidateName(packageName); err != nil {
			return nil, fmt.Errorf("plugin %d (%s): sanitized name %q is invalid: %w", i, plugin.Name, packageName, err)
		}

		// Discover resources in plugin source directory
		resources := discoverResources(sourcePath)

		// Skip plugins with no resources
		if len(resources) == 0 {
			continue
		}

		// Create package with resource references
		pkg := &resource.Package{
			Name:        packageName,
			Description: plugin.Description,
			Resources:   buildResourceReferences(resources),
		}

		// Store package info with source path
		pkgInfo := &PackageInfo{
			Package:    pkg,
			SourcePath: sourcePath,
		}

		packages = append(packages, pkgInfo)
	}

	return packages, nil
}

// sanitizeName converts a plugin name to a valid aimgr package name.
// Rules:
// - Convert to lowercase
// - Replace spaces and underscores with hyphens
// - Remove invalid characters (keep only alphanumeric and hyphens)
// - Remove consecutive hyphens
// - Trim leading/trailing hyphens
// - Truncate to 64 characters max
//
// Returns empty string if the result would be invalid.
func sanitizeName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace spaces and underscores with hyphens
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Remove invalid characters (keep only alphanumeric and hyphens)
	validChars := regexp.MustCompile(`[^a-z0-9-]`)
	name = validChars.ReplaceAllString(name, "")

	// Remove consecutive hyphens
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	// Trim leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Truncate to 64 characters
	if len(name) > 64 {
		name = name[:64]
		// Trim trailing hyphen if truncation created one
		name = strings.TrimRight(name, "-")
	}

	return name
}

// discoverResources discovers all resources (commands, skills, agents) in a directory.
// It searches for resources in standard locations and falls back to recursive search.
func discoverResources(sourcePath string) []*resource.Resource {
	var allResources []*resource.Resource

	// Discover commands
	commands, err := discovery.DiscoverCommands(sourcePath, "")
	if err != nil {
		// Don't fail if discovery fails, just skip this type
		// (the directory might not have commands)
	} else {
		allResources = append(allResources, commands...)
	}

	// Discover skills
	skills, err := discovery.DiscoverSkills(sourcePath, "")
	if err != nil {
		// Don't fail if discovery fails, just skip this type
	} else {
		allResources = append(allResources, skills...)
	}

	// Discover agents
	agents, err := discovery.DiscoverAgents(sourcePath, "")
	if err != nil {
		// Don't fail if discovery fails, just skip this type
	} else {
		allResources = append(allResources, agents...)
	}

	return allResources
}

// buildResourceReferences converts resources to type/name reference format.
// Returns a slice of strings in "type/name" format (e.g., "command/test", "skill/pdf").
func buildResourceReferences(resources []*resource.Resource) []string {
	refs := make([]string, 0, len(resources))

	for _, res := range resources {
		ref := fmt.Sprintf("%s/%s", res.Type, res.Name)
		refs = append(refs, ref)
	}

	return refs
}
