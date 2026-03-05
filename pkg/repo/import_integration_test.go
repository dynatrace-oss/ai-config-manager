package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// TestImportNestedCommands_EndToEnd verifies the complete import workflow:
// 1. Start with git repo on disk layout
// 2. Import using discovery + AddBulk
// 3. Verify all files are in repo
// 4. Verify all metadata entries exist and are correct
//
// This test reproduces bug ai-config-manager-2tzg where nested commands were
// silently skipped during import.
func TestImportNestedCommands_EndToEnd(t *testing.T) {
	// 1. Setup temp repo (NOT ~/.local/share/ai-config/repo)
	tempRepo := t.TempDir()
	mgr := NewManagerWithPath(tempRepo)

	// 2. Use existing testdata fixture (simulates git repo on disk)
	fixtureDir := filepath.Join("..", "..", "testdata", "repos", "commands-nested")

	// Verify fixture exists
	if _, err := os.Stat(fixtureDir); err != nil {
		t.Fatalf("Fixture directory not found: %s (error: %v)", fixtureDir, err)
	}

	// 3. Discover resources (what SHOULD be imported)
	commands, err := discovery.DiscoverCommands(fixtureDir, "")
	if err != nil {
		t.Fatalf("Failed to discover commands: %v", err)
	}

	t.Logf("Discovered %d commands:", len(commands))
	for _, cmd := range commands {
		t.Logf("  - %s (path: %s)", cmd.Name, cmd.Path)
	}

	// We expect exactly 3 commands: build, test, nested/deploy
	if len(commands) != 3 {
		t.Errorf("Expected to discover 3 commands, but found %d", len(commands))
	}

	// 4. Import using AddBulk (matches real workflow)
	var paths []string
	for _, cmd := range commands {
		paths = append(paths, cmd.Path)
	}

	result, err := mgr.AddBulk(paths, BulkImportOptions{})
	if err != nil {
		t.Fatalf("AddBulk failed: %v", err)
	}

	// 5. CRITICAL ASSERTION: discovered count == imported count
	// This is where the bug manifests: nested commands are silently skipped
	if result.CommandCount != len(commands) {
		t.Errorf("Imported %d commands but discovered %d", result.CommandCount, len(commands))
		t.Logf("Added: %v", result.Added)
		t.Logf("Failed: %v", result.Failed)
		t.Logf("Skipped: %v", result.Skipped)
	}

	// 6. Verify each command file exists in repo
	expectedCommands := []struct {
		name string
		file string
	}{
		{"build", "build.md"},
		{"test", "test.md"},
		{"nested/deploy", "nested/deploy.md"},
	}

	for _, cmd := range expectedCommands {
		expectedPath := filepath.Join(tempRepo, "commands", cmd.file)
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf("Command file not imported: %s (expected at: %s)", cmd.file, expectedPath)
		} else {
			t.Logf("✓ Command file exists: %s", cmd.file)
		}
	}

	// 7. Verify metadata exists and is correct for each command
	for _, cmd := range expectedCommands {
		meta, err := metadata.Load(cmd.name, resource.Command, tempRepo)
		if err != nil {
			t.Errorf("Metadata not found for command '%s': %v", cmd.name, err)
			continue
		}

		// Check metadata fields
		if meta.Name != cmd.name {
			t.Errorf("Metadata name mismatch: got '%s', want '%s'", meta.Name, cmd.name)
		}
		if meta.Type != resource.Command {
			t.Errorf("Metadata type mismatch for '%s': got '%s', want 'command'", cmd.name, meta.Type)
		}
		if meta.SourceURL == "" {
			t.Errorf("Metadata missing SourceURL for '%s'", cmd.name)
		}
		if meta.SourceType == "" {
			t.Errorf("Metadata missing SourceType for '%s'", cmd.name)
		}

		t.Logf("✓ Metadata verified: %s (type=%s, source=%s)", cmd.name, meta.Type, meta.SourceType)
	}

	// 8. Report on import failures (should be none)
	if len(result.Failed) > 0 {
		t.Errorf("Import had %d failures:", len(result.Failed))
		for _, f := range result.Failed {
			t.Errorf("  - %s: %s", f.Path, f.Message)
		}
	}

	t.Logf("✓ Import test complete: %d commands discovered, %d imported, all files and metadata verified",
		len(commands), result.CommandCount)
}

// TestImportNestedSkills_EndToEnd verifies the complete import workflow for skills:
// 1. Start with git repo on disk layout
// 2. Import using discovery + AddBulk
// 3. Verify all skill directories are in repo
// 4. Verify all SKILL.md files exist
// 5. Verify all metadata entries exist and are correct
func TestImportNestedSkills_EndToEnd(t *testing.T) {
	// 1. Setup temp repo (NOT ~/.local/share/ai-config/repo)
	tempRepo := t.TempDir()
	mgr := NewManagerWithPath(tempRepo)

	// 2. Use existing testdata fixture (simulates git repo on disk)
	fixtureDir := filepath.Join("..", "..", "testdata", "repos", "comprehensive-fixture")

	// Verify fixture exists
	if _, err := os.Stat(fixtureDir); err != nil {
		t.Fatalf("Fixture directory not found: %s (error: %v)", fixtureDir, err)
	}

	// 3. Discover resources (what SHOULD be imported)
	skills, err := discovery.DiscoverSkills(fixtureDir, "")
	if err != nil {
		t.Fatalf("Failed to discover skills: %v", err)
	}

	t.Logf("Discovered %d skills:", len(skills))
	for _, skill := range skills {
		t.Logf("  - %s (path: %s)", skill.Name, skill.Path)
	}

	// We expect exactly 2 skills: skill-one, skill-two
	if len(skills) != 2 {
		t.Errorf("Expected to discover 2 skills, but found %d", len(skills))
	}

	// 4. Import using AddBulk (matches real workflow)
	var paths []string
	for _, skill := range skills {
		paths = append(paths, skill.Path)
	}

	result, err := mgr.AddBulk(paths, BulkImportOptions{})
	if err != nil {
		t.Fatalf("AddBulk failed: %v", err)
	}

	// 5. CRITICAL ASSERTION: discovered count == imported count
	if result.SkillCount != len(skills) {
		t.Errorf("Imported %d skills but discovered %d", result.SkillCount, len(skills))
		t.Logf("Added: %v", result.Added)
		t.Logf("Failed: %v", result.Failed)
		t.Logf("Skipped: %v", result.Skipped)
	}

	// 6. Verify each skill directory exists in repo
	expectedSkills := []struct {
		name string
		dir  string
	}{
		{"skill-one", "skill-one"},
		{"skill-two", "skill-two"},
	}

	for _, skill := range expectedSkills {
		expectedPath := filepath.Join(tempRepo, "skills", skill.dir)
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf("Skill directory not imported: %s (expected at: %s)", skill.dir, expectedPath)
		} else {
			t.Logf("✓ Skill directory exists: %s", skill.dir)
		}

		// Check SKILL.md exists
		skillMdPath := filepath.Join(expectedPath, "SKILL.md")
		if _, err := os.Stat(skillMdPath); err != nil {
			t.Errorf("SKILL.md not found for skill '%s': %s", skill.name, skillMdPath)
		} else {
			t.Logf("✓ SKILL.md exists: %s", skill.name)
		}
	}

	// 7. Verify metadata exists and is correct for each skill
	for _, skill := range expectedSkills {
		meta, err := metadata.Load(skill.name, resource.Skill, tempRepo)
		if err != nil {
			t.Errorf("Metadata not found for skill '%s': %v", skill.name, err)
			continue
		}

		// Check metadata fields
		if meta.Name != skill.name {
			t.Errorf("Metadata name mismatch: got '%s', want '%s'", meta.Name, skill.name)
		}
		if meta.Type != resource.Skill {
			t.Errorf("Metadata type mismatch for '%s': got '%s', want 'skill'", skill.name, meta.Type)
		}
		if meta.SourceURL == "" {
			t.Errorf("Metadata missing SourceURL for '%s'", skill.name)
		}
		if meta.SourceType == "" {
			t.Errorf("Metadata missing SourceType for '%s'", skill.name)
		}

		t.Logf("✓ Metadata verified: %s (type=%s, source=%s)", skill.name, meta.Type, meta.SourceType)
	}

	// 8. Report on import failures (should be none)
	if len(result.Failed) > 0 {
		t.Errorf("Import had %d failures:", len(result.Failed))
		for _, f := range result.Failed {
			t.Errorf("  - %s: %s", f.Path, f.Message)
		}
	}

	t.Logf("✓ Import test complete: %d skills discovered, %d imported, all directories, files, and metadata verified",
		len(skills), result.SkillCount)
}

// TestImportNestedAgents_EndToEnd verifies the complete import workflow for agents:
// 1. Start with git repo on disk layout
// 2. Import using discovery + AddBulk
// 3. Verify all agent .md files are in repo
// 4. Verify all metadata entries exist and are correct
func TestImportNestedAgents_EndToEnd(t *testing.T) {
	// 1. Setup temp repo (NOT ~/.local/share/ai-config/repo)
	tempRepo := t.TempDir()
	mgr := NewManagerWithPath(tempRepo)

	// 2. Use existing testdata fixture (simulates git repo on disk)
	fixtureDir := filepath.Join("..", "..", "testdata", "repos", "comprehensive-fixture")

	// Verify fixture exists
	if _, err := os.Stat(fixtureDir); err != nil {
		t.Fatalf("Fixture directory not found: %s (error: %v)", fixtureDir, err)
	}

	// 3. Discover resources (what SHOULD be imported)
	agents, err := discovery.DiscoverAgents(fixtureDir, "")
	if err != nil {
		t.Fatalf("Failed to discover agents: %v", err)
	}

	t.Logf("Discovered %d agents:", len(agents))
	for _, agent := range agents {
		t.Logf("  - %s (path: %s)", agent.Name, agent.Path)
	}

	// We expect exactly 2 agents: agent-one, agent-two
	if len(agents) != 2 {
		t.Errorf("Expected to discover 2 agents, but found %d", len(agents))
	}

	// 4. Import using AddBulk (matches real workflow)
	var paths []string
	for _, agent := range agents {
		paths = append(paths, agent.Path)
	}

	result, err := mgr.AddBulk(paths, BulkImportOptions{})
	if err != nil {
		t.Fatalf("AddBulk failed: %v", err)
	}

	// 5. CRITICAL ASSERTION: discovered count == imported count
	if result.AgentCount != len(agents) {
		t.Errorf("Imported %d agents but discovered %d", result.AgentCount, len(agents))
		t.Logf("Added: %v", result.Added)
		t.Logf("Failed: %v", result.Failed)
		t.Logf("Skipped: %v", result.Skipped)
	}

	// 6. Verify each agent file exists in repo
	expectedAgents := []struct {
		name string
		file string
	}{
		{"agent-one", "agent-one.md"},
		{"agent-two", "agent-two.md"},
	}

	for _, agent := range expectedAgents {
		expectedPath := filepath.Join(tempRepo, "agents", agent.file)
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf("Agent file not imported: %s (expected at: %s)", agent.file, expectedPath)
		} else {
			t.Logf("✓ Agent file exists: %s", agent.file)
		}
	}

	// 7. Verify metadata exists and is correct for each agent
	for _, agent := range expectedAgents {
		meta, err := metadata.Load(agent.name, resource.Agent, tempRepo)
		if err != nil {
			t.Errorf("Metadata not found for agent '%s': %v", agent.name, err)
			continue
		}

		// Check metadata fields
		if meta.Name != agent.name {
			t.Errorf("Metadata name mismatch: got '%s', want '%s'", meta.Name, agent.name)
		}
		if meta.Type != resource.Agent {
			t.Errorf("Metadata type mismatch for '%s': got '%s', want 'agent'", agent.name, meta.Type)
		}
		if meta.SourceURL == "" {
			t.Errorf("Metadata missing SourceURL for '%s'", agent.name)
		}
		if meta.SourceType == "" {
			t.Errorf("Metadata missing SourceType for '%s'", agent.name)
		}

		t.Logf("✓ Metadata verified: %s (type=%s, source=%s)", agent.name, meta.Type, meta.SourceType)
	}

	// 8. Report on import failures (should be none)
	if len(result.Failed) > 0 {
		t.Errorf("Import had %d failures:", len(result.Failed))
		for _, f := range result.Failed {
			t.Errorf("  - %s: %s", f.Path, f.Message)
		}
	}

	t.Logf("✓ Import test complete: %d agents discovered, %d imported, all files and metadata verified",
		len(agents), result.AgentCount)
}

// TestImportPackageWithAllResourceTypes_EndToEnd verifies the complete import workflow
// for packages that reference commands, skills, and agents:
// 1. Start with git repo on disk layout with package definition
// 2. Import all resources first (commands, skills, agents)
// 3. Import package that references those resources
// 4. Verify package is imported
// 5. Verify all referenced resources exist in repo
// 6. Verify all metadata entries exist and are correct
func TestImportPackageWithAllResourceTypes_EndToEnd(t *testing.T) {
	// 1. Setup temp repo (NOT ~/.local/share/ai-config-manager/repo)
	tempRepo := t.TempDir()
	mgr := NewManagerWithPath(tempRepo)

	// 2. Use existing testdata fixture (simulates git repo on disk)
	fixtureDir := filepath.Join("..", "..", "testdata", "repos", "comprehensive-fixture")

	// Verify fixture exists
	if _, err := os.Stat(fixtureDir); err != nil {
		t.Fatalf("Fixture directory not found: %s (error: %v)", fixtureDir, err)
	}

	// 3. First, discover and import all resources that the package references
	// (In real workflow, user would: aimgr repo import <dir>, which imports everything)

	// Discover and import commands
	commands, err := discovery.DiscoverCommands(fixtureDir, "")
	if err != nil {
		t.Fatalf("Failed to discover commands: %v", err)
	}
	var commandPaths []string
	for _, cmd := range commands {
		commandPaths = append(commandPaths, cmd.Path)
	}

	// Discover and import skills
	skills, err := discovery.DiscoverSkills(fixtureDir, "")
	if err != nil {
		t.Fatalf("Failed to discover skills: %v", err)
	}
	var skillPaths []string
	for _, skill := range skills {
		skillPaths = append(skillPaths, skill.Path)
	}

	// Discover and import agents
	agents, err := discovery.DiscoverAgents(fixtureDir, "")
	if err != nil {
		t.Fatalf("Failed to discover agents: %v", err)
	}
	var agentPaths []string
	for _, agent := range agents {
		agentPaths = append(agentPaths, agent.Path)
	}

	// Import all resources
	allPaths := append(commandPaths, skillPaths...)
	allPaths = append(allPaths, agentPaths...)

	result, err := mgr.AddBulk(allPaths, BulkImportOptions{})
	if err != nil {
		t.Fatalf("AddBulk failed for resources: %v", err)
	}

	t.Logf("Imported %d commands, %d skills, %d agents",
		result.CommandCount, result.SkillCount, result.AgentCount)

	// 4. Now import the package that references these resources
	packagePath := filepath.Join(fixtureDir, "packages", "test-package.package.json")

	// Verify package file exists
	if _, err := os.Stat(packagePath); err != nil {
		t.Fatalf("Package file not found: %s (error: %v)", packagePath, err)
	}

	pkgResult, err := mgr.AddBulk([]string{packagePath}, BulkImportOptions{})
	if err != nil {
		t.Fatalf("AddBulk failed for package: %v", err)
	}

	// 5. VERIFY: package imported
	if pkgResult.PackageCount != 1 {
		t.Errorf("Expected 1 package imported, got %d", pkgResult.PackageCount)
	}

	t.Logf("Package import result: %d packages", pkgResult.PackageCount)

	// 6. VERIFY: all referenced commands exist in repo
	// From test-package.package.json: "command/api/deploy", "command/test"
	expectedCommands := []struct {
		name string
		file string
	}{
		{"api/deploy", "api/deploy.md"},
		{"test", "test.md"},
	}

	t.Logf("Verifying %d commands referenced by package...", len(expectedCommands))
	for _, cmd := range expectedCommands {
		// Check file exists
		expectedPath := filepath.Join(tempRepo, "commands", cmd.file)
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf("Command file not in repo: %s (expected at: %s)", cmd.file, expectedPath)
		} else {
			t.Logf("✓ Command file exists: %s", cmd.file)
		}

		// Check metadata exists
		meta, err := metadata.Load(cmd.name, resource.Command, tempRepo)
		if err != nil {
			t.Errorf("Metadata not found for command '%s': %v", cmd.name, err)
			continue
		}
		if meta.Name != cmd.name {
			t.Errorf("Command metadata name mismatch: got '%s', want '%s'", meta.Name, cmd.name)
		}
		t.Logf("✓ Command metadata verified: %s", cmd.name)
	}

	// 7. VERIFY: all referenced skills exist in repo
	// From test-package.package.json: "skill/skill-one"
	expectedSkills := []struct {
		name string
		dir  string
	}{
		{"skill-one", "skill-one"},
	}

	t.Logf("Verifying %d skills referenced by package...", len(expectedSkills))
	for _, skill := range expectedSkills {
		// Check directory + SKILL.md exists
		expectedPath := filepath.Join(tempRepo, "skills", skill.dir)
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf("Skill directory not in repo: %s (expected at: %s)", skill.dir, expectedPath)
		} else {
			t.Logf("✓ Skill directory exists: %s", skill.dir)
		}

		skillMdPath := filepath.Join(expectedPath, "SKILL.md")
		if _, err := os.Stat(skillMdPath); err != nil {
			t.Errorf("SKILL.md not found for skill '%s': %s", skill.name, skillMdPath)
		} else {
			t.Logf("✓ SKILL.md exists: %s", skill.name)
		}

		// Check metadata exists
		meta, err := metadata.Load(skill.name, resource.Skill, tempRepo)
		if err != nil {
			t.Errorf("Metadata not found for skill '%s': %v", skill.name, err)
			continue
		}
		if meta.Name != skill.name {
			t.Errorf("Skill metadata name mismatch: got '%s', want '%s'", meta.Name, skill.name)
		}
		t.Logf("✓ Skill metadata verified: %s", skill.name)
	}

	// 8. VERIFY: all referenced agents exist in repo
	// From test-package.package.json: "agent/agent-one"
	expectedAgents := []struct {
		name string
		file string
	}{
		{"agent-one", "agent-one.md"},
	}

	t.Logf("Verifying %d agents referenced by package...", len(expectedAgents))
	for _, agent := range expectedAgents {
		// Check file exists
		expectedPath := filepath.Join(tempRepo, "agents", agent.file)
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf("Agent file not in repo: %s (expected at: %s)", agent.file, expectedPath)
		} else {
			t.Logf("✓ Agent file exists: %s", agent.file)
		}

		// Check metadata exists
		meta, err := metadata.Load(agent.name, resource.Agent, tempRepo)
		if err != nil {
			t.Errorf("Metadata not found for agent '%s': %v", agent.name, err)
			continue
		}
		if meta.Name != agent.name {
			t.Errorf("Agent metadata name mismatch: got '%s', want '%s'", meta.Name, agent.name)
		}
		t.Logf("✓ Agent metadata verified: %s", agent.name)
	}

	// 9. Verify package metadata
	pkgMeta, err := metadata.LoadPackageMetadata("test-package", tempRepo)
	if err != nil {
		t.Errorf("Package metadata not found: %v", err)
	} else {
		if pkgMeta.Name != "test-package" {
			t.Errorf("Package metadata name mismatch: got '%s', want 'test-package'", pkgMeta.Name)
		}
		if pkgMeta.ResourceCount != 4 {
			t.Errorf("Package metadata resource count mismatch: got %d, want 4", pkgMeta.ResourceCount)
		}
		t.Logf("✓ Package metadata verified: %s (resources: %d)", pkgMeta.Name, pkgMeta.ResourceCount)
	}

	// 10. Report on import failures (should be none)
	if len(result.Failed) > 0 {
		t.Errorf("Import had %d failures:", len(result.Failed))
		for _, f := range result.Failed {
			t.Errorf("  - %s: %s", f.Path, f.Message)
		}
	}
	if len(pkgResult.Failed) > 0 {
		t.Errorf("Package import had %d failures:", len(pkgResult.Failed))
		for _, f := range pkgResult.Failed {
			t.Errorf("  - %s: %s", f.Path, f.Message)
		}
	}

	t.Logf("✓ Import test complete: 1 package imported, all referenced resources verified in repo")
}
