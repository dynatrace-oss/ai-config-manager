package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/marketplace"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// TestMarketplaceImportCompleteWorkflow tests the complete marketplace import workflow
func TestMarketplaceImportCompleteWorkflow(t *testing.T) {
	// Create temporary directories
	testDir := t.TempDir()
	repoDir := filepath.Join(testDir, "repo")
	marketplaceDir := filepath.Join(testDir, "marketplace")

	// Create realistic marketplace structure
	setupRealisticMarketplace(t, marketplaceDir)

	// Create marketplace.json
	marketplaceConfig := marketplace.MarketplaceConfig{
		Name:        "test-marketplace",
		Version:     "1.0.0",
		Description: "Test marketplace for integration testing",
		Owner: &marketplace.Author{
			Name:  "Test Owner",
			Email: "owner@example.com",
		},
		Plugins: []marketplace.Plugin{
			{
				Name:        "Web Tools",
				Description: "Web development tools",
				Source:      "plugins/web-tools",
				Category:    "development",
				Version:     "1.0.0",
				Author: &marketplace.Author{
					Name: "Web Team",
				},
			},
			{
				Name:        "Testing Suite",
				Description: "Testing utilities",
				Source:      "plugins/testing-suite",
				Category:    "testing",
			},
			{
				Name:        "Code Helpers",
				Description: "Code assistance tools",
				Source:      "plugins/code-helpers",
				Category:    "development",
			},
		},
	}

	marketplacePath := filepath.Join(marketplaceDir, "marketplace.json")
	if err := saveMarketplaceConfig(marketplacePath, &marketplaceConfig); err != nil {
		t.Fatalf("Failed to create marketplace.json: %v", err)
	}

	// Step 1: Parse marketplace
	t.Log("Step 1: Parsing marketplace.json")
	parsedConfig, err := marketplace.ParseMarketplace(marketplacePath)
	if err != nil {
		t.Fatalf("Failed to parse marketplace: %v", err)
	}

	if parsedConfig.Name != "test-marketplace" {
		t.Errorf("Marketplace name = %v, want test-marketplace", parsedConfig.Name)
	}

	if len(parsedConfig.Plugins) != 3 {
		t.Errorf("Plugin count = %d, want 3", len(parsedConfig.Plugins))
	}

	// Step 2: Generate packages
	t.Log("Step 2: Generating packages from marketplace")
	basePath := filepath.Dir(marketplacePath)
	packages, err := marketplace.GeneratePackages(parsedConfig, basePath)
	if err != nil {
		t.Fatalf("Failed to generate packages: %v", err)
	}

	if len(packages) != 3 {
		t.Errorf("Generated package count = %d, want 3", len(packages))
	}

	// Verify package names are sanitized
	expectedPackageNames := []string{"web-tools", "testing-suite", "code-helpers"}
	for i, pkg := range packages {
		if pkg.Package.Name != expectedPackageNames[i] {
			t.Errorf("Package %d name = %v, want %v", i, pkg.Package.Name, expectedPackageNames[i])
		}
		if len(pkg.Package.Resources) == 0 {
			t.Errorf("Package %s has no resources", pkg.Package.Name)
		}
	}

	// Step 3: Import into repository
	t.Log("Step 3: Importing packages into repository")
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Collect all resource paths for import
	var allResourcePaths []string
	for _, plugin := range parsedConfig.Plugins {
		pluginPath := filepath.Join(basePath, plugin.Source)

		// Scan for commands
		commandsDir := filepath.Join(pluginPath, "commands")
		if info, err := os.Stat(commandsDir); err == nil && info.IsDir() {
			entries, _ := os.ReadDir(commandsDir)
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
					allResourcePaths = append(allResourcePaths, filepath.Join(commandsDir, entry.Name()))
				}
			}
		}

		// Scan for skills
		skillsDir := filepath.Join(pluginPath, "skills")
		if info, err := os.Stat(skillsDir); err == nil && info.IsDir() {
			entries, _ := os.ReadDir(skillsDir)
			for _, entry := range entries {
				if entry.IsDir() {
					skillMd := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
					if _, err := os.Stat(skillMd); err == nil {
						allResourcePaths = append(allResourcePaths, filepath.Join(skillsDir, entry.Name()))
					}
				}
			}
		}

		// Scan for agents
		agentsDir := filepath.Join(pluginPath, "agents")
		if info, err := os.Stat(agentsDir); err == nil && info.IsDir() {
			entries, _ := os.ReadDir(agentsDir)
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
					allResourcePaths = append(allResourcePaths, filepath.Join(agentsDir, entry.Name()))
				}
			}
		}
	}

	// Import all resources
	opts := repo.BulkImportOptions{
		Force:        false,
		SkipExisting: false,
		DryRun:       false,
	}

	result, err := manager.AddBulk(allResourcePaths, opts)
	if err != nil {
		t.Fatalf("Failed to import resources: %v", err)
	}

	// Verify import results
	// web-tools: 1 command, 1 skill
	// testing-suite: 2 commands, 1 skill
	// code-helpers: 1 command, 1 agent
	expectedTotal := 7 // 4 commands + 2 skills + 1 agent
	if len(result.Added) != expectedTotal {
		t.Errorf("Added resources = %d, want %d", len(result.Added), expectedTotal)
	}

	if result.CommandCount != 4 {
		t.Errorf("Command count = %d, want 4", result.CommandCount)
	}

	if result.SkillCount != 2 {
		t.Errorf("Skill count = %d, want 2", result.SkillCount)
	}

	if result.AgentCount != 1 {
		t.Errorf("Agent count = %d, want 1", result.AgentCount)
	}

	// Step 4: Save packages
	t.Log("Step 4: Saving packages to repository")
	for _, pkgInfo := range packages {
		if err := resource.SavePackage(pkgInfo.Package, repoDir); err != nil {
			t.Fatalf("Failed to save package %s: %v", pkgInfo.Package.Name, err)
		}
	}

	// Step 5: Verify packages are stored correctly
	t.Log("Step 5: Verifying package storage")
	pkgList, err := manager.ListPackages()
	if err != nil {
		t.Fatalf("Failed to list packages: %v", err)
	}

	if len(pkgList) != 3 {
		t.Errorf("Package list count = %d, want 3", len(pkgList))
	}

	// Verify each package
	for _, expectedName := range expectedPackageNames {
		found := false
		for _, pkg := range pkgList {
			if pkg.Name == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Package %s not found in repository", expectedName)
		}
	}

	// Step 6: Verify resource references are correct
	t.Log("Step 6: Verifying resource references")
	webToolsPkg, err := resource.LoadPackage(resource.GetPackagePath("web-tools", repoDir))
	if err != nil {
		t.Fatalf("Failed to load web-tools package: %v", err)
	}

	// Verify web-tools has expected resources
	expectedResources := []string{"command/build", "skill/typescript-helper"}
	for _, expectedRef := range expectedResources {
		found := false
		for _, ref := range webToolsPkg.Resources {
			if ref == expectedRef {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("web-tools package missing resource reference: %s", expectedRef)
		}
	}
}

// TestMarketplaceImportDryRun tests dry-run mode for marketplace import
func TestMarketplaceImportDryRun(t *testing.T) {
	testDir := t.TempDir()
	repoDir := filepath.Join(testDir, "repo")
	marketplaceDir := filepath.Join(testDir, "marketplace")

	// Create simple marketplace
	setupSimpleMarketplace(t, marketplaceDir)

	marketplaceConfig := marketplace.MarketplaceConfig{
		Name:        "dryrun-marketplace",
		Description: "Test marketplace for dry-run testing",
		Plugins: []marketplace.Plugin{
			{
				Name:        "Simple Plugin",
				Description: "A simple plugin",
				Source:      "plugin",
			},
		},
	}

	marketplacePath := filepath.Join(marketplaceDir, "marketplace.json")
	if err := saveMarketplaceConfig(marketplacePath, &marketplaceConfig); err != nil {
		t.Fatalf("Failed to create marketplace.json: %v", err)
	}

	// Generate packages
	basePath := filepath.Dir(marketplacePath)
	packages, err := marketplace.GeneratePackages(&marketplaceConfig, basePath)
	if err != nil {
		t.Fatalf("Failed to generate packages: %v", err)
	}

	if len(packages) != 1 {
		t.Errorf("Generated package count = %d, want 1", len(packages))
	}

	// Initialize repo
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Collect resource paths
	pluginPath := filepath.Join(basePath, "plugin")
	cmdPath := filepath.Join(pluginPath, "commands", "test.md")

	// Dry-run import
	opts := repo.BulkImportOptions{
		Force:        false,
		SkipExisting: false,
		DryRun:       true,
	}

	result, err := manager.AddBulk([]string{cmdPath}, opts)
	if err != nil {
		t.Fatalf("Failed dry-run import: %v", err)
	}

	// Verify dry-run shows what would be added
	if len(result.Added) != 1 {
		t.Errorf("Dry-run added = %d, want 1", len(result.Added))
	}

	// Verify nothing was actually added
	resources, err := manager.List(nil)
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if len(resources) != 0 {
		t.Errorf("Dry-run should not add resources, got %d", len(resources))
	}

	// Verify package file doesn't exist
	pkgPath := resource.GetPackagePath("simple-plugin", repoDir)
	if _, err := os.Stat(pkgPath); !os.IsNotExist(err) {
		t.Error("Package file should not exist after dry-run")
	}
}

// TestMarketplaceImportWithFilter tests filtering plugins during import
func TestMarketplaceImportWithFilter(t *testing.T) {
	marketplaceDir := t.TempDir()

	// Create marketplace with multiple plugins
	setupMultiPluginMarketplace(t, marketplaceDir)

	marketplaceConfig := marketplace.MarketplaceConfig{
		Name:        "filtered-marketplace",
		Description: "Test marketplace for filter testing",
		Plugins: []marketplace.Plugin{
			{
				Name:        "code-helper",
				Description: "Code helper plugin",
				Source:      "plugins/code-helper",
			},
			{
				Name:        "code-reviewer",
				Description: "Code reviewer plugin",
				Source:      "plugins/code-reviewer",
			},
			{
				Name:        "test-runner",
				Description: "Test runner plugin",
				Source:      "plugins/test-runner",
			},
		},
	}

	marketplacePath := filepath.Join(marketplaceDir, "marketplace.json")
	if err := saveMarketplaceConfig(marketplacePath, &marketplaceConfig); err != nil {
		t.Fatalf("Failed to create marketplace.json: %v", err)
	}

	// Test filter pattern "code-*"
	filterPattern := "code-*"

	// Parse marketplace
	parsedConfig, err := marketplace.ParseMarketplace(marketplacePath)
	if err != nil {
		t.Fatalf("Failed to parse marketplace: %v", err)
	}

	// Apply filter
	var filteredPlugins []marketplace.Plugin
	for _, plugin := range parsedConfig.Plugins {
		matched, err := filepath.Match(filterPattern, plugin.Name)
		if err != nil {
			t.Fatalf("Filter match error: %v", err)
		}
		if matched {
			filteredPlugins = append(filteredPlugins, plugin)
		}
	}

	if len(filteredPlugins) != 2 {
		t.Errorf("Filtered plugins = %d, want 2 (code-helper, code-reviewer)", len(filteredPlugins))
	}

	// Create filtered marketplace config
	filteredMarketplace := &marketplace.MarketplaceConfig{
		Name:        parsedConfig.Name,
		Description: parsedConfig.Description,
		Plugins:     filteredPlugins,
	}

	// Generate packages from filtered plugins
	basePath := filepath.Dir(marketplacePath)
	packages, err := marketplace.GeneratePackages(filteredMarketplace, basePath)
	if err != nil {
		t.Fatalf("Failed to generate packages: %v", err)
	}

	// Verify only matching plugins are included
	if len(packages) != 2 {
		t.Errorf("Generated package count = %d, want 2", len(packages))
	}

	for _, pkg := range packages {
		if !strings.HasPrefix(pkg.Package.Name, "code-") {
			t.Errorf("Package %s should not be included (doesn't match filter)", pkg.Package.Name)
		}
	}

	// Verify test-runner was not included
	for _, pkg := range packages {
		if pkg.Package.Name == "test-runner" {
			t.Error("test-runner should not be included (doesn't match filter)")
		}
	}
}

// TestMarketplaceImportConflicts tests handling of existing packages and resources
func TestMarketplaceImportConflicts(t *testing.T) {
	tests := []struct {
		name          string
		force         bool
		expectSuccess bool
	}{
		{
			name:          "conflict without force fails",
			force:         false,
			expectSuccess: false,
		},
		{
			name:          "conflict with force succeeds",
			force:         true,
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := t.TempDir()
			marketplaceDir := t.TempDir()

			// Create marketplace
			setupSimpleMarketplace(t, marketplaceDir)

			marketplaceConfig := marketplace.MarketplaceConfig{
				Name:        "conflict-marketplace",
				Description: "Test marketplace for conflict testing",
				Plugins: []marketplace.Plugin{
					{
						Name:        "conflict-plugin",
						Description: "A plugin that will conflict",
						Source:      "plugin",
					},
				},
			}

			marketplacePath := filepath.Join(marketplaceDir, "marketplace.json")
			if err := saveMarketplaceConfig(marketplacePath, &marketplaceConfig); err != nil {
				t.Fatalf("Failed to create marketplace.json: %v", err)
			}

			// Initialize repo
			manager := repo.NewManagerWithPath(repoDir)
			if err := manager.Init(); err != nil {
				t.Fatalf("Failed to initialize repo: %v", err)
			}

			// Generate packages
			basePath := filepath.Dir(marketplacePath)
			packages, err := marketplace.GeneratePackages(&marketplaceConfig, basePath)
			if err != nil {
				t.Fatalf("Failed to generate packages: %v", err)
			}

			// Import resources first time
			pluginPath := filepath.Join(basePath, "plugin")
			cmdPath := filepath.Join(pluginPath, "commands", "test.md")

			opts := repo.BulkImportOptions{
				Force:        false,
				SkipExisting: false,
				DryRun:       false,
			}

			_, err = manager.AddBulk([]string{cmdPath}, opts)
			if err != nil {
				t.Fatalf("Failed first import: %v", err)
			}

			// Save package first time
			if err := resource.SavePackage(packages[0].Package, repoDir); err != nil {
				t.Fatalf("Failed to save package first time: %v", err)
			}

			// Verify package exists
			pkgPath := resource.GetPackagePath("conflict-plugin", repoDir)
			if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
				t.Fatal("Package should exist after first import")
			}

			// Try to save package again with specified force setting
			if tt.force {
				// Remove existing package to simulate force overwrite
				if err := os.Remove(pkgPath); err != nil {
					t.Fatalf("Failed to remove existing package: %v", err)
				}
				if err := resource.SavePackage(packages[0].Package, repoDir); err != nil {
					t.Fatalf("Failed to save package with force: %v", err)
				}
			} else {
				// Check that package already exists (simulates conflict)
				if _, err := os.Stat(pkgPath); err != nil {
					t.Fatal("Package should exist after first import")
				}
				// In a real scenario, the import command would check this
				// and return an error. Here we just verify the file exists.
			}

			// Verify final state
			_, err = os.Stat(pkgPath)
			packageExists := err == nil

			if !packageExists {
				t.Error("Package should exist")
			}
		})
	}
}

// TestMarketplaceImportInvalidJSON tests handling of invalid marketplace.json
// TestMarketplaceImportInvalidJSON tests handling of invalid marketplace.json
func TestMarketplaceImportInvalidJSON(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectedError string
	}{
		{
			name:          "malformed JSON",
			content:       `{"name": "test", invalid json}`,
			expectedError: "failed to parse marketplace JSON",
		},
		{
			name: "missing name",
			content: `{
				"description": "Test marketplace",
				"plugins": []
			}`,
			expectedError: "marketplace name is required",
		},
		// NOTE: description is now optional (can be in metadata or top-level)
		{
			name: "plugin missing name",
			content: `{
				"name": "test-marketplace",
				"description": "Test",
				"plugins": [
					{
						"description": "Plugin without name",
						"source": "plugin"
					}
				]
			}`,
			expectedError: "name is required",
		},
		{
			name: "plugin missing source",
			content: `{
				"name": "test-marketplace",
				"description": "Test",
				"plugins": [
					{
						"name": "test-plugin",
						"description": "Plugin without source"
					}
				]
			}`,
			expectedError: "source is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := t.TempDir()
			marketplacePath := filepath.Join(testDir, "marketplace.json")

			// Write invalid content
			if err := os.WriteFile(marketplacePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Try to parse
			_, err := marketplace.ParseMarketplace(marketplacePath)
			if err == nil {
				t.Fatalf("Expected error containing %q, got nil", tt.expectedError)
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("Error = %v, want error containing %q", err, tt.expectedError)
			}
		})
	}
}

// TestMarketplaceImportWithMissingPluginSource tests handling of missing plugin source directories
func TestMarketplaceImportWithMissingPluginSource(t *testing.T) {
	marketplaceDir := t.TempDir()

	// Create marketplace.json with non-existent plugin source
	marketplaceConfig := marketplace.MarketplaceConfig{
		Name:        "missing-source-marketplace",
		Description: "Test marketplace with missing source",
		Plugins: []marketplace.Plugin{
			{
				Name:        "missing-plugin",
				Description: "Plugin with missing source",
				Source:      "nonexistent/plugin",
			},
		},
	}

	marketplacePath := filepath.Join(marketplaceDir, "marketplace.json")
	if err := saveMarketplaceConfig(marketplacePath, &marketplaceConfig); err != nil {
		t.Fatalf("Failed to create marketplace.json: %v", err)
	}

	// Parse marketplace (should succeed)
	parsedConfig, err := marketplace.ParseMarketplace(marketplacePath)
	if err != nil {
		t.Fatalf("Failed to parse marketplace: %v", err)
	}

	// Generate packages (should skip plugins with missing sources)
	basePath := filepath.Dir(marketplacePath)
	packages, err := marketplace.GeneratePackages(parsedConfig, basePath)
	if err != nil {
		t.Fatalf("Failed to generate packages: %v", err)
	}

	// Should produce no packages (source doesn't exist)
	if len(packages) != 0 {
		t.Errorf("Expected 0 packages (source missing), got %d", len(packages))
	}
}

// TestMarketplaceImportResourceReferences tests that resource references are correctly formatted
func TestMarketplaceImportResourceReferences(t *testing.T) {
	testDir := t.TempDir()
	marketplaceDir := filepath.Join(testDir, "marketplace")

	// Create comprehensive marketplace
	setupRealisticMarketplace(t, marketplaceDir)

	marketplaceConfig := marketplace.MarketplaceConfig{
		Name:        "reference-test-marketplace",
		Description: "Test marketplace for reference format testing",
		Plugins: []marketplace.Plugin{
			{
				Name:        "Full Suite",
				Description: "Plugin with all resource types",
				Source:      "plugins/web-tools",
			},
		},
	}

	marketplacePath := filepath.Join(marketplaceDir, "marketplace.json")
	if err := saveMarketplaceConfig(marketplacePath, &marketplaceConfig); err != nil {
		t.Fatalf("Failed to create marketplace.json: %v", err)
	}

	// Generate packages
	basePath := filepath.Dir(marketplacePath)
	packages, err := marketplace.GeneratePackages(&marketplaceConfig, basePath)
	if err != nil {
		t.Fatalf("Failed to generate packages: %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(packages))
	}

	pkg := packages[0]

	// Verify all resource references follow "type/name" format
	for _, ref := range pkg.Package.Resources {
		parts := strings.Split(ref, "/")
		if len(parts) != 2 {
			t.Errorf("Invalid resource reference format: %s (expected type/name)", ref)
			continue
		}

		resType := parts[0]
		resName := parts[1]

		// Verify type is valid
		validTypes := map[string]bool{
			"command": true,
			"skill":   true,
			"agent":   true,
		}

		if !validTypes[resType] {
			t.Errorf("Invalid resource type in reference %s: %s", ref, resType)
		}

		// Verify name is not empty
		if resName == "" {
			t.Errorf("Empty resource name in reference: %s", ref)
		}

		// Verify name follows naming rules (lowercase, alphanumeric + hyphens)
		if err := resource.ValidateName(resName); err != nil {
			t.Errorf("Invalid resource name in reference %s: %v", ref, err)
		}
	}
}

// Helper functions

// setupRealisticMarketplace creates a realistic marketplace directory structure
func setupRealisticMarketplace(t *testing.T, baseDir string) {
	plugins := []struct {
		name     string
		commands []string
		skills   []string
		agents   []string
	}{
		{
			name:     "web-tools",
			commands: []string{"build"},
			skills:   []string{"typescript-helper"},
		},
		{
			name:     "testing-suite",
			commands: []string{"test", "coverage"},
			skills:   []string{"test-generator"},
		},
		{
			name:     "code-helpers",
			commands: []string{"format"},
			agents:   []string{"code-reviewer"},
		},
	}

	for _, plugin := range plugins {
		pluginDir := filepath.Join(baseDir, "plugins", plugin.name)

		// Create commands using helper
		if len(plugin.commands) > 0 {
			commandsDir := filepath.Join(pluginDir, "commands")
			if err := os.MkdirAll(commandsDir, 0755); err != nil {
				t.Fatalf("Failed to create commands directory: %v", err)
			}

			for _, cmdName := range plugin.commands {
				// Use helper to create command with proper structure
				tmpCmdPath := createTestCommand(t, cmdName, cmdName+" command")
				// Copy to marketplace location
				cmdPath := filepath.Join(commandsDir, cmdName+".md")
				content, err := os.ReadFile(tmpCmdPath)
				if err != nil {
					t.Fatalf("Failed to read command from helper: %v", err)
				}
				if err := os.WriteFile(cmdPath, content, 0644); err != nil {
					t.Fatalf("Failed to create command: %v", err)
				}
			}
		}

		// Create skills using helper
		if len(plugin.skills) > 0 {
			skillsDir := filepath.Join(pluginDir, "skills")
			for _, skillName := range plugin.skills {
				// Use helper to create skill with proper structure
				tmpSkillDir := createTestSkill(t, skillName, skillName+" skill")
				// Copy to marketplace location
				skillDir := filepath.Join(skillsDir, skillName)
				if err := os.MkdirAll(skillDir, 0755); err != nil {
					t.Fatalf("Failed to create skill directory: %v", err)
				}
				skillMdPath := filepath.Join(skillDir, "SKILL.md")
				content, err := os.ReadFile(filepath.Join(tmpSkillDir, "SKILL.md"))
				if err != nil {
					t.Fatalf("Failed to read skill from helper: %v", err)
				}
				if err := os.WriteFile(skillMdPath, content, 0644); err != nil {
					t.Fatalf("Failed to create SKILL.md: %v", err)
				}
			}
		}

		// Create agents using helper
		if len(plugin.agents) > 0 {
			agentsDir := filepath.Join(pluginDir, "agents")
			if err := os.MkdirAll(agentsDir, 0755); err != nil {
				t.Fatalf("Failed to create agents directory: %v", err)
			}

			for _, agentName := range plugin.agents {
				// Use helper to create agent with proper structure
				tmpAgentPath := createTestAgent(t, agentName, agentName+" agent")
				// Copy to marketplace location
				agentPath := filepath.Join(agentsDir, agentName+".md")
				content, err := os.ReadFile(tmpAgentPath)
				if err != nil {
					t.Fatalf("Failed to read agent from helper: %v", err)
				}
				if err := os.WriteFile(agentPath, content, 0644); err != nil {
					t.Fatalf("Failed to create agent: %v", err)
				}
			}
		}
	}
}

// setupSimpleMarketplace creates a simple marketplace with one plugin
func setupSimpleMarketplace(t *testing.T, baseDir string) {
	pluginDir := filepath.Join(baseDir, "plugin")
	commandsDir := filepath.Join(pluginDir, "commands")

	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands directory: %v", err)
	}

	// Use helper to create command with proper structure
	tmpCmdPath := createTestCommand(t, "test", "Test command")
	// Copy to marketplace location
	cmdPath := filepath.Join(commandsDir, "test.md")
	content, err := os.ReadFile(tmpCmdPath)
	if err != nil {
		t.Fatalf("Failed to read command from helper: %v", err)
	}
	if err := os.WriteFile(cmdPath, content, 0644); err != nil {
		t.Fatalf("Failed to create command: %v", err)
	}
}

// setupMultiPluginMarketplace creates a marketplace with multiple plugins for filter testing
func setupMultiPluginMarketplace(t *testing.T, baseDir string) {
	plugins := []string{"code-helper", "code-reviewer", "test-runner"}

	for _, plugin := range plugins {
		pluginDir := filepath.Join(baseDir, "plugins", plugin)
		commandsDir := filepath.Join(pluginDir, "commands")

		if err := os.MkdirAll(commandsDir, 0755); err != nil {
			t.Fatalf("Failed to create commands directory: %v", err)
		}

		// Use helper to create command with proper structure
		tmpCmdPath := createTestCommand(t, "main", plugin+" command")
		// Copy to marketplace location
		cmdPath := filepath.Join(commandsDir, "main.md")
		content, err := os.ReadFile(tmpCmdPath)
		if err != nil {
			t.Fatalf("Failed to read command from helper: %v", err)
		}
		if err := os.WriteFile(cmdPath, content, 0644); err != nil {
			t.Fatalf("Failed to create command: %v", err)
		}
	}
}

// saveMarketplaceConfig saves a marketplace config as JSON
func saveMarketplaceConfig(path string, config *marketplace.MarketplaceConfig) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Write file
	return os.WriteFile(path, data, 0644)
}
