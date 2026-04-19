package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/install"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/source"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/sourcemetadata"
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

// resolveSourcePathForInstallBootstrap is overridable in tests.
var resolveSourcePathForInstallBootstrap = resolveSourcePathForSync

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

	state, err := prepareManifestInstallState()
	if err != nil {
		return err
	}
	if err := validateManifestInstallRepository(state); err != nil {
		return err
	}

	printManifestInstallReadBanner(state.view)

	repoLock, err := acquireManifestInstallRepoLock(state)
	if err != nil {
		return err
	}
	defer func() {
		_ = repoLock.Unlock()
	}()

	if err := maybeHoldAfterRepoLock(context.Background(), "install"); err != nil {
		return err
	}

	if err := ensureManifestInstallRepository(state); err != nil {
		return err
	}

	if state.hasManifestSources() {
		bootstrapResult, bootstrapErr := bootstrapManifestSourcesForInstall(state.manager, state.manifest)
		if bootstrapErr != nil {
			return bootstrapErr
		}

		if err := failIfReusedSourceIncludeMismatch(state.manager, state.manifest, bootstrapResult.reusedSources); err != nil {
			return err
		}
	}

	// Check if manifest has any resources
	if len(state.manifest.Resources) == 0 {
		fmt.Printf("No resources defined in %s\n", manifest.ManifestFileName)
		return nil
	}

	fmt.Printf("Installing %d resources...\n", len(state.manifest.Resources))

	targetTools, err := resolveManifestInstallTargets(state.manifest)
	if err != nil {
		return err
	}

	installer, err := newManifestInstaller(state.projectPath, targetTools)
	if err != nil {
		return err
	}

	results := installResourcesFromManifest(state.manifest, installer, state.manager)

	// Print results
	fmt.Println()
	printInstallSummary(results)

	// Return error if any resource failed
	if installResultsContainFailures(results) {
		return fmt.Errorf("some resources failed to install")
	}

	return nil
}

type unlocker interface {
	Unlock() error
}

type installSourceBootstrapResult struct {
	reusedSources []*repomanifest.Source
}

type manifestInstallState struct {
	projectPath string
	manager     *repo.Manager
	manifest    *manifest.Manifest
	view        *manifest.ProjectManifests
	repoExists  bool
}

func (s *manifestInstallState) hasManifestSources() bool {
	return s != nil && s.manifest != nil && len(s.manifest.Sources) > 0
}

func resolveInstallProjectPath() (string, error) {
	projectPath := projectPathFlag
	if projectPath != "" {
		return projectPath, nil
	}

	projectPath, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	return projectPath, nil
}

func prepareManifestInstallState() (*manifestInstallState, error) {
	projectPath, err := resolveInstallProjectPath()
	if err != nil {
		return nil, err
	}

	manager, err := NewManagerWithLogLevel()
	if err != nil {
		return nil, fmt.Errorf("failed to create repository manager: %w", err)
	}

	m, view, err := loadEffectiveProjectManifest(projectPath)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("no resources specified and neither %s nor %s found\n\nTo install resources, either:\n  1. Specify resources: aimgr install skill/pdf-processing\n  2. Create %s in current directory", manifest.ManifestFileName, manifest.LocalManifestFileName, manifest.ManifestFileName)
	}

	repoExists, err := repoPathExists(manager.GetRepoPath())
	if err != nil {
		return nil, fmt.Errorf("failed to inspect repository state: %w", err)
	}

	return &manifestInstallState{
		projectPath: projectPath,
		manager:     manager,
		manifest:    m,
		view:        view,
		repoExists:  repoExists,
	}, nil
}

func validateManifestInstallRepository(state *manifestInstallState) error {
	if state.repoExists || state.hasManifestSources() {
		return nil
	}

	return fmt.Errorf("repository is not initialized at %s; run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first", state.manager.GetRepoPath())
}

func printManifestInstallReadBanner(view *manifest.ProjectManifests) {
	if view != nil && view.Local != nil {
		fmt.Printf("Reading %s + %s...\n", manifest.ManifestFileName, manifest.LocalManifestFileName)
		return
	}

	fmt.Printf("Reading %s...\n", manifest.ManifestFileName)
}

func acquireManifestInstallRepoLock(state *manifestInstallState) (unlocker, error) {
	if state.hasManifestSources() {
		repoLock, err := state.manager.AcquireRepoWriteLock(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to acquire repository lock at %s: %w", state.manager.RepoLockPath(), err)
		}
		return repoLock, nil
	}

	repoLock, err := state.manager.AcquireRepoReadLock(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to acquire repository read lock at %s: %w", state.manager.RepoLockPath(), err)
	}

	return repoLock, nil
}

func ensureManifestInstallRepository(state *manifestInstallState) error {
	if state.repoExists {
		return nil
	}
	if !state.hasManifestSources() {
		return fmt.Errorf("repository is not initialized at %s; run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first", state.manager.GetRepoPath())
	}

	if err := state.manager.Init(); err != nil {
		return fmt.Errorf("failed to initialize repository for install source bootstrap: %w", err)
	}

	return nil
}

func resolveManifestInstallTargets(m *manifest.Manifest) ([]tools.Tool, error) {
	explicitTargets, err := parseTargetFlag(installTargetFlag)
	if err != nil {
		return nil, err
	}
	if explicitTargets != nil {
		return explicitTargets, nil
	}

	var targetTools []tools.Tool
	for _, t := range m.Install.Targets {
		tool, parseErr := tools.ParseTool(t)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid target in manifest '%s': %w", t, parseErr)
		}
		targetTools = append(targetTools, tool)
	}

	return targetTools, nil
}

func newManifestInstaller(projectPath string, targetTools []tools.Tool) (*install.Installer, error) {
	if targetTools != nil {
		installer, err := install.NewInstallerWithTargets(projectPath, targetTools)
		if err != nil {
			return nil, fmt.Errorf("failed to create installer: %w", err)
		}
		return installer, nil
	}

	cfg, err := config.LoadGlobal()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	defaultTargets, err := cfg.GetDefaultTargets()
	if err != nil {
		return nil, fmt.Errorf("invalid default targets in config: %w", err)
	}

	installer, err := install.NewInstaller(projectPath, defaultTargets)
	if err != nil {
		return nil, fmt.Errorf("failed to create installer: %w", err)
	}

	return installer, nil
}

func installResourcesFromManifest(m *manifest.Manifest, installer *install.Installer, manager *repo.Manager) []installResult {
	results := make([]installResult, 0, len(m.Resources))
	for _, resourceRef := range m.Resources {
		results = append(results, installManifestResource(resourceRef, installer, manager)...)
	}
	return results
}

func installManifestResource(resourceRef string, installer *install.Installer, manager *repo.Manager) []installResult {
	if !strings.HasPrefix(resourceRef, "package/") {
		return []installResult{processInstall(resourceRef, installer, manager)}
	}

	packageName := strings.TrimPrefix(resourceRef, "package/")
	repoPath := manager.GetRepoPath()
	pkgPath := resource.GetPackagePath(packageName, repoPath)
	pkg, err := resource.LoadPackage(pkgPath)
	if err != nil {
		return []installResult{{
			name:    resourceRef,
			success: false,
			message: fmt.Sprintf("package '%s' not found in repository", packageName),
		}}
	}

	fmt.Printf("Expanding package '%s' (%d resources)...\n", packageName, len(pkg.Resources))

	results := make([]installResult, 0, len(pkg.Resources))
	for _, pkgResourceRef := range pkg.Resources {
		results = append(results, processInstall(pkgResourceRef, installer, manager))
	}

	return results
}

func installResultsContainFailures(results []installResult) bool {
	for _, result := range results {
		if !result.success && !result.skipped {
			return true
		}
	}

	return false
}

func bootstrapManifestSourcesForInstall(manager *repo.Manager, m *manifest.Manifest) (*installSourceBootstrapResult, error) {
	if manager == nil || m == nil || len(m.Sources) == 0 {
		return &installSourceBootstrapResult{}, nil
	}

	repoManifest, err := repomanifest.LoadForMutation(manager.GetRepoPath())
	if err != nil {
		return nil, fmt.Errorf("failed to load repository source manifest: %w", err)
	}

	metadata, err := sourcemetadata.Load(manager.GetRepoPath())
	if err != nil {
		return nil, fmt.Errorf("failed to load source metadata: %w", err)
	}

	reused := make(map[string]*repomanifest.Source)

	for _, declared := range m.Sources {
		existing, found := repoManifest.FindRemoteSourceByCanonical(declared.URL, declared.Subpath)
		if found {
			reused[existing.Name] = existing
			if strings.TrimSpace(declared.Ref) != strings.TrimSpace(existing.Ref) {
				existingRef := existing.Ref
				if existingRef == "" {
					existingRef = "<none>"
				}
				requestedRef := strings.TrimSpace(declared.Ref)
				if requestedRef == "" {
					requestedRef = "<none>"
				}
				fmt.Fprintf(os.Stderr, "Warning: reused existing source '%s' for %s; requested ref '%s' was not applied (existing ref: '%s')\n", existing.Name, describeManifestRemoteSource(declared.URL, declared.Subpath), requestedRef, existingRef)
			}
			continue
		}

		src := &repomanifest.Source{
			Name:      strings.TrimSpace(declared.Name),
			URL:       declared.URL,
			Ref:       strings.TrimSpace(declared.Ref),
			Subpath:   declared.Subpath,
			Discovery: repomanifest.DiscoveryModeAuto,
		}

		if err := repoManifest.AddSource(src); err != nil {
			return nil, fmt.Errorf("failed to track manifest source %s: %w", describeManifestRemoteSource(declared.URL, declared.Subpath), err)
		}
		if err := repoManifest.Save(manager.GetRepoPath()); err != nil {
			return nil, fmt.Errorf("failed to save repository source manifest: %w", err)
		}

		if state := metadata.Get(src.Name); state == nil {
			metadata.Sources[src.Name] = &sourcemetadata.SourceState{Added: time.Now(), SourceID: src.ID}
		} else {
			state.Added = time.Now()
			state.SourceID = src.ID
		}
		if err := metadata.Save(manager.GetRepoPath()); err != nil {
			return nil, fmt.Errorf("failed to persist source metadata for '%s': %w", src.Name, err)
		}

		if err := syncManifestSourceForInstall(manager, src); err != nil {
			return nil, err
		}

		if state := metadata.Get(src.Name); state == nil {
			metadata.Sources[src.Name] = &sourcemetadata.SourceState{Added: time.Now(), LastSynced: time.Now(), SourceID: src.ID}
		} else {
			if state.Added.IsZero() {
				state.Added = time.Now()
			}
			state.LastSynced = time.Now()
			state.SourceID = src.ID
		}
		if err := metadata.Save(manager.GetRepoPath()); err != nil {
			return nil, fmt.Errorf("failed to persist source sync metadata for '%s': %w", src.Name, err)
		}
	}

	reusedSources := make([]*repomanifest.Source, 0, len(reused))
	for _, src := range reused {
		reusedSources = append(reusedSources, src)
	}

	return &installSourceBootstrapResult{reusedSources: reusedSources}, nil
}

func syncManifestSourceForInstall(manager *repo.Manager, src *repomanifest.Source) error {
	sourcePath, err := resolveSourcePathForInstallBootstrap(src, manager)
	if err != nil {
		return fmt.Errorf("failed to prepare source '%s' (%s): %w", src.Name, sourceLocationSummary(src), err)
	}

	discovered, err := discoverImportResourcesByMode(sourcePath, src.Discovery)
	if err != nil {
		return fmt.Errorf("failed to discover resources for source '%s': %w", src.Name, err)
	}
	if len(discovered.discoveryErrors) > 0 {
		fmt.Println()
		printDiscoveryErrorsForSource(src.Name, discovered.discoveryErrors)
		fmt.Println()
	}

	commands, skills, agents, packages, err := applyFilter(src.Include, discovered.commands, discovered.skills, discovered.agents, discovered.packages)
	if err != nil {
		return fmt.Errorf("invalid include filter for source '%s': %w", src.Name, err)
	}

	allPaths := make([]string, 0, len(commands)+len(skills)+len(agents)+len(packages)+len(discovered.marketplacePackages))
	for _, cmdRes := range commands {
		allPaths = append(allPaths, cmdRes.Path)
	}
	for _, skillRes := range skills {
		allPaths = append(allPaths, skillRes.Path)
	}
	for _, agentRes := range agents {
		allPaths = append(allPaths, agentRes.Path)
	}
	for _, pkg := range packages {
		pkgPath, findErr := findPackageFile(sourcePath, pkg.Name)
		if findErr == nil {
			allPaths = append(allPaths, pkgPath)
		}
	}
	for _, pkgInfo := range discovered.marketplacePackages {
		for _, resRef := range pkgInfo.Package.Resources {
			resType, resName, parseErr := resource.ParseResourceReference(resRef)
			if parseErr != nil {
				continue
			}
			resPath, findErr := findResourceInPath(pkgInfo.SourcePath, resType, resName)
			if findErr == nil {
				allPaths = append(allPaths, resPath)
			}
		}
	}

	if len(allPaths) == 0 && len(discovered.marketplacePackages) == 0 {
		return fmt.Errorf("source '%s' produced no importable resources", src.Name)
	}

	parsed, err := parsedRemoteSourceForManifestEntry(src)
	if err != nil {
		return fmt.Errorf("invalid source '%s': %w", src.Name, err)
	}

	sourceType := sourceTypeGitHub
	if parsed.Type == source.GitURL || parsed.Type == source.GitLab {
		sourceType = "git-url"
	}

	bulkResult, err := manager.AddBulk(allPaths, repo.BulkImportOptions{
		SourceName:   src.Name,
		SourceID:     src.ID,
		ImportMode:   "copy",
		Force:        false,
		SkipExisting: false,
		DryRun:       false,
		SourceURL:    src.URL,
		SourceType:   sourceType,
		Ref:          src.Ref,
	})
	if err != nil {
		return fmt.Errorf("failed to sync source '%s': %w", src.Name, err)
	}
	if len(bulkResult.Failed) > 0 {
		return fmt.Errorf("failed to sync source '%s': %d resource(s) could not be imported", src.Name, len(bulkResult.Failed))
	}

	for _, pkgInfo := range discovered.marketplacePackages {
		if saveErr := persistGeneratedMarketplacePackage(manager, pkgInfo.Package, src.URL, sourceType, src.Ref, src.Name, src.ID); saveErr != nil {
			return fmt.Errorf("failed to persist generated package %q from source '%s': %w", pkgInfo.Package.Name, src.Name, saveErr)
		}
	}

	return nil
}

func failIfReusedSourceIncludeMismatch(manager *repo.Manager, m *manifest.Manifest, reusedSources []*repomanifest.Source) error {
	if manager == nil || m == nil || len(reusedSources) == 0 {
		return nil
	}

	missingRequired := collectMissingRequiredResources(manager, m.Resources)
	if len(missingRequired) == 0 {
		return nil
	}

	var mismatchMessages []string

	for _, src := range reusedSources {
		if src == nil || len(src.Include) == 0 {
			continue
		}

		sourcePath, err := resolveSourcePathForInstallBootstrap(src, manager)
		if err != nil {
			continue
		}

		allResources, err := scanSourceResources(sourcePath, src.Discovery)
		if err != nil {
			continue
		}

		filteredResources, err := scanSourceResources(sourcePath, src.Discovery)
		if err != nil {
			continue
		}
		if err := applyIncludeFilterToDiscovered(filteredResources, src.Include); err != nil {
			continue
		}

		sourceMissing := make([]string, 0)
		for _, ref := range missingRequired {
			resType, resName, parseErr := resource.ParseResourceReference(ref)
			if parseErr != nil {
				continue
			}
			if !resourceSetContains(allResources, resType, resName) {
				continue
			}
			if resourceSetContains(filteredResources, resType, resName) {
				continue
			}
			sourceMissing = append(sourceMissing, ref)
		}

		if len(sourceMissing) == 0 {
			continue
		}

		sort.Strings(sourceMissing)
		mismatchMessages = append(mismatchMessages, fmt.Sprintf("source '%s' reused with include filters [%s] excludes required resources: %s", src.Name, strings.Join(src.Include, ", "), strings.Join(sourceMissing, ", ")))
	}

	if len(mismatchMessages) == 0 {
		return nil
	}

	sort.Strings(mismatchMessages)
	return fmt.Errorf("install cannot continue because reused source filters are too narrow:\n  - %s\nUpdate include filters for the reused source in ai.repo.yaml (or via 'aimgr repo add --filter ...') and retry", strings.Join(mismatchMessages, "\n  - "))
}

func collectMissingRequiredResources(manager *repo.Manager, requiredRefs []string) []string {
	missing := make([]string, 0)
	seen := make(map[string]struct{})

	for _, ref := range requiredRefs {
		if _, exists := seen[ref]; exists {
			continue
		}
		seen[ref] = struct{}{}

		resType, resName, err := resource.ParseResourceReference(ref)
		if err != nil {
			continue
		}
		if _, err := manager.Get(resName, resType); err != nil {
			missing = append(missing, ref)
		}
	}

	sort.Strings(missing)
	return missing
}

func resourceSetContains(resources map[resource.ResourceType]map[string]bool, resourceType resource.ResourceType, name string) bool {
	if resources == nil {
		return false
	}
	typeSet, ok := resources[resourceType]
	if !ok {
		return false
	}
	return typeSet[name]
}

func describeManifestRemoteSource(url, subpath string) string {
	if strings.TrimSpace(subpath) == "" {
		return fmt.Sprintf("url %q", url)
	}
	return fmt.Sprintf("url %q (subpath %q)", url, subpath)
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
