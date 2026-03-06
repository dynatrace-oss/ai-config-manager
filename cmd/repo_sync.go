package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
	resmeta "github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/modifications"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/source"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/sourcemetadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/workspace"
	"github.com/spf13/cobra"
)

// resourceInfo holds the name and type of a resource for pre-sync inventory tracking.
type resourceInfo struct {
	Name string
	Type resource.ResourceType
}

// collectResourcesBySource returns a map of source identifier -> []resourceInfo
// for all resources in the repo that have a source assigned.
// This is used before sync to build a pre-sync inventory, enabling orphan detection
// by comparing the "before" set with the "after" set.
//
// Instead of using manager.List() (which skips dangling symlinks), this function
// scans the .metadata/ directories directly. Metadata files are real files that
// persist even when the resource symlink is dangling, ensuring complete inventory.
func collectResourcesBySource(repoPath string) (map[string][]resourceInfo, error) {
	bySource := make(map[string][]resourceInfo)

	// Resource types to scan with their metadata subdirectory names
	types := []struct {
		resType resource.ResourceType
		metaDir string
	}{
		{resource.Command, "commands"},
		{resource.Skill, "skills"},
		{resource.Agent, "agents"},
	}

	for _, rt := range types {
		metaDirPath := filepath.Join(repoPath, ".metadata", rt.metaDir)
		entries, err := os.ReadDir(metaDirPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read metadata dir %s: %w", rt.metaDir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-metadata.json") {
				continue
			}

			// Extract resource name from filename: <name>-metadata.json
			name := strings.TrimSuffix(entry.Name(), "-metadata.json")

			meta, err := resmeta.Load(name, rt.resType, repoPath)
			if err != nil {
				continue // Skip unreadable metadata
			}

			key := meta.SourceID
			if key == "" {
				key = meta.SourceName
			}
			if key == "" {
				continue // Skip resources without source info
			}

			bySource[key] = append(bySource[key], resourceInfo{Name: name, Type: rt.resType})
		}
	}

	// Handle packages separately (different metadata structure)
	pkgMetaDir := filepath.Join(repoPath, ".metadata", "packages")
	pkgEntries, err := os.ReadDir(pkgMetaDir)
	if err == nil {
		for _, entry := range pkgEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-metadata.json") {
				continue
			}

			name := strings.TrimSuffix(entry.Name(), "-metadata.json")
			pkgMeta, err := resmeta.LoadPackageMetadata(name, repoPath)
			if err != nil {
				continue
			}

			key := pkgMeta.SourceID
			if key == "" {
				key = pkgMeta.SourceName
			}
			if key == "" {
				continue
			}

			bySource[key] = append(bySource[key], resourceInfo{Name: name, Type: resource.PackageType})
		}
	}

	return bySource, nil
}

var (
	syncSkipExistingFlag bool
	syncDryRunFlag       bool
	syncFormatFlag       string
	syncForceFlag        bool
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync resources from configured sources",
	Long: `Sync resources from configured sources in ai.repo.yaml.

This command reads sources from the repository's ai.repo.yaml manifest file
and re-imports all resources from each source. This is useful for:
  - Pulling latest changes from remote repositories
  - Updating symlinked resources from local paths
  - Cleaning up orphaned resources from removed sources

By default, existing resources will be overwritten (force mode). Use --skip-existing
to skip resources that already exist in the repository.

The ai.repo.yaml file is automatically maintained when you use "aimgr repo add".

Include filters (set via "aimgr repo add --filter") are stored in ai.repo.yaml
and respected during sync: only resources matching the include patterns are imported
for that source. Removing a pattern from include and re-syncing removes the previously
imported resource. Sources without include filters import all resources.

Example ai.repo.yaml:

  version: 1
  sources:
    - name: my-team-resources
      path: /home/user/resources
    - name: community-skills
      url: https://github.com/owner/repo
      ref: main
      include:
        - skill/pdf-processing
        - skill/ocr*

Examples:
  # Sync all configured sources (overwrites existing)
  aimgr repo sync

  # Sync without overwriting existing resources
  aimgr repo sync --skip-existing

  # Preview what would be synced
  aimgr repo sync --dry-run`,
	RunE: runSync,
}

func init() {
	repoCmd.AddCommand(syncCmd)

	// Add flags
	syncCmd.Flags().BoolVar(&syncSkipExistingFlag, "skip-existing", false, "Skip conflicts silently")
	syncCmd.Flags().BoolVar(&syncDryRunFlag, "dry-run", false, "Preview without importing")
	syncCmd.Flags().BoolVar(&syncForceFlag, "force", false, "Overwrite existing resources (default: true)")
	syncCmd.Flags().StringVar(&syncFormatFlag, "format", "table", "Output format: table, json, yaml")
	_ = syncCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)
}

// scanSourceResources scans a source directory and returns the set of
// resource names it contains, keyed by type.
// This uses the same discovery functions as importFromLocalPathWithMode
// to ensure consistent resource detection.
func scanSourceResources(sourcePath string) (map[resource.ResourceType]map[string]bool, error) {
	result := make(map[resource.ResourceType]map[string]bool)

	// Discover commands
	commands, _, err := discovery.DiscoverCommandsWithErrors(sourcePath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to discover commands: %w", err)
	}
	if len(commands) > 0 {
		cmdSet := make(map[string]bool, len(commands))
		for _, cmd := range commands {
			cmdSet[cmd.Name] = true
		}
		result[resource.Command] = cmdSet
	}

	// Discover skills
	skills, _, err := discovery.DiscoverSkillsWithErrors(sourcePath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}
	if len(skills) > 0 {
		skillSet := make(map[string]bool, len(skills))
		for _, skill := range skills {
			skillSet[skill.Name] = true
		}
		result[resource.Skill] = skillSet
	}

	// Discover agents
	agents, _, err := discovery.DiscoverAgentsWithErrors(sourcePath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to discover agents: %w", err)
	}
	if len(agents) > 0 {
		agentSet := make(map[string]bool, len(agents))
		for _, agent := range agents {
			agentSet[agent.Name] = true
		}
		result[resource.Agent] = agentSet
	}

	// Discover packages
	packages, err := discovery.DiscoverPackages(sourcePath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to discover packages: %w", err)
	}
	if len(packages) > 0 {
		pkgSet := make(map[string]bool, len(packages))
		for _, pkg := range packages {
			pkgSet[pkg.Name] = true
		}
		result[resource.PackageType] = pkgSet
	}

	return result, nil
}

// syncSource syncs resources from a single manifest source.
// Returns the resolved source path (for use in post-sync scanning) and any error.
func syncSource(src *repomanifest.Source, manager *repo.Manager) (string, error) {
	var sourcePath string
	var mode string

	if src.URL != "" {
		// Remote source (url): download to workspace, copy to repo
		fmt.Printf("  Mode: Remote (download + copy)\n")
		mode = "copy"

		// Get repository path
		repoPath := manager.GetRepoPath()

		// Create workspace manager
		wsMgr, err := workspace.NewManager(repoPath)
		if err != nil {
			return "", fmt.Errorf("failed to create workspace manager: %w", err)
		}

		// Parse source URL to get clone URL
		// Note: We construct a pseudo-source string for parsing
		sourceStr := src.URL
		if src.Ref != "" {
			sourceStr = sourceStr + "@" + src.Ref
		}
		if src.Subpath != "" {
			sourceStr = sourceStr + "/" + src.Subpath
		}

		parsed, err := source.ParseSource(sourceStr)
		if err != nil {
			return "", fmt.Errorf("invalid source URL: %w", err)
		}

		// Get clone URL
		cloneURL, err := source.GetCloneURL(parsed)
		if err != nil {
			return "", fmt.Errorf("failed to get clone URL: %w", err)
		}

		// Get or clone repository (using ref if available)
		sourcePath, err = wsMgr.GetOrClone(cloneURL, src.Ref)
		if err != nil {
			return "", fmt.Errorf("failed to download repository: %w", err)
		}

		// Apply subpath if specified
		if src.Subpath != "" {
			sourcePath = filepath.Join(sourcePath, src.Subpath)
		}

	} else if src.Path != "" {
		// Local source (path): use path directly, symlink to repo
		fmt.Printf("  Mode: Local (symlink)\n")
		mode = src.GetMode() // Use mode from source (implicit: path=symlink, url=copy)

		// Convert to absolute path
		absPath, err := filepath.Abs(src.Path)
		if err != nil {
			return "", fmt.Errorf("invalid path %s: %w", src.Path, err)
		}
		sourcePath = absPath
	} else {
		return "", fmt.Errorf("source must have either URL or Path")
	}

	// Import from source path with appropriate mode
	// Pass src.Include as the filter: only matching resources will be imported.
	// Empty include (nil/[]) means import everything (backward compatible).
	err := addBulkFromLocalWithMode(sourcePath, manager, src.Include, src.ID, mode, src.Name)
	if err != nil {
		return "", err
	}
	return sourcePath, nil
}

// syncResult tracks the outcome of processing all sources.
type syncResult struct {
	sourcesProcessed int
	sourcesFailed    int
	failedSources    []string
	removedResources map[string][]resourceInfo
}

// detectRemovedForSource compares a source's pre-sync resource inventory with the
// current source contents to identify resources that were removed from the source.
func detectRemovedForSource(src *repomanifest.Source, sourcePath, repoPath string,
	preSyncResources map[string][]resourceInfo) []resourceInfo {

	sourceKey := src.ID
	if sourceKey == "" {
		sourceKey = src.Name
	}

	preSyncSet := preSyncResources[sourceKey]
	if len(preSyncSet) == 0 {
		return nil
	}

	sourceResources, scanErr := scanSourceResources(sourcePath)
	if scanErr != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not scan source for removal detection: %v\n", scanErr)
		return nil
	}

	// Apply include filter to source resources: if include patterns are set, only
	// resources matching the patterns are considered "present in source". Resources
	// that exist in the source but are excluded by include are treated as absent —
	// this ensures that removing an entry from include and re-syncing removes the
	// previously imported resource (orphan detection for filter changes).
	if len(src.Include) > 0 {
		mm, err := pattern.NewMultiMatcher(src.Include)
		if err != nil {
			slog.Warn("invalid include patterns in source, skipping include filter for orphan detection",
				"source", src.Name, "error", err)
		}
		if err == nil {
			for resType, typeSet := range sourceResources {
				for name := range typeSet {
					res := &resource.Resource{Type: resType, Name: name}
					if !mm.Match(res) {
						delete(typeSet, name)
					}
				}
				// Remove empty type sets to keep the map clean
				if len(typeSet) == 0 {
					delete(sourceResources, resType)
				}
			}
		}
	}

	var removed []resourceInfo
	for _, res := range preSyncSet {
		// Check if this resource still exists in the source
		if typeSet, ok := sourceResources[res.Type]; ok {
			if typeSet[res.Name] {
				continue // Still in source, keep it
			}
		}

		// Before marking for removal, re-check metadata to handle
		// cross-source name collisions (bmz1.7). If another source
		// overwrote this resource during sync, its metadata now
		// points to the other source — skip removal in that case.
		if !resmeta.HasSource(res.Name, res.Type, sourceKey, repoPath) {
			fmt.Fprintf(os.Stderr, "  ⚠ Skipping removal of %s/%s: metadata now points to a different source (name collision)\n",
				res.Type, res.Name)
			continue
		}

		// Resource was in repo but not in source -> removed
		removed = append(removed, res)
	}
	return removed
}

// removeOrphanedResources removes resources that are no longer present in their sources,
// or prints a dry-run preview. Returns the number of resources actually removed.
func removeOrphanedResources(manager *repo.Manager, removedResources map[string][]resourceInfo) int {
	totalToRemove := 0
	for _, resources := range removedResources {
		totalToRemove += len(resources)
	}

	if totalToRemove == 0 {
		return 0
	}

	// Warn about breaking project symlinks before removing (bmz1.3)
	fmt.Fprintf(os.Stderr, "\n⚠ Removed resources may have active project installations.\n")
	fmt.Fprintf(os.Stderr, "  Run 'aimgr repair' in affected projects to clean up broken symlinks.\n\n")

	if syncDryRunFlag {
		fmt.Printf("\nWould remove %d resource(s) no longer in sources:\n", totalToRemove)
		for sourceKey, resources := range removedResources {
			for _, res := range resources {
				fmt.Printf("  - %s/%s (from %s)\n", res.Type, res.Name, sourceKey)
			}
		}
		return 0
	}

	fmt.Printf("Removing %d resource(s) no longer in sources:\n", totalToRemove)
	removeCount := 0
	for sourceKey, resources := range removedResources {
		for _, res := range resources {
			fmt.Printf("  - %s/%s (from %s)\n", res.Type, res.Name, sourceKey)
			if err := manager.Remove(res.Name, res.Type); err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ Warning: failed to remove %s/%s: %v\n",
					res.Type, res.Name, err)
			} else {
				removeCount++
			}
		}
	}
	fmt.Printf("✓ Removed %d resource(s)\n", removeCount)
	return removeCount
}

// syncRegenerateModifications regenerates resource modifications based on current config.
func syncRegenerateModifications(manager *repo.Manager, repoPath string) {
	logger := manager.GetLogger()
	cfg, err := config.LoadGlobal()
	if err != nil {
		// If config load fails, skip modifications silently (not critical)
		return
	}

	gen := modifications.NewGenerator(repoPath, cfg.Mappings, logger)

	if cfg.Mappings.HasAny() {
		// Clean existing modifications first
		if err := gen.CleanupAll(); err != nil {
			if logger != nil {
				logger.Warn("failed to cleanup old modifications", "error", err.Error())
			}
		}

		// Regenerate all
		if err := gen.GenerateAll(); err != nil {
			if logger != nil {
				logger.Warn("failed to generate modifications", "error", err.Error())
			}
		} else {
			if logger != nil {
				logger.Info("regenerated modifications for all resources")
			}
		}
	} else {
		// No mappings - clean up any existing modifications
		if err := gen.CleanupAll(); err != nil {
			if logger != nil {
				logger.Warn("failed to cleanup modifications", "error", err.Error())
			}
		}
	}
}

// syncSaveMetadata saves updated source metadata and commits the changes.
func syncSaveMetadata(manager *repo.Manager, metadata *sourcemetadata.SourceMetadata) {
	if err := metadata.Save(manager.GetRepoPath()); err != nil {
		fmt.Printf("⚠ Warning: Failed to save metadata: %v\n", err)
		return
	}
	// Commit metadata changes to git
	if err := manager.CommitChanges("aimgr: update sync timestamps"); err != nil {
		// Don't fail if commit fails (e.g., not a git repo)
		fmt.Printf("⚠ Warning: Failed to commit metadata: %v\n", err)
	}
}

// runSync executes the sync command
func runSync(cmd *cobra.Command, args []string) error {
	// Create manager
	manager, err := NewManagerWithLogLevel()
	if err != nil {
		return fmt.Errorf("failed to create repo manager: %w", err)
	}

	// Load manifest from ai.repo.yaml
	manifest, err := repomanifest.Load(manager.GetRepoPath())
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Check if any sources configured
	if len(manifest.Sources) == 0 {
		return fmt.Errorf("no sync sources configured\n\nAdd sources using:\n  aimgr repo add <source>\n\nSources are automatically tracked in ai.repo.yaml")
	}

	// Load source metadata for timestamps
	metadata, err := sourcemetadata.Load(manager.GetRepoPath())
	if err != nil {
		// If metadata doesn't exist yet, create empty one
		metadata = &sourcemetadata.SourceMetadata{
			Version: 1,
			Sources: make(map[string]*sourcemetadata.SourceState),
		}
	}

	// Print header
	fmt.Printf("Syncing from %d configured source(s)...\n", len(manifest.Sources))
	if syncDryRunFlag {
		fmt.Println("Mode: DRY RUN (preview only)")
	}
	fmt.Println()

	// Set flags for the duration of the sync operation
	originalForceFlag := forceFlag
	originalDryRunFlag := dryRunFlag
	originalSkipExistingFlag := skipExistingFlag
	originalAddFormatFlag := addFormatFlag

	// Default to force=true unless --skip-existing specified
	forceFlag = !syncSkipExistingFlag
	skipExistingFlag = syncSkipExistingFlag
	dryRunFlag = syncDryRunFlag
	addFormatFlag = syncFormatFlag

	// Restore original flags when done
	defer func() {
		forceFlag = originalForceFlag
		dryRunFlag = originalDryRunFlag
		skipExistingFlag = originalSkipExistingFlag
		addFormatFlag = originalAddFormatFlag
	}()

	// Collect pre-sync inventory for orphan detection (bmz1.1)
	repoPath := manager.GetRepoPath()
	preSyncResources, err := collectResourcesBySource(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not collect pre-sync inventory: %v\n", err)
		preSyncResources = make(map[string][]resourceInfo)
	}

	result := syncResult{
		failedSources:    make([]string, 0),
		removedResources: make(map[string][]resourceInfo),
	}

	// Process each source
	for i, src := range manifest.Sources {
		sourceDesc := src.Name
		if src.URL != "" {
			sourceDesc = fmt.Sprintf("%s (%s)", src.Name, src.URL)
		} else if src.Path != "" {
			sourceDesc = fmt.Sprintf("%s (%s)", src.Name, src.Path)
		}

		fmt.Printf("[%d/%d] Processing: %s\n", i+1, len(manifest.Sources), sourceDesc)

		// Sync this source (returns resolved source path for scanning)
		sourcePath, syncErr := syncSource(src, manager)
		if syncErr != nil {
			fmt.Printf("  ⚠ Warning: %v\n", syncErr)
			fmt.Printf("  Skipping this source and continuing...\n\n")
			result.sourcesFailed++
			result.failedSources = append(result.failedSources, src.Name)
			continue
		}

		// Detect removed resources (bmz1.2)
		sourceKey := src.ID
		if sourceKey == "" {
			sourceKey = src.Name
		}
		if removed := detectRemovedForSource(src, sourcePath, repoPath, preSyncResources); len(removed) > 0 {
			result.removedResources[sourceKey] = removed
		}

		// Update last_synced timestamp in metadata after successful sync
		if !syncDryRunFlag {
			if state, ok := metadata.Sources[src.Name]; ok {
				state.LastSynced = time.Now()
				if src.ID != "" {
					state.SourceID = src.ID
				}
			} else {
				metadata.Sources[src.Name] = &sourcemetadata.SourceState{
					SourceID:   src.ID,
					Added:      time.Now(),
					LastSynced: time.Now(),
				}
			}
		}

		result.sourcesProcessed++
		fmt.Println()
	}

	// Save updated metadata with new timestamps
	if !syncDryRunFlag && result.sourcesProcessed > 0 {
		syncSaveMetadata(manager, metadata)
	}

	// Remove orphaned resources (bmz1.3)
	removeCount := removeOrphanedResources(manager, result.removedResources)

	// Print overall summary
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("Sync Complete: %d/%d sources synced, %d resource(s) removed\n",
		result.sourcesProcessed, len(manifest.Sources), removeCount)
	if result.sourcesFailed > 0 {
		fmt.Printf("  %d source(s) failed or skipped:\n", result.sourcesFailed)
		for _, name := range result.failedSources {
			fmt.Printf("    - %s\n", name)
		}
	}
	fmt.Println()

	// Regenerate modifications (skip in dry-run mode)
	if !syncDryRunFlag {
		syncRegenerateModifications(manager, repoPath)
	}

	// Return error if all sources failed
	if result.sourcesFailed == len(manifest.Sources) {
		return fmt.Errorf("all sources failed to sync")
	}

	return nil
}
