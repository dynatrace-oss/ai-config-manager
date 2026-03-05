package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// ResourceValidator is a callback function that validates if a file should be processed
// Returns true if the file should be processed, false otherwise
type ResourceValidator func(path string, info os.FileInfo) bool

// ResourceLoader is a callback function that loads a resource from a path
// Returns the loaded resource and any error encountered
type ResourceLoader func(path string, basePath string) (*resource.Resource, error)

// DirectoryFilter is a callback function that determines if a directory should be skipped
// Returns true if the directory should be skipped, false otherwise
type DirectoryFilter func(name string, depth int) bool

// TraversalConfig contains configuration for directory traversal
type TraversalConfig struct {
	// MaxDepth is the maximum depth for recursive traversal
	MaxDepth int
	// Validator checks if a file should be processed
	Validator ResourceValidator
	// Loader loads a resource from a file path
	Loader ResourceLoader
	// DirectoryFilter determines if a directory should be skipped
	DirectoryFilter DirectoryFilter
	// BasePath is used for calculating relative paths in resource loading
	BasePath string
}

// TraversalResult contains the results of a directory traversal
type TraversalResult struct {
	Resources []*resource.Resource
	Errors    []DiscoveryError
}

// traverseDirectory recursively traverses a directory and collects resources
// It uses callbacks to determine which files to process and how to load them
func traverseDirectory(dir string, depth int, config *TraversalConfig) ([]*resource.Resource, []DiscoveryError) {
	if depth > config.MaxDepth {
		return nil, nil
	}

	if logger != nil {
		logger.Debug("traversing directory",
			"directory", dir,
			"depth", depth)
	}

	var resources []*resource.Resource
	var errors []DiscoveryError

	// Check if directory exists
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Directory doesn't exist, not an error
		}
		if logger != nil {
			logger.Error("failed to stat directory",
				"directory", dir,
				"error", err)
		}
		return nil, []DiscoveryError{{
			Path:  dir,
			Error: fmt.Errorf("failed to stat directory: %w", err),
		}}
	}

	if !info.IsDir() {
		return nil, []DiscoveryError{{
			Path:  dir,
			Error: fmt.Errorf("path is not a directory: %s", dir),
		}}
	}

	// Read directory entries
	entries, err := os.ReadDir(dir)
	if err != nil {
		if logger != nil {
			logger.Error("failed to read directory",
				"directory", dir,
				"error", err)
		}
		errors = append(errors, DiscoveryError{
			Path:  dir,
			Error: fmt.Errorf("failed to read directory: %w", err),
		})
		return nil, errors
	}

	for _, entry := range entries {
		entryPath := filepath.Join(dir, entry.Name())

		// Follow symlinks with os.Stat
		entryInfo, err := os.Stat(entryPath)
		if err != nil {
			continue
		}

		if entryInfo.IsDir() {
			// Apply directory filter if provided
			if config.DirectoryFilter != nil && config.DirectoryFilter(entry.Name(), depth) {
				continue
			}

			// Recursively traverse subdirectories
			subResources, subErrors := traverseDirectory(entryPath, depth+1, config)
			resources = append(resources, subResources...)
			errors = append(errors, subErrors...)
			continue
		}

		// Check if file should be processed using validator
		if config.Validator != nil && !config.Validator(entryPath, entryInfo) {
			continue
		}

		// Load resource using the loader callback
		if config.Loader != nil {
			res, err := config.Loader(entryPath, config.BasePath)
			if err != nil {
				// Collect error instead of silently skipping
				if logger != nil {
					logger.Debug("failed to load resource",
						"path", entryPath,
						"error", err)
				}
				errors = append(errors, DiscoveryError{
					Path:  entryPath,
					Error: err,
				})
				continue
			}

			if logger != nil {
				logger.Debug("found resource",
					"name", res.Name,
					"path", entryPath)
			}
			resources = append(resources, res)
		}
	}

	return resources, errors
}

// deduplicateResources removes duplicate resources by name, keeping the first occurrence
func deduplicateResources(resources []*resource.Resource) []*resource.Resource {
	seen := make(map[string]bool)
	var unique []*resource.Resource

	for _, res := range resources {
		if !seen[res.Name] {
			seen[res.Name] = true
			unique = append(unique, res)
		}
	}

	return unique
}

// deduplicatePackages removes duplicate packages by name, keeping the first occurrence
func deduplicatePackages(packages []*resource.Package) []*resource.Package {
	seen := make(map[string]bool)
	var unique []*resource.Package

	for _, pkg := range packages {
		if !seen[pkg.Name] {
			seen[pkg.Name] = true
			unique = append(unique, pkg)
		}
	}

	return unique
}

// shouldSkipCommonDirectory returns true if the directory should be skipped during recursive search
// These are common directories that typically don't contain resources
func shouldSkipCommonDirectory(name string) bool {
	skipDirs := []string{
		"documentation",
		"docs",
		"node_modules",
		".git",
		".svn",
		".hg",
		"vendor",
		"build",
		"dist",
		"target",
		"bin",
		"obj",
		"__pycache__",
		".pytest_cache",
		".venv",
		"venv",
		"test",
		"tests",
		"examples",
	}

	for _, skip := range skipDirs {
		if name == skip {
			return true
		}
	}

	return false
}

// isMarkdownFile checks if a file has a .md extension
func isMarkdownFile(path string) bool {
	return filepath.Ext(path) == ".md"
}

// shouldSkipHiddenDirectory checks if a directory name starts with a dot (hidden)
func shouldSkipHiddenDirectory(name string) bool {
	return strings.HasPrefix(name, ".")
}

// isExcludedMarkdownFile checks if a markdown filename should be excluded from discovery
func isExcludedMarkdownFile(filename string) bool {
	excluded := []string{
		"SKILL.md",
		"README.md",
		"readme.md",
		"Readme.md",
		"REFERENCE.md",
		"reference.md",
		"Reference.md",
	}

	for _, excl := range excluded {
		if filename == excl {
			return true
		}
	}

	return false
}

// DirectoryResourceLoader is a callback function that loads a resource from a directory path
// Used for resources that are directory-based (like skills with SKILL.md)
type DirectoryResourceLoader func(dirPath string) (*resource.Resource, error)

// DirectoryValidator is a callback function that validates if a directory should be processed as a resource
// Returns true if the directory contains the resource, false otherwise
type DirectoryValidator func(dirPath string) bool

// DirectoryTraversalConfig contains configuration for directory-based resource traversal
type DirectoryTraversalConfig struct {
	// MaxDepth is the maximum depth for recursive traversal
	MaxDepth int
	// DirectoryValidator checks if a directory is a resource directory
	DirectoryValidator DirectoryValidator
	// DirectoryLoader loads a resource from a directory
	DirectoryLoader DirectoryResourceLoader
	// DirectoryFilter determines if a directory should be skipped during traversal
	DirectoryFilter DirectoryFilter
	// SkipResourceSubdirs if true, won't recurse into directories identified as resources
	SkipResourceSubdirs bool
}

// traverseDirectoriesForResources recursively traverses directories looking for directory-based resources
// This is used for resources like skills where the resource is defined by a directory structure
func traverseDirectoriesForResources(rootPath string, currentDepth int, config *DirectoryTraversalConfig) ([]*resource.Resource, []DiscoveryError, error) {
	var resources []*resource.Resource
	var errors []DiscoveryError

	// Stop if we've reached max depth
	if currentDepth >= config.MaxDepth {
		return resources, errors, nil
	}

	if logger != nil {
		logger.Debug("traversing directories for resources",
			"path", rootPath,
			"depth", currentDepth)
	}

	// Read directory entries
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		if logger != nil {
			logger.Error("failed to read directory during traversal",
				"path", rootPath,
				"error", err)
		}
		return nil, errors, fmt.Errorf("failed to read directory: %w", err)
	}

	// Check each entry
	for _, entry := range entries {
		entryPath := filepath.Join(rootPath, entry.Name())

		// Follow symlinks with os.Stat
		entryInfo, err := os.Stat(entryPath)
		if err != nil || !entryInfo.IsDir() {
			continue
		}

		// Apply directory filter if provided
		if config.DirectoryFilter != nil && config.DirectoryFilter(entry.Name(), currentDepth) {
			continue
		}

		// Check if this directory is a resource using the validator
		if config.DirectoryValidator != nil && config.DirectoryValidator(entryPath) {
			// Try to load the resource
			if config.DirectoryLoader != nil {
				res, err := config.DirectoryLoader(entryPath)
				if err != nil {
					// Collect error instead of silently skipping
					if logger != nil {
						logger.Debug("failed to load directory resource",
							"path", entryPath,
							"error", err)
					}
					errors = append(errors, DiscoveryError{
						Path:  entryPath,
						Error: err,
					})
				} else if res != nil && res.Name != "" && res.Description != "" {
					if logger != nil {
						logger.Debug("found directory resource",
							"name", res.Name,
							"path", entryPath)
					}
					resources = append(resources, res)
				}
			}
			// Skip recursing into resource directories if configured
			if config.SkipResourceSubdirs {
				continue
			}
		}

		// Recurse into subdirectory
		subResources, subErrs, err := traverseDirectoriesForResources(entryPath, currentDepth+1, config)
		errors = append(errors, subErrs...)
		if err == nil {
			resources = append(resources, subResources...)
		}
	}

	return resources, errors, nil
}
