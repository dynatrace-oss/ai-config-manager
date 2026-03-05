package cmd

import (
	"fmt"
	"os"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/logging"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/version"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile     string
	versionFlag bool
	logLevel    string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "aimgr",
	Short: "Manage AI resources (commands and skills) for LLM tools",
	Long: `aimgr is a CLI tool for discovering, installing, and managing
AI resources like commands and skills across different LLM tools
(Claude Code, OpenCode, etc.).

It helps you organize and share reusable AI configurations.`,
	Run: func(cmd *cobra.Command, args []string) {
		if versionFlag {
			fmt.Println(version.GetVersion())
			os.Exit(0)
		}
		// If no version flag and no subcommand, show help
		_ = cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// GetLogLevel returns the log level flag value
func GetLogLevel() string {
	return logLevel
}

// NewManagerWithLogLevel creates a new repo manager and sets the log level from the --log-level flag.
// Returns an error if the manager cannot be created or if the log level is invalid.
func NewManagerWithLogLevel() (*repo.Manager, error) {
	manager, err := repo.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	// Parse and set log level
	level, err := logging.ParseLogLevel(logLevel)
	if err != nil {
		return nil, err
	}
	manager.SetLogLevel(level)

	// Set logger for cmd, manifest, workspace, and discovery packages
	repoLogger := manager.GetLogger()
	if repoLogger != nil {
		SetLogger(repoLogger)
		manifest.SetLogger(repoLogger)
		workspace.SetLogger(repoLogger)
		discovery.SetLogger(repoLogger)
	}

	return manager, nil
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/aimgr/aimgr.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")

	// Version flag
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Show version information")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in XDG config directory first (~/.config/aimgr/)
		configDir := home + "/.config/aimgr"
		viper.AddConfigPath(configDir)

		// Also search in home directory for backward compatibility
		viper.AddConfigPath(home)

		viper.SetConfigType("yaml")
		viper.SetConfigName("aimgr") // Will find both aimgr.yaml and .aimgr.yaml
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	_ = viper.ReadInConfig() // read in config file if available
}
