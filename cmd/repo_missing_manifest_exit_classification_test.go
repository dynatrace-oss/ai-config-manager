package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestManifestRequiredCommands_MissingManifest_OperationalExitClassification(t *testing.T) {
	repoPath := createExistingRepoDirWithoutManifest(t)
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	originalDescribeFormat := describeFormatFlag
	describeFormatFlag = "table"
	defer func() { describeFormatFlag = originalDescribeFormat }()

	tests := []struct {
		name             string
		command          *cobra.Command
		run              func(*cobra.Command) error
		expectsSilenceOn bool
	}{
		{
			name:             "repo sync",
			command:          syncCmd,
			run:              func(cmd *cobra.Command) error { return runSync(cmd, []string{}) },
			expectsSilenceOn: true,
		},
		{
			name:             "repo rebuild",
			command:          repoRebuildCmd,
			run:              func(cmd *cobra.Command) error { return runRebuild(cmd, []string{}) },
			expectsSilenceOn: true,
		},
		{
			name:             "repo show-manifest",
			command:          repoShowManifestCmd,
			run:              func(cmd *cobra.Command) error { return runShowManifest(cmd, nil) },
			expectsSilenceOn: true,
		},
		{
			name:             "repo describe",
			command:          repoDescribeCmd,
			run:              func(cmd *cobra.Command) error { return cmd.RunE(cmd, []string{"skill/example"}) },
			expectsSilenceOn: true,
		},
		{
			name:             "repo remove",
			command:          repoRemoveCmd,
			run:              func(cmd *cobra.Command) error { return runRemove(cmd, []string{"example-source"}) },
			expectsSilenceOn: true,
		},
		{
			name: "repo override-source",
			command: func() *cobra.Command {
				cmd := &cobra.Command{}
				cmd.SetContext(context.Background())
				return cmd
			}(),
			run: func(cmd *cobra.Command) error {
				return runRepoOverrideSource(cmd, []string{"example-source", "local:/tmp/dev"})
			},
			expectsSilenceOn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSilence := tt.command.SilenceUsage
			defer func() { tt.command.SilenceUsage = originalSilence }()

			err := tt.run(tt.command)
			if err == nil {
				t.Fatal("expected error for missing manifest")
			}

			if got := getCommandExitCode(err); got != commandExitCodeOperationalFailure {
				t.Fatalf("exit code=%d want %d", got, commandExitCodeOperationalFailure)
			}

			var cmdErr *commandExitError
			if !errors.As(err, &cmdErr) {
				t.Fatalf("expected commandExitError, got %T", err)
			}

			if !strings.Contains(err.Error(), "run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first") {
				t.Fatalf("expected actionable init/apply guidance, got: %v", err)
			}

			if tt.expectsSilenceOn && !tt.command.SilenceUsage {
				t.Fatalf("expected command silence usage enabled for missing-manifest operational path")
			}
		})
	}
}

func TestManifestRequiredCommands_MissingManifest_ExistingDirectoryStateIsDistinctFromMissingRepoPath(t *testing.T) {
	existingRepoPath := createExistingRepoDirWithoutManifest(t)
	missingRepoPath := filepath.Join(t.TempDir(), "repo-does-not-exist")

	if _, err := os.Stat(existingRepoPath); err != nil {
		t.Fatalf("expected existing repo directory fixture, stat err: %v", err)
	}
	if _, err := os.Stat(missingRepoPath); !os.IsNotExist(err) {
		t.Fatalf("expected missing repo path fixture, stat err: %v", err)
	}

	t.Run("existing directory without manifest", func(t *testing.T) {
		t.Setenv("AIMGR_REPO_PATH", existingRepoPath)
		err := runShowManifest(repoShowManifestCmd, nil)
		if err == nil {
			t.Fatal("expected error for existing repo directory without manifest")
		}
		if got := getCommandExitCode(err); got != commandExitCodeOperationalFailure {
			t.Fatalf("exit code=%d want %d", got, commandExitCodeOperationalFailure)
		}
		if !strings.Contains(err.Error(), "run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first") {
			t.Fatalf("expected actionable init/apply guidance, got: %v", err)
		}
	})

	t.Run("completely missing repo path", func(t *testing.T) {
		t.Setenv("AIMGR_REPO_PATH", missingRepoPath)
		err := runShowManifest(repoShowManifestCmd, nil)
		if err == nil {
			t.Fatal("expected error for completely missing repo path")
		}
		if got := getCommandExitCode(err); got != commandExitCodeOperationalFailure {
			t.Fatalf("exit code=%d want %d", got, commandExitCodeOperationalFailure)
		}
		if !strings.Contains(err.Error(), "run 'aimgr repo init' or 'aimgr repo apply-manifest <path-or-url>' first") {
			t.Fatalf("expected actionable init/apply guidance, got: %v", err)
		}
	})
}

func TestManifestRequiredCommands_ArgumentValidationRemainsNonOperational(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "repo describe missing args",
			err:  repoDescribeCmd.Args(repoDescribeCmd, []string{}),
		},
		{
			name: "repo remove missing args",
			err:  repoRemoveCmd.Args(repoRemoveCmd, []string{}),
		},
		{
			name: "repo override-source missing target",
			err:  validateOverrideSourceArgs(false, []string{"team-tools"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("expected validation error")
			}

			if got := getCommandExitCode(tt.err); got != 1 {
				t.Fatalf("exit code=%d want 1", got)
			}

			var cmdErr *commandExitError
			if errors.As(tt.err, &cmdErr) {
				t.Fatalf("argument validation should not use commandExitError, got %+v", cmdErr)
			}
		})
	}
}
