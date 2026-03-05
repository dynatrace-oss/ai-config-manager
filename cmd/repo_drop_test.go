package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
)

func TestPerformSoftDrop(t *testing.T) {
	// Create temp directory for test repo
	tempDir := t.TempDir()

	// Create repo manager
	mgr := repo.NewManagerWithPath(tempDir)

	// Initialize repo
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Add some test resources
	commandsPath := filepath.Join(tempDir, "commands")
	skillsPath := filepath.Join(tempDir, "skills")
	agentsPath := filepath.Join(tempDir, "agents")

	// Create test command
	testCommandPath := filepath.Join(commandsPath, "test-command.md")
	testCommandContent := `---
name: test-command
description: Test command
---
# Test Command`
	if err := os.WriteFile(testCommandPath, []byte(testCommandContent), 0644); err != nil {
		t.Fatalf("Failed to create test command: %v", err)
	}

	// Create test skill directory
	testSkillPath := filepath.Join(skillsPath, "test-skill")
	if err := os.MkdirAll(testSkillPath, 0755); err != nil {
		t.Fatalf("Failed to create test skill dir: %v", err)
	}
	testSkillFile := filepath.Join(testSkillPath, "SKILL.md")
	testSkillContent := `---
name: test-skill
description: Test skill
---
# Test Skill`
	if err := os.WriteFile(testSkillFile, []byte(testSkillContent), 0644); err != nil {
		t.Fatalf("Failed to create test skill: %v", err)
	}

	// Create test agent
	testAgentPath := filepath.Join(agentsPath, "test-agent.md")
	testAgentContent := `---
name: test-agent
description: Test agent
---
# Test Agent`
	if err := os.WriteFile(testAgentPath, []byte(testAgentContent), 0644); err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	// Verify resources exist
	if _, err := os.Stat(testCommandPath); os.IsNotExist(err) {
		t.Fatal("Test command should exist before soft drop")
	}
	if _, err := os.Stat(testSkillPath); os.IsNotExist(err) {
		t.Fatal("Test skill should exist before soft drop")
	}
	if _, err := os.Stat(testAgentPath); os.IsNotExist(err) {
		t.Fatal("Test agent should exist before soft drop")
	}

	// Perform soft drop
	if err := performSoftDrop(mgr); err != nil {
		t.Fatalf("Soft drop failed: %v", err)
	}

	// Verify structure is preserved
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Error("Repository directory should still exist after soft drop")
	}

	// Verify ai.repo.yaml exists (recreated by Init())
	manifestPath := filepath.Join(tempDir, repomanifest.ManifestFileName)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("ai.repo.yaml should exist after soft drop")
	}

	// Verify .git directory exists (recreated by Init())
	gitPath := filepath.Join(tempDir, ".git")
	if _, err := os.Stat(gitPath); os.IsNotExist(err) {
		t.Error(".git directory should exist after soft drop")
	}

	// Verify .gitignore exists (recreated by Init())
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		t.Error(".gitignore should exist after soft drop")
	}

	// Verify subdirectories exist (recreated by Init())
	if _, err := os.Stat(commandsPath); os.IsNotExist(err) {
		t.Error("commands directory should exist after soft drop")
	}
	if _, err := os.Stat(skillsPath); os.IsNotExist(err) {
		t.Error("skills directory should exist after soft drop")
	}
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		t.Error("agents directory should exist after soft drop")
	}

	// Verify resources are removed
	if _, err := os.Stat(testCommandPath); !os.IsNotExist(err) {
		t.Error("Test command should be removed after soft drop")
	}
	if _, err := os.Stat(testSkillPath); !os.IsNotExist(err) {
		t.Error("Test skill should be removed after soft drop")
	}
	if _, err := os.Stat(testAgentPath); !os.IsNotExist(err) {
		t.Error("Test agent should be removed after soft drop")
	}

	// Verify ai.repo.yaml is empty (no sources)
	manifest, err := repomanifest.Load(tempDir)
	if err != nil {
		t.Fatalf("Failed to load manifest after soft drop: %v", err)
	}
	if len(manifest.Sources) != 0 {
		t.Errorf("Expected empty sources after soft drop, got %d sources", len(manifest.Sources))
	}
}

func TestPerformFullDelete_WithForce(t *testing.T) {
	// Create temp directory for test repo
	tempDir := t.TempDir()

	// Create repo manager
	mgr := repo.NewManagerWithPath(tempDir)

	// Initialize repo
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Add some test content
	testFile := filepath.Join(tempDir, "commands", "test.md")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify repo exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Fatal("Repository should exist before full delete")
	}

	// Perform full delete with force (no confirmation needed)
	if err := performFullDelete(mgr, true); err != nil {
		t.Fatalf("Full delete with force failed: %v", err)
	}

	// Verify entire directory is gone
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Error("Repository directory should not exist after full delete")
	}
}

func TestPerformFullDelete_NonExistentRepo(t *testing.T) {
	// Create a path that doesn't exist
	tempDir := filepath.Join(t.TempDir(), "nonexistent")

	// Create repo manager
	mgr := repo.NewManagerWithPath(tempDir)

	// Try to perform full delete on non-existent repo
	err := performFullDelete(mgr, true)
	if err == nil {
		t.Fatal("Expected error when deleting non-existent repository")
	}

	// Verify error message mentions repo doesn't exist
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected error about non-existent repo, got: %v", err)
	}
}

func TestPerformFullDelete_PreservesParentDir(t *testing.T) {
	// Create parent and repo directories
	parentDir := t.TempDir()
	repoDir := filepath.Join(parentDir, "repo")

	// Create repo manager
	mgr := repo.NewManagerWithPath(repoDir)

	// Initialize repo
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Create a file in parent directory (outside repo)
	markerFile := filepath.Join(parentDir, "marker.txt")
	if err := os.WriteFile(markerFile, []byte("marker"), 0644); err != nil {
		t.Fatalf("Failed to create marker file: %v", err)
	}

	// Verify both exist
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		t.Fatal("Repository should exist before full delete")
	}
	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Fatal("Marker file should exist before full delete")
	}

	// Perform full delete with force
	if err := performFullDelete(mgr, true); err != nil {
		t.Fatalf("Full delete failed: %v", err)
	}

	// Verify repo is gone but parent and marker file still exist
	if _, err := os.Stat(repoDir); !os.IsNotExist(err) {
		t.Error("Repository should not exist after full delete")
	}
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Error("Parent directory should still exist after full delete")
	}
	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Error("Marker file in parent directory should still exist after full delete")
	}
}

func TestSoftDrop_NestedCommands(t *testing.T) {
	// Create temp directory for test repo
	tempDir := t.TempDir()

	// Create repo manager
	mgr := repo.NewManagerWithPath(tempDir)

	// Initialize repo
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Create nested command structure
	nestedPath := filepath.Join(tempDir, "commands", "api", "deploy")
	if err := os.MkdirAll(nestedPath, 0755); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	testCommandPath := filepath.Join(nestedPath, "prod.md")
	testCommandContent := `---
name: api/deploy/prod
description: Deploy to production
---
# Deploy to Production`
	if err := os.WriteFile(testCommandPath, []byte(testCommandContent), 0644); err != nil {
		t.Fatalf("Failed to create nested command: %v", err)
	}

	// Verify nested command exists
	if _, err := os.Stat(testCommandPath); os.IsNotExist(err) {
		t.Fatal("Nested command should exist before soft drop")
	}

	// Perform soft drop
	if err := performSoftDrop(mgr); err != nil {
		t.Fatalf("Soft drop failed: %v", err)
	}

	// Verify nested structure is removed
	if _, err := os.Stat(testCommandPath); !os.IsNotExist(err) {
		t.Error("Nested command should be removed after soft drop")
	}

	// Verify base commands directory still exists (recreated by Init())
	commandsPath := filepath.Join(tempDir, "commands")
	if _, err := os.Stat(commandsPath); os.IsNotExist(err) {
		t.Error("Commands directory should exist after soft drop")
	}

	// Verify nested api/deploy directories are removed
	apiPath := filepath.Join(tempDir, "commands", "api")
	if _, err := os.Stat(apiPath); !os.IsNotExist(err) {
		t.Error("Nested api directory should be removed after soft drop")
	}
}

func TestSoftDrop_PreservesMetadataStructure(t *testing.T) {
	// Create temp directory for test repo
	tempDir := t.TempDir()

	// Create repo manager
	mgr := repo.NewManagerWithPath(tempDir)

	// Initialize repo
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Create .metadata directory with some content
	metadataDir := filepath.Join(tempDir, ".metadata", "commands")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		t.Fatalf("Failed to create metadata directory: %v", err)
	}

	metadataFile := filepath.Join(metadataDir, "test-metadata.json")
	if err := os.WriteFile(metadataFile, []byte(`{"name":"test"}`), 0644); err != nil {
		t.Fatalf("Failed to create metadata file: %v", err)
	}

	// Verify metadata exists
	if _, err := os.Stat(metadataFile); os.IsNotExist(err) {
		t.Fatal("Metadata file should exist before soft drop")
	}

	// Perform soft drop
	if err := performSoftDrop(mgr); err != nil {
		t.Fatalf("Soft drop failed: %v", err)
	}

	// Verify .metadata is removed (Drop() removes everything)
	if _, err := os.Stat(metadataFile); !os.IsNotExist(err) {
		t.Error("Metadata file should be removed after soft drop")
	}
	if _, err := os.Stat(metadataDir); !os.IsNotExist(err) {
		t.Error("Metadata directory should be removed after soft drop")
	}
}

func TestSoftDrop_EmptyRepo(t *testing.T) {
	// Create temp directory for test repo
	tempDir := t.TempDir()

	// Create repo manager
	mgr := repo.NewManagerWithPath(tempDir)

	// Initialize repo
	if err := mgr.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Don't add any resources - test soft drop on empty repo

	// Perform soft drop
	if err := performSoftDrop(mgr); err != nil {
		t.Fatalf("Soft drop on empty repo failed: %v", err)
	}

	// Verify structure is still created
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Error("Repository directory should exist after soft drop")
	}

	manifestPath := filepath.Join(tempDir, repomanifest.ManifestFileName)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("ai.repo.yaml should exist after soft drop")
	}

	gitPath := filepath.Join(tempDir, ".git")
	if _, err := os.Stat(gitPath); os.IsNotExist(err) {
		t.Error(".git directory should exist after soft drop")
	}
}
