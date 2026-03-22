package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
)

var (
	repoApplyDryRunFlag      bool
	repoApplyIncludeModeFlag string
)

// repoApplyManifestCmd represents the apply-manifest command.
var repoApplyManifestCmd = &cobra.Command{
	Use:   "apply-manifest <path-or-url>",
	Short: "Merge an external manifest into the local ai.repo.yaml",
	Long: `Read an external ai.repo.yaml and merge its sources into the local ai.repo.yaml.

Accepted inputs in v1:
  - Local path to ai.repo.yaml
  - Stdin via - or /dev/stdin
  - HTTP(S) URL that points directly to ai.repo.yaml

GitHub URL note:
  - Use raw file URLs (for example raw.githubusercontent.com/.../ai.repo.yaml)
  - GitHub web URLs with /blob/<ref>/... or /tree/<ref>/... are not valid manifest file URLs

Manifest relationship:
  - repo show-manifest reads and prints the current local ai.repo.yaml
  - repo apply-manifest <path-or-url> reads another ai.repo.yaml and merges it into that same local file

Merge behavior:
  - New source name: added
  - Apply is additive: existing local sources not present in the incoming manifest are kept
  - Same source name + identical definition: no-op
  - Same source name + different location: conflict (never overwritten)
  - Same location with different include filters: replace or preserve (configurable)
  - To remove stale sources after a team manifest changes, use repo drop-source explicitly

Fresh repository behavior:
  - apply-manifest auto-initializes the local repository (same bootstrap as repo init)
  - in --dry-run mode, bootstrap is previewed and not persisted

Relationship to repo init:
  - repo init bootstraps local repository structure only
	  - repo apply-manifest <path-or-url> bootstraps if needed, then merges manifest content into local ai.repo.yaml`,
	Example: `  # Show the current local manifest
	  aimgr repo show-manifest

	  # Apply a manifest from local disk
	  aimgr repo apply-manifest ./ai.repo.yaml
	  aimgr repo apply-manifest /tmp/team/ai.repo.yaml

	  # Apply a shared manifest from URL
	  aimgr repo apply-manifest https://example.com/platform/ai.repo.yaml

	  # Apply a pinned shared manifest from a GitHub tag/ref
	  aimgr repo apply-manifest https://raw.githubusercontent.com/example/platform-configs/v1.2.0/manifests/ai.repo.yaml

	  # Recommended bootstrap flow for developers and CI
	  aimgr repo apply-manifest https://example.com/platform/ai.repo.yaml
	  aimgr repo sync
	  aimgr install

	  # Re-apply is additive; stale sources are removed explicitly
	  aimgr repo drop-source old-source

	  # Pipe the current manifest back into apply-manifest (no-op round-trip)
	  aimgr repo show-manifest | aimgr repo apply-manifest -

	  # Preview merge actions without writing
	  aimgr repo apply-manifest ./ai.repo.yaml --dry-run

	  # Preserve existing include filters when source location matches
	  aimgr repo apply-manifest ./ai.repo.yaml --include-mode preserve`,
	Args: cobra.ExactArgs(1),
	RunE: runApplyManifest,
}

func runApplyManifest(cmd *cobra.Command, args []string) error {
	input := args[0]

	mgr, err := NewManagerWithLogLevel()
	if err != nil {
		return err
	}

	repoLock, err := mgr.AcquireRepoLock(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to acquire repository lock at %s: %w", mgr.RepoLockPath(), err)
	}
	defer func() {
		_ = repoLock.Unlock()
	}()

	if err := maybeHoldAfterRepoLock(cmd.Context(), "apply-manifest"); err != nil {
		return err
	}

	if !repoApplyDryRunFlag {
		if err := mgr.Init(); err != nil {
			return fmt.Errorf("failed to initialize repository for apply: %w", err)
		}
	}

	incoming, err := repomanifest.LoadForApply(input)
	if err != nil {
		return err
	}

	current, err := repomanifest.LoadForMutation(mgr.GetRepoPath())
	if err != nil {
		return fmt.Errorf("failed to load local manifest: %w", err)
	}

	merged, report, err := repomanifest.MergeForApply(current, incoming, repomanifest.ApplyMergeOptions{
		IncludeMode: repomanifest.IncludeMergeMode(repoApplyIncludeModeFlag),
	})
	if err != nil {
		return err
	}

	printApplyReport(report, repoApplyDryRunFlag)

	if report.HasConflicts() {
		return fmt.Errorf("manifest apply has %d conflict(s); resolve conflicts and retry:\n  - %s", report.Conflicts(), strings.Join(conflictMessages(report), "\n  - "))
	}

	if repoApplyDryRunFlag {
		fmt.Println("\nDry-run complete: no changes were written")
		return nil
	}

	if report.Added() == 0 && report.Updated() == 0 {
		fmt.Println("\nNo changes to apply")
		return nil
	}

	if err := merged.Save(mgr.GetRepoPath()); err != nil {
		return fmt.Errorf("failed to save merged manifest: %w", err)
	}

	if err := mgr.CommitChangesForPaths("aimgr: apply manifest sources", []string{repomanifest.ManifestFileName}); err != nil {
		fmt.Printf("Warning: Failed to commit manifest: %v\n", err)
	}

	fmt.Println("\n✓ Applied manifest successfully")

	return nil
}

func conflictMessages(report *repomanifest.ApplyMergeReport) []string {
	if report == nil {
		return nil
	}

	messages := make([]string, 0, report.Conflicts())
	for _, change := range report.Changes {
		if change.Action != repomanifest.ApplyActionConflict {
			continue
		}
		messages = append(messages, fmt.Sprintf("%s: %s", change.Name, change.Message))
	}

	return messages
}

func printApplyReport(report *repomanifest.ApplyMergeReport, dryRun bool) {
	header := "Manifest apply results"
	if dryRun {
		header = "Manifest apply dry-run results"
	}
	fmt.Println(header + ":")

	for _, change := range report.Changes {
		fmt.Printf("  - %s: %s (%s)\n", change.Action, change.Name, change.Message)
	}

	fmt.Printf("\nSummary: added=%d updated=%d noop=%d conflicts=%d\n",
		report.Added(), report.Updated(), report.NoOp(), report.Conflicts())
}

func init() {
	repoCmd.AddCommand(repoApplyManifestCmd)

	repoApplyManifestCmd.Flags().BoolVar(&repoApplyDryRunFlag, "dry-run", false, "Preview merge actions without writing ai.repo.yaml")
	repoApplyManifestCmd.Flags().StringVar(&repoApplyIncludeModeFlag, "include-mode", string(repomanifest.IncludeMergeReplace), "Include handling for same-location sources: replace or preserve")
}
