package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/marketplace"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/source"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/sourcemetadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/workspace"
	"github.com/spf13/cobra"
)

var forceFlag bool

var (
	skipExistingFlag bool
	dryRunFlag       bool
	filterFlags      []string
	addFormatFlag    string
	nameFlag         string
)

// repoAddCmd represents the add command
var repoAddCmd = &cobra.Command{
	Use:   "add <source>",
	Short: "Add resources to the repository",
	Long: `Add resources to the aimgr repository.

This command auto-discovers and adds all resources (commands, skills, agents, packages)
from the specified source location. It also automatically detects and adds marketplace.json
files if present.

Source Formats:
  GitHub (shorthand):
    gh:owner/repo              GitHub repository
    gh:owner/repo@v1.0.0       Specific branch or tag
    gh:owner/repo/subpath       Subdirectory within repo
    gh:owner/repo@ref/subpath   Ref and subdirectory combined

  HTTPS / HTTP (any Git host):
    https://github.com/owner/repo        GitHub
    https://gitlab.com/owner/repo        GitLab
    https://bitbucket.org/owner/repo     Bitbucket
    http://git.internal.com/owner/repo   Self-hosted

  HTTPS with subpath (via .git/ delimiter):
    https://host/path/repo.git/subpath   Clone repo, use subpath
    (Everything before .git/ is the clone URL; after is the subpath)

  SSH (any Git host):
    git@github.com:owner/repo.git        GitHub SSH
    git@gitlab.com:owner/repo.git        GitLab SSH
    git@bitbucket.org:owner/repo.git     Bitbucket SSH

  Local directory:
    local:./relative/path      Relative path (symlinked)
    local:/absolute/path       Absolute path (symlinked)
    local:~/home/path          Home-relative path (symlinked)

Commands are single .md files with YAML frontmatter.
Skills are directories containing a SKILL.md file.
Agents are single .md files with YAML frontmatter.
Packages are .package.json files in the packages/ directory.
Marketplace files (marketplace.json) are automatically discovered and converted to packages.

Examples:
  # Add from GitHub (shorthand)
  aimgr repo add gh:owner/repo
  aimgr repo add gh:owner/repo@v1.0.0
  aimgr repo add gh:owner/repo --filter "skill/*"

  # Add from any Git host (HTTPS)
  aimgr repo add https://github.com/owner/repo
  aimgr repo add https://bitbucket.org/org/repo
  aimgr repo add https://gitlab.com/group/project

  # HTTPS with subpath (non-GitHub hosts)
  aimgr repo add https://bitbucket.example.com/scm/TEAM/repo.git/skills
  aimgr repo add https://gitlab.internal.com/group/repo.git/packages/ai

  # Add from any Git host (SSH)
  aimgr repo add git@github.com:owner/repo.git
  aimgr repo add git@bitbucket.org:org/repo.git

  # Add from local directory
  aimgr repo add local:~/.opencode/
  aimgr repo add local:./my-resources/

  # With options
  aimgr repo add gh:owner/repo --force
  aimgr repo add gh:owner/repo --skip-existing
  aimgr repo add gh:owner/repo --dry-run
  aimgr repo add gh:owner/repo --name=my-custom-name

  # Filter resources to import (repeatable, OR logic):
  aimgr repo add gh:owner/repo --filter skill/pdf --filter skill/docx
  aimgr repo add gh:owner/repo --filter 'skill/*'
  aimgr repo add local:./my-resources --filter skill/test-util --filter 'command/*'

  Filters are persisted in ai.repo.yaml (source.include) and respected by repo sync.
  Multiple --filter flags may be specified; a resource is imported if ANY pattern matches.
  Re-adding an existing source with --filter replaces its include list; without --filter clears it.`,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf(`missing source argument

Source formats:
  gh:owner/repo                                GitHub shorthand
  https://host/owner/repo                      HTTPS (any Git host)
  https://host/path/repo.git/subpath           HTTPS with subpath
  git@host:owner/repo.git                      SSH
  local:./path                                 Local directory

Examples:
  aimgr repo add gh:my-org/ai-tools
  aimgr repo add https://bitbucket.example.com/scm/TEAM/repo.git/skills
  aimgr repo add local:./my-resources

Run 'aimgr repo add --help' for full documentation`)
		}
		if len(args) > 1 {
			return fmt.Errorf("accepts 1 arg, received %d", len(args))
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceInput := args[0]

		// Parse source
		parsed, err := source.ParseSource(sourceInput)
		if err != nil {
			return err
		}

		// Create manager
		manager, err := NewManagerWithLogLevel()
		if err != nil {
			return err
		}

		// Auto-detect: URL or local path?
		isRemote := parsed.Type == source.GitHub || parsed.Type == source.GitURL

		var addErr error
		var importMode string

		if isRemote {
			// Remote source: always use copy mode
			importMode = "copy"
			addErr = addBulkFromGitHub(parsed, manager)
		} else {
			// Local source: always use symlink mode
			importMode = "symlink"

			// Compute source ID from absolute path before import
			absPath, pathErr := filepath.Abs(parsed.LocalPath)
			if pathErr != nil {
				return fmt.Errorf("failed to get absolute path: %w", pathErr)
			}
			tempSource := repomanifest.Source{Path: absPath}
			sourceID := repomanifest.GenerateSourceID(&tempSource)

			addErr = addBulkFromLocalWithMode(parsed.LocalPath, manager, filterFlags, sourceID, importMode, "")
		}

		// If add operation failed or in dry-run mode, return early
		if addErr != nil || dryRunFlag {
			return addErr
		}

		// Add source to manifest after successful add
		if err := addSourceToManifest(manager, parsed, filterFlags); err != nil {
			// Don't fail the entire operation if manifest tracking fails
			fmt.Fprintf(os.Stderr, "Warning: Failed to track source in manifest: %v\n", err)
		} else {
			// Commit manifest changes to git
			if err := manager.CommitChanges("aimgr: track source in manifest"); err != nil {
				// Don't fail if commit fails (e.g., not a git repo)
				fmt.Fprintf(os.Stderr, "Warning: Failed to commit manifest: %v\n", err)
			}
		}

		return nil
	},
}

func init() {
	repoCmd.AddCommand(repoAddCmd)

	// Add flags
	repoAddCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Overwrite existing resources")
	repoAddCmd.Flags().BoolVar(&skipExistingFlag, "skip-existing", false, "Skip conflicts silently")
	repoAddCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Preview without adding")
	repoAddCmd.Flags().StringArrayVar(&filterFlags, "filter", nil, "Filter resources by pattern, repeatable (e.g., --filter skill/pdf --filter 'skill/web*')")
	repoAddCmd.Flags().StringVar(&addFormatFlag, "format", "table", "Output format: table, json, yaml")
	repoAddCmd.Flags().StringVar(&nameFlag, "name", "", "Override auto-generated source name")
	_ = repoAddCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)
}

// Helper functions for bulk add integration

// addSingleResource adds a single file (command or agent) to the repository
func addSingleResource(filePath string, manager *repo.Manager) error {
	// Validate it's a .md file
	if filepath.Ext(filePath) != ".md" {
		return fmt.Errorf("file must have .md extension: %s", filePath)
	}

	// Check parent directory to determine type
	parentDir := filepath.Base(filepath.Dir(filePath))

	// If in agents/ directory, treat as agent
	if parentDir == "agents" {
		agent, err := resource.LoadAgent(filePath)
		if err != nil {
			return fmt.Errorf("invalid agent resource: %w", err)
		}
		return addResourceFile(filePath, agent, resource.Agent, manager)
	}

	// If in commands/ directory, treat as command
	if parentDir == "commands" {
		cmd, err := resource.LoadCommand(filePath)
		if err != nil {
			return fmt.Errorf("invalid command resource: %w", err)
		}
		return addResourceFile(filePath, cmd, resource.Command, manager)
	}

	// Otherwise, try to determine by content
	// Try as agent first (agents have more specific fields)
	agent, agentErr := resource.LoadAgent(filePath)
	if agentErr == nil {
		// Check if it has agent-specific fields (type, instructions, capabilities)
		agentRes, err := resource.LoadAgentResource(filePath)
		if err == nil && (agentRes.Type != "" || agentRes.Instructions != "" || len(agentRes.Capabilities) > 0) {
			// Has agent-specific fields, treat as agent
			return addResourceFile(filePath, agent, resource.Agent, manager)
		}
	}

	// Try as command
	cmd, cmdErr := resource.LoadCommand(filePath)
	if cmdErr == nil {
		// It's a command
		return addResourceFile(filePath, cmd, resource.Command, manager)
	}

	// Neither worked, return both errors
	return fmt.Errorf("invalid resource file (tried as agent: %v; tried as command: %v)", agentErr, cmdErr)
}

// addResourceFile adds a single resource file to the repository.
// This is a generic function that handles commands, agents, and skills.
func addResourceFile(filePath string, res *resource.Resource, resType resource.ResourceType, manager *repo.Manager) error {
	// Check if already exists (if not force mode)
	if !forceFlag {
		existing, _ := manager.Get(res.Name, resType)
		if existing != nil {
			return fmt.Errorf("%s '%s' already exists in repository (use --force to overwrite)", resType, res.Name)
		}
	} else {
		// Remove existing if force mode
		_ = manager.Remove(res.Name, resType)
	}

	// Determine source info
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	sourceURL := "file://" + absPath

	// Add the resource based on type
	switch resType {
	case resource.Command:
		if err := manager.AddCommand(filePath, sourceURL, "file"); err != nil {
			return fmt.Errorf("failed to add command: %w", err)
		}
	case resource.Agent:
		if err := manager.AddAgent(filePath, sourceURL, "file"); err != nil {
			return fmt.Errorf("failed to add agent: %w", err)
		}
	case resource.Skill:
		if err := manager.AddSkill(filePath, sourceURL, "file"); err != nil {
			return fmt.Errorf("failed to add skill: %w", err)
		}
	default:
		return fmt.Errorf("unsupported resource type: %s", resType)
	}

	// Success message
	fmt.Printf("✓ Added %s '%s' to repository\n", resType, res.Name)
	if res.Description != "" {
		fmt.Printf("  Description: %s\n", res.Description)
	}

	return nil
}

// applyFilter filters discovered resources based on one or more patterns (OR logic).
// Returns filtered slices. Empty/nil filterPatterns returns all resources unchanged.
//
//nolint:gocyclo // Orchestrates pattern matching across 4 resource types with type-specific matching logic; splitting would scatter filter semantics.
func applyFilter(filterPatterns []string, commands, skills, agents []*resource.Resource, packages []*resource.Package) ([]*resource.Resource, []*resource.Resource, []*resource.Resource, []*resource.Package, error) {
	if len(filterPatterns) == 0 {
		// No filter, return all resources
		return commands, skills, agents, packages, nil
	}

	// Build multi-matcher (OR logic across all patterns)
	mm, err := pattern.NewMultiMatcher(filterPatterns)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("invalid filter pattern: %w", err)
	}

	var filteredCommands []*resource.Resource
	var filteredSkills []*resource.Resource
	var filteredAgents []*resource.Resource
	var filteredPackages []*resource.Package

	for _, cmd := range commands {
		if mm.Match(cmd) {
			filteredCommands = append(filteredCommands, cmd)
		}
	}

	for _, skill := range skills {
		if mm.Match(skill) {
			filteredSkills = append(filteredSkills, skill)
		}
	}

	for _, agent := range agents {
		if mm.Match(agent) {
			filteredAgents = append(filteredAgents, agent)
		}
	}

	for _, pkg := range packages {
		// Match package on its full reference "package/<name>" and plain name
		pkgRes := &resource.Resource{Type: resource.PackageType, Name: pkg.Name}
		if mm.Match(pkgRes) {
			filteredPackages = append(filteredPackages, pkg)
		}
	}

	return filteredCommands, filteredSkills, filteredAgents, filteredPackages, nil
}

// importFromLocalPath performs the core import logic from a local directory.
// This function is used by both local imports and remote imports (after cloning to workspace).
// It discovers resources, applies filters, and imports them into the repository.
// For backward compatibility, this defaults to "copy" mode.
func importFromLocalPath(
	localPath string, // Local directory to import from
	manager *repo.Manager, // Repository manager
	filter []string, // Optional filter patterns (nil/empty = no filter)
	sourceURL string, // Source URL for metadata tracking
	sourceType string, // "local", "github", "git-url", "test"
	ref string, // Git ref (empty for local/test)
) error {
	return importFromLocalPathWithMode(localPath, manager, filter, sourceURL, sourceType, ref, "copy", "", "")
}

// importFromLocalPathWithMode is the same as importFromLocalPath but allows specifying import mode.
//
//nolint:gocyclo // Multi-phase import pipeline (discover, filter, import, metadata, git) that must stay cohesive for correctness.
func importFromLocalPathWithMode(
	localPath string, // Local directory to import from
	manager *repo.Manager, // Repository manager
	filter []string, // Optional filter patterns (nil/empty = no filter)
	sourceURL string, // Source URL for metadata tracking
	sourceType string, // "local", "github", "git-url", "test"
	ref string, // Git ref (empty for local/test)
	importMode string, // "copy" or "symlink"
	sourceName string, // Explicit source name from manifest (empty = derive from URL)
	sourceID string, // Source ID for metadata tracking (empty = none)
) error {
	// Discover all resources (with error collection)
	commands, commandErrors, err := discovery.DiscoverCommandsWithErrors(localPath, "")
	if err != nil {
		return fmt.Errorf("failed to discover commands: %w", err)
	}

	skills, skillErrors, err := discovery.DiscoverSkillsWithErrors(localPath, "")
	if err != nil {
		return fmt.Errorf("failed to discover skills: %w", err)
	}

	agents, agentErrors, err := discovery.DiscoverAgentsWithErrors(localPath, "")
	if err != nil {
		return fmt.Errorf("failed to discover agents: %w", err)
	}

	packages, err := discovery.DiscoverPackages(localPath, "")
	if err != nil {
		return fmt.Errorf("failed to discover packages: %w", err)
	}

	// Collect all discovery errors
	var discoveryErrors []discovery.DiscoveryError
	discoveryErrors = append(discoveryErrors, commandErrors...)
	discoveryErrors = append(discoveryErrors, skillErrors...)
	discoveryErrors = append(discoveryErrors, agentErrors...)

	// Discover marketplace.json
	marketplaceConfig, marketplacePath, err := marketplace.DiscoverMarketplace(localPath, "")
	if err != nil {
		return fmt.Errorf("failed to parse marketplace: %w", err)
	}

	// Check if any resources found
	totalResources := len(commands) + len(skills) + len(agents) + len(packages)
	if totalResources == 0 && marketplaceConfig == nil {
		return fmt.Errorf("no resources found in: %s\nExpected commands (*.md), skills (*/SKILL.md), agents (*.md), packages (*.package.json), or marketplace.json", localPath)
	}

	// Get absolute path for display
	absPath, _ := filepath.Abs(localPath)

	// Store original counts for reporting
	origCommandCount := len(commands)
	origSkillCount := len(skills)
	origAgentCount := len(agents)
	origPackageCount := len(packages)

	// Determine if we should print informational output
	isHumanFormat := addFormatFlag == "" || addFormatFlag == "table"

	// Apply filter if specified
	if len(filter) > 0 {
		var err error
		commands, skills, agents, packages, err = applyFilter(filter, commands, skills, agents, packages)
		if err != nil {
			return err
		}

		// Check if filter matched any resources
		filteredTotal := len(commands) + len(skills) + len(agents) + len(packages)
		if filteredTotal == 0 {
			if isHumanFormat {
				fmt.Printf("⚠ Warning: Filter '%s' matched 0 resources (found %d total)\n\n", strings.Join(filter, ", "), totalResources)
			}
			return nil
		}

		// Show filtered counts
		if isHumanFormat {
			fmt.Printf("Found: %d commands, %d skills, %d agents, %d packages", origCommandCount, origSkillCount, origAgentCount, origPackageCount)
			if filteredTotal < totalResources {
				fmt.Printf(" (filtered to %d matching '%s')\n", filteredTotal, strings.Join(filter, ", "))
			} else {
				fmt.Println()
			}
		}
	} else {
		if isHumanFormat {
			fmt.Printf("Found: %d commands, %d skills, %d agents, %d packages\n", len(commands), len(skills), len(agents), len(packages))
		}
	}

	// Display marketplace info if found
	var marketplacePackages []*marketplace.PackageInfo
	if marketplaceConfig != nil {
		if isHumanFormat {
			relPath := strings.TrimPrefix(marketplacePath, absPath)
			if relPath == "" {
				relPath = "marketplace.json"
			} else {
				relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
			}
			fmt.Printf("Found marketplace: %s (%d plugins)\n", relPath, len(marketplaceConfig.Plugins))

			// Generate packages from marketplace
			fmt.Println("\nGenerating packages from marketplace:")
		}
		basePath := filepath.Dir(marketplacePath)
		marketplacePackages, err = marketplace.GeneratePackages(marketplaceConfig, basePath)
		if err != nil {
			return fmt.Errorf("failed to generate packages from marketplace: %w", err)
		}

		if isHumanFormat {
			for _, pkgInfo := range marketplacePackages {
				fmt.Printf("  ✓ %s (%d resources)\n", pkgInfo.Package.Name, len(pkgInfo.Package.Resources))
			}
		}
	}

	if isHumanFormat {
		fmt.Println()
	}

	// Collect all resource paths
	var allPaths []string

	// Add commands - use discovered paths directly
	for _, cmd := range commands {
		allPaths = append(allPaths, cmd.Path)
	}
	// Add skills - use discovered paths directly
	for _, skill := range skills {
		allPaths = append(allPaths, skill.Path)
	}

	// Add agents - use discovered paths directly
	for _, agent := range agents {
		allPaths = append(allPaths, agent.Path)
	}

	// Add packages
	for _, pkg := range packages {
		pkgPath, err := findPackageFile(localPath, pkg.Name)
		if err == nil {
			allPaths = append(allPaths, pkgPath)
		}
	}

	// Add resources from marketplace-generated packages
	for _, pkgInfo := range marketplacePackages {
		// Import resources for this package
		for _, resRef := range pkgInfo.Package.Resources {
			resType, resName, err := resource.ParseResourceReference(resRef)
			if err != nil {
				continue // Skip invalid references
			}

			// Find the resource file in the plugin source directory
			resPath, err := findResourceInPath(pkgInfo.SourcePath, resType, resName)
			if err == nil {
				allPaths = append(allPaths, resPath)
			}
		}
	}

	// Import using bulk add
	opts := repo.BulkImportOptions{
		SourceName:   sourceName,
		SourceID:     sourceID,
		ImportMode:   importMode,
		Force:        forceFlag,
		SkipExisting: skipExistingFlag,
		DryRun:       dryRunFlag,
		SourceURL:    sourceURL,
		SourceType:   sourceType,
		Ref:          ref,
	}

	result, err := manager.AddBulk(allPaths, opts)
	if err != nil && !skipExistingFlag {
		// Print partial results before error
		printImportResults(result)
		return err
	}

	// Save marketplace-generated packages if not in dry-run mode
	if !dryRunFlag {
		for _, pkgInfo := range marketplacePackages {
			// Save package to repository
			if err := resource.SavePackage(pkgInfo.Package, manager.GetRepoPath()); err != nil {
				fmt.Printf("⚠ Warning: Failed to save package %s: %v\n", pkgInfo.Package.Name, err)
				continue
			}
		}
	}

	// Print discovery errors if any (only for human-readable format)
	if isHumanFormat && len(discoveryErrors) > 0 {
		printDiscoveryErrors(discoveryErrors)
		fmt.Println()
	}

	// Print results
	printImportResults(result)

	// Exit with error if there were failures
	if len(result.Failed) > 0 {
		return fmt.Errorf("failed to import %d resource(s)", len(result.Failed))
	}

	return nil
}

// addBulkFromLocal handles bulk add from a local folder or single file
func addBulkFromLocal(localPath string, manager *repo.Manager) error {
	return addBulkFromLocalWithFilter(localPath, manager, filterFlags)
}

// addBulkFromLocalWithFilter handles bulk add from a local folder or single file with a custom filter
func addBulkFromLocalWithFilter(localPath string, manager *repo.Manager, filter []string) error {
	return addBulkFromLocalWithMode(localPath, manager, filter, "", "copy", "")
}

// addBulkFromLocalWithMode handles bulk add from a local folder or single file with custom filter and import mode.
// If sourceName is non-empty it is used as-is; otherwise the name is derived from --name flag or filepath.Base.
func addBulkFromLocalWithMode(localPath string, manager *repo.Manager, filter []string, sourceID string, importMode string, sourceName string) error {
	// Validate path exists
	if _, err := os.Stat(localPath); err != nil {
		return fmt.Errorf("path does not exist: %s", localPath)
	}

	// Check if it's a file or directory
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	// If it's a single file, handle it specially
	if !info.IsDir() {
		return addSingleResource(localPath, manager)
	}

	absPath, _ := filepath.Abs(localPath)

	// Print header (LOCAL-SPECIFIC) - only for table format
	if addFormatFlag == "" || addFormatFlag == "table" {
		fmt.Printf("Adding from: %s\n", absPath)
		if dryRunFlag {
			fmt.Println("  Mode: DRY RUN (preview only)")
		}
		if importMode == "symlink" {
			fmt.Println("  Add Mode: SYMLINK (local paths linked, not copied)")
		} else {
			fmt.Println("  Add Mode: COPY (resources copied to repository)")
		}
		if len(filter) > 0 {
			fmt.Printf("  Filter: %s\n", strings.Join(filter, ", "))
		}
		fmt.Println()
	}

	// Determine source info
	sourceURL := "file://" + absPath
	sourceType := "local"

	// Determine source name: prefer explicit parameter, then --name flag, then filepath.Base
	if sourceName == "" {
		sourceName = nameFlag
	}
	if sourceName == "" {
		// Derive from path (same logic as manifest generation)
		sourceName = filepath.Base(absPath)
	}

	// Call common import function with import mode
	return importFromLocalPathWithMode(localPath, manager, filter, sourceURL, sourceType, "", importMode, sourceName, sourceID)
}

// addBulkFromGitHub handles bulk add from a GitHub repository
func addBulkFromGitHub(parsed *source.ParsedSource, manager *repo.Manager) error {
	// Compute source ID from URL before import
	tempSource := repomanifest.Source{URL: parsed.URL}
	sourceID := repomanifest.GenerateSourceID(&tempSource)

	return addBulkFromGitHubWithFilter(parsed, manager, filterFlags, sourceID)
}

// addBulkFromGitHubWithFilter handles bulk add from a GitHub repository with a custom filter
func addBulkFromGitHubWithFilter(parsed *source.ParsedSource, manager *repo.Manager, filter []string, sourceID string) error {
	// Clone repository to workspace
	cloneURL, err := source.GetCloneURL(parsed)
	if err != nil {
		return fmt.Errorf("failed to get clone URL: %w", err)
	}

	// Create workspace manager
	workspaceManager, err := workspace.NewManager(manager.GetRepoPath())
	if err != nil {
		return fmt.Errorf("failed to create workspace manager: %w", err)
	}

	// Get or clone repository using workspace cache
	cachePath, err := workspaceManager.GetOrClone(cloneURL, parsed.Ref)
	if err != nil {
		return fmt.Errorf("failed to get cached repo: %w", err)
	}

	// Update cached repository to latest changes
	if err := workspaceManager.Update(cloneURL, parsed.Ref); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to update cached repo (using existing cache): %v\n", err)
	}

	// Determine search path
	searchPath := cachePath
	if parsed.Subpath != "" {
		searchPath = filepath.Join(cachePath, parsed.Subpath)
	}

	// Print header (GitHub-specific) - only for table format
	if addFormatFlag == "" || addFormatFlag == "table" {
		fmt.Printf("Adding from: %s\n", parsed.URL)
		if parsed.Ref != "" {
			fmt.Printf("  Branch/Tag: %s\n", parsed.Ref)
		}
		if parsed.Subpath != "" {
			fmt.Printf("  Subpath: %s\n", parsed.Subpath)
		}
		if dryRunFlag {
			fmt.Println("  Mode: DRY RUN (preview only)")
		}
		if len(filter) > 0 {
			fmt.Printf("  Filter: %s\n", strings.Join(filter, ", "))
		}
		fmt.Println()
	}

	// Determine source type
	sourceType := "github"
	if parsed.Type == source.GitURL {
		sourceType = "git-url"
	}

	// Determine source name (use --name flag if provided, otherwise derive from URL)
	sourceName := nameFlag
	if sourceName == "" {
		// Derive from URL (extract repo name)
		// Examples:
		// https://github.com/user/repo -> repo
		// https://github.com/user/repo.git -> repo
		url := parsed.URL
		url = strings.TrimSuffix(url, ".git")
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			sourceName = parts[len(parts)-1]
		}
		if sourceName == "" {
			sourceName = "source"
		}
	}

	// Call common import function with workspace path
	return importFromLocalPathWithMode(searchPath, manager, filter, parsed.URL, sourceType, parsed.Ref, "copy", sourceName, sourceID)
}

// selectResource handles resource selection when multiple resources are found
func selectResource(resources []*resource.Resource, resourceType string) (*resource.Resource, error) {
	if len(resources) == 1 {
		// Single resource found, use it automatically
		return resources[0], nil
	}

	// Multiple resources found, prompt user to select
	// Print header
	fmt.Printf("Multiple %ss found in repository:\n\n", resourceType)

	// Create table using shared infrastructure
	table := output.NewTable("#", "Name", "Description")
	table.WithResponsive().
		WithDynamicColumn(2).          // Description stretches
		WithMinColumnWidths(3, 40, 30) // # min=3, Name min=40, Description min=30

	// Add rows
	for i, res := range resources {
		desc := res.Description
		if desc == "" {
			desc = "(no description)"
		}

		table.AddRow(fmt.Sprintf("%d", i+1), res.Name, desc)
	}

	// Render table
	if err := table.Format(output.Table); err != nil {
		return nil, fmt.Errorf("failed to render table: %w", err)
	}

	// Prompt for selection
	fmt.Printf("\nSelect a resource (1-%d, or 'q' to cancel): ", len(resources))
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read selection: %w", err)
	}

	input = strings.TrimSpace(input)

	// Allow 'q' to cancel
	if input == "q" || input == "Q" {
		return nil, fmt.Errorf("selection cancelled by user")
	}

	var selection int
	_, err = fmt.Sscanf(input, "%d", &selection)
	if err != nil || selection < 1 || selection > len(resources) {
		return nil, fmt.Errorf("invalid selection: %s", input)
	}

	return resources[selection-1], nil
}

// NOTE: The find*File/Dir functions below (lines 720-923) are STILL NEEDED despite discovery
// system improvements. They serve distinct purposes:
//
// 1. findPackageFile() - Packages are discovered by name only, need to find actual .package.json path
// 2. findResourceInPath() - Marketplace imports need to resolve resource references (e.g., "command/test")
//    to actual file paths within plugin source directories
// 3. All functions skip .claude/.opencode/.github dirs to avoid importing installed resources
//
// DO NOT DELETE - These complement the discovery system, they don't duplicate it.
// See: ai-config-manager-m5zt investigation

// findResourceFile finds the actual file path for a command or agent resource by name.
// It walks the searchPath, skipping tool installation directories, and uses the provided
// loader function to validate and match resources by name.
func findResourceFile(searchPath, name string, loader func(string) (*resource.Resource, error), typeName string) (string, error) {
	var found string
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip tool installation directories
			dirName := info.Name()
			if dirName == ".claude" || dirName == ".opencode" || dirName == ".github" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		// Load and check if it's the resource we're looking for
		res, err := loader(path)
		if err != nil {
			return nil // Skip invalid resources
		}
		if res.Name == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s file not found for: %s", typeName, name)
	}
	return found, nil
}

// findCommandFile finds the actual file path for a command by name
func findCommandFile(searchPath, name string) (string, error) {
	return findResourceFile(searchPath, name, resource.LoadCommand, "command")
}

// findSkillDir finds the directory path for a skill by name
func findSkillDir(searchPath, name string) (string, error) {
	var found string
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		// Skip tool installation directories
		dirName := info.Name()
		if dirName == ".claude" || dirName == ".opencode" || dirName == ".github" {
			return filepath.SkipDir
		}
		// Check if this directory contains SKILL.md
		skillMdPath := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(skillMdPath); err != nil {
			return nil // No SKILL.md, continue
		}
		// Load and check if it's the skill we're looking for
		skill, err := resource.LoadSkill(path)
		if err != nil {
			return nil // Skip invalid skills
		}
		if skill.Name == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("skill directory not found for: %s", name)
	}
	return found, nil
}

// findAgentFile finds the actual file path for an agent by name
func findAgentFile(searchPath, name string) (string, error) {
	return findResourceFile(searchPath, name, resource.LoadAgent, "agent")
}

// findPackageFile finds the actual file path for a package by name
func findPackageFile(searchPath, name string) (string, error) {
	// Use recursive search to find the package (packages can be nested in subdirectories)
	var foundPath string

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if it's a package file with matching name
		if strings.HasSuffix(path, ".package.json") {
			pkg, err := resource.LoadPackage(path)
			if err == nil && pkg.Name == name {
				foundPath = path
				// Use a sentinel error to stop the walk early
				return fmt.Errorf("found")
			}
		}

		return nil
	})

	// Check if we stopped because we found the package
	if err != nil && err.Error() == "found" {
		return foundPath, nil
	}

	// Check for other walk errors
	if err != nil && err.Error() != "found" {
		return "", fmt.Errorf("error searching for package: %w", err)
	}

	if foundPath == "" {
		return "", fmt.Errorf("package file not found for: %s", name)
	}

	return foundPath, nil
}
func findResourceInPath(sourcePath string, resType resource.ResourceType, resName string) (string, error) {
	var candidatePaths []string

	switch resType {
	case resource.Command:
		// Check standard locations
		candidatePaths = []string{
			filepath.Join(sourcePath, "commands", resName+".md"),
			filepath.Join(sourcePath, ".claude", "commands", resName+".md"),
			filepath.Join(sourcePath, ".opencode", "commands", resName+".md"),
		}
	case resource.Skill:
		// Check standard locations (skills use directory path, not SKILL.md path)
		candidatePaths = []string{
			filepath.Join(sourcePath, "skills", resName),
			filepath.Join(sourcePath, ".claude", "skills", resName),
			filepath.Join(sourcePath, ".opencode", "skills", resName),
		}
	case resource.Agent:
		// Check standard locations
		candidatePaths = []string{
			filepath.Join(sourcePath, "agents", resName+".md"),
			filepath.Join(sourcePath, ".claude", "agents", resName+".md"),
			filepath.Join(sourcePath, ".opencode", "agents", resName+".md"),
		}
	}

	// Try each candidate path
	for _, path := range candidatePaths {
		if resType == resource.Skill {
			// For skills, check if directory exists with SKILL.md
			skillMdPath := filepath.Join(path, "SKILL.md")
			if _, err := os.Stat(skillMdPath); err == nil {
				return path, nil
			}
		} else {
			// For commands and agents, check if file exists
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("source file not found for %s/%s", resType, resName)
}

// determineSourceInfo determines the source type and URL from a parsed source and local path
func determineSourceInfo(parsed *source.ParsedSource, localPath string) (string, string, error) {
	switch parsed.Type {
	case source.GitHub:
		// GitHub source: type="github", url="gh:owner/repo/path"
		return "github", formatGitHubShortURL(parsed), nil

	case source.GitURL:
		// Git URL: type="github", url=original URL
		return "github", parsed.URL, nil

	case source.Local:
		// Local source: need to determine if file or directory
		info, err := os.Stat(localPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to stat path: %w", err)
		}

		// Convert to absolute path
		absPath, err := filepath.Abs(localPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to get absolute path: %w", err)
		}

		// Format as file:// URL
		fileURL := "file://" + absPath

		if info.IsDir() {
			return "local", fileURL, nil
		}
		return "file", fileURL, nil

	default:
		return "", "", fmt.Errorf("unsupported source type: %s", parsed.Type)
	}
}

// formatGitHubSourceInfo formats GitHub source info for metadata
func formatGitHubSourceInfo(parsed *source.ParsedSource) (string, string) {
	return "github", formatGitHubShortURL(parsed)
}

// formatGitHubShortURL formats a GitHub URL in the gh:owner/repo/path format
func formatGitHubShortURL(parsed *source.ParsedSource) string {
	// Extract owner/repo from the GitHub URL
	// URL format: https://github.com/owner/repo
	url := strings.TrimPrefix(parsed.URL, "https://github.com/")
	url = strings.TrimPrefix(url, "http://github.com/")

	// Remove /tree/branch if present
	if strings.Contains(url, "/tree/") {
		parts := strings.SplitN(url, "/tree/", 2)
		url = parts[0]
	}

	// Build the gh: format URL
	result := "gh:" + url

	// Add subpath if present
	if parsed.Subpath != "" {
		result = result + "/" + parsed.Subpath
	}

	// Add ref if present
	if parsed.Ref != "" {
		result = result + "@" + parsed.Ref
	}

	return result
}

// addSourceToManifest adds the source to ai.repo.yaml manifest.
// include contains the filter patterns to persist (nil/empty = no include filter).
// If the source already exists, its Include field is replaced (REPLACE semantics).
func addSourceToManifest(manager *repo.Manager, parsed *source.ParsedSource, include []string) error {
	// Load existing manifest
	manifest, err := repomanifest.Load(manager.GetRepoPath())
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Create source entry
	manifestSource := &repomanifest.Source{
		Name:    nameFlag, // Will be auto-generated if empty
		Include: include,
	}

	// Set path or URL based on source type
	if parsed.Type == source.Local {
		// For local sources, store absolute path
		absPath, err := filepath.Abs(parsed.LocalPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		manifestSource.Path = absPath
	} else {
		// For remote sources (GitHub, GitURL), store URL
		manifestSource.URL = parsed.URL
		manifestSource.Ref = parsed.Ref
		manifestSource.Subpath = parsed.Subpath
	}

	// Check if source already exists — REPLACE semantics
	identifier := manifestSource.Path + manifestSource.URL
	if existing, found := manifest.GetSource(identifier); found {
		// Update Include with new values (replace, not merge)
		existing.Include = include
		// Save manifest with updated include
		if err := manifest.Save(manager.GetRepoPath()); err != nil {
			return fmt.Errorf("failed to save manifest: %w", err)
		}
		if addFormatFlag == "" || addFormatFlag == "table" {
			fmt.Printf("\n✓ Source '%s' updated in ai.repo.yaml\n", existing.Name)
		}
		return nil
	}

	// Add source to manifest (new source)
	if err := manifest.AddSource(manifestSource); err != nil {
		return fmt.Errorf("failed to add source to manifest: %w", err)
	}

	// Save manifest
	if err := manifest.Save(manager.GetRepoPath()); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	// Load and update source metadata
	metadata, err := sourcemetadata.Load(manager.GetRepoPath())
	if err != nil {
		// If metadata doesn't exist, create new one
		metadata = &sourcemetadata.SourceMetadata{
			Version: 1,
			Sources: make(map[string]*sourcemetadata.SourceState),
		}
	}

	// Add source state with current timestamp
	metadata.Sources[manifestSource.Name] = &sourcemetadata.SourceState{
		Added:      time.Now(),
		LastSynced: time.Time{}, // Zero time for "never synced"
	}

	// Save metadata
	if err := metadata.Save(manager.GetRepoPath()); err != nil {
		return fmt.Errorf("failed to save source metadata: %w", err)
	}

	// Print success message for human-readable format
	if addFormatFlag == "" || addFormatFlag == "table" {
		fmt.Printf("\n✓ Source '%s' tracked in ai.repo.yaml\n", manifestSource.Name)
	}

	return nil
}

// printImportResults prints a formatted summary of import results
func printImportResults(result *repo.BulkImportResult) {
	// Parse format flag
	format, err := output.ParseFormat(addFormatFlag)
	if err != nil {
		// Fall back to default human-readable output if format is invalid
		fmt.Fprintf(os.Stderr, "Warning: %v, using table format\n", err)
		format = output.Table
	}

	// Use structured output formatter for all formats
	bulkResult := output.FromBulkImportResult(result)
	if err := output.FormatBulkResult(bulkResult, format); err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
	}
}

// printDiscoveryErrors prints discovery errors with helpful suggestions
func printDiscoveryErrors(errors []discovery.DiscoveryError) {
	if len(errors) == 0 {
		return
	}

	// Deduplicate errors by path (first error for each path wins)
	seen := make(map[string]bool)
	uniqueErrors := make([]discovery.DiscoveryError, 0, len(errors))
	for _, err := range errors {
		if !seen[err.Path] {
			seen[err.Path] = true
			uniqueErrors = append(uniqueErrors, err)
		}
	}

	fmt.Printf("⚠ Discovery Issues (%d):\n", len(uniqueErrors))
	fmt.Println()

	for _, err := range uniqueErrors {
		// Extract just the directory/file name for display
		shortPath := filepath.Base(err.Path)
		if filepath.Base(filepath.Dir(err.Path)) != "." {
			// Include parent directory for context
			shortPath = filepath.Join(filepath.Base(filepath.Dir(err.Path)), filepath.Base(err.Path))
		}

		fmt.Printf("  ✗ %s\n", shortPath)
		fmt.Printf("    Error: %v\n", err.Error)
		fmt.Println()
	}

	fmt.Println("  Tip: These resources were skipped due to validation errors.")
	fmt.Println("       Fix the issues above and re-run the import.")
}
