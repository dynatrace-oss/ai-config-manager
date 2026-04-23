package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InferTypeFromPath infers a resource type from path segments and well-known suffixes.
//
// It is intentionally conservative: it only returns ok=true when a type can be
// inferred from explicit path conventions (commands/, skills/, agents/, packages/)
// or package filename suffixes.
func InferTypeFromPath(path string) (ResourceType, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}

	normalizedPath := strings.ReplaceAll(path, "\\", "/")
	normalizedPath = filepath.ToSlash(filepath.Clean(normalizedPath))
	for _, segment := range strings.Split(normalizedPath, "/") {
		switch segment {
		case "commands":
			return Command, true
		case "skills":
			return Skill, true
		case "agents":
			return Agent, true
		case "packages":
			return PackageType, true
		}
	}

	if strings.HasSuffix(normalizedPath, ".package.json") {
		return PackageType, true
	}

	return "", false
}

// Load loads a resource from the filesystem
// It detects whether the path is a command, agent, or skill
func Load(path string) (*Resource, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	if info.IsDir() {
		// Load as skill
		return LoadSkill(path)
	}

	// For .md files, detect type and load appropriately
	resourceType, err := DetectType(path)
	if err != nil {
		return nil, fmt.Errorf("failed to detect type: %w", err)
	}

	switch resourceType {
	case Agent:
		return LoadAgent(path)
	case Command:
		return LoadCommand(path)
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

// DetectType detects the resource type from a filesystem path
func DetectType(path string) (ResourceType, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat path: %w", err)
	}

	if info.IsDir() {
		// Check if SKILL.md exists
		skillPath := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			return Skill, nil
		}
		return "", fmt.Errorf("directory does not contain SKILL.md")
	}

	// Check if it's a .md file
	if filepath.Ext(path) == ".md" {
		// Use path-based detection first (more reliable for bulk imports)
		if inferredType, ok := InferTypeFromPath(path); ok {
			switch inferredType {
			case Agent:
				return Agent, nil
			case Command:
				return Command, nil
			}
		}

		// Parse frontmatter to distinguish between agent and command
		frontmatter, _, err := ParseFrontmatter(path)
		if err != nil {
			// If we can't parse frontmatter, fall back to Command
			return Command, nil
		}

		// Check for agent-specific fields (type, instructions, capabilities)
		// If any exist, it's an agent
		_, hasType := frontmatter["type"]
		_, hasInstructions := frontmatter["instructions"]
		_, hasCapabilities := frontmatter["capabilities"]
		if hasType || hasInstructions || hasCapabilities {
			return Agent, nil
		}

		// Check for command-specific fields (agent, model, allowed-tools)
		// If any exist, it's a command
		_, hasAgent := frontmatter["agent"]
		_, hasModel := frontmatter["model"]
		_, hasAllowedTools := frontmatter["allowed-tools"]
		if hasAgent || hasModel || hasAllowedTools {
			return Command, nil
		}

		// Default to command for backward compatibility
		// (Most .md files without specific agent fields are commands)
		return Command, nil
	}

	return "", fmt.Errorf("not a valid resource (must be .md file or directory with SKILL.md)")
}
