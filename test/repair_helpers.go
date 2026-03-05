package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// repairTestProject holds the paths for a repair integration test setup.
type repairTestProject struct {
	// repoDir is the isolated aimgr repository
	repoDir string
	// projectDir is the project directory with tool dirs
	projectDir string
	// manifestPath is the path to ai.package.yaml
	manifestPath string
	// manager is the repo manager
	manager *repo.Manager
}

// setupRepairTestProject creates:
//   - an isolated temp repo (AIMGR_REPO_PATH set via t.Setenv)
//   - a temp project directory with a .claude subdirectory
//   - a repo.Manager with Init() called
//
// The caller is responsible for adding resources to the repo via manager.Add*.
func setupRepairTestProject(t *testing.T) *repairTestProject {
	t.Helper()

	repoDir := t.TempDir()
	projectDir := t.TempDir()
	configDir := t.TempDir()

	// Isolate the aimgr environment
	t.Setenv("AIMGR_REPO_PATH", repoDir)
	t.Setenv("XDG_CONFIG_HOME", configDir)
	t.Setenv("XDG_DATA_HOME", repoDir)

	// Write a minimal config so the CLI starts cleanly
	aimgrConfigDir := filepath.Join(configDir, "aimgr")
	if err := os.MkdirAll(aimgrConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create aimgr config dir: %v", err)
	}
	configContent := "install:\n  targets:\n    - claude\n"
	if err := os.WriteFile(filepath.Join(aimgrConfigDir, "aimgr.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write aimgr.yaml: %v", err)
	}

	// Create a .claude directory so the tool is detected
	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0755); err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	// Initialize the repo manager
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	return &repairTestProject{
		repoDir:      repoDir,
		projectDir:   projectDir,
		manifestPath: filepath.Join(projectDir, manifest.ManifestFileName),
		manager:      manager,
	}
}

// addCommandToRepo adds a command to the repo and returns the command resource name.
func (p *repairTestProject) addCommandToRepo(t *testing.T, name, description string) {
	t.Helper()
	cmdPath := createTestCommand(t, name, description)
	if err := p.manager.AddCommand(cmdPath, "file://"+cmdPath, "file"); err != nil {
		t.Fatalf("Failed to add command %s to repo: %v", name, err)
	}
}

// addSkillToRepo adds a skill to the repo.
func (p *repairTestProject) addSkillToRepo(t *testing.T, name, description string) {
	t.Helper()
	skillDir := createTestSkill(t, name, description)
	if err := p.manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("Failed to add skill %s to repo: %v", name, err)
	}
}

// installCommandSymlink creates a symlink in the project's .claude/commands directory
// that points to the resource in the repo (mimicking a real install).
func (p *repairTestProject) installCommandSymlink(t *testing.T, name string) string {
	t.Helper()
	cmdsDir := filepath.Join(p.projectDir, ".claude", "commands")
	if err := os.MkdirAll(cmdsDir, 0755); err != nil {
		t.Fatalf("Failed to create commands dir: %v", err)
	}
	target := filepath.Join(p.repoDir, "commands", name+".md")
	link := filepath.Join(cmdsDir, name+".md")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Failed to create command symlink: %v", err)
	}
	return link
}

// installSkillSymlink creates a symlink in the project's .claude/skills directory.
func (p *repairTestProject) installSkillSymlink(t *testing.T, name string) string {
	t.Helper()
	skillsDir := filepath.Join(p.projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("Failed to create skills dir: %v", err)
	}
	target := filepath.Join(p.repoDir, "skills", name)
	link := filepath.Join(skillsDir, name)
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Failed to create skill symlink: %v", err)
	}
	return link
}

// writeManifest saves a manifest with the given resource refs to the project.
func (p *repairTestProject) writeManifest(t *testing.T, refs ...string) {
	t.Helper()
	m := &manifest.Manifest{Resources: refs}
	if err := m.Save(p.manifestPath); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}
}

// addPackageToRepo creates a package in the repo with the given member references.
func (p *repairTestProject) addPackageToRepo(t *testing.T, pkgName string, members []string) {
	t.Helper()
	pkg := &resource.Package{
		Name:        pkgName,
		Description: "Test package " + pkgName,
		Resources:   members,
	}
	if err := resource.SavePackage(pkg, p.repoDir); err != nil {
		t.Fatalf("Failed to save package %s: %v", pkgName, err)
	}
}

// createBrokenSymlink creates a symlink at path pointing to a non-existent target.
func createBrokenSymlink(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("Failed to create parent dir for broken symlink: %v", err)
	}
	if err := os.Symlink("/nonexistent/target/does-not-exist", path); err != nil {
		t.Fatalf("Failed to create broken symlink at %s: %v", path, err)
	}
	// Verify it's actually broken
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("Expected broken symlink at %s but it resolves successfully", path)
	}
}

// assertFileExists fails the test if the path does not exist.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Errorf("Expected file/symlink to exist at %s but got: %v", path, err)
	}
}

// assertFileRemoved fails the test if the path still exists.
func assertFileRemoved(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Errorf("Expected file/symlink at %s to be removed but it still exists", path)
	}
}

// assertManifestContains fails the test if the manifest at manifestPath does not contain ref.
func assertManifestContains(t *testing.T, manifestPath, ref string) {
	t.Helper()
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load manifest at %s: %v", manifestPath, err)
	}
	if !m.Has(ref) {
		t.Errorf("Manifest should contain %q but it does not. Resources: %v", ref, m.Resources)
	}
}

// assertManifestNotContains fails the test if the manifest at manifestPath still contains ref.
func assertManifestNotContains(t *testing.T, manifestPath, ref string) {
	t.Helper()
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load manifest at %s: %v", manifestPath, err)
	}
	if m.Has(ref) {
		t.Errorf("Manifest should NOT contain %q but it does. Resources: %v", ref, m.Resources)
	}
}

// assertOutputContains fails the test if output does not contain substr.
func assertOutputContains(t *testing.T, output, substr string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Errorf("Expected output to contain %q\nFull output:\n%s", substr, output)
	}
}

// assertOutputNotContains fails the test if output contains substr.
func assertOutputNotContains(t *testing.T, output, substr string) {
	t.Helper()
	if strings.Contains(output, substr) {
		t.Errorf("Expected output NOT to contain %q\nFull output:\n%s", substr, output)
	}
}

// createNestedCommandInRepo creates a namespaced command (namespace/name) in the repo.
func (p *repairTestProject) createNestedCommandInRepo(t *testing.T, namespace, name, description string) {
	t.Helper()
	// Create a temp dir with the correct nested structure for AddCommand to parse the name
	tempBase := t.TempDir()
	nestedDir := filepath.Join(tempBase, "commands", namespace)
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested command dir: %v", err)
	}
	cmdPath := filepath.Join(nestedDir, name+".md")
	content := "---\ndescription: " + description + "\n---\n\n# " + name + "\nTest command.\n"
	if err := os.WriteFile(cmdPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write command file: %v", err)
	}
	if err := p.manager.AddCommand(cmdPath, "file://"+cmdPath, "file"); err != nil {
		t.Fatalf("Failed to add nested command %s/%s: %v", namespace, name, err)
	}
}

// installNestedCommandSymlink creates a broken symlink for a nested command.
func (p *repairTestProject) installNestedCommandSymlink(t *testing.T, namespace, name string) string {
	t.Helper()
	nsDir := filepath.Join(p.projectDir, ".claude", "commands", namespace)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		t.Fatalf("Failed to create namespace dir: %v", err)
	}
	target := filepath.Join(p.repoDir, "commands", namespace, name+".md")
	link := filepath.Join(nsDir, name+".md")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Failed to create nested command symlink: %v", err)
	}
	return link
}
