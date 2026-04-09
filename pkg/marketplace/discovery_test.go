package marketplace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverMarketplace(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) string
		subpath       string
		expectFound   bool
		expectError   bool
		expectedPath  string // Relative to base path
		checkFileName bool   // Whether to check the file name
	}{
		{
			name: "parses direct marketplace file path",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				marketplaceFile := filepath.Join(dir, "marketplace.json")
				content := `{
					"name": "direct-file-marketplace",
					"description": "Direct file",
					"plugins": []
				}`
				if err := os.WriteFile(marketplaceFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return marketplaceFile
			},
			expectFound:   true,
			expectedPath:  "",
			checkFileName: false,
		},
		{
			name: "parses marketplace file path via subpath",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				if err := os.MkdirAll(filepath.Join(dir, "nested"), 0755); err != nil {
					t.Fatal(err)
				}
				marketplaceFile := filepath.Join(dir, "nested", "marketplace.json")
				content := `{
					"name": "subpath-file-marketplace",
					"description": "Subpath file",
					"plugins": []
				}`
				if err := os.WriteFile(marketplaceFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			subpath:       filepath.Join("nested", "marketplace.json"),
			expectFound:   true,
			expectedPath:  "",
			checkFileName: false,
		},
		{
			name: "finds marketplace.json in .claude-plugin directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				claudeDir := filepath.Join(dir, ".claude-plugin")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					t.Fatal(err)
				}
				marketplaceFile := filepath.Join(claudeDir, "marketplace.json")
				content := `{
					"name": "test-marketplace",
					"description": "Test marketplace",
					"plugins": []
				}`
				if err := os.WriteFile(marketplaceFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			expectFound:   true,
			expectedPath:  ".claude-plugin/marketplace.json",
			checkFileName: true,
		},
		{
			name: "finds marketplace.json in .opencode-plugin directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				opencodePluginDir := filepath.Join(dir, ".opencode-plugin")
				if err := os.MkdirAll(opencodePluginDir, 0755); err != nil {
					t.Fatal(err)
				}
				marketplaceFile := filepath.Join(opencodePluginDir, "marketplace.json")
				content := `{
					"name": "test-marketplace",
					"description": "Test marketplace",
					"plugins": []
				}`
				if err := os.WriteFile(marketplaceFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			expectFound:   true,
			expectedPath:  ".opencode-plugin/marketplace.json",
			checkFileName: true,
		},
		{
			name: "finds marketplace.json in root directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				marketplaceFile := filepath.Join(dir, "marketplace.json")
				content := `{
					"name": "test-marketplace",
					"description": "Test marketplace",
					"plugins": []
				}`
				if err := os.WriteFile(marketplaceFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			expectFound:   true,
			expectedPath:  "marketplace.json",
			checkFileName: true,
		},
		{
			name: "finds marketplace.json in .opencode directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				opencodeDir := filepath.Join(dir, ".opencode")
				if err := os.MkdirAll(opencodeDir, 0755); err != nil {
					t.Fatal(err)
				}
				marketplaceFile := filepath.Join(opencodeDir, "marketplace.json")
				content := `{
					"name": "test-marketplace",
					"description": "Test marketplace",
					"plugins": []
				}`
				if err := os.WriteFile(marketplaceFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			expectFound:   true,
			expectedPath:  ".opencode/marketplace.json",
			checkFileName: true,
		},
		{
			name: "prioritizes .opencode-plugin over root",
			setup: func(t *testing.T) string {
				dir := t.TempDir()

				opencodePluginDir := filepath.Join(dir, ".opencode-plugin")
				if err := os.MkdirAll(opencodePluginDir, 0755); err != nil {
					t.Fatal(err)
				}
				opencodeFile := filepath.Join(opencodePluginDir, "marketplace.json")
				content1 := `{
					"name": "opencode-plugin-marketplace",
					"description": "Opencode plugin marketplace",
					"plugins": []
				}`
				if err := os.WriteFile(opencodeFile, []byte(content1), 0644); err != nil {
					t.Fatal(err)
				}

				rootFile := filepath.Join(dir, "marketplace.json")
				content2 := `{
					"name": "root-marketplace",
					"description": "Root marketplace",
					"plugins": []
				}`
				if err := os.WriteFile(rootFile, []byte(content2), 0644); err != nil {
					t.Fatal(err)
				}

				return dir
			},
			expectFound:   true,
			expectedPath:  ".opencode-plugin/marketplace.json",
			checkFileName: true,
		},
		{
			name: "prioritizes .claude-plugin over root",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				// Create both .claude-plugin and root marketplace.json
				claudeDir := filepath.Join(dir, ".claude-plugin")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					t.Fatal(err)
				}
				claudeFile := filepath.Join(claudeDir, "marketplace.json")
				content1 := `{
					"name": "claude-marketplace",
					"description": "Claude marketplace",
					"plugins": []
				}`
				if err := os.WriteFile(claudeFile, []byte(content1), 0644); err != nil {
					t.Fatal(err)
				}

				rootFile := filepath.Join(dir, "marketplace.json")
				content2 := `{
					"name": "root-marketplace",
					"description": "Root marketplace",
					"plugins": []
				}`
				if err := os.WriteFile(rootFile, []byte(content2), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			expectFound:   true,
			expectedPath:  ".claude-plugin/marketplace.json",
			checkFileName: true,
		},
		{
			name: "returns nil for non-marketplace file path",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				readme := filepath.Join(dir, "README.md")
				if err := os.WriteFile(readme, []byte("readme"), 0644); err != nil {
					t.Fatal(err)
				}
				return readme
			},
			expectFound: false,
		},
		{
			name: "returns nil when no marketplace.json found",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				// Create some other files but no marketplace.json
				if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme"), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			expectFound: false,
		},
		{
			name: "returns error for invalid marketplace.json",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				marketplaceFile := filepath.Join(dir, "marketplace.json")
				// Invalid JSON
				content := `{invalid json}`
				if err := os.WriteFile(marketplaceFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			expectFound: true,
			expectError: true,
		},
		{
			name: "returns error for marketplace.json missing required fields",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				marketplaceFile := filepath.Join(dir, "marketplace.json")
				// Missing description field
				content := `{
					"name": "test-marketplace",
					"plugins": []
				}`
				if err := os.WriteFile(marketplaceFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			expectFound: true,
			expectError: false,
		},
		{
			name: "works with subpath",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				subdir := filepath.Join(dir, "subdir")
				claudeDir := filepath.Join(subdir, ".claude-plugin")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					t.Fatal(err)
				}
				marketplaceFile := filepath.Join(claudeDir, "marketplace.json")
				content := `{
					"name": "test-marketplace",
					"description": "Test marketplace",
					"plugins": []
				}`
				if err := os.WriteFile(marketplaceFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			subpath:       "subdir",
			expectFound:   true,
			expectedPath:  ".claude-plugin/marketplace.json",
			checkFileName: true,
		},
		{
			name: "returns nil for non-existent path",
			setup: func(t *testing.T) string {
				return "/non/existent/path"
			},
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			basePath := tt.setup(t)

			config, marketplacePath, err := DiscoverMarketplace(basePath, tt.subpath)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check if marketplace was found
			if tt.expectFound {
				if config == nil {
					t.Fatal("Expected marketplace config but got nil")
				}
				if marketplacePath == "" {
					t.Fatal("Expected marketplace path but got empty string")
				}

				// Check file name if requested
				if tt.checkFileName {
					expectedFullPath := filepath.Join(basePath, tt.subpath, tt.expectedPath)
					if marketplacePath != expectedFullPath {
						t.Errorf("Expected path %s, got %s", expectedFullPath, marketplacePath)
					}
				}
			} else {
				if config != nil {
					t.Errorf("Expected no marketplace config but got: %+v", config)
				}
				if marketplacePath != "" {
					t.Errorf("Expected empty marketplace path but got: %s", marketplacePath)
				}
			}
		})
	}
}
