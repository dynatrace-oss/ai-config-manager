// Package modifications handles creation of tool-specific file variants.
//
// When resources have frontmatter fields that need different values for different
// AI tools (e.g., model names), this package generates modified versions in a
// .modifications directory within the repository.
//
// Directory structure:
//
//	.modifications/
//	  opencode/
//	    skills/
//	      my-skill/
//	        SKILL.md
//	    agents/
//	      reviewer.md
//	    commands/
//	      deploy.md
//	  claude/
//	    skills/
//	      my-skill/
//	        SKILL.md
package modifications

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/config"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/frontmatter"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// ModificationsDirName is the name of the modifications directory in the repo
const ModificationsDirName = ".modifications"

// Generator handles creation of tool-specific file modifications
type Generator struct {
	repoPath string
	mappings config.TypeMappings
	logger   *slog.Logger
}

// NewGenerator creates a generator for the given repo
func NewGenerator(repoPath string, mappings config.TypeMappings, logger *slog.Logger) *Generator {
	return &Generator{
		repoPath: repoPath,
		mappings: mappings,
		logger:   logger,
	}
}

// ModificationsDir returns the .modifications directory path
func (g *Generator) ModificationsDir() string {
	return filepath.Join(g.repoPath, ModificationsDirName)
}

// GenerateForResource creates tool-specific variants for a resource.
// Returns list of tools that had modifications generated.
func (g *Generator) GenerateForResource(res *resource.Resource) ([]string, error) {
	if res == nil {
		return nil, fmt.Errorf("resource is nil")
	}

	// Get list of tools that have mappings defined
	toolsWithMappings := g.mappings.GetToolsWithMappings()
	if len(toolsWithMappings) == 0 {
		return nil, nil
	}

	var generatedTools []string

	for _, toolName := range toolsWithMappings {
		generated, err := g.generateForTool(res, toolName)
		if err != nil {
			return generatedTools, fmt.Errorf("generating modification for tool %s: %w", toolName, err)
		}
		if generated {
			generatedTools = append(generatedTools, toolName)
		}
	}

	return generatedTools, nil
}

// generateForTool creates a modification for a specific tool if needed.
// Returns true if a modification was generated.
func (g *Generator) generateForTool(res *resource.Resource, toolName string) (bool, error) {
	// Determine which file to process based on resource type
	filePath := g.getSourceFilePath(res)
	if filePath == "" {
		return false, nil
	}

	// Read source file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("reading source file: %w", err)
	}

	// Parse frontmatter
	fm, err := frontmatter.Parse(content)
	if err != nil {
		return false, fmt.Errorf("parsing frontmatter: %w", err)
	}

	// If no frontmatter, skip this file
	if fm == nil {
		return false, nil
	}

	// Get fields that have mappings for this resource type
	fieldMappings := g.getFieldMappingsForType(res.Type)
	if fieldMappings == nil {
		return false, nil
	}

	// Apply mappings to frontmatter fields
	modified := false
	for fieldName := range fieldMappings {
		currentValue := fm.GetString(fieldName)
		mappedValue, found := g.mappings.GetMappingWithNull(res.Type, fieldName, currentValue, toolName)
		if found {
			fm.SetField(fieldName, mappedValue)
			modified = true
			if g.logger != nil {
				g.logger.Debug("applied field mapping",
					"resource", res.Name,
					"tool", toolName,
					"field", fieldName,
					"from", currentValue,
					"to", mappedValue,
				)
			}
		}
	}

	// If no fields were modified, don't create a modification file
	if !modified {
		return false, nil
	}

	// Create output path
	outputPath := g.getModificationFilePath(res, toolName)

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return false, fmt.Errorf("creating output directory: %w", err)
	}

	// Write transformed file
	renderedContent := fm.Render()
	if err := os.WriteFile(outputPath, renderedContent, 0644); err != nil {
		return false, fmt.Errorf("writing modification file: %w", err)
	}

	if g.logger != nil {
		g.logger.Info("generated modification",
			"resource", res.Name,
			"tool", toolName,
			"path", outputPath,
		)
	}

	return true, nil
}

// getSourceFilePath returns the path to the file that should be transformed.
func (g *Generator) getSourceFilePath(res *resource.Resource) string {
	switch res.Type {
	case resource.Skill:
		// For skills, process SKILL.md in the resource directory
		return filepath.Join(res.Path, "SKILL.md")
	case resource.Agent, resource.Command:
		// For agents/commands, process the resource file itself
		return res.Path
	default:
		return ""
	}
}

// getFieldMappingsForType returns the field mappings for a resource type.
func (g *Generator) getFieldMappingsForType(resType resource.ResourceType) config.FieldMappings {
	switch resType {
	case resource.Skill:
		return g.mappings.Skill
	case resource.Agent:
		return g.mappings.Agent
	case resource.Command:
		return g.mappings.Command
	default:
		return nil
	}
}

// getModificationFilePath returns the path where the modification file should be written.
func (g *Generator) getModificationFilePath(res *resource.Resource, toolName string) string {
	typePlural := g.getTypePluralDir(res.Type)
	if typePlural == "" {
		return ""
	}

	switch res.Type {
	case resource.Skill:
		// Skills: .modifications/<tool>/skills/<name>/SKILL.md
		return filepath.Join(g.ModificationsDir(), toolName, typePlural, res.Name, "SKILL.md")
	case resource.Agent, resource.Command:
		// Agents/Commands: .modifications/<tool>/<type>s/<name>.md
		return filepath.Join(g.ModificationsDir(), toolName, typePlural, res.Name+".md")
	default:
		return ""
	}
}

// getTypePluralDir returns the plural directory name for a resource type.
func (g *Generator) getTypePluralDir(resType resource.ResourceType) string {
	switch resType {
	case resource.Skill:
		return "skills"
	case resource.Agent:
		return "agents"
	case resource.Command:
		return "commands"
	default:
		return ""
	}
}

// GenerateAll regenerates all modifications for all resources in the repo.
func (g *Generator) GenerateAll() error {
	// Clean up existing modifications first
	if err := g.CleanupAll(); err != nil {
		return fmt.Errorf("cleaning up existing modifications: %w", err)
	}

	// Find all resources in the repo
	resourceTypes := []struct {
		dir      string
		resType  resource.ResourceType
		loadFunc func(string) (*resource.Resource, error)
		isDir    bool
	}{
		{"skills", resource.Skill, resource.LoadSkill, true},
		{"agents", resource.Agent, resource.LoadAgent, false},
		{"commands", resource.Command, resource.LoadCommand, false},
	}

	for _, rt := range resourceTypes {
		typeDir := filepath.Join(g.repoPath, rt.dir)
		if _, err := os.Stat(typeDir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(typeDir)
		if err != nil {
			return fmt.Errorf("reading %s directory: %w", rt.dir, err)
		}

		for _, entry := range entries {
			entryPath := filepath.Join(typeDir, entry.Name())

			// Use os.Stat to follow symlinks when checking if directory
			info, err := os.Stat(entryPath)
			if err != nil {
				continue // Skip entries we can't stat
			}
			isDir := info.IsDir()

			var resPath string
			if rt.isDir {
				if !isDir {
					continue
				}
				resPath = entryPath
			} else {
				if isDir || filepath.Ext(entry.Name()) != ".md" {
					continue
				}
				resPath = entryPath
			}

			res, err := rt.loadFunc(resPath)
			if err != nil {
				if g.logger != nil {
					g.logger.Warn("failed to load resource",
						"path", resPath,
						"error", err.Error(),
					)
				}
				continue
			}

			if _, err := g.GenerateForResource(res); err != nil {
				return fmt.Errorf("generating modifications for %s: %w", res.Name, err)
			}
		}
	}

	return nil
}

// CleanupForResource removes modifications for a resource.
func (g *Generator) CleanupForResource(res *resource.Resource) error {
	if res == nil {
		return fmt.Errorf("resource is nil")
	}

	toolsWithMappings := g.mappings.GetToolsWithMappings()
	for _, toolName := range toolsWithMappings {
		modPath := g.GetModificationPath(res, toolName)
		if modPath == "" {
			continue
		}

		// For skills, modPath is the directory; for others, it's the file
		if res.Type == resource.Skill {
			// Remove the skill directory
			if err := os.RemoveAll(modPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing modification directory for tool %s: %w", toolName, err)
			}
		} else {
			// Remove the file
			if err := os.Remove(modPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing modification file for tool %s: %w", toolName, err)
			}
		}

		if g.logger != nil {
			g.logger.Debug("cleaned up modification",
				"resource", res.Name,
				"tool", toolName,
				"path", modPath,
			)
		}
	}

	return nil
}

// CleanupAll removes all modifications (deletes .modifications directory).
func (g *Generator) CleanupAll() error {
	modDir := g.ModificationsDir()
	if _, err := os.Stat(modDir); os.IsNotExist(err) {
		return nil
	}

	if err := os.RemoveAll(modDir); err != nil {
		return fmt.Errorf("removing modifications directory: %w", err)
	}

	if g.logger != nil {
		g.logger.Info("cleaned up all modifications",
			"path", modDir,
		)
	}

	return nil
}

// GetModificationPath returns path to modified file/directory for a tool.
// For skills, returns the directory path (e.g., .modifications/opencode/skills/my-skill/).
// For agents/commands, returns the file path.
// Returns empty string if no modification exists.
func (g *Generator) GetModificationPath(res *resource.Resource, toolName string) string {
	if res == nil {
		return ""
	}

	typePlural := g.getTypePluralDir(res.Type)
	if typePlural == "" {
		return ""
	}

	var checkPath string
	switch res.Type {
	case resource.Skill:
		// For skills, return the directory path
		checkPath = filepath.Join(g.ModificationsDir(), toolName, typePlural, res.Name)
	case resource.Agent, resource.Command:
		// For agents/commands, return the file path
		checkPath = filepath.Join(g.ModificationsDir(), toolName, typePlural, res.Name+".md")
	default:
		return ""
	}

	// Check if the path exists
	if _, err := os.Stat(checkPath); os.IsNotExist(err) {
		return ""
	}

	return checkPath
}

// HasModification checks if a modification exists for resource+tool.
func (g *Generator) HasModification(res *resource.Resource, toolName string) bool {
	return g.GetModificationPath(res, toolName) != ""
}
