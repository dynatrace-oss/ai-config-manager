package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// Test ParseResourceArg

func TestParseResourceArg_ValidFormats(t *testing.T) {
	tests := []struct {
		name        string
		arg         string
		wantType    resource.ResourceType
		wantName    string
		wantErr     bool
		errContains string
	}{
		// Valid formats - singular
		{
			name:     "skill singular",
			arg:      "skill/pdf-processing",
			wantType: resource.Skill,
			wantName: "pdf-processing",
			wantErr:  false,
		},
		{
			name:     "command singular",
			arg:      "command/test-command",
			wantType: resource.Command,
			wantName: "test-command",
			wantErr:  false,
		},
		{
			name:     "agent singular",
			arg:      "agent/code-reviewer",
			wantType: resource.Agent,
			wantName: "code-reviewer",
			wantErr:  false,
		},
		// Valid formats - plural
		{
			name:     "skills plural",
			arg:      "skills/pdf-processing",
			wantType: resource.Skill,
			wantName: "pdf-processing",
			wantErr:  false,
		},
		{
			name:     "commands plural",
			arg:      "commands/test-command",
			wantType: resource.Command,
			wantName: "test-command",
			wantErr:  false,
		},
		{
			name:     "agents plural",
			arg:      "agents/code-reviewer",
			wantType: resource.Agent,
			wantName: "code-reviewer",
			wantErr:  false,
		},
		// Valid formats - uppercase/mixed case
		{
			name:     "uppercase type",
			arg:      "SKILL/test",
			wantType: resource.Skill,
			wantName: "test",
			wantErr:  false,
		},
		{
			name:     "mixed case type",
			arg:      "Command/test",
			wantType: resource.Command,
			wantName: "test",
			wantErr:  false,
		},
		// Valid formats - with spaces (trimmed)
		{
			name:     "spaces around type",
			arg:      " skill /test",
			wantType: resource.Skill,
			wantName: "test",
			wantErr:  false,
		},
		{
			name:     "spaces around name",
			arg:      "skill/ test ",
			wantType: resource.Skill,
			wantName: "test",
			wantErr:  false,
		},
		// Invalid formats
		{
			name:        "missing slash",
			arg:         "skill-name",
			wantErr:     true,
			errContains: "invalid format",
		},
		{
			name:        "empty name",
			arg:         "skill/",
			wantErr:     true,
			errContains: "name cannot be empty",
		},
		{
			name:        "empty name with spaces",
			arg:         "skill/   ",
			wantErr:     true,
			errContains: "name cannot be empty",
		},
		{
			name:        "invalid type",
			arg:         "invalid/test",
			wantErr:     true,
			errContains: "invalid resource type",
		},
		{
			name:        "empty type",
			arg:         "/test",
			wantErr:     true,
			errContains: "invalid resource type",
		},
		{
			// hierarchical names (e.g. api/deploy) are intentionally supported via SplitN(..., 2)
			name:     "too many slashes",
			arg:      "skill/sub/name",
			wantType: resource.Skill,
			wantName: "sub/name",
			wantErr:  false,
		},
		{
			name:        "just slash",
			arg:         "/",
			wantErr:     true,
			errContains: "name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotName, err := ParseResourceArg(tt.arg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseResourceArg(%q) expected error, got nil", tt.arg)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ParseResourceArg(%q) error = %v, want error containing %q", tt.arg, err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseResourceArg(%q) unexpected error: %v", tt.arg, err)
				return
			}

			if gotType != tt.wantType {
				t.Errorf("ParseResourceArg(%q) type = %v, want %v", tt.arg, gotType, tt.wantType)
			}
			if gotName != tt.wantName {
				t.Errorf("ParseResourceArg(%q) name = %q, want %q", tt.arg, gotName, tt.wantName)
			}
		})
	}
}

// Test FormatResourceArg

func TestFormatResourceArg(t *testing.T) {
	tests := []struct {
		name string
		res  *resource.Resource
		want string
	}{
		{
			name: "skill resource",
			res: &resource.Resource{
				Type: resource.Skill,
				Name: "pdf-processing",
			},
			want: "skill/pdf-processing",
		},
		{
			name: "command resource",
			res: &resource.Resource{
				Type: resource.Command,
				Name: "test-command",
			},
			want: "command/test-command",
		},
		{
			name: "agent resource",
			res: &resource.Resource{
				Type: resource.Agent,
				Name: "code-reviewer",
			},
			want: "agent/code-reviewer",
		},
		{
			name: "resource with hyphens",
			res: &resource.Resource{
				Type: resource.Skill,
				Name: "multi-word-skill-name",
			},
			want: "skill/multi-word-skill-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatResourceArg(tt.res)
			if got != tt.want {
				t.Errorf("FormatResourceArg() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Test parseResourceType

func TestParseResourceType(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        resource.ResourceType
		wantErr     bool
		errContains string
	}{
		// Valid types - singular
		{name: "skill", input: "skill", want: resource.Skill, wantErr: false},
		{name: "command", input: "command", want: resource.Command, wantErr: false},
		{name: "agent", input: "agent", want: resource.Agent, wantErr: false},
		// Valid types - plural
		{name: "skills", input: "skills", want: resource.Skill, wantErr: false},
		{name: "commands", input: "commands", want: resource.Command, wantErr: false},
		{name: "agents", input: "agents", want: resource.Agent, wantErr: false},
		// Valid types - mixed case
		{name: "SKILL uppercase", input: "SKILL", want: resource.Skill, wantErr: false},
		{name: "Command mixed", input: "Command", want: resource.Command, wantErr: false},
		{name: "AGENTS uppercase plural", input: "AGENTS", want: resource.Agent, wantErr: false},
		// Invalid types
		{
			name:        "invalid type",
			input:       "invalid",
			wantErr:     true,
			errContains: "invalid resource type",
		},
		{
			name:        "empty string",
			input:       "",
			wantErr:     true,
			errContains: "invalid resource type",
		},
		{
			name:        "typo",
			input:       "skil",
			wantErr:     true,
			errContains: "invalid resource type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseResourceType(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseResourceType(%q) expected error, got nil", tt.input)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("parseResourceType(%q) error = %v, want error containing %q", tt.input, err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("parseResourceType(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got != tt.want {
				t.Errorf("parseResourceType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// Test ExpandPattern

func TestExpandPattern_ExactMatch(t *testing.T) {
	repoPath, cleanup := createTestRepoForPatternUtils(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "exact skill name",
			pattern:  "skill/pdf-processing",
			expected: []string{"skill/pdf-processing"},
		},
		{
			name:     "exact command name",
			pattern:  "command/test-command",
			expected: []string{"command/test-command"},
		},
		{
			name:     "exact agent name",
			pattern:  "agent/code-reviewer",
			expected: []string{"agent/code-reviewer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := ExpandPattern(mgr, tt.pattern)
			if err != nil {
				t.Fatalf("ExpandPattern(%q) failed: %v", tt.pattern, err)
			}

			if len(matches) != len(tt.expected) {
				t.Errorf("ExpandPattern(%q) returned %d matches, want %d", tt.pattern, len(matches), len(tt.expected))
			}

			for i, expected := range tt.expected {
				if i >= len(matches) {
					break
				}
				if matches[i] != expected {
					t.Errorf("ExpandPattern(%q)[%d] = %q, want %q", tt.pattern, i, matches[i], expected)
				}
			}
		})
	}
}

func TestExpandPattern_Wildcards(t *testing.T) {
	repoPath, cleanup := createTestRepoForPatternUtils(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	tests := []struct {
		name          string
		pattern       string
		expectedCount int
		shouldContain []string
	}{
		{
			name:          "all skills",
			pattern:       "skill/*",
			expectedCount: 4,
			shouldContain: []string{"skill/pdf-processing", "skill/pdf-extraction", "skill/image-processing", "skill/test-skill"},
		},
		{
			name:          "skill prefix match",
			pattern:       "skill/pdf*",
			expectedCount: 2,
			shouldContain: []string{"skill/pdf-processing", "skill/pdf-extraction"},
		},
		{
			name:          "all commands",
			pattern:       "command/*",
			expectedCount: 3,
			shouldContain: []string{"command/test-command", "command/pdf-command", "command/review-command"},
		},
		{
			name:          "all agents",
			pattern:       "agent/*",
			expectedCount: 3,
			shouldContain: []string{"agent/code-reviewer", "agent/test-agent", "agent/doc-generator"},
		},
		{
			name:          "cross-type pattern",
			pattern:       "*test*",
			expectedCount: 3,
			shouldContain: []string{"command/test-command", "skill/test-skill", "agent/test-agent"},
		},
		{
			name:          "all resources",
			pattern:       "*",
			expectedCount: 10, // 3 commands + 4 skills + 3 agents
			shouldContain: []string{"command/test-command", "skill/pdf-processing", "agent/code-reviewer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := ExpandPattern(mgr, tt.pattern)
			if err != nil {
				t.Fatalf("ExpandPattern(%q) failed: %v", tt.pattern, err)
			}

			if len(matches) != tt.expectedCount {
				t.Errorf("ExpandPattern(%q) returned %d matches, want %d", tt.pattern, len(matches), tt.expectedCount)
				t.Logf("Matches: %v", matches)
			}

			// Check that expected items are present
			matchMap := make(map[string]bool)
			for _, m := range matches {
				matchMap[m] = true
			}

			for _, expected := range tt.shouldContain {
				if !matchMap[expected] {
					t.Errorf("ExpandPattern(%q) missing expected match %q", tt.pattern, expected)
				}
			}
		})
	}
}

func TestExpandPattern_NoMatches(t *testing.T) {
	repoPath, cleanup := createTestRepoForPatternUtils(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	tests := []struct {
		name    string
		pattern string
	}{
		{name: "no matching skills", pattern: "skill/nonexistent*"},
		{name: "no matching commands", pattern: "command/xyz*"},
		{name: "no matching agents", pattern: "agent/missing*"},
		{name: "no cross-type matches", pattern: "*zzz*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := ExpandPattern(mgr, tt.pattern)
			if err != nil {
				t.Fatalf("ExpandPattern(%q) failed: %v", tt.pattern, err)
			}

			if len(matches) != 0 {
				t.Errorf("ExpandPattern(%q) returned %d matches, want 0", tt.pattern, len(matches))
				t.Logf("Unexpected matches: %v", matches)
			}
		})
	}
}

func TestExpandPattern_InvalidPatternSyntax(t *testing.T) {
	repoPath, cleanup := createTestRepoForPatternUtils(t)
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
			name:        "invalid glob syntax",
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

			if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("ExpandPattern(%q) error = %v, want error containing %q", tt.pattern, err, tt.errContains)
			}
		})
	}
}

func TestExpandPattern_AdvancedGlobPatterns(t *testing.T) {
	repoPath, cleanup := createTestRepoForPatternUtils(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	tests := []struct {
		name          string
		pattern       string
		expectedCount int
		shouldContain []string
	}{
		{
			name:          "question mark pattern",
			pattern:       "command/????-command",
			expectedCount: 1,
			shouldContain: []string{"command/test-command"},
		},
		{
			name:          "alternatives pattern",
			pattern:       "skill/{pdf,test}*",
			expectedCount: 3,
			shouldContain: []string{"skill/pdf-processing", "skill/pdf-extraction", "skill/test-skill"},
		},
		{
			name:          "suffix pattern",
			pattern:       "skill/*-processing",
			expectedCount: 2,
			shouldContain: []string{"skill/pdf-processing", "skill/image-processing"},
		},
		{
			name:          "middle wildcard",
			pattern:       "skill/*-*",
			expectedCount: 4,
			shouldContain: []string{"skill/pdf-processing", "skill/pdf-extraction", "skill/image-processing", "skill/test-skill"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := ExpandPattern(mgr, tt.pattern)
			if err != nil {
				t.Fatalf("ExpandPattern(%q) failed: %v", tt.pattern, err)
			}

			if len(matches) != tt.expectedCount {
				t.Errorf("ExpandPattern(%q) returned %d matches, want %d", tt.pattern, len(matches), tt.expectedCount)
				t.Logf("Matches: %v", matches)
			}

			matchMap := make(map[string]bool)
			for _, m := range matches {
				matchMap[m] = true
			}

			for _, expected := range tt.shouldContain {
				if !matchMap[expected] {
					t.Errorf("ExpandPattern(%q) missing expected match %q", tt.pattern, expected)
				}
			}
		})
	}
}

func TestExpandPattern_EdgeCases(t *testing.T) {
	repoPath, cleanup := createTestRepoForPatternUtils(t)
	defer cleanup()

	mgr := repo.NewManagerWithPath(repoPath)

	tests := []struct {
		name          string
		pattern       string
		expectError   bool
		expectedCount int
	}{
		{
			name:          "empty name part returns as-is",
			pattern:       "skill/",
			expectError:   false,
			expectedCount: 1, // Returns as-is (will fail validation later)
		},
		{
			name:          "just type with slash",
			pattern:       "skill/",
			expectError:   false,
			expectedCount: 1,
		},
		{
			name:          "double star",
			pattern:       "skill/**",
			expectError:   false,
			expectedCount: 4, // Matches all skills
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
				t.Logf("Matches: %v", matches)
			}
		})
	}
}

// Test helper to create test repo

func createTestRepoForPatternUtils(t *testing.T) (repoPath string, cleanup func()) {
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

	cleanup = func() {
		// Cleanup is automatic with t.TempDir()
	}

	return repoPath, cleanup
}
