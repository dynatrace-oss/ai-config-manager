package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// Package-level logger for command parsing logging
var logger *slog.Logger

// SetLogger sets the logger for the cmd package.
// This should be called by the application during initialization.
func SetLogger(l *slog.Logger) {
	logger = l
}

// ExpandPattern expands a pattern to matching resources from repository.
// If the argument is not a pattern, returns it as-is in a single-element slice.
// Returns a slice of resource arguments in "type/name" format.
func ExpandPattern(mgr *repo.Manager, resourceArg string) ([]string, error) {
	// Parse pattern
	resourceType, _, isPattern := pattern.ParsePattern(resourceArg)
	if !isPattern {
		// Not a pattern, return as-is
		return []string{resourceArg}, nil
	}

	// Create matcher
	matcher, err := pattern.NewMatcher(resourceArg)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern '%s': %w", resourceArg, err)
	}

	// List resources from repository
	var resources []resource.Resource
	if resourceType != "" {
		// Specific type filter
		resources, err = mgr.List(&resourceType)
	} else {
		// All types
		resources, err = mgr.List(nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	// Filter by pattern
	var matches []string
	for _, res := range resources {
		if matcher.Match(&res) {
			// Return in "type/name" format
			matches = append(matches, FormatResourceArg(&res))
		}
	}

	return matches, nil
}

// ParseResourceArg parses a resource argument in the format "type/name".
// Supports both singular and plural type names (e.g., "skill" and "skills").
// Returns the resource type, name, and any error.
func ParseResourceArg(arg string) (resource.ResourceType, string, error) {
	if logger != nil {
		logger.Debug("parsing resource argument",
			"argument", arg,
			"parser", "cmd.ParseResourceArg",
			"method", "SplitN")
	}

	parts := strings.SplitN(arg, "/", 2)
	if logger != nil {
		logger.Debug("split resource argument",
			"parts", parts,
			"count", len(parts))
	}

	if len(parts) != 2 {
		if logger != nil {
			logger.Debug("parsing failed: invalid part count",
				"count", len(parts),
				"reason", "expected exactly 2 parts (type/name)")
		}
		return "", "", fmt.Errorf("invalid format: must be 'type/name' (e.g., skill/my-skill, command/my-command, agent/my-agent)")
	}

	typeStr := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])

	if logger != nil {
		logger.Debug("parsed resource components",
			"type_string", typeStr,
			"name", name)
	}

	if name == "" {
		if logger != nil {
			logger.Debug("parsing failed: empty name")
		}
		return "", "", fmt.Errorf("resource name cannot be empty")
	}

	// Parse and validate type
	resourceType, err := parseResourceType(typeStr)
	if err != nil {
		if logger != nil {
			logger.Debug("parsing failed: invalid type",
				"type_string", typeStr,
				"error", err.Error())
		}
		return "", "", err
	}

	if logger != nil {
		logger.Debug("parsing succeeded",
			"type", resourceType,
			"name", name)
	}

	return resourceType, name, nil
}

// FormatResourceArg formats a resource as "type/name".
func FormatResourceArg(res *resource.Resource) string {
	return fmt.Sprintf("%s/%s", res.Type, res.Name)
}

// parseResourceType converts a string to ResourceType.
// Supports both singular and plural forms (e.g., "skill" and "skills").
func parseResourceType(s string) (resource.ResourceType, error) {
	// Normalize: lowercase and handle plural forms
	s = strings.ToLower(s)
	switch s {
	case "skill", "skills":
		return resource.Skill, nil
	case "command", "commands":
		return resource.Command, nil
	case "agent", "agents":
		return resource.Agent, nil
	case "package", "packages":
		return resource.PackageType, nil
	default:
		return "", fmt.Errorf("invalid resource type '%s': must be one of 'skill', 'command', 'agent', or 'package'", s)
	}
}
