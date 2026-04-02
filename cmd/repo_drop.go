package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
)

// repoDropCmd represents the drop command
var repoDropCmd = &cobra.Command{
	Use:   "drop",
	Short: "Remove all resources from the repository",
	Long: `Remove all resources from the repository while preserving structure (default),
or completely delete the repository directory (--full-delete).

Soft Drop (Default):
  Removes all resource files but keeps ai.repo.yaml, .git/, and directory structure.
  After soft drop, ai.repo.yaml is recreated as empty (ready for new sources).
  Use 'repo sync' to rebuild from configured sources.

Full Delete (--full-delete):
  Completely removes the entire repository directory.
  Requires typing 'yes' to confirm (unless --force is also specified).
  This is permanent and cannot be undone.

When to use soft drop:
  - Clear resources while keeping sync configuration
  - Reset repository to clean state
  - Prepare for 'repo sync' to rebuild

When to use full delete:
  - Completely remove aimgr from system
  - Migrate to a new repository location
  - Start fresh with no history

Examples:
  aimgr repo drop                              # Soft drop: remove all resources
  aimgr repo drop --full-delete                # Full delete with confirmation prompt
  aimgr repo drop --full-delete --force        # Full delete without confirmation`,
	RunE: runDrop,
}

func runDrop(cmd *cobra.Command, args []string) error {
	fullDelete, _ := cmd.Flags().GetBool("full-delete")
	force, _ := cmd.Flags().GetBool("force")

	mgr, err := NewManagerWithLogLevel()
	if err != nil {
		return err
	}

	repoPath := mgr.GetRepoPath()
	repoExists, err := repoPathExists(repoPath)
	if err != nil {
		return err
	}
	if !repoExists {
		return missingRepoPathError(repoPath)
	}

	repoLock, err := mgr.AcquireRepoWriteLock(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to acquire repository lock at %s: %w", mgr.RepoLockPath(), err)
	}
	defer func() {
		_ = repoLock.Unlock()
	}()

	if err := maybeHoldAfterRepoLock(cmd.Context(), "drop"); err != nil {
		return err
	}

	if fullDelete {
		// Full delete mode: remove entire repository directory
		return performFullDelete(mgr, force)
	}

	// Soft drop mode: remove resources, keep structure
	return performSoftDrop(mgr)
}

// performSoftDrop removes all resources but keeps ai.repo.yaml, .git/, and directory structure
func performSoftDrop(mgr *repo.Manager) error {
	// Soft drop is implemented by the manager's Drop() method
	// which removes everything and calls Init() to recreate empty structure
	if err := mgr.Drop(); err != nil {
		return err
	}

	fmt.Println("✓ Repository soft drop completed")
	fmt.Println("  All resources removed, structure preserved")
	fmt.Println("  Run 'aimgr repo sync' to rebuild from sources")
	return nil
}

// performFullDelete completely removes the entire repository directory
func performFullDelete(mgr *repo.Manager, force bool) error {
	repoPath := mgr.GetRepoPath()

	// Check if repository exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository does not exist at: %s", repoPath)
	}

	// Require confirmation unless --force is specified
	if !force {
		fmt.Printf("WARNING: This will permanently delete the entire repository at:\n")
		fmt.Printf("  %s\n\n", repoPath)
		fmt.Printf("Type 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		response = strings.TrimSpace(response)
		if response != "yes" {
			return fmt.Errorf("operation cancelled (expected 'yes', got '%s')", response)
		}
	}

	// Perform full deletion
	if err := os.RemoveAll(repoPath); err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	fmt.Println("✓ Repository fully deleted")
	fmt.Printf("  Removed: %s\n", repoPath)
	return nil
}

func init() {
	repoCmd.AddCommand(repoDropCmd)
	repoDropCmd.Flags().Bool("full-delete", false, "Completely delete entire repository directory")
	repoDropCmd.Flags().Bool("force", false, "Skip confirmation prompt (use with --full-delete)")
}
