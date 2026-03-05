package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/modifications"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// resourceLoader is a strategy function type for loading different resource types
type resourceLoader func(sourcePath string) (*resource.Resource, error)

// AddCommand adds a command resource to the repository.
// Metadata is automatically saved to .metadata/commands/<name>-metadata.json
func (m *Manager) AddCommand(sourcePath, sourceURL, sourceType string) error {
	return m.AddCommandWithRef(sourcePath, sourceURL, sourceType, "")
}

// AddCommandWithRef adds a command resource to the repository with a specified Git ref.
// Metadata is automatically saved to .metadata/commands/<name>-metadata.json
func (m *Manager) AddCommandWithRef(sourcePath, sourceURL, sourceType, ref string) error {
	return m.addCommandWithOptions(sourcePath, sourceURL, sourceType, ref, ImportOptions{ImportMode: "copy"})
}

// resourceLoader is a strategy function type for loading different resource types
// addResource is the generic internal method that adds any resource type using the strategy pattern
// The loader parameter is a strategy function that knows how to load the specific resource type
// The isDirectory parameter indicates whether to copy/symlink a directory (true) or file (false)
//
//nolint:gocyclo // Core import pipeline handling copy/symlink modes, conflict detection, metadata, and git; must remain atomic.
func (m *Manager) addResource(sourcePath, sourceURL, sourceType, ref string, opts ImportOptions,
	resType resource.ResourceType, loader resourceLoader, isDirectory bool) error {
	// Ensure repo is initialized
	if err := m.Init(); err != nil {
		return err
	}

	// Load the resource using the provided loader strategy
	res, err := loader(sourcePath)
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to load resource",
				"source", sourcePath,
				"type", string(resType),
				"error", err.Error(),
			)
		}
		return fmt.Errorf("failed to load %s: %w", resType, err)
	}

	// Get destination path
	var destPath string
	if resType == resource.Command {
		// Commands may have nested structure, use GetPathForResource
		destPath = m.GetPathForResource(res)

		// Create parent directories if needed (for nested structure)
		parentDir := filepath.Dir(destPath)
		if m.logger != nil {
			m.logger.Debug("creating parent directories",
				"path", parentDir,
				"resource", res.Name,
				"permissions", "0755",
			)
		}
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to create parent directories",
					"path", parentDir,
					"resource", res.Name,
					"error", err.Error(),
				)
			}
			return fmt.Errorf("failed to create parent directories: %w", err)
		}
	} else {
		destPath = m.GetPath(res.Name, resType)
	}

	// Log before copying/symlinking
	if m.logger != nil {
		m.logger.Info("repo add",
			"resource", res.Name,
			"type", string(resType),
			"source", sourcePath,
		)
	}

	// Copy or symlink based on mode
	if opts.ImportMode == "symlink" {
		// Ensure source is absolute path
		absSource, err := filepath.Abs(sourcePath)
		if err != nil {
			if m.logger != nil {
				m.logger.Error("failed to get absolute path",
					"source", sourcePath,
					"error", err.Error(),
				)
			}
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		// Create symlink
		if m.logger != nil {
			m.logger.Debug("creating symlink",
				"resource", res.Name,
				"type", string(resType),
				"source", absSource,
				"dest", destPath,
			)
		}
		if err := os.Symlink(absSource, destPath); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to create symlink",
					"resource", res.Name,
					"type", string(resType),
					"source", absSource,
					"dest", destPath,
					"error", err.Error(),
				)
			}
			return fmt.Errorf("failed to create symlink: %w", err)
		}
	} else {
		// Copy the file or directory (default)
		if isDirectory {
			// Copy directory (for skills)
			if m.logger != nil {
				m.logger.Debug("copying directory",
					"resource", res.Name,
					"type", string(resType),
					"source", sourcePath,
					"dest", destPath,
				)
			}
			if err := m.copyDir(sourcePath, destPath); err != nil {
				if m.logger != nil {
					m.logger.Error("failed to copy directory",
						"resource", res.Name,
						"source", sourcePath,
						"dest", destPath,
						"error", err.Error(),
					)
				}
				return fmt.Errorf("failed to copy %s: %w", resType, err)
			}
		} else {
			// Copy file (for commands and agents)
			if m.logger != nil {
				m.logger.Debug("copying file",
					"resource", res.Name,
					"type", string(resType),
					"source", sourcePath,
					"dest", destPath,
				)
			}
			if err := m.copyFile(sourcePath, destPath); err != nil {
				if m.logger != nil {
					m.logger.Error("failed to copy file",
						"resource", res.Name,
						"source", sourcePath,
						"dest", destPath,
						"error", err.Error(),
					)
				}
				return fmt.Errorf("failed to copy %s: %w", resType, err)
			}
		}
	}

	// Create and save metadata
	now := time.Now()
	meta := &metadata.ResourceMetadata{
		Name:           res.Name,
		Type:           resType,
		SourceType:     sourceType,
		SourceURL:      sourceURL,
		SourceID:       opts.SourceID,
		Ref:            ref,
		FirstInstalled: now,
		LastUpdated:    now,
	}

	// Use explicit sourceName from opts if provided, otherwise derive
	sourceName := opts.SourceName
	if sourceName == "" {
		sourceName = metadata.DeriveSourceName(sourceURL)
	}

	// DEBUG: Log metadata creation with source name
	if m.logger != nil {
		m.logger.Debug("creating resource metadata",
			"resource", res.Name,
			"type", string(resType),
			"source_name", sourceName,
			"source_url", sourceURL,
			"source_type", sourceType,
		)
	}

	if err := metadata.Save(meta, m.repoPath, sourceName); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to save metadata",
				"resource", res.Name,
				"type", string(resType),
				"path", metadata.GetMetadataPath(res.Name, resType, m.repoPath),
				"error", err.Error(),
			)
		}
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Generate modifications if mappings exist in config
	cfg, err := config.LoadGlobal()
	if err == nil && cfg.Mappings.HasAny() {
		gen := modifications.NewGenerator(m.repoPath, cfg.Mappings, m.logger)
		tools, genErr := gen.GenerateForResource(res)
		if genErr != nil {
			if m.logger != nil {
				m.logger.Warn("failed to generate modifications", "error", genErr)
			}
		} else if len(tools) > 0 {
			if m.logger != nil {
				m.logger.Info("generated modifications", "resource", res.Name, "tools", tools)
			}
		}
	}

	return nil
}

// addCommandWithOptions is an internal method that adds a command with import options
func (m *Manager) addCommandWithOptions(sourcePath, sourceURL, sourceType, ref string, opts ImportOptions) error {
	// Special base path calculation for commands (supports nested structure)
	basePath := ""
	cleanPath := filepath.Clean(sourcePath)

	// First try to find a "commands" directory in the path
	if strings.Contains(cleanPath, "commands") {
		parts := strings.Split(cleanPath, string(filepath.Separator))
		for i, part := range parts {
			if part == "commands" {
				// Reconstruct path up to and including "commands"
				// If the original path is absolute, preserve that
				if filepath.IsAbs(cleanPath) {
					basePath = string(filepath.Separator) + filepath.Join(parts[:i+1]...)
				} else {
					basePath = filepath.Join(parts[:i+1]...)
				}
				// Clean to normalize the path
				basePath = filepath.Clean(basePath)
				break
			}
		}
	}

	// If no "commands" directory found, try to find source repo root
	// by looking for common markers (.git, .claude, etc.)
	if basePath == "" {
		dir := filepath.Dir(cleanPath)
		// Walk up to find a suitable base, but stop before system directories
		for dir != "." && dir != "/" && dir != "/tmp" {
			// Don't go too far up - stop at directories with less than 3 segments
			// This prevents walking into /tmp, /var, etc.
			segments := strings.Split(filepath.Clean(dir), string(filepath.Separator))
			if len(segments) < 3 {
				break
			}

			// Check for markers of a repo root
			if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
				basePath = dir
				break
			}
			if _, err := os.Stat(filepath.Join(dir, ".claude")); err == nil {
				basePath = dir
				break
			}
			if _, err := os.Stat(filepath.Join(dir, ".opencode")); err == nil {
				basePath = dir
				break
			}
			dir = filepath.Dir(dir)
		}
	}

	// Use the command loader with base path
	loader := func(path string) (*resource.Resource, error) {
		return resource.LoadCommandWithBase(path, basePath)
	}

	return m.addResource(sourcePath, sourceURL, sourceType, ref, opts, resource.Command, loader, false)
}

// AddSkill adds a skill resource to the repository.
// Metadata is automatically saved to .metadata/skills/<name>-metadata.json
func (m *Manager) AddSkill(sourcePath, sourceURL, sourceType string) error {
	return m.AddSkillWithRef(sourcePath, sourceURL, sourceType, "")
}

// AddSkillWithRef adds a skill resource to the repository with a specified Git ref.
// Metadata is automatically saved to .metadata/skills/<name>-metadata.json
func (m *Manager) AddSkillWithRef(sourcePath, sourceURL, sourceType, ref string) error {
	return m.addSkillWithOptions(sourcePath, sourceURL, sourceType, ref, ImportOptions{ImportMode: "copy"})
}

// addSkillWithOptions is an internal method that adds a skill with import options
func (m *Manager) addSkillWithOptions(sourcePath, sourceURL, sourceType, ref string, opts ImportOptions) error {
	return m.addResource(sourcePath, sourceURL, sourceType, ref, opts, resource.Skill, resource.LoadSkill, true)
}

// AddAgent adds an agent resource to the repository.
// Metadata is automatically saved to .metadata/agents/<name>-metadata.json
func (m *Manager) AddAgent(sourcePath, sourceURL, sourceType string) error {
	return m.AddAgentWithRef(sourcePath, sourceURL, sourceType, "")
}

// AddAgentWithRef adds an agent resource to the repository with a specified Git ref.
// Metadata is automatically saved to .metadata/agents/<name>-metadata.json
func (m *Manager) AddAgentWithRef(sourcePath, sourceURL, sourceType, ref string) error {
	return m.addAgentWithOptions(sourcePath, sourceURL, sourceType, ref, ImportOptions{ImportMode: "copy"})
}

// addAgentWithOptions is an internal method that adds an agent with import options
func (m *Manager) addAgentWithOptions(sourcePath, sourceURL, sourceType, ref string, opts ImportOptions) error {
	return m.addResource(sourcePath, sourceURL, sourceType, ref, opts, resource.Agent, resource.LoadAgent, false)
}

// AddPackage adds a package resource to the repository.
// Metadata is automatically saved to .metadata/packages/<name>-metadata.json
func (m *Manager) AddPackage(sourcePath, sourceURL, sourceType string) error {
	return m.AddPackageWithRef(sourcePath, sourceURL, sourceType, "")
}

// AddPackageWithRef adds a package resource to the repository with a specified Git ref.
// Metadata is automatically saved to .metadata/packages/<name>-metadata.json
func (m *Manager) AddPackageWithRef(sourcePath, sourceURL, sourceType, ref string) error {
	return m.addPackageWithOptions(sourcePath, sourceURL, sourceType, ref, ImportOptions{ImportMode: "copy"})
}

// addPackageWithOptions is an internal method that adds a package with import options
// Note: Packages use a different metadata structure, so we handle them separately
func (m *Manager) addPackageWithOptions(sourcePath, sourceURL, sourceType, ref string, opts ImportOptions) error {
	// Ensure repo is initialized
	if err := m.Init(); err != nil {
		return err
	}

	// DEBUG: Log package loading
	if m.logger != nil {
		m.logger.Debug("loading package",
			"source", sourcePath,
			"source_url", sourceURL,
			"source_type", sourceType,
		)
	}

	// Validate and load the package
	pkg, err := resource.LoadPackage(sourcePath)
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to load package",
				"source", sourcePath,
				"error", err.Error(),
			)
		}
		return fmt.Errorf("failed to load package: %w", err)
	}

	// DEBUG: Log package metadata
	if m.logger != nil {
		m.logger.Debug("package loaded",
			"name", pkg.Name,
			"description", pkg.Description,
			"resource_count", len(pkg.Resources),
			"resources", pkg.Resources,
		)
	}

	// Get destination path
	destPath := resource.GetPackagePath(pkg.Name, m.repoPath)

	// Log before copying/symlinking
	if m.logger != nil {
		m.logger.Info("repo add",
			"resource", pkg.Name,
			"type", "package",
			"source", sourcePath,
		)
	}

	// Copy or symlink based on mode
	if opts.ImportMode == "symlink" {
		// Ensure source is absolute path
		absSource, err := filepath.Abs(sourcePath)
		if err != nil {
			if m.logger != nil {
				m.logger.Error("failed to get absolute path",
					"source", sourcePath,
					"error", err.Error(),
				)
			}
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		// Create symlink
		if m.logger != nil {
			m.logger.Debug("creating symlink",
				"package", pkg.Name,
				"source", absSource,
				"dest", destPath,
			)
		}
		if err := os.Symlink(absSource, destPath); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to create symlink",
					"package", pkg.Name,
					"source", absSource,
					"dest", destPath,
					"error", err.Error(),
				)
			}
			return fmt.Errorf("failed to create symlink: %w", err)
		}
	} else {
		// Copy the file (default)
		if m.logger != nil {
			m.logger.Debug("copying file",
				"package", pkg.Name,
				"source", sourcePath,
				"dest", destPath,
			)
		}
		if err := m.copyFile(sourcePath, destPath); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to copy package",
					"package", pkg.Name,
					"source", sourcePath,
					"dest", destPath,
					"error", err.Error(),
				)
			}
			return fmt.Errorf("failed to copy package: %w", err)
		}
	}

	// Create and save metadata
	now := time.Now()
	// Use explicit sourceName from opts if provided, otherwise derive
	sourceName := opts.SourceName
	if sourceName == "" {
		sourceName = metadata.DeriveSourceName(sourceURL)
	}

	// DEBUG: Log metadata creation with source name
	if m.logger != nil {
		m.logger.Debug("creating package metadata",
			"resource", pkg.Name,
			"type", "package",
			"source_name", sourceName,
			"source_url", sourceURL,
			"source_type", sourceType,
		)
	}

	pkgMeta := &metadata.PackageMetadata{
		Name:          pkg.Name,
		SourceType:    sourceType,
		SourceURL:     sourceURL,
		SourceName:    sourceName,
		SourceID:      opts.SourceID,
		SourceRef:     ref,
		FirstAdded:    now,
		LastUpdated:   now,
		ResourceCount: len(pkg.Resources),
	}
	if err := metadata.SavePackageMetadata(pkgMeta, m.repoPath); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to save package metadata",
				"package", pkg.Name,
				"path", metadata.GetPackageMetadataPath(pkg.Name, m.repoPath),
				"error", err.Error(),
			)
		}
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

// validatePackageResources checks if all referenced resources exist in the repository.
// Returns a slice of missing resource references.
