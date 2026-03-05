package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// statusIconOK is the checkmark icon displayed for healthy/in-sync resources.
const statusIconOK = "✓"

// statusIconFail is the cross icon displayed for broken/unhealthy resources.
const statusIconFail = "✗"

// parseResourceArg parses a resource argument in the format "type/name"
// Returns the resource type, name, and any error
func parseResourceArg(arg string) (resource.ResourceType, string, error) {
	parts := strings.SplitN(arg, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid format: must be 'type/name' (e.g., skill/my-skill, command/my-command, agent/my-agent)")
	}

	typeStr := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])

	if name == "" {
		return "", "", fmt.Errorf("resource name cannot be empty")
	}

	// Normalize type (support plural forms)
	var resourceType resource.ResourceType
	switch strings.ToLower(typeStr) {
	case "skill", "skills":
		resourceType = resource.Skill
	case "command", "commands":
		resourceType = resource.Command
	case "agent", "agents":
		resourceType = resource.Agent
	default:
		return "", "", fmt.Errorf("invalid resource type '%s': must be one of 'skill', 'command', or 'agent'", typeStr)
	}

	return resourceType, name, nil
}

// completionOptions controls behavior of the base resource completion function
type completionOptions struct {
	includePackages bool // Include "package/" prefix in type suggestions
	multiArg        bool // Support completing multiple resource arguments
}

// completeResourcesWithOptions is the base completion function for resource arguments
// Provides shell completion for resource arguments in "type/name" format
// Supports completing both the type prefix and resource names after the slash
func completeResourcesWithOptions(opts completionOptions) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// For single-arg commands, stop suggesting after first argument
		if !opts.multiArg && len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		manager, err := NewManagerWithLogLevel()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// If toComplete doesn't contain a slash, suggest type prefixes
		if !strings.Contains(toComplete, "/") {
			prefixes := []string{"skill/", "command/", "agent/"}
			if opts.includePackages {
				prefixes = append(prefixes, "package/")
			}
			var matches []string
			for _, prefix := range prefixes {
				if strings.HasPrefix(prefix, toComplete) {
					matches = append(matches, prefix)
				}
			}
			return matches, cobra.ShellCompDirectiveNoSpace
		}

		// Parse the type prefix
		parts := strings.SplitN(toComplete, "/", 2)
		if len(parts) != 2 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		typeStr := parts[0]
		namePrefix := parts[1]

		// Determine resource type
		var resourceType resource.ResourceType
		switch strings.ToLower(typeStr) {
		case "skill", "skills":
			resourceType = resource.Skill
		case "command", "commands":
			resourceType = resource.Command
		case "agent", "agents":
			resourceType = resource.Agent
		case "package", "packages":
			if opts.includePackages {
				resourceType = resource.PackageType
			} else {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Get resources of that type
		resources, err := manager.List(&resourceType)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// Build completions with type prefix
		var suggestions []string
		for _, res := range resources {
			if strings.HasPrefix(res.Name, namePrefix) {
				suggestions = append(suggestions, typeStr+"/"+res.Name)
			}
		}

		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeResourceArgs provides shell completion for resource arguments in "type/name" format
// Supports completing both the type prefix and resource names after the slash
// Used by commands that accept a single resource without package support
func completeResourceArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeResourcesWithOptions(completionOptions{
		includePackages: false,
		multiArg:        false,
	})(cmd, args, toComplete)
}

// completeResourceNames provides completion for resource names of the given type from the repository
func completeResourceNames(resType resource.ResourceType) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		manager, err := NewManagerWithLogLevel()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		t := resType
		resources, err := manager.List(&t)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var names []string
		for _, res := range resources {
			names = append(names, res.Name)
		}

		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeCommandNames provides completion for command names from the repository
func completeCommandNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeResourceNames(resource.Command)(cmd, args, toComplete)
}

// completeSkillNames provides completion for skill names from the repository
func completeSkillNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeResourceNames(resource.Skill)(cmd, args, toComplete)
}

// completeAgentNames provides completion for agent names from the repository
func completeAgentNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeResourceNames(resource.Agent)(cmd, args, toComplete)
}

// completeInstallResources provides shell completion for install command with type prefix support
// Handles both old-style subcommands and new type/name format
// Supports multiple resource arguments and package completion
func completeInstallResources(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeResourcesWithOptions(completionOptions{
		includePackages: true,
		multiArg:        true,
	})(cmd, args, toComplete)
}

// completeUninstallResources provides shell completion for uninstall command with type prefix support
// Same as completeInstallResources but for uninstall (could be merged if logic is identical)
func completeUninstallResources(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// For now, reuse the same logic as install
	return completeInstallResources(cmd, args, toComplete)
}

// completeResourceTypes provides completion for resource type arguments
func completeResourceTypes(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	types := []string{"command", "skill", "agent", "package"}
	return types, cobra.ShellCompDirectiveNoFileComp
}

// completeInstalledResources provides completion for installed resource patterns
func completeInstalledResources(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Offer type-based patterns and common wildcards
	patterns := []string{
		"skill/*",
		"command/*",
		"agent/*",
		"*",
	}
	return patterns, cobra.ShellCompDirectiveNoFileComp
}

// completeVerifyPatterns provides completion for repo verify command patterns
func completeVerifyPatterns(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Offer type-based patterns including packages
	patterns := []string{
		"skill/*",
		"command/*",
		"agent/*",
		"package/*",
		"*",
	}
	return patterns, cobra.ShellCompDirectiveNoFileComp
}

// completeConfigKeys provides completion for config key names
func completeConfigKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	keys := []string{"install.targets"}
	return keys, cobra.ShellCompDirectiveNoFileComp
}

// completeConfigSetArgs provides completion for config set command (key and value)
func completeConfigSetArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		// Complete config keys
		return []string{"install.targets"}, cobra.ShellCompDirectiveNoFileComp
	} else if len(args) == 1 && args[0] == "install.targets" {
		// Complete tool names for install.targets
		return []string{"claude", "opencode", "copilot", "windsurf"}, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// completeToolNames provides completion for tool names (claude, opencode, copilot, windsurf)
func completeToolNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"claude", "opencode", "copilot", "windsurf"}, cobra.ShellCompDirectiveNoFileComp
}

// completeFormatFlag provides completion for --format flag values
func completeFormatFlag(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"table", "json", "yaml"}, cobra.ShellCompDirectiveNoFileComp
}

// completeSourceNames provides completion for source names from ai.repo.yaml
func completeSourceNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	manager, err := NewManagerWithLogLevel()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// Load manifest
	manifest, err := repomanifest.Load(manager.GetRepoPath())
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// Extract source names
	var names []string
	for _, source := range manifest.Sources {
		if strings.HasPrefix(source.Name, toComplete) {
			names = append(names, source.Name)
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}
