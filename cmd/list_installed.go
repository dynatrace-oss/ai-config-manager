package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/install"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	listInstalledFormatFlag string
	listInstalledPathFlag   string
)

// SyncStatus represents the synchronization state between installed resources and ai.package.yaml manifest
type SyncStatus string

const (
	// SyncStatusInSync means the resource is both in the manifest and installed
	SyncStatusInSync SyncStatus = "in-sync"
	// SyncStatusNotInManifest means the resource is installed but not in the manifest
	SyncStatusNotInManifest SyncStatus = "not-in-manifest"
	// SyncStatusNotInstalled means the resource is in the manifest but not installed
	SyncStatusNotInstalled SyncStatus = "not-installed"
	// SyncStatusNoManifest means no ai.package.yaml file exists in the project
	SyncStatusNoManifest SyncStatus = "no-manifest"
)

// listInstalledCmd represents the list command for installed resources
var listInstalledCmd = &cobra.Command{
	Use:   "list [pattern]",
	Short: "List installed resources in the current directory",
	Long: `List all resources installed in the current directory (or specified path).

This command shows resources that were installed using 'aimgr install',
displaying which tools (claude, opencode, copilot) each resource is installed to
and their synchronization status with ai.package.yaml.

Output columns:
  - NAME: Resource reference (e.g., skill/pdf-processing, command/test)
  - TARGETS: Tools where the resource is installed (e.g., claude, opencode)
  - SYNC: Synchronization status with ai.package.yaml manifest
  - DESCRIPTION: Brief description of the resource

Sync Status Symbols:
  ✓ = In sync (resource is both in manifest and installed)
  * = Not in ai.package.yaml (installed but not declared in ai.package.yaml)
  ⚠ = Not installed (declared in manifest but not installed yet)
  - = No ai.package.yaml (no ai.package.yaml file exists in current directory)

The sync status helps you keep your installations aligned with your project's
ai.package.yaml manifest file. Resources marked with * should be added to the
manifest if you want to track them. Resources marked with ⚠ need to be installed
using 'aimgr install <resource>'.

Only resources installed via aimgr (symlinks) are shown - manually copied files are excluded.

The optional pattern argument supports glob wildcards (* ? [ ]) and type prefixes:
  - No pattern: list all installed resources
  - Type patterns: skill/*, command/*, agent/*
  - Name patterns: *test*, *debug*, pdf*
  - Combined: skill/pdf*, command/test-*

Examples:
  aimgr list                         # List all installed resources with sync status
  aimgr list skill/*                 # List all installed skills
  aimgr list command/*               # List all installed commands
  aimgr list agent/*                 # List all installed agents
  aimgr list *test*                  # List all resources with "test" in name
  aimgr list skill/pdf*              # List installed skills starting with "pdf"
  aimgr list command/test-*          # List installed commands starting with "test-"
  aimgr list --format=json           # Output as JSON with sync_status field
  aimgr list --format=yaml           # Output as YAML
  aimgr list --path ~/project        # List in specific directory`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeInstalledResources,
	RunE: func(cmd *cobra.Command, args []string) error {

		// Get project path (current directory or flag)
		projectPath := listInstalledPathFlag
		if projectPath == "" {
			var err error
			projectPath, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Detect which tools exist in the project
		detectedTools, err := tools.DetectExistingTools(projectPath)
		if err != nil {
			return fmt.Errorf("failed to detect tools: %w", err)
		}

		if len(detectedTools) == 0 {
			fmt.Println("No tool directories found in this project.")
			fmt.Println("\nExpected directories: .claude, .opencode, .github/skills, or .github/agents")
			fmt.Println("Install resources with: aimgr install <resource>")
			return nil
		}

		// Create installer to list resources
		installer, err := install.NewInstallerWithTargets(projectPath, detectedTools)
		if err != nil {
			return fmt.Errorf("failed to create installer: %w", err)
		}

		// List all installed resources
		resources, err := installer.List()
		if err != nil {
			return fmt.Errorf("failed to list installed resources: %w", err)
		}

		// Inject package entries from the manifest (packages have no symlinks,
		// so installer.List() never returns them — we add them manually here).
		resources = appendManifestPackages(resources, projectPath)

		// Filter by pattern if specified
		if len(args) > 0 {
			patternArg := args[0]
			matcher, err := pattern.NewMatcher(patternArg)
			if err != nil {
				return fmt.Errorf("invalid pattern '%s': %w", patternArg, err)
			}

			filtered := []resource.Resource{}
			for _, res := range resources {
				if matcher.Match(&res) {
					filtered = append(filtered, res)
				}
			}
			resources = filtered

			// Handle no matches
			if len(resources) == 0 {
				fmt.Printf("No installed resources match pattern '%s'.\n", patternArg)
				fmt.Println("\nInstall resources with: aimgr install <resource>")
				return nil
			}
		}

		// Handle empty results (no pattern specified)
		if len(resources) == 0 {
			fmt.Println("No resources installed in this project.")
			fmt.Println("\nInstall resources with: aimgr install <resource>")
			return nil
		}

		// Get tool installation info for each resource
		resourceInfos := buildResourceInfo(resources, projectPath, detectedTools)

		// Format output based on --format flag
		switch listInstalledFormatFlag {
		case "json":
			return outputInstalledJSON(resourceInfos)
		case "yaml":
			return outputInstalledYAML(resourceInfos)
		case "table":
			return outputInstalledTable(resourceInfos, projectPath)
		default:
			return fmt.Errorf("invalid format: %s (must be 'table', 'json', or 'yaml')", listInstalledFormatFlag)
		}
	},
}

// ResourceInfo extends Resource with installation target information and sync status
type ResourceInfo struct {
	Type        resource.ResourceType `json:"type" yaml:"type"`
	Name        string                `json:"name" yaml:"name"`
	Description string                `json:"description" yaml:"description"`
	Version     string                `json:"version,omitempty" yaml:"version,omitempty"`
	Targets     []string              `json:"targets" yaml:"targets"`
	SyncStatus  string                `json:"sync_status" yaml:"sync_status"`
	Health      string                `json:"health" yaml:"health"`
}

// expandManifestResources returns a set of all resource references in the manifest,
// including individual resources that are members of declared packages.
// Returns nil if no manifest exists or cannot be loaded (signals "no manifest").
func expandManifestResources(projectPath string) map[string]bool {
	manifestPath := filepath.Join(projectPath, manifest.ManifestFileName)
	m, err := manifest.Load(manifestPath)
	if err != nil {
		return nil // No manifest or error — return nil to signal "no manifest"
	}

	expanded := make(map[string]bool)

	// Try to get the repo manager for package expansion
	manager, managerErr := NewManagerWithLogLevel()

	for _, ref := range m.Resources {
		expanded[ref] = true

		// If this is a package reference, expand its members
		if strings.HasPrefix(ref, "package/") && managerErr == nil {
			packageName := strings.TrimPrefix(ref, "package/")
			repoPath := manager.GetRepoPath()
			pkgPath := resource.GetPackagePath(packageName, repoPath)
			pkg, err := resource.LoadPackage(pkgPath)
			if err != nil {
				continue // Package not found in repo, skip expansion
			}
			for _, memberRef := range pkg.Resources {
				expanded[memberRef] = true
			}
		}
	}

	return expanded
}

// appendManifestPackages reads ai.package.yaml and appends package/ entries as Resource objects
// to the given slice. Packages have no symlinks, so installer.List() never finds them;
// this function makes them visible to the rest of the list pipeline (pattern filtering,
// buildResourceInfo, output formatters).
//
// Errors (no manifest, repo unavailable, package not found in repo) are silently skipped
// so that missing configuration never breaks the overall list command.
func appendManifestPackages(resources []resource.Resource, projectPath string) []resource.Resource {
	manifestPath := filepath.Join(projectPath, manifest.ManifestFileName)
	m, err := manifest.Load(manifestPath)
	if err != nil {
		return resources // No manifest — nothing to inject
	}

	// Try to get the repo manager so we can load package descriptions
	manager, managerErr := NewManagerWithLogLevel()

	for _, ref := range m.Resources {
		if !strings.HasPrefix(ref, "package/") {
			continue
		}
		packageName := strings.TrimPrefix(ref, "package/")

		res := resource.Resource{
			Type:   resource.PackageType,
			Name:   packageName,
			Health: resource.HealthOK,
		}

		// Load description (and resource list) from the repo if available
		if managerErr == nil {
			repoPath := manager.GetRepoPath()
			pkgPath := resource.GetPackagePath(packageName, repoPath)
			if pkg, loadErr := resource.LoadPackage(pkgPath); loadErr == nil {
				res.Description = pkg.Description
			}
		}

		resources = append(resources, res)
	}

	return resources
}

// getSyncStatus determines the sync status of a resource relative to ai.package.yaml.
// expandedManifest is a pre-computed set of all resource refs (including package members).
// Pass nil to indicate no manifest exists.
func getSyncStatus(projectPath string, resourceRef string, isInstalled bool, expandedManifest ...map[string]bool) SyncStatus {
	// If an expanded manifest was provided, use it directly
	var manifestSet map[string]bool
	if len(expandedManifest) > 0 {
		manifestSet = expandedManifest[0]
	} else {
		// Fallback: compute expanded manifest on the fly (for backward compatibility with tests)
		manifestSet = expandManifestResources(projectPath)
	}

	if manifestSet == nil {
		return SyncStatusNoManifest
	}

	// Check if the resource reference exists in the expanded manifest
	inManifest := manifestSet[resourceRef]

	// Determine sync status based on both installation and manifest presence
	if inManifest && isInstalled {
		return SyncStatusInSync
	} else if inManifest && !isInstalled {
		return SyncStatusNotInstalled
	} else if !inManifest && isInstalled {
		return SyncStatusNotInManifest
	}

	// This case shouldn't normally occur (not in manifest and not installed)
	// But we handle it by treating it as not-in-manifest
	return SyncStatusNotInManifest
}

// formatSyncStatus gets the sync status and returns the appropriate symbol
func formatSyncStatus(projectPath string, resourceRef string, isInstalled bool, expandedManifest map[string]bool) string {
	status := getSyncStatus(projectPath, resourceRef, isInstalled, expandedManifest)

	switch status {
	case SyncStatusInSync:
		return statusIconOK
	case SyncStatusNotInManifest:
		return "*"
	case SyncStatusNotInstalled:
		return "⚠"
	case SyncStatusNoManifest:
		return "-"
	default:
		return "-"
	}
}

// buildResourceInfo creates ResourceInfo entries with target tool information and sync status
func buildResourceInfo(resources []resource.Resource, projectPath string, detectedTools []tools.Tool) []ResourceInfo {
	// Pre-compute expanded manifest (includes package member resources)
	expandedManifest := expandManifestResources(projectPath)

	infos := make([]ResourceInfo, 0, len(resources))

	for _, res := range resources {
		health := string(resource.HealthOK)
		if res.Health == resource.HealthBroken {
			health = string(resource.HealthBroken)
		}

		info := ResourceInfo{
			Type:        res.Type,
			Name:        res.Name,
			Description: res.Description,
			Version:     res.Version,
			Targets:     []string{},
			Health:      health,
		}

		// Check which tools have this resource installed
		// For broken resources, isInstalledInTool uses os.Lstat which detects broken symlinks
		for _, tool := range detectedTools {
			if isInstalledInTool(projectPath, res.Name, res.Type, tool) {
				info.Targets = append(info.Targets, tool.String())
			}
		}

		// Determine if resource is installed (has at least one target)
		// Packages have no symlinks, so their Targets will always be empty.
		// They are considered "installed" by virtue of being in the manifest.
		isInstalled := len(info.Targets) > 0
		if res.Type == resource.PackageType {
			isInstalled = true
		}

		// Build resource reference for manifest lookup
		resourceRef := fmt.Sprintf("%s/%s", res.Type, res.Name)

		// Get sync status
		syncStatus := getSyncStatus(projectPath, resourceRef, isInstalled, expandedManifest)
		info.SyncStatus = string(syncStatus)

		infos = append(infos, info)
	}

	return infos
}

// isInstalledInTool checks if a resource is installed in a specific tool directory
func isInstalledInTool(projectPath, name string, resType resource.ResourceType, tool tools.Tool) bool {
	toolInfo := tools.GetToolInfo(tool)
	var checkPath string

	switch resType {
	case resource.Command:
		if !toolInfo.SupportsCommands {
			return false
		}
		checkPath = filepath.Join(projectPath, toolInfo.CommandsDir, name+".md")
	case resource.Skill:
		if !toolInfo.SupportsSkills {
			return false
		}
		checkPath = filepath.Join(projectPath, toolInfo.SkillsDir, name)
	case resource.Agent:
		if !toolInfo.SupportsAgents {
			return false
		}
		checkPath = filepath.Join(projectPath, toolInfo.AgentsDir, tools.AgentArtifactName(tool, name))
	default:
		return false
	}

	// Check if symlink exists
	info, err := os.Lstat(checkPath)
	if err != nil {
		return false
	}

	// Verify it's a symlink (only count aimgr-managed installations)
	return info.Mode()&os.ModeSymlink != 0
}

func outputInstalledTable(infos []ResourceInfo, projectPath string) error {
	// Group by type
	commands := []ResourceInfo{}
	skills := []ResourceInfo{}
	agents := []ResourceInfo{}
	packages := []ResourceInfo{}

	for _, info := range infos {
		switch info.Type {
		case resource.Command:
			commands = append(commands, info)
		case resource.Skill:
			skills = append(skills, info)
		case resource.Agent:
			agents = append(agents, info)
		case resource.PackageType:
			packages = append(packages, info)
		}
	}

	// Pre-compute expanded manifest for sync status display
	expandedManifest := expandManifestResources(projectPath)

	// Create table with NAME, TARGETS, SYNC, STATUS, DESCRIPTION using shared infrastructure
	table := output.NewTable("Name", "Targets", "Sync", "Status", "Description")
	table.WithResponsive().
		WithDynamicColumn(4).                 // Description stretches
		WithMinColumnWidths(40, 12, 4, 8, 30) // Name min=40, Targets min=12, Sync min=4, Status min=8, Description min=30

	// Add commands
	for _, cmd := range commands {
		targets := strings.Join(cmd.Targets, ", ")
		resourceRef := fmt.Sprintf("command/%s", cmd.Name)
		syncSymbol := formatSyncStatus(projectPath, resourceRef, len(cmd.Targets) > 0, expandedManifest)
		status := statusIconOK
		if cmd.Health == string(resource.HealthBroken) {
			status = statusIconFail
		}
		table.AddRow(resourceRef, targets, syncSymbol, status, cmd.Description)
	}

	// Add empty row between types if commands exist and skills or agents exist
	if len(commands) > 0 && (len(skills) > 0 || len(agents) > 0) {
		table.AddSeparator()
	}

	// Add skills
	for _, skill := range skills {
		targets := strings.Join(skill.Targets, ", ")
		resourceRef := fmt.Sprintf("skill/%s", skill.Name)
		syncSymbol := formatSyncStatus(projectPath, resourceRef, len(skill.Targets) > 0, expandedManifest)
		status := statusIconOK
		if skill.Health == string(resource.HealthBroken) {
			status = statusIconFail
		}
		table.AddRow(resourceRef, targets, syncSymbol, status, skill.Description)
	}

	// Add empty row between types if skills exist and agents exist
	if len(skills) > 0 && len(agents) > 0 {
		table.AddSeparator()
	}

	// Add agents
	for _, agent := range agents {
		targets := strings.Join(agent.Targets, ", ")
		resourceRef := fmt.Sprintf("agent/%s", agent.Name)
		syncSymbol := formatSyncStatus(projectPath, resourceRef, len(agent.Targets) > 0, expandedManifest)
		status := statusIconOK
		if agent.Health == string(resource.HealthBroken) {
			status = statusIconFail
		}
		table.AddRow(resourceRef, targets, syncSymbol, status, agent.Description)
	}

	// Add separator before packages if any prior groups exist
	if len(packages) > 0 && (len(commands) > 0 || len(skills) > 0 || len(agents) > 0) {
		table.AddSeparator()
	}

	// Add packages — load repo manager once to resolve resource counts
	var pkgManager interface{ GetRepoPath() string }
	if m, err := NewManagerWithLogLevel(); err == nil {
		pkgManager = m
	}

	for _, pkg := range packages {
		resourceRef := fmt.Sprintf("package/%s", pkg.Name)
		syncSymbol := formatSyncStatus(projectPath, resourceRef, true, expandedManifest)

		// TARGETS column: show resource count from the package definition
		targets := "-"
		if pkgManager != nil {
			repoPath := pkgManager.GetRepoPath()
			pkgPath := resource.GetPackagePath(pkg.Name, repoPath)
			if pkgDef, loadErr := resource.LoadPackage(pkgPath); loadErr == nil {
				targets = fmt.Sprintf("%d resources", len(pkgDef.Resources))
			}
		}

		// Packages always show ✓ for STATUS (no symlink health applies)
		table.AddRow(resourceRef, targets, syncSymbol, statusIconOK, pkg.Description)
	}

	// Render the table
	if err := table.Format(output.Table); err != nil {
		return err
	}

	// Print legend
	fmt.Println("\nLegend:")
	fmt.Println("  ✓ = In sync  * = Not in ai.package.yaml  ⚠ = Not installed  - = No ai.package.yaml")

	// Count broken resources and print warning to stderr
	brokenCount := 0
	for _, info := range infos {
		if info.Health == string(resource.HealthBroken) {
			brokenCount++
		}
	}
	if brokenCount > 0 {
		fmt.Fprintf(os.Stderr, "\n⚠ %d broken resource(s) found. Run 'aimgr repair' to fix.\n", brokenCount)
	}

	return nil
}

func outputInstalledJSON(infos []ResourceInfo) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(infos)
}

func outputInstalledYAML(infos []ResourceInfo) error {
	encoder := yaml.NewEncoder(os.Stdout)
	defer func() { _ = encoder.Close() }()
	return encoder.Encode(infos)
}

func init() {
	rootCmd.AddCommand(listInstalledCmd)
	listInstalledCmd.Flags().StringVar(&listInstalledFormatFlag, "format", "table", "Output format (table|json|yaml)")
	listInstalledCmd.Flags().StringVar(&listInstalledPathFlag, "path", "", "Project directory path (default: current directory)")
	_ = listInstalledCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)
}
