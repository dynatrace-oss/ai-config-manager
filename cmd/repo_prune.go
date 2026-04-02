package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/giturl"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/workspace"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	pruneForceFlag  bool
	pruneDryRunFlag bool
	pruneFormatFlag string
)

// CachedRepo represents a cached repository to be pruned
type CachedRepo struct {
	URL  string
	Path string
	Size int64 // Size in bytes
}

// PruneResult represents the result of a prune operation
type PruneResult struct {
	UnreferencedCaches []CachedRepoInfo `json:"unreferenced_caches" yaml:"unreferenced_caches"`
	TotalCount         int              `json:"total_count" yaml:"total_count"`
	TotalSizeBytes     int64            `json:"total_size_bytes" yaml:"total_size_bytes"`
	TotalSizeHuman     string           `json:"total_size_human" yaml:"total_size_human"`
	Removed            int              `json:"removed,omitempty" yaml:"removed,omitempty"`
	Failed             int              `json:"failed,omitempty" yaml:"failed,omitempty"`
	FreedBytes         int64            `json:"freed_bytes,omitempty" yaml:"freed_bytes,omitempty"`
	FreedHuman         string           `json:"freed_human,omitempty" yaml:"freed_human,omitempty"`
	DryRun             bool             `json:"dry_run" yaml:"dry_run"`
}

// CachedRepoInfo represents detailed information about a cached repository
type CachedRepoInfo struct {
	URL       string `json:"url" yaml:"url"`
	Path      string `json:"path" yaml:"path"`
	SizeBytes int64  `json:"size_bytes" yaml:"size_bytes"`
	SizeHuman string `json:"size_human" yaml:"size_human"`
}

// repoPruneCmd represents the prune command
var repoPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove unreferenced workspace caches",
	Long: `Remove Git repositories from .workspace/ that are not referenced by any installed resources.

What gets pruned:
  - Git repository clones in .workspace/ that are not used by any current resources
  - Cached repositories from removed or outdated resources
  - Orphaned caches from failed operations

What is NOT pruned:
  - Caches referenced by currently installed resources
  - Local file sources (not cached in .workspace/)
  - Resource files themselves (only Git caches are removed)

Output Formats:
  --format=table (default): Interactive with confirmation prompts
  --format=json:  Structured JSON (best with --force or --dry-run)
  --format=yaml:  Structured YAML (best with --force or --dry-run)

Examples:
  aimgr repo prune                         # Interactive preview with confirmation
  aimgr repo prune --dry-run               # Preview without removing
  aimgr repo prune --format=json --force   # Non-interactive JSON output
  aimgr repo prune --format=yaml --dry-run # YAML preview
  aimgr repo prune --force                 # Remove without confirmation`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate format
		parsedFormat, err := output.ParseFormat(pruneFormatFlag)
		if err != nil {
			return err
		}

		manager, err := NewManagerWithLogLevel()
		if err != nil {
			return err
		}

		repoExists, err := repoPathExists(manager.GetRepoPath())
		if err != nil {
			return err
		}
		if !repoExists {
			return outputPruneResult(buildPruneResult(nil, pruneDryRunFlag), parsedFormat)
		}

		repoLock, err := manager.AcquireRepoWriteLock(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to acquire repository lock at %s: %w", manager.RepoLockPath(), err)
		}
		defer func() {
			_ = repoLock.Unlock()
		}()

		// Create workspace manager
		workspaceManager, err := workspace.NewManager(manager.GetRepoPath())
		if err != nil {
			return err
		}

		// Find unreferenced cached repos
		unreferenced, err := findUnreferencedCaches(manager, workspaceManager)
		if err != nil {
			return err
		}

		// Build result struct
		result := buildPruneResult(unreferenced, pruneDryRunFlag)

		// If no unreferenced caches found
		if len(unreferenced) == 0 {
			return outputPruneResult(result, parsedFormat)
		}

		// For table format, show interactive display
		if parsedFormat == output.Table {
			displayUnreferencedCaches(unreferenced)

			if pruneDryRunFlag {
				fmt.Printf("\n[DRY RUN] Would remove %d cached %s, freeing %s\n",
					result.TotalCount,
					pluralize("repository", "repositories", result.TotalCount),
					result.TotalSizeHuman)
				return nil
			}

			// Confirmation prompt (unless --force)
			if !pruneForceFlag {
				fmt.Printf("\nRemove %d cached %s? This will free %s. [y/N] ",
					result.TotalCount,
					pluralize("repository", "repositories", result.TotalCount),
					result.TotalSizeHuman)

				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}

				response = strings.ToLower(strings.TrimSpace(response))
				if response != "y" && response != "yes" {
					fmt.Println("Cancelled.")
					return nil
				}
			}
		}

		// Perform removal (if not dry-run)
		if !pruneDryRunFlag {
			removed, failed, freedSize := removeUnreferencedCaches(workspaceManager, unreferenced)
			result.Removed = removed
			result.Failed = failed
			result.FreedBytes = freedSize
			result.FreedHuman = formatSize(freedSize)
		}

		// Output results
		return outputPruneResult(result, parsedFormat)
	},
}

func init() {
	repoCmd.AddCommand(repoPruneCmd)
	repoPruneCmd.Flags().BoolVar(&pruneForceFlag, "force", false, "Skip confirmation prompt")
	repoPruneCmd.Flags().BoolVar(&pruneDryRunFlag, "dry-run", false, "Preview what would be removed without removing")
	repoPruneCmd.Flags().StringVar(&pruneFormatFlag, "format", "table", "Output format (table|json|yaml)")
	_ = repoPruneCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)
}

// buildPruneResult constructs a PruneResult from unreferenced caches
func buildPruneResult(unreferenced []CachedRepo, dryRun bool) *PruneResult {
	cacheInfos := make([]CachedRepoInfo, len(unreferenced))
	var totalSize int64

	for i, cache := range unreferenced {
		totalSize += cache.Size
		cacheInfos[i] = CachedRepoInfo{
			URL:       cache.URL,
			Path:      cache.Path,
			SizeBytes: cache.Size,
			SizeHuman: formatSize(cache.Size),
		}
	}

	return &PruneResult{
		UnreferencedCaches: cacheInfos,
		TotalCount:         len(unreferenced),
		TotalSizeBytes:     totalSize,
		TotalSizeHuman:     formatSize(totalSize),
		DryRun:             dryRun,
	}
}

// outputPruneResult outputs prune results in the requested format
func outputPruneResult(result *PruneResult, format output.Format) error {
	switch format {
	case output.JSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)

	case output.YAML:
		encoder := yaml.NewEncoder(os.Stdout)
		defer func() { _ = encoder.Close() }()
		return encoder.Encode(result)

	case output.Table:
		// Table output handled in RunE (interactive)
		if result.TotalCount == 0 {
			fmt.Println("No unreferenced workspace caches found.")
			return nil
		}
		if result.Removed > 0 {
			fmt.Printf("\n✓ Removed %d cached %s, freed %s",
				result.Removed,
				pluralize("repository", "repositories", result.Removed),
				result.FreedHuman)
			if result.Failed > 0 {
				fmt.Printf(" (%d failed)", result.Failed)
			}
			fmt.Println()
		}
		return nil

	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// findUnreferencedCaches finds all cached repos that are not referenced by any resource metadata
func findUnreferencedCaches(manager *repo.Manager, workspaceManager *workspace.Manager) ([]CachedRepo, error) {
	// Get all cached URLs
	cachedURLs, err := workspaceManager.ListCached()
	if err != nil {
		return nil, fmt.Errorf("failed to list cached repositories: %w", err)
	}

	// If no caches, nothing to prune
	if len(cachedURLs) == 0 {
		return []CachedRepo{}, nil
	}

	// Collect all referenced Git URLs from metadata
	referencedURLs, err := collectReferencedGitURLs(manager)
	if err != nil {
		return nil, fmt.Errorf("failed to collect referenced URLs: %w", err)
	}

	// Build set of referenced URLs (normalized) for fast lookup
	referencedSet := make(map[string]bool)
	for _, url := range referencedURLs {
		referencedSet[normalizeURL(url)] = true
	}

	// Find unreferenced caches
	var unreferenced []CachedRepo
	workspaceDir := filepath.Join(manager.GetRepoPath(), ".workspace")

	for _, cachedURL := range cachedURLs {
		normalized := normalizeURL(cachedURL)
		if !referencedSet[normalized] {
			// This cache is not referenced - calculate its path and size
			hash := workspace.ComputeHash(normalized)
			cachePath := filepath.Join(workspaceDir, hash)

			size, err := getDirSize(cachePath)
			if err != nil {
				// If we can't calculate size, use 0
				size = 0
			}

			unreferenced = append(unreferenced, CachedRepo{
				URL:  normalized,
				Path: cachePath,
				Size: size,
			})
		}
	}

	return unreferenced, nil
}

// collectReferencedGitURLs collects all Git URLs from resource metadata
func collectReferencedGitURLs(manager *repo.Manager) ([]string, error) {
	repoPath := manager.GetRepoPath()
	metadataDir := filepath.Join(repoPath, ".metadata")

	var gitURLs []string

	// Check all resource types
	resourceTypes := []resource.ResourceType{resource.Command, resource.Skill, resource.Agent}

	for _, resType := range resourceTypes {
		typeDir := filepath.Join(metadataDir, string(resType)+"s")

		// Check if type directory exists
		if _, err := os.Stat(typeDir); os.IsNotExist(err) {
			continue
		}

		// Read metadata files in directory
		entries, err := os.ReadDir(typeDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read metadata directory %s: %w", typeDir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-metadata.json") {
				continue
			}

			// Extract resource name from filename
			name := strings.TrimSuffix(entry.Name(), "-metadata.json")

			// Load metadata
			meta, err := metadata.Load(name, resType, repoPath)
			if err != nil {
				// Skip if we can't load metadata
				continue
			}

			// Only collect Git source URLs
			if isGitSource(meta.SourceType) {
				gitURLs = append(gitURLs, meta.SourceURL)
			}
		}
	}

	return gitURLs, nil
}

// isGitSource checks if a source type is a Git source
func isGitSource(sourceType string) bool {
	return sourceType == "github" || sourceType == "git-url" || sourceType == "gitlab"
}

// normalizeURL normalizes a Git URL for consistent comparison
func normalizeURL(url string) string {
	return giturl.NormalizeURL(url)
}

// getDirSize calculates the total size of a directory in bytes
func getDirSize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// displayUnreferencedCaches displays the list of unreferenced caches
func displayUnreferencedCaches(unreferenced []CachedRepo) {
	fmt.Printf("Found %d unreferenced cached %s:\n\n",
		len(unreferenced),
		pluralize("repository", "repositories", len(unreferenced)))

	for _, cache := range unreferenced {
		fmt.Printf("  • %s (%s)\n", cache.URL, formatSize(cache.Size))
	}
}

// removeUnreferencedCaches removes unreferenced caches and returns counts
func removeUnreferencedCaches(workspaceManager *workspace.Manager, unreferenced []CachedRepo) (removed int, failed int, freedSize int64) {
	for _, cache := range unreferenced {
		if err := workspaceManager.Remove(cache.URL); err != nil {
			fmt.Printf("✗ Failed to remove cache for %s: %v\n", cache.URL, err)
			failed++
		} else {
			removed++
			freedSize += cache.Size
		}
	}
	return removed, failed, freedSize
}

// formatSize formats a byte size as a human-readable string
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// pluralize returns singular or plural form based on count
func pluralize(singular, plural string, count int) string {
	if count == 1 {
		return singular
	}
	return plural
}
