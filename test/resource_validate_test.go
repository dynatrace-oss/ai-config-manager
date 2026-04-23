package test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"gopkg.in/yaml.v3"
)

func TestCLIResourceValidate_PathStandaloneCommand(t *testing.T) {
	setupTestEnvironment(t)

	cmdPath := filepath.Join(t.TempDir(), "standalone.md")
	if err := os.WriteFile(cmdPath, []byte("---\ndescription: standalone command\n---\n# Cmd\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}

	output, err := runAimgr(t, "resource", "validate", cmdPath)
	if err != nil {
		t.Fatalf("validate failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "valid") {
		t.Fatalf("expected valid output, got: %s", output)
	}
	if !strings.Contains(output, "command") {
		t.Fatalf("expected command type in output, got: %s", output)
	}
}

func TestCLIResourceValidate_PathAmbiguousMarkdownDefaultsToCommand(t *testing.T) {
	setupTestEnvironment(t)

	cmdPath := filepath.Join(t.TempDir(), "ambiguous.md")
	if err := os.WriteFile(cmdPath, []byte("---\ndescription: ambiguous markdown\n---\n# Cmd\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}

	output, err := runAimgr(t, "resource", "validate", "--format=json", cmdPath)
	if err != nil {
		t.Fatalf("validate failed: %v\nOutput: %s", err, output)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("parse json output: %v\nOutput: %s", err, output)
	}

	if parsed["resource_type"] != "command" {
		t.Fatalf("expected resource_type=command, got: %v", parsed["resource_type"])
	}
	if parsed["valid"] != true {
		t.Fatalf("expected valid=true, got: %v", parsed["valid"])
	}
}

func TestCLIResourceValidate_CanonicalIDWithSourceRoot_JSON(t *testing.T) {
	setupTestEnvironment(t)

	sourceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceRoot, "skills", "demo-skill"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "skills", "demo-skill", "SKILL.md"), []byte("---\nname: demo-skill\ndescription: demo\n---\n# Skill\n"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	output, err := runAimgr(t, "resource", "validate", "--format=json", "--source-root", sourceRoot, "skill/demo-skill")
	if err != nil {
		t.Fatalf("validate failed: %v\nOutput: %s", err, output)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("parse json output: %v\nOutput: %s", err, output)
	}

	if parsed["valid"] != true {
		t.Fatalf("expected valid=true, got: %v", parsed["valid"])
	}
	if parsed["resource_type"] != "skill" {
		t.Fatalf("expected resource_type=skill, got: %v", parsed["resource_type"])
	}
	if parsed["resolved_id"] != "skill/demo-skill" {
		t.Fatalf("expected resolved_id, got: %v", parsed["resolved_id"])
	}
}

func TestCLIResourceValidate_OutputYAML(t *testing.T) {
	setupTestEnvironment(t)

	agentPath := filepath.Join(t.TempDir(), "agent.md")
	if err := os.WriteFile(agentPath, []byte("---\ndescription: agent\n---\n# Agent\n"), 0644); err != nil {
		t.Fatalf("write agent: %v", err)
	}

	output, err := runAimgr(t, "resource", "validate", "--format=yaml", agentPath)
	if err != nil {
		t.Fatalf("validate failed: %v\nOutput: %s", err, output)
	}

	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("parse yaml output: %v\nOutput: %s", err, output)
	}

	if parsed["valid"] != true {
		t.Fatalf("expected valid=true, got: %v", parsed["valid"])
	}
}

func TestCLIResourceValidate_ExitCodeValidationError(t *testing.T) {
	setupTestEnvironment(t)

	invalidSkill := filepath.Join(t.TempDir(), "bad-skill")
	if err := os.MkdirAll(invalidSkill, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(invalidSkill, "SKILL.md"), []byte("---\nname: bad-skill\n---\n# Skill\n"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	_, err := runAimgr(t, "resource", "validate", invalidSkill)
	if err == nil {
		t.Fatalf("expected non-zero exit")
	}

	if code := commandExitCode(err); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestCLIResourceValidate_ExitCodeUsageError(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "missing-repo")
	t.Setenv("AIMGR_REPO_PATH", repoPath)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	_, err := runAimgr(t, "resource", "validate", "skill/without-context")
	if err == nil {
		t.Fatalf("expected non-zero exit")
	}

	if code := commandExitCode(err); code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestCLIResourceValidate_DoesNotMutateRepoPathForPathValidation(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "uncreated-repo")
	t.Setenv("AIMGR_REPO_PATH", repoPath)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	commandPath := filepath.Join(t.TempDir(), "cmd.md")
	if err := os.WriteFile(commandPath, []byte("---\ndescription: standalone\n---\n# Command\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}

	if _, err := runAimgr(t, "resource", "validate", commandPath); err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	if _, err := os.Stat(repoPath); !os.IsNotExist(err) {
		t.Fatalf("expected repo path to remain absent, got err=%v", err)
	}
}

func TestCLIResourceValidate_PackagePathWithSourceRoot_JSON(t *testing.T) {
	setupTestEnvironment(t)

	root := t.TempDir()
	for _, dir := range []string{"commands/team", "skills/helper", "agents", "packages"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(root, "commands", "team", "deploy.md"), []byte("---\ndescription: deploy\n---\n# Deploy\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "helper", "SKILL.md"), []byte("---\nname: helper\ndescription: helper\n---\n# Skill\n"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	pkgPath := filepath.Join(root, "packages", "team.package.json")
	pkgJSON := `{"name":"team-pkg","description":"team","resources":["command/team/deploy","skill/helper"]}`
	if err := os.WriteFile(pkgPath, []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package: %v", err)
	}

	output, err := runAimgr(t, "resource", "validate", "--format=json", "--source-root", root, pkgPath)
	if err != nil {
		t.Fatalf("validate failed: %v\nOutput: %s", err, output)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("parse json output: %v\nOutput: %s", err, output)
	}

	if parsed["resource_type"] != "package" {
		t.Fatalf("expected resource_type=package, got: %v", parsed["resource_type"])
	}
	if parsed["valid"] != true {
		t.Fatalf("expected valid=true, got: %v", parsed["valid"])
	}
}

func TestCLIResourceValidate_PackageMissingRefExitCodeAndDiagnostic(t *testing.T) {
	setupTestEnvironment(t)

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

	output, err := runAimgr(t, "resource", "validate", "--format=json", "--source-root", root, pkgPath)
	if err == nil {
		t.Fatalf("expected non-zero exit for missing refs")
	}
	if code := commandExitCode(err); code != 1 {
		t.Fatalf("expected exit code 1, got %d\nOutput: %s", code, output)
	}
	if !strings.Contains(output, "missing_package_ref") {
		t.Fatalf("expected missing_package_ref diagnostic, got: %s", output)
	}
	if !strings.Contains(output, "command/team/deploy") {
		t.Fatalf("expected canonical suggestion in output, got: %s", output)
	}
}

func TestCLIResourceValidate_PackageValidationDoesNotMutateLiveRepo(t *testing.T) {
	setupTestEnvironment(t)

	liveRepo := t.TempDir()
	liveManager := repo.NewManagerWithPath(liveRepo)
	if err := liveManager.Init(); err != nil {
		t.Fatalf("init live repo: %v", err)
	}

	manifestPath := filepath.Join(liveRepo, "ai.repo.yaml")
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest before: %v", err)
	}

	t.Setenv("AIMGR_REPO_PATH", liveRepo)

	validationRoot := t.TempDir()
	for _, dir := range []string{"commands/team", "skills", "agents", "packages"} {
		if err := os.MkdirAll(filepath.Join(validationRoot, dir), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(validationRoot, "commands", "team", "deploy.md"), []byte("---\ndescription: deploy\n---\n# Deploy\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}
	pkgPath := filepath.Join(validationRoot, "packages", "team.package.json")
	pkgJSON := `{"name":"team-pkg","description":"team","resources":["command/team/deploy"]}`
	if err := os.WriteFile(pkgPath, []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package: %v", err)
	}

	if output, err := runAimgr(t, "resource", "validate", "--source-root", validationRoot, pkgPath); err != nil {
		t.Fatalf("validate failed: %v\nOutput: %s", err, output)
	}

	after, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("expected live repo manifest unchanged")
	}
}

func TestCLIResourceValidate_MissingRepoPath_ManifestBackedOutputIsMachineReadable(t *testing.T) {
	missingRepo := filepath.Join(t.TempDir(), "missing-repo")
	t.Setenv("AIMGR_REPO_PATH", missingRepo)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	root := t.TempDir()
	for _, dir := range []string{"commands/team", "skills/helper", "agents", "packages"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(root, "commands", "team", "deploy.md"), []byte("---\ndescription: deploy\n---\n# Deploy\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "helper", "SKILL.md"), []byte("---\nname: helper\ndescription: helper\n---\n# Skill\n"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	pkgPath := filepath.Join(root, "packages", "release-bundle.package.json")
	pkgJSON := `{"name":"release-bundle","description":"bundle","resources":["command/team/deploy","skill/helper"]}`
	if err := os.WriteFile(pkgPath, []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package: %v", err)
	}

	manifestPath := filepath.Join(t.TempDir(), "ai.repo.yaml")
	manifestYAML := "version: 1\nsources:\n  - name: local\n    path: " + root + "\n"
	if err := os.WriteFile(manifestPath, []byte(manifestYAML), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	t.Run("json", func(t *testing.T) {
		output, err := runAimgr(t, "resource", "validate", "--format=json", "--repo-manifest", manifestPath, "package/release-bundle")
		if err != nil {
			t.Fatalf("validate failed: %v\nOutput: %s", err, output)
		}
		if strings.Contains(output, "Warning: failed to initialize logger") {
			t.Fatalf("unexpected warning prefix in machine-readable output: %s", output)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(output), &parsed); err != nil {
			t.Fatalf("parse json output: %v\nOutput: %s", err, output)
		}
		if parsed["valid"] != true {
			t.Fatalf("expected valid=true, got: %v", parsed["valid"])
		}
		if parsed["resource_type"] != "package" {
			t.Fatalf("expected resource_type=package, got: %v", parsed["resource_type"])
		}

		ctx, ok := parsed["context"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected context object, got: %T", parsed["context"])
		}
		if ctx["kind"] != "repo-manifest" {
			t.Fatalf("expected context kind repo-manifest, got: %v", ctx["kind"])
		}
	})

	t.Run("yaml", func(t *testing.T) {
		output, err := runAimgr(t, "resource", "validate", "--format=yaml", "--repo-manifest", manifestPath, "package/release-bundle")
		if err != nil {
			t.Fatalf("validate failed: %v\nOutput: %s", err, output)
		}
		if strings.Contains(output, "Warning: failed to initialize logger") {
			t.Fatalf("unexpected warning prefix in machine-readable output: %s", output)
		}

		var parsed map[string]interface{}
		if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
			t.Fatalf("parse yaml output: %v\nOutput: %s", err, output)
		}
		if parsed["valid"] != true {
			t.Fatalf("expected valid=true, got: %v", parsed["valid"])
		}
		if parsed["resource_type"] != "package" {
			t.Fatalf("expected resource_type=package, got: %v", parsed["resource_type"])
		}

		ctx, ok := parsed["context"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected context object, got: %T", parsed["context"])
		}
		if ctx["kind"] != "repo-manifest" {
			t.Fatalf("expected context kind repo-manifest, got: %v", ctx["kind"])
		}
	})
}

func TestCLIResourceValidate_MissingRepoPath_MissingContextOutputIsMachineReadable(t *testing.T) {
	missingRepo := filepath.Join(t.TempDir(), "missing-repo")
	t.Setenv("AIMGR_REPO_PATH", missingRepo)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	t.Run("json", func(t *testing.T) {
		output, err := runAimgr(t, "resource", "validate", "--format=json", "skill/without-context")
		if err == nil {
			t.Fatalf("expected non-zero exit")
		}
		if code := commandExitCode(err); code != 2 {
			t.Fatalf("expected exit code 2, got %d\nOutput: %s", code, output)
		}
		if strings.Contains(output, "Warning: failed to initialize logger") {
			t.Fatalf("unexpected warning prefix in machine-readable output: %s", output)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(output), &parsed); err != nil {
			t.Fatalf("parse json output: %v\nOutput: %s", err, output)
		}
		if parsed["valid"] != false {
			t.Fatalf("expected valid=false, got: %v", parsed["valid"])
		}
	})

	t.Run("yaml", func(t *testing.T) {
		output, err := runAimgr(t, "resource", "validate", "--format=yaml", "skill/without-context")
		if err == nil {
			t.Fatalf("expected non-zero exit")
		}
		if code := commandExitCode(err); code != 2 {
			t.Fatalf("expected exit code 2, got %d\nOutput: %s", code, output)
		}
		if strings.Contains(output, "Warning: failed to initialize logger") {
			t.Fatalf("unexpected warning prefix in machine-readable output: %s", output)
		}

		var parsed map[string]interface{}
		if err := yaml.Unmarshal([]byte(output), &parsed); err != nil {
			t.Fatalf("parse yaml output: %v\nOutput: %s", err, output)
		}
		if parsed["valid"] != false {
			t.Fatalf("expected valid=false, got: %v", parsed["valid"])
		}
	})
}

func TestCLIResourceValidate_GoldenMachineReadableBaselines(t *testing.T) {
	setupTestEnvironment(t)

	scenario := t.TempDir()
	for _, dir := range []string{"commands/team", "skills/helper", "agents", "packages"} {
		if err := os.MkdirAll(filepath.Join(scenario, dir), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(scenario, "commands", "team", "deploy.md"), []byte("---\ndescription: deploy\n---\n# Deploy\n"), 0644); err != nil {
		t.Fatalf("write command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scenario, "skills", "helper", "SKILL.md"), []byte("---\nname: helper\ndescription: helper\n---\n# Skill\n"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	missingPkgPath := filepath.Join(scenario, "packages", "team.package.json")
	pkgJSON := `{"name":"team-pkg","description":"team","resources":["command/team/deployy"]}`
	if err := os.WriteFile(missingPkgPath, []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("write package: %v", err)
	}

	t.Run("json validation error baseline", func(t *testing.T) {
		output, err := runAimgr(t, "resource", "validate", "--format=json", "--source-root", scenario, missingPkgPath)
		if err == nil {
			t.Fatalf("expected non-zero exit for missing refs")
		}
		if code := commandExitCode(err); code != 1 {
			t.Fatalf("expected exit code 1, got %d\nOutput: %s", code, output)
		}

		stable := normalizeValidateMachineReadableOutput(t, output, scenario)
		assertTestGoldenText(t, "resource_validate/package_missing_ref.json", stable)
	})

	t.Run("yaml validation error baseline", func(t *testing.T) {
		output, err := runAimgr(t, "resource", "validate", "--format=yaml", "--source-root", scenario, missingPkgPath)
		if err == nil {
			t.Fatalf("expected non-zero exit for missing refs")
		}
		if code := commandExitCode(err); code != 1 {
			t.Fatalf("expected exit code 1, got %d\nOutput: %s", code, output)
		}

		stable := normalizeValidateMachineReadableOutput(t, output, scenario)
		assertTestGoldenText(t, "resource_validate/package_missing_ref.yaml", stable)
	})
}

func normalizeValidateMachineReadableOutput(t *testing.T, output string, scenarioRoot string) string {
	t.Helper()

	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err == nil {
		if diags, ok := parsed["diagnostics"].([]any); ok {
			slices.SortFunc(diags, func(a, b any) int {
				am := a.(map[string]any)
				bm := b.(map[string]any)
				ac := am["code"].(string)
				bc := bm["code"].(string)
				if ac < bc {
					return -1
				}
				if ac > bc {
					return 1
				}
				return 0
			})
			parsed["diagnostics"] = diags
		}
		stable, marshalErr := json.MarshalIndent(parsed, "", "  ")
		if marshalErr != nil {
			t.Fatalf("failed to marshal normalized JSON output: %v", marshalErr)
		}
		normalized := strings.ReplaceAll(string(stable), scenarioRoot, "<SCENARIO_ROOT>")
		return normalized + "\n"
	}

	var parsedYAML map[string]any
	if err := yaml.Unmarshal([]byte(output), &parsedYAML); err == nil {
		if diags, ok := parsedYAML["diagnostics"].([]any); ok {
			slices.SortFunc(diags, func(a, b any) int {
				am := a.(map[string]any)
				bm := b.(map[string]any)
				ac := am["code"].(string)
				bc := bm["code"].(string)
				if ac < bc {
					return -1
				}
				if ac > bc {
					return 1
				}
				return 0
			})
			parsedYAML["diagnostics"] = diags
		}
		stable, marshalErr := yaml.Marshal(parsedYAML)
		if marshalErr != nil {
			t.Fatalf("failed to marshal normalized YAML output: %v", marshalErr)
		}
		normalized := strings.ReplaceAll(strings.ReplaceAll(string(stable), "\r\n", "\n"), scenarioRoot, "<SCENARIO_ROOT>")
		return normalized
	}

	t.Fatalf("unexpected machine-readable output format:\n%s", output)
	return ""
}

func assertTestGoldenText(t *testing.T, relPath string, actual string) {
	t.Helper()

	goldenPath := filepath.Join("testdata", "golden", relPath)
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	if os.Getenv("AIMGR_UPDATE_BASELINES") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatalf("failed to create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0644); err != nil {
			t.Fatalf("failed to write golden file %s: %v", goldenPath, err)
		}
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file %s: %v", goldenPath, err)
	}

	if actual != string(expected) {
		t.Fatalf("golden mismatch for %s\nset AIMGR_UPDATE_BASELINES=1 to refresh", goldenPath)
	}
}

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
