package cmd

import (
	"github.com/spf13/cobra"
)

// repoCmd represents the repo command group
var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage resources in the aimgr repository",
	Long: `Manage resources in the aimgr repository.

The repo command group provides subcommands for adding, removing, and listing
resources (commands, skills, and agents) in the centralized aimgr repository.

Manifest-oriented commands:
  - repo show-manifest reads the current local ai.repo.yaml
  - repo apply-manifest <path-or-url> merges another ai.repo.yaml into that same local file

Use 'aimgr repo --help' to see all available subcommands.`,
}

func init() {
	rootCmd.AddCommand(repoCmd)
}
