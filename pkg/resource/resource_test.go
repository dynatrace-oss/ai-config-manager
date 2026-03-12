package resource

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectType(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) string
		wantType     ResourceType
		wantError    bool
		errorMessage string
	}{
		{
			name: "skill directory with SKILL.md",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				skillDir := filepath.Join(tmpDir, "test-skill")
				if err := os.Mkdir(skillDir, 0755); err != nil {
					t.Fatal(err)
				}
				skillFile := filepath.Join(skillDir, "SKILL.md")
				content := "---\ndescription: Test skill\n---\n# Test Skill"
				if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return skillDir
			},
			wantType:  Skill,
			wantError: false,
		},
		{
			name: "directory without SKILL.md",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				skillDir := filepath.Join(tmpDir, "not-a-skill")
				if err := os.Mkdir(skillDir, 0755); err != nil {
					t.Fatal(err)
				}
				return skillDir
			},
			wantType:     "",
			wantError:    true,
			errorMessage: "does not contain SKILL.md",
		},
		{
			name: "agent in agents/ directory with only description",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				agentsDir := filepath.Join(tmpDir, "agents")
				if err := os.Mkdir(agentsDir, 0755); err != nil {
					t.Fatal(err)
				}
				agentFile := filepath.Join(agentsDir, "minimal-agent.md")
				content := "---\ndescription: Minimal agent\n---\n# Minimal Agent"
				if err := os.WriteFile(agentFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return agentFile
			},
			wantType:  Agent,
			wantError: false,
		},
		{
			name: "agent in agents/ directory with type field",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				agentsDir := filepath.Join(tmpDir, "agents")
				if err := os.Mkdir(agentsDir, 0755); err != nil {
					t.Fatal(err)
				}
				agentFile := filepath.Join(agentsDir, "opencode-agent.md")
				content := "---\ndescription: OpenCode agent\ntype: reviewer\n---\n# Agent"
				if err := os.WriteFile(agentFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return agentFile
			},
			wantType:  Agent,
			wantError: false,
		},
		{
			name: "command in commands/ directory",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				commandsDir := filepath.Join(tmpDir, "commands")
				if err := os.Mkdir(commandsDir, 0755); err != nil {
					t.Fatal(err)
				}
				commandFile := filepath.Join(commandsDir, "test-command.md")
				content := "---\ndescription: Test command\n---\n# Command"
				if err := os.WriteFile(commandFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return commandFile
			},
			wantType:  Command,
			wantError: false,
		},
		{
			name: "command with agent field (not in commands/ dir)",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				commandFile := filepath.Join(tmpDir, "test-command.md")
				content := "---\ndescription: Test command\nagent: build\n---\n# Command"
				if err := os.WriteFile(commandFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return commandFile
			},
			wantType:  Command,
			wantError: false,
		},
		{
			name: "command with model field (not in commands/ dir)",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				commandFile := filepath.Join(tmpDir, "test-command.md")
				content := "---\ndescription: Test command\nmodel: gpt-4\n---\n# Command"
				if err := os.WriteFile(commandFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return commandFile
			},
			wantType:  Command,
			wantError: false,
		},
		{
			name: "agent with type field (not in agents/ dir)",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				agentFile := filepath.Join(tmpDir, "test-agent.md")
				content := "---\ndescription: Test agent\ntype: reviewer\n---\n# Agent"
				if err := os.WriteFile(agentFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return agentFile
			},
			wantType:  Agent,
			wantError: false,
		},
		{
			name: "agent with instructions field (not in agents/ dir)",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				agentFile := filepath.Join(tmpDir, "test-agent.md")
				content := "---\ndescription: Test agent\ninstructions: Do something\n---\n# Agent"
				if err := os.WriteFile(agentFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return agentFile
			},
			wantType:  Agent,
			wantError: false,
		},
		{
			name: "agent with capabilities field (not in agents/ dir)",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				agentFile := filepath.Join(tmpDir, "test-agent.md")
				content := "---\ndescription: Test agent\ncapabilities:\n  - review\n  - analyze\n---\n# Agent"
				if err := os.WriteFile(agentFile, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return agentFile
			},
			wantType:  Agent,
			wantError: false,
		},
		{
			name: "minimal file with only description (no directory hint)",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				file := filepath.Join(tmpDir, "ambiguous.md")
				content := "---\ndescription: Ambiguous resource\n---\n# Ambiguous"
				if err := os.WriteFile(file, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return file
			},
			wantType:  Command, // Prefer Command for backward compatibility when ambiguous
			wantError: false,
		},
		{
			name: "file with invalid frontmatter",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				file := filepath.Join(tmpDir, "invalid.md")
				content := "---\ninvalid yaml: [[\n---\n# Invalid"
				if err := os.WriteFile(file, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return file
			},
			wantType:  Command,
			wantError: false,
		},
		{
			name: "non-markdown file",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				file := filepath.Join(tmpDir, "test.txt")
				if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
					t.Fatal(err)
				}
				return file
			},
			wantType:     "",
			wantError:    true,
			errorMessage: "not a valid resource",
		},
		{
			name: "non-existent file",
			setup: func(t *testing.T) string {
				return "/nonexistent/path/file.md"
			},
			wantType:     "",
			wantError:    true,
			errorMessage: "failed to stat path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			gotType, err := DetectType(path)

			if (err != nil) != tt.wantError {
				t.Errorf("DetectType() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if err != nil && tt.errorMessage != "" {
				if err.Error() != tt.errorMessage && !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("DetectType() error message = %v, want to contain %v", err.Error(), tt.errorMessage)
				}
			}

			if gotType != tt.wantType {
				t.Errorf("DetectType() = %v, want %v", gotType, tt.wantType)
			}
		})
	}
}

func TestDetectType_RealTestdata(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantType ResourceType
	}{
		{
			name:     "minimal agent from testdata",
			path:     "testdata/agents/minimal-agent.md",
			wantType: Agent,
		},
		{
			name:     "opencode agent from testdata",
			path:     "testdata/agents/opencode-agent.md",
			wantType: Agent,
		},
		{
			name:     "claude agent from testdata",
			path:     "testdata/agents/claude-agent.md",
			wantType: Agent,
		},
		{
			name:     "command from testdata",
			path:     "testdata/commands/test-command.md",
			wantType: Command,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, err := DetectType(tt.path)
			if err != nil {
				t.Errorf("DetectType() unexpected error = %v", err)
				return
			}
			if gotType != tt.wantType {
				t.Errorf("DetectType() = %v, want %v", gotType, tt.wantType)
			}
		})
	}
}

func TestLoad_MinimalAgent(t *testing.T) {
	// Test that Load() correctly identifies and loads a minimal agent
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	if err := os.Mkdir(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	agentFile := filepath.Join(agentsDir, "minimal-test.md")
	content := "---\ndescription: Minimal test agent\n---\n# Minimal Test Agent\n\nThis is a minimal agent."
	if err := os.WriteFile(agentFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Load(agentFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if res.Type != Agent {
		t.Errorf("Load() type = %v, want %v", res.Type, Agent)
	}
	if res.Name != "minimal-test" {
		t.Errorf("Load() name = %v, want %v", res.Name, "minimal-test")
	}
	if res.Description != "Minimal test agent" {
		t.Errorf("Load() description = %v, want %v", res.Description, "Minimal test agent")
	}
}

// TestDetectType_NestedCommands tests detection of commands in nested directories
// This test reproduces bug ai-config-manager-2tzg
func TestDetectType_NestedCommands(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantType ResourceType
	}{
		{
			name:     "flat command in commands/",
			path:     "commands/build.md",
			wantType: Command,
		},
		{
			name:     "1-level nested command",
			path:     "commands/nested/deploy.md",
			wantType: Command,
		},
		{
			name:     "2-level nested command",
			path:     "commands/api/v2/deploy.md",
			wantType: Command,
		},
		{
			name:     "3-level nested command (opencode-coder example)",
			path:     "commands/opencode-coder/api/doctor.md",
			wantType: Command,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file structure
			tmpDir := t.TempDir()
			fullPath := filepath.Join(tmpDir, tt.path)

			// Create directories
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Fatalf("Failed to create dirs: %v", err)
			}

			// Create command file
			content := "---\ndescription: Test command\n---\n# Test"
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to write file: %v", err)
			}

			// Test DetectType
			gotType, err := DetectType(fullPath)
			if err != nil {
				t.Errorf("DetectType() unexpected error = %v", err)
				return
			}

			if gotType != tt.wantType {
				t.Errorf("DetectType() = %v, want %v", gotType, tt.wantType)
			}
		})
	}
}

// TestDetectType_NestedAgents tests detection of agents in nested directories
// Agents have the same bug as commands
func TestDetectType_NestedAgents(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantType ResourceType
	}{
		{
			name:     "flat agent in agents/",
			path:     "agents/reviewer.md",
			wantType: Agent,
		},
		{
			name:     "1-level nested agent",
			path:     "agents/code/reviewer.md",
			wantType: Agent,
		},
		{
			name:     "2-level nested agent",
			path:     "agents/specialized/code/reviewer.md",
			wantType: Agent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file structure
			tmpDir := t.TempDir()
			fullPath := filepath.Join(tmpDir, tt.path)

			// Create directories
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Fatalf("Failed to create dirs: %v", err)
			}

			// Create agent file
			content := "---\ndescription: Test agent\n---\n# Test Agent"
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to write file: %v", err)
			}

			// Test DetectType
			gotType, err := DetectType(fullPath)
			if err != nil {
				t.Errorf("DetectType() unexpected error = %v", err)
				return
			}

			if gotType != tt.wantType {
				t.Errorf("DetectType() = %v, want %v", gotType, tt.wantType)
			}
		})
	}
}
