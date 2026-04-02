package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/spf13/cobra"
)

func withOverrideSourceState(t *testing.T, autoSync func() error, fn func()) {
	t.Helper()

	originalClear := repoOverrideSourceClearFlag
	originalAutoSync := repoOverrideSourceAutoSync

	repoOverrideSourceClearFlag = false
	repoOverrideSourceAutoSync = autoSync

	defer func() {
		repoOverrideSourceClearFlag = originalClear
		repoOverrideSourceAutoSync = originalAutoSync
	}()

	fn()
}

func setupOverrideSourceTestRepo(t *testing.T, sources ...*repomanifest.Source) string {
	t.Helper()

	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	mgr := repo.NewManagerWithPath(repoPath)
	if err := mgr.Init(); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	manifest := &repomanifest.Manifest{Version: 1, Sources: sources}
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	return repoPath
}

func TestValidateOverrideSourceArgs(t *testing.T) {
	tests := []struct {
		name      string
		clear     bool
		args      []string
		wantError string
	}{
		{name: "override valid", clear: false, args: []string{"my-src", "local:/tmp/src"}},
		{name: "clear valid", clear: true, args: []string{"my-src"}},
		{name: "missing all", clear: false, args: nil, wantError: "missing source name and override target"},
		{name: "missing target", clear: false, args: []string{"my-src"}, wantError: "missing override target"},
		{name: "clear missing source", clear: true, args: nil, wantError: "missing source name"},
		{name: "clear with target ambiguous", clear: true, args: []string{"my-src", "local:/tmp/src"}, wantError: "--clear cannot be combined"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOverrideSourceArgs(tt.clear, tt.args)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestRepoOverrideSource_UnknownSourceRejected(t *testing.T) {
	_ = setupOverrideSourceTestRepo(t, &repomanifest.Source{Name: "known", URL: "https://github.com/example/repo"})

	withOverrideSourceState(t, func() error { return nil }, func() {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		err := runRepoOverrideSource(cmd, []string{"unknown", "local:/tmp/dev"})
		if err == nil {
			t.Fatal("expected error for unknown source")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected not-found error, got %v", err)
		}
	})
}

func TestRepoOverrideSource_RejectsNonLocalTarget(t *testing.T) {
	_ = setupOverrideSourceTestRepo(t, &repomanifest.Source{Name: "team-tools", URL: "https://github.com/example/repo"})

	withOverrideSourceState(t, func() error { return nil }, func() {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		err := runRepoOverrideSource(cmd, []string{"team-tools", "https://github.com/example/other"})
		if err == nil {
			t.Fatal("expected non-local target error")
		}
		if !strings.Contains(err.Error(), "must use local:/path") {
			t.Fatalf("expected local:/path guidance, got %v", err)
		}
	})
}

func TestRepoOverrideSource_MissingRepoFailsBeforeCreatingLockState(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)
	if err := os.RemoveAll(repoPath); err != nil {
		t.Fatalf("failed to remove repo path: %v", err)
	}

	withOverrideSourceState(t, func() error { return nil }, func() {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		err := runRepoOverrideSource(cmd, []string{"team-tools", "local:/tmp/dev"})
		if err == nil {
			t.Fatal("expected error for missing repo")
		}
		if !strings.Contains(err.Error(), "run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first") {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(repoPath, ".workspace")); !os.IsNotExist(statErr) {
			t.Fatalf("expected missing repo path to remain untouched, stat err: %v", statErr)
		}
	})
}

func TestRepoOverrideSource_SuccessOverrideAndClearPreserveInclude(t *testing.T) {
	repoPath := setupOverrideSourceTestRepo(t, &repomanifest.Source{
		Name:    "team-tools",
		URL:     "https://github.com/example/tools",
		Ref:     "main",
		Subpath: "resources",
		Include: []string{"skill/pdf", "command/*"},
	})

	localSourcePath := t.TempDir()

	syncCalls := 0
	withOverrideSourceState(t, func() error {
		syncCalls++
		return nil
	}, func() {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())

		err := runRepoOverrideSource(cmd, []string{"team-tools", "local:" + localSourcePath})
		if err != nil {
			t.Fatalf("override failed: %v", err)
		}

		manifestAfterOverride, err := repomanifest.Load(repoPath)
		if err != nil {
			t.Fatalf("failed to load manifest after override: %v", err)
		}
		src, found := manifestAfterOverride.GetSource("team-tools")
		if !found {
			t.Fatalf("source not found after override")
		}
		if src.Path != localSourcePath {
			t.Fatalf("expected active local path %q, got %q", localSourcePath, src.Path)
		}
		if src.OverrideOriginalURL != "https://github.com/example/tools" || src.OverrideOriginalRef != "main" || src.OverrideOriginalSubpath != "resources" {
			t.Fatalf("expected breadcrumbs to be set, got %+v", src)
		}
		if got := strings.Join(src.Include, ","); got != "skill/pdf,command/*" {
			t.Fatalf("expected include filters to be preserved, got %v", src.Include)
		}

		repoOverrideSourceClearFlag = true
		err = runRepoOverrideSource(cmd, []string{"team-tools"})
		repoOverrideSourceClearFlag = false
		if err != nil {
			t.Fatalf("clear failed: %v", err)
		}

		manifestAfterClear, err := repomanifest.Load(repoPath)
		if err != nil {
			t.Fatalf("failed to load manifest after clear: %v", err)
		}
		src, found = manifestAfterClear.GetSource("team-tools")
		if !found {
			t.Fatalf("source not found after clear")
		}
		if src.URL != "https://github.com/example/tools" || src.Ref != "main" || src.Subpath != "resources" {
			t.Fatalf("expected remote source restored, got %+v", src)
		}
		if src.OverrideOriginalURL != "" || src.OverrideOriginalRef != "" || src.OverrideOriginalSubpath != "" {
			t.Fatalf("expected breadcrumbs cleared, got %+v", src)
		}
		if got := strings.Join(src.Include, ","); got != "skill/pdf,command/*" {
			t.Fatalf("expected include filters preserved after clear, got %v", src.Include)
		}
	})

	if syncCalls != 2 {
		t.Fatalf("expected auto-sync to be called twice (override + clear), got %d", syncCalls)
	}
}

func TestRepoOverrideSource_ClearRejectedWhenNotOverridden(t *testing.T) {
	_ = setupOverrideSourceTestRepo(t, &repomanifest.Source{Name: "team-tools", URL: "https://github.com/example/repo"})

	withOverrideSourceState(t, func() error { return nil }, func() {
		repoOverrideSourceClearFlag = true
		defer func() { repoOverrideSourceClearFlag = false }()

		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		err := runRepoOverrideSource(cmd, []string{"team-tools"})
		if err == nil {
			t.Fatal("expected clear rejection for non-overridden source")
		}
		if !strings.Contains(err.Error(), "not currently overridden") {
			t.Fatalf("expected not-overridden error, got %v", err)
		}
	})
}

func TestRepoOverrideSource_DoubleOverrideRejected(t *testing.T) {
	localCurrent := t.TempDir()
	_ = setupOverrideSourceTestRepo(t, &repomanifest.Source{
		Name:                "team-tools",
		Path:                localCurrent,
		OverrideOriginalURL: "https://github.com/example/repo",
	})

	localNew := t.TempDir()
	withOverrideSourceState(t, func() error { return nil }, func() {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		err := runRepoOverrideSource(cmd, []string{"team-tools", "local:" + localNew})
		if err == nil {
			t.Fatal("expected double-override rejection")
		}
		if !strings.Contains(err.Error(), "already overridden") {
			t.Fatalf("expected already-overridden error, got %v", err)
		}
	})
}

func TestRepoOverrideSource_AlreadyLocalRejected(t *testing.T) {
	localCurrent := t.TempDir()
	_ = setupOverrideSourceTestRepo(t, &repomanifest.Source{Name: "local-only", Path: localCurrent})

	localNew := t.TempDir()
	withOverrideSourceState(t, func() error { return nil }, func() {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		err := runRepoOverrideSource(cmd, []string{"local-only", "local:" + localNew})
		if err == nil {
			t.Fatal("expected already-local rejection")
		}
		if !strings.Contains(err.Error(), "already a local path source") {
			t.Fatalf("expected already-local policy error, got %v", err)
		}
	})
}

func TestRepoOverrideSource_SyncFailureAfterOverrideReportsActiveState(t *testing.T) {
	repoPath := setupOverrideSourceTestRepo(t, &repomanifest.Source{Name: "team-tools", URL: "https://github.com/example/repo", Ref: "main"})
	localPath := t.TempDir()

	withOverrideSourceState(t, func() error { return fmt.Errorf("sync exploded") }, func() {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		err := runRepoOverrideSource(cmd, []string{"team-tools", "local:" + localPath})
		if err == nil {
			t.Fatal("expected sync-failure error")
		}
		for _, expected := range []string{"override persisted", "automatic sync failed", "Active source is now", "Restore metadata is intact", "aimgr repo sync"} {
			if !strings.Contains(err.Error(), expected) {
				t.Fatalf("expected sync-failure message to contain %q, got %v", expected, err)
			}
		}
	})

	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest after failed auto-sync: %v", err)
	}
	src, found := manifest.GetSource("team-tools")
	if !found {
		t.Fatal("source missing after failed auto-sync")
	}
	if src.Path != localPath || src.OverrideOriginalURL != "https://github.com/example/repo" {
		t.Fatalf("expected persisted override state after sync failure, got %+v", src)
	}
}

func TestRepoOverrideSource_SyncFailureAfterClearReportsActiveState(t *testing.T) {
	localPath := t.TempDir()
	repoPath := setupOverrideSourceTestRepo(t, &repomanifest.Source{
		Name:                "team-tools",
		Path:                localPath,
		OverrideOriginalURL: "https://github.com/example/repo",
		OverrideOriginalRef: "main",
	})

	withOverrideSourceState(t, func() error { return fmt.Errorf("sync exploded") }, func() {
		repoOverrideSourceClearFlag = true
		defer func() { repoOverrideSourceClearFlag = false }()

		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		err := runRepoOverrideSource(cmd, []string{"team-tools"})
		if err == nil {
			t.Fatal("expected sync-failure error")
		}
		for _, expected := range []string{"clear persisted", "automatic sync failed", "Active source is now", "Restore metadata has been removed", "aimgr repo sync"} {
			if !strings.Contains(err.Error(), expected) {
				t.Fatalf("expected sync-failure message to contain %q, got %v", expected, err)
			}
		}
	})

	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest after failed clear auto-sync: %v", err)
	}
	src, found := manifest.GetSource("team-tools")
	if !found {
		t.Fatal("source missing after failed clear auto-sync")
	}
	if src.URL != "https://github.com/example/repo" || src.Ref != "main" || src.Path != "" {
		t.Fatalf("expected restored remote to remain active after sync failure, got %+v", src)
	}
	if src.OverrideOriginalURL != "" || src.OverrideOriginalRef != "" || src.OverrideOriginalSubpath != "" {
		t.Fatalf("expected restore breadcrumbs removed after failed clear sync, got %+v", src)
	}
}

func TestRepoOverrideSource_RejectsOverrideTargetFilePath(t *testing.T) {
	_ = setupOverrideSourceTestRepo(t, &repomanifest.Source{Name: "team-tools", URL: "https://github.com/example/repo"})

	filePath := filepath.Join(t.TempDir(), "not-a-dir.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to create file target: %v", err)
	}

	withOverrideSourceState(t, func() error { return nil }, func() {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		err := runRepoOverrideSource(cmd, []string{"team-tools", "local:" + filePath})
		if err == nil {
			t.Fatal("expected directory validation error")
		}
		if !strings.Contains(err.Error(), "must be a directory") {
			t.Fatalf("expected directory error, got %v", err)
		}
	})
}
