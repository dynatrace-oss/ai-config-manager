package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/modifications"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
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
type resourceInfo = repo.SyncResourceInfo

// sourceSyncResult holds the result for one source.
type sourceSyncResult struct {
	Name            string                      `json:"name"`
	URL             string                      `json:"url,omitempty"`
	Path            string                      `json:"path,omitempty"`
	Mode            string                      `json:"mode"` // "remote" or "local"
	Result          *output.BulkOperationResult `json:"result"`
	RemovedCount    int                         `json:"removed_count"`
	Warnings        []string                    `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	DiscoveryIssues []renderedDiscoveryIssue    `json:"discovery_issues,omitempty" yaml:"discovery_issues,omitempty"`
	Error           string                      `json:"error,omitempty"`
	Failed          bool                        `json:"failed"`
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
	return repo.CollectResourcesBySource(repoPath)
}

var (
	syncSkipExistingFlag bool
	syncDryRunFlag       bool
	syncPruneFlag        bool
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
  - Reconciling stale source-owned state with --prune when needed

By default, existing resources will be overwritten (force mode). Use --skip-existing
to skip resources that already exist in the repository.

The ai.repo.yaml file is automatically maintained when you use "aimgr repo add".

Include filters (set via "aimgr repo add --filter") are stored in ai.repo.yaml
and respected during sync: only resources matching the include patterns are imported
for that source. Sources without include filters import all resources.

Use --prune to reconcile stale source-owned resources and packages after include,
subpath, or discovery changes. Prune cleanup is source-aware and only targets
resources that no longer match a synced source's effective definition.

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
  aimgr repo sync --dry-run

  # Reconcile stale source-owned resources/packages
  aimgr repo sync --prune

  # Preview prune cleanup without changing the repository
  aimgr repo sync --dry-run --prune`,
	RunE: runSync,
}

func init() {
	repoCmd.AddCommand(syncCmd)

	// Add flags
	syncCmd.Flags().BoolVar(&syncSkipExistingFlag, "skip-existing", false, "Skip conflicts silently")
	syncCmd.Flags().BoolVar(&syncDryRunFlag, "dry-run", false, "Preview without importing")
	syncCmd.Flags().BoolVar(&syncPruneFlag, "prune", false, "Remove stale source-owned resources/packages after sync reconciliation")
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

		sourcePath, err := prepareRemoteSourcePath(wsMgr, cloneURL, parsed.Ref)
		if err != nil {
			return "", err
		}

		if parsed.Subpath != "" {
			sourcePath = filepath.Join(sourcePath, parsed.Subpath)
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

	// Manifest-driven operations should use the persisted ref/subpath fields.
	// Compatibility fallback: when BOTH fields are empty, keep parser-derived
	// inline values so legacy manifests with inline URL coordinates continue to
	// function until explicitly migrated.
	if strings.TrimSpace(src.Ref) != "" || strings.TrimSpace(src.Subpath) != "" {
		parsed.Ref = src.Ref
		parsed.Subpath = src.Subpath
	}

	return parsed, nil
}

func applyIncludeFilterToDiscovered(sourceResources map[resource.ResourceType]map[string]bool, include []string) error {
	return repo.ApplyIncludeFilterToDiscovered(sourceResources, include)
}

func canonicalSourceID(src *repomanifest.Source) string {
	return repo.CanonicalSourceID(src)
}

func sourceLegacyRemoteIDAlias(src *repomanifest.Source) (legacyID string, canonicalID string) {
	return repo.SourceLegacyRemoteIDAlias(src)
}

func remapLegacyRemoteSourceIDs(preSyncResources map[string][]resourceInfo, manifest *repomanifest.Manifest) {
	repo.RemapLegacyRemoteSourceIDs(preSyncResources, manifest)
}

func sourceLocationSummary(src *repomanifest.Source) string {
	return repo.SourceLocationSummary(src)
}

func detectSyncResourceCollisions(manifest *repomanifest.Manifest, manager *repo.Manager) error {
	return repo.DetectSyncResourceCollisions(manifest, manager, resolveSourcePathForSync, scanSourceResources)
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
func syncSource(src *repomanifest.Source, manager *repo.Manager) (string, *output.BulkOperationResult, []discovery.DiscoveryError, error) {
	sourcePath, err := resolveSourcePathForSync(src, manager)
	if err != nil {
		return "", nil, nil, err
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
		return "", nil, nil, fmt.Errorf("source must have either URL or Path")
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
	discovered, err := discoverImportResourcesByMode(sourcePath, src.Discovery)
	if err != nil {
		return "", nil, nil, err
	}

	bulkResult, err := importDiscoveredResources(sourcePath, manager, src.Include, sourceURL, sourceType, src.Ref, mode, src.Name, src.ID, discovered)
	if err != nil {
		return "", bulkResult, discovered.discoveryErrors, err
	}
	return sourcePath, bulkResult, discovered.discoveryErrors, nil
}

// syncResult tracks the outcome of processing all sources.
type syncResult struct {
	sourcesProcessed int
	sourcesFailed    int
	failedSources    []string
	removedResources map[string][]resourceInfo
}

type syncRunState struct {
	manifest           *repomanifest.Manifest
	metadata           *sourcemetadata.SourceMetadata
	format             output.Format
	mode               syncOutputMode
	repoPath           string
	sourceDisplayNames map[string]string
	preSyncResources   map[string][]resourceInfo
	warnings           []string
}

func applySyncDryRunOverride(overrideDryRun bool) func() {
	effectiveDryRun := syncDryRunFlag
	if overrideDryRun {
		effectiveDryRun = true
	}

	originalSyncDryRun := syncDryRunFlag
	syncDryRunFlag = effectiveDryRun

	return func() {
		syncDryRunFlag = originalSyncDryRun
	}
}

func acquireSyncRunLock(cmd *cobra.Command, manager *repo.Manager, lockAlreadyHeld bool) (unlocker, error) {
	if lockAlreadyHeld {
		return nil, nil
	}

	repoLock, err := manager.AcquireRepoWriteLock(cmd.Context())
	if err != nil {
		return nil, wrapLockAcquireError(manager.RepoLockPath(), err)
	}
	if err := maybeHoldAfterRepoLock(cmd.Context(), "sync"); err != nil {
		_ = repoLock.Unlock()
		return nil, err
	}

	return repoLock, nil
}

func emptySourceMetadata() *sourcemetadata.SourceMetadata {
	return &sourcemetadata.SourceMetadata{
		Version: 1,
		Sources: make(map[string]*sourcemetadata.SourceState),
	}
}

func loadSyncMetadata(repoPath string) *sourcemetadata.SourceMetadata {
	metadata, err := sourcemetadata.Load(repoPath)
	if err == nil {
		return metadata
	}

	return emptySourceMetadata()
}

func collectPreSyncInventoryForSync(repoPath string, manifest *repomanifest.Manifest) (map[string][]resourceInfo, []string) {
	preSyncResources, err := collectResourcesBySource(repoPath)
	if err != nil {
		return make(map[string][]resourceInfo), []string{fmt.Sprintf("could not collect pre-sync inventory: %v", err)}
	}

	remapLegacyRemoteSourceIDs(preSyncResources, manifest)
	return preSyncResources, nil
}

func prepareSyncRunState(manager *repo.Manager) (*syncRunState, error) {
	if err := ensureRepoInitialized(manager); err != nil {
		return nil, newOperationalFailureError(err)
	}

	manifest, err := repomanifest.LoadForMutation(manager.GetRepoPath())
	if err != nil {
		return nil, newOperationalFailureError(fmt.Errorf("failed to load manifest: %w", err))
	}
	if err := detectSyncResourceCollisions(manifest, manager); err != nil {
		return nil, newOperationalFailureError(err)
	}
	if len(manifest.Sources) == 0 {
		return nil, newOperationalFailureError(fmt.Errorf("no sync sources configured\n\nAdd sources using:\n  aimgr repo add <source>\n\nSources are automatically tracked in ai.repo.yaml"))
	}

	format, err := output.ParseFormat(syncFormatFlag)
	if err != nil {
		return nil, newOperationalFailureError(err)
	}

	repoPath := manager.GetRepoPath()
	preSyncResources, warnings := collectPreSyncInventoryForSync(repoPath, manifest)

	return &syncRunState{
		manifest:           manifest,
		metadata:           loadSyncMetadata(repoPath),
		format:             format,
		mode:               syncOutputMode{format: format, verbose: syncVerboseFlag},
		repoPath:           repoPath,
		sourceDisplayNames: buildSourceDisplayNames(manifest.Sources),
		preSyncResources:   preSyncResources,
		warnings:           warnings,
	}, nil
}

func printSyncStart(state *syncRunState) {
	if !state.mode.human() {
		return
	}

	fmt.Printf("Syncing from %d configured source(s)...\n", len(state.manifest.Sources))
	if syncDryRunFlag {
		fmt.Println("Mode: DRY RUN (preview only)")
	}
	if syncPruneFlag {
		fmt.Println("Prune mode: enabled (--prune)")
	}
	fmt.Println()
}

func applySyncOperationFlags() func() {
	originalForceFlag := forceFlag
	originalDryRunFlag := dryRunFlag
	originalSkipExistingFlag := skipExistingFlag
	originalAddFormatFlag := addFormatFlag
	originalSyncSilentMode := syncSilentMode

	forceFlag = !syncSkipExistingFlag
	skipExistingFlag = syncSkipExistingFlag
	dryRunFlag = syncDryRunFlag
	addFormatFlag = syncFormatFlag
	syncSilentMode = true

	return func() {
		forceFlag = originalForceFlag
		dryRunFlag = originalDryRunFlag
		skipExistingFlag = originalSkipExistingFlag
		addFormatFlag = originalAddFormatFlag
		syncSilentMode = originalSyncSilentMode
	}
}

func buildSourceSyncResult(src *repomanifest.Source) sourceSyncResult {
	sr := sourceSyncResult{Name: src.Name}
	if src.URL != "" {
		sr.URL = src.URL
		sr.Mode = "remote"
		return sr
	}

	sr.Path = src.Path
	sr.Mode = "local"
	return sr
}

func updateSourceMetadataAfterSync(metadata *sourcemetadata.SourceMetadata, src *repomanifest.Source) {
	now := time.Now()
	canonicalID := canonicalSourceID(src)
	if state, ok := metadata.Sources[src.Name]; ok {
		state.LastSynced = now
		if canonicalID != "" {
			state.SourceID = canonicalID
		}
		return
	}

	metadata.Sources[src.Name] = &sourcemetadata.SourceState{
		SourceID:   canonicalID,
		Added:      now,
		LastSynced: now,
	}
}

func syncManifestSources(state *syncRunState, manager *repo.Manager) ([]sourceSyncResult, syncResult, []string) {
	orchestration := repo.RunSyncOrchestrator(repo.SyncOrchestratorDeps{
		Manager:          manager,
		Manifest:         state.manifest,
		RepoPath:         state.repoPath,
		PreSyncResources: state.preSyncResources,
		DryRun:           syncDryRunFlag,
		Prune:            syncPruneFlag,
		SyncSource: func(src *repomanifest.Source, manager *repo.Manager) (string, any, []discovery.DiscoveryError, error) {
			return syncSource(src, manager)
		},
		SourceScanner: scanSourceResources,
		Logger:        slog.Default(),
		OnSourceStart: func(src *repomanifest.Source) {
			if state.mode.human() {
				sr := buildSourceSyncResult(src)
				fmt.Printf("  Syncing %s (%s)...\n", sr.Name, sr.Mode)
			}
		},
		OnDiscoveryError: func(sourceName string, discoveryErrors []discovery.DiscoveryError) {
			if state.mode.human() {
				fmt.Println()
				printDiscoveryErrorsForSource(sourceName, discoveryErrors)
				fmt.Println()
			}
		},
		UpdateMetadata: func(src *repomanifest.Source) {
			updateSourceMetadataAfterSync(state.metadata, src)
		},
	})

	internalResult := syncResult{
		sourcesProcessed: orchestration.SourcesProcessed,
		sourcesFailed:    orchestration.SourcesFailed,
		failedSources:    orchestration.FailedSources,
		removedResources: orchestration.RemovedResources,
	}

	sourceResults := make([]sourceSyncResult, 0, len(orchestration.SourceResults))
	for _, run := range orchestration.SourceResults {
		sr := buildSourceSyncResult(run.Source)
		if run.Result != nil {
			if bulk, ok := run.Result.(*output.BulkOperationResult); ok {
				sr.Result = bulk
			}
		}
		sr.RemovedCount = run.RemovedCount
		if len(run.DiscoveryErrors) > 0 {
			sr.DiscoveryIssues = renderDiscoveryIssues(run.DiscoveryErrors)
			sr.Warnings = append(sr.Warnings, fmt.Sprintf("%d resource(s) were skipped during discovery due to validation issues", len(sr.DiscoveryIssues)))
		}
		if run.Failed && run.Error != nil {
			sr.Failed = true
			sr.Error = run.Error.Error()
		}
		sourceResults = append(sourceResults, sr)
	}

	return sourceResults, internalResult, orchestration.Warnings
}

func buildSyncOutput(sourceResults []sourceSyncResult, removed []removedResource, warnings []string, sourcesTotal int) *syncOutput {
	summary := syncSummary{
		SourcesTotal:     sourcesTotal,
		ResourcesRemoved: len(removed),
	}
	for _, sr := range sourceResults {
		if sr.Failed {
			summary.SourcesFailed++
			continue
		}

		summary.SourcesSynced++
		if sr.Result == nil {
			continue
		}

		summary.ResourcesAdded += len(sr.Result.Added)
		summary.ResourcesUpdated += len(sr.Result.Updated)
		summary.ResourcesFailed += len(sr.Result.Failed)
	}

	so := &syncOutput{
		Sources:  sourceResults,
		Removed:  removed,
		Summary:  summary,
		Warnings: warnings,
	}
	if so.Removed == nil {
		so.Removed = []removedResource{}
	}

	return so
}

func syncCompletionError(failedSources, totalSources int) error {
	if failedSources == 0 {
		return nil
	}
	if failedSources == totalSources {
		return newCompletedWithFindingsError("repository sync completed: all sources failed")
	}

	return newCompletedWithFindingsError("repository sync completed with source failures")
}

// detectRemovedForSource compares a source's pre-sync resource inventory with the
// current source contents to identify resources that were removed from the source.
func detectRemovedForSource(src *repomanifest.Source, sourcePath, repoPath string,
	preSyncResources map[string][]resourceInfo) ([]resourceInfo, []string) {
	return repo.DetectRemovedForSource(src, sourcePath, repoPath, preSyncResources, scanSourceResources, slog.Default())
}

func preSyncResourcesForSource(preSyncResources map[string][]resourceInfo, sourceKey, sourceName string) []resourceInfo {
	return repo.PreSyncResourcesForSource(preSyncResources, sourceKey, sourceName)
}

func resourceBelongsToSource(name string, resType resource.ResourceType, sourceIdentifier, sourceName, repoPath string) bool {
	return repo.ResourceBelongsToSource(name, resType, sourceIdentifier, sourceName, repoPath)
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
			fmt.Printf("\nWould prune %d resource(s) no longer in effective source definitions:\n", totalToRemove)
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
		fmt.Printf("Pruning %d resource(s) no longer in effective source definitions:\n", totalToRemove)
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
	manager, err := NewManagerWithLogLevel()
	if err != nil {
		return newOperationalFailureError(fmt.Errorf("failed to create repo manager: %w", err))
	}

	return runSyncWithManager(cmd, manager, false, syncDryRunFlag)
}

func runSyncWithManager(cmd *cobra.Command, manager *repo.Manager, lockAlreadyHeld bool, overrideDryRun bool) error {
	restoreDryRun := applySyncDryRunOverride(overrideDryRun)
	defer restoreDryRun()

	if err := ensureRepoInitialized(manager); err != nil {
		return newOperationalFailureError(err)
	}

	repoLock, err := acquireSyncRunLock(cmd, manager, lockAlreadyHeld)
	if err != nil {
		return err
	}
	if repoLock != nil {
		defer func() {
			_ = repoLock.Unlock()
		}()
	}

	state, err := prepareSyncRunState(manager)
	if err != nil {
		return err
	}
	printSyncStart(state)

	restoreFlags := applySyncOperationFlags()
	defer restoreFlags()

	sourceResults, internalResult, sourceWarnings := syncManifestSources(state, manager)
	state.warnings = append(state.warnings, sourceWarnings...)

	if !syncDryRunFlag && internalResult.sourcesProcessed > 0 {
		state.warnings = append(state.warnings, syncSaveMetadata(manager, state.metadata)...)
	}

	var removed []removedResource
	if syncPruneFlag {
		var removalWarnings []string
		removed, removalWarnings = removeOrphanedResources(manager, internalResult.removedResources, state.sourceDisplayNames, state.mode)
		state.warnings = append(state.warnings, removalWarnings...)
	}

	so := buildSyncOutput(sourceResults, removed, state.warnings, len(state.manifest.Sources))
	if err := renderSyncOutput(so, state.format, syncVerboseFlag); err != nil {
		return newOperationalFailureError(err)
	}

	// Regenerate modifications (skip in dry-run mode)
	if !syncDryRunFlag {
		syncRegenerateModifications(manager, state.repoPath)
	}

	return syncCompletionError(internalResult.sourcesFailed, len(state.manifest.Sources))
}
