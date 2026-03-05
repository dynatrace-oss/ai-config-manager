package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

const (
	// MaxRecursiveDepth is the maximum depth for recursive agent search
	MaxRecursiveDepth = 5
)

// DiscoverAgents discovers agent resources (single .md files) in a repository
// It searches in priority locations first, then falls back to recursive search
// if no agents are found.
//
// Priority locations:
//   - basePath/subpath/agents/
//   - basePath/subpath/.claude/agents/
//   - basePath/subpath/.opencode/agents/
//
// If no agents found in priority locations, performs recursive search (max depth 5)
// looking for .md files with valid agent frontmatter (description required).
//
// Returns deduplicated list of agents by agent name.
func DiscoverAgents(basePath string, subpath string) ([]*resource.Resource, error) {
	resources, _, err := DiscoverAgentsWithErrors(basePath, subpath)
	return resources, err
}

// DiscoverAgentsWithErrors discovers agent resources and returns both successful discoveries and errors
// It searches in priority locations first, then falls back to recursive search
// if no agents are found.
func DiscoverAgentsWithErrors(basePath string, subpath string) ([]*resource.Resource, []DiscoveryError, error) {
	if basePath == "" {
		return nil, nil, fmt.Errorf("basePath cannot be empty")
	}

	var allErrors []DiscoveryError

	// Build full search path
	searchPath := basePath
	if subpath != "" {
		searchPath = filepath.Join(basePath, subpath)
	}

	// Log discovery start
	if logger != nil {
		logger.Debug("starting agent discovery",
			"base_path", basePath,
			"subpath", subpath,
			"search_path", searchPath)
	}

	// Check if search path exists, but be lenient with subpaths
	searchPathInfo, searchPathErr := os.Stat(searchPath)

	// If searchPath doesn't exist but we have a subpath, try parent directories
	if searchPathErr != nil && subpath != "" {
		currentPath := searchPath
		for {
			parentPath := filepath.Dir(currentPath)
			if parentPath == currentPath || parentPath == basePath {
				searchPath = basePath
				break
			}
			if info, err := os.Stat(parentPath); err == nil && info.IsDir() {
				searchPath = parentPath
				break
			}
			currentPath = parentPath
		}
		// Re-check the new search path
		searchPathInfo, searchPathErr = os.Stat(searchPath)
	}

	// Check if search path exists (after trying to find a valid parent)
	if searchPathErr != nil {
		return nil, allErrors, fmt.Errorf("search path does not exist: %w", searchPathErr)
	}

	if !searchPathInfo.IsDir() {
		return nil, allErrors, fmt.Errorf("search path is not a directory: %s", searchPath)
	}

	// Priority locations to search
	priorityLocations := []string{
		filepath.Join(searchPath, "agents"),
		filepath.Join(searchPath, ".claude", "agents"),
		filepath.Join(searchPath, ".opencode", "agents"),
	}

	// Try priority locations first (with recursive search within them)
	agents := make([]*resource.Resource, 0)
	for _, location := range priorityLocations {
		if logger != nil {
			logger.Debug("searching priority location for agents",
				"location", location)
		}
		found, errors := searchAgentsInDirectory(location, 0)
		agents = append(agents, found...)
		allErrors = append(allErrors, errors...)
		if len(found) > 0 && logger != nil {
			logger.Debug("found agents in priority location",
				"location", location,
				"count", len(found))
		}
	}

	// If no agents found in priority locations, do recursive search
	if len(agents) == 0 {
		if logger != nil {
			logger.Debug("no agents found in priority locations, falling back to recursive search",
				"search_path", searchPath)
		}
		found, errors, err := discoverAgentsRecursive(searchPath, 0)
		if err != nil {
			if logger != nil {
				logger.Error("recursive search failed",
					"error", err)
			}
			return nil, allErrors, fmt.Errorf("recursive search failed: %w", err)
		}
		agents = append(agents, found...)
		allErrors = append(allErrors, errors...)
		if logger != nil {
			logger.Debug("recursive search completed",
				"agents_found", len(found))
		}
	}

	// Deduplicate by agent name (keep first occurrence)
	agents = deduplicateResources(agents)

	if logger != nil {
		logger.Debug("agent discovery completed",
			"total_agents", len(agents),
			"total_errors", len(allErrors))
	}

	return agents, allErrors, nil
}

// searchAgentsInDirectory recursively searches for agent .md files within a directory
// This is used for priority locations to find agents at any depth within that location
// Returns both successfully loaded agents and any errors encountered
func searchAgentsInDirectory(dir string, depth int) ([]*resource.Resource, []DiscoveryError) {
	config := &TraversalConfig{
		MaxDepth: MaxRecursiveDepth,
		Validator: func(path string, info os.FileInfo) bool {
			// Only process .md files
			return isMarkdownFile(path)
		},
		Loader: func(path string, base string) (*resource.Resource, error) {
			return resource.LoadAgent(path)
		},
		DirectoryFilter: nil, // No directory filtering for agents in priority locations
		BasePath:        dir,
	}

	return traverseDirectory(dir, depth, config)
}

// discoverAgentsInDirectory finds agent .md files in a specific directory
// If recursive is true, it will search subdirectories up to MaxRecursiveDepth
// Returns both successfully loaded agents and any errors encountered
func discoverAgentsInDirectory(dirPath string, recursive bool) ([]*resource.Resource, []DiscoveryError) {
	// Check if directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Directory doesn't exist, not an error
		}
		return nil, []DiscoveryError{{
			Path:  dirPath,
			Error: fmt.Errorf("failed to stat directory: %w", err),
		}}
	}

	if !info.IsDir() {
		return nil, []DiscoveryError{{
			Path:  dirPath,
			Error: fmt.Errorf("path is not a directory: %s", dirPath),
		}}
	}

	agents := make([]*resource.Resource, 0)
	var errors []DiscoveryError

	// Read directory entries
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, []DiscoveryError{{
			Path:  dirPath,
			Error: fmt.Errorf("failed to read directory: %w", err),
		}}
	}

	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())

		// Skip hidden directories and files (except .claude and .opencode)
		if strings.HasPrefix(entry.Name(), ".") && !recursive {
			continue
		}

		// Follow symlinks with os.Stat
		entryInfo, err := os.Stat(entryPath)
		if err != nil {
			continue
		}

		// If it's a .md file, try to load as agent
		if !entryInfo.IsDir() && filepath.Ext(entry.Name()) == ".md" {
			// Only parse files that are in an agents/ subtree
			if !isInAgentsSubtree(entryPath) {
				continue
			}

			agent, err := resource.LoadAgent(entryPath)
			if err != nil {
				// Collect error instead of silently skipping
				errors = append(errors, DiscoveryError{
					Path:  entryPath,
					Error: err,
				})
				continue
			}
			agents = append(agents, agent)
		}
	}

	return agents, errors
}

// discoverAgentsRecursive performs recursive search for agent files
// up to MaxRecursiveDepth
// Returns both successfully loaded agents and any errors encountered
func discoverAgentsRecursive(dirPath string, currentDepth int) ([]*resource.Resource, []DiscoveryError, error) {
	// Check depth limit
	if currentDepth > MaxRecursiveDepth {
		return nil, nil, nil
	}

	if logger != nil {
		logger.Debug("recursive agent search",
			"path", dirPath,
			"depth", currentDepth)
	}

	// Check if directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		if logger != nil {
			logger.Error("failed to stat directory during recursive search",
				"path", dirPath,
				"error", err)
		}
		return nil, nil, fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return nil, nil, nil
	}

	agents := make([]*resource.Resource, 0)
	var allErrors []DiscoveryError

	// Find agents in current directory
	found, errors := discoverAgentsInDirectory(dirPath, true)
	agents = append(agents, found...)
	allErrors = append(allErrors, errors...)

	// Recursively search subdirectories
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return agents, allErrors, nil // Return what we have so far
	}

	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())

		// Follow symlinks with os.Stat
		entryInfo, err := os.Stat(entryPath)
		if err != nil || !entryInfo.IsDir() {
			continue
		}

		// Skip common directories that typically don't contain agents
		// Also skip "commands" and "skills" directories (handled by their own discovery)
		if entry.Name() == "commands" || entry.Name() == "skills" || shouldSkipCommonDirectory(entry.Name()) {
			continue
		}

		subAgents, subErrors, err := discoverAgentsRecursive(entryPath, currentDepth+1)
		if err != nil {
			// Continue on error
			continue
		}
		agents = append(agents, subAgents...)
		allErrors = append(allErrors, subErrors...)
	}

	return agents, allErrors, nil
}

// shouldSkipDirectory returns true if the directory should be skipped during recursive search
// This is a wrapper function for backward compatibility with tests
func shouldSkipDirectory(name string) bool {
	// Skip resource-specific directories
	if name == "commands" || name == "skills" {
		return true
	}
	// Skip common non-resource directories
	return shouldSkipCommonDirectory(name)
}

// isInAgentsSubtree checks if a file path is within an agents/ directory subtree
// Returns true if the path contains /agents/ or /.claude/agents/ or /.opencode/agents/
func isInAgentsSubtree(path string) bool {
	// Normalize path separators
	normalizedPath := filepath.ToSlash(path)

	// Check for agents/ directories
	return strings.Contains(normalizedPath, "/agents/") ||
		strings.Contains(normalizedPath, "/.claude/agents/") ||
		strings.Contains(normalizedPath, "/.opencode/agents/") ||
		strings.HasSuffix(normalizedPath, "/agents") ||
		strings.HasSuffix(normalizedPath, "/.claude/agents") ||
		strings.HasSuffix(normalizedPath, "/.opencode/agents")
}

// deduplicateAgents removes duplicate agents, keeping the first occurrence
// This is a wrapper around deduplicateResources for backward compatibility with tests
func deduplicateAgents(agents []*resource.Resource) []*resource.Resource {
	return deduplicateResources(agents)
}
