package marketplace

import (
	"os"
	"path/filepath"
)

const marketplaceFileName = "marketplace.json"

// DiscoverMarketplace searches for marketplace.json files in common locations
// and returns the parsed marketplace configuration along with the file path.
//
// It searches in the following locations (relative to basePath/subpath):
// 1. .claude-plugin/marketplace.json
// 2. .opencode-plugin/marketplace.json
// 3. marketplace.json (root)
// 4. .opencode/marketplace.json (legacy fallback)
//
// Returns:
// - *MarketplaceConfig: The parsed marketplace configuration (nil if not found)
// - string: The absolute path to the marketplace.json file (empty if not found)
// - error: Any error during parsing (nil if not found or successful)
func DiscoverMarketplace(basePath string, subpath string) (*MarketplaceConfig, string, error) {
	searchPath := basePath
	if subpath != "" {
		searchPath = filepath.Join(basePath, subpath)
	}
	searchPath = filepath.Clean(searchPath)

	// Verify search path exists
	info, err := os.Stat(searchPath)
	if err != nil {
		// Path doesn't exist, return nil (not an error)
		return nil, "", nil
	}

	// Direct file-mode handling: allow explicit marketplace.json file inputs
	// (for local file imports and repo subpaths resolving to marketplace.json).
	if !info.IsDir() {
		if filepath.Base(searchPath) != marketplaceFileName {
			return nil, "", nil
		}

		config, parseErr := ParseMarketplace(searchPath)
		if parseErr != nil {
			return nil, searchPath, parseErr
		}
		return config, searchPath, nil
	}

	// Common locations to search for marketplace.json
	candidatePaths := []string{
		filepath.Join(searchPath, ".claude-plugin", marketplaceFileName),
		filepath.Join(searchPath, ".opencode-plugin", marketplaceFileName),
		filepath.Join(searchPath, marketplaceFileName),
		filepath.Join(searchPath, ".opencode", marketplaceFileName),
	}

	// Try each candidate path
	for _, path := range candidatePaths {
		if _, err := os.Stat(path); err == nil {
			// File exists, try to parse it
			config, err := ParseMarketplace(path)
			if err != nil {
				return nil, path, err
			}
			return config, path, nil
		}
	}

	// No marketplace.json found (not an error)
	return nil, "", nil
}
