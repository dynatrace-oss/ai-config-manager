package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/sourcemetadata"
	"github.com/spf13/cobra"
)

var repoInfoFormatFlag string

// repoInfoCmd represents the repo info command
var repoInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display repository information and statistics",
	Long: `Display comprehensive information about the aimgr repository.

Shows repository location, total resource counts, breakdown by type,
and disk usage statistics.

Output Formats:
  --format=table (default): Human-readable text
  --format=json:  JSON for scripting
  --format=yaml:  YAML for configuration

Examples:
  aimgr repo info
  aimgr repo info --format=json
  aimgr repo info --format=yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create a new repo manager
		manager, err := NewManagerWithLogLevel()
		if err != nil {
			return err
		}

		// Log the repo info operation
		logger := manager.GetLogger()
		if logger != nil {
			logger.Debug("repo info")
		}

		// Get repository path
		repoPath := manager.GetRepoPath()

		// Check if repository exists
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			fmt.Println("Repository not initialized")
			fmt.Printf("Expected location: %s\n", repoPath)
			fmt.Println()
			fmt.Println("Run 'aimgr repo add <resource>' to add resources and initialize the repository.")
			return nil
		}

		// List all resources to get counts
		allResources, err := manager.List(nil)
		if err != nil {
			return fmt.Errorf("failed to list resources: %w", err)
		}

		// Count by type
		commandCount := 0
		skillCount := 0
		agentCount := 0

		for _, res := range allResources {
			switch res.Type {
			case resource.Command:
				commandCount++
			case resource.Skill:
				skillCount++
			case resource.Agent:
				agentCount++
			}
		}

		// Validate format
		parsedFormat, err := output.ParseFormat(repoInfoFormatFlag)
		if err != nil {
			return err
		}

		// Calculate disk usage
		size, _ := calculateDirSize(repoPath)

		// Load manifest to get sources
		manifest, err := repomanifest.Load(repoPath)
		if err != nil {
			return fmt.Errorf("failed to load manifest: %w", err)
		}

		// Load source metadata for timestamps
		metadata, err := sourcemetadata.Load(repoPath)
		if err != nil {
			// If metadata doesn't exist yet, create empty one
			metadata = &sourcemetadata.SourceMetadata{
				Version: 1,
				Sources: make(map[string]*sourcemetadata.SourceState),
			}
		}

		// Build output using KeyValueBuilder
		info := output.NewKeyValue("Repository Information").
			Add("Location", repoPath).
			AddSection().
			Add("Total Resources", fmt.Sprintf("%d", len(allResources))).
			Add("  Commands", fmt.Sprintf("%d", commandCount)).
			Add("  Skills", fmt.Sprintf("%d", skillCount)).
			Add("  Agents", fmt.Sprintf("%d", agentCount))

		// Add disk usage if calculated successfully
		if size > 0 {
			info.AddSection().Add("Disk Usage", formatBytes(size))
		}

		// Add sources count (but not the details - we'll render table separately for table format)
		if manifest != nil && len(manifest.Sources) > 0 {
			info.AddSection().Add("Sources", fmt.Sprintf("%d", len(manifest.Sources)))
		} else {
			info.AddSection().Add("Sources", "0 (use 'aimgr repo add' to add sources)")
		}

		// Format key-value output first
		if err := info.Format(parsedFormat); err != nil {
			return err
		}

		// For table format, render sources table after key-value section
		if parsedFormat == output.Table && manifest != nil && len(manifest.Sources) > 0 {
			fmt.Println() // Add blank line between key-value and table
			return renderSourcesTable(manifest.Sources, metadata)
		}

		return nil
	},
}

// calculateDirSize calculates the total size of a directory
func calculateDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// formatBytes formats bytes as human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// renderSourcesTable renders sources as a table
func renderSourcesTable(sources []*repomanifest.Source, metadata *sourcemetadata.SourceMetadata) error {
	// Create table with columns: NAME, TYPE, LOCATION, MODE, LAST SYNCED
	table := output.NewTable("NAME", "TYPE", "LOCATION", "MODE", "LAST SYNCED")
	table.WithResponsive().
		WithDynamicColumn(2).                  // LOCATION column stretches
		WithMinColumnWidths(20, 8, 30, 10, 12) // NAME, TYPE, LOCATION, MODE, LAST SYNCED

	// Add row for each source
	for _, source := range sources {
		// Health check
		health := checkSourceHealth(source)
		healthIcon := statusIconOK
		if !health {
			healthIcon = statusIconFail
		}

		// Determine source type and location
		sourceType := "local"
		location := source.Path
		if source.URL != "" {
			sourceType = "remote"
			location = source.URL
		}

		// Get mode from source (implicit based on path/url)
		mode := source.GetMode()

		// Format last synced time from metadata
		lastSynced := "never"
		if state, ok := metadata.Sources[source.Name]; ok && !state.LastSynced.IsZero() {
			lastSynced = formatTimeSince(state.LastSynced)
		}

		// Add row with health indicator prepended to name
		table.AddRow(
			fmt.Sprintf("%s %s", healthIcon, source.Name),
			sourceType,
			location,
			mode,
			lastSynced,
		)
	}

	// Render the table
	return table.Format(output.Table)
}

// formatSource formats a source with health indicator, type, path/URL, mode, and last synced
func formatSource(source *repomanifest.Source, metadata *sourcemetadata.SourceMetadata) string {
	// Health check
	health := checkSourceHealth(source)
	healthIcon := statusIconOK
	if !health {
		healthIcon = statusIconFail
	}

	// Determine source type and location
	sourceType := "local"
	location := source.Path
	if source.URL != "" {
		sourceType = "remote"
		location = source.URL
	}

	// Get mode from source (implicit based on path/url)
	mode := source.GetMode()

	// Format last synced time from metadata
	lastSynced := "never"
	if state, ok := metadata.Sources[source.Name]; ok && !state.LastSynced.IsZero() {
		lastSynced = formatTimeSince(state.LastSynced)
	}

	// Build the formatted line
	return fmt.Sprintf("  %s %s (%s: %s) [%s] - synced %s",
		healthIcon,
		source.Name,
		sourceType,
		location,
		mode,
		lastSynced)
}

// checkSourceHealth checks if a source is accessible
// For local sources, checks if the path exists
// For remote sources, always returns true (shows "remote")
func checkSourceHealth(source *repomanifest.Source) bool {
	if source.Path != "" {
		// Local source - check if path exists
		_, err := os.Stat(source.Path)
		return err == nil
	}
	// Remote source - always healthy (we don't do network checks)
	return true
}

// formatTimeSince formats a time duration in human-readable format
// Examples: "2h ago", "1d ago", "3w ago"
func formatTimeSince(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	duration := time.Since(t)

	// Format as human-readable duration
	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%dm ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	} else if duration < 30*24*time.Hour {
		weeks := int(duration.Hours() / 24 / 7)
		return fmt.Sprintf("%dw ago", weeks)
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / 24 / 30)
		return fmt.Sprintf("%dmo ago", months)
	} else {
		years := int(duration.Hours() / 24 / 365)
		return fmt.Sprintf("%dy ago", years)
	}
}

func init() {
	repoCmd.AddCommand(repoInfoCmd)
	repoInfoCmd.Flags().StringVar(&repoInfoFormatFlag, "format", "table", "Output format (table|json|yaml)")
	_ = repoInfoCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)
}
