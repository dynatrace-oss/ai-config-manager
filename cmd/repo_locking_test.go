package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
)

func TestRepoMutatingCommands_FailWhenRepoLockHeld(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to initialize setup repo: %v", err)
	}

	lock, err := manager.AcquireRepoWriteLock(context.Background())
	if err != nil {
		t.Fatalf("failed to acquire setup repo lock: %v", err)
	}
	t.Cleanup(func() {
		_ = lock.Unlock()
	})

	manifestPath := filepath.Join(t.TempDir(), "ai.repo.yaml")
	if err := os.WriteFile(manifestPath, []byte("version: 1\nsources: []\n"), 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}

	sourceDir := t.TempDir()

	oldVerifyFix := verifyFix
	verifyFix = true
	t.Cleanup(func() { verifyFix = oldVerifyFix })

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "repo init", run: func() error { return repoInitCmd.RunE(repoInitCmd, nil) }},
		{name: "repo add", run: func() error { return repoAddCmd.RunE(repoAddCmd, []string{"local:" + sourceDir}) }},
		{name: "repo sync", run: func() error { return runSync(syncCmd, nil) }},
		{name: "repo rebuild", run: func() error { return runRebuild(repoRebuildCmd, nil) }},
		{name: "repo remove", run: func() error { return runRemove(repoRemoveCmd, []string{"missing-source"}) }},
		{name: "repo drop", run: func() error { return runDrop(repoDropCmd, nil) }},
		{name: "repo apply-manifest", run: func() error { return runApplyManifest(repoApplyManifestCmd, []string{manifestPath}) }},
		{name: "repo repair", run: func() error { return repoRepairCmd.RunE(repoRepairCmd, nil) }},
		{name: "repo verify --fix", run: func() error { return repoVerifyCmd.RunE(repoVerifyCmd, nil) }},
		{name: "repo prune", run: func() error { return repoPruneCmd.RunE(repoPruneCmd, nil) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil {
				t.Fatalf("expected lock acquisition error")
			}

			errMsg := err.Error()
			if !strings.Contains(errMsg, "failed to acquire repository lock") {
				t.Fatalf("expected lock acquisition message, got: %v", err)
			}
			if !strings.Contains(errMsg, manager.RepoLockPath()) {
				t.Fatalf("expected lock path %q in error, got: %v", manager.RepoLockPath(), err)
			}
		})
	}
}

func TestRepoReadCommands_FailWhenRepoWriteLockHeld(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to initialize setup repo: %v", err)
	}
	lock, err := manager.AcquireRepoWriteLock(context.Background())
	if err != nil {
		t.Fatalf("failed to acquire setup repo write lock: %v", err)
	}
	t.Cleanup(func() {
		_ = lock.Unlock()
	})

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "repo verify", run: func() error { return repoVerifyCmd.RunE(repoVerifyCmd, nil) }},
		{name: "repo info", run: func() error { return repoInfoCmd.RunE(repoInfoCmd, nil) }},
		{name: "repo list", run: func() error { return listCmd.RunE(listCmd, nil) }},
		{name: "repo describe", run: func() error { return repoDescribeCmd.RunE(repoDescribeCmd, []string{"skill/*"}) }},
		{name: "repo show-manifest", run: func() error { return runShowManifest(repoShowManifestCmd, nil) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil {
				t.Fatalf("expected lock acquisition error")
			}

			errMsg := err.Error()
			if !strings.Contains(errMsg, "failed to acquire repository read lock") {
				t.Fatalf("expected read lock acquisition message, got: %v", err)
			}
			if !strings.Contains(errMsg, manager.RepoLockPath()) {
				t.Fatalf("expected lock path %q in error, got: %v", manager.RepoLockPath(), err)
			}
		})
	}
}

func TestProjectRepoBackedCommands_FailWhenRepoWriteLockHeld(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to initialize setup repo: %v", err)
	}
	lock, err := manager.AcquireRepoWriteLock(context.Background())
	if err != nil {
		t.Fatalf("failed to acquire setup repo write lock: %v", err)
	}
	t.Cleanup(func() {
		_ = lock.Unlock()
	})

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "project verify", run: func() error { return projectVerifyCmd.RunE(projectVerifyCmd, nil) }},
		{name: "project repair", run: func() error { return repairCmd.RunE(repairCmd, nil) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil {
				t.Fatalf("expected lock acquisition error")
			}

			errMsg := err.Error()
			if !strings.Contains(errMsg, "failed to acquire repository read lock") {
				t.Fatalf("expected read lock acquisition message, got: %v", err)
			}
			if !strings.Contains(errMsg, manager.RepoLockPath()) {
				t.Fatalf("expected lock path %q in error, got: %v", manager.RepoLockPath(), err)
			}
		})
	}
}
