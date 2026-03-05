package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var formatFlag string
var sourceFilterFlag string

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list [pattern]",
	Short: "List resources in the repository",
	Long: `List all resources in the aimgr repository, optionally filtered by pattern.

The list command shows resources available in the global repository:
  - NAME: Resource reference (e.g., skill/pdf-processing, command/test)
  - SOURCE: Source name the resource came from (e.g., ai-tools, anthropics-skills)
  - DESCRIPTION: Brief description of the resource

This is a pure repository view showing what resources are available.
The output is identical regardless of current directory.

For installation status and sync information, use 'aimgr list' instead.

Patterns support wildcards (* for multiple characters, ? for single character) 
and optional type prefixes.

Examples:
  aimgr repo list                    # List all resources and packages
  aimgr repo list skill/*            # List all skills
  aimgr repo list command/test*      # List commands starting with "test"
  aimgr repo list package/*          # List all packages
  aimgr repo list *pdf*              # List all resources with "pdf" in name
  aimgr repo list --format=json      # Output as JSON with full details
  aimgr repo list --format=yaml      # Output as YAML
  aimgr repo list --source ai-tools  # Filter by source name
  aimgr repo list "skill/*" --source ai-tools  # Combine pattern and source filter

Output Format Examples:
  Table format (default):
  ┌──────────────────────┬──────────────────┬────────────────────────────────────────┐
  │         NAME         │      SOURCE      │              DESCRIPTION               │
  ├──────────────────────┼──────────────────┼────────────────────────────────────────┤
  │ skill/skill-creator  │ ai-tools         │ Guide for creating effective skills... │
  │ skill/webapp-testing │ anthropics-skills│ Toolkit for interacting with webapps.. │
  └──────────────────────┴──────────────────┴────────────────────────────────────────┘

  JSON format includes full resource details for scripting:
  {
    "resources": [
      {
        "type": "skill",
        "name": "skill-creator",
        "description": "Guide for creating effective skills"
      }
    ]
  }

See also:
  aimgr list              # List installed resources with targets and sync status
  aimgr install <resource>  # Install a resource
  aimgr uninstall <resource>  # Uninstall a resource`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create manager
		manager, err := NewManagerWithLogLevel()
		if err != nil {
			return err
		}

		// Handle pattern-based filtering
		if len(args) > 0 {
			// Parse pattern to check if it's a package pattern
			resourceType, _, _ := pattern.ParsePattern(args[0])

			// Special handling for package patterns
			if resourceType == resource.PackageType {
				packages, err := manager.ListPackages()
				if err != nil {
					return fmt.Errorf("failed to list packages: %w", err)
				}

				// Create matcher for filtering
				matcher, err := pattern.NewMatcher(args[0])
				if err != nil {
					return fmt.Errorf("invalid pattern '%s': %w", args[0], err)
				}

				// Filter packages by pattern
				var filteredPackages []repo.PackageInfo
				for _, pkg := range packages {
					// Create a temporary resource for matching
					tempRes := resource.Resource{
						Type: resource.PackageType,
						Name: pkg.Name,
					}
					if matcher.Match(&tempRes) {
						filteredPackages = append(filteredPackages, pkg)
					}
				}

				if len(filteredPackages) == 0 {
					fmt.Printf("No packages matching pattern '%s' found in repository.\n", args[0])
					return nil
				}

				// Format output based on --format flag
				switch formatFlag {
				case "json":
					return outputPackagesJSON(filteredPackages)
				case "yaml":
					return outputPackagesYAML(filteredPackages)
				case "table":
					return outputPackagesTable(filteredPackages)
				default:
					return fmt.Errorf("invalid format: %s (must be 'table', 'json', or 'yaml')", formatFlag)
				}
			}

			// Handle non-package resource patterns
			matcher, err := pattern.NewMatcher(args[0])
			if err != nil {
				return fmt.Errorf("invalid pattern '%s': %w", args[0], err)
			}

			// Get resource type filter if pattern specifies it (performance optimization)
			var typeFilter *resource.ResourceType
			if resourceType != "" && resourceType != resource.PackageType {
				typeFilter = &resourceType
			}

			// List resources with optional type filter
			resources, err := manager.List(typeFilter)
			if err != nil {
				return fmt.Errorf("failed to list resources: %w", err)
			}

			// Apply pattern matching
			var filtered []resource.Resource
			for _, res := range resources {
				if matcher.Match(&res) {
					filtered = append(filtered, res)
				}
			}

			// Apply source filter if specified
			if sourceFilterFlag != "" {
				var sourceFiltered []resource.Resource
				for _, res := range filtered {
					meta, err := manager.GetMetadata(res.Name, res.Type)
					if err == nil && meta.SourceName == sourceFilterFlag {
						sourceFiltered = append(sourceFiltered, res)
					}
				}
				filtered = sourceFiltered
			}

			// Handle empty results
			if len(filtered) == 0 {
				if sourceFilterFlag != "" {
					fmt.Printf("No resources matching pattern '%s' from source '%s' found in repository.\n", args[0], sourceFilterFlag)
				} else {
					fmt.Printf("No resources matching pattern '%s' found in repository.\n", args[0])
				}
				return nil
			}

			// Format output (no packages when pattern is used for resources)
			switch formatFlag {
			case "json":
				return outputWithPackagesJSON(filtered, nil)
			case "yaml":
				return outputWithPackagesYAML(filtered, nil)
			case "table":
				return outputWithPackagesTable(manager, filtered, nil)
			default:
				return fmt.Errorf("invalid format: %s (must be 'table', 'json', or 'yaml')", formatFlag)
			}
		}

		// No pattern provided - list everything
		resources, err := manager.List(nil)
		if err != nil {
			return fmt.Errorf("failed to list resources: %w", err)
		}

		packages, err := manager.ListPackages()
		if err != nil {
			return fmt.Errorf("failed to list packages: %w", err)
		}

		// Apply source filter if specified
		if sourceFilterFlag != "" {
			var filtered []resource.Resource
			for _, res := range resources {
				meta, err := manager.GetMetadata(res.Name, res.Type)
				if err == nil && meta.SourceName == sourceFilterFlag {
					filtered = append(filtered, res)
				}
			}
			resources = filtered
		}

		// Handle empty results
		if len(resources) == 0 && len(packages) == 0 {
			if sourceFilterFlag != "" {
				fmt.Printf("No resources from source '%s' found in repository.\n", sourceFilterFlag)
			} else {
				fmt.Println("No resources or packages found in repository.")
				fmt.Println("\nAdd resources with: aimgr repo add command <file>, aimgr repo add skill <folder>, or aimgr repo add agent <file>")
			}
			return nil
		}

		// Format output based on --format flag
		switch formatFlag {
		case "json":
			return outputWithPackagesJSON(resources, packages)
		case "yaml":
			return outputWithPackagesYAML(resources, packages)
		case "table":
			return outputWithPackagesTable(manager, resources, packages)
		default:
			return fmt.Errorf("invalid format: %s (must be 'table', 'json', or 'yaml')", formatFlag)
		}
	},
}

func outputWithPackagesTable(manager *repo.Manager, resources []resource.Resource, packages []repo.PackageInfo) error {
	// Group resources by type
	commands := []resource.Resource{}
	skills := []resource.Resource{}
	agents := []resource.Resource{}

	for _, res := range resources {
		switch res.Type {
		case resource.Command:
			commands = append(commands, res)
		case resource.Skill:
			skills = append(skills, res)
		case resource.Agent:
			agents = append(agents, res)
		}
	}

	// Sort packages alphabetically by name
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})

	// Create table with NAME, SOURCE, and DESCRIPTION using shared infrastructure
	table := output.NewTable("Name", "Source", "Description")
	table.WithResponsive().
		WithDynamicColumn(2).           // Description stretches
		WithMinColumnWidths(30, 15, 30) // Name min=30, Source min=15, Description min=30

	// Add commands
	for _, cmd := range commands {
		meta, err := manager.GetMetadata(cmd.Name, cmd.Type)
		sourceName := "-"
		if err == nil && meta.SourceName != "" {
			sourceName = meta.SourceName
		}
		table.AddRow(fmt.Sprintf("command/%s", cmd.Name), sourceName, cmd.Description)
	}

	// Add empty row between types if commands exist and skills or agents exist
	if len(commands) > 0 && (len(skills) > 0 || len(agents) > 0) {
		table.AddSeparator()
	}

	// Add skills
	for _, skill := range skills {
		meta, err := manager.GetMetadata(skill.Name, skill.Type)
		sourceName := "-"
		if err == nil && meta.SourceName != "" {
			sourceName = meta.SourceName
		}
		table.AddRow(fmt.Sprintf("skill/%s", skill.Name), sourceName, skill.Description)
	}

	// Add empty row between types if skills exist and agents exist
	if len(skills) > 0 && len(agents) > 0 {
		table.AddSeparator()
	}

	// Add agents
	for _, agent := range agents {
		meta, err := manager.GetMetadata(agent.Name, agent.Type)
		sourceName := "-"
		if err == nil && meta.SourceName != "" {
			sourceName = meta.SourceName
		}
		table.AddRow(fmt.Sprintf("agent/%s", agent.Name), sourceName, agent.Description)
	}

	// Add empty row between agents and packages if both exist
	if len(agents) > 0 && len(packages) > 0 {
		table.AddSeparator()
	}

	// Add packages
	for _, pkg := range packages {
		meta, err := manager.GetMetadata(pkg.Name, resource.PackageType)
		sourceName := "-"
		if err == nil && meta.SourceName != "" {
			sourceName = meta.SourceName
		}
		countStr := fmt.Sprintf("%d resources", pkg.ResourceCount)
		fullDesc := fmt.Sprintf("%s %s", countStr, pkg.Description)
		table.AddRow(fmt.Sprintf("package/%s", pkg.Name), sourceName, fullDesc)
	}

	// Render the table
	return table.Format(output.Table)
}

func outputPackagesTable(packages []repo.PackageInfo) error {
	// Sort packages alphabetically by name
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})

	// Create table using shared infrastructure
	table := output.NewTable("Name", "Resources", "Description")
	table.WithResponsive().
		WithDynamicColumn(2).          // Description stretches
		WithMinColumnWidths(40, 9, 30) // Name min=40, Resources min=9, Description min=30

	for _, pkg := range packages {
		table.AddRow(pkg.Name, fmt.Sprintf("%d", pkg.ResourceCount), pkg.Description)
	}

	return table.Format(output.Table)
}

func outputWithPackagesJSON(resources []resource.Resource, packages []repo.PackageInfo) error {
	output := map[string]interface{}{
		"resources": resources,
		"packages":  packages,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputPackagesJSON(packages []repo.PackageInfo) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(packages)
}

func outputWithPackagesYAML(resources []resource.Resource, packages []repo.PackageInfo) error {
	output := map[string]interface{}{
		"resources": resources,
		"packages":  packages,
	}
	encoder := yaml.NewEncoder(os.Stdout)
	defer func() { _ = encoder.Close() }()
	return encoder.Encode(output)
}

func outputPackagesYAML(packages []repo.PackageInfo) error {
	encoder := yaml.NewEncoder(os.Stdout)
	defer func() { _ = encoder.Close() }()
	return encoder.Encode(packages)
}

func outputTable(manager *repo.Manager, resources []resource.Resource) error {
	return outputWithPackagesTable(manager, resources, nil)
}

func outputJSON(resources []resource.Resource) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resources)
}

func outputYAML(resources []resource.Resource) error {
	encoder := yaml.NewEncoder(os.Stdout)
	defer func() { _ = encoder.Close() }()
	return encoder.Encode(resources)
}

func init() {
	repoCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&formatFlag, "format", "table", "Output format (table|json|yaml)")
	listCmd.Flags().StringVar(&sourceFilterFlag, "source", "", "Filter by source name")
	_ = listCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)
	_ = listCmd.RegisterFlagCompletionFunc("source", completeSourceNames)
}
