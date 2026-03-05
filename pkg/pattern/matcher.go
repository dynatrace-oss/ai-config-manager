package pattern

import (
	"fmt"
	"strings"

	"github.com/gobwas/glob"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// Matcher holds compiled glob patterns for efficient matching
type Matcher struct {
	pattern      glob.Glob
	resourceType resource.ResourceType // empty string means match all types
	isPattern    bool
}

// NewMatcher creates a new pattern matcher from a pattern string.
// The pattern can be in the format:
// - "pattern" - matches pattern against all resource types
// - "type/pattern" - matches pattern only for specified type
//
// Supported glob patterns:
// - * matches any sequence of characters
// - ? matches any single character
// - [abc] matches any character in the set
// - {a,b} matches any alternative
func NewMatcher(pattern string) (*Matcher, error) {
	if pattern == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}

	resourceType, patternStr, isPattern := ParsePattern(pattern)

	// Compile the glob pattern
	g, err := glob.Compile(patternStr)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}

	return &Matcher{
		pattern:      g,
		resourceType: resourceType,
		isPattern:    isPattern,
	}, nil
}

// Match tests if a resource matches the pattern.
// It checks both the resource type (if specified) and the resource name.
func (m *Matcher) Match(res *resource.Resource) bool {
	// Check resource type if specified
	if m.resourceType != "" && res.Type != m.resourceType {
		return false
	}

	// Match the name against the pattern
	return m.pattern.Match(res.Name)
}

// MatchName tests if a resource name matches the pattern without type checking.
func (m *Matcher) MatchName(name string) bool {
	return m.pattern.Match(name)
}

// IsPattern returns true if the matcher was created from a pattern (contains glob characters).
func (m *Matcher) IsPattern() bool {
	return m.isPattern
}

// GetResourceType returns the resource type filter, or empty string if matching all types.
func (m *Matcher) GetResourceType() resource.ResourceType {
	return m.resourceType
}

// ParsePattern parses a pattern string into resource type and pattern components.
// It detects whether the string contains glob characters.
//
// Format:
// - "pattern" -> ("", "pattern", hasGlobChars)
// - "type/pattern" -> ("type", "pattern", hasGlobChars)
//
// Returns: resourceType, pattern, isPattern
func ParsePattern(arg string) (resource.ResourceType, string, bool) {
	// Check if contains type prefix (e.g., "skill/pdf*")
	parts := strings.SplitN(arg, "/", 2)

	var resourceType resource.ResourceType
	var pattern string

	if len(parts) == 2 {
		// Has type prefix
		typeStr := parts[0]
		pattern = parts[1]

		// Validate and convert type
		switch typeStr {
		case "command":
			resourceType = resource.Command
		case "skill":
			resourceType = resource.Skill
		case "agent":
			resourceType = resource.Agent
		case "package":
			resourceType = resource.PackageType
		default:
			// If not a valid type, treat the whole thing as a pattern
			// (e.g., "foo/bar" becomes pattern "foo/bar")
			pattern = arg
			resourceType = ""
		}
	} else {
		// No type prefix
		pattern = arg
	}

	// Detect if it's a glob pattern
	isPattern := IsPattern(pattern)

	return resourceType, pattern, isPattern
}

// IsPattern returns true if the string contains glob pattern characters.
func IsPattern(s string) bool {
	// Check for glob special characters
	return strings.ContainsAny(s, "*?[{")
}

// MatchesPattern is a convenience function that tests if a name matches a pattern string.
// It compiles the pattern on each call, so use NewMatcher for repeated matching.
func MatchesPattern(pattern, name string) (bool, error) {
	m, err := NewMatcher(pattern)
	if err != nil {
		return false, err
	}
	return m.MatchName(name), nil
}

// FilterResources filters a slice of resources by a pattern.
// Returns a new slice containing only matching resources.
func FilterResources(resources []*resource.Resource, pattern string) ([]*resource.Resource, error) {
	m, err := NewMatcher(pattern)
	if err != nil {
		return nil, err
	}

	var filtered []*resource.Resource
	for _, res := range resources {
		if m.Match(res) {
			filtered = append(filtered, res)
		}
	}

	return filtered, nil
}
