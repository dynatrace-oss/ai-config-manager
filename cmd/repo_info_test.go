package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/sourcemetadata"
)

func TestFormatTimeSince(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "zero time",
			time:     time.Time{},
			expected: "never",
		},
		{
			name:     "just now",
			time:     time.Now().Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "minutes ago",
			time:     time.Now().Add(-5 * time.Minute),
			expected: "5m ago",
		},
		{
			name:     "hours ago",
			time:     time.Now().Add(-2 * time.Hour),
			expected: "2h ago",
		},
		{
			name:     "days ago",
			time:     time.Now().Add(-3 * 24 * time.Hour),
			expected: "3d ago",
		},
		{
			name:     "weeks ago",
			time:     time.Now().Add(-14 * 24 * time.Hour),
			expected: "2w ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimeSince(tt.time)
			if result != tt.expected {
				t.Errorf("formatTimeSince() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckSourceHealth(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	existingPath := filepath.Join(tempDir, "existing")
	if err := os.Mkdir(existingPath, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name     string
		source   *repomanifest.Source
		expected bool
	}{
		{
			name: "existing local path",
			source: &repomanifest.Source{
				Name: "test-local",
				Path: existingPath,
			},
			expected: true,
		},
		{
			name: "non-existing local path",
			source: &repomanifest.Source{
				Name: "test-missing",
				Path: filepath.Join(tempDir, "nonexistent"),
			},
			expected: false,
		},
		{
			name: "remote URL (always healthy)",
			source: &repomanifest.Source{
				Name: "test-remote",
				URL:  "https://github.com/user/repo",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkSourceHealth(tt.source)
			if result != tt.expected {
				t.Errorf("checkSourceHealth() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatSource(t *testing.T) {
	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)

	// Create metadata with sync time for first test
	metadataWithSync := &sourcemetadata.SourceMetadata{
		Version: 1,
		Sources: map[string]*sourcemetadata.SourceState{
			"my-local-commands": {
				LastSynced: twoHoursAgo,
			},
		},
	}

	// Create empty metadata for second test
	emptyMetadata := &sourcemetadata.SourceMetadata{
		Version: 1,
		Sources: make(map[string]*sourcemetadata.SourceState),
	}

	tests := []struct {
		name     string
		source   *repomanifest.Source
		metadata *sourcemetadata.SourceMetadata
		contains []string // Substrings that should be present
	}{
		{
			name: "local source with sync time",
			source: &repomanifest.Source{
				Name: "my-local-commands",
				Path: "/home/user/resources",
			},
			metadata: metadataWithSync,
			contains: []string{
				"my-local-commands",
				"local:",
				"/home/user/resources",
				"[symlink]",
				"ago",
			},
		},
		{
			name: "remote source never synced",
			source: &repomanifest.Source{
				Name: "agentskills-catalog",
				URL:  "https://github.com/agentskills/catalog",
			},
			metadata: emptyMetadata,
			contains: []string{
				"agentskills-catalog",
				"remote:",
				"https://github.com/agentskills/catalog",
				"[copy]",
				"never",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSource(tt.source, tt.metadata)
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("formatSource() result missing %q\nGot: %s", substr, result)
				}
			}
		})
	}
}
