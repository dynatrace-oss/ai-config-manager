package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRunResourceValidate_PathTargets(t *testing.T) {
	t.Run("skill path", func(t *testing.T) {
		skillDir := filepath.Join(t.TempDir(), "my-skill")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		skillContent := `---
name: my-skill
description: test skill
---
# Skill
`
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
			t.Fatalf("write skill: %v", err)
		}

		result := runResourceValidate(skillDir, resourceValidateOptions{format: "table"})
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", result.ExitCode)
		}
		if !result.Output.Valid {
			t.Fatalf("expected valid result")
		}
		if result.Output.ResourceType != "skill" {
			t.Fatalf("expected skill, got %q", result.Output.ResourceType)
		}
	})

	t.Run("agent path", func(t *testing.T) {
		agentsDir := filepath.Join(t.TempDir(), "agents")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatalf("mkdir agents dir: %v", err)
		}
		agentPath := filepath.Join(agentsDir, "my-agent.md")
		agentContent := `---
description: test agent
---
# Agent
`
		if err := os.WriteFile(agentPath, []byte(agentContent), 0644); err != nil {
			t.Fatalf("write agent: %v", err)
		}

		result := runResourceValidate(agentPath, resourceValidateOptions{format: "table"})
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", result.ExitCode)
		}
		if !result.Output.Valid {
			t.Fatalf("expected valid result")
		}
		if result.Output.ResourceType != "agent" {
			t.Fatalf("expected agent, got %q", result.Output.ResourceType)
		}
	})

	t.Run("standalone command file outside commands dir", func(t *testing.T) {
		cmdPath := filepath.Join(t.TempDir(), "my-command.md")
		cmdContent := `---
description: test command
---
# Command
`
		if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
			t.Fatalf("write command: %v", err)
		}

		result := runResourceValidate(cmdPath, resourceValidateOptions{format: "table"})
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", result.ExitCode)
		}
		if !result.Output.Valid {
			t.Fatalf("expected valid result")
		}
		if result.Output.ResourceType != "command" {
			t.Fatalf("expected command, got %q", result.Output.ResourceType)
		}
	})

	t.Run("ambiguous bare markdown defaults to command", func(t *testing.T) {
		mdPath := filepath.Join(t.TempDir(), "ambiguous.md")
		content := "---\ndescription: ambiguous markdown\n---\n# Ambiguous\n"
		if err := os.WriteFile(mdPath, []byte(content), 0644); err != nil {
			t.Fatalf("write markdown: %v", err)
		}

		result := runResourceValidate(mdPath, resourceValidateOptions{format: "table"})
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", result.ExitCode)
		}
		if !result.Output.Valid {
			t.Fatalf("expected valid result")
		}
		if result.Output.ResourceType != "command" {
			t.Fatalf("expected command, got %q", result.Output.ResourceType)
		}
	})
}

func TestRunResourceValidate_CanonicalIDWithSourceRoot(t *testing.T) {
	root := t.TempDir()
	commandsDir := filepath.Join(root, "commands", "team")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cmdPath := filepath.Join(commandsDir, "deploy.md")
	if err := os.WriteFile(cmdPath, []byte("---\ndescription: deploy\n---\n# Deploy\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}

	result := runResourceValidate("command/team/deploy", resourceValidateOptions{
		format:     "table",
		sourceRoot: root,
	})

	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if !result.Output.Valid {
		t.Fatalf("expected valid result")
	}
	if result.Output.ResolvedID != "command/team/deploy" {
		t.Fatalf("unexpected resolved id: %q", result.Output.ResolvedID)
	}
	if result.Output.Context.Kind != "source-root" {
		t.Fatalf("expected source-root context, got %q", result.Output.Context.Kind)
	}
}

func TestRunResourceValidate_PackagePathWithSourceRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "commands", "team"), 0755); err != nil {
		t.Fatalf("mkdir commands: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "skills", "helper"), 0755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "agents"), 0755); err != nil {
		t.Fatalf("mkdir agents: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "packages"), 0755); err != nil {
		t.Fatalf("mkdir packages: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "commands", "team", "deploy.md"), []byte("---\ndescription: deploy\n---\n# Deploy\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "helper", "SKILL.md"), []byte("---\nname: helper\ndescription: helper\n---\n# Skill\n"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	pkgPath := filepath.Join(root, "packages", "team.package.json")
	pkgJSON := `{"name":"team-package","description":"team","resources":["command/team/deploy","skill/helper"]}`
	if err := os.WriteFile(pkgPath, []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package: %v", err)
	}

	result := runResourceValidate(pkgPath, resourceValidateOptions{format: "table", sourceRoot: root})
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
	if !result.Output.Valid {
		t.Fatalf("expected valid package result")
	}
	if result.Output.ResourceType != "package" {
		t.Fatalf("expected package type, got %q", result.Output.ResourceType)
	}
	if result.Output.Mode != "static+contextual" {
		t.Fatalf("expected static+contextual mode, got %q", result.Output.Mode)
	}
}

func TestRunResourceValidate_PackageCanonicalIDWithManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "commands", "ops"), 0755); err != nil {
		t.Fatalf("mkdir commands: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "skills", "docs"), 0755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "agents"), 0755); err != nil {
		t.Fatalf("mkdir agents: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "packages"), 0755); err != nil {
		t.Fatalf("mkdir packages: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "commands", "ops", "deploy.md"), []byte("---\ndescription: deploy\n---\n# Deploy\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "docs", "SKILL.md"), []byte("---\nname: docs\ndescription: docs\n---\n# Skill\n"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	pkgPath := filepath.Join(root, "packages", "ops-package.package.json")
	pkgJSON := `{"name":"ops-package","description":"ops","resources":["command/ops/deploy","skill/docs"]}`
	if err := os.WriteFile(pkgPath, []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package: %v", err)
	}

	manifestDir := t.TempDir()
	manifestPath := filepath.Join(manifestDir, "ai.repo.yaml")
	manifestYAML := "version: 1\nsources:\n  - name: local\n    path: " + root + "\n"
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	missingRepo := filepath.Join(t.TempDir(), "missing-repo")
	t.Setenv("AIMGR_REPO_PATH", missingRepo)

	result := runResourceValidate("package/ops-package", resourceValidateOptions{format: "table", repoManifest: manifestPath})
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (diagnostics=%+v)", result.ExitCode, result.Output.Diagnostics)
	}
	if result.Output.Context.Kind != "repo-manifest" {
		t.Fatalf("expected repo-manifest context, got %q", result.Output.Context.Kind)
	}
	if result.Output.ResolvedID != "package/ops-package" {
		t.Fatalf("unexpected resolved id: %q", result.Output.ResolvedID)
	}
}

func TestRunResourceValidate_PackageMissingReferenceIncludesSuggestion(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"commands/team", "skills", "agents", "packages"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "commands", "team", "deploy.md"), []byte("---\ndescription: deploy\n---\n# Deploy\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}

	pkgPath := filepath.Join(root, "packages", "team.package.json")
	pkgJSON := `{"name":"team-pkg","description":"team","resources":["command/team/deployy"]}`
	if err := os.WriteFile(pkgPath, []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package: %v", err)
	}

	result := runResourceValidate(pkgPath, resourceValidateOptions{format: "table", sourceRoot: root})
	if result.ExitCode != resourceValidateExitValidationError {
		t.Fatalf("expected validation exit code %d, got %d", resourceValidateExitValidationError, result.ExitCode)
	}
	if len(result.Output.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics for missing package reference")
	}
	d := result.Output.Diagnostics[0]
	if d.Code != "missing_package_ref" {
		t.Fatalf("expected missing_package_ref, got %q", d.Code)
	}
	if d.MissingReference != "command/team/deployy" {
		t.Fatalf("expected missing reference in diagnostic, got %q", d.MissingReference)
	}
	if !strings.Contains(d.Suggestion, "command/team/deploy") {
		t.Fatalf("expected suggestion with canonical ID, got %q", d.Suggestion)
	}
}

func TestRunResourceValidate_CanonicalIDRequiresContext(t *testing.T) {
	nonRepo := filepath.Join(t.TempDir(), "does-not-exist")
	t.Setenv("AIMGR_REPO_PATH", nonRepo)

	result := runResourceValidate("skill/my-skill", resourceValidateOptions{format: "table"})
	if result.ExitCode != resourceValidateExitUsageError {
		t.Fatalf("expected exit code %d, got %d", resourceValidateExitUsageError, result.ExitCode)
	}
	if result.Output.Valid {
		t.Fatalf("expected invalid result")
	}
	if len(result.Output.Diagnostics) == 0 || result.Output.Diagnostics[0].Code != "context_required" {
		t.Fatalf("expected context_required diagnostic, got %+v", result.Output.Diagnostics)
	}
}

func TestOutputResourceValidateResult_Formats(t *testing.T) {
	sample := &resourceValidationResult{
		Target:       "command/demo",
		ResolvedPath: "/tmp/commands/demo.md",
		ResolvedID:   "command/demo",
		ResourceType: "command",
		Mode:         "static",
		Context:      validateContext{Kind: "none"},
		Valid:        false,
		Diagnostics: []validateDiagnostic{{
			Severity: "error",
			Code:     "resource_validation_error",
			Message:  "invalid frontmatter",
		}},
		Summary: validateSummary{ErrorCount: 1, WarningCount: 0},
	}

	t.Run("json", func(t *testing.T) {
		out := captureValidateStdout(t, func() {
			err := outputResourceValidateResult(sample, sample.Valid, resourceValidateExitValidationError, "json")
			if err != nil {
				t.Fatalf("output json: %v", err)
			}
		})

		var parsed resourceValidationResult
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("parse json: %v", err)
		}
		if parsed.Target != sample.Target {
			t.Fatalf("target mismatch: %q", parsed.Target)
		}
	})

	t.Run("yaml", func(t *testing.T) {
		out := captureValidateStdout(t, func() {
			err := outputResourceValidateResult(sample, sample.Valid, resourceValidateExitValidationError, "yaml")
			if err != nil {
				t.Fatalf("output yaml: %v", err)
			}
		})

		var parsed resourceValidationResult
		if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("parse yaml: %v", err)
		}
		if parsed.Target != sample.Target {
			t.Fatalf("target mismatch: %q", parsed.Target)
		}
	})

	t.Run("table", func(t *testing.T) {
		out := captureValidateStdout(t, func() {
			err := outputResourceValidateResult(sample, sample.Valid, resourceValidateExitValidationError, "table")
			if err != nil {
				t.Fatalf("output table: %v", err)
			}
		})
		if !strings.Contains(out, "Resource validation") {
			t.Fatalf("expected table output header, got: %s", out)
		}
	})
}

func TestRunResourceValidate_DoesNotMutateRepoForPathValidation(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "repo-should-not-exist")
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	commandPath := filepath.Join(t.TempDir(), "standalone.md")
	if err := os.WriteFile(commandPath, []byte("---\ndescription: standalone\n---\n# Cmd\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}

	result := runResourceValidate(commandPath, resourceValidateOptions{format: "table"})
	if result.ExitCode != 0 {
		t.Fatalf("expected success, got %d", result.ExitCode)
	}

	if _, err := os.Stat(repoPath); !os.IsNotExist(err) {
		t.Fatalf("expected repo path to remain untouched, got err=%v", err)
	}
}

func captureValidateStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read capture: %v", err)
	}
	return buf.String()
}
