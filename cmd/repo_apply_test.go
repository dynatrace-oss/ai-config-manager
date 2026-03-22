package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
)

func TestRepoApplyCommandHelpText(t *testing.T) {
	if repoApplyManifestCmd.Use != "apply-manifest <path-or-url>" {
		t.Fatalf("unexpected Use: %s", repoApplyManifestCmd.Use)
	}

	help := repoApplyManifestCmd.Long + "\n" + repoApplyManifestCmd.Example
	for _, expected := range []string{
		"repo init",
		"repo show-manifest",
		"repo apply-manifest <path-or-url>",
		"Apply is additive",
		"repo drop-source",
		"aimgr repo apply-manifest ./ai.repo.yaml",
		"aimgr repo apply-manifest -",
		"https://example.com/platform/ai.repo.yaml",
		"raw.githubusercontent.com/example/platform-configs/v1.2.0/manifests/ai.repo.yaml",
		"/blob/<ref>/",
		"aimgr repo sync",
		"aimgr install",
		"--include-mode preserve",
	} {
		if !strings.Contains(help, expected) {
			t.Fatalf("expected help text to contain %q", expected)
		}
	}
}

func TestRepoApply_LocalManifest_AutoInitializesFreshRepo(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	manifestPath := filepath.Join(t.TempDir(), repomanifest.ManifestFileName)
	content := `version: 1
sources:
  - name: team-tools
    url: https://github.com/example/tools
`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write input manifest: %v", err)
	}

	withApplyFlags(false, string(repomanifest.IncludeMergeReplace), func() {
		if err := runApplyManifest(repoApplyManifestCmd, []string{manifestPath}); err != nil {
			t.Fatalf("runApplyManifest() error = %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		t.Fatalf("expected fresh repo to be initialized with .git")
	}

	merged, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load merged manifest: %v", err)
	}
	if len(merged.Sources) != 1 || merged.Sources[0].Name != "team-tools" {
		t.Fatalf("unexpected merged sources: %+v", merged.Sources)
	}
}

func TestRepoApply_RemoteManifest(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/team/ai.repo.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`version: 1
sources:
  - name: remote-tools
    url: https://github.com/example/remote-tools
`))
	}))
	defer server.Close()

	withApplyFlags(false, string(repomanifest.IncludeMergeReplace), func() {
		if err := runApplyManifest(repoApplyManifestCmd, []string{server.URL + "/team/ai.repo.yaml"}); err != nil {
			t.Fatalf("runApplyManifest() error = %v", err)
		}
	})

	merged, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load merged manifest: %v", err)
	}
	if len(merged.Sources) != 1 || merged.Sources[0].Name != "remote-tools" {
		t.Fatalf("unexpected merged sources: %+v", merged.Sources)
	}
}

func TestRepoApply_RepeatedApplyIsIdempotent(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	manifestPath := filepath.Join(t.TempDir(), repomanifest.ManifestFileName)
	content := `version: 1
sources:
  - name: team-tools
    url: https://github.com/example/tools
    include:
      - skill/*
`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write input manifest: %v", err)
	}

	withApplyFlags(false, string(repomanifest.IncludeMergeReplace), func() {
		if err := runApplyManifest(repoApplyManifestCmd, []string{manifestPath}); err != nil {
			t.Fatalf("first apply error = %v", err)
		}
		if err := runApplyManifest(repoApplyManifestCmd, []string{manifestPath}); err != nil {
			t.Fatalf("second apply error = %v", err)
		}
	})

	merged, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load merged manifest: %v", err)
	}
	if len(merged.Sources) != 1 {
		t.Fatalf("expected one source after repeated apply, got %d", len(merged.Sources))
	}
}

func TestRepoApply_IncludeModePreserveAndReplace(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	baseline := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{{
		Name:    "team-tools",
		URL:     "https://github.com/example/tools",
		Include: []string{"skill/pdf*"},
	}}}
	if err := baseline.Save(repoDir); err != nil {
		t.Fatalf("failed to save baseline manifest: %v", err)
	}

	incomingPath := filepath.Join(t.TempDir(), repomanifest.ManifestFileName)
	incoming := `version: 1
sources:
  - name: team-tools
    url: https://github.com/example/tools
    include:
      - command/lint-*
`
	if err := os.WriteFile(incomingPath, []byte(incoming), 0644); err != nil {
		t.Fatalf("failed to write incoming manifest: %v", err)
	}

	withApplyFlags(false, string(repomanifest.IncludeMergePreserve), func() {
		if err := runApplyManifest(repoApplyManifestCmd, []string{incomingPath}); err != nil {
			t.Fatalf("preserve apply error = %v", err)
		}
	})
	afterPreserve, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("load after preserve: %v", err)
	}
	if got := strings.Join(afterPreserve.Sources[0].Include, ","); got != "skill/pdf*" {
		t.Fatalf("preserve mode changed include unexpectedly: %s", got)
	}

	withApplyFlags(false, string(repomanifest.IncludeMergeReplace), func() {
		if err := runApplyManifest(repoApplyManifestCmd, []string{incomingPath}); err != nil {
			t.Fatalf("replace apply error = %v", err)
		}
	})
	afterReplace, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("load after replace: %v", err)
	}
	if got := strings.Join(afterReplace.Sources[0].Include, ","); got != "command/lint-*" {
		t.Fatalf("replace mode include mismatch: %s", got)
	}
}

func TestRepoApply_ReapplyUpdatesSourceRef(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	baseline := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{{
		Name: "team-tools",
		URL:  "https://github.com/example/tools",
		Ref:  "v1.2.0",
	}}}
	if err := baseline.Save(repoDir); err != nil {
		t.Fatalf("failed to save baseline manifest: %v", err)
	}

	incomingPath := filepath.Join(t.TempDir(), repomanifest.ManifestFileName)
	incoming := `version: 1
sources:
  - name: team-tools
    url: https://github.com/example/tools
    ref: v1.3.0
`
	if err := os.WriteFile(incomingPath, []byte(incoming), 0644); err != nil {
		t.Fatalf("failed to write incoming manifest: %v", err)
	}

	withApplyFlags(false, string(repomanifest.IncludeMergeReplace), func() {
		if err := runApplyManifest(repoApplyManifestCmd, []string{incomingPath}); err != nil {
			t.Fatalf("apply-manifest failed: %v", err)
		}
	})

	after, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load manifest after apply: %v", err)
	}
	if got := after.Sources[0].Ref; got != "v1.3.0" {
		t.Fatalf("expected ref to be updated to v1.3.0, got %q", got)
	}
}

func TestRepoApply_ConflictDoesNotOverwrite(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	baseline := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{{
		Name: "team-tools",
		URL:  "https://github.com/example/tools",
	}}}
	if err := baseline.Save(repoDir); err != nil {
		t.Fatalf("failed to save baseline manifest: %v", err)
	}

	incomingPath := filepath.Join(t.TempDir(), repomanifest.ManifestFileName)
	incoming := `version: 1
sources:
  - name: team-tools
    url: https://github.com/example/other-tools
`
	if err := os.WriteFile(incomingPath, []byte(incoming), 0644); err != nil {
		t.Fatalf("failed to write incoming manifest: %v", err)
	}

	withApplyFlags(false, string(repomanifest.IncludeMergeReplace), func() {
		err := runApplyManifest(repoApplyManifestCmd, []string{incomingPath})
		if err == nil {
			t.Fatal("expected conflict error, got nil")
		}
		if !strings.Contains(err.Error(), "conflict") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	after, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load manifest after conflict: %v", err)
	}
	if after.Sources[0].URL != "https://github.com/example/tools" {
		t.Fatalf("conflict should not overwrite existing source")
	}
}

func TestRepoApply_DryRunReportsAddsUpdatesNoOpsConflicts(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	baseline := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{
		{Name: "source-update", URL: "https://github.com/example/update", Include: []string{"skill/old"}},
		{Name: "source-noop", URL: "https://github.com/example/noop", Include: []string{"skill/noop"}},
		{Name: "source-conflict", URL: "https://github.com/example/original"},
	}}
	if err := baseline.Save(repoDir); err != nil {
		t.Fatalf("failed to save baseline manifest: %v", err)
	}

	incomingPath := filepath.Join(t.TempDir(), repomanifest.ManifestFileName)
	incoming := `version: 1
sources:
  - name: source-add
    url: https://github.com/example/add
  - name: source-update
    url: https://github.com/example/update
    include:
      - skill/new
  - name: source-noop
    url: https://github.com/example/noop
    include:
      - skill/noop
  - name: source-conflict
    url: https://github.com/example/conflicted
`
	if err := os.WriteFile(incomingPath, []byte(incoming), 0644); err != nil {
		t.Fatalf("failed to write incoming manifest: %v", err)
	}

	output := captureStdout(t, func() {
		withApplyFlags(true, string(repomanifest.IncludeMergeReplace), func() {
			err := runApplyManifest(repoApplyManifestCmd, []string{incomingPath})
			if err == nil {
				t.Fatal("expected conflict error in dry-run, got nil")
			}
		})
	})

	for _, expected := range []string{"add", "update", "noop", "conflict", "added=1 updated=1 noop=1 conflicts=1"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected dry-run output to contain %q, got:\n%s", expected, output)
		}
	}

	after, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load manifest after dry-run: %v", err)
	}
	if len(after.Sources) != 3 {
		t.Fatalf("dry-run should not persist changes, expected 3 sources got %d", len(after.Sources))
	}
}

func TestRepoApply_RejectsCanonicalSourceCollisionAndPreservesManifest(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	baseline := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{{
		Name: "team-tools",
		URL:  "https://github.com/example/tools.git/",
	}}}
	if err := baseline.Save(repoDir); err != nil {
		t.Fatalf("failed to save baseline manifest: %v", err)
	}

	incomingPath := filepath.Join(t.TempDir(), repomanifest.ManifestFileName)
	incoming := `version: 1
sources:
  - name: platform-tools
    url: https://github.com/EXAMPLE/tools
`
	if err := os.WriteFile(incomingPath, []byte(incoming), 0644); err != nil {
		t.Fatalf("failed to write incoming manifest: %v", err)
	}

	withApplyFlags(false, string(repomanifest.IncludeMergeReplace), func() {
		err := runApplyManifest(repoApplyManifestCmd, []string{incomingPath})
		if err == nil {
			t.Fatal("expected canonical source collision error, got nil")
		}
		for _, expected := range []string{"platform-tools", "team-tools", "same canonical location", "resolve conflicts"} {
			if !strings.Contains(err.Error(), expected) {
				t.Fatalf("expected error to contain %q, got: %v", expected, err)
			}
		}
	})

	after, err := repomanifest.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load manifest after failed apply: %v", err)
	}
	if len(after.Sources) != 1 || after.Sources[0].Name != "team-tools" {
		t.Fatalf("failed apply must keep manifest valid and unchanged, got %+v", after.Sources)
	}
}

func TestRepoApply_StdinRoundTripNoChanges(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	baseline := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{{
		Name: "team-tools",
		URL:  "https://github.com/example/tools",
	}}}
	if err := baseline.Save(repoDir); err != nil {
		t.Fatalf("failed to save baseline manifest: %v", err)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(repoDir, repomanifest.ManifestFileName))
	if err != nil {
		t.Fatalf("failed to read baseline manifest: %v", err)
	}

	output := captureStdoutWithStdin(t, string(manifestBytes), func() {
		withApplyFlags(false, string(repomanifest.IncludeMergeReplace), func() {
			if err := runApplyManifest(repoApplyManifestCmd, []string{"-"}); err != nil {
				t.Fatalf("stdin apply-manifest error = %v", err)
			}
		})
	})

	for _, expected := range []string{"added=0", "updated=0", "noop=1", "No changes to apply"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRepoApply_CommitIsScopedToManifestFile(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	mgr := repo.NewManagerWithPath(repoDir)
	if err := mgr.Init(); err != nil {
		t.Fatalf("failed to initialize repository: %v", err)
	}

	// Ensure manifest exists so apply writes an update.
	baseline := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{{
		Name: "team-tools",
		URL:  "https://github.com/example/tools",
	}}}
	if err := baseline.Save(repoDir); err != nil {
		t.Fatalf("failed to save baseline manifest: %v", err)
	}

	gitRun := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
		}
		return string(out)
	}

	gitCommit := func(message string) {
		t.Helper()
		cmd := exec.Command("git", "-C", repoDir, "-c", "user.name=Test User", "-c", "user.email=test@example.com", "commit", "-m", message)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git commit failed: %v\n%s", err, string(out))
		}
	}

	// Commit baseline manifest and create unrelated modified file.
	gitRun("add", "ai.repo.yaml")
	gitCommit("test: baseline manifest")

	unrelated := filepath.Join(repoDir, "apply-unrelated.txt")
	if err := os.WriteFile(unrelated, []byte("baseline\n"), 0644); err != nil {
		t.Fatalf("failed to write unrelated file: %v", err)
	}
	gitRun("add", "apply-unrelated.txt")
	gitCommit("test: add unrelated")
	if err := os.WriteFile(unrelated, []byte("baseline\nlocal change\n"), 0644); err != nil {
		t.Fatalf("failed to modify unrelated file: %v", err)
	}

	incomingPath := filepath.Join(t.TempDir(), repomanifest.ManifestFileName)
	incoming := `version: 1
sources:
  - name: team-tools
    url: https://github.com/example/tools
    include:
      - skill/new
`
	if err := os.WriteFile(incomingPath, []byte(incoming), 0644); err != nil {
		t.Fatalf("failed to write incoming manifest: %v", err)
	}

	withApplyFlags(false, string(repomanifest.IncludeMergeReplace), func() {
		if err := runApplyManifest(repoApplyManifestCmd, []string{incomingPath}); err != nil {
			t.Fatalf("apply-manifest failed: %v", err)
		}
	})

	show := gitRun("show", "--name-only", "--pretty=format:", "HEAD")
	if !strings.Contains(show, "ai.repo.yaml") {
		t.Fatalf("expected apply commit to include ai.repo.yaml, got:\n%s", show)
	}
	if strings.Contains(show, "apply-unrelated.txt") {
		t.Fatalf("apply commit must not include unrelated file, got:\n%s", show)
	}

	status := gitRun("status", "--porcelain")
	if !strings.Contains(status, " M apply-unrelated.txt") {
		t.Fatalf("expected unrelated file to remain modified after apply-manifest, status:\n%s", status)
	}
}

func withApplyFlags(dryRun bool, includeMode string, fn func()) {
	oldDryRun := repoApplyDryRunFlag
	oldIncludeMode := repoApplyIncludeModeFlag
	repoApplyDryRunFlag = dryRun
	repoApplyIncludeModeFlag = includeMode
	defer func() {
		repoApplyDryRunFlag = oldDryRun
		repoApplyIncludeModeFlag = oldIncludeMode
	}()

	fn()
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func captureStdoutWithStdin(t *testing.T, stdinContent string, fn func()) string {
	t.Helper()

	oldStdin := os.Stdin
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdin pipe: %v", err)
	}
	if _, err := io.WriteString(stdinW, stdinContent); err != nil {
		_ = stdinW.Close()
		_ = stdinR.Close()
		t.Fatalf("failed to write stdin content: %v", err)
	}
	_ = stdinW.Close()
	os.Stdin = stdinR
	defer func() {
		os.Stdin = oldStdin
		_ = stdinR.Close()
	}()

	return captureStdout(t, fn)
}
