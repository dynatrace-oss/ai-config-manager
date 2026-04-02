package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/install"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
	"github.com/spf13/cobra"
)

var (
	projectPathFlag        string
	installForceFlag       bool
	installTargetFlag      string
	installSaveFlag        bool = true
	installNoSaveFlag      bool
	installingFromManifest bool
)

// installResult tracks the result of installing a single resource
type installResult struct {
	resourceType resource.ResourceType
	name         string
	success      bool
	skipped      bool
	message      string
	toolsAdded   []tools.Tool
}

// parseTargetFlag parses the --target flag and returns a list of tools
// If the flag is empty, returns nil (use defaults)
// If the flag contains values, parses and validates them
func parseTargetFlag(targetFlag string) ([]tools.Tool, error) {
	if targetFlag == "" {
		return nil, nil
	}

	var targets []tools.Tool
	targetStrs := strings.Split(targetFlag, ",")
	for _, t := range targetStrs {
		tool, err := tools.ParseTool(strings.TrimSpace(t))
		if err != nil {
			return nil, fmt.Errorf("invalid target '%s': %w", t, err)
		}
		targets = append(targets, tool)
	}

	return targets, nil
}

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install [resource]...",
	Short: "Install resources to a project",
	Long: `Install one or more resources (commands, skills, agents, or packages) to a project.

If no resources are specified, installs all resources from ai.package.yaml in the current directory.

Resources are specified using the format 'type/name':
  - command/name (or commands/name)
  - skill/name (or skills/name)
  - agent/name (or agents/name)
  - package/name (or packages/name) - installs all resources in the package

Pattern matching is supported using glob syntax:
  - * matches any sequence of characters
  - ? matches any single character
  - [abc] matches any character in the set
  - {a,b} matches any alternative

Multi-tool behavior:
  - If tool directories exist (.claude, .opencode, .github/skills, .github/agents), installs to ALL of them
  - If no tool directories exist, creates and installs to your default tool
  - Default tool is configured in ~/.config/aimgr/aimgr.yaml (use 'aimgr config set install.targets <tool>')

Supported tools:
  - claude:   Claude Code (.claude/commands, .claude/skills, .claude/agents)
  - opencode: OpenCode (.opencode/commands, .opencode/skills, .opencode/agents)
  - copilot:  GitHub Copilot (.github/skills and .github/agents; commands/prompt files remain unsupported)

Examples:
  # Install from ai.package.yaml
  aimgr install

  # Install a single skill
  aimgr install skill/pdf-processing

  # Install a package (installs all resources in it)
  aimgr install package/web-tools

  # Install multiple resources at once
  aimgr install skill/foo command/bar agent/reviewer

  # Install all skills
  aimgr install "skill/*"

  # Install all test resources across all types
  aimgr install "*test*"

  # Install skills starting with "pdf"
  aimgr install "skill/pdf*"

  # Install multiple patterns
  aimgr install "skill/pdf*" "command/test*"

  # Install to specific project
  aimgr install skill/test --project-path ~/project

  # Force reinstall
  aimgr install command/test --force

  # Install to specific target
  aimgr install skill/utils --target claude`,
	Args:              cobra.ArbitraryArgs, // Allow 0 or more args
	ValidArgsFunction: completeInstallResources,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle zero-arg install (from ai.package.yaml)
		if len(args) == 0 {
			return installFromManifest()
		}

		// Get project path (current directory or flag)
		projectPath := projectPathFlag
		if projectPath == "" {
			var err error
			projectPath, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Parse target flag (if provided)
		explicitTargets, err := parseTargetFlag(installTargetFlag)
		if err != nil {
			return err
		}

		// Create installer
		var installer *install.Installer
		if explicitTargets != nil {
			// Use explicit targets from --target flag (bypass detection)
			installer, err = install.NewInstallerWithTargets(projectPath, explicitTargets)
		} else {
			// Auto-detect existing tools or use config defaults
			cfg, cfgErr := config.LoadGlobal()
			if cfgErr != nil {
				return fmt.Errorf("failed to load config: %w", cfgErr)
			}
			defaultTargets, tgtErr := cfg.GetDefaultTargets()
			if tgtErr != nil {
				return fmt.Errorf("invalid default targets in config: %w", tgtErr)
			}
			// NewInstaller will auto-detect existing tool directories
			installer, err = install.NewInstaller(projectPath, defaultTargets)
		}
		if err != nil {
			return fmt.Errorf("failed to create installer: %w", err)
		}

		// Create repo manager
		manager, err := NewManagerWithLogLevel()
		if err != nil {
			return fmt.Errorf("failed to create repository manager: %w", err)
		}

		repoExists, err := repoPathExists(manager.GetRepoPath())
		if err != nil {
			return fmt.Errorf("failed to inspect repository state: %w", err)
		}
		if !repoExists {
			return fmt.Errorf("repository is not initialized at %s; run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first", manager.GetRepoPath())
		}

		repoLock, err := manager.AcquireRepoReadLock(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to acquire repository read lock at %s: %w", manager.RepoLockPath(), err)
		}
		defer func() {
			_ = repoLock.Unlock()
		}()

		// Separate packages from other resources BEFORE pattern expansion
		var packageRefs []string
		var resourceRefs []string
		for _, arg := range args {
			if strings.HasPrefix(arg, "package/") || strings.HasPrefix(arg, "packages/") {
				normalizedArg := arg
				if strings.HasPrefix(arg, "packages/") {
					normalizedArg = "package/" + strings.TrimPrefix(arg, "packages/")
				}
				packageRefs = append(packageRefs, normalizedArg)
				continue
			}

			// Expand patterns for non-package resources
			matches, err := ExpandPattern(manager, arg)
			if err != nil {
				return fmt.Errorf("failed to expand pattern '%s': %w", arg, err)
			}

			if len(matches) == 0 {
				// Pattern matched nothing - show warning
				fmt.Printf("⚠ Warning: pattern '%s' matched no resources\n", arg)
				continue
			}

			// If it was a pattern with matches, show count
			_, _, isPattern := pattern.ParsePattern(arg)
			if isPattern && len(matches) > 1 {
				fmt.Printf("Installing %d resources matching '%s'...\n", len(matches), arg)
			}

			resourceRefs = append(resourceRefs, matches...)
		}

		// Track results
		var results []installResult

		// Process packages via installPackage()
		for _, pkgRef := range packageRefs {
			packageName := strings.TrimPrefix(pkgRef, "package/")
			err := installPackage(packageName, installer, manager)
			if err != nil {
				results = append(results, installResult{
					resourceType: resource.PackageType,
					name:         packageName,
					success:      false,
					message:      err.Error(),
				})
			} else {
				results = append(results, installResult{
					resourceType: resource.PackageType,
					name:         packageName,
					success:      true,
					message:      "",
				})
			}
		}

		// Process non-package resources via processInstall()
		for _, arg := range resourceRefs {
			result := processInstall(arg, installer, manager)
			results = append(results, result)
		}

		// Update manifest for successfully installed or already-installed resources
		if err := updateManifestFromResults(projectPath, results); err != nil {
			fmt.Printf("⚠ Warning: failed to update manifest: %v\n", err)
		}

		// Print results
		printInstallSummary(results)

		// Return error if any resource failed
		for _, result := range results {
			if !result.success && !result.skipped {
				return fmt.Errorf("some resources failed to install")
			}
		}

		return nil
	},
}

// installFromManifest installs all resources from ai.package.yaml
func installFromManifest() error {
	installingFromManifest = true
	defer func() { installingFromManifest = false }()

	// Get project path
	projectPath := projectPathFlag
	if projectPath == "" {
		var err error
		projectPath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Create repo manager early to initialize logging
	manager, err := NewManagerWithLogLevel()
	if err != nil {
		return fmt.Errorf("failed to create repository manager: %w", err)
	}

	repoExists, err := repoPathExists(manager.GetRepoPath())
	if err != nil {
		return fmt.Errorf("failed to inspect repository state: %w", err)
	}
	if !repoExists {
		return fmt.Errorf("repository is not initialized at %s; run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first", manager.GetRepoPath())
	}

	repoLock, err := manager.AcquireRepoReadLock(context.Background())
	if err != nil {
		return fmt.Errorf("failed to acquire repository read lock at %s: %w", manager.RepoLockPath(), err)
	}
	defer func() {
		_ = repoLock.Unlock()
	}()

	// Load effective project manifest (base + optional local overlay)
	m, view, err := loadEffectiveProjectManifest(projectPath)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("no resources specified and neither %s nor %s found\n\nTo install resources, either:\n  1. Specify resources: aimgr install skill/pdf-processing\n  2. Create %s in current directory", manifest.ManifestFileName, manifest.LocalManifestFileName, manifest.ManifestFileName)
	}

	if view.Local != nil {
		fmt.Printf("Reading %s + %s...\n", manifest.ManifestFileName, manifest.LocalManifestFileName)
	} else {
		fmt.Printf("Reading %s...\n", manifest.ManifestFileName)
	}

	// Check if manifest has any resources
	if len(m.Resources) == 0 {
		fmt.Printf("No resources defined in %s\n", manifest.ManifestFileName)
		return nil
	}

	fmt.Printf("Installing %d resources...\n", len(m.Resources))

	// Parse target flag or use manifest targets
	var targetTools []tools.Tool
	explicitTargets, err := parseTargetFlag(installTargetFlag)
	if err != nil {
		return err
	}

	if explicitTargets != nil {
		// Use explicit --target flag
		targetTools = explicitTargets
	} else if len(m.Install.Targets) > 0 {
		// Use manifest targets
		for _, t := range m.Install.Targets {
			tool, err := tools.ParseTool(t)
			if err != nil {
				return fmt.Errorf("invalid target in manifest '%s': %w", t, err)
			}
			targetTools = append(targetTools, tool)
		}
	}

	// Create installer
	var installer *install.Installer
	if targetTools != nil {
		installer, err = install.NewInstallerWithTargets(projectPath, targetTools)
	} else {
		// Use defaults from config
		cfg, cfgErr := config.LoadGlobal()
		if cfgErr != nil {
			return fmt.Errorf("failed to load config: %w", cfgErr)
		}
		defaultTargets, tgtErr := cfg.GetDefaultTargets()
		if tgtErr != nil {
			return fmt.Errorf("invalid default targets in config: %w", tgtErr)
		}
		installer, err = install.NewInstaller(projectPath, defaultTargets)
	}
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	// Track results
	var results []installResult

	// Process each resource - expand packages first
	for _, resourceRef := range m.Resources {
		// Check if this is a package reference
		if strings.HasPrefix(resourceRef, "package/") {
			packageName := strings.TrimPrefix(resourceRef, "package/")

			// Load package
			repoPath := manager.GetRepoPath()
			pkgPath := resource.GetPackagePath(packageName, repoPath)
			pkg, err := resource.LoadPackage(pkgPath)
			if err != nil {
				// Package not found - record error
				result := installResult{
					name:    resourceRef,
					success: false,
					message: fmt.Sprintf("package '%s' not found in repository", packageName),
				}
				results = append(results, result)
				continue
			}

			fmt.Printf("Expanding package '%s' (%d resources)...\n", packageName, len(pkg.Resources))

			// Install each resource from the package
			for _, pkgResourceRef := range pkg.Resources {
				result := processInstall(pkgResourceRef, installer, manager)
				results = append(results, result)
			}
		} else {
			// Normal resource (not a package)
			result := processInstall(resourceRef, installer, manager)
			results = append(results, result)
		}
	}

	// Print results
	fmt.Println()
	printInstallSummary(results)

	// Return error if any resource failed
	for _, result := range results {
		if !result.success && !result.skipped {
			return fmt.Errorf("some resources failed to install")
		}
	}

	return nil
}

// processInstall processes installing a single resource
func processInstall(arg string, installer *install.Installer, manager *repo.Manager) installResult {
	// Parse resource argument
	resourceType, name, err := ParseResourceArg(arg)
	if err != nil {
		return installResult{
			name:    arg,
			success: false,
			message: err.Error(),
		}
	}

	result := installResult{
		resourceType: resourceType,
		name:         name,
		toolsAdded:   []tools.Tool{},
	}

	// Verify resource exists in repo
	res, err := manager.Get(name, resourceType)
	if err != nil {
		result.success = false
		result.message = fmt.Sprintf("%s '%s' not found in repository. Use 'aimgr list' to see available resources.", resourceType, name)
		return result
	}

	// Check if already installed
	if !installForceFlag && installer.IsInstalled(name, resourceType) {
		result.skipped = true
		result.message = "already installed (use --force to reinstall)"
		return result
	}

	// Remove existing if force mode
	if installForceFlag && installer.IsInstalled(name, resourceType) {
		if err := installer.Uninstall(name, resourceType, manager); err != nil {
			result.success = false
			result.message = fmt.Sprintf("failed to remove existing installation: %v", err)
			return result
		}
	}

	// Install based on resource type
	var installErr error
	switch resourceType {
	case resource.Command:
		installErr = installer.InstallCommand(name, manager)
	case resource.Skill:
		installErr = installer.InstallSkill(name, manager)
	case resource.Agent:
		installErr = installer.InstallAgent(name, manager)
	default:
		result.success = false
		result.message = fmt.Sprintf("unsupported resource type: %s", resourceType)
		return result
	}

	if installErr != nil {
		result.success = false
		result.message = fmt.Sprintf("failed to install: %v", installErr)
		return result
	}

	// Success
	result.success = true
	result.toolsAdded = installer.GetTargetTools()

	// Add description/version to message if available
	var metaParts []string
	if res.Version != "" {
		metaParts = append(metaParts, fmt.Sprintf("Version: %s", res.Version))
	}
	if res.Description != "" {
		metaParts = append(metaParts, fmt.Sprintf("Description: %s", res.Description))
	}
	if len(metaParts) > 0 {
		result.message = strings.Join(metaParts, ", ")
	}

	return result
}

// printInstallSummary prints a summary of install results
func printInstallSummary(results []installResult) {
	successCount := 0
	skipCount := 0
	failCount := 0

	for _, result := range results {
		if result.success {
			successCount++
			// Print success
			fmt.Printf("✓ Installed %s '%s'\n", result.resourceType, result.name)
			for _, tool := range result.toolsAdded {
				toolInfo := tools.GetToolInfo(tool)
				var installPath string
				switch result.resourceType {
				case resource.Command:
					if toolInfo.SupportsCommands {
						installPath = fmt.Sprintf("%s/%s.md", toolInfo.CommandsDir, result.name)
					}
				case resource.Skill:
					if toolInfo.SupportsSkills {
						installPath = fmt.Sprintf("%s/%s", toolInfo.SkillsDir, result.name)
					}
				case resource.Agent:
					if toolInfo.SupportsAgents {
						installPath = fmt.Sprintf("%s/%s", toolInfo.AgentsDir, tools.AgentArtifactName(tool, result.name))
					}
				}
				if installPath != "" {
					fmt.Printf("  → %s\n", installPath)
				}
			}
			if result.message != "" {
				fmt.Printf("  %s\n", result.message)
			}
		} else if result.skipped {
			skipCount++
			// Print skipped
			fmt.Printf("⊘ Skipped %s '%s': %s\n", result.resourceType, result.name, result.message)
		} else {
			failCount++
			// Print failure
			if result.resourceType != "" {
				fmt.Printf("✗ Failed to install %s '%s': %s\n", result.resourceType, result.name, result.message)
			} else {
				fmt.Printf("✗ Failed: %s\n", result.message)
			}
		}
	}

	// Print summary
	fmt.Println()
	fmt.Printf("Summary: %d installed, %d skipped, %d failed\n", successCount, skipCount, failCount)
}

func init() {
	rootCmd.AddCommand(installCmd)

	// Add flags to install command
	installCmd.Flags().StringVar(&projectPathFlag, "project-path", "", "Project directory path (default: current directory)")
	installCmd.Flags().BoolVarP(&installForceFlag, "force", "f", false, "Overwrite existing installation")
	installCmd.Flags().StringVar(&installTargetFlag, "target", "", "Target tools (comma-separated: claude,opencode,copilot)")
	installCmd.Flags().BoolVar(&installSaveFlag, "save", true, "Save installed resources to ai.package.yaml")
	installCmd.Flags().BoolVar(&installNoSaveFlag, "no-save", false, "Don't save to ai.package.yaml")
	// Register completion for --target flag
	_ = installCmd.RegisterFlagCompletionFunc("target", completeToolNames)
}

// installPackage installs all resources from a package
func installPackage(packageName string, installer *install.Installer, manager *repo.Manager) error {
	return installPackageWithWriter(packageName, installer, manager, os.Stdout)
}

// installPackageWithWriter installs all resources from a package definition,
// writing progress output to the given writer. Use os.Stdout for interactive
// use and os.Stderr (or io.Discard) when stdout must contain only structured output.
func installPackageWithWriter(packageName string, installer *install.Installer, manager *repo.Manager, w io.Writer) error {
	repoPath := manager.GetRepoPath()
	pkgPath := resource.GetPackagePath(packageName, repoPath)

	// Load package
	pkg, err := resource.LoadPackage(pkgPath)
	if err != nil {
		return fmt.Errorf("package '%s' not found in repository: %w", packageName, err)
	}

	fmt.Fprintf(w, "Installing package: %s\n", pkg.Name)
	fmt.Fprintf(w, "Description: %s\n\n", pkg.Description)

	installed := 0
	missing := 0
	errors := []string{}

	// Install each resource
	for _, ref := range pkg.Resources {
		// Parse type/name format
		resType, resName, err := resource.ParseResourceReference(ref)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", ref, err))
			continue
		}

		// Check if resource exists in repo
		_, err = manager.Get(resName, resType)
		if err != nil {
			fmt.Fprintf(w, "  ✗ %s - not found in repo\n", ref)
			missing++
			continue
		}

		// Check if already installed
		if !installForceFlag && installer.IsInstalled(resName, resType) {
			fmt.Fprintf(w, "  ○ %s - already installed, skipping\n", ref)
			continue
		}

		// Remove existing if force mode
		if installForceFlag && installer.IsInstalled(resName, resType) {
			if err := installer.Uninstall(resName, resType, manager); err != nil {
				errors = append(errors, fmt.Sprintf("%s: failed to remove existing: %v", ref, err))
				continue
			}
		}

		// Install based on resource type
		var installErr error
		switch resType {
		case resource.Command:
			installErr = installer.InstallCommand(resName, manager)
		case resource.Skill:
			installErr = installer.InstallSkill(resName, manager)
		case resource.Agent:
			installErr = installer.InstallAgent(resName, manager)
		default:
			errors = append(errors, fmt.Sprintf("%s: unsupported resource type", ref))
			continue
		}

		if installErr != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", ref, installErr))
		} else {
			fmt.Fprintf(w, "  ✓ %s\n", ref)
			installed++
		}
	}

	// Print summary
	fmt.Fprintln(w)
	if missing > 0 {
		fmt.Fprintf(w, "⚠ Warning: %d resource(s) not found in repo\n", missing)
	}
	if len(errors) > 0 {
		fmt.Fprintln(w, "Errors:")
		for _, e := range errors {
			fmt.Fprintf(w, "  ✗ %s\n", e)
		}
	}

	totalResources := len(pkg.Resources)
	fmt.Fprintf(w, "Installed %d of %d resources from package '%s'\n", installed, totalResources, pkg.Name)

	if len(errors) > 0 {
		return fmt.Errorf("package installation completed with errors")
	}

	return nil

}

// updateManifest adds a resource to ai.package.yaml after successful installation
func updateManifest(projectPath string, resource string) error {
	// Skip if --no-save is set, or if not saving, or if installing from manifest
	if installNoSaveFlag || !installSaveFlag || installingFromManifest {
		return nil
	}

	manifestPath := filepath.Join(projectPath, manifest.ManifestFileName)

	// Load or create manifest
	m, err := manifest.LoadOrCreate(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load/create manifest: %w", err)
	}

	// Add resource (will skip if already exists)
	if err := m.Add(resource); err != nil {
		return fmt.Errorf("failed to add resource to manifest: %w", err)
	}

	// Save manifest
	if err := m.Save(manifestPath); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	fmt.Printf("✓ Added to %s\n", manifest.ManifestFileName)
	return nil
}

// updateManifestFromResults batches ai.package.yaml updates from install results.
// Only successful and skipped resources are added.
func updateManifestFromResults(projectPath string, results []installResult) error {
	// Skip if --no-save is set, or if not saving, or if installing from manifest
	if installNoSaveFlag || !installSaveFlag || installingFromManifest {
		return nil
	}

	resources := make([]string, 0, len(results))
	for _, result := range results {
		if !result.success && !result.skipped {
			continue
		}
		if result.resourceType == "" || result.name == "" {
			continue
		}
		resources = append(resources, fmt.Sprintf("%s/%s", result.resourceType, result.name))
	}

	if len(resources) == 0 {
		return nil
	}

	manifestPath := filepath.Join(projectPath, manifest.ManifestFileName)

	// Load or create manifest once
	m, err := manifest.LoadOrCreate(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load/create manifest: %w", err)
	}

	for _, resourceRef := range resources {
		if err := m.Add(resourceRef); err != nil {
			return fmt.Errorf("failed to add resource to manifest: %w", err)
		}
	}

	// Save manifest once
	if err := m.Save(manifestPath); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	fmt.Printf("✓ Added to %s\n", manifest.ManifestFileName)
	return nil
}
