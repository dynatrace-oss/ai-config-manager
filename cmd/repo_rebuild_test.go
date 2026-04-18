package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/spf13/cobra"
)

func TestRunRebuild_SuccessfulReimportPreservesManifest(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to initialize repo: %v", err)
	}

	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create source commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "rebuild-cmd.md"), []byte("---\ndescription: rebuilt\n---\n# rebuild-cmd\n"), 0644); err != nil {
		t.Fatalf("failed to write source command: %v", err)
	}

	manifest := &repomanifest.Manifest{
		Version: 1,
		Sources: []*repomanifest.Source{{
			Name: "local-source",
			Path: sourceDir,
		}},
	}
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	staleCommand := filepath.Join(repoPath, "commands", "stale.md")
	if err := os.WriteFile(staleCommand, []byte("---\ndescription: stale\n---\n# stale\n"), 0644); err != nil {
		t.Fatalf("failed to write stale command: %v", err)
	}

	originalDryRun := repoRebuildDryRunFlag
	repoRebuildDryRunFlag = false
	t.Cleanup(func() { repoRebuildDryRunFlag = originalDryRun })

	if err := runRebuild(repoRebuildCmd, []string{}); err != nil {
		t.Fatalf("rebuild failed: %v", err)
	}

	if _, err := os.Stat(staleCommand); !os.IsNotExist(err) {
		t.Fatalf("expected stale command to be removed during rebuild, stat err: %v", err)
	}

	imported := filepath.Join(repoPath, "commands", "rebuild-cmd.md")
	if _, err := os.Stat(imported); err != nil {
		t.Fatalf("expected rebuilt command to exist: %v", err)
	}

	after, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest after rebuild: %v", err)
	}
	if len(after.Sources) != 1 {
		t.Fatalf("expected one preserved source, got %d", len(after.Sources))
	}
	if after.Sources[0].Name != "local-source" || after.Sources[0].Path != sourceDir {
		t.Fatalf("expected preserved source local-source:%s, got %+v", sourceDir, after.Sources[0])
	}
}

func TestRunRebuild_ReportsPerSourceProgressDuringReimport(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to initialize repo: %v", err)
	}

	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create source commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "rebuild-progress.md"), []byte("---\ndescription: rebuilt\n---\n# rebuild-progress\n"), 0644); err != nil {
		t.Fatalf("failed to write source command: %v", err)
	}

	manifest := &repomanifest.Manifest{
		Version: 1,
		Sources: []*repomanifest.Source{{
			Name: "local-source",
			Path: sourceDir,
		}},
	}
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	originalDryRun := repoRebuildDryRunFlag
	repoRebuildDryRunFlag = false
	t.Cleanup(func() { repoRebuildDryRunFlag = originalDryRun })

	stdout := captureRebuildStdout(t, func() {
		if err := runRebuild(repoRebuildCmd, []string{}); err != nil {
			t.Fatalf("rebuild failed: %v", err)
		}
	})

	reimportIdx := strings.Index(stdout, "Re-importing configured sources...")
	if reimportIdx == -1 {
		t.Fatalf("expected rebuild output to include re-import banner, got: %q", stdout)
	}

	progressLine := "  Syncing local-source (local)..."
	progressIdx := strings.Index(stdout, progressLine)
	if progressIdx == -1 {
		t.Fatalf("expected rebuild output to include per-source progress line %q, got: %q", progressLine, stdout)
	}

	completeIdx := strings.Index(stdout, "Sync complete:")
	if completeIdx == -1 {
		t.Fatalf("expected rebuild output to include sync completion summary, got: %q", stdout)
	}

	if reimportIdx >= progressIdx || progressIdx >= completeIdx {
		t.Fatalf("expected per-source progress between re-import banner and sync summary; indices reimport=%d progress=%d complete=%d; output=%q", reimportIdx, progressIdx, completeIdx, stdout)
	}
}

func TestRunRebuild_DryRunSkipsDropAndImport(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to initialize repo: %v", err)
	}

	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create source commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "existing.md"), []byte("---\ndescription: existing\n---\n# existing\n"), 0644); err != nil {
		t.Fatalf("failed to write existing source command: %v", err)
	}

	manifest := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{{Name: "local-source", Path: sourceDir}}}
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	if err := runSync(syncCmd, []string{}); err != nil {
		t.Fatalf("initial sync failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "new-in-source.md"), []byte("---\ndescription: new\n---\n# new-in-source\n"), 0644); err != nil {
		t.Fatalf("failed to write new source command: %v", err)
	}

	originalDryRun := repoRebuildDryRunFlag
	repoRebuildDryRunFlag = true
	t.Cleanup(func() { repoRebuildDryRunFlag = originalDryRun })

	if err := runRebuild(repoRebuildCmd, []string{}); err != nil {
		t.Fatalf("rebuild dry-run failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repoPath, "commands", "existing.md")); err != nil {
		t.Fatalf("expected existing imported command to remain after dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoPath, "commands", "new-in-source.md")); !os.IsNotExist(err) {
		t.Fatalf("expected dry-run to not import new command, stat err: %v", err)
	}
}

func TestRunRebuild_EmptySourcesFailsWithoutMutation(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to initialize repo: %v", err)
	}

	manifest := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{}}
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	untouched := filepath.Join(repoPath, "commands", "keep.md")
	if err := os.WriteFile(untouched, []byte("---\ndescription: keep\n---\n# keep\n"), 0644); err != nil {
		t.Fatalf("failed to write keep marker: %v", err)
	}

	err := runRebuild(repoRebuildCmd, []string{})
	if err == nil {
		t.Fatal("expected rebuild to fail for empty source list")
	}
	if !strings.Contains(err.Error(), "no rebuild sources configured") {
		t.Fatalf("unexpected error for empty source list: %v", err)
	}

	if _, statErr := os.Stat(untouched); statErr != nil {
		t.Fatalf("expected existing repo content to remain untouched on empty-source failure: %v", statErr)
	}
}

func TestRunRebuild_PartialSourceFailuresReported(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to initialize repo: %v", err)
	}

	goodSource := t.TempDir()
	if err := os.MkdirAll(filepath.Join(goodSource, "commands"), 0755); err != nil {
		t.Fatalf("failed to create good source commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goodSource, "commands", "good.md"), []byte("---\ndescription: good\n---\n# good\n"), 0644); err != nil {
		t.Fatalf("failed to write good source command: %v", err)
	}

	manifest := &repomanifest.Manifest{
		Version: 1,
		Sources: []*repomanifest.Source{
			{Name: "good-source", Path: goodSource},
			{Name: "bad-source", Path: filepath.Join(t.TempDir(), "missing")},
		},
	}
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	err := runRebuild(repoRebuildCmd, []string{})
	if err == nil {
		t.Fatal("expected rebuild to report partial source failures")
	}
	if got := getCommandExitCode(err); got != commandExitCodeCompletedWithFindings {
		t.Fatalf("exit code=%d want %d (completed with findings)", got, commandExitCodeCompletedWithFindings)
	}
	if !strings.Contains(err.Error(), "repository sync completed with source failures") {
		t.Fatalf("expected partial failure message, got: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(repoPath, "commands", "good.md")); statErr != nil {
		t.Fatalf("expected successful sources to be imported despite partial failure: %v", statErr)
	}
}

func TestRunSyncWithManager_LockAlreadyHeldAvoidsNestedAcquire(t *testing.T) {
	repoPath := t.TempDir()
	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to initialize repo: %v", err)
	}

	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create source commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "locked-sync.md"), []byte("---\ndescription: lock test\n---\n# locked-sync\n"), 0644); err != nil {
		t.Fatalf("failed to write lock-test command: %v", err)
	}

	manifest := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{{Name: "local-source", Path: sourceDir}}}
	if err := manifest.Save(repoPath); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	lock, err := manager.AcquireRepoWriteLock(context.Background())
	if err != nil {
		t.Fatalf("failed to acquire setup lock: %v", err)
	}
	defer func() {
		_ = lock.Unlock()
	}()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err = runSyncWithManager(cmd, manager, false, false)
	if err == nil {
		t.Fatal("expected nested write-lock acquisition to fail when lockAlreadyHeld=false")
	}
	if !strings.Contains(err.Error(), "failed to acquire repository lock") {
		t.Fatalf("expected lock acquisition failure, got: %v", err)
	}

	if err := runSyncWithManager(cmd, manager, true, false); err != nil {
		t.Fatalf("expected sync with pre-held lock to succeed, got: %v", err)
	}
}

func captureRebuildStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}

	os.Stdout = w
	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = oldStdout

	return <-outCh
}
