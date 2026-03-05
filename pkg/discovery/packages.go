package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

const maxPackageDepth = 5

// DiscoverPackages discovers package resources (*.package.json files) in a repository
// It searches in priority locations first, then falls back to recursive search
func DiscoverPackages(basePath string, subpath string) ([]*resource.Package, error) {
	searchPath := basePath
	if subpath != "" {
		searchPath = filepath.Join(basePath, subpath)
	}

	// Log discovery start
	if logger != nil {
		logger.Debug("starting package discovery",
			"base_path", basePath,
			"subpath", subpath,
			"search_path", searchPath)
	}

	// Verify search path exists
	searchPathInfo, err := os.Stat(searchPath)
	if err != nil {
		if logger != nil {
			logger.Error("search path does not exist",
				"path", searchPath,
				"error", err)
		}
		return nil, fmt.Errorf("path does not exist: %w", err)
	}

	if !searchPathInfo.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", searchPath)
	}

	// Try priority locations first
	packages := searchPriorityPackageLocations(searchPath)
	if len(packages) > 0 {
		if logger != nil {
			logger.Debug("package discovery completed from priority locations",
				"packages_found", len(packages))
		}
		return deduplicatePackages(packages), nil
	}

	// Fall back to recursive search (max depth 5)
	if logger != nil {
		logger.Debug("no packages found in priority locations, falling back to recursive search",
			"search_path", searchPath)
	}
	packages, err = recursiveSearchPackages(searchPath, 0)
	if err != nil {
		if logger != nil {
			logger.Error("recursive search failed",
				"error", err)
		}
		return nil, fmt.Errorf("failed to search for packages: %w", err)
	}

	dedupedPackages := deduplicatePackages(packages)
	if logger != nil {
		logger.Debug("package discovery completed",
			"total_packages", len(dedupedPackages))
	}

	return dedupedPackages, nil
}

// searchPriorityPackageLocations searches standard package directories
func searchPriorityPackageLocations(basePath string) []*resource.Package {
	priorityDirs := []string{
		filepath.Join(basePath, "packages"),
		filepath.Join(basePath, ".claude", "packages"),
		filepath.Join(basePath, ".opencode", "packages"),
	}

	var allPackages []*resource.Package

	for _, dir := range priorityDirs {
		if _, err := os.Stat(dir); err != nil {
			continue // Directory doesn't exist, skip
		}

		if logger != nil {
			logger.Debug("searching priority location for packages",
				"location", dir)
		}

		packages, err := searchPackagesDirectory(dir)
		if err != nil {
			// Continue searching other directories even if one fails
			if logger != nil {
				logger.Debug("failed to search directory",
					"directory", dir,
					"error", err)
			}
			continue
		}

		if len(packages) > 0 && logger != nil {
			logger.Debug("found packages in priority location",
				"location", dir,
				"count", len(packages))
		}

		allPackages = append(allPackages, packages...)
	}

	return allPackages
}

// recursiveSearchPackages performs a recursive search for package files
func recursiveSearchPackages(basePath string, depth int) ([]*resource.Package, error) {
	if depth > maxPackageDepth {
		return nil, nil
	}

	if logger != nil {
		logger.Debug("recursive package search",
			"path", basePath,
			"depth", depth)
	}

	var allPackages []*resource.Package

	// Search current directory for *.package.json files
	packages, err := searchPackagesDirectory(basePath)
	if err == nil {
		allPackages = append(allPackages, packages...)
	}

	// Recursively search subdirectories
	entries, err := os.ReadDir(basePath)
	if err != nil {
		if logger != nil {
			logger.Error("failed to read directory during recursive search",
				"path", basePath,
				"error", err)
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(basePath, entry.Name())

		// Follow symlinks with os.Stat
		entryInfo, err := os.Stat(entryPath)
		if err != nil || !entryInfo.IsDir() {
			continue
		}

		// Skip hidden directories (except .claude and .opencode which are handled in priority)
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		// Skip other resource type directories
		if entry.Name() == "agents" || entry.Name() == "skills" || entry.Name() == "commands" {
			continue
		}

		subPackages, err := recursiveSearchPackages(entryPath, depth+1)
		if err == nil {
			allPackages = append(allPackages, subPackages...)
		}
	}

	return allPackages, nil
}

// searchPackagesDirectory searches a directory for package files
func searchPackagesDirectory(dir string) ([]*resource.Package, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if logger != nil {
			logger.Error("failed to read directory",
				"directory", dir,
				"error", err)
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var packages []*resource.Package

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check if it's a .package.json file
		if !strings.HasSuffix(entry.Name(), ".package.json") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())

		// Try to load as package
		pkg, err := resource.LoadPackage(filePath)
		if err != nil {
			// Invalid package file, skip it
			if logger != nil {
				logger.Debug("failed to load package",
					"path", filePath,
					"error", err)
			}
			continue
		}

		if logger != nil {
			logger.Debug("found package",
				"name", pkg.Name,
				"path", filePath)
		}
		packages = append(packages, pkg)
	}

	return packages, nil
}
