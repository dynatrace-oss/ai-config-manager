package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
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

	if got := out.String(); got != content {
		t.Fatalf("unexpected manifest output:\n%s", got)
	}
}

func TestRepoShowManifestErrorsWhenManifestMissing(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	err := runShowManifest(repoShowManifestCmd, nil)
	if err == nil {
		t.Fatal("expected error when manifest is missing")
	}
	if !strings.Contains(err.Error(), "run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first") {
		t.Fatalf("unexpected error: %v", err)
	}
}
