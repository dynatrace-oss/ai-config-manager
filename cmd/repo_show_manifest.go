package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
)

// repoShowManifestCmd represents the show-manifest command.
var repoShowManifestCmd = &cobra.Command{
	Use:   "show-manifest",
	Short: "Print the current local ai.repo.yaml",
	Long: `Read and print the current local ai.repo.yaml.

Manifest relationship:
  - repo show-manifest reads the current local ai.repo.yaml
  - repo apply-manifest <path-or-url> reads another ai.repo.yaml and merges it into that same local file

This command is read-only. It does not initialize the repository or modify ai.repo.yaml.

Examples:
  aimgr repo show-manifest
  AIMGR_REPO_PATH=/tmp/team-repo aimgr repo show-manifest`,
	RunE: runShowManifest,
}

func runShowManifest(cmd *cobra.Command, args []string) error {
	mgr, err := NewManagerWithLogLevel()
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(mgr.GetRepoPath(), repomanifest.ManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found in %s; run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first", repomanifest.ManifestFileName, mgr.GetRepoPath())
		}
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	_, err = cmd.OutOrStdout().Write(data)
	return err
}

func init() {
	repoCmd.AddCommand(repoShowManifestCmd)
}
