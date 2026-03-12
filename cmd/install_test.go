package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
)

func TestParseTargetFlag(t *testing.T) {
	tests := []struct {
		name        string
		targetFlag  string
		want        []tools.Tool
		wantErr     bool
		errContains string
	}{
		{
			name:       "empty flag returns nil",
			targetFlag: "",
			want:       nil,
			wantErr:    false,
		},
		{
			name:       "single valid target - claude",
			targetFlag: "claude",
			want:       []tools.Tool{tools.Claude},
			wantErr:    false,
		},
		{
			name:       "single valid target - opencode",
			targetFlag: "opencode",
			want:       []tools.Tool{tools.OpenCode},
			wantErr:    false,
		},
		{
			name:       "single valid target - copilot",
			targetFlag: "copilot",
			want:       []tools.Tool{tools.Copilot},
			wantErr:    false,
		},
		{
			name:       "multiple valid targets",
			targetFlag: "claude,opencode",
			want:       []tools.Tool{tools.Claude, tools.OpenCode},
			wantErr:    false,
		},
		{
			name:       "all three targets",
			targetFlag: "claude,opencode,copilot",
			want:       []tools.Tool{tools.Claude, tools.OpenCode, tools.Copilot},
			wantErr:    false,
		},
		{
			name:       "targets with spaces",
			targetFlag: "claude, opencode, copilot",
			want:       []tools.Tool{tools.Claude, tools.OpenCode, tools.Copilot},
			wantErr:    false,
		},
		{
			name:        "invalid target",
			targetFlag:  "invalid",
			want:        nil,
			wantErr:     true,
			errContains: "invalid target 'invalid'",
		},
		{
			name:        "mixed valid and invalid",
			targetFlag:  "claude,invalid,opencode",
			want:        nil,
			wantErr:     true,
			errContains: "invalid target 'invalid'",
		},
		{
			name:       "case insensitive",
			targetFlag: "CLAUDE,OpenCode,CoPilot",
			want:       []tools.Tool{tools.Claude, tools.OpenCode, tools.Copilot},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTargetFlag(tt.targetFlag)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTargetFlag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.errContains != "" {
					if !strings.Contains(err.Error(), tt.errContains) {
						t.Errorf("parseTargetFlag() error = %v, want error containing %q", err, tt.errContains)
					}
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("parseTargetFlag() got %d tools, want %d", len(got), len(tt.want))
				return
			}

			for i, tool := range tt.want {
				if got[i] != tool {
					t.Errorf("parseTargetFlag()[%d] = %v, want %v", i, got[i], tool)
				}
			}
		})
	}
}

func captureInstallSummaryOutput(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close stdout writer: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("failed to close stdout reader: %v", err)
	}

	return buf.String()
}

func TestPrintInstallSummary_CopilotAgentPathUsesAgentSuffix(t *testing.T) {
	results := []installResult{{
		resourceType: resource.Agent,
		name:         "reviewer",
		success:      true,
		toolsAdded:   []tools.Tool{tools.Copilot},
	}}

	output := captureInstallSummaryOutput(t, func() {
		printInstallSummary(results)
	})

	if !strings.Contains(output, "  → .github/agents/reviewer.agent.md") {
		t.Fatalf("expected Copilot agent summary path with .agent.md, got output:\n%s", output)
	}

	if strings.Contains(output, "  → .github/agents/reviewer.md") {
		t.Fatalf("expected Copilot agent summary path to not use .md suffix, got output:\n%s", output)
	}
}

func TestPrintInstallSummary_ClaudeOpenCodeAgentPathsRemainMd(t *testing.T) {
	results := []installResult{{
		resourceType: resource.Agent,
		name:         "reviewer",
		success:      true,
		toolsAdded:   []tools.Tool{tools.Claude, tools.OpenCode},
	}}

	output := captureInstallSummaryOutput(t, func() {
		printInstallSummary(results)
	})

	if !strings.Contains(output, "  → .claude/agents/reviewer.md") {
		t.Fatalf("expected Claude agent summary path with .md, got output:\n%s", output)
	}

	if !strings.Contains(output, "  → .opencode/agents/reviewer.md") {
		t.Fatalf("expected OpenCode agent summary path with .md, got output:\n%s", output)
	}
}

// Test helpers for setting up test resources

// createTestRepo creates a temporary repo with test resources
func createTestRepo(t *testing.T) string {
	t.Helper()

	// Create temp directory for repo
	repoPath := t.TempDir()

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
	testCommands := []string{"test-command", "pdf-command", "review-command"}
	for _, name := range testCommands {
		content := fmt.Sprintf("---\ndescription: %s\n---\n# %s", name, name)
		path := filepath.Join(repoPath, "commands", name+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create command %s: %v", name, err)
		}
	}

	// Create test skills
	testSkills := []struct {
		name    string
		content string
	}{
		{"pdf-processing", "PDF processing skill"},
		{"pdf-extraction", "PDF extraction skill"},
		{"image-processing", "Image processing skill"},
		{"test-skill", "Test skill"},
	}
	for _, skill := range testSkills {
		skillDir := filepath.Join(repoPath, "skills", skill.name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("failed to create skill dir %s: %v", skill.name, err)
		}
		content := fmt.Sprintf("---\ndescription: %s\n---\n# %s", skill.content, skill.name)
		path := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create skill %s: %v", skill.name, err)
		}
	}

	// Create test agents
	testAgents := []string{"code-reviewer", "test-agent", "doc-generator"}
	for _, name := range testAgents {
		content := fmt.Sprintf("---\ndescription: %s\n---\n# %s", name, name)
		path := filepath.Join(repoPath, "agents", name+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create agent %s: %v", name, err)
		}
	}

	return repoPath
}

// createTestProject creates a temporary project directory with tool directories
func createTestProject(t *testing.T) string {
	t.Helper()

	projectDir := t.TempDir()

	// Create .claude directories
	if err := os.MkdirAll(filepath.Join(projectDir, ".claude", "commands"), 0755); err != nil {
		t.Fatalf("failed to create .claude/commands: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".claude", "skills"), 0755); err != nil {
		t.Fatalf("failed to create .claude/skills: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".claude", "agents"), 0755); err != nil {
		t.Fatalf("failed to create .claude/agents: %v", err)
	}

	return projectDir
}

// verifyInstalled checks if resources are installed in the project
func verifyInstalled(t *testing.T, projectPath, toolDir string, resourceType resource.ResourceType, names ...string) {
	t.Helper()

	for _, name := range names {
		var path string
		switch resourceType {
		case resource.Command:
			path = filepath.Join(projectPath, toolDir, "commands", name+".md")
		case resource.Skill:
			path = filepath.Join(projectPath, toolDir, "skills", name)
		case resource.Agent:
			path = filepath.Join(projectPath, toolDir, "agents", name+".md")
		}

		info, err := os.Lstat(path)
		if err != nil {
			t.Errorf("resource %s/%s not installed at %s: %v", resourceType, name, path, err)
			continue
		}

		// Verify it's a symlink
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("resource %s/%s at %s is not a symlink", resourceType, name, path)
		}
	}
}

// verifyNotInstalled checks that resources are NOT installed
func verifyNotInstalled(t *testing.T, projectPath, toolDir string, resourceType resource.ResourceType, names ...string) {
	t.Helper()

	for _, name := range names {
		var path string
		switch resourceType {
		case resource.Command:
			path = filepath.Join(projectPath, toolDir, "commands", name+".md")
		case resource.Skill:
			path = filepath.Join(projectPath, toolDir, "skills", name)
		case resource.Agent:
			path = filepath.Join(projectPath, toolDir, "agents", name+".md")
		}

		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("resource %s/%s should not be installed at %s", resourceType, name, path)
		}
	}
}

// Test Pattern Expansion

func TestExpandPattern_AllSkills(t *testing.T) {
	repoPath := createTestRepo(t)

	mgr := repo.NewManagerWithPath(repoPath)

	// Expand "skill/*" pattern
	matches, err := ExpandPattern(mgr, "skill/*")
	if err != nil {
		t.Fatalf("ExpandPattern failed: %v", err)
	}

	// Should match all 4 skills
	expectedCount := 4
	if len(matches) != expectedCount {
		t.Errorf("ExpandPattern(skill/*) returned %d matches, want %d", len(matches), expectedCount)
	}

	// Verify all matches start with "skill/"
	for _, match := range matches {
		if !strings.HasPrefix(match, "skill/") {
			t.Errorf("match %q doesn't start with 'skill/'", match)
		}
	}
}

func TestExpandPattern_PrefixMatch(t *testing.T) {
	repoPath := createTestRepo(t)

	mgr := repo.NewManagerWithPath(repoPath)

	// Expand "skill/pdf*" pattern
	matches, err := ExpandPattern(mgr, "skill/pdf*")
	if err != nil {
		t.Fatalf("ExpandPattern failed: %v", err)
	}

	// Should match 2 skills (pdf-processing, pdf-extraction)
	expectedCount := 2
	if len(matches) != expectedCount {
		t.Errorf("ExpandPattern(skill/pdf*) returned %d matches, want %d", len(matches), expectedCount)
	}

	// Verify matches
	expectedMatches := map[string]bool{
		"skill/pdf-processing": true,
		"skill/pdf-extraction": true,
	}
	for _, match := range matches {
		if !expectedMatches[match] {
			t.Errorf("unexpected match: %q", match)
		}
	}
}

func TestExpandPattern_AllTypes(t *testing.T) {
	repoPath := createTestRepo(t)

	mgr := repo.NewManagerWithPath(repoPath)

	// Expand "*test*" pattern (matches across all types)
	matches, err := ExpandPattern(mgr, "*test*")
	if err != nil {
		t.Fatalf("ExpandPattern failed: %v", err)
	}

	// Should match: test-command, test-skill, test-agent
	expectedCount := 3
	if len(matches) != expectedCount {
		t.Errorf("ExpandPattern(*test*) returned %d matches, want %d", len(matches), expectedCount)
		for _, m := range matches {
			t.Logf("  match: %s", m)
		}
	}
}

func TestExpandPattern_ExactName(t *testing.T) {
	repoPath := createTestRepo(t)

	mgr := repo.NewManagerWithPath(repoPath)

	// Expand exact name (no pattern)
	matches, err := ExpandPattern(mgr, "skill/pdf-processing")
	if err != nil {
		t.Fatalf("ExpandPattern failed: %v", err)
	}

	// Should return the exact name
	if len(matches) != 1 {
		t.Fatalf("ExpandPattern(skill/pdf-processing) returned %d matches, want 1", len(matches))
	}
	if matches[0] != "skill/pdf-processing" {
		t.Errorf("ExpandPattern(skill/pdf-processing) = %q, want 'skill/pdf-processing'", matches[0])
	}
}

func TestExpandPattern_NoMatch(t *testing.T) {
	repoPath := createTestRepo(t)

	mgr := repo.NewManagerWithPath(repoPath)

	// Expand pattern with no matches
	matches, err := ExpandPattern(mgr, "skill/nomatch*")
	if err != nil {
		t.Fatalf("ExpandPattern failed: %v", err)
	}

	// Should return empty slice
	if len(matches) != 0 {
		t.Errorf("ExpandPattern(skill/nomatch*) returned %d matches, want 0", len(matches))
	}
}

func TestExpandPattern_InvalidPattern(t *testing.T) {
	repoPath := createTestRepo(t)

	mgr := repo.NewManagerWithPath(repoPath)

	// Test invalid glob pattern
	_, err := ExpandPattern(mgr, "skill/[unclosed")
	if err == nil {
		t.Error("ExpandPattern([unclosed) should return error for invalid glob")
	}
}

func TestExpandPattern_EmptyPattern(t *testing.T) {
	repoPath := createTestRepo(t)

	mgr := repo.NewManagerWithPath(repoPath)

	// Empty pattern should be treated as non-pattern
	matches, err := ExpandPattern(mgr, "skill/")
	if err != nil {
		t.Fatalf("ExpandPattern failed: %v", err)
	}

	// Should return the arg as-is (will fail later in validation)
	if len(matches) != 1 || matches[0] != "skill/" {
		t.Errorf("ExpandPattern(skill/) = %v, want [skill/]", matches)
	}
}

// Integration Tests - Full workflow with patterns

func TestProcessInstall_WithPatternExpansion(t *testing.T) {
	// This test verifies that processInstall works correctly with expanded patterns
	// Note: This is more of a unit test of processInstall, full integration would need
	// actual installer setup which is complex for testing

	repoPath := createTestRepo(t)

	mgr := repo.NewManagerWithPath(repoPath)

	// Test expanding and verifying we get correct resource types back
	matches, err := ExpandPattern(mgr, "skill/pdf*")
	if err != nil {
		t.Fatalf("ExpandPattern failed: %v", err)
	}

	// Verify each match can be parsed correctly
	for _, match := range matches {
		resourceType, name, err := ParseResourceArg(match)
		if err != nil {
			t.Errorf("ParseResourceArg(%q) failed: %v", match, err)
		}
		if resourceType != resource.Skill {
			t.Errorf("ParseResourceArg(%q) type = %v, want %v", match, resourceType, resource.Skill)
		}
		if !strings.HasPrefix(name, "pdf") {
			t.Errorf("ParseResourceArg(%q) name = %q, doesn't start with 'pdf'", match, name)
		}
	}
}

func TestInstallPattern_MultipleTypes(t *testing.T) {
	repoPath := createTestRepo(t)

	mgr := repo.NewManagerWithPath(repoPath)

	// Test pattern that matches multiple resource types
	matches, err := ExpandPattern(mgr, "*review*")
	if err != nil {
		t.Fatalf("ExpandPattern failed: %v", err)
	}

	// Should match: review-command, code-reviewer (agent)
	expectedCount := 2
	if len(matches) != expectedCount {
		t.Errorf("ExpandPattern(*review*) returned %d matches, want %d", len(matches), expectedCount)
		for _, m := range matches {
			t.Logf("  match: %s", m)
		}
	}

	// Verify we got different types
	types := make(map[resource.ResourceType]bool)
	for _, match := range matches {
		resourceType, _, err := ParseResourceArg(match)
		if err != nil {
			t.Errorf("ParseResourceArg(%q) failed: %v", match, err)
			continue
		}
		types[resourceType] = true
	}

	if len(types) < 2 {
		t.Errorf("ExpandPattern(*review*) matched %d types, want at least 2", len(types))
	}
}

func TestInstallPattern_EdgeCases(t *testing.T) {
	repoPath := createTestRepo(t)

	mgr := repo.NewManagerWithPath(repoPath)

	tests := []struct {
		name          string
		pattern       string
		expectError   bool
		expectedCount int
	}{
		{
			name:          "wildcard all skills",
			pattern:       "skill/*",
			expectError:   false,
			expectedCount: 4, // all 4 test skills
		},
		{
			name:          "wildcard all",
			pattern:       "*",
			expectError:   false,
			expectedCount: 10, // all resources (3 commands + 4 skills + 3 agents)
		},
		{
			name:          "question mark single char",
			pattern:       "command/????-command", // matches test-command (4 chars before dash)
			expectError:   false,
			expectedCount: 1,
		},
		{
			name:          "complex alternatives",
			pattern:       "skill/{pdf,test}*",
			expectError:   false,
			expectedCount: 3, // pdf-processing, pdf-extraction, test-skill
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := ExpandPattern(mgr, tt.pattern)

			if tt.expectError {
				if err == nil {
					t.Errorf("ExpandPattern(%q) expected error, got nil", tt.pattern)
				}
				return
			}

			if err != nil {
				t.Errorf("ExpandPattern(%q) unexpected error: %v", tt.pattern, err)
				return
			}

			if len(matches) != tt.expectedCount {
				t.Errorf("ExpandPattern(%q) returned %d matches, want %d", tt.pattern, len(matches), tt.expectedCount)
				for _, m := range matches {
					t.Logf("  match: %s", m)
				}
			}
		})
	}
}

func TestInstallPattern_WithFlags(t *testing.T) {
	// Test that parseTargetFlag works correctly with pattern installs
	tests := []struct {
		name       string
		targetFlag string
		wantTools  []tools.Tool
		wantErr    bool
	}{
		{
			name:       "single target",
			targetFlag: "claude",
			wantTools:  []tools.Tool{tools.Claude},
			wantErr:    false,
		},
		{
			name:       "multiple targets",
			targetFlag: "claude,opencode",
			wantTools:  []tools.Tool{tools.Claude, tools.OpenCode},
			wantErr:    false,
		},
		{
			name:       "invalid target",
			targetFlag: "invalid",
			wantTools:  nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTargetFlag(tt.targetFlag)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseTargetFlag(%q) expected error, got nil", tt.targetFlag)
				}
				return
			}

			if err != nil {
				t.Errorf("parseTargetFlag(%q) unexpected error: %v", tt.targetFlag, err)
				return
			}

			if len(got) != len(tt.wantTools) {
				t.Errorf("parseTargetFlag(%q) returned %d tools, want %d", tt.targetFlag, len(got), len(tt.wantTools))
			}
		})
	}
}

func TestExpandPattern_SpecialCharacters(t *testing.T) {
	repoPath := createTestRepo(t)

	mgr := repo.NewManagerWithPath(repoPath)

	tests := []struct {
		name          string
		pattern       string
		shouldMatch   []string
		shouldntMatch []string
	}{
		{
			name:        "hyphen in pattern",
			pattern:     "skill/test-*",
			shouldMatch: []string{"skill/test-skill"},
		},
		{
			name:        "multiple hyphens",
			pattern:     "skill/*-processing",
			shouldMatch: []string{"skill/pdf-processing", "skill/image-processing"},
		},
		{
			name:          "middle wildcard",
			pattern:       "skill/*-*",
			shouldMatch:   []string{"skill/pdf-processing", "skill/pdf-extraction", "skill/image-processing", "skill/test-skill"},
			shouldntMatch: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := ExpandPattern(mgr, tt.pattern)
			if err != nil {
				t.Fatalf("ExpandPattern(%q) failed: %v", tt.pattern, err)
			}

			matchMap := make(map[string]bool)
			for _, m := range matches {
				matchMap[m] = true
			}

			// Check expected matches
			for _, expected := range tt.shouldMatch {
				if !matchMap[expected] {
					t.Errorf("ExpandPattern(%q) missing expected match %q", tt.pattern, expected)
				}
			}

			// Check unexpected matches
			for _, unexpected := range tt.shouldntMatch {
				if matchMap[unexpected] {
					t.Errorf("ExpandPattern(%q) unexpectedly matched %q", tt.pattern, unexpected)
				}
			}
		})
	}
}

// Test helpers for package tests

// createTestPackage creates a test package in the repo
func createTestPackage(t *testing.T, repoPath, packageName string, resources []string) {
	t.Helper()

	// Create packages directory
	packagesDir := filepath.Join(repoPath, "packages", packageName)
	if err := os.MkdirAll(packagesDir, 0755); err != nil {
		t.Fatalf("failed to create packages dir: %v", err)
	}

	// Create package.yaml content
	content := fmt.Sprintf("name: %s\ndescription: Test package\nresources:\n", packageName)
	for _, res := range resources {
		content += fmt.Sprintf("  - %s\n", res)
	}

	// Write package.yaml
	pkgFile := filepath.Join(packagesDir, "package.yaml")
	if err := os.WriteFile(pkgFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create package.yaml: %v", err)
	}
}

// TestInstallMultipleSkills tests installing multiple skills
func TestInstallMultipleSkills(t *testing.T) {
	repoPath := createTestRepo(t)

	mgr := repo.NewManagerWithPath(repoPath)

	// Verify skills exist in repo
	skills := []string{"pdf-processing", "pdf-extraction"}
	for _, skillName := range skills {
		_, err := mgr.Get(skillName, resource.Skill)
		if err != nil {
			t.Fatalf("skill %s not found in test repo: %v", skillName, err)
		}
	}
}

// TestInstallSkillAndPackage tests installing a skill and a package together
func TestInstallSkillAndPackage(t *testing.T) {
	repoPath := createTestRepo(t)

	// Create a test package with some skills
	createTestPackage(t, repoPath, "test-pkg", []string{"skill/test-skill", "command/test-command"})

	mgr := repo.NewManagerWithPath(repoPath)

	// Verify skill exists
	_, err := mgr.Get("pdf-processing", resource.Skill)
	if err != nil {
		t.Fatalf("skill not found in test repo: %v", err)
	}

	// Verify package resources exist
	_, err = mgr.Get("test-skill", resource.Skill)
	if err != nil {
		t.Fatalf("skill test-skill not found in test repo: %v", err)
	}
	_, err = mgr.Get("test-command", resource.Command)
	if err != nil {
		t.Fatalf("command test-command not found in test repo: %v", err)
	}
}

// TestInstallMultiplePackages tests installing multiple packages
func TestInstallMultiplePackages(t *testing.T) {
	repoPath := createTestRepo(t)

	// Create test packages
	createTestPackage(t, repoPath, "pkg-a", []string{"skill/pdf-processing"})
	createTestPackage(t, repoPath, "pkg-b", []string{"skill/test-skill"})

	mgr := repo.NewManagerWithPath(repoPath)

	// Verify both package resources exist
	_, err := mgr.Get("pdf-processing", resource.Skill)
	if err != nil {
		t.Fatalf("skill pdf-processing not found: %v", err)
	}
	_, err = mgr.Get("test-skill", resource.Skill)
	if err != nil {
		t.Fatalf("skill test-skill not found: %v", err)
	}
}

// TestInstallArgProcessing tests that arguments are correctly separated into packages and resources
func TestInstallArgProcessing(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantPackages []string
		wantOthers   []string
	}{
		{
			name:         "only skills",
			args:         []string{"skill/pdf-processing", "skill/test-skill"},
			wantPackages: nil,
			wantOthers:   []string{"skill/pdf-processing", "skill/test-skill"},
		},
		{
			name:         "only packages",
			args:         []string{"package/pkg-a", "package/pkg-b"},
			wantPackages: []string{"package/pkg-a", "package/pkg-b"},
			wantOthers:   nil,
		},
		{
			name:         "mixed skills and packages",
			args:         []string{"skill/pdf-processing", "package/pkg-a", "command/test"},
			wantPackages: []string{"package/pkg-a"},
			wantOthers:   []string{"skill/pdf-processing", "command/test"},
		},
		{
			name:         "packages prefix normalization",
			args:         []string{"packages/pkg-a", "package/pkg-b"},
			wantPackages: []string{"package/pkg-a", "package/pkg-b"},
			wantOthers:   nil,
		},
		{
			name:         "skill and package together",
			args:         []string{"skill/pdf-processing", "package/test-pkg"},
			wantPackages: []string{"package/test-pkg"},
			wantOthers:   []string{"skill/pdf-processing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var packageRefs []string
			var resourceRefs []string

			// Simulate the arg processing logic from install.go RunE
			for _, arg := range tt.args {
				if strings.HasPrefix(arg, "package/") || strings.HasPrefix(arg, "packages/") {
					normalizedArg := arg
					if strings.HasPrefix(arg, "packages/") {
						normalizedArg = "package/" + strings.TrimPrefix(arg, "packages/")
					}
					packageRefs = append(packageRefs, normalizedArg)
				} else {
					resourceRefs = append(resourceRefs, arg)
				}
			}

			// Verify packages
			if len(packageRefs) != len(tt.wantPackages) {
				t.Errorf("got %d packages, want %d", len(packageRefs), len(tt.wantPackages))
			}
			for i, pkg := range tt.wantPackages {
				if i >= len(packageRefs) || packageRefs[i] != pkg {
					t.Errorf("package[%d] = %v, want %v", i, packageRefs, tt.wantPackages)
					break
				}
			}

			// Verify other resources
			if len(resourceRefs) != len(tt.wantOthers) {
				t.Errorf("got %d resources, want %d", len(resourceRefs), len(tt.wantOthers))
			}
			for i, res := range tt.wantOthers {
				if i >= len(resourceRefs) || resourceRefs[i] != res {
					t.Errorf("resource[%d] = %v, want %v", i, resourceRefs, tt.wantOthers)
					break
				}
			}
		})
	}
}
