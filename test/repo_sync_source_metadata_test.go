//go:build integration

package test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"gopkg.in/yaml.v3"
)

// TestSyncSourceNamePassthrough verifies that repo sync passes the manifest
// source name (not the directory basename) through to resource metadata.
//
// Scenario: A source directory is named "tools-v2" but the manifest source
// has name "my-tools". After sync, resource metadata should have
// source_name="my-tools" (NOT "tools-v2").
//
// This also verifies that source_id is threaded through correctly.
func TestSyncSourceNamePassthrough(t *testing.T) {
	testDir := t.TempDir()
	configDir := filepath.Join(testDir, "config")
	dataDir := filepath.Join(testDir, "data")

	// Create a source directory with a DIFFERENT basename than the manifest name
	// Directory: "tools-v2" but manifest name: "my-tools"
	sourceDir := filepath.Join(testDir, "tools-v2")

	// Create directories
	for _, dir := range []string{configDir, dataDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create a command resource in the source directory
	commandsDir := filepath.Join(sourceDir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}
	cmdContent := `---
description: Test command for source metadata passthrough
---
# sync-meta-test
This is a test command for verifying source name passthrough during sync.
`
	cmdPath := filepath.Join(commandsDir, "sync-meta-test.md")
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}

	// Create minimal config
	aimgrConfigDir := filepath.Join(configDir, "aimgr")
	if err := os.MkdirAll(aimgrConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create aimgr config dir: %v", err)
	}
	configContent := &config.Config{
		Install: config.InstallConfig{
			Targets: []string{"claude"},
		},
	}
	configData, err := yaml.Marshal(configContent)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	configPath := filepath.Join(aimgrConfigDir, "aimgr.yaml")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Repo path
	repoDir := filepath.Join(dataDir, "repo")

	// Step 1: Initialize the repo by adding the source with aimgr repo add
	// This establishes the manifest. But to control the source name, we need to
	// use --name flag OR write the manifest ourselves.
	// We'll write the manifest directly so the source name differs from dir name.
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Compute expected source ID
	absSourceDir, err := filepath.Abs(sourceDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}
	expectedSourceID := repomanifest.GenerateSourceID(&repomanifest.Source{Path: absSourceDir})
	t.Logf("Expected source ID: %s", expectedSourceID)

	// Write ai.repo.yaml with manifest name "my-tools" but pointing to dir "tools-v2"
	manifestContent := "version: 1\nsources:\n  - id: " + expectedSourceID +
		"\n    name: my-tools\n    path: " + sourceDir + "\n"
	aiRepoPath := filepath.Join(repoDir, "ai.repo.yaml")
	if err := os.WriteFile(aiRepoPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to write ai.repo.yaml: %v", err)
	}
	t.Logf("Created ai.repo.yaml with source name 'my-tools' -> dir '%s'", sourceDir)

	// Step 2: Run repo sync
	binPath := filepath.Join("..", "aimgr")
	runCommand := func(name string, args ...string) (string, int) {
		t.Helper()
		t.Logf("[%s] Running: aimgr %s", name, strings.Join(args, " "))
		cmd := exec.Command(binPath, args...)
		cmd.Env = append(os.Environ(),
			"XDG_CONFIG_HOME="+configDir,
			"XDG_DATA_HOME="+dataDir,
			"AIMGR_REPO_PATH="+repoDir,
		)
		output, err := cmd.CombinedOutput()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				t.Fatalf("[%s] Failed to execute: %v", name, err)
			}
		}
		t.Logf("[%s] Exit code: %d\n%s", name, exitCode, string(output))
		return string(output), exitCode
	}

	output, exitCode := runCommand("sync", "repo", "sync")
	if exitCode != 0 {
		t.Fatalf("Sync failed with exit code %d\nOutput: %s", exitCode, output)
	}

	// Step 3: Verify resource was synced
	syncedCmd := filepath.Join(repoDir, "commands", "sync-meta-test.md")
	if _, err := os.Stat(syncedCmd); os.IsNotExist(err) {
		t.Fatalf("Command should have been synced to repo: %s", syncedCmd)
	}

	// Step 4: Verify resource metadata has correct source_name and source_id
	metaPath := metadata.GetMetadataPath("sync-meta-test", resource.Command, repoDir)
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read metadata file %s: %v", metaPath, err)
	}

	var meta metadata.ResourceMetadata
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	t.Logf("Resource metadata: source_name=%q, source_id=%q", meta.SourceName, meta.SourceID)

	// Verify source_name is from manifest (NOT from directory basename)
	if meta.SourceName != "my-tools" {
		t.Errorf("source_name = %q, want %q (should be manifest name, not dir basename %q)",
			meta.SourceName, "my-tools", filepath.Base(sourceDir))
	}

	// Verify source_id matches what GenerateSourceID produces
	if meta.SourceID != expectedSourceID {
		t.Errorf("source_id = %q, want %q", meta.SourceID, expectedSourceID)
	}

	// Verify source_name is NOT the directory basename
	if meta.SourceName == "tools-v2" {
		t.Error("source_name should be manifest name 'my-tools', not directory basename 'tools-v2' — this indicates the passthrough bug")
	}

	t.Log("✓ Source name and ID correctly threaded through sync")
}
