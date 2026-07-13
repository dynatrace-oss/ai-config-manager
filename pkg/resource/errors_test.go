package resource

import (
	"errors"
	"strings"
	"testing"
)

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ValidationError
		contains []string
	}{
		{
			name: "full error with all fields",
			err: &ValidationError{
				FilePath:     "/path/to/skill/SKILL.md",
				ResourceName: "my-skill",
				ResourceType: "skill",
				FieldName:    "description",
				Err:          errors.New("missing required field"),
				Suggestion:   "Add a description field",
			},
			contains: []string{
				"skill 'my-skill'",
				"field 'description'",
				"/path/to/skill/SKILL.md",
				"missing required field",
				"Suggestion: Add a description field",
			},
		},
		{
			name: "error without suggestion",
			err: &ValidationError{
				FilePath:     "/path/to/command.md",
				ResourceType: "command",
				Err:          errors.New("parse error"),
			},
			contains: []string{
				"command",
				"/path/to/command.md",
				"parse error",
			},
		},
		{
			name: "minimal error",
			err: &ValidationError{
				Err: errors.New("generic error"),
			},
			contains: []string{
				"generic error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			for _, substr := range tt.contains {
				if !strings.Contains(msg, substr) {
					t.Errorf("Error message missing expected substring.\nExpected to contain: %q\nGot: %q",
						substr, msg)
				}
			}
		})
	}
}

func TestSuggestFix(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantMatch string // substring that should be in the suggestion
	}{
		{
			name:      "YAML mapping error",
			err:       errors.New("yaml: line 2: mapping values are not allowed"),
			wantMatch: "Quote values containing ':'",
		},
		{
			name:      "YAML mapping error mentions nested-mapping fix (issue #12)",
			err:       errors.New("yaml: line 3: mapping values are not allowed in this context"),
			wantMatch: "indented lines",
		},
		{
			name:      "name mismatch",
			err:       errors.New("skill name 'foo' must match directory name 'bar'"),
			wantMatch: "Rename the directory",
		},
		{
			name:      "missing description",
			err:       errors.New("description is required"),
			wantMatch: "Add a 'description' field",
		},
		{
			name:      "missing SKILL.md",
			err:       errors.New("directory must contain SKILL.md"),
			wantMatch: "Create a SKILL.md file",
		},
		{
			name:      "not a .md file",
			err:       errors.New("command must be a .md file"),
			wantMatch: "Rename the file with a .md extension",
		},
		{
			name:      "not a directory",
			err:       errors.New("skill must be a directory"),
			wantMatch: "Skills must be directories",
		},
		{
			name:      "no frontmatter",
			err:       errors.New("no frontmatter found"),
			wantMatch: "Add YAML frontmatter",
		},
		{
			name:      "no closing delimiter",
			err:       errors.New("no closing frontmatter delimiter found"),
			wantMatch: "Add closing '---' delimiter",
		},
		{
			name:      "invalid name format",
			err:       errors.New("invalid name format"),
			wantMatch: "lowercase alphanumeric",
		},
		{
			name:      "name starts with hyphen",
			err:       errors.New("name cannot start with hyphen"),
			wantMatch: "Remove leading hyphen",
		},
		{
			name:      "name ends with hyphen",
			err:       errors.New("name cannot end with hyphen"),
			wantMatch: "Remove trailing hyphen",
		},
		{
			name:      "consecutive hyphens",
			err:       errors.New("name contains consecutive hyphens"),
			wantMatch: "Replace consecutive hyphens",
		},
		{
			name:      "name too long",
			err:       errors.New("name too long (max 64 characters)"),
			wantMatch: "Shorten the name to 64 characters",
		},
		{
			name:      "description too long",
			err:       errors.New("description too long"),
			wantMatch: "Shorten the description to 1024 characters",
		},
		{
			name:      "unknown error",
			err:       errors.New("something completely unexpected"),
			wantMatch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestion := SuggestFix(tt.err)

			if tt.wantMatch == "" {
				// For unknown errors, we expect no suggestion
				if suggestion != "" {
					t.Errorf("Expected no suggestion for unknown error, got: %q", suggestion)
				}
				return
			}

			if !strings.Contains(suggestion, tt.wantMatch) {
				t.Errorf("Suggestion missing expected text.\nExpected to contain: %q\nGot: %q",
					tt.wantMatch, suggestion)
			}
		})
	}
}

func TestNewValidationError(t *testing.T) {
	path := "/path/to/resource"
	resType := "skill"
	resName := "my-skill"
	field := "description"
	err := errors.New("yaml: mapping values are not allowed")

	verr := NewValidationError(path, resType, resName, field, err)

	if verr.FilePath != path {
		t.Errorf("FilePath = %q, want %q", verr.FilePath, path)
	}
	if verr.ResourceType != resType {
		t.Errorf("ResourceType = %q, want %q", verr.ResourceType, resType)
	}
	if verr.ResourceName != resName {
		t.Errorf("ResourceName = %q, want %q", verr.ResourceName, resName)
	}
	if verr.FieldName != field {
		t.Errorf("FieldName = %q, want %q", verr.FieldName, field)
	}
	if verr.Err != err {
		t.Errorf("Err = %v, want %v", verr.Err, err)
	}

	// Suggestion should be auto-generated
	if verr.Suggestion == "" {
		t.Error("Expected auto-generated suggestion, got empty string")
	}
	if !strings.Contains(verr.Suggestion, "Quote") {
		t.Errorf("Expected suggestion about quoting, got: %q", verr.Suggestion)
	}
}

func TestWrapLoadError(t *testing.T) {
	path := "/path/to/skill"
	resType := Skill
	err := errors.New("parse error")

	wrapped := WrapLoadError(path, resType, err)

	verr, ok := wrapped.(*ValidationError)
	if !ok {
		t.Fatalf("Expected *ValidationError, got %T", wrapped)
	}

	if verr.FilePath != path {
		t.Errorf("FilePath = %q, want %q", verr.FilePath, path)
	}
	if verr.ResourceType != string(resType) {
		t.Errorf("ResourceType = %q, want %q", verr.ResourceType, string(resType))
	}
	if verr.Err != err {
		t.Errorf("Err = %v, want %v", verr.Err, err)
	}

	// Test that wrapping an already-wrapped error returns it as-is
	wrapped2 := WrapLoadError("/another/path", Command, verr)
	if wrapped2 != verr {
		t.Error("Expected WrapLoadError to return existing ValidationError as-is")
	}
}

func TestValidationError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	verr := &ValidationError{
		Err: innerErr,
	}

	unwrapped := errors.Unwrap(verr)
	if unwrapped != innerErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, innerErr)
	}
}
