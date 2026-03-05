package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
)

// TestPrintDiscoveryErrors_Deduplication verifies that duplicate errors for the same path are deduplicated
func TestPrintDiscoveryErrors_Deduplication(t *testing.T) {
	tests := []struct {
		name            string
		errors          []discovery.DiscoveryError
		expectedCount   int
		expectedPaths   []string
		shouldContain   []string
		shouldNotRepeat bool // Should not contain duplicates
	}{
		{
			name: "duplicate errors for same path",
			errors: []discovery.DiscoveryError{
				{Path: "/path/to/skills/opencode-coder", Error: fmt.Errorf("YAML parse error")},
				{Path: "/path/to/skills/opencode-coder", Error: fmt.Errorf("YAML parse error")},
			},
			expectedCount:   1,
			expectedPaths:   []string{"skills/opencode-coder"},
			shouldContain:   []string{"Discovery Issues (1)", "skills/opencode-coder", "YAML parse error"},
			shouldNotRepeat: true,
		},
		{
			name: "different errors for different paths",
			errors: []discovery.DiscoveryError{
				{Path: "/path/to/skills/skill-a", Error: fmt.Errorf("error A")},
				{Path: "/path/to/skills/skill-b", Error: fmt.Errorf("error B")},
			},
			expectedCount: 2,
			expectedPaths: []string{"skill-a", "skill-b"},
			shouldContain: []string{"Discovery Issues (2)", "skill-a", "error A", "skill-b", "error B"},
		},
		{
			name: "multiple duplicates mixed with unique errors",
			errors: []discovery.DiscoveryError{
				{Path: "/path/to/skills/skill-a", Error: fmt.Errorf("error A")},
				{Path: "/path/to/skills/skill-a", Error: fmt.Errorf("error A duplicate")},
				{Path: "/path/to/skills/skill-b", Error: fmt.Errorf("error B")},
				{Path: "/path/to/skills/skill-a", Error: fmt.Errorf("error A third")},
			},
			expectedCount: 2,
			expectedPaths: []string{"skill-a", "skill-b"},
			shouldContain: []string{"Discovery Issues (2)", "skill-a", "skill-b"},
		},
		{
			name:          "no errors",
			errors:        []discovery.DiscoveryError{},
			expectedCount: 0,
			expectedPaths: []string{},
			shouldContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Call the function
			printDiscoveryErrors(tt.errors)

			// Restore stdout and read captured output
			_ = w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			// If no errors, output should be empty
			if tt.expectedCount == 0 {
				if output != "" {
					t.Errorf("Expected no output for empty errors, got: %s", output)
				}
				return
			}

			// Verify all expected strings are present
			for _, expected := range tt.shouldContain {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", expected, output)
				}
			}

			// Verify count in the header
			expectedHeader := fmt.Sprintf("Discovery Issues (%d)", tt.expectedCount)
			if !strings.Contains(output, expectedHeader) {
				t.Errorf("Expected header %q, but output was:\n%s", expectedHeader, output)
			}

			// Check for duplicates if specified
			if tt.shouldNotRepeat {
				// For the duplicate test case, verify the path appears only once in error list
				for _, path := range tt.expectedPaths {
					// Count occurrences of "✗ <path>"
					marker := fmt.Sprintf("✗ %s", path)
					count := strings.Count(output, marker)
					if count != 1 {
						t.Errorf("Expected path %q to appear exactly once as error, but appeared %d times.\nOutput:\n%s", path, count, output)
					}
				}
			}
		})
	}
}

// TestPrintDiscoveryErrors_OutputFormat verifies the output format is correct
func TestPrintDiscoveryErrors_OutputFormat(t *testing.T) {
	errors := []discovery.DiscoveryError{
		{Path: "/home/user/project/skills/test-skill", Error: fmt.Errorf("validation failed")},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printDiscoveryErrors(errors)

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify structure
	expectedElements := []string{
		"⚠ Discovery Issues (1):",
		"✗",
		"test-skill",
		"Error: validation failed",
		"Tip: These resources were skipped due to validation errors.",
	}

	for _, elem := range expectedElements {
		if !strings.Contains(output, elem) {
			t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", elem, output)
		}
	}
}
