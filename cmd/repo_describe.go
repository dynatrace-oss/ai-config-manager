package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var describeFormatFlag string

// DescribeResourceOutput represents detailed resource information for JSON/YAML output
type DescribeResourceOutput struct {
	Type        string                     `json:"type" yaml:"type"`
	Name        string                     `json:"name" yaml:"name"`
	Description string                     `json:"description" yaml:"description"`
	Version     string                     `json:"version,omitempty" yaml:"version,omitempty"`
	Author      string                     `json:"author,omitempty" yaml:"author,omitempty"`
	License     string                     `json:"license,omitempty" yaml:"license,omitempty"`
	Metadata    *metadata.ResourceMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Location    string                     `json:"location" yaml:"location"`
	// Type-specific fields
	Compatibility   []string                  `json:"compatibility,omitempty" yaml:"compatibility,omitempty"`       // skill only
	HasScripts      *bool                     `json:"has_scripts,omitempty" yaml:"has_scripts,omitempty"`           // skill only
	HasReferences   *bool                     `json:"has_references,omitempty" yaml:"has_references,omitempty"`     // skill only
	HasAssets       *bool                     `json:"has_assets,omitempty" yaml:"has_assets,omitempty"`             // skill only
	Agent           string                    `json:"agent,omitempty" yaml:"agent,omitempty"`                       // command only
	Model           string                    `json:"model,omitempty" yaml:"model,omitempty"`                       // command only
	AllowedTools    []string                  `json:"allowed_tools,omitempty" yaml:"allowed_tools,omitempty"`       // command only
	AgentType       string                    `json:"agent_type,omitempty" yaml:"agent_type,omitempty"`             // agent only
	Instructions    string                    `json:"instructions,omitempty" yaml:"instructions,omitempty"`         // agent only
	Capabilities    []string                  `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`         // agent only
	ResourceCount   *int                      `json:"resource_count,omitempty" yaml:"resource_count,omitempty"`     // package only
	Resources       []string                  `json:"resources,omitempty" yaml:"resources,omitempty"`               // package only
	PackageMetadata *metadata.PackageMetadata `json:"package_metadata,omitempty" yaml:"package_metadata,omitempty"` // package only
}

// repoDescribeCmd represents the repo describe command
var repoDescribeCmd = &cobra.Command{
	Use:     "describe <pattern> [pattern...]",
	Aliases: []string{"show"}, // Deprecated: use 'describe' instead
	ValidArgsFunction: completeResourcesWithOptions(completionOptions{
		includePackages: true,
		multiArg:        true,
	}),
	Short: "Display detailed resource information",
	Long: `Display detailed information about resources in the repository.

Examples:
  aimgr repo describe skill/pdf-processing    # Describe specific skill
  aimgr repo describe command/test            # Describe specific command
  aimgr repo describe agent/code-reviewer     # Describe specific agent
  aimgr repo describe package/dynatrace-core  # Describe specific package
  aimgr repo describe skill/* command/*       # Describe multiple patterns
  aimgr repo describe *pdf*                   # Describe all resources with "pdf"

Supports glob patterns: *, ?, [abc], {a,b}
Accepts multiple arguments to describe several resources at once.

When multiple resources match, shows a summary list.
When a single resource matches, shows detailed information.

Note: 'repo show' is deprecated, use 'repo describe' instead.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate format flag
		if describeFormatFlag != "table" && describeFormatFlag != "json" && describeFormatFlag != "yaml" {
			return fmt.Errorf("invalid format: %s (must be 'table', 'json', or 'yaml')", describeFormatFlag)
		}

		manager, err := NewManagerWithLogLevel()
		if err != nil {
			return err
		}

		if err := ensureRepoInitialized(manager); err != nil {
			return err
		}

		repoLock, err := manager.AcquireRepoReadLock(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to acquire repository read lock at %s: %w", manager.RepoLockPath(), err)
		}
		defer func() {
			_ = repoLock.Unlock()
		}()

		if err := maybeHoldAfterRepoLock(cmd.Context(), "describe"); err != nil {
			return err
		}

		// Collect all matches from all arguments
		var allMatches []string
		seen := make(map[string]bool)
		for _, pat := range args {
			matches, err := ExpandPattern(manager, pat)
			if err != nil {
				return err
			}
			if len(matches) == 0 {
				return fmt.Errorf("no resources found matching '%s'", pat)
			}
			for _, m := range matches {
				if !seen[m] {
					seen[m] = true
					allMatches = append(allMatches, m)
				}
			}
		}

		if len(allMatches) == 1 {
			// Single match - show detailed view
			return describeDetailedResource(manager, allMatches[0], describeFormatFlag)
		}

		// Multiple matches - show summary
		return describeResourceSummary(manager, allMatches, describeFormatFlag)
	},
}

// describeDetailedResource displays detailed information for a single resource
func describeDetailedResource(manager *repo.Manager, resourceArg string, format string) error {
	// Parse the resource argument
	resourceType, name, err := ParseResourceArg(resourceArg)
	if err != nil {
		return err
	}

	// Handle packages separately (they use different structure)
	if resourceType == resource.PackageType {
		return describePackage(manager, name, format)
	}

	// Load the resource
	res, err := manager.Get(name, resourceType)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", resourceType, err)
	}

	// Load metadata
	meta, err := manager.GetMetadata(name, resourceType)
	metadataAvailable := err == nil

	// Route to format-specific output
	switch format {
	case "json":
		return outputDescribeJSON(manager, res, resourceType, metadataAvailable, meta)
	case "yaml":
		return outputDescribeYAML(manager, res, resourceType, metadataAvailable, meta)
	case "table":
		// Display based on resource type (existing table format)
		switch resourceType {
		case resource.Skill:
			return describeSkillDetails(manager, res, metadataAvailable, meta)
		case resource.Command:
			return describeCommandDetails(manager, res, metadataAvailable, meta)
		case resource.Agent:
			return describeAgentDetails(manager, res, metadataAvailable, meta)
		default:
			return fmt.Errorf("unsupported resource type: %s", resourceType)
		}
	default:
		return fmt.Errorf("invalid format: %s", format)
	}
}

// describeSkillDetails displays detailed information for a skill
func describeSkillDetails(manager *repo.Manager, res *resource.Resource, metadataAvailable bool, meta *metadata.ResourceMetadata) error {
	// Load full skill resource for additional details
	skillPath := manager.GetPath(res.Name, resource.Skill)
	skill, err := resource.LoadSkillResource(skillPath)
	if err != nil {
		return fmt.Errorf("failed to load skill details: %w", err)
	}

	// Display information
	fmt.Printf("Skill: %s\n", res.Name)
	if res.Version != "" {
		fmt.Printf("Version: %s\n", res.Version)
	}
	fmt.Printf("Description: %s\n", res.Description)
	if res.License != "" {
		fmt.Printf("License: %s\n", res.License)
	}
	if res.Author != "" {
		fmt.Printf("Author: %s\n", res.Author)
	}

	// Display compatibility if available
	if len(skill.Compatibility) > 0 {
		fmt.Printf("Compatibility: %s\n", strings.Join(skill.Compatibility, ", "))
	}

	// Display skill structure
	features := []string{}
	if skill.HasScripts {
		features = append(features, "scripts")
	}
	if skill.HasReferences {
		features = append(features, "references")
	}
	if skill.HasAssets {
		features = append(features, "assets")
	}
	if len(features) > 0 {
		fmt.Printf("Features: %s\n", strings.Join(features, ", "))
	}

	fmt.Println()

	// Display metadata if available
	if metadataAvailable {
		fmt.Printf("Source: %s\n", meta.SourceURL)
		fmt.Printf("Source Type: %s\n", meta.SourceType)
		if meta.SourceName != "" {
			fmt.Printf("Source Name: %s\n", meta.SourceName)
		}
		fmt.Printf("First Installed: %s\n", formatTimestamp(meta.FirstInstalled))
		fmt.Printf("Last Updated: %s\n", formatTimestamp(meta.LastUpdated))
		fmt.Println()
	} else {
		fmt.Println("Metadata: Not available")
		fmt.Println()
	}

	fmt.Printf("Location: %s\n", skillPath)

	return nil
}

// printResourceHeader prints common resource fields (version, description, license, author)
func printResourceHeader(typeName string, res *resource.Resource) {
	fmt.Printf("%s: %s\n", typeName, res.Name)
	if res.Version != "" {
		fmt.Printf("Version: %s\n", res.Version)
	}
	fmt.Printf("Description: %s\n", res.Description)
	if res.License != "" {
		fmt.Printf("License: %s\n", res.License)
	}
	if res.Author != "" {
		fmt.Printf("Author: %s\n", res.Author)
	}
}

// printMetadataBlock prints metadata source info or "not available"
func printMetadataBlock(metadataAvailable bool, meta *metadata.ResourceMetadata) {
	fmt.Println()
	if metadataAvailable {
		fmt.Printf("Source: %s\n", meta.SourceURL)
		fmt.Printf("Source Type: %s\n", meta.SourceType)
		if meta.SourceName != "" {
			fmt.Printf("Source Name: %s\n", meta.SourceName)
		}
		fmt.Printf("First Installed: %s\n", formatTimestamp(meta.FirstInstalled))
		fmt.Printf("Last Updated: %s\n", formatTimestamp(meta.LastUpdated))
		fmt.Println()
	} else {
		fmt.Println("Metadata: Not available")
		fmt.Println()
	}
}

// describeCommandDetails displays detailed information for a command
func describeCommandDetails(manager *repo.Manager, res *resource.Resource, metadataAvailable bool, meta *metadata.ResourceMetadata) error { //nolint:dupl // command and agent describe share structure but have different type-specific fields
	// Load full command resource for additional details
	commandPath := manager.GetPath(res.Name, resource.Command)
	command, err := resource.LoadCommandResource(commandPath)
	if err != nil {
		return fmt.Errorf("failed to load command details: %w", err)
	}

	// Display information
	printResourceHeader("Command", res)

	// Display command-specific fields
	if command.Agent != "" {
		fmt.Printf("Agent: %s\n", command.Agent)
	}
	if command.Model != "" {
		fmt.Printf("Model: %s\n", command.Model)
	}
	if len(command.AllowedTools) > 0 {
		fmt.Printf("Allowed Tools: %s\n", strings.Join(command.AllowedTools, ", "))
	}

	printMetadataBlock(metadataAvailable, meta)
	fmt.Printf("Location: %s\n", commandPath)

	return nil
}

// describeAgentDetails displays detailed information for an agent
func describeAgentDetails(manager *repo.Manager, res *resource.Resource, metadataAvailable bool, meta *metadata.ResourceMetadata) error { //nolint:dupl // agent and command describe share structure but have different type-specific fields
	// Load full agent resource for additional details
	agentPath := manager.GetPath(res.Name, resource.Agent)
	agent, err := resource.LoadAgentResource(agentPath)
	if err != nil {
		return fmt.Errorf("failed to load agent details: %w", err)
	}

	// Display information
	printResourceHeader("Agent", res)

	// Display agent-specific fields (OpenCode format)
	if agent.Type != "" {
		fmt.Printf("Type: %s\n", agent.Type)
	}
	if agent.Instructions != "" {
		fmt.Printf("Instructions: %s\n", agent.Instructions)
	}
	if len(agent.Capabilities) > 0 {
		fmt.Printf("Capabilities: %s\n", strings.Join(agent.Capabilities, ", "))
	}

	printMetadataBlock(metadataAvailable, meta)
	fmt.Printf("Location: %s\n", agentPath)

	return nil
}

// describeResourceSummary displays a summary table for multiple resources
func describeResourceSummary(manager *repo.Manager, matches []string, format string) error {
	// Route to format-specific output
	switch format {
	case "json":
		return outputDescribeSummaryJSON(manager, matches)
	case "yaml":
		return outputDescribeSummaryYAML(manager, matches)
	case "table":
		return outputDescribeSummaryTable(manager, matches)
	default:
		return fmt.Errorf("invalid format: %s", format)
	}
}

// outputDescribeSummaryTable displays a summary table for multiple resources
func outputDescribeSummaryTable(manager *repo.Manager, matches []string) error {
	fmt.Printf("Found %d matching resources:\n\n", len(matches))

	// Display table header
	fmt.Printf("%-10s %-30s %s\n", "TYPE", "NAME", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", 80))

	// Display each resource
	for _, match := range matches {
		resourceType, name, err := ParseResourceArg(match)
		if err != nil {
			// Skip invalid matches
			continue
		}

		var desc string
		if resourceType == resource.PackageType {
			// Handle packages separately
			pkg, err := manager.GetPackage(name)
			if err != nil {
				continue
			}
			desc = pkg.Description
		} else {
			// Handle regular resources
			res, err := manager.Get(name, resourceType)
			if err != nil {
				// Skip if can't load
				continue
			}
			desc = res.Description
		}

		// Truncate description if too long
		if len(desc) > 45 {
			desc = desc[:42] + "..."
		}

		fmt.Printf("%-10s %-30s %s\n", resourceType, name, desc)
	}

	fmt.Println()
	fmt.Println("Use 'aimgr repo describe <type>/<name>' to see detailed information")

	return nil
}

// outputDescribeSummaryJSON outputs resource summary in JSON format
func outputDescribeSummaryJSON(manager *repo.Manager, matches []string) error {
	resources := []map[string]string{}

	for _, match := range matches {
		resourceType, name, err := ParseResourceArg(match)
		if err != nil {
			continue
		}

		var desc string
		if resourceType == resource.PackageType {
			// Handle packages separately
			pkg, err := manager.GetPackage(name)
			if err != nil {
				continue
			}
			desc = pkg.Description
		} else {
			// Handle regular resources
			res, err := manager.Get(name, resourceType)
			if err != nil {
				continue
			}
			desc = res.Description
		}

		resources = append(resources, map[string]string{
			"type":        string(resourceType),
			"name":        name,
			"description": desc,
		})
	}

	output := map[string]interface{}{
		"count":     len(resources),
		"resources": resources,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// outputDescribeSummaryYAML outputs resource summary in YAML format
func outputDescribeSummaryYAML(manager *repo.Manager, matches []string) error {
	resources := []map[string]string{}

	for _, match := range matches {
		resourceType, name, err := ParseResourceArg(match)
		if err != nil {
			continue
		}

		var desc string
		if resourceType == resource.PackageType {
			// Handle packages separately
			pkg, err := manager.GetPackage(name)
			if err != nil {
				continue
			}
			desc = pkg.Description
		} else {
			// Handle regular resources
			res, err := manager.Get(name, resourceType)
			if err != nil {
				continue
			}
			desc = res.Description
		}

		resources = append(resources, map[string]string{
			"type":        string(resourceType),
			"name":        name,
			"description": desc,
		})
	}

	output := map[string]interface{}{
		"count":     len(resources),
		"resources": resources,
	}

	encoder := yaml.NewEncoder(os.Stdout)
	defer func() { _ = encoder.Close() }()
	return encoder.Encode(output)
}

// describePackage handles package description (separate flow from other resources)
func describePackage(manager *repo.Manager, name string, format string) error {
	// Load the package
	packagePath := resource.GetPackagePath(name, manager.GetRepoPath())
	if _, err := os.Stat(packagePath); err != nil {
		return fmt.Errorf("package '%s' not found", name)
	}

	pkg, err := resource.LoadPackage(packagePath)
	if err != nil {
		return fmt.Errorf("failed to load package: %w", err)
	}

	// Convert to Resource format for consistency
	res := &resource.Resource{
		Name:        pkg.Name,
		Type:        resource.PackageType,
		Description: pkg.Description,
		Path:        packagePath,
	}

	// Load package metadata
	pkgMeta, pkgMetaErr := metadata.LoadPackageMetadata(name, manager.GetRepoPath())
	pkgMetadataAvailable := pkgMetaErr == nil

	// Route to format-specific output
	switch format {
	case "json":
		return outputDescribePackageJSON(manager, res, pkg, pkgMetadataAvailable, pkgMeta)
	case "yaml":
		return outputDescribePackageYAML(manager, res, pkg, pkgMetadataAvailable, pkgMeta)
	case "table":
		return describePackageDetails(manager, res, pkgMetadataAvailable, pkgMeta)
	default:
		return fmt.Errorf("invalid format: %s", format)
	}
}

// describePackageDetails displays detailed information for a package
func describePackageDetails(manager *repo.Manager, res *resource.Resource, metadataAvailable bool, meta *metadata.PackageMetadata) error {
	// Load full package resource for additional details
	packagePath := resource.GetPackagePath(res.Name, manager.GetRepoPath())
	pkg, err := resource.LoadPackage(packagePath)
	if err != nil {
		return fmt.Errorf("failed to load package details: %w", err)
	}

	// Display information
	fmt.Printf("Package: %s\n", res.Name)
	fmt.Printf("Description: %s\n", res.Description)
	fmt.Printf("Resource Count: %d\n", len(pkg.Resources))

	// Display resources list
	if len(pkg.Resources) > 0 {
		fmt.Println("\nResources:")
		for _, resRef := range pkg.Resources {
			fmt.Printf("  - %s\n", resRef)
		}
	}

	fmt.Println()

	// Display metadata if available
	if metadataAvailable {
		fmt.Printf("Source: %s\n", meta.SourceURL)
		fmt.Printf("Source Type: %s\n", meta.SourceType)
		if meta.SourceName != "" {
			fmt.Printf("Source Name: %s\n", meta.SourceName)
		}
		if meta.SourceRef != "" {
			fmt.Printf("Source Ref: %s\n", meta.SourceRef)
		}
		fmt.Printf("First Added: %s\n", formatTimestamp(meta.FirstAdded))
		fmt.Printf("Last Updated: %s\n", formatTimestamp(meta.LastUpdated))
		if meta.OriginalFormat != "" {
			fmt.Printf("Original Format: %s\n", meta.OriginalFormat)
		}
		fmt.Println()
	} else {
		fmt.Println("Metadata: Not available")
		fmt.Println()
	}

	fmt.Printf("Location: %s\n", packagePath)

	return nil
}

// outputDescribePackageJSON outputs package details in JSON format
func outputDescribePackageJSON(manager *repo.Manager, res *resource.Resource, pkg *resource.Package, metadataAvailable bool, meta *metadata.PackageMetadata) error {
	output := &DescribeResourceOutput{
		Type:        string(resource.PackageType),
		Name:        res.Name,
		Description: res.Description,
		Location:    resource.GetPackagePath(res.Name, manager.GetRepoPath()),
	}

	resourceCount := len(pkg.Resources)
	output.ResourceCount = &resourceCount
	output.Resources = pkg.Resources

	if metadataAvailable {
		output.PackageMetadata = meta
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// outputDescribePackageYAML outputs package details in YAML format
func outputDescribePackageYAML(manager *repo.Manager, res *resource.Resource, pkg *resource.Package, metadataAvailable bool, meta *metadata.PackageMetadata) error {
	output := &DescribeResourceOutput{
		Type:        string(resource.PackageType),
		Name:        res.Name,
		Description: res.Description,
		Location:    resource.GetPackagePath(res.Name, manager.GetRepoPath()),
	}

	resourceCount := len(pkg.Resources)
	output.ResourceCount = &resourceCount
	output.Resources = pkg.Resources

	if metadataAvailable {
		output.PackageMetadata = meta
	}

	encoder := yaml.NewEncoder(os.Stdout)
	defer func() { _ = encoder.Close() }()
	return encoder.Encode(output)
}

// formatTimestamp formats a timestamp in a human-readable format
func formatTimestamp(t time.Time) string {
	// Format: "Jan 2, 2006 at 3:04pm (MST)"
	return t.Format("Jan 2, 2006 at 3:04pm (MST)")
}

// outputDescribeJSON outputs resource details in JSON format
func outputDescribeJSON(manager *repo.Manager, res *resource.Resource, resourceType resource.ResourceType, metadataAvailable bool, meta *metadata.ResourceMetadata) error {
	output, err := buildDescribeOutput(manager, res, resourceType, metadataAvailable, meta)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// outputDescribeYAML outputs resource details in YAML format
func outputDescribeYAML(manager *repo.Manager, res *resource.Resource, resourceType resource.ResourceType, metadataAvailable bool, meta *metadata.ResourceMetadata) error {
	output, err := buildDescribeOutput(manager, res, resourceType, metadataAvailable, meta)
	if err != nil {
		return err
	}

	encoder := yaml.NewEncoder(os.Stdout)
	defer func() { _ = encoder.Close() }()
	return encoder.Encode(output)
}

// buildDescribeOutput creates a DescribeResourceOutput struct from resource data
func buildDescribeOutput(manager *repo.Manager, res *resource.Resource, resourceType resource.ResourceType, metadataAvailable bool, meta *metadata.ResourceMetadata) (*DescribeResourceOutput, error) {
	output := &DescribeResourceOutput{
		Type:        string(resourceType),
		Name:        res.Name,
		Description: res.Description,
		Version:     res.Version,
		Author:      res.Author,
		License:     res.License,
		Location:    manager.GetPath(res.Name, resourceType),
	}

	// Add metadata if available
	if metadataAvailable {
		output.Metadata = meta
	}

	// Add type-specific fields
	switch resourceType {
	case resource.Skill:
		skillPath := manager.GetPath(res.Name, resource.Skill)
		skill, err := resource.LoadSkillResource(skillPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load skill details: %w", err)
		}
		output.Compatibility = skill.Compatibility
		if skill.HasScripts {
			output.HasScripts = &skill.HasScripts
		}
		if skill.HasReferences {
			output.HasReferences = &skill.HasReferences
		}
		if skill.HasAssets {
			output.HasAssets = &skill.HasAssets
		}

	case resource.Command:
		commandPath := manager.GetPath(res.Name, resource.Command)
		command, err := resource.LoadCommandResource(commandPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load command details: %w", err)
		}
		output.Agent = command.Agent
		output.Model = command.Model
		output.AllowedTools = command.AllowedTools

	case resource.Agent:
		agentPath := manager.GetPath(res.Name, resource.Agent)
		agent, err := resource.LoadAgentResource(agentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent details: %w", err)
		}
		output.AgentType = agent.Type
		output.Instructions = agent.Instructions
		output.Capabilities = agent.Capabilities

	case resource.PackageType:
		packagePath := resource.GetPackagePath(res.Name, manager.GetRepoPath())
		pkg, err := resource.LoadPackage(packagePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load package details: %w", err)
		}
		resourceCount := len(pkg.Resources)
		output.ResourceCount = &resourceCount
		output.Resources = pkg.Resources
		// Load package metadata
		pkgMeta, pkgMetaErr := metadata.LoadPackageMetadata(res.Name, manager.GetRepoPath())
		if pkgMetaErr == nil {
			output.PackageMetadata = pkgMeta
		}
	}

	return output, nil
}

func init() {
	repoCmd.AddCommand(repoDescribeCmd)
	repoDescribeCmd.Flags().StringVar(&describeFormatFlag, "format", "table", "Output format (table|json|yaml)")
	_ = repoDescribeCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)
}
