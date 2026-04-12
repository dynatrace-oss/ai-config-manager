package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/source"
)

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}

	return string(output)
}

func withRepoAddFlagsReset(t *testing.T, fn func()) {
	t.Helper()

	originalForce := forceFlag
	originalSkip := skipExistingFlag
	originalDryRun := dryRunFlag
	originalFilters := append([]string(nil), filterFlags...)
	originalFormat := addFormatFlag
	originalName := nameFlag
	originalDiscovery := discoveryFlag
	originalSilent := syncSilentMode

	forceFlag = false
	skipExistingFlag = false
	dryRunFlag = false
	filterFlags = nil
	addFormatFlag = "table"
	nameFlag = ""
	discoveryFlag = repomanifest.DiscoveryModeAuto
	syncSilentMode = false

	defer func() {
		forceFlag = originalForce
		skipExistingFlag = originalSkip
		dryRunFlag = originalDryRun
		filterFlags = originalFilters
		addFormatFlag = originalFormat
		nameFlag = originalName
		discoveryFlag = originalDiscovery
		syncSilentMode = originalSilent
	}()

	fn()
}

func TestRepoAdd_DiscoveryFlagValidation(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		discoveryFlag = "bogus"

		err := repoAddCmd.RunE(repoAddCmd, []string{"local:/tmp"})
		if err == nil {
			t.Fatal("expected invalid discovery mode error, got nil")
		}

		msg := err.Error()
		if !strings.Contains(msg, "invalid --discovery value") {
			t.Fatalf("expected CLI discovery validation error, got: %v", err)
		}
		for _, mode := range []string{"auto", "marketplace", "generic"} {
			if !strings.Contains(msg, mode) {
				t.Fatalf("expected error to list mode %q, got: %v", mode, err)
			}
		}
	})
}

func TestAddSourceToManifest_PersistsDiscoveryMode(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	srcPath := t.TempDir()
	parsed, err := source.ParseSource("local:" + srcPath)
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	withRepoAddFlagsReset(t, func() {
		if err := addSourceToManifest(manager, parsed, nil, repomanifest.DiscoveryModeMarketplace); err != nil {
			t.Fatalf("addSourceToManifest failed: %v", err)
		}
	})

	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if len(manifest.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(manifest.Sources))
	}
	if manifest.Sources[0].Discovery != repomanifest.DiscoveryModeMarketplace {
		t.Fatalf("expected discovery %q, got %q", repomanifest.DiscoveryModeMarketplace, manifest.Sources[0].Discovery)
	}
}

func TestAddSourceToManifest_StoresNormalizedRepoBackedMarketplaceSource(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	parsed, err := source.ParseSource("https://raw.githubusercontent.com/example/tools/main/.claude-plugin/marketplace.json")
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	withRepoAddFlagsReset(t, func() {
		if err := addSourceToManifest(manager, parsed, nil, repomanifest.DiscoveryModeAuto); err != nil {
			t.Fatalf("addSourceToManifest failed: %v", err)
		}
	})

	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if len(manifest.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(manifest.Sources))
	}

	src := manifest.Sources[0]
	if src.URL != "https://github.com/example/tools" {
		t.Fatalf("source URL = %q, want %q", src.URL, "https://github.com/example/tools")
	}
	if src.Ref != "main" {
		t.Fatalf("source ref = %q, want %q", src.Ref, "main")
	}
	if src.Subpath != ".claude-plugin/marketplace.json" {
		t.Fatalf("source subpath = %q, want %q", src.Subpath, ".claude-plugin/marketplace.json")
	}
}

func TestAddSourceToManifest_RemoteCanonicalReuseKeepsExistingName(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	firstParsed, err := source.ParseSource("https://GitHub.com/Example/Tools.git/")
	if err != nil {
		t.Fatalf("failed to parse first source: %v", err)
	}

	withRepoAddFlagsReset(t, func() {
		nameFlag = "primary-alias"
		if err := addSourceToManifest(manager, firstParsed, []string{"skill/*"}, repomanifest.DiscoveryModeAuto); err != nil {
			t.Fatalf("first addSourceToManifest failed: %v", err)
		}
	})

	// Capture stderr warning for alias/ref mismatch reuse path.
	originalStderr := os.Stderr
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create stderr pipe: %v", pipeErr)
	}
	os.Stderr = w

	secondParsed, err := source.ParseSource("https://github.com/example/tools")
	if err != nil {
		t.Fatalf("failed to parse second source: %v", err)
	}
	secondParsed.Ref = "release/v2"

	withRepoAddFlagsReset(t, func() {
		nameFlag = "second-alias"
		if err := addSourceToManifest(manager, secondParsed, []string{"command/*"}, repomanifest.DiscoveryModeGeneric); err != nil {
			t.Fatalf("second addSourceToManifest failed: %v", err)
		}
	})

	_ = w.Close()
	os.Stderr = originalStderr

	var errBuf bytes.Buffer
	if _, err := io.Copy(&errBuf, r); err != nil {
		t.Fatalf("failed reading captured stderr: %v", err)
	}
	stderrText := errBuf.String()
	if !strings.Contains(stderrText, "already exists as 'primary-alias'") {
		t.Fatalf("expected alias reuse warning, got stderr: %s", stderrText)
	}
	if !strings.Contains(stderrText, "ignoring requested ref 'release/v2'") {
		t.Fatalf("expected ref mismatch reuse warning, got stderr: %s", stderrText)
	}

	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if len(manifest.Sources) != 1 {
		t.Fatalf("expected one canonical remote source, got %d", len(manifest.Sources))
	}

	src := manifest.Sources[0]
	if src.Name != "primary-alias" {
		t.Fatalf("expected existing source name to be preserved, got %q", src.Name)
	}
	if src.Ref != "" {
		t.Fatalf("expected existing ref to remain unchanged (empty), got %q", src.Ref)
	}
	if got := strings.Join(src.Include, ","); got != "command/*" {
		t.Fatalf("expected include to be replaced on reuse, got %q", got)
	}
	if src.Discovery != repomanifest.DiscoveryModeGeneric {
		t.Fatalf("expected discovery mode update on reuse, got %q", src.Discovery)
	}
}

func TestAddSourceToManifest_RemoteCanonicalReuseDistinctSubpaths(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	firstParsed, err := source.ParseSource("gh:example/tools/skills")
	if err != nil {
		t.Fatalf("failed to parse first source: %v", err)
	}
	secondParsed, err := source.ParseSource("gh:example/tools/agents")
	if err != nil {
		t.Fatalf("failed to parse second source: %v", err)
	}

	withRepoAddFlagsReset(t, func() {
		nameFlag = "skills-source"
		if err := addSourceToManifest(manager, firstParsed, nil, repomanifest.DiscoveryModeAuto); err != nil {
			t.Fatalf("failed adding first subpath source: %v", err)
		}
	})

	withRepoAddFlagsReset(t, func() {
		nameFlag = "agents-source"
		if err := addSourceToManifest(manager, secondParsed, nil, repomanifest.DiscoveryModeAuto); err != nil {
			t.Fatalf("failed adding second subpath source: %v", err)
		}
	})

	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if len(manifest.Sources) != 2 {
		t.Fatalf("expected distinct subpaths to remain separate sources, got %d", len(manifest.Sources))
	}
}

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
		"Tip: Some files were skipped during discovery.",
	}

	for _, elem := range expectedElements {
		if !strings.Contains(output, elem) {
			t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", elem, output)
		}
	}
}

func TestPrintDiscoveryErrors_AgentNoFrontmatterUsesNeutralMessage(t *testing.T) {
	err := resource.NewValidationError(
		"/tmp/source/agents/index.md",
		"agent",
		"index",
		"frontmatter",
		fmt.Errorf("no frontmatter found (must start with '---')"),
	)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printDiscoveryErrors([]discovery.DiscoveryError{{Path: "/tmp/source/agents/index.md", Error: err}})

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	expected := []string{
		"✗ agents/index.md",
		"Skipped: markdown file in agents/ does not look like an agent definition because it does not start with YAML frontmatter",
		"If this file is documentation, no action is needed.",
		"If this file is meant to be an agent, add YAML frontmatter starting with '---'.",
		"If a skipped file is only documentation, no action is needed.",
	}

	for _, elem := range expected {
		if !strings.Contains(output, elem) {
			t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", elem, output)
		}
	}

	unexpected := []string{
		"Suggestion: Add YAML frontmatter at the top of the file:",
		"Fix the issues above and re-run the import.",
	}

	for _, elem := range unexpected {
		if strings.Contains(output, elem) {
			t.Errorf("Did not expect output to contain %q.\nOutput:\n%s", elem, output)
		}
	}
}

func TestPrintDiscoveryErrors_CommandNoFrontmatterUsesNeutralMessage(t *testing.T) {
	err := resource.NewValidationError(
		"/tmp/source/commands/index.md",
		"command",
		"index",
		"frontmatter",
		fmt.Errorf("no frontmatter found (must start with '---')"),
	)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printDiscoveryErrors([]discovery.DiscoveryError{{Path: "/tmp/source/commands/index.md", Error: err}})

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	expected := []string{
		"✗ commands/index.md",
		"Skipped: markdown file in commands/ does not look like a command definition because it does not start with YAML frontmatter",
		"If this file is documentation, no action is needed.",
		"If this file is meant to be a command, add YAML frontmatter starting with '---'.",
		"If a skipped file is only documentation, no action is needed.",
	}

	for _, elem := range expected {
		if !strings.Contains(output, elem) {
			t.Errorf("Expected output to contain %q, but it didn't.\nOutput:\n%s", elem, output)
		}
	}
}

func TestPrintDiscoveryErrors_CommandAndAgentNoFrontmatterShareNeutralWarningShape(t *testing.T) {
	commandErr := resource.NewValidationError(
		"/tmp/source/commands/index.md",
		"command",
		"index",
		"frontmatter",
		fmt.Errorf("no frontmatter found (must start with '---')"),
	)
	agentErr := resource.NewValidationError(
		"/tmp/source/agents/index.md",
		"agent",
		"index",
		"frontmatter",
		fmt.Errorf("no frontmatter found (must start with '---')"),
	)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printDiscoveryErrors([]discovery.DiscoveryError{
		{Path: "/tmp/source/commands/index.md", Error: commandErr},
		{Path: "/tmp/source/agents/index.md", Error: agentErr},
	})

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "⚠ Discovery Issues (2):") {
		t.Fatalf("expected two discovery warnings, got:\n%s", output)
	}

	if strings.Count(output, "Skipped: markdown file in ") != 2 {
		t.Fatalf("expected command and agent warnings to both use Skipped neutral shape, got:\n%s", output)
	}
	if strings.Count(output, "does not start with YAML frontmatter") != 2 {
		t.Fatalf("expected matching no-frontmatter neutral wording for command and agent warnings, got:\n%s", output)
	}
	if strings.Count(output, "If this file is documentation, no action is needed.") != 2 {
		t.Fatalf("expected neutral documentation guidance for both command and agent warnings, got:\n%s", output)
	}

	if !strings.Contains(output, "If this file is meant to be a command, add YAML frontmatter starting with '---'.") {
		t.Fatalf("expected command-specific follow-up guidance, got:\n%s", output)
	}
	if !strings.Contains(output, "If this file is meant to be an agent, add YAML frontmatter starting with '---'.") {
		t.Fatalf("expected agent-specific follow-up guidance, got:\n%s", output)
	}
}

func TestImportFromLocalPathWithMode_GenericDiscoveryWarningMatrix_SkillsReadmeIsStructurallySilent(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		sourceDir := t.TempDir()

		if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
			t.Fatalf("failed to create commands directory: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(sourceDir, "agents"), 0755); err != nil {
			t.Fatalf("failed to create agents directory: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(sourceDir, "skills", "valid-skill"), 0755); err != nil {
			t.Fatalf("failed to create skill directory: %v", err)
		}

		if err := os.WriteFile(filepath.Join(sourceDir, "commands", "valid-command.md"), []byte("---\ndescription: valid command\n---\n# valid-command\n"), 0644); err != nil {
			t.Fatalf("failed to write valid command: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sourceDir, "commands", "index.md"), []byte("# Command Docs\n"), 0644); err != nil {
			t.Fatalf("failed to write command docs file: %v", err)
		}

		if err := os.WriteFile(filepath.Join(sourceDir, "agents", "valid-agent.md"), []byte("---\ndescription: valid agent\n---\n# valid-agent\n"), 0644); err != nil {
			t.Fatalf("failed to write valid agent: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sourceDir, "agents", "index.md"), []byte("# Agent Docs\n"), 0644); err != nil {
			t.Fatalf("failed to write agent docs file: %v", err)
		}

		if err := os.WriteFile(filepath.Join(sourceDir, "skills", "valid-skill", "SKILL.md"), []byte("---\nname: valid-skill\ndescription: valid skill\n---\n# skill\n"), 0644); err != nil {
			t.Fatalf("failed to write valid skill: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sourceDir, "skills", "README.md"), []byte("# skills docs\n"), 0644); err != nil {
			t.Fatalf("failed to write loose skills README: %v", err)
		}

		repoPath := t.TempDir()
		manager := repo.NewManagerWithPath(repoPath)
		if err := manager.Init(); err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		_, err := importFromLocalPathWithMode(
			sourceDir,
			manager,
			nil,
			"file://"+sourceDir,
			string(source.Local),
			"",
			"copy",
			repomanifest.DiscoveryModeGeneric,
			"test-source",
			"src-test",
		)

		_ = w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		output := buf.String()

		if err != nil {
			t.Fatalf("import failed: %v\noutput:\n%s", err, output)
		}

		if !strings.Contains(output, "⚠ Discovery Issues (2):") {
			t.Fatalf("expected warnings only for command/agent no-frontmatter candidates, got:\n%s", output)
		}
		if !strings.Contains(output, "✗ commands/index.md") {
			t.Fatalf("expected command docs warning for commands/index.md, got:\n%s", output)
		}
		if !strings.Contains(output, "✗ agents/index.md") {
			t.Fatalf("expected agent docs warning for agents/index.md, got:\n%s", output)
		}

		// Structural skill rule: only subdirectories containing SKILL.md are skill
		// candidates. Loose markdown files under skills/ are not candidates and must
		// therefore remain warning-silent.
		if strings.Contains(output, "skills/README.md") {
			t.Fatalf("did not expect discovery warning for loose skills markdown file, got:\n%s", output)
		}
	})
}

func TestFormatDiscoveryErrorDisplay_NoFrontmatterDetectionContract(t *testing.T) {
	t.Run("matching sentinel substring returns neutral skipped display", func(t *testing.T) {
		err := resource.NewValidationError(
			"/tmp/source/commands/index.md",
			"command",
			"index",
			"frontmatter",
			fmt.Errorf("%s (must start with '---')", noFrontmatterFoundErrorSubstring),
		)

		label, message, suggestions := formatDiscoveryErrorDisplay(err)

		if label != "Skipped" {
			t.Fatalf("expected label Skipped, got %q", label)
		}
		if !strings.Contains(message, "does not start with YAML frontmatter") {
			t.Fatalf("expected neutral no-frontmatter message, got %q", message)
		}
		if len(suggestions) == 0 {
			t.Fatal("expected neutral suggestions for no-frontmatter case")
		}
	})

	t.Run("different wording no longer matches sentinel substring", func(t *testing.T) {
		err := resource.NewValidationError(
			"/tmp/source/commands/index.md",
			"command",
			"index",
			"frontmatter",
			fmt.Errorf("no closing frontmatter delimiter"),
		)

		label, message, suggestions := formatDiscoveryErrorDisplay(err)

		if label != "Error" {
			t.Fatalf("expected label Error when sentinel wording is absent, got %q", label)
		}
		if strings.Contains(message, "does not look like a command definition") {
			t.Fatalf("unexpected neutral skipped message without sentinel wording: %q", message)
		}
		if len(suggestions) == 0 {
			t.Fatal("expected standard validation suggestion to remain for non-matching wording")
		}
	})
}

func TestRepoAdd_ManifestCommitIsScopedToManifestFiles(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	unrelated := filepath.Join(repoPath, "unrelated.txt")
	if err := os.WriteFile(unrelated, []byte("base\n"), 0644); err != nil {
		t.Fatalf("failed to create unrelated file: %v", err)
	}
	if err := manager.CommitChangesForPaths("test: add unrelated file", []string{"unrelated.txt"}); err != nil {
		t.Fatalf("failed to commit unrelated baseline: %v", err)
	}
	if err := os.WriteFile(unrelated, []byte("base\nlocal change\n"), 0644); err != nil {
		t.Fatalf("failed to modify unrelated file: %v", err)
	}

	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create source commands dir: %v", err)
	}
	cmdContent := "---\ndescription: Add test command\n---\n# add-test\n"
	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "add-test.md"), []byte(cmdContent), 0644); err != nil {
		t.Fatalf("failed to write source command: %v", err)
	}

	withRepoAddFlagsReset(t, func() {
		if err := repoAddCmd.RunE(repoAddCmd, []string{"local:" + sourceDir}); err != nil {
			t.Fatalf("repo add failed: %v", err)
		}
	})

	status := gitOutput(t, repoPath, "status", "--porcelain")
	if !strings.Contains(status, " M unrelated.txt") {
		t.Fatalf("expected unrelated.txt to remain unstaged after repo add, status:\n%s", status)
	}

	manifestCommitFiles := gitOutput(t, repoPath, "show", "--name-only", "--pretty=format:", "HEAD")
	if !strings.Contains(manifestCommitFiles, "ai.repo.yaml") {
		t.Fatalf("expected manifest tracking commit to include ai.repo.yaml, got:\n%s", manifestCommitFiles)
	}
	if !strings.Contains(manifestCommitFiles, ".metadata/sources.json") {
		t.Fatalf("expected manifest tracking commit to include .metadata/sources.json, got:\n%s", manifestCommitFiles)
	}
	if strings.Contains(manifestCommitFiles, "unrelated.txt") {
		t.Fatalf("manifest tracking commit must not include unrelated.txt, got:\n%s", manifestCommitFiles)
	}
}

func TestImportFromLocalPathWithMode_DiscoveryModes(t *testing.T) {
	sourceDir := createSourceWithMarketplaceAndLooseResources(t)

	tests := []struct {
		name              string
		discoveryMode     string
		expectLoose       bool
		expectPlugin      bool
		expectPackage     bool
		expectImportError string
	}{
		{
			name:          "auto prefers marketplace only",
			discoveryMode: repomanifest.DiscoveryModeAuto,
			expectLoose:   false,
			expectPlugin:  true,
			expectPackage: true,
		},
		{
			name:          "marketplace imports marketplace only",
			discoveryMode: repomanifest.DiscoveryModeMarketplace,
			expectLoose:   false,
			expectPlugin:  true,
			expectPackage: true,
		},
		{
			name:          "generic ignores marketplace",
			discoveryMode: repomanifest.DiscoveryModeGeneric,
			expectLoose:   true,
			expectPlugin:  true,
			expectPackage: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withRepoAddFlagsReset(t, func() {
				repoPath := t.TempDir()
				manager := repo.NewManagerWithPath(repoPath)
				if err := manager.Init(); err != nil {
					t.Fatalf("failed to init repo: %v", err)
				}

				_, err := importFromLocalPathWithMode(
					sourceDir,
					manager,
					nil,
					"file://"+sourceDir,
					string(source.Local),
					"",
					"symlink",
					tt.discoveryMode,
					"test-source",
					"src-test",
				)
				if tt.expectImportError != "" {
					if err == nil || !strings.Contains(err.Error(), tt.expectImportError) {
						t.Fatalf("expected error containing %q, got %v", tt.expectImportError, err)
					}
					return
				}
				if err != nil {
					t.Fatalf("import failed: %v", err)
				}

				loose, _ := manager.Get("loose-command", resource.Command)
				if tt.expectLoose && loose == nil {
					t.Fatalf("expected loose-command to be imported")
				}
				if !tt.expectLoose && loose != nil {
					t.Fatalf("expected loose-command to be excluded")
				}

				plugin, _ := manager.Get("plugin-command", resource.Command)
				if tt.expectPlugin && plugin == nil {
					t.Fatalf("expected plugin-command to be imported")
				}
				if !tt.expectPlugin && plugin != nil {
					t.Fatalf("expected plugin-command to be excluded")
				}

				pkgPath := filepath.Join(repoPath, "packages", "market-plugin.package.json")
				_, pkgErr := os.Stat(pkgPath)
				hasPkg := pkgErr == nil
				if tt.expectPackage && !hasPkg {
					t.Fatalf("expected marketplace package to exist")
				}
				if !tt.expectPackage && hasPkg {
					t.Fatalf("expected marketplace package to be absent")
				}
			})
		})
	}
}

func TestImportFromLocalPathWithMode_MarketplaceRequirementsAndZeroResolvable(t *testing.T) {
	t.Run("marketplace mode requires marketplace file", func(t *testing.T) {
		withRepoAddFlagsReset(t, func() {
			sourceDir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
				t.Fatalf("failed to create commands dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(sourceDir, "commands", "just-command.md"), []byte("---\ndescription: only command\n---\n# just-command\n"), 0644); err != nil {
				t.Fatalf("failed to write command: %v", err)
			}

			repoPath := t.TempDir()
			manager := repo.NewManagerWithPath(repoPath)
			if err := manager.Init(); err != nil {
				t.Fatalf("failed to init repo: %v", err)
			}

			_, err := importFromLocalPathWithMode(sourceDir, manager, nil, "file://"+sourceDir, string(source.Local), "", "symlink", repomanifest.DiscoveryModeMarketplace, "test-source", "src-test")
			if err == nil {
				t.Fatal("expected marketplace-mode error when marketplace.json is missing")
			}
			if !strings.Contains(err.Error(), "requires marketplace.json") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})

	t.Run("zero-resolvable marketplace errors in auto and marketplace", func(t *testing.T) {
		sourceDir := createSourceWithMarketplaceNoResolvablePlugins(t)

		for _, mode := range []string{repomanifest.DiscoveryModeAuto, repomanifest.DiscoveryModeMarketplace} {
			t.Run(mode, func(t *testing.T) {
				withRepoAddFlagsReset(t, func() {
					repoPath := t.TempDir()
					manager := repo.NewManagerWithPath(repoPath)
					if err := manager.Init(); err != nil {
						t.Fatalf("failed to init repo: %v", err)
					}

					_, err := importFromLocalPathWithMode(sourceDir, manager, nil, "file://"+sourceDir, string(source.Local), "", "symlink", mode, "test-source", "src-test")
					if err == nil {
						t.Fatalf("expected zero-resolvable error for mode %q", mode)
					}
					if !strings.Contains(err.Error(), "no plugin resources were resolvable") {
						t.Fatalf("unexpected error for mode %q: %v", mode, err)
					}
				})
			})
		}
	})
}

func TestDiscoverImportResourcesByMode_MissingNormalizedMarketplacePath(t *testing.T) {
	baseDir := t.TempDir()
	missingMarketplacePath := filepath.Join(baseDir, ".missing", "marketplace.json")

	for _, mode := range []string{repomanifest.DiscoveryModeAuto, repomanifest.DiscoveryModeMarketplace} {
		t.Run(mode, func(t *testing.T) {
			_, err := discoverImportResourcesByMode(missingMarketplacePath, mode)
			if err == nil {
				t.Fatalf("expected missing normalized marketplace path error for mode %q", mode)
			}

			errMsg := err.Error()
			if !strings.Contains(errMsg, "marketplace.json/subpath lookup failed") {
				t.Fatalf("expected marketplace lookup error for mode %q, got: %v", mode, err)
			}
			if !strings.Contains(errMsg, missingMarketplacePath) {
				t.Fatalf("expected error to mention normalized path %q, got: %v", missingMarketplacePath, err)
			}
			if strings.Contains(errMsg, "failed to discover commands") {
				t.Fatalf("expected failure before generic discovery for mode %q, got: %v", mode, err)
			}
		})
	}
}

func TestAddBulkFromLocalWithMode_DirectMarketplaceFileInput(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		sourceDir := createSourceWithMarketplaceAndLooseResources(t)
		marketplaceFile := filepath.Join(sourceDir, "marketplace.json")

		repoPath := t.TempDir()
		manager := repo.NewManagerWithPath(repoPath)
		if err := manager.Init(); err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		discoveryFlag = repomanifest.DiscoveryModeAuto
		if err := addBulkFromLocalWithMode(marketplaceFile, manager, nil, "src-local-file", "symlink", "file-source"); err != nil {
			t.Fatalf("addBulkFromLocalWithMode failed for direct marketplace file: %v", err)
		}

		plugin, _ := manager.Get("plugin-command", resource.Command)
		if plugin == nil {
			t.Fatal("expected plugin-command to be imported from direct marketplace file")
		}

		loose, _ := manager.Get("loose-command", resource.Command)
		if loose != nil {
			t.Fatal("expected loose-command to remain excluded in auto marketplace mode")
		}
	})
}

func TestImportFromLocalPathWithMode_RepoMarketplaceSubpathFileInput(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		sourceDir := createSourceWithMarketplaceAndLooseResources(t)
		marketplaceFile := filepath.Join(sourceDir, "marketplace.json")

		repoPath := t.TempDir()
		manager := repo.NewManagerWithPath(repoPath)
		if err := manager.Init(); err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		// Simulate remote clone + subpath resolution ending in marketplace.json.
		_, err := importFromLocalPathWithMode(
			marketplaceFile,
			manager,
			nil,
			"https://github.com/example/repo",
			"github",
			"main",
			"copy",
			repomanifest.DiscoveryModeAuto,
			"remote-file-source",
			"src-remote-file",
		)
		if err != nil {
			t.Fatalf("importFromLocalPathWithMode failed for repo marketplace subpath file: %v", err)
		}

		plugin, _ := manager.Get("plugin-command", resource.Command)
		if plugin == nil {
			t.Fatal("expected plugin-command to be imported from subpath marketplace file")
		}
	})
}

func createSourceWithPluginDirectoryMarketplace(t *testing.T, marketplaceDirectory string) string {
	t.Helper()

	sourceDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create loose commands dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "plugins", "dt-github", "commands"), 0755); err != nil {
		t.Fatalf("failed to create plugin commands dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, marketplaceDirectory), 0755); err != nil {
		t.Fatalf("failed to create marketplace dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "loose-command.md"), []byte("---\ndescription: loose\n---\n# loose-command\n"), 0644); err != nil {
		t.Fatalf("failed to write loose command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "plugins", "dt-github", "commands", "plugin-command.md"), []byte("---\ndescription: plugin\n---\n# plugin-command\n"), 0644); err != nil {
		t.Fatalf("failed to write plugin command: %v", err)
	}

	marketplaceContent := `{
		"name": "plugin-manifest",
		"description": "plugin dir manifest",
		"plugins": [
			{
				"name": "dt-github",
				"description": "test plugin",
				"source": "plugins/dt-github"
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(sourceDir, marketplaceDirectory, "marketplace.json"), []byte(marketplaceContent), 0644); err != nil {
		t.Fatalf("failed to write marketplace.json: %v", err)
	}

	return sourceDir
}

func TestAddBulkFromLocalWithMode_DirectPluginDirectoryMarketplaceFileInput(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		tests := []struct {
			name                 string
			marketplaceDirectory string
		}{
			{name: "claude-plugin manifest", marketplaceDirectory: ".claude-plugin"},
			{name: "opencode-plugin manifest", marketplaceDirectory: ".opencode-plugin"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sourceDir := createSourceWithPluginDirectoryMarketplace(t, tt.marketplaceDirectory)
				marketplaceFile := filepath.Join(sourceDir, tt.marketplaceDirectory, "marketplace.json")

				repoPath := t.TempDir()
				manager := repo.NewManagerWithPath(repoPath)
				if err := manager.Init(); err != nil {
					t.Fatalf("failed to init repo: %v", err)
				}

				discoveryFlag = repomanifest.DiscoveryModeAuto
				if err := addBulkFromLocalWithMode(marketplaceFile, manager, nil, "src-local-file", "symlink", "file-source"); err != nil {
					t.Fatalf("addBulkFromLocalWithMode failed for direct plugin-dir marketplace file: %v", err)
				}

				plugin, _ := manager.Get("plugin-command", resource.Command)
				if plugin == nil {
					t.Fatal("expected plugin-command to be imported from direct plugin-dir marketplace file")
				}

				loose, _ := manager.Get("loose-command", resource.Command)
				if loose != nil {
					t.Fatal("expected loose-command to remain excluded in auto marketplace mode")
				}
			})
		}
	})
}

func TestImportFromLocalPathWithMode_RepoPluginDirectoryMarketplaceSubpathFileInput(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		tests := []struct {
			name                 string
			marketplaceDirectory string
			subpath              string
		}{
			{name: "claude-plugin manifest", marketplaceDirectory: ".claude-plugin", subpath: ".claude-plugin/marketplace.json"},
			{name: "opencode-plugin manifest", marketplaceDirectory: ".opencode-plugin", subpath: ".opencode-plugin/marketplace.json"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sourceDir := createSourceWithPluginDirectoryMarketplace(t, tt.marketplaceDirectory)
				marketplaceFile := filepath.Join(sourceDir, tt.marketplaceDirectory, "marketplace.json")

				repoPath := t.TempDir()
				manager := repo.NewManagerWithPath(repoPath)
				if err := manager.Init(); err != nil {
					t.Fatalf("failed to init repo: %v", err)
				}

				_, err := importFromLocalPathWithMode(
					marketplaceFile,
					manager,
					nil,
					"https://github.com/example/repo",
					"github",
					"main",
					"copy",
					repomanifest.DiscoveryModeAuto,
					"remote-file-source",
					"src-remote-file",
				)
				if err != nil {
					t.Fatalf("importFromLocalPathWithMode failed for repo plugin-dir marketplace subpath file (%s): %v", tt.subpath, err)
				}

				plugin, _ := manager.Get("plugin-command", resource.Command)
				if plugin == nil {
					t.Fatal("expected plugin-command to be imported from plugin-dir subpath marketplace file")
				}

				loose, _ := manager.Get("loose-command", resource.Command)
				if loose != nil {
					t.Fatal("expected loose-command to remain excluded in auto marketplace mode")
				}
			})
		}
	})
}

func TestMarketplaceSourceBasePath(t *testing.T) {
	tests := []struct {
		name            string
		localPath       string
		marketplacePath string
		want            string
	}{
		{
			name:            "directory discovery at repo root",
			localPath:       "/tmp/repo",
			marketplacePath: "/tmp/repo/.claude-plugin/marketplace.json",
			want:            "/tmp/repo",
		},
		{
			name:            "directory discovery in subpath",
			localPath:       "/tmp/repo/subdir",
			marketplacePath: "/tmp/repo/subdir/.opencode-plugin/marketplace.json",
			want:            "/tmp/repo/subdir",
		},
		{
			name:            "direct file path import",
			localPath:       "/tmp/repo/.claude-plugin/marketplace.json",
			marketplacePath: "/tmp/repo/.claude-plugin/marketplace.json",
			want:            "/tmp/repo",
		},
		{
			name:            "direct root marketplace file import",
			localPath:       "/tmp/repo/marketplace.json",
			marketplacePath: "/tmp/repo/marketplace.json",
			want:            "/tmp/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := marketplaceSourceBasePath(tt.localPath, tt.marketplacePath)
			if got != tt.want {
				t.Fatalf("marketplaceSourceBasePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestImportFromLocalPathWithMode_PluginDirectoryMarketplaceUsesRepoRootBase(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		tests := []struct {
			name                 string
			marketplaceDirectory string
		}{
			{name: "claude-plugin manifest", marketplaceDirectory: ".claude-plugin"},
			{name: "opencode-plugin manifest", marketplaceDirectory: ".opencode-plugin"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sourceDir := t.TempDir()

				if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
					t.Fatalf("failed to create loose commands dir: %v", err)
				}
				if err := os.MkdirAll(filepath.Join(sourceDir, "plugins", "dt-github", "commands"), 0755); err != nil {
					t.Fatalf("failed to create plugin commands dir: %v", err)
				}
				if err := os.MkdirAll(filepath.Join(sourceDir, tt.marketplaceDirectory), 0755); err != nil {
					t.Fatalf("failed to create marketplace dir: %v", err)
				}

				if err := os.WriteFile(filepath.Join(sourceDir, "commands", "loose-command.md"), []byte("---\ndescription: loose\n---\n# loose-command\n"), 0644); err != nil {
					t.Fatalf("failed to write loose command: %v", err)
				}
				if err := os.WriteFile(filepath.Join(sourceDir, "plugins", "dt-github", "commands", "plugin-command.md"), []byte("---\ndescription: plugin\n---\n# plugin-command\n"), 0644); err != nil {
					t.Fatalf("failed to write plugin command: %v", err)
				}

				marketplaceContent := `{
					"name": "plugin-manifest",
					"description": "plugin dir manifest",
					"plugins": [
						{
							"name": "dt-github",
							"description": "test plugin",
							"source": "plugins/dt-github"
						}
					]
				}`
				if err := os.WriteFile(filepath.Join(sourceDir, tt.marketplaceDirectory, "marketplace.json"), []byte(marketplaceContent), 0644); err != nil {
					t.Fatalf("failed to write marketplace.json: %v", err)
				}

				repoPath := t.TempDir()
				manager := repo.NewManagerWithPath(repoPath)
				if err := manager.Init(); err != nil {
					t.Fatalf("failed to init repo: %v", err)
				}

				_, err := importFromLocalPathWithMode(
					sourceDir,
					manager,
					nil,
					"file://"+sourceDir,
					string(source.Local),
					"",
					"symlink",
					repomanifest.DiscoveryModeMarketplace,
					"test-source",
					"src-test",
				)
				if err != nil {
					t.Fatalf("importFromLocalPathWithMode failed: %v", err)
				}

				plugin, _ := manager.Get("plugin-command", resource.Command)
				if plugin == nil {
					t.Fatal("expected plugin-command to be imported from plugin source")
				}

				loose, _ := manager.Get("loose-command", resource.Command)
				if loose != nil {
					t.Fatal("expected loose command to remain excluded in marketplace mode")
				}
			})
		}
	})
}

func TestImportFromLocalPathWithMode_MarketplaceImportsReferencedDotAgentFiles(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		sourceDir := t.TempDir()

		if err := os.MkdirAll(filepath.Join(sourceDir, "plugins", "dt-service-onboarding", "commands"), 0755); err != nil {
			t.Fatalf("failed to create plugin commands dir: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(sourceDir, "plugins", "dt-service-onboarding", "agents"), 0755); err != nil {
			t.Fatalf("failed to create plugin agents dir: %v", err)
		}

		commandContent := "---\ndescription: onboarding command\n---\n# dt-onboarding\n"
		if err := os.WriteFile(filepath.Join(sourceDir, "plugins", "dt-service-onboarding", "commands", "dt-onboarding.md"), []byte(commandContent), 0644); err != nil {
			t.Fatalf("failed to write command: %v", err)
		}

		agentContent := "---\ndescription: onboarding agent\ntype: onboarding\n---\n# onboarding\n"
		if err := os.WriteFile(filepath.Join(sourceDir, "plugins", "dt-service-onboarding", "agents", "dt-service-onboarding.agent.md"), []byte(agentContent), 0644); err != nil {
			t.Fatalf("failed to write agent: %v", err)
		}

		marketplaceContent := `{
			"name": "agent-marketplace",
			"description": "marketplace with agent references",
			"plugins": [
				{
					"name": "dt-service-onboarding",
					"description": "plugin with command + agent",
					"source": "plugins/dt-service-onboarding"
				}
			]
		}`
		if err := os.WriteFile(filepath.Join(sourceDir, "marketplace.json"), []byte(marketplaceContent), 0644); err != nil {
			t.Fatalf("failed to write marketplace.json: %v", err)
		}

		repoPath := t.TempDir()
		manager := repo.NewManagerWithPath(repoPath)
		if err := manager.Init(); err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		_, err := importFromLocalPathWithMode(
			sourceDir,
			manager,
			nil,
			"file://"+sourceDir,
			string(source.Local),
			"",
			"symlink",
			repomanifest.DiscoveryModeMarketplace,
			"test-source",
			"src-test",
		)
		if err != nil {
			t.Fatalf("importFromLocalPathWithMode failed: %v", err)
		}

		agent, _ := manager.Get("dt-service-onboarding", resource.Agent)
		if agent == nil {
			t.Fatal("expected referenced agent to be imported")
		}

		pkgPath := filepath.Join(repoPath, "packages", "dt-service-onboarding.package.json")
		pkg, err := resource.LoadPackage(pkgPath)
		if err != nil {
			t.Fatalf("failed to load generated marketplace package: %v", err)
		}

		foundAgentRef := false
		for _, ref := range pkg.Resources {
			if ref == "agent/dt-service-onboarding" {
				foundAgentRef = true
				break
			}
		}
		if !foundAgentRef {
			t.Fatalf("expected package to reference imported agent, resources: %v", pkg.Resources)
		}

		index, err := repo.BuildPackageReferenceIndexFromRoots([]string{repoPath})
		if err != nil {
			t.Fatalf("failed to build package reference index: %v", err)
		}
		issues := repo.ValidatePackageReferences(pkg, index)
		if len(issues) != 0 {
			t.Fatalf("expected no missing references in generated package, got: %#v", issues)
		}
	})
}
