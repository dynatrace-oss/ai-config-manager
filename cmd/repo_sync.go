package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	resmeta "github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/modifications"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/source"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/sourcemetadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/workspace"
	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

// resourceInfo holds the name and type of a resource for pre-sync inventory tracking.
type resourceInfo struct {
	Name string
	Type resource.ResourceType
}

// sourceSyncResult holds the result for one source.
type sourceSyncResult struct {
	Name         string                      `json:"name"`
	URL          string                      `json:"url,omitempty"`
	Path         string                      `json:"path,omitempty"`
	Mode         string                      `json:"mode"` // "remote" or "local"
	Result       *output.BulkOperationResult `json:"result"`
	RemovedCount int                         `json:"removed_count"`
	Error        string                      `json:"error,omitempty"`
	Failed       bool                        `json:"failed"`
}

// removedResource describes a resource that was removed during sync.
type removedResource struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Source string `json:"source"`
}

type workspaceManager interface {
	GetOrClone(url string, ref string) (string, error)
	Update(url string, ref string) error
}

// syncSummary holds aggregate counts for the sync operation.
type syncSummary struct {
	SourcesTotal     int `json:"sources_total"`
	SourcesSynced    int `json:"sources_synced"`
	SourcesFailed    int `json:"sources_failed"`
	ResourcesAdded   int `json:"resources_added"`
	ResourcesUpdated int `json:"resources_updated"`
	ResourcesRemoved int `json:"resources_removed"`
	ResourcesFailed  int `json:"resources_failed"`
}

// syncOutput is the complete sync output, used for JSON/YAML formatting.
type syncOutput struct {
	Sources  []sourceSyncResult `json:"sources"`
	Removed  []removedResource  `json:"removed"`
	Summary  syncSummary        `json:"summary"`
	Warnings []string           `json:"warnings,omitempty"`
}

type syncOutputMode struct {
	format  output.Format
	verbose bool
}

type sourceResourceClaim struct {
	canonicalSourceID string
	sourceName        string
	sourceLocation    string
}

func (m syncOutputMode) human() bool {
	return m.format == output.Table
}

func (m syncOutputMode) detailed() bool {
	return m.format == output.Table && m.verbose
}

func buildSourceDisplayNames(sources []*repomanifest.Source) map[string]string {
	displayNames := make(map[string]string, len(sources)*2)
	for _, src := range sources {
		if src == nil || src.Name == "" {
			continue
		}
		displayNames[src.Name] = src.Name
		if src.ID != "" {
			displayNames[src.ID] = src.Name
		}
		if canonicalID := canonicalSourceID(src); canonicalID != "" {
			displayNames[canonicalID] = src.Name
		}
	}
	return displayNames
}

func resolveSourceDisplayName(sourceKey string, displayNames map[string]string) string {
	if name, ok := displayNames[sourceKey]; ok && name != "" {
		return name
	}
	return sourceKey
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

			// Extract fallback name from filename: <name>-metadata.json
			fallbackName := strings.TrimSuffix(entry.Name(), "-metadata.json")

			meta, err := resmeta.Load(fallbackName, rt.resType, repoPath)
			if err != nil {
				continue // Skip unreadable metadata
			}

			name := meta.Name
			if name == "" {
				name = fallbackName
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

			fallbackName := strings.TrimSuffix(entry.Name(), "-metadata.json")
			pkgMeta, err := resmeta.LoadPackageMetadata(fallbackName, repoPath)
			if err != nil {
				continue
			}

			name := pkgMeta.Name
			if name == "" {
				name = fallbackName
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
	syncVerboseFlag      bool
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:          "sync",
	Short:        "Sync resources from configured sources",
	SilenceUsage: true,
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

Discovery mode (set via "aimgr repo add --discovery") is stored in
ai.repo.yaml as sources[].discovery and reused during sync. This preserves
the source's marketplace-first (auto), marketplace-only, or generic behavior.

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
	syncCmd.Flags().BoolVarP(&syncVerboseFlag, "verbose", "v", false, "Show full per-resource tables (table format only)")
	_ = syncCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)
}

// scanSourceResources scans a source directory and returns the set of
// resource names it contains, keyed by type.
// This uses the same discovery functions as importFromLocalPathWithMode
// to ensure consistent resource detection.
func scanSourceResources(sourcePath, discoveryMode string) (map[resource.ResourceType]map[string]bool, error) {
	result := make(map[resource.ResourceType]map[string]bool)

	discovered, err := discoverImportResourcesByMode(sourcePath, discoveryMode)
	if err != nil {
		return nil, err
	}

	commands := discovered.commands
	if len(commands) > 0 {
		cmdSet := make(map[string]bool, len(commands))
		for _, cmd := range commands {
			cmdSet[cmd.Name] = true
		}
		result[resource.Command] = cmdSet
	}

	skills := discovered.skills
	if len(skills) > 0 {
		skillSet := make(map[string]bool, len(skills))
		for _, skill := range skills {
			skillSet[skill.Name] = true
		}
		result[resource.Skill] = skillSet
	}

	agents := discovered.agents
	if len(agents) > 0 {
		agentSet := make(map[string]bool, len(agents))
		for _, agent := range agents {
			agentSet[agent.Name] = true
		}
		result[resource.Agent] = agentSet
	}

	packages := discovered.packages
	if len(packages) > 0 {
		pkgSet := make(map[string]bool, len(packages))
		for _, pkg := range packages {
			pkgSet[pkg.Name] = true
		}
		result[resource.PackageType] = pkgSet
	}

	if len(discovered.marketplacePackages) > 0 {
		pkgSet, ok := result[resource.PackageType]
		if !ok {
			pkgSet = make(map[string]bool)
			result[resource.PackageType] = pkgSet
		}

		for _, pkgInfo := range discovered.marketplacePackages {
			pkgSet[pkgInfo.Package.Name] = true
			for _, ref := range pkgInfo.Package.Resources {
				resType, resName, parseErr := resource.ParseResourceReference(ref)
				if parseErr != nil {
					continue
				}
				typeSet, exists := result[resType]
				if !exists {
					typeSet = make(map[string]bool)
					result[resType] = typeSet
				}
				typeSet[resName] = true
			}
		}
	}

	return result, nil
}

func resolveSourcePathForSync(src *repomanifest.Source, manager *repo.Manager) (string, error) {
	if src.URL != "" {
		repoPath := manager.GetRepoPath()
		wsMgr, err := workspace.NewManager(repoPath)
		if err != nil {
			return "", fmt.Errorf("failed to create workspace manager: %w", err)
		}

		parsed, err := parsedRemoteSourceForManifestEntry(src)
		if err != nil {
			return "", fmt.Errorf("invalid source URL: %w", err)
		}

		cloneURL, err := source.GetCloneURL(parsed)
		if err != nil {
			return "", fmt.Errorf("failed to get clone URL: %w", err)
		}

		sourcePath, err := prepareRemoteSourcePath(wsMgr, cloneURL, src.Ref)
		if err != nil {
			return "", err
		}

		if src.Subpath != "" {
			sourcePath = filepath.Join(sourcePath, src.Subpath)
		}

		return sourcePath, nil
	}

	if src.Path != "" {
		absPath, err := filepath.Abs(src.Path)
		if err != nil {
			return "", fmt.Errorf("invalid path %s: %w", src.Path, err)
		}
		return absPath, nil
	}

	return "", fmt.Errorf("source must have either URL or Path")
}

func parsedRemoteSourceForManifestEntry(src *repomanifest.Source) (*source.ParsedSource, error) {
	if src == nil || src.URL == "" {
		return nil, fmt.Errorf("source url cannot be empty")
	}

	parsed, err := source.ParseSource(src.URL)
	if err != nil {
		return nil, err
	}

	if src.Ref != "" {
		parsed.Ref = src.Ref
	}
	if src.Subpath != "" {
		parsed.Subpath = src.Subpath
	}

	return parsed, nil
}

func applyIncludeFilterToDiscovered(sourceResources map[resource.ResourceType]map[string]bool, include []string) error {
	if len(include) == 0 {
		return nil
	}

	mm, err := pattern.NewMultiMatcher(include)
	if err != nil {
		return fmt.Errorf("invalid include patterns: %w", err)
	}

	for resType, typeSet := range sourceResources {
		for name := range typeSet {
			res := &resource.Resource{Type: resType, Name: name}
			if !mm.Match(res) {
				delete(typeSet, name)
			}
		}
		if len(typeSet) == 0 {
			delete(sourceResources, resType)
		}
	}

	return nil
}

func canonicalSourceID(src *repomanifest.Source) string {
	if src == nil {
		return ""
	}

	if src.ID != "" && src.OverrideOriginalURL == "" {
		return src.ID
	}

	return repomanifest.GenerateSourceID(src)
}

func sourceLocationSummary(src *repomanifest.Source) string {
	if src == nil {
		return "unknown location"
	}

	if src.URL != "" {
		if src.Ref != "" {
			if src.Subpath != "" {
				return fmt.Sprintf("url %q (ref %q, subpath %q)", src.URL, src.Ref, src.Subpath)
			}
			return fmt.Sprintf("url %q (ref %q)", src.URL, src.Ref)
		}
		if src.Subpath != "" {
			return fmt.Sprintf("url %q (subpath %q)", src.URL, src.Subpath)
		}
		return fmt.Sprintf("url %q", src.URL)
	}

	if src.Path != "" {
		return fmt.Sprintf("path %q", src.Path)
	}

	return "unknown location"
}

func detectSyncResourceCollisions(manifest *repomanifest.Manifest, manager *repo.Manager) error {
	if manifest == nil || len(manifest.Sources) == 0 {
		return nil
	}

	claims := make(map[string]sourceResourceClaim)
	conflicts := make([]string, 0)

	for _, src := range manifest.Sources {
		sourcePath, err := resolveSourcePathForSync(src, manager)
		if err != nil {
			// Keep existing partial-sync behavior for unreachable/invalid sources.
			continue
		}

		sourceResources, err := scanSourceResources(sourcePath, src.Discovery)
		if err != nil {
			continue
		}

		if err := applyIncludeFilterToDiscovered(sourceResources, src.Include); err != nil {
			return fmt.Errorf("source %q has invalid include filters for sync collision precheck: %w", src.Name, err)
		}

		currentSourceID := canonicalSourceID(src)
		currentLocation := sourceLocationSummary(src)

		for resType, typeSet := range sourceResources {
			for name := range typeSet {
				resourceRef := fmt.Sprintf("%s/%s", resType, name)
				if claim, exists := claims[resourceRef]; exists {
					if claim.canonicalSourceID != currentSourceID {
						conflicts = append(conflicts, fmt.Sprintf(
							"%s provided by source %q (%s) and source %q (%s)",
							resourceRef,
							claim.sourceName,
							claim.sourceLocation,
							src.Name,
							currentLocation,
						))
					}
					continue
				}

				claims[resourceRef] = sourceResourceClaim{
					canonicalSourceID: currentSourceID,
					sourceName:        src.Name,
					sourceLocation:    currentLocation,
				}
			}
		}
	}

	if len(conflicts) == 0 {
		return nil
	}

	sort.Strings(conflicts)
	return fmt.Errorf("sync rejected: conflicting resource names across different sources:\n  - %s\nResolve by renaming one resource, narrowing include filters, or removing one of the conflicting sources", strings.Join(conflicts, "\n  - "))
}

func prepareRemoteSourcePath(wsMgr workspaceManager, cloneURL string, ref string) (string, error) {
	sourcePath, err := wsMgr.GetOrClone(cloneURL, ref)
	if err != nil {
		return "", fmt.Errorf("failed to download repository: %w", err)
	}

	// Remote sync must refresh an existing cache before importing resources.
	// Unlike repo add, sync should not silently proceed from stale cached content.
	if err := wsMgr.Update(cloneURL, ref); err != nil {
		return "", fmt.Errorf("failed to update cached repository: %w", err)
	}

	return sourcePath, nil
}

// syncSource syncs resources from a single manifest source.
// Returns the resolved source path (for use in post-sync scanning), the bulk result, and any error.
// When syncSilentMode is true, "Mode: Remote/Local" lines are suppressed.
func syncSource(src *repomanifest.Source, manager *repo.Manager) (string, *output.BulkOperationResult, error) {
	sourcePath, err := resolveSourcePathForSync(src, manager)
	if err != nil {
		return "", nil, err
	}

	var mode string

	if src.URL != "" {
		// Remote source (url): download to workspace, copy to repo
		if !syncSilentMode {
			fmt.Printf("  Mode: Remote (download + copy)\n")
		}
		mode = "copy"
	} else if src.Path != "" {
		// Local source (path): use path directly, symlink to repo
		if !syncSilentMode {
			fmt.Printf("  Mode: Local (symlink)\n")
		}
		mode = src.GetMode() // Use mode from source (implicit: path=symlink, url=copy)
	} else {
		return "", nil, fmt.Errorf("source must have either URL or Path")
	}

	// Import from source path with appropriate mode
	// Pass src.Include as the filter: only matching resources will be imported.
	// Empty include (nil/[]) means import everything (backward compatible).
	var sourceURL string
	var sourceType string
	if src.URL != "" {
		sourceURL = src.URL
		sourceType = "github"
	} else {
		sourceURL = "file://" + sourcePath
		sourceType = string(source.Local)
	}
	bulkResult, err := importFromLocalPathWithMode(sourcePath, manager, src.Include, sourceURL, sourceType, src.Ref, mode, src.Discovery, src.Name, src.ID)
	if err != nil {
		return "", bulkResult, err
	}
	return sourcePath, bulkResult, nil
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
	preSyncResources map[string][]resourceInfo) ([]resourceInfo, []string) {

	sourceKey := canonicalSourceID(src)
	if sourceKey == "" {
		sourceKey = src.Name
	}

	preSyncSet := preSyncResources[sourceKey]
	if len(preSyncSet) == 0 {
		return nil, nil
	}

	sourceResources, scanErr := scanSourceResources(sourcePath, src.Discovery)
	if scanErr != nil {
		return nil, []string{fmt.Sprintf("could not scan source %s for removal detection: %v", src.Name, scanErr)}
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
	var warnings []string
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
			warnings = append(warnings,
				fmt.Sprintf("skipping removal of %s/%s from %s: metadata now points to a different source", res.Type, res.Name, src.Name),
			)
			continue
		}

		// Resource was in repo but not in source -> removed
		removed = append(removed, res)
	}
	return removed, warnings
}

// removeOrphanedResources removes resources that are no longer present in their sources,
// or prints a dry-run preview. Returns the list of successfully removed resources.
func removeOrphanedResources(manager *repo.Manager, removedResources map[string][]resourceInfo, sourceDisplayNames map[string]string, mode syncOutputMode) ([]removedResource, []string) {
	totalToRemove := 0
	for _, resources := range removedResources {
		totalToRemove += len(resources)
	}

	if totalToRemove == 0 {
		return nil, nil
	}

	warnings := []string{"removed resources may have active project installations; run 'aimgr repair' in affected projects if needed"}
	if mode.human() {
		fmt.Fprintf(os.Stdout, "\n⚠ Removed resources may have active project installations.\n")
		fmt.Fprintf(os.Stdout, "  Run 'aimgr repair' in affected projects to clean up broken symlinks.\n\n")
		warnings = nil
	}

	if syncDryRunFlag {
		if mode.human() {
			fmt.Printf("\nWould remove %d resource(s) no longer in sources:\n", totalToRemove)
			for sourceKey, resources := range removedResources {
				displayName := resolveSourceDisplayName(sourceKey, sourceDisplayNames)
				for _, res := range resources {
					fmt.Printf("  - %s/%s (from %s)\n", res.Type, res.Name, displayName)
				}
			}
		}
		return nil, warnings
	}

	if mode.human() {
		fmt.Printf("Removing %d resource(s) no longer in sources:\n", totalToRemove)
	}
	var removed []removedResource
	for sourceKey, resources := range removedResources {
		displayName := resolveSourceDisplayName(sourceKey, sourceDisplayNames)
		for _, res := range resources {
			if mode.human() {
				fmt.Printf("  - %s/%s (from %s)\n", res.Type, res.Name, displayName)
			}
			if err := manager.Remove(res.Name, res.Type); err != nil {
				if mode.human() {
					fmt.Printf("  ⚠ Warning: failed to remove %s/%s from %s: %v\n", res.Type, res.Name, displayName, err)
				} else {
					warnings = append(warnings, fmt.Sprintf("failed to remove %s/%s from %s: %v", res.Type, res.Name, displayName, err))
				}
			} else {
				removed = append(removed, removedResource{
					Name:   res.Name,
					Type:   string(res.Type),
					Source: sourceKey,
				})
			}
		}
	}
	if mode.human() {
		fmt.Printf("✓ Removed %d resource(s)\n", len(removed))
	}
	return removed, warnings
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
func syncSaveMetadata(manager *repo.Manager, metadata *sourcemetadata.SourceMetadata) []string {
	if err := metadata.Save(manager.GetRepoPath()); err != nil {
		return []string{fmt.Sprintf("failed to save metadata: %v", err)}
	}
	// Commit metadata changes to git
	if err := manager.CommitChangesForPaths("aimgr: update sync timestamps", []string{
		filepath.Join(".metadata", "sources.json"),
	}); err != nil {
		// Don't fail if commit fails (e.g., not a git repo)
		return []string{fmt.Sprintf("failed to commit metadata: %v", err)}
	}

	return nil
}

// printSyncOutputTable prints compact table output (one line per source) plus summary.
// When verbose is true, it also prints full per-resource tables for each source.
func printSyncOutputTable(so *syncOutput, verbose bool) {
	for _, src := range so.Sources {
		modeLabel := src.Mode
		if src.Failed {
			fmt.Printf("  ✗ %-30s — error: %s\n", fmt.Sprintf("%s (%s)", src.Name, modeLabel), src.Error)
		} else {
			var added, updated int
			if src.Result != nil {
				added = len(src.Result.Added)
				updated = len(src.Result.Updated)
			}

			counts := []string{
				fmt.Sprintf("%d added", added),
				fmt.Sprintf("%d updated", updated),
			}
			if src.RemovedCount > 0 {
				counts = append(counts, fmt.Sprintf("%d removed", src.RemovedCount))
			}

			fmt.Printf("  ✓ %-30s — %s\n",
				fmt.Sprintf("%s (%s)", src.Name, modeLabel), strings.Join(counts, ", "))
		}

		if verbose && !src.Failed && src.Result != nil {
			fmt.Println()
			if err := output.FormatBulkResult(src.Result, output.Table); err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: failed to format result for %s: %v\n", src.Name, err)
			}
		}
	}

	fmt.Printf("\nSync complete: %d/%d sources, %d resources synced, %d removed\n",
		so.Summary.SourcesSynced,
		so.Summary.SourcesTotal,
		so.Summary.ResourcesAdded+so.Summary.ResourcesUpdated,
		so.Summary.ResourcesRemoved,
	)

	if so.Summary.SourcesFailed > 0 {
		var failedNames []string
		for _, src := range so.Sources {
			if src.Failed {
				failedNames = append(failedNames, src.Name)
			}
		}
		fmt.Printf("  %d source(s) failed: %s\n", so.Summary.SourcesFailed, strings.Join(failedNames, ", "))
	}
	if len(so.Warnings) > 0 {
		if verbose {
			fmt.Printf("  warnings (%d):\n", len(so.Warnings))
			for _, warning := range so.Warnings {
				fmt.Printf("    - %s\n", warning)
			}
		} else {
			fmt.Printf("  warnings: %d (run with --verbose for details)\n", len(so.Warnings))
		}
	}
	fmt.Println()
}

func renderSyncOutput(so *syncOutput, format output.Format, verbose bool) error {
	switch format {
	case output.JSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(so); err != nil {
			return fmt.Errorf("failed to encode JSON output: %w", err)
		}
	case output.YAML:
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		defer func() { _ = enc.Close() }()
		if err := enc.Encode(so); err != nil {
			return fmt.Errorf("failed to encode YAML output: %w", err)
		}
	default: // table
		printSyncOutputTable(so, verbose)
	}

	return nil
}

// runSync executes the sync command
func runSync(cmd *cobra.Command, args []string) error {
	// Create manager
	manager, err := NewManagerWithLogLevel()
	if err != nil {
		return newOperationalFailureError(fmt.Errorf("failed to create repo manager: %w", err))
	}

	if err := ensureRepoInitialized(manager); err != nil {
		return newOperationalFailureError(err)
	}

	repoLock, err := manager.AcquireRepoWriteLock(cmd.Context())
	if err != nil {
		return wrapLockAcquireError(manager.RepoLockPath(), err)
	}
	defer func() {
		_ = repoLock.Unlock()
	}()

	if err := maybeHoldAfterRepoLock(cmd.Context(), "sync"); err != nil {
		return err
	}

	// Load manifest from ai.repo.yaml
	manifest, err := repomanifest.LoadForMutation(manager.GetRepoPath())
	if err != nil {
		return newOperationalFailureError(fmt.Errorf("failed to load manifest: %w", err))
	}

	if err := detectSyncResourceCollisions(manifest, manager); err != nil {
		return newOperationalFailureError(err)
	}

	// Check if any sources configured
	if len(manifest.Sources) == 0 {
		return newOperationalFailureError(fmt.Errorf("no sync sources configured\n\nAdd sources using:\n  aimgr repo add <source>\n\nSources are automatically tracked in ai.repo.yaml"))
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

	// Determine output format
	format, formatErr := output.ParseFormat(syncFormatFlag)
	if formatErr != nil {
		return newOperationalFailureError(formatErr)
	}
	mode := syncOutputMode{format: format, verbose: syncVerboseFlag}
	if mode.human() {
		fmt.Printf("Syncing from %d configured source(s)...\n", len(manifest.Sources))
		if syncDryRunFlag {
			fmt.Println("Mode: DRY RUN (preview only)")
		}
		fmt.Println()
	}
	sourceDisplayNames := buildSourceDisplayNames(manifest.Sources)

	// Set flags for the duration of the sync operation
	originalForceFlag := forceFlag
	originalDryRunFlag := dryRunFlag
	originalSkipExistingFlag := skipExistingFlag
	originalAddFormatFlag := addFormatFlag
	originalSyncSilentMode := syncSilentMode

	// Default to force=true unless --skip-existing specified
	forceFlag = !syncSkipExistingFlag
	skipExistingFlag = syncSkipExistingFlag
	dryRunFlag = syncDryRunFlag
	addFormatFlag = syncFormatFlag
	// Enable silent mode: suppress inline output from importFromLocalPathWithMode
	syncSilentMode = true

	// Restore original flags when done
	defer func() {
		forceFlag = originalForceFlag
		dryRunFlag = originalDryRunFlag
		skipExistingFlag = originalSkipExistingFlag
		addFormatFlag = originalAddFormatFlag
		syncSilentMode = originalSyncSilentMode
	}()

	// Collect pre-sync inventory for orphan detection (bmz1.1)
	repoPath := manager.GetRepoPath()
	var warnings []string
	preSyncResources, err := collectResourcesBySource(repoPath)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("could not collect pre-sync inventory: %v", err))
		preSyncResources = make(map[string][]resourceInfo)
	}

	internalResult := syncResult{
		failedSources:    make([]string, 0),
		removedResources: make(map[string][]resourceInfo),
	}

	// Collect per-source results
	var sourceResults []sourceSyncResult

	// Process each source
	for _, src := range manifest.Sources {
		sr := sourceSyncResult{
			Name: src.Name,
		}
		if src.URL != "" {
			sr.URL = src.URL
			sr.Mode = "remote"
		} else {
			sr.Path = src.Path
			sr.Mode = "local"
		}

		// Sync this source (returns resolved source path for scanning)
		sourcePath, bulkResult, syncErr := syncSource(src, manager)
		sr.Result = bulkResult

		if syncErr != nil {
			sr.Failed = true
			sr.Error = syncErr.Error()
			internalResult.sourcesFailed++
			internalResult.failedSources = append(internalResult.failedSources, src.Name)
			sourceResults = append(sourceResults, sr)
			continue
		}

		// Detect removed resources (bmz1.2)
		sourceKey := canonicalSourceID(src)
		if sourceKey == "" {
			sourceKey = src.Name
		}
		removed, detectWarnings := detectRemovedForSource(src, sourcePath, repoPath, preSyncResources)
		if len(removed) > 0 {
			internalResult.removedResources[sourceKey] = removed
			sr.RemovedCount = len(removed)
		}
		warnings = append(warnings, detectWarnings...)

		// Update last_synced timestamp in metadata after successful sync
		if !syncDryRunFlag {
			canonicalID := canonicalSourceID(src)
			if state, ok := metadata.Sources[src.Name]; ok {
				state.LastSynced = time.Now()
				if canonicalID != "" {
					state.SourceID = canonicalID
				}
			} else {
				metadata.Sources[src.Name] = &sourcemetadata.SourceState{
					SourceID:   canonicalID,
					Added:      time.Now(),
					LastSynced: time.Now(),
				}
			}
		}

		internalResult.sourcesProcessed++
		sourceResults = append(sourceResults, sr)
	}

	// Save updated metadata with new timestamps
	if !syncDryRunFlag && internalResult.sourcesProcessed > 0 {
		warnings = append(warnings, syncSaveMetadata(manager, metadata)...)
	}

	// Remove orphaned resources (bmz1.3)
	removed, removalWarnings := removeOrphanedResources(manager, internalResult.removedResources, sourceDisplayNames, mode)
	warnings = append(warnings, removalWarnings...)

	// Build the complete sync output struct
	summary := syncSummary{
		SourcesTotal:  len(manifest.Sources),
		SourcesSynced: internalResult.sourcesProcessed,
		SourcesFailed: internalResult.sourcesFailed,
	}
	for _, sr := range sourceResults {
		if sr.Result != nil {
			summary.ResourcesAdded += len(sr.Result.Added)
			summary.ResourcesUpdated += len(sr.Result.Updated)
			summary.ResourcesFailed += len(sr.Result.Failed)
		}
	}
	summary.ResourcesRemoved = len(removed)

	so := &syncOutput{
		Sources:  sourceResults,
		Removed:  removed,
		Summary:  summary,
		Warnings: warnings,
	}
	if so.Removed == nil {
		so.Removed = []removedResource{}
	}

	// Format and print output
	if err := renderSyncOutput(so, format, syncVerboseFlag); err != nil {
		return newOperationalFailureError(err)
	}

	// Regenerate modifications (skip in dry-run mode)
	if !syncDryRunFlag {
		syncRegenerateModifications(manager, repoPath)
	}

	if internalResult.sourcesFailed == 0 {
		return nil
	}

	if internalResult.sourcesFailed == len(manifest.Sources) {
		return newCompletedWithFindingsError("repository sync completed: all sources failed")
	}

	return newCompletedWithFindingsError("repository sync completed with source failures")
}
