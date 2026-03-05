package repo

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// scanOrphanedMetadata scans for metadata files that don't have corresponding source files
func (m *Manager) scanOrphanedMetadata(resourceType resource.ResourceType, sourcePath string) {
	if m.logger == nil {
		return
	}

	// Get metadata directory for this resource type
	metadataDir := filepath.Join(m.repoPath, ".metadata", string(resourceType)+"s")
	if _, err := os.Stat(metadataDir); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-metadata.json") {
			continue
		}

		metadataPath := filepath.Join(metadataDir, entry.Name())

		// Extract name from filename: <name>-metadata.json where name might have hyphens from nested paths
		baseName := strings.TrimSuffix(entry.Name(), "-metadata.json")

		// Check if source file exists
		var sourceFilePath string
		var alternativePath string
		switch resourceType {
		case resource.Command:
			// For commands, try with .md extension
			sourceFilePath = filepath.Join(sourcePath, baseName+".md")
			// Also try with slashes instead of hyphens for nested commands (e.g., api-deploy -> api/deploy)
			nestedName := strings.ReplaceAll(baseName, "-", "/")
			alternativePath = filepath.Join(sourcePath, nestedName+".md")
		case resource.Skill:
			sourceFilePath = filepath.Join(sourcePath, baseName)
		case resource.Agent:
			sourceFilePath = filepath.Join(sourcePath, baseName+".md")
		default:
			continue
		}

		// Check if either the direct path or alternative path exists
		_, err1 := os.Stat(sourceFilePath)
		exists := err1 == nil

		if alternativePath != "" {
			_, err2 := os.Stat(alternativePath)
			exists = exists || err2 == nil
		}

		if !exists {
			m.logger.Warn("orphaned metadata detected",
				"resource_type", resourceType,
				"metadata_file", metadataPath,
				"expected_source", sourceFilePath,
				"action", "run 'aimgr repo verify' to identify issues",
			)
		}
	}
}

// checkOrphanedFiles checks if a resource file exists without corresponding metadata
func (m *Manager) checkOrphanedFiles(name string, resourceType resource.ResourceType) {
	if m.logger == nil {
		return
	}

	// Check if metadata exists for this resource
	metadataPath := metadata.GetMetadataPath(name, resourceType, m.repoPath)
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		// Metadata doesn't exist - this is an orphaned file
		sourcePath := m.GetPath(name, resourceType)
		m.logger.Warn("orphaned file detected",
			"resource_type", resourceType,
			"resource_name", name,
			"source_file", sourcePath,
			"missing_metadata", metadataPath,
			"action", "resource may have been manually added; consider running 'aimgr repo verify'",
		)
	}
}
