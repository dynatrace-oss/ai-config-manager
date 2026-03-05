package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
)

// Test describeDetailedResource with exact match

func TestDescribeDetailedResource_Skill(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	err := describeDetailedResource(mgr, "skill/test-skill", "table")
	if err != nil {
		t.Errorf("describeDetailedResource() error = %v, want nil", err)
	}
}

func TestDescribeDetailedResource_Command(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	err := describeDetailedResource(mgr, "command/test-command", "table")
	if err != nil {
		t.Errorf("describeDetailedResource() error = %v, want nil", err)
	}
}

func TestDescribeDetailedResource_Agent(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	err := describeDetailedResource(mgr, "agent/test-agent", "table")
	if err != nil {
		t.Errorf("describeDetailedResource() error = %v, want nil", err)
	}
}

func TestDescribeDetailedResource_NotFound(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	err := describeDetailedResource(mgr, "skill/nonexistent", "table")
	if err == nil {
		t.Error("describeDetailedResource() expected error for nonexistent resource, got nil")
	}
}

func TestDescribeDetailedResource_InvalidFormat(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	tests := []struct {
		name        string
		arg         string
		errContains string
	}{
		{
			name:        "missing slash",
			arg:         "test-skill",
			errContains: "invalid format",
		},
		{
			name:        "invalid type",
			arg:         "invalid/test",
			errContains: "invalid resource type",
		},
		{
			name:        "empty name",
			arg:         "skill/",
			errContains: "name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := describeDetailedResource(mgr, tt.arg, "table")
			if err == nil {
				t.Errorf("describeDetailedResource(%q) expected error, got nil", tt.arg)
				return
			}

			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("describeDetailedResource(%q) error = %v, want error containing %q", tt.arg, err, tt.errContains)
			}
		})
	}
}

// Test describeResourceSummary

func TestDescribeResourceSummary_MultipleMatches(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	matches := []string{
		"skill/test-skill",
		"skill/pdf-processing",
		"command/test-command",
	}

	err := describeResourceSummary(mgr, matches, "table")
	if err != nil {
		t.Errorf("describeResourceSummary() error = %v, want nil", err)
	}
}

func TestDescribeResourceSummary_EmptyMatches(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	matches := []string{}

	err := describeResourceSummary(mgr, matches, "table")
	if err != nil {
		t.Errorf("describeResourceSummary() error = %v, want nil", err)
	}
}

func TestDescribeResourceSummary_InvalidMatches(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	// Include some invalid matches - should be skipped
	matches := []string{
		"skill/test-skill",
		"invalid-format",
		"skill/nonexistent",
		"command/test-command",
	}

	err := describeResourceSummary(mgr, matches, "table")
	if err != nil {
		t.Errorf("describeResourceSummary() error = %v, want nil", err)
	}
}

// Integration tests with ExpandPattern

func TestRepoDescribe_ExactMatch(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "exact skill match",
			pattern: "skill/test-skill",
			wantErr: false,
		},
		{
			name:    "exact command match",
			pattern: "command/test-command",
			wantErr: false,
		},
		{
			name:    "exact agent match",
			pattern: "agent/test-agent",
			wantErr: false,
		},
		{
			name:    "nonexistent resource",
			pattern: "skill/nonexistent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := ExpandPattern(mgr, tt.pattern)
			if err != nil {
				t.Fatalf("ExpandPattern() error = %v", err)
			}

			if len(matches) == 0 {
				if !tt.wantErr {
					t.Error("ExpandPattern() returned no matches, expected at least one")
				}
				return
			}

			if len(matches) == 1 {
				err = describeDetailedResource(mgr, matches[0], "table")
			} else {
				err = describeResourceSummary(mgr, matches, "table")
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("describe operation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRepoDescribe_PatternWithSingleMatch(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	pattern := "skill/test-skill"
	matches, err := ExpandPattern(mgr, pattern)
	if err != nil {
		t.Fatalf("ExpandPattern() error = %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("ExpandPattern() returned %d matches, want 1", len(matches))
	}

	// Should show detailed view
	err = describeDetailedResource(mgr, matches[0], "table")
	if err != nil {
		t.Errorf("describeDetailedResource() error = %v", err)
	}
}

func TestRepoDescribe_PatternWithMultipleMatches(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	tests := []struct {
		name        string
		pattern     string
		minMatches  int
		shouldError bool
	}{
		{
			name:        "all skills",
			pattern:     "skill/*",
			minMatches:  2,
			shouldError: false,
		},
		{
			name:        "all commands",
			pattern:     "command/*",
			minMatches:  2,
			shouldError: false,
		},
		{
			name:        "cross-type pattern",
			pattern:     "*test*",
			minMatches:  2,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := ExpandPattern(mgr, tt.pattern)
			if err != nil {
				t.Fatalf("ExpandPattern() error = %v", err)
			}

			if len(matches) < tt.minMatches {
				t.Fatalf("ExpandPattern() returned %d matches, want at least %d", len(matches), tt.minMatches)
			}

			// Should show summary
			err = describeResourceSummary(mgr, matches, "table")
			if (err != nil) != tt.shouldError {
				t.Errorf("describeResourceSummary() error = %v, wantErr %v", err, tt.shouldError)
			}
		})
	}
}

func TestRepoDescribe_PatternWithNoMatches(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	pattern := "skill/nonexistent*"
	matches, err := ExpandPattern(mgr, pattern)
	if err != nil {
		t.Fatalf("ExpandPattern() error = %v", err)
	}

	if len(matches) != 0 {
		t.Errorf("ExpandPattern() returned %d matches, want 0", len(matches))
	}
}

func TestRepoDescribe_InvalidPattern(t *testing.T) {
	repoPath, cleanup := createTestRepoForDescribe(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	tests := []struct {
		name        string
		pattern     string
		errContains string
	}{
		{
			name:        "unclosed bracket",
			pattern:     "skill/[unclosed",
			errContains: "invalid pattern",
		},
		{
			name:        "invalid glob",
			pattern:     "skill/[",
			errContains: "invalid pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExpandPattern(mgr, tt.pattern)
			if err == nil {
				t.Errorf("ExpandPattern(%q) expected error, got nil", tt.pattern)
				return
			}

			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("ExpandPattern(%q) error = %v, want error containing %q", tt.pattern, err, tt.errContains)
			}
		})
	}
}

// Test helpers

func createTestRepoForDescribe(t *testing.T) (repoPath string, cleanup func()) {
	t.Helper()

	// Create temp directory for repo
	tempDir := t.TempDir()
	repoPath = tempDir

	// Create directory structure
	if err := os.MkdirAll(filepath.Join(repoPath, "commands"), 0755); err != nil {
		t.Fatalf("failed to create commands dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoPath, "skills"), 0755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoPath, "agents"), 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}

	// Create test commands
	testCommands := []struct {
		name    string
		desc    string
		version string
	}{
		{"test-command", "Test command for unit tests", "1.0.0"},
		{"pdf-command", "PDF processing command", "1.1.0"},
		{"review-command", "Code review command", "2.0.0"},
	}
	for _, cmd := range testCommands {
		content := fmt.Sprintf("---\ndescription: %s\nversion: %s\n---\n# %s\n\nCommand content here.", cmd.desc, cmd.version, cmd.name)
		path := filepath.Join(repoPath, "commands", cmd.name+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create command %s: %v", cmd.name, err)
		}
	}

	// Create test skills
	testSkills := []struct {
		name    string
		desc    string
		version string
	}{
		{"test-skill", "Test skill for unit tests", "1.0.0"},
		{"pdf-processing", "PDF processing skill", "1.2.0"},
		{"image-processing", "Image processing skill", "2.1.0"},
	}
	for _, skill := range testSkills {
		skillDir := filepath.Join(repoPath, "skills", skill.name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("failed to create skill dir %s: %v", skill.name, err)
		}
		content := fmt.Sprintf("---\ndescription: %s\nversion: %s\n---\n# %s\n\nSkill content here.", skill.desc, skill.version, skill.name)
		path := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create skill %s: %v", skill.name, err)
		}
	}

	// Create test agents
	testAgents := []struct {
		name    string
		desc    string
		version string
	}{
		{"test-agent", "Test agent for unit tests", "1.0.0"},
		{"code-reviewer", "Code review agent", "1.5.0"},
		{"doc-generator", "Documentation generator agent", "2.0.0"},
	}
	for _, agent := range testAgents {
		content := fmt.Sprintf("---\ndescription: %s\nversion: %s\n---\n# %s\n\nAgent content here.", agent.desc, agent.version, agent.name)
		path := filepath.Join(repoPath, "agents", agent.name+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create agent %s: %v", agent.name, err)
		}
	}

	cleanup = func() {
		// Cleanup is automatic with t.TempDir()
	}

	return repoPath, cleanup
}
