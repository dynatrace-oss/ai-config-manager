package cmd

import (
	"fmt"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/spf13/cobra"
)

var repoRebuildDryRunFlag bool

// repoRebuildCmd represents the rebuild command.
var repoRebuildCmd = &cobra.Command{
	Use:          "rebuild",
	Short:        "Reset imported state and re-import all configured sources",
	SilenceUsage: true,
	Long: `Safely rebuild repository content from ai.repo.yaml in one command.

Rebuild is a thin orchestration over soft drop + sync:
  1. Soft drop imported state while preserving ai.repo.yaml and source definitions
  2. Re-import all configured sources (same behavior as 'aimgr repo sync')

Cache behavior:
  - Rebuild clears .workspace/ caches as part of soft drop
  - .workspace/locks is preserved for lock-path stability
  - Remote sources are cloned/fetched again during re-import

Dry-run behavior:
  - Does NOT perform soft drop
  - Runs sync preview only (same as 'aimgr repo sync --dry-run')

Failure behavior:
  - Empty source list: fails without mutating repository state
  - Partial source failures: command completes with findings after attempting all sources
  - All source failures: command completes with findings

Examples:
  aimgr repo rebuild
  aimgr repo rebuild --dry-run`,
	RunE: runRebuild,
}

func runRebuild(cmd *cobra.Command, args []string) error {
	manager, err := NewManagerWithLogLevel()
	if err != nil {
		return newOperationalFailureError(fmt.Errorf("failed to create repo manager: %w", err))
	}

	if err := ensureRepoInitialized(manager); err != nil {
		return operationalMissingManifestError(cmd, err)
	}

	repoLock, err := manager.AcquireRepoWriteLock(cmd.Context())
	if err != nil {
		return wrapLockAcquireError(manager.RepoLockPath(), err)
	}
	defer func() {
		_ = repoLock.Unlock()
	}()

	if err := maybeHoldAfterRepoLock(cmd.Context(), "rebuild"); err != nil {
		return err
	}

	manifest, err := repomanifest.LoadForMutation(manager.GetRepoPath())
	if err != nil {
		return newOperationalFailureError(fmt.Errorf("failed to load manifest: %w", err))
	}
	if len(manifest.Sources) == 0 {
		return newOperationalFailureError(fmt.Errorf("no rebuild sources configured\n\nAdd sources using:\n  aimgr repo add <source>\n\nSources are automatically tracked in ai.repo.yaml"))
	}

	if repoRebuildDryRunFlag {
		fmt.Println("Rebuild dry-run: skipping soft drop, previewing sync re-import")
		return runSyncWithManager(cmd, manager, true, true)
	}

	if err := performSoftDropWithHint(manager, false); err != nil {
		return newOperationalFailureError(err)
	}

	fmt.Println("Re-importing configured sources...")
	return runSyncWithManager(cmd, manager, true, false)
}

func init() {
	repoCmd.AddCommand(repoRebuildCmd)
	repoRebuildCmd.Flags().BoolVar(&repoRebuildDryRunFlag, "dry-run", false, "Preview re-import without dropping or importing")
}
