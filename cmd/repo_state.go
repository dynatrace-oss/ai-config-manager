package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repolock"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
)

func repoManifestPath(repoPath string) string {
	return filepath.Join(repoPath, repomanifest.ManifestFileName)
}

func repoPathExists(repoPath string) (bool, error) {
	info, err := os.Stat(repoPath)
	if err == nil {
		return info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("failed to access repository path %s: %w", repoPath, err)
}

func repoInitialized(repoPath string) (bool, error) {
	_, err := os.Stat(repoManifestPath(repoPath))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("failed to access manifest %s: %w", repoManifestPath(repoPath), err)
}

func ensureRepoInitialized(manager *repo.Manager) error {
	initialized, err := repoInitialized(manager.GetRepoPath())
	if err != nil {
		return err
	}
	if !initialized {
		return missingRepoInitializationError(manager.GetRepoPath())
	}

	return nil
}

func missingRepoInitializationError(repoPath string) error {
	return fmt.Errorf("%s not found in %s; run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first", repomanifest.ManifestFileName, repoPath)
}

func missingRepoPathError(repoPath string) error {
	return fmt.Errorf("repository does not exist at: %s", repoPath)
}

func acquireRepoReadLockIfRepoExists(ctx context.Context, manager *repo.Manager) (*repolock.Lock, bool, error) {
	exists, err := repoPathExists(manager.GetRepoPath())
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}

	lock, err := manager.AcquireRepoReadLock(ctx)
	if err != nil {
		return nil, true, err
	}

	return lock, true, nil
}
