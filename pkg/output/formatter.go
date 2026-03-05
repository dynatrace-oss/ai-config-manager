package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/olekukonko/tablewriter"
	"gopkg.in/yaml.v3"
)

// Format represents an output format type
type Format string

const (
	Table Format = "table"
	JSON  Format = "json"
	YAML  Format = "yaml"
)

// ParseFormat parses a format string into a Format type
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "table":
		return Table, nil
	case "json":
		return JSON, nil
	case "yaml":
		return YAML, nil
	default:
		return "", fmt.Errorf("invalid format: %s (valid: table, json, yaml)", s)
	}
}

// BulkOperationResult represents the result of a bulk repository operation
type BulkOperationResult struct {
	Added        []ResourceResult `json:"added" yaml:"added"`
	Updated      []ResourceResult `json:"updated" yaml:"updated"`
	Skipped      []ResourceResult `json:"skipped" yaml:"skipped"`
	Failed       []ResourceResult `json:"failed" yaml:"failed"`
	CommandCount int              `json:"command_count" yaml:"command_count"`
	SkillCount   int              `json:"skill_count" yaml:"skill_count"`
	AgentCount   int              `json:"agent_count" yaml:"agent_count"`
	PackageCount int              `json:"package_count" yaml:"package_count"`
}

// ResourceResult represents the result of a single resource operation
type ResourceResult struct {
	Name    string `json:"name" yaml:"name"`
	Type    string `json:"type" yaml:"type"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	Path    string `json:"path,omitempty" yaml:"path,omitempty"`
}

// FromBulkImportResult converts a repo.BulkImportResult to BulkOperationResult
func FromBulkImportResult(result *repo.BulkImportResult) *BulkOperationResult {
	bor := &BulkOperationResult{
		Added:        make([]ResourceResult, 0, len(result.Added)),
		Updated:      make([]ResourceResult, 0, len(result.Updated)),
		Skipped:      make([]ResourceResult, 0, len(result.Skipped)),
		Failed:       make([]ResourceResult, 0, len(result.Failed)),
		CommandCount: result.CommandCount,
		SkillCount:   result.SkillCount,
		AgentCount:   result.AgentCount,
		PackageCount: result.PackageCount,
	}

	// Convert added resources
	for _, path := range result.Added {
		bor.Added = append(bor.Added, ResourceResult{
			Name: extractResourceName(path),
			Type: extractResourceType(path),
			Path: path,
		})
	}

	// Convert updated resources
	for _, path := range result.Updated {
		bor.Updated = append(bor.Updated, ResourceResult{
			Name: extractResourceName(path),
			Type: extractResourceType(path),
			Path: path,
		})
	}

	// Convert skipped resources
	for _, path := range result.Skipped {
		bor.Skipped = append(bor.Skipped, ResourceResult{
			Name:    extractResourceName(path),
			Type:    extractResourceType(path),
			Path:    path,
			Message: "already exists",
		})
	}

	// Convert failed resources
	for _, fail := range result.Failed {
		bor.Failed = append(bor.Failed, ResourceResult{
			Name:    extractResourceName(fail.Path),
			Type:    extractResourceType(fail.Path),
			Path:    fail.Path,
			Message: fail.Message,
		})
	}

	return bor
}

// extractResourceName extracts the resource name from a file path
// For nested resources, returns the relative path from the resource type directory
// Example: "/path/to/repo/commands/opencode-coder/doctor.md" -> "opencode-coder/doctor"
// extractResourceName extracts the resource name from a file path
// For nested resources, returns the relative path from the resource type directory
// Example: "/path/to/repo/commands/opencode-coder/doctor.md" -> "opencode-coder/doctor"
func extractResourceName(path string) string {
	// Handle empty or invalid paths
	if path == "" || strings.HasSuffix(path, "/") || strings.HasSuffix(path, "\\") {
		return ""
	}

	// Normalize path separators
	path = strings.ReplaceAll(path, "\\", "/")

	// Find the resource type directory (commands/, skills/, agents/, packages/)
	var relPath string

	if idx := strings.Index(path, "/commands/"); idx != -1 {
		relPath = path[idx+len("/commands/"):]
	} else if idx := strings.Index(path, "/skills/"); idx != -1 {
		relPath = path[idx+len("/skills/"):]
	} else if idx := strings.Index(path, "/agents/"); idx != -1 {
		relPath = path[idx+len("/agents/"):]
	} else if idx := strings.Index(path, "/packages/"); idx != -1 {
		relPath = path[idx+len("/packages/"):]
	} else {
		// Fallback: just get the basename
		if idx := strings.LastIndex(path, "/"); idx != -1 {
			relPath = path[idx+1:]
		} else {
			relPath = path
		}
	}

	// Remove file extensions
	relPath = strings.TrimSuffix(relPath, ".md")
	relPath = strings.TrimSuffix(relPath, ".package.json")

	return relPath
}

// extractResourceType extracts the resource type from a file path
func extractResourceType(path string) string {
	if strings.Contains(path, "/commands/") || strings.Contains(path, "\\commands\\") {
		return "command"
	}
	if strings.Contains(path, "/skills/") || strings.Contains(path, "\\skills\\") {
		return "skill"
	}
	if strings.Contains(path, "/agents/") || strings.Contains(path, "\\agents\\") {
		return "agent"
	}
	if strings.Contains(path, "/packages/") || strings.Contains(path, "\\packages\\") {
		return "package"
	}
	if strings.HasSuffix(path, ".package.json") {
		return "package"
	}
	if strings.HasSuffix(path, ".md") {
		return "command"
	}
	return "resource"
}

// FormatBulkResult formats a BulkOperationResult according to the specified format
func FormatBulkResult(result *BulkOperationResult, format Format) error {
	switch format {
	case Table:
		return formatAsTable(result)
	case JSON:
		return formatAsJSON(result)
	case YAML:
		return formatAsYAML(result)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// formatAsTable formats the result as a table (human-readable)
func formatAsTable(result *BulkOperationResult) error {
	// Create unified table with all resources
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("NAME", "STATUS", "MESSAGE")

	// Add all resources to table
	hasContent := false

	// Add successful resources
	for _, res := range result.Added {
		status := "SUCCESS"
		message := res.Message
		if message == "" {
			message = "Added to repository"
		}
		if err := table.Append(fmt.Sprintf("%s/%s", res.Type, res.Name), status, message); err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
		hasContent = true
	}

	// Add updated resources
	for _, res := range result.Updated {
		status := "SUCCESS"
		message := res.Message
		if message == "" {
			message = "Updated in repository"
		}
		if err := table.Append(fmt.Sprintf("%s/%s", res.Type, res.Name), status, message); err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
		hasContent = true
	}

	// Add skipped resources
	for _, res := range result.Skipped {
		status := "SKIPPED"
		message := res.Message
		if message == "" {
			message = "Already exists"
		}
		if err := table.Append(fmt.Sprintf("%s/%s", res.Type, res.Name), status, message); err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
		hasContent = true
	}

	// Add failed resources
	for _, res := range result.Failed {
		status := "FAILED"
		message := res.Message
		if message == "" {
			message = "Unknown error"
		}
		if err := table.Append(fmt.Sprintf("%s/%s", res.Type, res.Name), status, message); err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
		hasContent = true
	}

	// Render table if there's content
	if hasContent {
		if err := table.Render(); err != nil {
			return fmt.Errorf("failed to render table: %w", err)
		}
		fmt.Println()
	}

	// Print summary line
	totalAdded := len(result.Added)
	totalUpdated := len(result.Updated)
	totalSkipped := len(result.Skipped)
	totalFailed := len(result.Failed)
	totalResources := totalAdded + totalUpdated + totalSkipped + totalFailed

	if totalResources == 0 {
		fmt.Println("No resources to process")
	} else {
		fmt.Printf("Summary: %d added, %d updated, %d failed, %d skipped (%d total)\n",
			totalAdded, totalUpdated, totalFailed, totalSkipped, totalResources)
	}

	// Show tip about JSON format if there are failures
	if totalFailed > 0 {
		fmt.Println()
		fmt.Println("⚠ Use --format=json to see detailed error messages")
	}

	return nil
}

// formatAsJSON formats the result as JSON
func formatAsJSON(result *BulkOperationResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// formatAsYAML formats the result as YAML
func formatAsYAML(result *BulkOperationResult) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	defer func() { _ = encoder.Close() }()
	return encoder.Encode(result)
}

// FormatOutput formats data according to the specified format
// It handles common data types (TableData, KeyValueData) and falls back
// to generic encoding for other types
func FormatOutput(data interface{}, format Format) error {
	switch d := data.(type) {
	case *TableData:
		return formatTableData(d, format)
	case *KeyValueData:
		return formatKeyValueData(d, format)
	case Renderable:
		return d.Render(format)
	default:
		// Fallback: direct JSON/YAML encoding
		return formatGeneric(data, format)
	}
}

// formatGeneric handles types that don't have specialized formatters
func formatGeneric(data interface{}, format Format) error {
	switch format {
	case Table:
		return fmt.Errorf("table format not supported for this data type")
	case JSON:
		return EncodeJSON(os.Stdout, data)
	case YAML:
		return EncodeYAML(os.Stdout, data)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}
