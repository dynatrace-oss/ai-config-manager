package repo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/modifications"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func (m *Manager) Remove(name string, resourceType resource.ResourceType) error {
	path := m.GetPath(name, resourceType)

	// Check if resource file exists (Lstat so dangling symlinks are found too)
	resourceExists := true
	if _, err := os.Lstat(path); err != nil {
		resourceExists = false
	}

	// Get resource info before removal (needed for modifications cleanup)
	res, _ := m.Get(name, resourceType) // Ignore error - resource might be partially broken

	// Cleanup modifications if they exist
	if res != nil {
		cfg, err := config.LoadGlobal()
		if err == nil {
			gen := modifications.NewGenerator(m.repoPath, cfg.Mappings, m.logger)
			if err := gen.CleanupForResource(res); err != nil {
				if m.logger != nil {
					m.logger.Warn("failed to cleanup modifications", "resource", name, "error", err)
				}
			}
		}
	}

	// Log before removing
	if m.logger != nil {
		m.logger.Info("repo remove",
			"resource", name,
			"type", string(resourceType),
		)
	}

	// Remove the resource file only if it exists
	if resourceExists {
		if m.logger != nil {
			m.logger.Debug("removing resource file/directory",
				"resource", name,
				"type", string(resourceType),
				"path", path,
			)
		}
		if err := os.RemoveAll(path); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to remove resource",
					"resource", name,
					"type", string(resourceType),
					"path", path,
					"error", err.Error(),
				)
			}
			return fmt.Errorf("failed to remove resource: %w", err)
		}
	}

	// Remove metadata file (always attempt — handles orphaned metadata)
	metadataPath := metadata.GetMetadataPath(name, resourceType, m.repoPath)
	metadataRemoved := false
	if _, err := os.Stat(metadataPath); err == nil {
		if m.logger != nil {
			m.logger.Debug("removing metadata file",
				"resource", name,
				"type", string(resourceType),
				"path", metadataPath,
			)
		}
		if err := os.Remove(metadataPath); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to remove metadata",
					"resource", name,
					"type", string(resourceType),
					"path", metadataPath,
					"error", err.Error(),
				)
			}
			return fmt.Errorf("failed to remove metadata: %w", err)
		}
		metadataRemoved = true
	}

	// If neither resource file nor metadata existed, it's truly not found
	if !resourceExists && !metadataRemoved {
		return fmt.Errorf("resource '%s' not found", name)
	}

	// Commit the removal
	commitMsg := fmt.Sprintf("aimgr: remove %s: %s", resourceType, name)
	if err := m.CommitChanges(commitMsg); err != nil {
		// Log warning but don't fail the operation
		fmt.Fprintf(os.Stderr, "Warning: failed to commit changes: %v\n", err)
	}

	return nil
}

// HasSource checks if a resource belongs to the specified source.
// This method checks SourceID first for precise matching, then falls back to
// SourceName matching for backward compatibility with resources that don't yet
// have a SourceID set.
// DEBUG logging helps diagnose source name/ID mismatches during orphan detection.
func (m *Manager) HasSource(name string, resourceType resource.ResourceType, sourceName string) bool {
	// Load metadata to check source
	meta, err := metadata.Load(name, resourceType, m.repoPath)

	// DEBUG: Log each metadata check
	if m.logger != nil {
		if err != nil {
			m.logger.Debug("checking metadata - load failed",
				"resource", name,
				"type", string(resourceType),
				"expected_source", sourceName,
				"error", err.Error(),
			)
			return false
		}

		// Check SourceID first, then fall back to SourceName
		matchByID := meta.SourceID != "" && meta.SourceID == sourceName
		matchByName := meta.SourceName != "" && meta.SourceName == sourceName
		match := matchByID || matchByName

		m.logger.Debug("checking metadata",
			"resource", name,
			"type", string(resourceType),
			"metadata_source_id", meta.SourceID,
			"metadata_source_name", meta.SourceName,
			"expected_source", sourceName,
			"match_by_id", matchByID,
			"match_by_name", matchByName,
			"match", match,
		)

		// Log mismatch explanation
		if !match {
			if meta.SourceName == "" && meta.SourceID == "" {
				m.logger.Debug("resource not removed - no source identity in metadata",
					"resource", name,
					"type", string(resourceType),
					"expected_source", sourceName,
					"reason", "legacy resource without source tracking",
				)
			} else {
				m.logger.Debug("resource not removed - source mismatch",
					"resource", name,
					"type", string(resourceType),
					"expected_source", sourceName,
					"actual_source_id", meta.SourceID,
					"actual_source_name", meta.SourceName,
					"reason", "source identity does not match",
				)
			}
		} else {
			m.logger.Debug("resource will be removed",
				"resource", name,
				"type", string(resourceType),
				"source_name", sourceName,
				"matched_by", func() string {
					if matchByID {
						return "source_id"
					}
					return "source_name"
				}(),
			)
		}

		return match
	}

	// Fallback to simple check without logging
	if meta.SourceID != "" && meta.SourceID == sourceName {
		return true
	}
	return meta.SourceName != "" && meta.SourceName == sourceName
}

// GetPath returns the full path to a resource in the repository
// Handles nested paths for commands (e.g., "api/deploy" -> "commands/api/deploy.md")
func (m *Manager) GetPath(name string, resourceType resource.ResourceType) string {
	switch resourceType {
	case resource.Command:
		return filepath.Join(m.repoPath, "commands", name) + ".md"
	case resource.Skill:
		return filepath.Join(m.repoPath, "skills", name)
	case resource.Agent:
		return filepath.Join(m.repoPath, "agents", name+".md")
	case resource.PackageType:
		return filepath.Join(m.repoPath, "packages", name+".package.json")
	default:
		return ""
	}
}

// GetPathForResource returns the full path for a resource
// For commands with nested names (e.g., "api/deploy"), creates nested directory structure
func (m *Manager) GetPathForResource(res *resource.Resource) string {
	switch res.Type {
	case resource.Command:
		return filepath.Join(m.repoPath, "commands", res.Name+".md")
	case resource.Skill:
		return filepath.Join(m.repoPath, "skills", res.Name)
	case resource.Agent:
		return filepath.Join(m.repoPath, "agents", res.Name+".md")
	default:
		return ""
	}
}

// GetMetadata retrieves metadata for a specific resource.
// Loads metadata from .metadata/<type>s/<name>-metadata.json
func (m *Manager) GetMetadata(name string, resourceType resource.ResourceType) (*metadata.ResourceMetadata, error) {
	// Log the get metadata operation
	if m.logger != nil {
		m.logger.Debug("repo get metadata",
			"resource", name,
			"type", string(resourceType),
		)
	}

	return metadata.Load(name, resourceType, m.repoPath)
}

func (m *Manager) Drop() error {
	// Remove entire repo directory
	if m.logger != nil {
		m.logger.Debug("removing entire repository directory",
			"path", m.repoPath,
		)
	}
	if err := os.RemoveAll(m.repoPath); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to remove repository",
				"path", m.repoPath,
				"error", err.Error(),
			)
		}
		return fmt.Errorf("failed to remove repository: %w", err)
	}

	// Recreate empty structure (including empty ai.repo.yaml)
	return m.Init()
}
