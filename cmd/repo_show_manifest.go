package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
)

// repoShowManifestCmd represents the show-manifest command.
var repoShowManifestCmd = &cobra.Command{
	Use:   "show-manifest",
	Short: "Print the current local ai.repo.yaml",
	Long: `Read and print the current local ai.repo.yaml.

Manifest relationship:
  - repo show-manifest reads local state and prints a shareable ai.repo.yaml view
  - repo apply-manifest <path-or-url> reads another ai.repo.yaml and merges it into that same local file

Override behavior:
  - Active local overrides are shown in 'repo info'
  - show-manifest intentionally hides local-only override runtime state and emits the restore remote definition
  - Clear overrides before sharing if you want the exact local active transport reflected

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

	if err := ensureRepoInitialized(mgr); err != nil {
		return err
	}

	repoLock, err := mgr.AcquireRepoReadLock(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to acquire repository read lock at %s: %w", mgr.RepoLockPath(), err)
	}
	defer func() {
		_ = repoLock.Unlock()
	}()

	if err := maybeHoldAfterRepoLock(cmd.Context(), "show-manifest"); err != nil {
		return err
	}

	manifestPath := repoManifestPath(mgr.GetRepoPath())
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return missingRepoInitializationError(mgr.GetRepoPath())
		}
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	manifest, err := repomanifest.Load(mgr.GetRepoPath())
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	shareable := manifestForShowManifest(manifest)
	data, err := yaml.Marshal(shareable)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	_, err = cmd.OutOrStdout().Write(data)
	return err
}

func manifestForShowManifest(manifest *repomanifest.Manifest) *repomanifest.Manifest {
	if manifest == nil {
		return &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{}}
	}

	projected := &repomanifest.Manifest{
		Version: manifest.Version,
		Sources: make([]*repomanifest.Source, 0, len(manifest.Sources)),
	}

	for _, src := range manifest.Sources {
		if src == nil {
			continue
		}

		view := *src
		view.ID = ""

		if src.OverrideOriginalURL != "" {
			view.Path = ""
			view.URL = src.OverrideOriginalURL
			view.Ref = src.OverrideOriginalRef
			view.Subpath = src.OverrideOriginalSubpath
		}

		view.OverrideOriginalURL = ""
		view.OverrideOriginalRef = ""
		view.OverrideOriginalSubpath = ""

		projected.Sources = append(projected.Sources, &view)
	}

	return projected
}

func init() {
	repoCmd.AddCommand(repoShowManifestCmd)
}
