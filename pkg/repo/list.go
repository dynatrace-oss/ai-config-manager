package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// List returns resources from the repository, optionally filtered by type.
//
//nolint:gocyclo // Iterates 4 resource types with per-type loading, symlink resolution, and metadata enrichment; splitting would duplicate setup logic.
func (m *Manager) List(resourceType *resource.ResourceType) ([]resource.Resource, error) {
	var resources []resource.Resource

	// Log the list operation
	if m.logger != nil {
		if resourceType != nil {
			m.logger.Debug("repo list",
				"type_filter", string(*resourceType),
			)
		} else {
			m.logger.Debug("repo list",
				"type_filter", "all",
			)
		}
	}

	// List commands if no filter or filter is Command
	if resourceType == nil || *resourceType == resource.Command {
		commandsPath := filepath.Join(m.repoPath, "commands")
		if _, err := os.Stat(commandsPath); err == nil {
			// Use filepath.Walk to find commands recursively (supports nested structure)
			err := filepath.Walk(commandsPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Skip directories and non-.md files
				if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
					return nil
				}

				// Load command with base path to calculate RelativePath
				res, err := resource.LoadCommandWithBase(path, commandsPath)
				if err != nil {
					// Skip invalid commands
					return nil
				}
				resources = append(resources, *res)

				// Check for orphaned files (files without metadata)
				m.checkOrphanedFiles(res.Name, resource.Command)

				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to walk commands directory: %w", err)
			}

			// Check for orphaned metadata (metadata without files)
			m.scanOrphanedMetadata(resource.Command, commandsPath)
		}
	}

	// List skills if no filter or filter is Skill
	if resourceType == nil || *resourceType == resource.Skill {
		skillsPath := filepath.Join(m.repoPath, "skills")
		if _, err := os.Stat(skillsPath); err == nil {
			entries, err := os.ReadDir(skillsPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read skills directory: %w", err)
			}

			for _, entry := range entries {
				skillPath := filepath.Join(skillsPath, entry.Name())

				// Follow symlinks to check if target is a directory
				info, err := os.Stat(skillPath)
				if err != nil || !info.IsDir() {
					continue
				}

				res, err := resource.LoadSkill(skillPath)
				if err != nil {
					// Skip invalid skills
					continue
				}
				resources = append(resources, *res)

				// Check for orphaned files (files without metadata)
				m.checkOrphanedFiles(res.Name, resource.Skill)
			}

			// Check for orphaned metadata (metadata without files)
			m.scanOrphanedMetadata(resource.Skill, skillsPath)
		}
	}

	// List agents if no filter or filter is Agent
	if resourceType == nil || *resourceType == resource.Agent {
		agentsPath := filepath.Join(m.repoPath, "agents")
		if _, err := os.Stat(agentsPath); err == nil {
			entries, err := os.ReadDir(agentsPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read agents directory: %w", err)
			}

			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
					continue
				}

				agentPath := filepath.Join(agentsPath, entry.Name())
				res, err := resource.LoadAgent(agentPath)
				if err != nil {
					// Skip invalid agents
					continue
				}
				resources = append(resources, *res)

				// Check for orphaned files (files without metadata)
				m.checkOrphanedFiles(res.Name, resource.Agent)
			}

			// Check for orphaned metadata (metadata without files)
			m.scanOrphanedMetadata(resource.Agent, agentsPath)
		}
	}

	// List packages if no filter or filter is PackageType
	if resourceType == nil || *resourceType == resource.PackageType {
		packagesPath := filepath.Join(m.repoPath, "packages")
		if _, err := os.Stat(packagesPath); err == nil {
			entries, err := os.ReadDir(packagesPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read packages directory: %w", err)
			}

			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".package.json") {
					continue
				}

				packagePath := filepath.Join(packagesPath, entry.Name())
				pkg, err := resource.LoadPackage(packagePath)
				if err != nil {
					// Skip invalid packages
					continue
				}
				// Convert Package to Resource format
				res := resource.Resource{
					Name:        pkg.Name,
					Type:        resource.PackageType,
					Description: pkg.Description,
					Path:        packagePath,
				}
				resources = append(resources, res)
			}
		}
	}

	// Log the result count
	sortResources(resources)

	if m.logger != nil {
		if resourceType != nil {
			m.logger.Debug("repo list result",
				"type_filter", string(*resourceType),
				"count", len(resources),
			)
		} else {
			m.logger.Debug("repo list result",
				"type_filter", "all",
				"count", len(resources),
			)
		}
	}

	return resources, nil
}

// PackageInfo represents package information for listing
type PackageInfo struct {
	Name          string `json:"name" yaml:"name"`
	Description   string `json:"description" yaml:"description"`
	ResourceCount int    `json:"resource_count" yaml:"resource_count"`
}

// ListPackages lists all packages in the repository
func (m *Manager) ListPackages() ([]PackageInfo, error) {
	var packages []PackageInfo

	// Log the list operation
	if m.logger != nil {
		m.logger.Debug("repo list packages")
	}

	packagesPath := filepath.Join(m.repoPath, "packages")
	if _, err := os.Stat(packagesPath); err != nil {
		// Packages directory doesn't exist, return empty list
		return packages, nil
	}

	entries, err := os.ReadDir(packagesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read packages directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".package.json") {
			continue
		}

		pkgPath := filepath.Join(packagesPath, entry.Name())
		pkg, err := resource.LoadPackageLenient(pkgPath)
		if err != nil {
			// Skip invalid packages
			continue
		}

		packages = append(packages, PackageInfo{
			Name:          pkg.Name,
			Description:   pkg.Description,
			ResourceCount: len(pkg.Resources),
		})
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})

	// Log the result count
	if m.logger != nil {
		m.logger.Debug("repo list packages result",
			"count", len(packages),
		)
	}

	return packages, nil
}

func sortResources(resources []resource.Resource) {
	sort.Slice(resources, func(i, j int) bool {
		leftOrder := resourceTypeOrder(resources[i].Type)
		rightOrder := resourceTypeOrder(resources[j].Type)
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}

		return resources[i].Name < resources[j].Name
	})
}

func resourceTypeOrder(resType resource.ResourceType) int {
	switch resType {
	case resource.Command:
		return 0
	case resource.Skill:
		return 1
	case resource.Agent:
		return 2
	case resource.PackageType:
		return 3
	default:
		return 4
	}
}

// Get retrieves a specific resource by name and type
func (m *Manager) Get(name string, resourceType resource.ResourceType) (*resource.Resource, error) {
	// Log the get operation
	if m.logger != nil {
		m.logger.Debug("repo get",
			"resource", name,
			"type", string(resourceType),
		)
	}

	path := m.GetPath(name, resourceType)

	// Check if resource exists
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("resource '%s' not found", name)
	}

	// Load the resource
	switch resourceType {
	case resource.Command:
		// Use LoadCommandWithBase to preserve nested names
		commandsPath := filepath.Join(m.repoPath, "commands")
		return resource.LoadCommandWithBase(path, commandsPath)
	case resource.Skill:
		return resource.LoadSkill(path)
	case resource.Agent:
		return resource.LoadAgent(path)
	default:
		return nil, fmt.Errorf("invalid resource type: %s", resourceType)
	}
}

// Remove removes a resource from the repository.
// Also removes associated metadata from .metadata/<type>s/<name>-metadata.json
