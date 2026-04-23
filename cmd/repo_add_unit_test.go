package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/discovery"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/source"
)

func TestApplyExplicitRemoteSourceFlags_AppliesToGitHubSource(t *testing.T) {
	parsed, err := source.ParseSource("gh:owner/repo")
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	err = applyExplicitRemoteSourceFlags(parsed, "gh:owner/repo", "main", "skills/core")
	if err != nil {
		t.Fatalf("expected explicit flags to be applied, got error: %v", err)
	}

	if parsed.Ref != "main" {
		t.Fatalf("parsed.Ref = %q, want %q", parsed.Ref, "main")
	}
	if parsed.Subpath != "skills/core" {
		t.Fatalf("parsed.Subpath = %q, want %q", parsed.Subpath, "skills/core")
	}
}

func TestApplyExplicitRemoteSourceFlags_RejectsMixedInlineRefAndExplicitRef(t *testing.T) {
	parsed, err := source.ParseSource("gh:owner/repo@main")
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	err = applyExplicitRemoteSourceFlags(parsed, "gh:owner/repo@main", "release", "")
	if err == nil {
		t.Fatal("expected mixed inline+explicit ref to fail")
	}
	if !strings.Contains(err.Error(), "do not mix inline ref/subpath syntax with --ref/--subpath") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyExplicitRemoteSourceFlags_RejectsMixedInlineSubpathAndExplicitSubpath(t *testing.T) {
	parsed, err := source.ParseSource("https://example.com/team/repo.git/skills")
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	err = applyExplicitRemoteSourceFlags(parsed, "https://example.com/team/repo.git/skills", "", "agents")
	if err == nil {
		t.Fatal("expected mixed inline+explicit subpath to fail")
	}
	if !strings.Contains(err.Error(), "do not mix inline ref/subpath syntax with --ref/--subpath") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyExplicitRemoteSourceFlags_RejectsMixedInlineSubpathAndExplicitRef(t *testing.T) {
	parsed, err := source.ParseSource("https://github.com/dynatrace-oss/ai-config-manager.git/ai-resources/skills/ai-resource-manager")
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	err = applyExplicitRemoteSourceFlags(parsed, "https://github.com/dynatrace-oss/ai-config-manager.git/ai-resources/skills/ai-resource-manager", "main", "")
	if err == nil {
		t.Fatal("expected inline subpath + explicit --ref to fail")
	}
	if !strings.Contains(err.Error(), "do not mix inline ref/subpath syntax with --ref/--subpath") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyExplicitRemoteSourceFlags_RejectsMixedInlineRefAndExplicitSubpath(t *testing.T) {
	parsed, err := source.ParseSource("gh:dynatrace-oss/ai-config-manager@main")
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	err = applyExplicitRemoteSourceFlags(parsed, "gh:dynatrace-oss/ai-config-manager@main", "", "ai-resources/skills/ai-resource-manager")
	if err == nil {
		t.Fatal("expected inline ref + explicit --subpath to fail")
	}
	if !strings.Contains(err.Error(), "do not mix inline ref/subpath syntax with --ref/--subpath") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyExplicitRemoteSourceFlags_RejectsMixedLegacyInlineRefSubpathAndExplicitFlags(t *testing.T) {
	parsed, err := source.ParseSource("gh:dynatrace-oss/ai-config-manager@main/skills")
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	err = applyExplicitRemoteSourceFlags(parsed, "gh:dynatrace-oss/ai-config-manager@main/skills", "", "skills/core")
	if err == nil {
		t.Fatal("expected inline legacy @ref/subpath + explicit --subpath to fail")
	}
	if !strings.Contains(err.Error(), "do not mix inline ref/subpath syntax with --ref/--subpath") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyExplicitRemoteSourceFlags_RejectsLocalSources(t *testing.T) {
	parsed, err := source.ParseSource("local:./resources")
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	err = applyExplicitRemoteSourceFlags(parsed, "local:./resources", "main", "skills")
	if err == nil {
		t.Fatal("expected explicit flags on local source to fail")
	}
	if !strings.Contains(err.Error(), "only supported for remote sources") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepoAddHelpText_PrefersExplicitRefSubpathAndDocumentsLegacyCompatibility(t *testing.T) {
	longHelp := repoAddCmd.Long

	checks := []string{
		"Preferred explicit ref syntax",
		"Preferred explicit subpath syntax",
		"Preferred explicit ref+subpath syntax",
		"Legacy compatibility forms still parse",
		"Ambiguous slash-containing refs + subpaths are rejected inline",
		"use --ref/--subpath",
	}

	for _, check := range checks {
		if !strings.Contains(longHelp, check) {
			t.Fatalf("expected repo add help text to contain %q", check)
		}
	}
}

func TestAddSourceToManifest_StoresExplicitRefAndSubpathForRemote(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	parsed, err := source.ParseSource("gh:owner/repo")
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}
	if err := applyExplicitRemoteSourceFlags(parsed, "gh:owner/repo", "main", "skills/core"); err != nil {
		t.Fatalf("failed to apply explicit source flags: %v", err)
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
	if src.URL != "https://github.com/owner/repo" {
		t.Fatalf("source URL = %q, want %q", src.URL, "https://github.com/owner/repo")
	}
	if src.Ref != "main" {
		t.Fatalf("source ref = %q, want %q", src.Ref, "main")
	}
	if src.Subpath != "skills/core" {
		t.Fatalf("source subpath = %q, want %q", src.Subpath, "skills/core")
	}
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

func TestRepoAdd_AmbiguousInlineGitHubRefSubpathRejectedBeforeRepoWork(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		err := repoAddCmd.RunE(repoAddCmd, []string{"gh:owner/repo@feature/x/skills"})
		if err == nil {
			t.Fatal("expected ambiguous inline slash-ref shorthand to fail")
		}

		errMsg := err.Error()
		if !strings.Contains(errMsg, "ambiguous GitHub shorthand") {
			t.Fatalf("expected ambiguous shorthand error, got: %v", err)
		}
		if !strings.Contains(errMsg, "--ref <ref> --subpath <path>") {
			t.Fatalf("expected explicit --ref/--subpath guidance, got: %v", err)
		}

		assertGoldenText(t, "repo_add/ambiguous_inline_ref_subpath_error.txt", errMsg+"\n")
	})
}

func TestFormatGitHubShortURL_PreservesRefBeforeSubpath(t *testing.T) {
	parsed := &source.ParsedSource{
		Type:    source.GitHub,
		URL:     "https://github.com/example/tools",
		Ref:     "release/v1",
		Subpath: "skills/core",
	}

	got := formatGitHubShortURL(parsed)
	if got != "gh:example/tools@release/v1/skills/core" {
		t.Fatalf("formatGitHubShortURL() = %q, want %q", got, "gh:example/tools@release/v1/skills/core")
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
