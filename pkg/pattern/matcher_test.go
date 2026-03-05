package pattern

import (
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestIsPattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "asterisk wildcard",
			input: "test*",
			want:  true,
		},
		{
			name:  "question mark wildcard",
			input: "test?",
			want:  true,
		},
		{
			name:  "character class",
			input: "test[abc]",
			want:  true,
		},
		{
			name:  "alternatives",
			input: "test{a,b}",
			want:  true,
		},
		{
			name:  "no pattern characters",
			input: "test-command",
			want:  false,
		},
		{
			name:  "hyphen only",
			input: "test-cmd",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPattern(tt.input)
			if got != tt.want {
				t.Errorf("IsPattern(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParsePattern(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantType      resource.ResourceType
		wantPattern   string
		wantIsPattern bool
	}{
		{
			name:          "bare pattern with asterisk",
			input:         "test*",
			wantType:      "",
			wantPattern:   "test*",
			wantIsPattern: true,
		},
		{
			name:          "bare exact name",
			input:         "test-command",
			wantType:      "",
			wantPattern:   "test-command",
			wantIsPattern: false,
		},
		{
			name:          "skill with pattern",
			input:         "skill/pdf*",
			wantType:      resource.Skill,
			wantPattern:   "pdf*",
			wantIsPattern: true,
		},
		{
			name:          "command with pattern",
			input:         "command/test*",
			wantType:      resource.Command,
			wantPattern:   "test*",
			wantIsPattern: true,
		},
		{
			name:          "agent with pattern",
			input:         "agent/*reviewer",
			wantType:      resource.Agent,
			wantPattern:   "*reviewer",
			wantIsPattern: true,
		},
		{
			name:          "skill with exact name",
			input:         "skill/pdf-processing",
			wantType:      resource.Skill,
			wantPattern:   "pdf-processing",
			wantIsPattern: false,
		},
		{
			name:          "invalid type treated as pattern",
			input:         "invalid/test*",
			wantType:      "",
			wantPattern:   "invalid/test*",
			wantIsPattern: true,
		},
		{
			name:          "slash in pattern without type",
			input:         "foo/bar",
			wantType:      "",
			wantPattern:   "foo/bar",
			wantIsPattern: false,
		},
		{
			name:          "wildcard with multiple segments",
			input:         "skill/test-*-cmd",
			wantType:      resource.Skill,
			wantPattern:   "test-*-cmd",
			wantIsPattern: true,
		},
		{
			name:          "question mark pattern",
			input:         "cmd?",
			wantType:      "",
			wantPattern:   "cmd?",
			wantIsPattern: true,
		},
		{
			name:          "package with pattern",
			input:         "package/web-*",
			wantType:      resource.PackageType,
			wantPattern:   "web-*",
			wantIsPattern: true,
		},
		{
			name:          "package with exact name",
			input:         "package/web-tools",
			wantType:      resource.PackageType,
			wantPattern:   "web-tools",
			wantIsPattern: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotPattern, gotIsPattern := ParsePattern(tt.input)
			if gotType != tt.wantType {
				t.Errorf("ParsePattern(%q) type = %q, want %q", tt.input, gotType, tt.wantType)
			}
			if gotPattern != tt.wantPattern {
				t.Errorf("ParsePattern(%q) pattern = %q, want %q", tt.input, gotPattern, tt.wantPattern)
			}
			if gotIsPattern != tt.wantIsPattern {
				t.Errorf("ParsePattern(%q) isPattern = %v, want %v", tt.input, gotIsPattern, tt.wantIsPattern)
			}
		})
	}
}

func TestNewMatcher(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		wantError bool
	}{
		{
			name:      "valid simple pattern",
			pattern:   "test*",
			wantError: false,
		},
		{
			name:      "valid type/pattern",
			pattern:   "skill/pdf*",
			wantError: false,
		},
		{
			name:      "valid exact name",
			pattern:   "test-command",
			wantError: false,
		},
		{
			name:      "empty pattern",
			pattern:   "",
			wantError: true,
		},
		{
			name:      "valid alternatives",
			pattern:   "{test,prod}*",
			wantError: false,
		},
		{
			name:      "valid character class",
			pattern:   "test[0-9]",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.pattern)
			if tt.wantError {
				if err == nil {
					t.Errorf("NewMatcher(%q) expected error, got nil", tt.pattern)
				}
			} else {
				if err != nil {
					t.Errorf("NewMatcher(%q) unexpected error: %v", tt.pattern, err)
				}
				if m == nil {
					t.Errorf("NewMatcher(%q) returned nil matcher", tt.pattern)
				}
			}
		})
	}
}

func TestMatcher_Match(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		resource *resource.Resource
		want     bool
	}{
		{
			name:    "exact match",
			pattern: "test-command",
			resource: &resource.Resource{
				Name: "test-command",
				Type: resource.Command,
			},
			want: true,
		},
		{
			name:    "prefix wildcard match",
			pattern: "test*",
			resource: &resource.Resource{
				Name: "test-command",
				Type: resource.Command,
			},
			want: true,
		},
		{
			name:    "suffix wildcard match",
			pattern: "*command",
			resource: &resource.Resource{
				Name: "test-command",
				Type: resource.Command,
			},
			want: true,
		},
		{
			name:    "middle wildcard match",
			pattern: "test-*-end",
			resource: &resource.Resource{
				Name: "test-middle-end",
				Type: resource.Command,
			},
			want: true,
		},
		{
			name:    "no match",
			pattern: "test*",
			resource: &resource.Resource{
				Name: "other-command",
				Type: resource.Command,
			},
			want: false,
		},
		{
			name:    "type filter match",
			pattern: "skill/pdf*",
			resource: &resource.Resource{
				Name: "pdf-processing",
				Type: resource.Skill,
			},
			want: true,
		},
		{
			name:    "type filter no match - wrong type",
			pattern: "skill/pdf*",
			resource: &resource.Resource{
				Name: "pdf-processing",
				Type: resource.Command,
			},
			want: false,
		},
		{
			name:    "type filter no match - wrong name",
			pattern: "skill/pdf*",
			resource: &resource.Resource{
				Name: "image-processing",
				Type: resource.Skill,
			},
			want: false,
		},
		{
			name:    "wildcard all",
			pattern: "*",
			resource: &resource.Resource{
				Name: "any-name",
				Type: resource.Command,
			},
			want: true,
		},
		{
			name:    "question mark single char",
			pattern: "cmd?",
			resource: &resource.Resource{
				Name: "cmd1",
				Type: resource.Command,
			},
			want: true,
		},
		{
			name:    "question mark no match",
			pattern: "cmd?",
			resource: &resource.Resource{
				Name: "cmd12",
				Type: resource.Command,
			},
			want: false,
		},
		{
			name:    "agent type filter",
			pattern: "agent/*reviewer",
			resource: &resource.Resource{
				Name: "code-reviewer",
				Type: resource.Agent,
			},
			want: true,
		},
		{
			name:    "package type filter match",
			pattern: "package/web-*",
			resource: &resource.Resource{
				Name: "web-tools",
				Type: resource.PackageType,
			},
			want: true,
		},
		{
			name:    "package type filter no match - wrong type",
			pattern: "package/web-*",
			resource: &resource.Resource{
				Name: "web-tools",
				Type: resource.Command,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.pattern)
			if err != nil {
				t.Fatalf("NewMatcher(%q) error: %v", tt.pattern, err)
			}

			got := m.Match(tt.resource)
			if got != tt.want {
				t.Errorf("Matcher.Match(%q) = %v, want %v", tt.resource.Name, got, tt.want)
			}
		})
	}
}

func TestMatcher_MatchName(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		input   string
		want    bool
	}{
		{
			name:    "exact match",
			pattern: "test",
			input:   "test",
			want:    true,
		},
		{
			name:    "prefix match",
			pattern: "test*",
			input:   "test-command",
			want:    true,
		},
		{
			name:    "no match",
			pattern: "test*",
			input:   "other",
			want:    false,
		},
		{
			name:    "type prefix ignored in MatchName",
			pattern: "skill/test*",
			input:   "test-command",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.pattern)
			if err != nil {
				t.Fatalf("NewMatcher(%q) error: %v", tt.pattern, err)
			}

			got := m.MatchName(tt.input)
			if got != tt.want {
				t.Errorf("Matcher.MatchName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatcher_GetResourceType(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		wantType resource.ResourceType
	}{
		{
			name:     "no type filter",
			pattern:  "test*",
			wantType: "",
		},
		{
			name:     "skill type filter",
			pattern:  "skill/test*",
			wantType: resource.Skill,
		},
		{
			name:     "command type filter",
			pattern:  "command/test*",
			wantType: resource.Command,
		},
		{
			name:     "agent type filter",
			pattern:  "agent/test*",
			wantType: resource.Agent,
		},
		{
			name:     "package type filter",
			pattern:  "package/test*",
			wantType: resource.PackageType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.pattern)
			if err != nil {
				t.Fatalf("NewMatcher(%q) error: %v", tt.pattern, err)
			}

			got := m.GetResourceType()
			if got != tt.wantType {
				t.Errorf("Matcher.GetResourceType() = %q, want %q", got, tt.wantType)
			}
		})
	}
}

func TestMatcher_IsPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		{
			name:    "pattern with wildcard",
			pattern: "test*",
			want:    true,
		},
		{
			name:    "exact name",
			pattern: "test-command",
			want:    false,
		},
		{
			name:    "type with pattern",
			pattern: "skill/test*",
			want:    true,
		},
		{
			name:    "type with exact",
			pattern: "skill/test-skill",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.pattern)
			if err != nil {
				t.Fatalf("NewMatcher(%q) error: %v", tt.pattern, err)
			}

			got := m.IsPattern()
			if got != tt.want {
				t.Errorf("Matcher.IsPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		input     string
		want      bool
		wantError bool
	}{
		{
			name:      "exact match",
			pattern:   "test",
			input:     "test",
			want:      true,
			wantError: false,
		},
		{
			name:      "wildcard match",
			pattern:   "test*",
			input:     "test-command",
			want:      true,
			wantError: false,
		},
		{
			name:      "no match",
			pattern:   "test*",
			input:     "other",
			want:      false,
			wantError: false,
		},
		{
			name:      "empty pattern",
			pattern:   "",
			input:     "test",
			want:      false,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MatchesPattern(tt.pattern, tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("MatchesPattern(%q, %q) expected error, got nil", tt.pattern, tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("MatchesPattern(%q, %q) unexpected error: %v", tt.pattern, tt.input, err)
				}
				if got != tt.want {
					t.Errorf("MatchesPattern(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
				}
			}
		})
	}
}

func TestFilterResources(t *testing.T) {
	resources := []*resource.Resource{
		{Name: "test-command", Type: resource.Command},
		{Name: "test-skill", Type: resource.Skill},
		{Name: "pdf-processing", Type: resource.Skill},
		{Name: "image-processing", Type: resource.Skill},
		{Name: "code-reviewer", Type: resource.Agent},
		{Name: "other-command", Type: resource.Command},
	}

	tests := []struct {
		name      string
		pattern   string
		wantCount int
		wantNames []string
		wantError bool
	}{
		{
			name:      "match all with wildcard",
			pattern:   "*",
			wantCount: 6,
			wantNames: []string{"test-command", "test-skill", "pdf-processing", "image-processing", "code-reviewer", "other-command"},
			wantError: false,
		},
		{
			name:      "match prefix",
			pattern:   "test*",
			wantCount: 2,
			wantNames: []string{"test-command", "test-skill"},
			wantError: false,
		},
		{
			name:      "match suffix",
			pattern:   "*processing",
			wantCount: 2,
			wantNames: []string{"pdf-processing", "image-processing"},
			wantError: false,
		},
		{
			name:      "match with type filter",
			pattern:   "skill/*processing",
			wantCount: 2,
			wantNames: []string{"pdf-processing", "image-processing"},
			wantError: false,
		},
		{
			name:      "match exact with type",
			pattern:   "skill/test-skill",
			wantCount: 1,
			wantNames: []string{"test-skill"},
			wantError: false,
		},
		{
			name:      "no matches",
			pattern:   "nonexistent*",
			wantCount: 0,
			wantNames: []string{},
			wantError: false,
		},
		{
			name:      "agent type filter",
			pattern:   "agent/*",
			wantCount: 1,
			wantNames: []string{"code-reviewer"},
			wantError: false,
		},
		{
			name:      "empty pattern",
			pattern:   "",
			wantCount: 0,
			wantNames: nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterResources(resources, tt.pattern)
			if tt.wantError {
				if err == nil {
					t.Errorf("FilterResources(%q) expected error, got nil", tt.pattern)
				}
				return
			}
			if err != nil {
				t.Fatalf("FilterResources(%q) unexpected error: %v", tt.pattern, err)
			}

			if len(got) != tt.wantCount {
				t.Errorf("FilterResources(%q) returned %d resources, want %d", tt.pattern, len(got), tt.wantCount)
			}

			// Check that all expected names are present
			gotNames := make(map[string]bool)
			for _, res := range got {
				gotNames[res.Name] = true
			}

			for _, wantName := range tt.wantNames {
				if !gotNames[wantName] {
					t.Errorf("FilterResources(%q) missing expected resource %q", tt.pattern, wantName)
				}
			}
		})
	}
}

func TestMatcher_ComplexPatterns(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		testName string
		want     bool
	}{
		{
			name:     "alternatives - first option",
			pattern:  "{test,prod}-*",
			testName: "test-command",
			want:     true,
		},
		{
			name:     "alternatives - second option",
			pattern:  "{test,prod}-*",
			testName: "prod-command",
			want:     true,
		},
		{
			name:     "alternatives - no match",
			pattern:  "{test,prod}-*",
			testName: "dev-command",
			want:     false,
		},
		{
			name:     "character class - match",
			pattern:  "cmd[0-9]",
			testName: "cmd5",
			want:     true,
		},
		{
			name:     "character class - no match",
			pattern:  "cmd[0-9]",
			testName: "cmda",
			want:     false,
		},
		{
			name:     "multiple wildcards",
			pattern:  "*-test-*",
			testName: "foo-test-bar",
			want:     true,
		},
		{
			name:     "complex pattern",
			pattern:  "test-{cmd,skill}-*[0-9]",
			testName: "test-cmd-foo5",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(tt.pattern)
			if err != nil {
				t.Fatalf("NewMatcher(%q) error: %v", tt.pattern, err)
			}

			got := m.MatchName(tt.testName)
			if got != tt.want {
				t.Errorf("Matcher.MatchName(%q) = %v, want %v", tt.testName, got, tt.want)
			}
		})
	}
}
