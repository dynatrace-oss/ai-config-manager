package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/source"
	"github.com/spf13/cobra"
)

var repoOverrideSourceClearFlag bool

var repoOverrideSourceAutoSync = func() error {
	return runSync(syncCmd, []string{})
}

type overrideSourceResult struct {
	sourceName            string
	operation             string
	activeLocation        string
	restoreLocation       string
	restoreMetadataIntact bool
}

var repoOverrideSourceCmd = &cobra.Command{
	Use:   "override-source <source-name> <local:/path>",
	Short: "Temporarily override a remote source with a local checkout",
	Long: `Temporarily override a named remote source with a local checkout.

Use this command for development/testing workflows where a configured remote source
should point to a local repository checkout temporarily.

Modes:
  - Override: aimgr repo override-source <source-name> local:/path
  - Clear:    aimgr repo override-source <source-name> --clear

The command persists state first and then runs 'aimgr repo sync' automatically.
If sync fails after persistence, the source definition remains changed and the
error explains the active state and recovery command.`,
	Args: func(cmd *cobra.Command, args []string) error {
		return validateOverrideSourceArgs(repoOverrideSourceClearFlag, args)
	},
	ValidArgsFunction: completeSourceNames,
	RunE:              runRepoOverrideSource,
}

func init() {
	repoCmd.AddCommand(repoOverrideSourceCmd)
	repoOverrideSourceCmd.Flags().BoolVar(&repoOverrideSourceClearFlag, "clear", false, "Clear local override and restore the original remote source")
}

func validateOverrideSourceArgs(clear bool, args []string) error {
	if clear {
		switch len(args) {
		case 0:
			return fmt.Errorf("missing source name\n\nUsage:\n  aimgr repo override-source <source-name> --clear")
		case 1:
			return nil
		default:
			return fmt.Errorf("--clear cannot be combined with an override target\n\nUsage:\n  aimgr repo override-source <source-name> --clear")
		}
	}

	switch len(args) {
	case 0:
		return fmt.Errorf("missing source name and override target\n\nUsage:\n  aimgr repo override-source <source-name> local:/path")
	case 1:
		return fmt.Errorf("missing override target\n\nUsage:\n  aimgr repo override-source <source-name> local:/path")
	case 2:
		return nil
	default:
		return fmt.Errorf("accepts exactly 2 args (<source-name> <local:/path>) or 1 arg with --clear; received %d", len(args))
	}
}

func runRepoOverrideSource(cmd *cobra.Command, args []string) error {
	mgr, err := NewManagerWithLogLevel()
	if err != nil {
		return err
	}

	if err := ensureRepoInitialized(mgr); err != nil {
		return operationalMissingManifestError(cmd, err)
	}

	repoLock, err := mgr.AcquireRepoWriteLock(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to acquire repository lock at %s: %w", mgr.RepoLockPath(), err)
	}
	lockHeld := true
	defer func() {
		if lockHeld {
			_ = repoLock.Unlock()
		}
	}()

	if err := maybeHoldAfterRepoLock(cmd.Context(), "override-source"); err != nil {
		return err
	}

	manifest, err := repomanifest.LoadForMutation(mgr.GetRepoPath())
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}
	if len(args) == 0 {
		return fmt.Errorf("missing source name")
	}

	sourceName := args[0]
	src, found := findSourceByNameOnly(manifest, sourceName)
	if !found {
		return fmt.Errorf("source %q not found in manifest by name", sourceName)
	}

	var result overrideSourceResult
	if repoOverrideSourceClearFlag {
		result, err = clearSourceOverride(mgr, manifest, src)
	} else {
		if len(args) < 2 {
			return fmt.Errorf("missing override target")
		}
		overrideTarget := args[1]
		result, err = applySourceOverride(mgr, manifest, src, overrideTarget)
	}
	if err != nil {
		return err
	}

	if err := repoLock.Unlock(); err != nil {
		return fmt.Errorf("failed to release repository lock before automatic sync: %w", err)
	}
	lockHeld = false

	if err := repoOverrideSourceAutoSync(); err != nil {
		if result.restoreMetadataIntact {
			return fmt.Errorf("%s persisted for source %q, but automatic sync failed: %w\nActive source is now %s. Restore metadata is intact (%s).\nRun 'aimgr repo sync' to retry sync, or run 'aimgr repo override-source %s --clear' to restore the remote definition", result.operation, result.sourceName, err, result.activeLocation, result.restoreLocation, result.sourceName)
		}

		return fmt.Errorf("%s persisted for source %q, but automatic sync failed: %w\nActive source is now %s. Restore metadata has been removed.\nRun 'aimgr repo sync' to retry sync", result.operation, result.sourceName, err, result.activeLocation)
	}

	if result.operation == "override" {
		fmt.Printf("✓ Source %q overridden\n", result.sourceName)
		fmt.Printf("  Active source: %s\n", result.activeLocation)
		fmt.Printf("  Restore target: %s\n", result.restoreLocation)
	} else {
		fmt.Printf("✓ Source %q override cleared\n", result.sourceName)
		fmt.Printf("  Active source: %s\n", result.activeLocation)
	}

	return nil
}

func findSourceByNameOnly(manifest *repomanifest.Manifest, sourceName string) (*repomanifest.Source, bool) {
	if manifest == nil {
		return nil, false
	}

	for _, src := range manifest.Sources {
		if src != nil && src.Name == sourceName {
			return src, true
		}
	}

	return nil, false
}

func applySourceOverride(mgr *repo.Manager, manifest *repomanifest.Manifest, src *repomanifest.Source, target string) (overrideSourceResult, error) {
	if src == nil {
		return overrideSourceResult{}, fmt.Errorf("source cannot be nil")
	}

	if src.OverrideOriginalURL != "" {
		return overrideSourceResult{}, fmt.Errorf("source %q is already overridden; active source is %s\nRun 'aimgr repo override-source %s --clear' first", src.Name, sourceLocationSummary(src), src.Name)
	}

	if src.Path != "" && src.URL == "" {
		return overrideSourceResult{}, fmt.Errorf("source %q is already a local path source (%q); overriding already-local sources is not supported", src.Name, src.Path)
	}

	if src.URL == "" || src.Path != "" {
		return overrideSourceResult{}, fmt.Errorf("source %q has unsupported shape for override (expected remote URL source, got %s)", src.Name, sourceLocationSummary(src))
	}

	parsed, err := source.ParseSource(target)
	if err != nil {
		return overrideSourceResult{}, fmt.Errorf("invalid override target %q: %w", target, err)
	}
	if parsed.Type != source.Local {
		return overrideSourceResult{}, fmt.Errorf("override target must use local:/path format, got %q", target)
	}

	absPath, err := filepath.Abs(parsed.LocalPath)
	if err != nil {
		return overrideSourceResult{}, fmt.Errorf("failed to resolve local override path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return overrideSourceResult{}, fmt.Errorf("override target path %q is not accessible: %w", absPath, err)
	}
	if !info.IsDir() {
		return overrideSourceResult{}, fmt.Errorf("override target path %q must be a directory", absPath)
	}

	src.OverrideOriginalURL = src.URL
	src.OverrideOriginalRef = src.Ref
	src.OverrideOriginalSubpath = src.Subpath
	src.URL = ""
	src.Ref = ""
	src.Subpath = ""
	src.Path = absPath

	if err := manifest.Save(mgr.GetRepoPath()); err != nil {
		return overrideSourceResult{}, fmt.Errorf("failed to persist override state for source %q: %w", src.Name, err)
	}

	return overrideSourceResult{
		sourceName:            src.Name,
		operation:             "override",
		activeLocation:        sourceLocationSummary(src),
		restoreLocation:       sourceLocationSummary(&repomanifest.Source{URL: src.OverrideOriginalURL, Ref: src.OverrideOriginalRef, Subpath: src.OverrideOriginalSubpath}),
		restoreMetadataIntact: true,
	}, nil
}

func clearSourceOverride(mgr *repo.Manager, manifest *repomanifest.Manifest, src *repomanifest.Source) (overrideSourceResult, error) {
	if src == nil {
		return overrideSourceResult{}, fmt.Errorf("source cannot be nil")
	}

	if src.OverrideOriginalURL == "" {
		return overrideSourceResult{}, fmt.Errorf("source %q is not currently overridden; active source is %s", src.Name, sourceLocationSummary(src))
	}

	if src.Path == "" || src.URL != "" {
		return overrideSourceResult{}, fmt.Errorf("source %q has unsupported override shape for clear (active source is %s)", src.Name, sourceLocationSummary(src))
	}

	originalURL := src.OverrideOriginalURL
	originalRef := src.OverrideOriginalRef
	originalSubpath := src.OverrideOriginalSubpath

	src.Path = ""
	src.URL = originalURL
	src.Ref = originalRef
	src.Subpath = originalSubpath
	src.OverrideOriginalURL = ""
	src.OverrideOriginalRef = ""
	src.OverrideOriginalSubpath = ""

	if err := manifest.Save(mgr.GetRepoPath()); err != nil {
		return overrideSourceResult{}, fmt.Errorf("failed to persist clear state for source %q: %w", src.Name, err)
	}

	return overrideSourceResult{
		sourceName:            src.Name,
		operation:             "clear",
		activeLocation:        sourceLocationSummary(src),
		restoreMetadataIntact: false,
	}, nil
}
