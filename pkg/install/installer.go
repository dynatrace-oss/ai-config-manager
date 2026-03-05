package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/modifications"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

// Installer manages resource installations in a project
type Installer struct {
	projectPath string
	targetTools []tools.Tool // tools to install to
}

// NewInstaller creates a new installer for a project with default tools
func NewInstaller(projectPath string, defaultTools []tools.Tool) (*Installer, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project path: %w", err)
	}

	installer := &Installer{
		projectPath: absPath,
	}

	// Detect install targets
	targets, err := DetectInstallTargets(absPath, defaultTools)
	if err != nil {
		return nil, fmt.Errorf("detecting install targets: %w", err)
	}

	installer.targetTools = targets
	return installer, nil
}

// NewInstallerWithTargets creates a new installer with explicit target tools
// This bypasses tool detection and uses the specified targets directly
func NewInstallerWithTargets(projectPath string, targets []tools.Tool) (*Installer, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project path: %w", err)
	}

	installer := &Installer{
		projectPath: absPath,
		targetTools: targets,
	}

	return installer, nil
}

// DetectInstallTargets determines which tools to install to
// Precedence (highest to lowest):
// 1. Existing tool directories (if any exist, installs to ALL)
// 2. ai.package.yaml install.targets (if manifest exists and has targets)
// 3. Global config install.targets (defaultTools parameter)
func DetectInstallTargets(projectPath string, defaultTools []tools.Tool) ([]tools.Tool, error) {
	existingTools, err := tools.DetectExistingTools(projectPath)
	if err != nil {
		return nil, fmt.Errorf("detecting existing tools: %w", err)
	}

	// If tools are found, install to ALL of them
	if len(existingTools) > 0 {
		return existingTools, nil
	}

	// Check for ai.package.yaml manifest
	manifestPath := filepath.Join(projectPath, manifest.ManifestFileName)
	if manifest.Exists(manifestPath) {
		m, err := manifest.Load(manifestPath)
		if err != nil {
			// If manifest exists but can't be loaded, continue with defaults
			// (Don't fail the install operation due to manifest errors)
		} else if len(m.Install.Targets) > 0 {
			// Use manifest targets (overrides global config)
			manifestTargets := make([]tools.Tool, 0, len(m.Install.Targets))
			for _, targetStr := range m.Install.Targets {
				tool, err := tools.ParseTool(targetStr)
				if err != nil {
					return nil, fmt.Errorf("invalid install.targets in manifest '%s': %w", targetStr, err)
				}
				manifestTargets = append(manifestTargets, tool)
			}
			return manifestTargets, nil
		}
	}

	// Fall back to global config defaults
	return defaultTools, nil
}

// ensureValidSymlink checks if a symlink exists and is valid.
// If the symlink exists but points to a broken target or wrong repository, it removes the symlink.
// Returns true if installation should proceed, false if valid symlink already exists.
func ensureValidSymlink(symlinkPath string, expectedTarget string, repoPath string) (bool, error) {
	// Check if symlink exists
	linkInfo, err := os.Lstat(symlinkPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Doesn't exist, proceed with installation
			return true, nil
		}
		return false, fmt.Errorf("failed to check symlink: %w", err)
	}

	// Verify it's a symlink
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		// Not a symlink, but something exists at this path - skip to avoid overwriting
		return false, nil
	}

	// Check if target exists
	if _, err := os.Stat(symlinkPath); err != nil {
		// Broken symlink - remove it so we can recreate
		if err := os.Remove(symlinkPath); err != nil {
			return false, fmt.Errorf("failed to remove broken symlink: %w", err)
		}
		return true, nil // Proceed with installation
	}

	// Symlink exists and target is valid - verify it points to correct repo
	actualTarget, err := os.Readlink(symlinkPath)
	if err != nil {
		return false, fmt.Errorf("failed to read symlink target: %w", err)
	}

	// Resolve to absolute path for comparison
	absActualTarget := actualTarget
	if !filepath.IsAbs(actualTarget) {
		absActualTarget = filepath.Join(filepath.Dir(symlinkPath), actualTarget)
	}

	// Check if target is within the expected repository
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return false, fmt.Errorf("failed to resolve repo path: %w", err)
	}

	// Compare the actual target with expected - if they don't match or wrong repo, recreate
	if absActualTarget != expectedTarget {
		// Check if it's just pointing to a different repo location
		relPath, err := filepath.Rel(absRepoPath, absActualTarget)
		if err != nil || strings.HasPrefix(relPath, "..") {
			// Points to outside repo or different location - remove and recreate
			if err := os.Remove(symlinkPath); err != nil {
				return false, fmt.Errorf("failed to remove symlink pointing to wrong location: %w", err)
			}
			return true, nil // Proceed with installation
		}
	}

	// Valid symlink pointing to correct location - skip installation
	return false, nil
}

// getSymlinkSource returns the path to symlink to for a resource and tool.
// Returns modification path if it exists for the target tool, otherwise returns the original path.
func (i *Installer) getSymlinkSource(res *resource.Resource, tool tools.Tool, repoPath string) string {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return res.Path // Fall back to original on config error
	}

	gen := modifications.NewGenerator(repoPath, cfg.Mappings, nil) // logger can be nil for read-only ops
	toolName := tool.String()                                      // e.g., "opencode", "claude"

	if modPath := gen.GetModificationPath(res, toolName); modPath != "" {
		return modPath
	}
	return res.Path
}

// InstallCommand installs a command resource by creating symlinks to target tools
func (i *Installer) InstallCommand(name string, repoManager *repo.Manager) error {
	// Get command from repo
	res, err := repoManager.Get(name, resource.Command)
	if err != nil {
		return fmt.Errorf("command not found in repository: %w", err)
	}

	// Install to each target tool
	for _, tool := range i.targetTools {
		toolInfo := tools.GetToolInfo(tool)

		// Skip tools that don't support commands
		if !toolInfo.SupportsCommands {
			continue
		}

		// Create commands directory if needed
		commandsDir := filepath.Join(i.projectPath, toolInfo.CommandsDir)
		if err := os.MkdirAll(commandsDir, 0755); err != nil {
			return fmt.Errorf("failed to create commands directory for %s: %w", tool, err)
		}

		// Determine symlink path using resource name (supports nested structure)
		// For nested commands (e.g., name="api/deploy"), create nested directories
		symlinkPath := filepath.Join(commandsDir, res.Name+".md")

		// Create parent directories if needed (for nested structure)
		if err := os.MkdirAll(filepath.Dir(symlinkPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", tool, err)
		}

		// Determine source path (modification if exists, otherwise original)
		sourcePath := i.getSymlinkSource(res, tool, repoManager.GetRepoPath())

		// Check if valid symlink already exists (removes broken symlinks)
		shouldInstall, err := ensureValidSymlink(symlinkPath, sourcePath, repoManager.GetRepoPath())
		if err != nil {
			return fmt.Errorf("failed to check existing installation for %s: %w", tool, err)
		}
		if !shouldInstall {
			// Valid symlink exists, skip this tool
			continue
		}

		// Create symlink
		if err := os.Symlink(sourcePath, symlinkPath); err != nil {
			return fmt.Errorf("failed to create symlink for %s: %w", tool, err)
		}

		// Log successful installation
		if logger := repoManager.GetLogger(); logger != nil {
			logger.Info("resource installed",
				"operation", "install",
				"resource_type", "command",
				"resource_name", res.Name,
				"tool", tool.String(),
				"dest_path", symlinkPath,
				"source_path", sourcePath,
			)
		}
	}

	return nil
}

// InstallSkill installs a skill resource by creating symlinks to target tools
func (i *Installer) InstallSkill(name string, repoManager *repo.Manager) error {
	// Get skill from repo
	res, err := repoManager.Get(name, resource.Skill)
	if err != nil {
		return fmt.Errorf("skill not found in repository: %w", err)
	}

	// Install to each target tool
	for _, tool := range i.targetTools {
		toolInfo := tools.GetToolInfo(tool)

		// Skip tools that don't support skills (though all current tools do)
		if !toolInfo.SupportsSkills {
			continue
		}

		// Create skills directory if needed
		skillsDir := filepath.Join(i.projectPath, toolInfo.SkillsDir)
		if err := os.MkdirAll(skillsDir, 0755); err != nil {
			return fmt.Errorf("failed to create skills directory for %s: %w", tool, err)
		}

		// Symlink path
		symlinkPath := filepath.Join(skillsDir, name)

		// Determine source path (modification if exists, otherwise original)
		sourcePath := i.getSymlinkSource(res, tool, repoManager.GetRepoPath())

		// Check if valid symlink already exists (removes broken symlinks)
		shouldInstall, err := ensureValidSymlink(symlinkPath, sourcePath, repoManager.GetRepoPath())
		if err != nil {
			return fmt.Errorf("failed to check existing installation for %s: %w", tool, err)
		}
		if !shouldInstall {
			// Valid symlink exists, skip this tool
			continue
		}

		// Create symlink
		if err := os.Symlink(sourcePath, symlinkPath); err != nil {
			return fmt.Errorf("failed to create symlink for %s: %w", tool, err)
		}

		// Log successful installation
		if logger := repoManager.GetLogger(); logger != nil {
			logger.Info("resource installed",
				"operation", "install",
				"resource_type", "skill",
				"resource_name", res.Name,
				"tool", tool.String(),
				"dest_path", symlinkPath,
				"source_path", sourcePath,
			)
		}
	}

	return nil
}

// InstallAgent installs an agent resource by creating symlinks to target tools
func (i *Installer) InstallAgent(name string, repoManager *repo.Manager) error {
	// Get agent from repo
	res, err := repoManager.Get(name, resource.Agent)
	if err != nil {
		return fmt.Errorf("agent not found in repository: %w", err)
	}

	// Install to each target tool
	for _, tool := range i.targetTools {
		toolInfo := tools.GetToolInfo(tool)

		// Skip tools that don't support agents
		if !toolInfo.SupportsAgents {
			continue
		}

		// Create agents directory if needed
		agentsDir := filepath.Join(i.projectPath, toolInfo.AgentsDir)
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			return fmt.Errorf("failed to create agents directory for %s: %w", tool, err)
		}

		// Symlink path
		symlinkPath := filepath.Join(agentsDir, filepath.Base(res.Path))

		// Determine source path (modification if exists, otherwise original)
		sourcePath := i.getSymlinkSource(res, tool, repoManager.GetRepoPath())

		// Check if valid symlink already exists (removes broken symlinks)
		shouldInstall, err := ensureValidSymlink(symlinkPath, sourcePath, repoManager.GetRepoPath())
		if err != nil {
			return fmt.Errorf("failed to check existing installation for %s: %w", tool, err)
		}
		if !shouldInstall {
			// Valid symlink exists, skip this tool
			continue
		}

		// Create symlink
		if err := os.Symlink(sourcePath, symlinkPath); err != nil {
			return fmt.Errorf("failed to create symlink for %s: %w", tool, err)
		}

		// Log successful installation
		if logger := repoManager.GetLogger(); logger != nil {
			logger.Info("resource installed",
				"operation", "install",
				"resource_type", "agent",
				"resource_name", res.Name,
				"tool", tool.String(),
				"dest_path", symlinkPath,
				"source_path", sourcePath,
			)
		}
	}

	return nil
}

// Uninstall removes an installed resource by removing symlinks from all tool directories
func (i *Installer) Uninstall(name string, resourceType resource.ResourceType, repoManager *repo.Manager) error {
	removed := false
	var lastErr error

	// Try to uninstall from all target tools
	for _, tool := range i.targetTools {
		toolInfo := tools.GetToolInfo(tool)
		var symlinkPath string

		switch resourceType {
		case resource.Command:
			if !toolInfo.SupportsCommands {
				continue
			}
			symlinkPath = filepath.Join(i.projectPath, toolInfo.CommandsDir, name+".md")
		case resource.Skill:
			if !toolInfo.SupportsSkills {
				continue
			}
			symlinkPath = filepath.Join(i.projectPath, toolInfo.SkillsDir, name)
		case resource.Agent:
			if !toolInfo.SupportsAgents {
				continue
			}
			symlinkPath = filepath.Join(i.projectPath, toolInfo.AgentsDir, name+".md")
		default:
			return fmt.Errorf("invalid resource type: %s", resourceType)
		}

		// Check if symlink exists
		info, err := os.Lstat(symlinkPath)
		if err != nil {
			// Doesn't exist in this tool directory, continue
			continue
		}

		// Verify it's a symlink
		if info.Mode()&os.ModeSymlink == 0 {
			lastErr = fmt.Errorf("'%s' in %s is not a symlink (manual installation?)", name, tool)
			continue
		}

		// Remove the symlink
		if err := os.Remove(symlinkPath); err != nil {
			lastErr = fmt.Errorf("failed to remove symlink from %s: %w", tool, err)
			continue
		}

		// Log successful uninstallation
		if logger := repoManager.GetLogger(); logger != nil {
			logger.Info("resource uninstalled",
				"operation", "uninstall",
				"resource_type", resourceType,
				"resource_name", name,
				"tool", tool.String(),
				"dest_path", symlinkPath,
			)
		}

		removed = true
	}

	if !removed {
		if lastErr != nil {
			return lastErr
		}
		return fmt.Errorf("resource '%s' is not installed", name)
	}

	return nil
}

// scanFileSymlinks scans a directory for symlinked file-based resources (commands, agents)
// and adds them to the resourceMap. loader is the function to load the resource from a target path.
func scanFileSymlinks(dir string, resType resource.ResourceType, loader func(string) (*resource.Resource, error), tool tools.Tool, resourceMap map[string]resource.Resource) error {
	if _, err := os.Stat(dir); err != nil {
		return nil // Directory doesn't exist, skip
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read %s directory for %s: %w", resType, tool, err)
	}

	for _, entry := range entries {
		symlinkPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			// Recurse into subdirectory for namespaced resources (one level only)
			subEntries, err := os.ReadDir(symlinkPath)
			if err != nil {
				continue
			}
			for _, subEntry := range subEntries {
				if subEntry.IsDir() {
					continue // Only one level of nesting
				}
				subPath := filepath.Join(symlinkPath, subEntry.Name())
				subInfo, err := os.Lstat(subPath)
				if err != nil {
					continue
				}
				// Only list symlinks
				if subInfo.Mode()&os.ModeSymlink == 0 {
					continue
				}
				// Read the symlink target
				target, err := os.Readlink(subPath)
				if err != nil {
					continue
				}
				// Build namespaced name: "dirname/filename-without-ext"
				name := entry.Name() + "/" + strings.TrimSuffix(subEntry.Name(), ".md")
				// Check if target exists
				if _, err := os.Stat(target); err != nil {
					// Broken symlink
					resourceMap[name] = resource.Resource{
						Name:   name,
						Type:   resType,
						Path:   target,
						Health: resource.HealthBroken,
					}
					continue
				}
				// Load the resource
				res, err := loader(target)
				if err != nil {
					continue
				}
				res.Health = resource.HealthOK
				// Override name with namespaced version
				res.Name = name
				resourceMap[res.Name] = *res
			}
			continue
		}

		info, err := os.Lstat(symlinkPath)
		if err != nil {
			continue
		}

		// Only list symlinks
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		// Read the symlink target
		target, err := os.Readlink(symlinkPath)
		if err != nil {
			continue
		}

		// Check if target exists
		if _, err := os.Stat(target); err != nil {
			// Broken symlink — create a minimal resource entry
			name := strings.TrimSuffix(entry.Name(), ".md")
			resourceMap[name] = resource.Resource{
				Name:   name,
				Type:   resType,
				Path:   target,
				Health: resource.HealthBroken,
			}
			continue
		}

		// Load the resource
		res, err := loader(target)
		if err != nil {
			continue
		}

		res.Health = resource.HealthOK
		// Deduplicate by name
		resourceMap[res.Name] = *res
	}
	return nil
}

// List lists all installed resources in the project (deduplicated across tools)
func (i *Installer) List() ([]resource.Resource, error) {
	// Use a map to deduplicate resources by name
	resourceMap := make(map[string]resource.Resource)

	// Scan all target tools
	for _, tool := range i.targetTools {
		toolInfo := tools.GetToolInfo(tool)

		// List commands
		if toolInfo.SupportsCommands {
			commandsDir := filepath.Join(i.projectPath, toolInfo.CommandsDir)
			if err := scanFileSymlinks(commandsDir, resource.Command, resource.LoadCommand, tool, resourceMap); err != nil {
				return nil, err
			}
		}

		// List skills
		if toolInfo.SupportsSkills {
			skillsDir := filepath.Join(i.projectPath, toolInfo.SkillsDir)
			if _, err := os.Stat(skillsDir); err == nil {
				entries, err := os.ReadDir(skillsDir)
				if err != nil {
					return nil, fmt.Errorf("failed to read skills directory for %s: %w", tool, err)
				}

				for _, entry := range entries {
					symlinkPath := filepath.Join(skillsDir, entry.Name())
					info, err := os.Lstat(symlinkPath)
					if err != nil {
						continue
					}

					// Only list symlinks
					if info.Mode()&os.ModeSymlink == 0 {
						continue
					}

					// Read the symlink target
					target, err := os.Readlink(symlinkPath)
					if err != nil {
						continue
					}

					// Verify target is a directory (skill)
					targetInfo, err := os.Stat(target)
					if err != nil || !targetInfo.IsDir() {
						// Broken symlink or not a directory
						resourceMap[entry.Name()] = resource.Resource{
							Name:   entry.Name(),
							Type:   resource.Skill,
							Path:   target,
							Health: resource.HealthBroken,
						}
						continue
					}

					// Load the resource
					res, err := resource.LoadSkill(target)
					if err != nil {
						continue
					}

					res.Health = resource.HealthOK
					// Deduplicate by name
					resourceMap[res.Name] = *res
				}
			}
		}

		// List agents
		if toolInfo.SupportsAgents {
			agentsDir := filepath.Join(i.projectPath, toolInfo.AgentsDir)
			if err := scanFileSymlinks(agentsDir, resource.Agent, resource.LoadAgent, tool, resourceMap); err != nil {
				return nil, err
			}
		}
	}

	// Convert map to slice
	resources := make([]resource.Resource, 0, len(resourceMap))
	for _, res := range resourceMap {
		resources = append(resources, res)
	}

	return resources, nil
}

// IsInstalled checks if a resource is installed in any tool directory
func (i *Installer) IsInstalled(name string, resourceType resource.ResourceType) bool {
	// Check if installed in any target tool
	for _, tool := range i.targetTools {
		toolInfo := tools.GetToolInfo(tool)
		var symlinkPath string

		switch resourceType {
		case resource.Command:
			if !toolInfo.SupportsCommands {
				continue
			}
			symlinkPath = filepath.Join(i.projectPath, toolInfo.CommandsDir, name+".md")
		case resource.Skill:
			if !toolInfo.SupportsSkills {
				continue
			}
			symlinkPath = filepath.Join(i.projectPath, toolInfo.SkillsDir, name)
		case resource.Agent:
			if !toolInfo.SupportsAgents {
				continue
			}
			symlinkPath = filepath.Join(i.projectPath, toolInfo.AgentsDir, name+".md")
		default:
			return false
		}

		// Check if symlink exists
		info, err := os.Lstat(symlinkPath)
		if err != nil {
			continue
		}

		// Verify it's a symlink and check if target is valid
		if info.Mode()&os.ModeSymlink != 0 {
			// Check if target exists (os.Stat follows symlinks)
			if _, err := os.Stat(symlinkPath); err == nil {
				return true // Valid symlink exists
			}
			// Broken symlink - not considered installed
		}
	}

	return false
}

// GetTargetTools returns the list of tools being targeted for installation
func (i *Installer) GetTargetTools() []tools.Tool {
	return i.targetTools
}
