package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all installed resources from the current project",
	Long: `Remove all installed resources (commands, skills, agents) from the current project.

This command removes ALL symlinks in tool directories (.claude, .opencode, etc.)
that point to the aimgr repository. It does NOT modify ai.package.yaml, so you
can reinstall everything with 'aimgr install'.

This is useful for:
  - Starting fresh after repository path changes
  - Cleaning up orphaned or broken symlinks
  - Troubleshooting installation issues
  - Resetting CI/CD environments

WARNING: This will remove all aimgr-managed resources from the project.
You will be prompted to confirm before removal.

Examples:
  aimgr clean                    # Remove all resources (with confirmation)
  aimgr clean --yes              # Skip confirmation prompt
  aimgr clean --project-path ~/myproject
  
  # Common workflow: clean and reinstall
  aimgr clean --yes && aimgr install
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get project path
		projectPath := cleanProjectPath
		if projectPath == "" {
			var err error
			projectPath, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Get repo manager
		manager, err := NewManagerWithLogLevel()
		if err != nil {
			return err
		}

		// Detect tools
		detectedTools, err := tools.DetectExistingTools(projectPath)
		if err != nil {
			return fmt.Errorf("failed to detect tools: %w", err)
		}

		if len(detectedTools) == 0 {
			fmt.Println("No tool directories found in this project.")
			return nil
		}

		// Preview what will be removed
		resources, err := previewClean(projectPath, detectedTools, manager.GetRepoPath())
		if err != nil {
			return fmt.Errorf("failed to scan resources: %w", err)
		}

		if len(resources) == 0 {
			fmt.Println("No aimgr-managed resources found to clean.")
			return nil
		}

		// Show what will be removed
		fmt.Printf("Found %d aimgr-managed resource(s) to remove:\n\n", len(resources))
		displayCleanPreview(resources)
		fmt.Println()

		// Confirm unless --yes flag
		if !cleanYesFlag {
			fmt.Print("Remove all these resources? [y/N]: ")
			var response string
			_, _ = fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Clean cancelled.")
				return nil
			}
		}

		// Remove all resources
		removed, failed := cleanAll(projectPath, detectedTools, manager.GetRepoPath())

		// Report results
		fmt.Printf("\n✓ Removed %d resource(s)\n", removed)
		if failed > 0 {
			fmt.Printf("✗ Failed to remove %d resource(s)\n", failed)
		}
		fmt.Println("\nTo reinstall resources from ai.package.yaml, run:")
		fmt.Println("  aimgr install")

		return nil
	},
}

type CleanResource struct {
	Name string
	Type string
	Tool string
	Path string
}

func previewClean(projectPath string, detectedTools []tools.Tool, repoPath string) ([]CleanResource, error) {
	var resources []CleanResource

	for _, tool := range detectedTools {
		toolInfo := tools.GetToolInfo(tool)
		toolName := tool.String()

		// Scan commands
		if toolInfo.SupportsCommands {
			commandsDir := filepath.Join(projectPath, toolInfo.CommandsDir)
			found, err := scanDirectory(commandsDir, "command", toolName, repoPath)
			if err != nil {
				return nil, err
			}
			resources = append(resources, found...)
		}

		// Scan skills
		if toolInfo.SupportsSkills {
			skillsDir := filepath.Join(projectPath, toolInfo.SkillsDir)
			found, err := scanDirectory(skillsDir, "skill", toolName, repoPath)
			if err != nil {
				return nil, err
			}
			resources = append(resources, found...)
		}

		// Scan agents
		if toolInfo.SupportsAgents {
			agentsDir := filepath.Join(projectPath, toolInfo.AgentsDir)
			found, err := scanDirectory(agentsDir, "agent", toolName, repoPath)
			if err != nil {
				return nil, err
			}
			resources = append(resources, found...)
		}
	}

	return resources, nil
}

func scanDirectory(dir, resType, tool, repoPath string) ([]CleanResource, error) {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var resources []CleanResource
	for _, entry := range entries {
		symlinkPath := filepath.Join(dir, entry.Name())

		// Check if it's a symlink
		linkInfo, err := os.Lstat(symlinkPath)
		if err != nil {
			continue
		}

		if linkInfo.Mode()&os.ModeSymlink == 0 {
			continue // Not a symlink
		}

		// Read target
		target, err := os.Readlink(symlinkPath)
		if err != nil {
			continue
		}

		// Only include if points to repo
		if !strings.HasPrefix(target, repoPath) {
			continue
		}

		resources = append(resources, CleanResource{
			Name: entry.Name(),
			Type: resType,
			Tool: tool,
			Path: symlinkPath,
		})
	}

	return resources, nil
}

func displayCleanPreview(resources []CleanResource) {
	table := output.NewTable("Resource", "Type", "Tool", "Path")
	table.WithResponsive().
		WithDynamicColumn(3).
		WithMinColumnWidths(20, 10, 12, 40)

	for _, res := range resources {
		table.AddRow(res.Name, res.Type, res.Tool, res.Path)
	}

	_ = table.Format(output.Table)
}

func cleanAll(projectPath string, detectedTools []tools.Tool, repoPath string) (int, int) {
	removed := 0
	failed := 0

	for _, tool := range detectedTools {
		toolInfo := tools.GetToolInfo(tool)

		// Clean commands
		if toolInfo.SupportsCommands {
			commandsDir := filepath.Join(projectPath, toolInfo.CommandsDir)
			r, f := cleanDirectory(commandsDir, repoPath)
			removed += r
			failed += f
		}

		// Clean skills
		if toolInfo.SupportsSkills {
			skillsDir := filepath.Join(projectPath, toolInfo.SkillsDir)
			r, f := cleanDirectory(skillsDir, repoPath)
			removed += r
			failed += f
		}

		// Clean agents
		if toolInfo.SupportsAgents {
			agentsDir := filepath.Join(projectPath, toolInfo.AgentsDir)
			r, f := cleanDirectory(agentsDir, repoPath)
			removed += r
			failed += f
		}
	}

	return removed, failed
}

func cleanDirectory(dir, repoPath string) (int, int) {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0, 0
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}

	removed := 0
	failed := 0

	for _, entry := range entries {
		symlinkPath := filepath.Join(dir, entry.Name())

		// Check if it's a symlink
		linkInfo, err := os.Lstat(symlinkPath)
		if err != nil {
			failed++
			continue
		}

		if linkInfo.Mode()&os.ModeSymlink == 0 {
			continue // Not a symlink
		}

		// Read target
		target, err := os.Readlink(symlinkPath)
		if err != nil {
			failed++
			continue
		}

		// Only remove if points to repo
		if !strings.HasPrefix(target, repoPath) {
			continue
		}

		// Remove symlink
		if err := os.Remove(symlinkPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to remove %s: %v\n", symlinkPath, err)
			failed++
		} else {
			removed++
		}
	}

	return removed, failed
}

var (
	cleanProjectPath string
	cleanYesFlag     bool
)

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().StringVar(&cleanProjectPath, "project-path", "", "Project directory path (default: current directory)")
	cleanCmd.Flags().BoolVarP(&cleanYesFlag, "yes", "y", false, "Skip confirmation prompt")
}
