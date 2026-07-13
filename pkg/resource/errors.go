package resource

import (
	"fmt"
	"strings"
)

// ValidationError represents a validation error with context and suggestions
type ValidationError struct {
	FilePath     string // Path to the file that failed
	ResourceName string // Resource name (if available)
	ResourceType string // Resource type (command, skill, agent, package)
	FieldName    string // Field that failed validation
	Err          error  // Underlying error
	Suggestion   string // Actionable suggestion for fixing the error
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	var parts []string

	// Add resource type and name if available
	if e.ResourceType != "" && e.ResourceName != "" {
		parts = append(parts, fmt.Sprintf("%s '%s'", e.ResourceType, e.ResourceName))
	} else if e.ResourceType != "" {
		parts = append(parts, e.ResourceType)
	}

	// Add field name if available
	if e.FieldName != "" {
		parts = append(parts, fmt.Sprintf("field '%s'", e.FieldName))
	}

	// Add file path
	if e.FilePath != "" {
		parts = append(parts, fmt.Sprintf("in %s", e.FilePath))
	}

	// Build error message
	msg := strings.Join(parts, " ")
	if msg != "" {
		msg = msg + ": "
	}

	// Add underlying error
	if e.Err != nil {
		msg = msg + e.Err.Error()
	} else {
		msg = msg + "validation failed"
	}

	// Add suggestion if available
	if e.Suggestion != "" {
		msg = msg + "\n  → Suggestion: " + e.Suggestion
	}

	return msg
}

// Unwrap returns the underlying error
func (e *ValidationError) Unwrap() error {
	return e.Err
}

// NewValidationError creates a new validation error with context
func NewValidationError(path, resourceType, resourceName, field string, err error) *ValidationError {
	verr := &ValidationError{
		FilePath:     path,
		ResourceType: resourceType,
		ResourceName: resourceName,
		FieldName:    field,
		Err:          err,
	}

	// Auto-generate suggestion based on error
	verr.Suggestion = SuggestFix(err)

	return verr
}

// SuggestFix suggests fixes for common validation errors
func SuggestFix(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// YAML parsing errors
	if strings.Contains(errMsg, "mapping values are not allowed") ||
		strings.Contains(errMsg, "did not find expected key") {
		return "YAML mapping error: an unquoted ':' inside a scalar value, " +
			"or a nested mapping written on the same line as its parent key.\n" +
			"  Fixes:\n" +
			"    - Quote values containing ':' (e.g., description: \"My tool: a helper\")\n" +
			"    - Put nested mappings on their own indented lines, e.g.:\n" +
			"        permissions:\n" +
			"          bash:\n" +
			"            \"*\": ask"
	}

	if strings.Contains(errMsg, "yaml: unmarshal errors") {
		return "Check YAML syntax - ensure proper indentation and field formatting"
	}

	if strings.Contains(errMsg, "cannot unmarshal") {
		return "Check field types - arrays should use YAML list syntax with dashes"
	}

	// Name mismatch errors
	if strings.Contains(errMsg, "name") && strings.Contains(errMsg, "must match") {
		return "Rename the directory or file to match the 'name' field in frontmatter"
	}

	// Missing required fields
	if strings.Contains(errMsg, "description is required") ||
		strings.Contains(errMsg, "description: required") {
		return "Add a 'description' field to the frontmatter with a brief explanation"
	}

	if strings.Contains(errMsg, "name is required") ||
		strings.Contains(errMsg, "name: required") {
		return "Add a 'name' field to the frontmatter"
	}

	// File structure errors
	if strings.Contains(errMsg, "SKILL.md") {
		return "Create a SKILL.md file in the skill directory with proper frontmatter"
	}

	if strings.Contains(errMsg, "must be a .md file") {
		return "Rename the file with a .md extension"
	}

	if strings.Contains(errMsg, "must be a directory") {
		return "Skills must be directories containing SKILL.md, not single files"
	}

	// Frontmatter errors
	if strings.Contains(errMsg, "no frontmatter found") {
		return "Add YAML frontmatter at the top of the file:\n---\ndescription: Your description here\n---"
	}

	if strings.Contains(errMsg, "no closing frontmatter delimiter") {
		return "Add closing '---' delimiter after the frontmatter section"
	}

	// Name validation errors
	if strings.Contains(errMsg, "invalid name format") {
		return "Use lowercase alphanumeric characters and hyphens only (e.g., 'my-skill', 'test-command')"
	}

	if strings.Contains(errMsg, "name cannot start with hyphen") {
		return "Remove leading hyphen from the name"
	}

	if strings.Contains(errMsg, "name cannot end with hyphen") {
		return "Remove trailing hyphen from the name"
	}

	if strings.Contains(errMsg, "consecutive hyphens") {
		return "Replace consecutive hyphens (--) with a single hyphen (-)"
	}

	// Length errors
	if strings.Contains(errMsg, "name too long") {
		return "Shorten the name to 64 characters or less"
	}

	if strings.Contains(errMsg, "description too long") {
		return "Shorten the description to 1024 characters or less"
	}

	// Generic fallback
	return ""
}

// WrapLoadError wraps a resource loading error with context
func WrapLoadError(path string, resType ResourceType, err error) error {
	// If it's already a ValidationError, return as-is
	if _, ok := err.(*ValidationError); ok {
		return err
	}

	// Create a new ValidationError with context
	verr := &ValidationError{
		FilePath:     path,
		ResourceType: string(resType),
		Err:          err,
	}

	// Auto-generate suggestion
	verr.Suggestion = SuggestFix(err)

	return verr
}
