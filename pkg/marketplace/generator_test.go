package marketplace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// TestGeneratePackages tests the GeneratePackages function
func TestGeneratePackages(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) (marketplace *MarketplaceConfig, basePath string)
		wantCount    int
		wantError    bool
		errorMessage string
		validate     func(t *testing.T, packages []*PackageInfo)
	}{
		{
			name: "single plugin with command",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				tmpDir := t.TempDir()
				pluginDir := filepath.Join(tmpDir, "plugins", "test-plugin")
				commandsDir := filepath.Join(pluginDir, "commands")
				if err := os.MkdirAll(commandsDir, 0755); err != nil {
					t.Fatal(err)
				}

				// Create a command file
				commandFile := filepath.Join(commandsDir, "test-command.md")
				content := `---
description: A test command
---
# Test Command
This is a test command.
`
				if err := os.WriteFile(commandFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}

				marketplace := &MarketplaceConfig{
					Name:        "test-marketplace",
					Description: "Test marketplace",
					Plugins: []Plugin{
						{
							Name:        "Test Plugin",
							Description: "A test plugin",
							Source:      "./plugins/test-plugin",
						},
					},
				}

				return marketplace, tmpDir
			},
			wantCount: 1,
			wantError: false,
			validate: func(t *testing.T, packages []*PackageInfo) {
				pkg := packages[0].Package
				if pkg.Name != "test-plugin" {
					t.Errorf("Package name = %v, want test-plugin", pkg.Name)
				}
				if pkg.Description != "A test plugin" {
					t.Errorf("Package description = %v, want 'A test plugin'", pkg.Description)
				}
				if len(pkg.Resources) != 1 {
					t.Fatalf("Resources length = %v, want 1", len(pkg.Resources))
				}
				if pkg.Resources[0] != "command/test-command" {
					t.Errorf("Resource = %v, want command/test-command", pkg.Resources[0])
				}
			},
		},
		{
			name: "single plugin with skill",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				tmpDir := t.TempDir()
				pluginDir := filepath.Join(tmpDir, "plugins", "skill-plugin")
				skillsDir := filepath.Join(pluginDir, "skills", "test-skill")
				if err := os.MkdirAll(skillsDir, 0755); err != nil {
					t.Fatal(err)
				}

				// Create SKILL.md file
				skillFile := filepath.Join(skillsDir, "SKILL.md")
				content := `---
description: A test skill
---
# Test Skill
This is a test skill.
`
				if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}

				marketplace := &MarketplaceConfig{
					Name:        "test-marketplace",
					Description: "Test marketplace",
					Plugins: []Plugin{
						{
							Name:        "Skill Plugin",
							Description: "A skill plugin",
							Source:      "./plugins/skill-plugin",
						},
					},
				}

				return marketplace, tmpDir
			},
			wantCount: 1,
			wantError: false,
			validate: func(t *testing.T, packages []*PackageInfo) {
				if packages[0].Package.Name != "skill-plugin" {
					t.Errorf("Package name = %v, want skill-plugin", packages[0].Package.Name)
				}
				if len(packages[0].Package.Resources) != 1 {
					t.Fatalf("Resources length = %v, want 1", len(packages[0].Package.Resources))
				}
				if packages[0].Package.Resources[0] != "skill/test-skill" {
					t.Errorf("Resource = %v, want skill/test-skill", packages[0].Package.Resources[0])
				}
			},
		},
		{
			name: "single plugin with agent",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				tmpDir := t.TempDir()
				pluginDir := filepath.Join(tmpDir, "plugins", "agent-plugin")
				agentsDir := filepath.Join(pluginDir, "agents")
				if err := os.MkdirAll(agentsDir, 0755); err != nil {
					t.Fatal(err)
				}

				// Create agent file
				agentFile := filepath.Join(agentsDir, "test-agent.md")
				content := `---
description: A test agent
---
# Test Agent
This is a test agent.
`
				if err := os.WriteFile(agentFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}

				marketplace := &MarketplaceConfig{
					Name:        "test-marketplace",
					Description: "Test marketplace",
					Plugins: []Plugin{
						{
							Name:        "Agent Plugin",
							Description: "An agent plugin",
							Source:      "./plugins/agent-plugin",
						},
					},
				}

				return marketplace, tmpDir
			},
			wantCount: 1,
			wantError: false,
			validate: func(t *testing.T, packages []*PackageInfo) {
				if packages[0].Package.Name != "agent-plugin" {
					t.Errorf("Package name = %v, want agent-plugin", packages[0].Package.Name)
				}
				if len(packages[0].Package.Resources) != 1 {
					t.Fatalf("Resources length = %v, want 1", len(packages[0].Package.Resources))
				}
				if packages[0].Package.Resources[0] != "agent/test-agent" {
					t.Errorf("Resource = %v, want agent/test-agent", packages[0].Package.Resources[0])
				}
			},
		},
		{
			name: "plugin with multiple resource types",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				tmpDir := t.TempDir()
				pluginDir := filepath.Join(tmpDir, "plugins", "multi-plugin")

				// Create commands
				commandsDir := filepath.Join(pluginDir, "commands")
				if err := os.MkdirAll(commandsDir, 0755); err != nil {
					t.Fatal(err)
				}
				commandFile := filepath.Join(commandsDir, "cmd1.md")
				if err := os.WriteFile(commandFile, []byte("---\ndescription: Command 1\n---\n# Cmd1"), 0644); err != nil {
					t.Fatal(err)
				}

				// Create skills
				skillsDir := filepath.Join(pluginDir, "skills", "skill1")
				if err := os.MkdirAll(skillsDir, 0755); err != nil {
					t.Fatal(err)
				}
				skillFile := filepath.Join(skillsDir, "SKILL.md")
				if err := os.WriteFile(skillFile, []byte("---\ndescription: Skill 1\n---\n# Skill1"), 0644); err != nil {
					t.Fatal(err)
				}

				// Create agents
				agentsDir := filepath.Join(pluginDir, "agents")
				if err := os.MkdirAll(agentsDir, 0755); err != nil {
					t.Fatal(err)
				}
				agentFile := filepath.Join(agentsDir, "agent1.md")
				if err := os.WriteFile(agentFile, []byte("---\ndescription: Agent 1\n---\n# Agent1"), 0644); err != nil {
					t.Fatal(err)
				}

				marketplace := &MarketplaceConfig{
					Name:        "test-marketplace",
					Description: "Test marketplace",
					Plugins: []Plugin{
						{
							Name:        "Multi Plugin",
							Description: "Plugin with all resource types",
							Source:      "./plugins/multi-plugin",
						},
					},
				}

				return marketplace, tmpDir
			},
			wantCount: 1,
			wantError: false,
			validate: func(t *testing.T, packages []*PackageInfo) {
				if len(packages[0].Package.Resources) != 3 {
					t.Errorf("Resources length = %v, want 3", len(packages[0].Package.Resources))
				}
				// Check that we have one of each type
				hasCommand := false
				hasSkill := false
				hasAgent := false
				for _, res := range packages[0].Package.Resources {
					if strings.HasPrefix(res, "command/") {
						hasCommand = true
					}
					if strings.HasPrefix(res, "skill/") {
						hasSkill = true
					}
					if strings.HasPrefix(res, "agent/") {
						hasAgent = true
					}
				}
				if !hasCommand || !hasSkill || !hasAgent {
					t.Errorf("Missing resource types: command=%v, skill=%v, agent=%v", hasCommand, hasSkill, hasAgent)
				}
			},
		},
		{
			name: "multiple plugins",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				tmpDir := t.TempDir()

				// Plugin 1
				plugin1Dir := filepath.Join(tmpDir, "plugins", "plugin1")
				commands1Dir := filepath.Join(plugin1Dir, "commands")
				if err := os.MkdirAll(commands1Dir, 0755); err != nil {
					t.Fatal(err)
				}
				cmd1File := filepath.Join(commands1Dir, "cmd1.md")
				if err := os.WriteFile(cmd1File, []byte("---\ndescription: Command 1\n---\n# Cmd1"), 0644); err != nil {
					t.Fatal(err)
				}

				// Plugin 2
				plugin2Dir := filepath.Join(tmpDir, "plugins", "plugin2")
				commands2Dir := filepath.Join(plugin2Dir, "commands")
				if err := os.MkdirAll(commands2Dir, 0755); err != nil {
					t.Fatal(err)
				}
				cmd2File := filepath.Join(commands2Dir, "cmd2.md")
				if err := os.WriteFile(cmd2File, []byte("---\ndescription: Command 2\n---\n# Cmd2"), 0644); err != nil {
					t.Fatal(err)
				}

				marketplace := &MarketplaceConfig{
					Name:        "test-marketplace",
					Description: "Test marketplace",
					Plugins: []Plugin{
						{
							Name:        "Plugin 1",
							Description: "First plugin",
							Source:      "./plugins/plugin1",
						},
						{
							Name:        "Plugin 2",
							Description: "Second plugin",
							Source:      "./plugins/plugin2",
						},
					},
				}

				return marketplace, tmpDir
			},
			wantCount: 2,
			wantError: false,
			validate: func(t *testing.T, packages []*PackageInfo) {
				if packages[0].Package.Name != "plugin-1" {
					t.Errorf("Package[0] name = %v, want plugin-1", packages[0].Package.Name)
				}
				if packages[1].Package.Name != "plugin-2" {
					t.Errorf("Package[1] name = %v, want plugin-2", packages[1].Package.Name)
				}
			},
		},
		{
			name: "plugin with no resources - should be skipped",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				tmpDir := t.TempDir()
				pluginDir := filepath.Join(tmpDir, "plugins", "empty-plugin")
				if err := os.MkdirAll(pluginDir, 0755); err != nil {
					t.Fatal(err)
				}

				marketplace := &MarketplaceConfig{
					Name:        "test-marketplace",
					Description: "Test marketplace",
					Plugins: []Plugin{
						{
							Name:        "Empty Plugin",
							Description: "Plugin with no resources",
							Source:      "./plugins/empty-plugin",
						},
					},
				}

				return marketplace, tmpDir
			},
			wantCount: 0, // Should be skipped
			wantError: false,
		},
		{
			name: "plugin with missing source directory - should be skipped",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				tmpDir := t.TempDir()

				marketplace := &MarketplaceConfig{
					Name:        "test-marketplace",
					Description: "Test marketplace",
					Plugins: []Plugin{
						{
							Name:        "Missing Plugin",
							Description: "Plugin with missing source",
							Source:      "./plugins/nonexistent",
						},
					},
				}

				return marketplace, tmpDir
			},
			wantCount: 0, // Should be skipped
			wantError: false,
		},
		{
			name: "plugin with source as file not directory",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				tmpDir := t.TempDir()
				pluginsDir := filepath.Join(tmpDir, "plugins")
				if err := os.MkdirAll(pluginsDir, 0755); err != nil {
					t.Fatal(err)
				}

				// Create a file instead of directory
				filePath := filepath.Join(pluginsDir, "not-a-dir.txt")
				if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
					t.Fatal(err)
				}

				marketplace := &MarketplaceConfig{
					Name:        "test-marketplace",
					Description: "Test marketplace",
					Plugins: []Plugin{
						{
							Name:        "File Plugin",
							Description: "Plugin source is a file",
							Source:      "./plugins/not-a-dir.txt",
						},
					},
				}

				return marketplace, tmpDir
			},
			wantCount:    0,
			wantError:    true,
			errorMessage: "source is not a directory",
		},
		{
			name: "absolute path source",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				tmpDir := t.TempDir()
				pluginDir := filepath.Join(tmpDir, "somewhere", "plugin")
				commandsDir := filepath.Join(pluginDir, "commands")
				if err := os.MkdirAll(commandsDir, 0755); err != nil {
					t.Fatal(err)
				}

				commandFile := filepath.Join(commandsDir, "cmd.md")
				if err := os.WriteFile(commandFile, []byte("---\ndescription: Command\n---\n# Cmd"), 0644); err != nil {
					t.Fatal(err)
				}

				marketplace := &MarketplaceConfig{
					Name:        "test-marketplace",
					Description: "Test marketplace",
					Plugins: []Plugin{
						{
							Name:        "Absolute Plugin",
							Description: "Plugin with absolute path",
							Source:      pluginDir, // Absolute path
						},
					},
				}

				return marketplace, tmpDir
			},
			wantCount: 1,
			wantError: false,
			validate: func(t *testing.T, packages []*PackageInfo) {
				if len(packages[0].Package.Resources) != 1 {
					t.Errorf("Resources length = %v, want 1", len(packages[0].Package.Resources))
				}
			},
		},
		{
			name: "nil marketplace config",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				return nil, t.TempDir()
			},
			wantCount:    0,
			wantError:    true,
			errorMessage: "marketplace config cannot be nil",
		},
		{
			name: "empty basePath",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				marketplace := &MarketplaceConfig{
					Name:        "test-marketplace",
					Description: "Test marketplace",
					Plugins:     []Plugin{},
				}
				return marketplace, ""
			},
			wantCount:    0,
			wantError:    true,
			errorMessage: "basePath cannot be empty",
		},
		{
			name: "basePath does not exist",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				marketplace := &MarketplaceConfig{
					Name:        "test-marketplace",
					Description: "Test marketplace",
					Plugins:     []Plugin{},
				}
				return marketplace, "/nonexistent/path"
			},
			wantCount:    0,
			wantError:    true,
			errorMessage: "basePath does not exist",
		},
		{
			name: "basePath is a file not directory",
			setup: func(t *testing.T) (*MarketplaceConfig, string) {
				tmpDir := t.TempDir()
				filePath := filepath.Join(tmpDir, "not-a-dir.txt")
				if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
					t.Fatal(err)
				}

				marketplace := &MarketplaceConfig{
					Name:        "test-marketplace",
					Description: "Test marketplace",
					Plugins:     []Plugin{},
				}
				return marketplace, filePath
			},
			wantCount:    0,
			wantError:    true,
			errorMessage: "basePath is not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marketplace, basePath := tt.setup(t)
			packages, err := GeneratePackages(marketplace, basePath)

			// Check error expectation
			if (err != nil) != tt.wantError {
				t.Errorf("GeneratePackages() error = %v, wantError %v", err, tt.wantError)
				return
			}

			// Check error message if error expected
			if err != nil && tt.errorMessage != "" {
				if !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("GeneratePackages() error message = %v, want to contain %v", err.Error(), tt.errorMessage)
				}
				return
			}

			// Check package count
			if len(packages) != tt.wantCount {
				t.Errorf("GeneratePackages() returned %d packages, want %d", len(packages), tt.wantCount)
				return
			}

			// Run validation if provided
			if tt.validate != nil {
				tt.validate(t, packages)
			}
		})
	}
}

// TestSanitizeName tests the sanitizeName function
func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "lowercase alphanumeric",
			input: "testplugin",
			want:  "testplugin",
		},
		{
			name:  "with hyphens",
			input: "test-plugin",
			want:  "test-plugin",
		},
		{
			name:  "uppercase to lowercase",
			input: "TestPlugin",
			want:  "testplugin",
		},
		{
			name:  "spaces to hyphens",
			input: "Test Plugin",
			want:  "test-plugin",
		},
		{
			name:  "underscores to hyphens",
			input: "test_plugin",
			want:  "test-plugin",
		},
		{
			name:  "mixed spaces and underscores",
			input: "Test Plugin_Name",
			want:  "test-plugin-name",
		},
		{
			name:  "remove invalid characters",
			input: "test@plugin#123",
			want:  "testplugin123",
		},
		{
			name:  "consecutive hyphens",
			input: "test--plugin",
			want:  "test-plugin",
		},
		{
			name:  "leading hyphen",
			input: "-test",
			want:  "test",
		},
		{
			name:  "trailing hyphen",
			input: "test-",
			want:  "test",
		},
		{
			name:  "leading and trailing hyphens",
			input: "-test-",
			want:  "test",
		},
		{
			name:  "special characters",
			input: "test!@#$%^&*()plugin",
			want:  "testplugin",
		},
		{
			name:  "dots and slashes",
			input: "test.plugin/name",
			want:  "testpluginname",
		},
		{
			name:  "complex sanitization",
			input: "My   Test__Plugin--123",
			want:  "my-test-plugin-123",
		},
		{
			name:  "very long name - truncate to 64",
			input: "this-is-a-very-long-plugin-name-that-exceeds-the-maximum-length-of-64-characters-and-should-be-truncated",
			want:  "this-is-a-very-long-plugin-name-that-exceeds-the-maximum-length",
		},
		{
			name:  "truncation removes trailing hyphen",
			input: "this-is-a-very-long-plugin-name-that-exceeds-the-maximum-length-and-ends-with-hyphen-",
			want:  "this-is-a-very-long-plugin-name-that-exceeds-the-maximum-length",
		},
		{
			name:  "empty after sanitization",
			input: "@#$%",
			want:  "",
		},
		{
			name:  "only hyphens",
			input: "---",
			want:  "",
		},
		{
			name:  "unicode characters",
			input: "test-플러그인",
			want:  "test",
		},
		{
			name:  "numbers only",
			input: "12345",
			want:  "12345",
		},
		{
			name:  "alphanumeric with version",
			input: "plugin-v2-beta1",
			want:  "plugin-v2-beta1",
		},
		{
			name:  "camelCase",
			input: "testPluginName",
			want:  "testpluginname",
		},
		{
			name:  "PascalCase with spaces",
			input: "Test Plugin Name",
			want:  "test-plugin-name",
		},
		{
			name:  "mixed everything",
			input: "  My__Test--Plugin  123  ",
			want:  "my-test-plugin-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestBuildResourceReferences tests the buildResourceReferences function
func TestBuildResourceReferences(t *testing.T) {
	tests := []struct {
		name      string
		resources []*resource.Resource
		want      []string
	}{
		{
			name:      "empty resources",
			resources: []*resource.Resource{},
			want:      []string{},
		},
		{
			name: "single command",
			resources: []*resource.Resource{
				{Type: resource.Command, Name: "test"},
			},
			want: []string{"command/test"},
		},
		{
			name: "single skill",
			resources: []*resource.Resource{
				{Type: resource.Skill, Name: "pdf"},
			},
			want: []string{"skill/pdf"},
		},
		{
			name: "single agent",
			resources: []*resource.Resource{
				{Type: resource.Agent, Name: "reviewer"},
			},
			want: []string{"agent/reviewer"},
		},
		{
			name: "multiple resources",
			resources: []*resource.Resource{
				{Type: resource.Command, Name: "test"},
				{Type: resource.Skill, Name: "pdf"},
				{Type: resource.Agent, Name: "reviewer"},
			},
			want: []string{"command/test", "skill/pdf", "agent/reviewer"},
		},
		{
			name: "multiple of same type",
			resources: []*resource.Resource{
				{Type: resource.Command, Name: "cmd1"},
				{Type: resource.Command, Name: "cmd2"},
				{Type: resource.Command, Name: "cmd3"},
			},
			want: []string{"command/cmd1", "command/cmd2", "command/cmd3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildResourceReferences(tt.resources)
			if len(got) != len(tt.want) {
				t.Fatalf("buildResourceReferences() returned %d refs, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("buildResourceReferences()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestGeneratePackages_InvalidPluginName tests that invalid plugin names cause errors
func TestGeneratePackages_InvalidPluginName(t *testing.T) {
	tests := []struct {
		name         string
		pluginName   string
		wantError    bool
		errorMessage string
	}{
		{
			name:       "valid name",
			pluginName: "valid-plugin",
			wantError:  false,
		},
		{
			name:         "sanitizes to empty",
			pluginName:   "@#$%",
			wantError:    true,
			errorMessage: "cannot be sanitized to valid aimgr name",
		},
		{
			name:         "only special characters",
			pluginName:   "!@#$%^&*()",
			wantError:    true,
			errorMessage: "cannot be sanitized to valid aimgr name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			pluginDir := filepath.Join(tmpDir, "plugins", "test")
			commandsDir := filepath.Join(pluginDir, "commands")
			if err := os.MkdirAll(commandsDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Create a command so the plugin isn't skipped for having no resources
			commandFile := filepath.Join(commandsDir, "cmd.md")
			if err := os.WriteFile(commandFile, []byte("---\ndescription: Cmd\n---\n# Cmd"), 0644); err != nil {
				t.Fatal(err)
			}

			marketplace := &MarketplaceConfig{
				Name:        "test-marketplace",
				Description: "Test marketplace",
				Plugins: []Plugin{
					{
						Name:        tt.pluginName,
						Description: "Test plugin",
						Source:      "./plugins/test",
					},
				},
			}

			_, err := GeneratePackages(marketplace, tmpDir)

			if (err != nil) != tt.wantError {
				t.Errorf("GeneratePackages() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if err != nil && tt.errorMessage != "" {
				if !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("GeneratePackages() error message = %v, want to contain %v", err.Error(), tt.errorMessage)
				}
			}
		})
	}
}

// TestGeneratePackages_Integration tests full workflow with real files
func TestGeneratePackages_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a realistic plugin structure
	plugin1Dir := filepath.Join(tmpDir, "plugins", "web-tools")

	// Commands
	commandsDir := filepath.Join(plugin1Dir, "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatal(err)
	}
	buildCmd := filepath.Join(commandsDir, "build.md")
	if err := os.WriteFile(buildCmd, []byte("---\ndescription: Build the project\n---\n# Build"), 0644); err != nil {
		t.Fatal(err)
	}
	devCmd := filepath.Join(commandsDir, "dev.md")
	if err := os.WriteFile(devCmd, []byte("---\ndescription: Start dev server\n---\n# Dev"), 0644); err != nil {
		t.Fatal(err)
	}

	// Skills
	skillsDir := filepath.Join(plugin1Dir, "skills", "typescript-helper")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillFile := filepath.Join(skillsDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("---\ndescription: TypeScript helper\n---\n# TS Helper"), 0644); err != nil {
		t.Fatal(err)
	}

	// Agents
	agentsDir := filepath.Join(plugin1Dir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	agentFile := filepath.Join(agentsDir, "code-reviewer.md")
	if err := os.WriteFile(agentFile, []byte("---\ndescription: Code reviewer agent\n---\n# Reviewer"), 0644); err != nil {
		t.Fatal(err)
	}

	marketplace := &MarketplaceConfig{
		Name:        "web-dev-marketplace",
		Version:     "1.0.0",
		Description: "Web development tools",
		Plugins: []Plugin{
			{
				Name:        "Web Tools",
				Description: "Collection of web development tools",
				Source:      "./plugins/web-tools",
				Category:    "development",
			},
		},
	}

	packages, err := GeneratePackages(marketplace, tmpDir)
	if err != nil {
		t.Fatalf("GeneratePackages() error = %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(packages))
	}

	pkgInfo := packages[0]
	pkg := pkgInfo.Package
	if pkg.Name != "web-tools" {
		t.Errorf("Package name = %v, want web-tools", pkg.Name)
	}
	if pkg.Description != "Collection of web development tools" {
		t.Errorf("Package description = %v, want 'Collection of web development tools'", pkg.Description)
	}
	if len(pkg.Resources) != 4 {
		t.Fatalf("Expected 4 resources, got %d: %v", len(pkg.Resources), pkg.Resources)
	}

	// Check that all resources are present
	expectedResources := map[string]bool{
		"command/build":           false,
		"command/dev":             false,
		"skill/typescript-helper": false,
		"agent/code-reviewer":     false,
	}

	for _, res := range pkg.Resources {
		if _, ok := expectedResources[res]; ok {
			expectedResources[res] = true
		} else {
			t.Errorf("Unexpected resource: %v", res)
		}
	}

	for res, found := range expectedResources {
		if !found {
			t.Errorf("Missing expected resource: %v", res)
		}
	}
}
