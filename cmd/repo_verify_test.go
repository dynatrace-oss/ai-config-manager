package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repolock"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestRepoVerifyCommand(t *testing.T) {
	tests := []struct {
		name         string
		setupRepo    func(*repo.Manager) error
		wantErrors   bool
		wantWarnings bool
	}{
		{
			name: "healthy repository with no issues",
			setupRepo: func(mgr *repo.Manager) error {
				// Create a command with proper metadata (.md extension required by List())
				commandContent := "---\nname: test-cmd\ndescription: A test command\n---\nCommand content here\n"
				commandPath := filepath.Join(mgr.GetRepoPath(), "commands", "test-cmd.md")
				if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
					return err
				}

				// Create metadata for the command. Use a non-file:// source URL so that
				// verifyRepository does not trigger a "missing source path" warning.
				meta := &metadata.ResourceMetadata{
					Name:       "test-cmd",
					Type:       resource.Command,
					SourceType: "github",
					SourceURL:  "https://github.com/test/repo",
				}
				return metadata.Save(meta, mgr.GetRepoPath(), "local")
			},
			wantErrors:   false,
			wantWarnings: false,
		},
		{
			name: "resource without metadata",
			setupRepo: func(mgr *repo.Manager) error {
				// Create a command without metadata (.md extension required by List())
				commandContent := "---\nname: no-meta-cmd\ndescription: A command without metadata\n---\nCommand content here\n"
				commandPath := filepath.Join(mgr.GetRepoPath(), "commands", "no-meta-cmd.md")
				return os.WriteFile(commandPath, []byte(commandContent), 0644)
			},
			wantErrors:   false,
			wantWarnings: true,
		},
		{
			name: "orphaned metadata without resource",
			setupRepo: func(mgr *repo.Manager) error {
				// Create metadata without corresponding resource
				meta := &metadata.ResourceMetadata{
					Name:       "orphaned-cmd",
					Type:       resource.Command,
					SourceType: "local",
					SourceURL:  "file:///test/source",
				}
				return metadata.Save(meta, mgr.GetRepoPath(), "local")
			},
			wantErrors:   true,
			wantWarnings: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for test repository
			repoDir := t.TempDir()

			// Set AIMGR_REPO_PATH to use test directory
			oldEnv := os.Getenv("AIMGR_REPO_PATH")
			defer func() {
				if oldEnv != "" {
					_ = os.Setenv("AIMGR_REPO_PATH", oldEnv)
				} else {
					_ = os.Unsetenv("AIMGR_REPO_PATH")
				}
			}()
			_ = os.Setenv("AIMGR_REPO_PATH", repoDir)

			// Initialize repository
			manager := repo.NewManagerWithPath(repoDir)
			if err := manager.Init(); err != nil {
				t.Fatalf("Failed to initialize repo: %v", err)
			}

			// Setup test scenario
			if tt.setupRepo != nil {
				if err := tt.setupRepo(manager); err != nil {
					t.Fatalf("Failed to setup repo: %v", err)
				}
			}

			// Run verification
			result, err := verifyRepository(manager, false, nil)
			if err != nil {
				t.Fatalf("Verification failed: %v", err)
			}

			// Check expectations
			if result.HasErrors != tt.wantErrors {
				t.Errorf("HasErrors = %v, want %v", result.HasErrors, tt.wantErrors)
			}
			if result.HasWarnings != tt.wantWarnings {
				t.Errorf("HasWarnings = %v, want %v", result.HasWarnings, tt.wantWarnings)
			}
		})
	}
}

func TestRepoVerifyWithFix(t *testing.T) {
	// Create temp directory for test repository
	repoDir := t.TempDir()

	// Set AIMGR_REPO_PATH to use test directory
	oldEnv := os.Getenv("AIMGR_REPO_PATH")
	defer func() {
		if oldEnv != "" {
			_ = os.Setenv("AIMGR_REPO_PATH", oldEnv)
		} else {
			_ = os.Unsetenv("AIMGR_REPO_PATH")
		}
	}()
	_ = os.Setenv("AIMGR_REPO_PATH", repoDir)

	// Initialize repository
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Create a resource without metadata (.md extension required by List())
	commandContent := "---\nname: test-cmd\ndescription: A test command\n---\nCommand content here\n"
	commandPath := filepath.Join(repoDir, "commands", "test-cmd.md")
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Run verification without fix - should have warnings
	result, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}

	if !result.HasWarnings {
		t.Error("Expected warnings for missing metadata")
	}

	// Run verification with fix
	_, err = verifyRepository(manager, true, nil)
	if err != nil {
		t.Fatalf("Verification with fix failed: %v", err)
	}

	// Verify metadata was created
	meta, err := manager.GetMetadata("test-cmd", resource.Command)
	if err != nil {
		t.Errorf("Metadata was not created by fix: %v", err)
	}

	if meta.Name != "test-cmd" {
		t.Errorf("Metadata name = %v, want test-cmd", meta.Name)
	}
}

func TestFormatResourceReference(t *testing.T) {
	tests := []struct {
		name         string
		resourceType resource.ResourceType
		resourceName string
		want         string
	}{
		{
			name:         "command reference",
			resourceType: resource.Command,
			resourceName: "test-cmd",
			want:         "command/test-cmd",
		},
		{
			name:         "skill reference",
			resourceType: resource.Skill,
			resourceName: "my-skill",
			want:         "skill/my-skill",
		},
		{
			name:         "agent reference",
			resourceType: resource.Agent,
			resourceName: "my-agent",
			want:         "agent/my-agent",
		},
		{
			name:         "package reference",
			resourceType: resource.PackageType,
			resourceName: "my-package",
			want:         "package/my-package",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatResourceReference(tt.resourceType, tt.resourceName)
			if got != tt.want {
				t.Errorf("formatResourceReference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOutputVerifyResults(t *testing.T) {
	tests := []struct {
		name   string
		result *VerifyResult
		format output.Format
	}{
		{
			name: "empty result in table format",
			result: &VerifyResult{
				HasErrors:   false,
				HasWarnings: false,
			},
			format: output.Table,
		},
		{
			name: "result with issues in table format",
			result: &VerifyResult{
				ResourcesWithoutMetadata: []ResourceIssue{
					{Name: "test-cmd", Type: resource.Command, Path: "/test/path"},
				},
				HasErrors:   false,
				HasWarnings: true,
			},
			format: output.Table,
		},
		{
			name: "result in JSON format",
			result: &VerifyResult{
				HasErrors:   false,
				HasWarnings: false,
			},
			format: output.JSON,
		},
		{
			name: "result in YAML format",
			result: &VerifyResult{
				HasErrors:   false,
				HasWarnings: false,
			},
			format: output.YAML,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic or error
			err := outputVerifyResults(tt.result, tt.format, false)
			if err != nil {
				t.Errorf("outputVerifyResults() error = %v", err)
			}
		})
	}
}

func TestVerifyRepository_TypeMismatch(t *testing.T) {
	// Create temp directory for test repository
	repoDir := t.TempDir()

	// Set AIMGR_REPO_PATH to use test directory
	oldEnv := os.Getenv("AIMGR_REPO_PATH")
	defer func() {
		if oldEnv != "" {
			_ = os.Setenv("AIMGR_REPO_PATH", oldEnv)
		} else {
			_ = os.Unsetenv("AIMGR_REPO_PATH")
		}
	}()
	_ = os.Setenv("AIMGR_REPO_PATH", repoDir)

	// Initialize repository
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Create a command (.md extension required by List())
	commandContent := "---\nname: test-cmd\ndescription: A test command\n---\nCommand content here\n"
	commandPath := filepath.Join(repoDir, "commands", "test-cmd.md")
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Create metadata with wrong type (skill instead of command).
	// The metadata must be stored in the commands metadata directory so that
	// verifyRepository can find it via GetMetadata("test-cmd", resource.Command).
	// We write the JSON directly so the file path uses the command type but the
	// "type" field inside the JSON contains "skill", triggering the mismatch check.
	wrongTypeMeta := metadata.ResourceMetadata{
		Name:           "test-cmd",
		Type:           resource.Skill, // Wrong type in JSON content!
		SourceType:     "local",
		SourceURL:      "file:///test/source",
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	metaData, err := json.MarshalIndent(wrongTypeMeta, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}
	// Store at the commands path so GetMetadata("test-cmd", resource.Command) finds it
	metaPath := metadata.GetMetadataPath("test-cmd", resource.Command, repoDir)
	if err := os.MkdirAll(filepath.Dir(metaPath), 0755); err != nil {
		t.Fatalf("Failed to create metadata directory: %v", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Run verification
	result, err := verifyRepository(manager, false, nil)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}

	// Should have type mismatch error
	if !result.HasErrors {
		t.Error("Expected errors for type mismatch")
	}

	if len(result.TypeMismatches) == 0 {
		t.Error("Expected type mismatch to be detected")
	}
}

// TestRepoVerifyFixDeprecationWarning verifies that using --fix with 'aimgr repo verify'
// prints a deprecation warning to stderr.
func TestRepoVerifyFixDeprecationWarning(t *testing.T) {
	// Create temp directory for test repository
	repoDir := t.TempDir()

	// Set AIMGR_REPO_PATH to use test directory
	oldEnv := os.Getenv("AIMGR_REPO_PATH")
	defer func() {
		if oldEnv != "" {
			_ = os.Setenv("AIMGR_REPO_PATH", oldEnv)
		} else {
			_ = os.Unsetenv("AIMGR_REPO_PATH")
		}
	}()
	_ = os.Setenv("AIMGR_REPO_PATH", repoDir)

	// Initialize repository
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Create a resource without metadata so there is something for --fix to act on
	// (.md extension required by List())
	commandContent := "---\nname: dep-test-cmd\ndescription: A test command for deprecation warning test\n---\nCommand content here\n"
	commandPath := filepath.Join(repoDir, "commands", "dep-test-cmd.md")
	if err := os.WriteFile(commandPath, []byte(commandContent), 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}

	// Set the verifyFix flag to simulate --fix being passed
	oldVerifyFix := verifyFix
	verifyFix = true
	defer func() { verifyFix = oldVerifyFix }()

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Run verify command
	_ = repoVerifyCmd.RunE(repoVerifyCmd, []string{})

	// Restore stderr and read captured output
	w.Close()
	os.Stderr = oldStderr
	captured := make([]byte, 4096)
	n, _ := r.Read(captured)
	stderrOutput := string(captured[:n])

	// Verify deprecation warning was printed
	if !strings.Contains(stderrOutput, "Warning: --fix is deprecated. Use 'aimgr repo repair' instead.") {
		t.Errorf("Expected deprecation warning on stderr, got: %q", stderrOutput)
	}
}

func TestSetVerifyStatusForCompletedResult(t *testing.T) {
	t.Run("clean result", func(t *testing.T) {
		result := &VerifyResult{HasErrors: false, HasWarnings: false}
		setVerifyStatusForCompletedResult(result, false)
		if result.Status != verifyStatusClean {
			t.Fatalf("status=%q want %q", result.Status, verifyStatusClean)
		}
	})

	t.Run("warnings only is completed_with_findings", func(t *testing.T) {
		result := &VerifyResult{HasErrors: false, HasWarnings: true}
		setVerifyStatusForCompletedResult(result, false)
		if result.Status != verifyStatusCompletedWithFindings {
			t.Fatalf("status=%q want %q", result.Status, verifyStatusCompletedWithFindings)
		}
	})

	t.Run("errors is completed_with_findings", func(t *testing.T) {
		result := &VerifyResult{HasErrors: true, HasWarnings: false}
		setVerifyStatusForCompletedResult(result, false)
		if result.Status != verifyStatusCompletedWithFindings {
			t.Fatalf("status=%q want %q", result.Status, verifyStatusCompletedWithFindings)
		}
	})

	t.Run("fixed resources without remaining findings is clean", func(t *testing.T) {
		result := &VerifyResult{
			ResourcesWithoutMetadata: []ResourceIssue{{Name: "cmd", Type: resource.Command}},
			HasWarnings:              true,
		}
		setVerifyStatusForCompletedResult(result, true)
		if result.Status != verifyStatusClean {
			t.Fatalf("status=%q want %q", result.Status, verifyStatusClean)
		}
		if result.HasWarnings {
			t.Fatalf("expected repaired missing metadata not to leave warnings")
		}
	})

	t.Run("fixed with unresolved missing source path remains completed_with_findings", func(t *testing.T) {
		result := &VerifyResult{
			ResourcesWithoutMetadata: []ResourceIssue{{Name: "cmd", Type: resource.Command}},
			MissingSourcePaths:       []MetadataIssue{{Name: "cmd", Type: resource.Command}},
			HasWarnings:              true,
		}
		setVerifyStatusForCompletedResult(result, true)
		if result.Status != verifyStatusCompletedWithFindings {
			t.Fatalf("status=%q want %q", result.Status, verifyStatusCompletedWithFindings)
		}
	})
}

func TestOutputVerifyOperationalFailure(t *testing.T) {
	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("pipe: %v", pipeErr)
	}
	os.Stdout = w

	err := outputVerifyOperationalFailure(output.JSON, &repolock.AcquireTimeoutError{Path: "/tmp/repo.lock", Timeout: time.Second})
	_ = w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("outputVerifyOperationalFailure returned error: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	raw := string(buf[:n])

	var parsed VerifyResult
	if unmarshalErr := json.Unmarshal([]byte(raw), &parsed); unmarshalErr != nil {
		t.Fatalf("unmarshal output: %v; output=%s", unmarshalErr, raw)
	}

	if parsed.Status != verifyStatusExecutionFailed {
		t.Fatalf("status=%q want %q", parsed.Status, verifyStatusExecutionFailed)
	}
	if parsed.Error == nil {
		t.Fatalf("expected error payload")
	}
	if parsed.Error.Category != string(commandErrorCategoryRepoBusy) {
		t.Fatalf("error.category=%q want %q", parsed.Error.Category, commandErrorCategoryRepoBusy)
	}
	if parsed.HasErrors || parsed.HasWarnings {
		t.Fatalf("expected has_errors=false and has_warnings=false for execution failure")
	}
}

func TestRepoVerifyLockContentionReturnsTypedExitError(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	manager := repo.NewManagerWithPath(repoDir)
	lock, err := manager.AcquireRepoWriteLock(context.Background())
	if err != nil {
		t.Fatalf("acquire setup lock: %v", err)
	}
	defer func() { _ = lock.Unlock() }()

	oldFormat := verifyFormatFlag
	oldJSON := verifyJSON
	verifyFormatFlag = "json"
	verifyJSON = false
	defer func() {
		verifyFormatFlag = oldFormat
		verifyJSON = oldJSON
	}()

	err = repoVerifyCmd.RunE(repoVerifyCmd, nil)
	if err == nil {
		t.Fatalf("expected error")
	}

	var cmdErr *commandExitError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected commandExitError, got %T", err)
	}
	if cmdErr.ExitCode != commandExitCodeOperationalFailure {
		t.Fatalf("exit code=%d want %d", cmdErr.ExitCode, commandExitCodeOperationalFailure)
	}
	if cmdErr.Category != commandErrorCategoryRepoBusy {
		t.Fatalf("category=%q want %q", cmdErr.Category, commandErrorCategoryRepoBusy)
	}
}

func TestRepoVerifyWarningsReturnCompletedWithFindingsExit(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	commandPath := filepath.Join(repoDir, "commands", "warn-only.md")
	if err := os.WriteFile(commandPath, []byte("---\ndescription: warn\n---\nbody\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}

	oldFormat := verifyFormatFlag
	oldJSON := verifyJSON
	verifyFormatFlag = "json"
	verifyJSON = false
	defer func() {
		verifyFormatFlag = oldFormat
		verifyJSON = oldJSON
	}()

	err := repoVerifyCmd.RunE(repoVerifyCmd, nil)
	if err == nil {
		t.Fatalf("expected completed-with-findings error")
	}

	var cmdErr *commandExitError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected commandExitError, got %T", err)
	}
	if cmdErr.ExitCode != commandExitCodeCompletedWithFindings {
		t.Fatalf("exit code=%d want %d", cmdErr.ExitCode, commandExitCodeCompletedWithFindings)
	}
}
