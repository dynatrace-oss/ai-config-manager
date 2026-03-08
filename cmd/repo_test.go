package cmd

import (
	"strings"
	"testing"
)

func TestRepoCommand(t *testing.T) {
	// Verify command metadata
	if repoCmd.Use != "repo" {
		t.Errorf("Expected Use 'repo', got %s", repoCmd.Use)
	}

	if repoCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if repoCmd.Long == "" {
		t.Error("Long description should not be empty")
	}
}

func TestRepoCommandHasSubcommands(t *testing.T) {
	// Verify repo command has expected subcommands
	expectedSubcommands := []string{
		"add",
		"apply-manifest",
		"remove",
		"init",
		"show-manifest",
		"verify",
		"sync",
		"drop",
		"prune",
		"describe",
		"info",
	}

	for _, expectedCmd := range expectedSubcommands {
		found := false
		for _, cmd := range repoCmd.Commands() {
			if cmd.Name() == expectedCmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected subcommand %s not found", expectedCmd)
		}
	}
}

func TestRepoCommandDocumentation(t *testing.T) {
	// Verify long description mentions key concepts
	expectedMentions := []string{"repository", "resources"}

	for _, mention := range expectedMentions {
		if !strings.Contains(repoCmd.Long, mention) {
			t.Errorf("Long description should mention '%s'", mention)
		}
	}
}

func TestRepoCommandInitialization(t *testing.T) {
	// Verify repo command is added to root command
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "repo" {
			found = true
			break
		}
	}

	if !found {
		t.Error("repo command should be added to root command")
	}
}
