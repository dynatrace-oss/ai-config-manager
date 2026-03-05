package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/spf13/cobra"
)

var (
	initYesFlag bool
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new ai.package.yaml file",
	Long: `Initialize a new ai.package.yaml file in the current directory.

This creates a project manifest that declares AI resource dependencies,
similar to npm's package.json. The manifest allows you to:
  - Declare project dependencies (commands, skills, agents, packages)
  - Share consistent AI tooling across teams
  - Install all dependencies with 'aimgr install'

The init command creates an empty ai.package.yaml file with an empty
resources array. You can then add resources by:
  - Installing resources: aimgr install skill/pdf-processing
  - Manually editing ai.package.yaml

Examples:
  # Initialize with defaults (non-interactive)
  aimgr init

  # Explicit yes flag (same as above)
  aimgr init --yes`,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	manifestPath := filepath.Join(cwd, manifest.ManifestFileName)

	// Check if ai.package.yaml already exists
	if manifest.Exists(manifestPath) {
		return fmt.Errorf("ai.package.yaml already exists in this directory")
	}

	// Create empty manifest
	m := &manifest.Manifest{
		Resources: []string{},
	}

	// Save manifest
	if err := m.Save(manifestPath); err != nil {
		return fmt.Errorf("failed to create ai.package.yaml: %w", err)
	}

	// Show success message
	fmt.Println("✓ Created ai.package.yaml")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Install resources: aimgr install skill/pdf-processing")
	fmt.Println("  2. Install all dependencies: aimgr install")
	fmt.Println("  3. Edit ai.package.yaml to add more resources")

	return nil
}

func init() {
	rootCmd.AddCommand(initCmd)

	// Add flags to init command
	initCmd.Flags().BoolVarP(&initYesFlag, "yes", "y", false, "Non-interactive mode (use defaults)")
}
