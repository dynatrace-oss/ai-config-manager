package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func createExistingRepoDirWithoutManifest(t *testing.T) string {
	t.Helper()

	repoPath := filepath.Join(t.TempDir(), "existing-repo-without-manifest")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create existing repo path fixture: %v", err)
	}

	manifestPath := filepath.Join(repoPath, "ai.repo.yaml")
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("expected ai.repo.yaml to be absent, stat err: %v", err)
	}

	return repoPath
}

func assertNoRepoStateSideEffects(t *testing.T, repoPath string) {
	t.Helper()

	for _, rel := range []string{
		"logs",
		filepath.Join("logs", "operations.log"),
		".workspace",
		".metadata",
		"ai.repo.yaml",
	} {
		if _, err := os.Stat(filepath.Join(repoPath, rel)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to not exist, stat err: %v", rel, err)
		}
	}
}

func TestManifestRequiredCommands_MissingManifestDoNotCreateRepoState(t *testing.T) {
	repoPath := createExistingRepoDirWithoutManifest(t)
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	originalDescribeFormat := describeFormatFlag
	describeFormatFlag = "table"
	defer func() { describeFormatFlag = originalDescribeFormat }()

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "repo sync",
			run: func() error {
				return runSync(syncCmd, []string{})
			},
		},
		{
			name: "repo rebuild",
			run: func() error {
				return runRebuild(repoRebuildCmd, []string{})
			},
		},
		{
			name: "repo show-manifest",
			run: func() error {
				return runShowManifest(repoShowManifestCmd, nil)
			},
		},
		{
			name: "repo describe",
			run: func() error {
				return repoDescribeCmd.RunE(repoDescribeCmd, []string{"skill/example"})
			},
		},
		{
			name: "repo remove",
			run: func() error {
				return runRemove(repoRemoveCmd, []string{"some-source"})
			},
		},
		{
			name: "repo override-source",
			run: func() error {
				cmd := &cobra.Command{}
				cmd.SetContext(context.Background())
				return runRepoOverrideSource(cmd, []string{"some-source", "local:/tmp/dev"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatalf("expected missing-manifest error")
			}
			if !strings.Contains(err.Error(), "run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first") {
				t.Fatalf("unexpected error: %v", err)
			}

			assertNoRepoStateSideEffects(t, repoPath)
		})
	}
}
