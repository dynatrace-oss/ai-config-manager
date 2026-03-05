package test

import (
	"os/exec"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/source"
)

// getRefOrDefault returns the ref from parsed source or "main" as default
func getRefOrDefault(parsed *source.ParsedSource) string {
	if parsed.Ref != "" {
		return parsed.Ref
	}
	return "main"
}

// isGitAvailable checks if git is available in PATH
func isGitAvailable() bool {
	cmd := exec.Command("git", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
