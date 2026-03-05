package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/sourcemetadata"
)

var (
	removeDryRunFlag        bool
	removeKeepResourcesFlag bool
)

// repoRemoveCmd represents the remove command
var repoRemoveCmd = &cobra.Command{
	Use:   "remove <name|path|url>",
	Short: "Remove a source from the repository",
	Long: `Remove a source from the repository (symmetrical with 'repo add').

This command removes a source entry from ai.repo.yaml and by default also removes
any resources that came from that source (orphan cleanup).

This command operates on SOURCES (not individual resources). It is symmetrical
with 'repo add', which adds sources.

To remove individual resources, modify the source and run 'repo sync'.

Matching Priority:
  Sources are matched by name first, then by path, then by URL.

Orphan Cleanup:
  By default, resources that came from the removed source are deleted (orphans).
  Use --keep-resources to preserve resources while removing the source entry.

Examples:
  # Remove source by name
  aimgr repo remove my-source

  # Remove source by path
  aimgr repo remove ~/my-resources/

  # Remove source by URL
  aimgr repo remove https://github.com/owner/repo

  # Preview what would be removed
  aimgr repo remove my-source --dry-run

  # Remove source but keep resources
  aimgr repo remove my-source --keep-resources`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeSourceNames,
	RunE:              runRemove,
}

func runRemove(cmd *cobra.Command, args []string) error {
	nameOrPathOrURL := args[0]

	mgr, err := NewManagerWithLogLevel()
	if err != nil {
		return err
	}

	return performRemove(mgr, nameOrPathOrURL, removeDryRunFlag, removeKeepResourcesFlag)
}

// performRemove removes a source from the manifest and optionally cleans up orphaned resources
func performRemove(mgr *repo.Manager, nameOrPathOrURL string, dryRun bool, keepResources bool) error {
	repoPath := mgr.GetRepoPath()

	// Load manifest
	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Find source by name, path, or URL
	source, found := manifest.GetSource(nameOrPathOrURL)
	if !found {
		return fmt.Errorf("source not found: %s\n\nAvailable sources:\n%s",
			nameOrPathOrURL, formatSourcesList(manifest))
	}

	// Find orphaned resources (resources from this source)
	orphanedResources := []resource.Resource{}
	if !keepResources {
		resources, err := mgr.List(nil)
		if err != nil {
			return fmt.Errorf("failed to list resources: %w", err)
		}

		for _, res := range resources {
			// Check if resource came from this source (uses Manager.HasSource for DEBUG logging).
			// Try source ID first for precise matching (handles renamed sources),
			// then fall back to source name for backward compatibility.
			matched := false
			if source.ID != "" {
				matched = mgr.HasSource(res.Name, res.Type, source.ID)
			}
			if !matched {
				matched = mgr.HasSource(res.Name, res.Type, source.Name)
			}
			if matched {
				orphanedResources = append(orphanedResources, res)
			}
		}
	}

	// Warn about breaking project symlinks when orphaned resources will be removed
	if len(orphanedResources) > 0 {
		warnProjectSymlinks(orphanedResources)
	}

	// Dry run mode: show what would be removed
	if dryRun {
		fmt.Printf("Would remove source: %s\n", source.Name)
		fmt.Printf("  Type: %s\n", getSourceType(source))
		fmt.Printf("  Location: %s\n", getSourceLocation(source))

		if keepResources {
			fmt.Println("\nResources would be kept (--keep-resources)")
		} else if len(orphanedResources) > 0 {
			fmt.Printf("\nWould remove %d orphaned resource(s):\n", len(orphanedResources))
			for _, res := range orphanedResources {
				fmt.Printf("  - %s/%s\n", res.Type, res.Name)
			}
		} else {
			fmt.Println("\nNo orphaned resources found")
		}

		return nil
	}

	// Remove source from manifest
	_, err = manifest.RemoveSource(nameOrPathOrURL)
	if err != nil {
		return fmt.Errorf("failed to remove source: %w", err)
	}

	// Save manifest
	if err := manifest.Save(repoPath); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	// Clean up source metadata (regardless of --keep-resources)
	sourceMetadata, err := sourcemetadata.Load(repoPath)
	if err == nil {
		removed := false
		if source.ID != "" {
			if _, exists := sourceMetadata.Sources[source.ID]; exists {
				sourceMetadata.Delete(source.ID)
				removed = true
			}
		}
		if !removed {
			sourceMetadata.Delete(source.Name)
		}
		if err := sourceMetadata.Save(repoPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update source metadata: %v\n", err)
		}
	}

	// Commit manifest changes to git
	if err := mgr.CommitChanges("aimgr: remove source from manifest"); err != nil {
		// Don't fail if commit fails (e.g., not a git repo)
		fmt.Fprintf(os.Stderr, "Warning: Failed to commit manifest: %v\n", err)
	}

	fmt.Printf("✓ Removed source: %s\n", source.Name)

	// Remove orphaned resources unless --keep-resources is specified
	if !keepResources {
		if len(orphanedResources) > 0 {
			removeCount := 0
			for _, res := range orphanedResources {
				if err := mgr.Remove(res.Name, res.Type); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to remove %s/%s: %v\n",
						res.Type, res.Name, err)
				} else {
					removeCount++
				}
			}
			fmt.Printf("✓ Removed %d orphaned resource(s)\n", removeCount)
		} else {
			fmt.Println("  No orphaned resources found")
		}
	} else {
		fmt.Println("  Resources kept (--keep-resources)")
	}

	return nil
}

// getSourceType returns a human-readable source type
func getSourceType(source *repomanifest.Source) string {
	if source.URL != "" {
		return "remote"
	}
	return "local"
}

// getSourceLocation returns the path or URL of the source
func getSourceLocation(source *repomanifest.Source) string {
	if source.URL != "" {
		location := source.URL
		if source.Ref != "" {
			location += "@" + source.Ref
		}
		if source.Subpath != "" {
			location += " (subpath: " + source.Subpath + ")"
		}
		return location
	}
	return source.Path
}

// formatSourcesList formats the list of available sources for error messages
func formatSourcesList(manifest *repomanifest.Manifest) string {
	if len(manifest.Sources) == 0 {
		return "  (no sources configured)"
	}

	var lines []string
	for _, source := range manifest.Sources {
		location := getSourceLocation(source)
		lines = append(lines, fmt.Sprintf("  - %s (%s)", source.Name, location))
	}
	return strings.Join(lines, "\n")
}

// warnProjectSymlinks prints a warning when removing resources that may be
// installed in project directories. Since there is no central registry of
// project installations, this emits a generic warning directing users to
// run 'aimgr verify' in affected projects.
func warnProjectSymlinks(resources []resource.Resource) {
	var names []string
	for _, res := range resources {
		names = append(names, fmt.Sprintf("%s/%s", res.Type, res.Name))
	}
	fmt.Fprintf(os.Stderr, "⚠ Warning: removing %d resource(s) (%s) may break symlinks in projects that have them installed.\n",
		len(resources), strings.Join(names, ", "))
	fmt.Fprintf(os.Stderr, "  Run 'aimgr repair' in affected projects to clean up broken symlinks.\n\n")
}

func init() {
	repoCmd.AddCommand(repoRemoveCmd)
	repoRemoveCmd.Flags().BoolVar(&removeDryRunFlag, "dry-run", false, "Show what would be removed without actually removing")
	repoRemoveCmd.Flags().BoolVar(&removeKeepResourcesFlag, "keep-resources", false, "Remove source from manifest but keep resources")
}
