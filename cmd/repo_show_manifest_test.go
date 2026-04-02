package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/sourcemetadata"
)

func TestRepoShowManifestHelpText(t *testing.T) {
	if repoShowManifestCmd.Use != "show-manifest" {
		t.Fatalf("unexpected Use: %s", repoShowManifestCmd.Use)
	}

	help := repoShowManifestCmd.Long
	for _, expected := range []string{
		"ai.repo.yaml",
		"repo apply-manifest <path-or-url>",
		"aimgr repo show-manifest",
		"Override behavior",
		"repo info",
	} {
		if !strings.Contains(help, expected) {
			t.Fatalf("expected help text to contain %q", expected)
		}
	}
}

func TestRepoShowManifestPrintsCurrentManifest(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	manifestPath := filepath.Join(repoDir, repomanifest.ManifestFileName)
	content := "version: 1\nsources:\n  - name: team-tools\n    url: https://github.com/example/tools\n"
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	var out bytes.Buffer
	repoShowManifestCmd.SetOut(&out)
	defer repoShowManifestCmd.SetOut(os.Stdout)

	if err := runShowManifest(repoShowManifestCmd, nil); err != nil {
		t.Fatalf("runShowManifest() error = %v", err)
	}

	got := out.String()
	for _, expected := range []string{"version: 1", "name: team-tools", "url: https://github.com/example/tools"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected show-manifest output to contain %q, got:\n%s", expected, got)
		}
	}
}

func TestRepoShowManifestErrorsWhenManifestMissing(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)
	if err := os.RemoveAll(repoDir); err != nil {
		t.Fatalf("failed to remove repo dir: %v", err)
	}

	err := runShowManifest(repoShowManifestCmd, nil)
	if err == nil {
		t.Fatal("expected error when manifest is missing")
	}
	if !strings.Contains(err.Error(), "run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(repoDir, ".workspace")); !os.IsNotExist(statErr) {
		t.Fatalf("expected missing repo path to remain untouched, stat err: %v", statErr)
	}
}

func TestRepoShowManifest_OverriddenSourcePrintsRestoreRemoteView(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	manifest := &repomanifest.Manifest{Version: 1, Sources: []*repomanifest.Source{{
		Name:    "team-tools",
		Path:    "/tmp/local/team-tools",
		Include: []string{"skill/*"},
	}}}
	if err := manifest.Save(repoDir); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	meta, err := sourcemetadata.Load(repoDir)
	if err != nil {
		t.Fatalf("failed to load source metadata: %v", err)
	}
	meta.Sources["team-tools"] = &sourcemetadata.SourceState{
		OverrideOriginalURL:     "https://github.com/example/tools",
		OverrideOriginalRef:     "main",
		OverrideOriginalSubpath: "resources",
	}
	if err := meta.Save(repoDir); err != nil {
		t.Fatalf("failed to save source metadata: %v", err)
	}

	var out bytes.Buffer
	repoShowManifestCmd.SetOut(&out)
	defer repoShowManifestCmd.SetOut(os.Stdout)

	if err := runShowManifest(repoShowManifestCmd, nil); err != nil {
		t.Fatalf("runShowManifest() error = %v", err)
	}

	got := out.String()
	for _, expected := range []string{
		"name: team-tools",
		"url: https://github.com/example/tools",
		"ref: main",
		"subpath: resources",
		"include:",
		"- skill/*",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected show-manifest output to contain %q, got:\n%s", expected, got)
		}
	}

	for _, forbidden := range []string{
		"path: /tmp/local/team-tools",
		"override_original_url",
		"override_original_ref",
		"override_original_subpath",
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("expected show-manifest output to hide %q, got:\n%s", forbidden, got)
		}
	}
}
