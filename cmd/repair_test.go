package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestRepair_CommandFlagsExist(t *testing.T) {
	expectedFlags := []string{"format", "prune-package", "dry-run", "project-path"}
	for _, flagName := range expectedFlags {
		flag := repairCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Expected flag --%s to be registered on repair command", flagName)
		}
	}

	if repairCmd.Flags().Lookup("reset") != nil {
		t.Fatalf("--reset should not be registered")
	}
	if repairCmd.Flags().Lookup("force") != nil {
		t.Fatalf("--force should not be registered")
	}
}

func TestRepairExpandManifestRefs_PackageExpansion(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	tempSkillDir := t.TempDir()
	skillDir := filepath.Join(tempSkillDir, "pkg-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: x\n---\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := manager.AddSkill(skillDir, "file://"+skillDir, "file"); err != nil {
		t.Fatalf("add skill: %v", err)
	}

	pkg := &resource.Package{Name: "my-pkg", Description: "pkg", Resources: []string{"skill/pkg-skill"}}
	if err := resource.SavePackage(pkg, repoDir); err != nil {
		t.Fatalf("save pkg: %v", err)
	}

	mf := &manifest.Manifest{Resources: []string{"package/my-pkg", "skill/pkg-skill"}}
	expanded, errs := expandManifestRefs(mf, repoDir)
	if len(errs) != 0 {
		t.Fatalf("unexpected expansion errors: %v", errs)
	}
	if len(expanded) != 1 || expanded[0] != "skill/pkg-skill" {
		t.Fatalf("unexpected expanded refs: %v", expanded)
	}
}

func TestRepairBuildReconcilePlan_DetectsFixInstallAndRemoval(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	owned := []OwnedResourceDir{{
		ResourceType: resource.Skill,
		Path:         filepath.Join(projectDir, ".opencode", "skills"),
	}}
	if err := os.MkdirAll(owned[0].Path, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// conflicting declared path => fix
	if err := os.WriteFile(filepath.Join(owned[0].Path, "declared-fix"), []byte("manual"), 0644); err != nil {
		t.Fatalf("write declared conflict: %v", err)
	}
	// undeclared path => removal
	if err := os.WriteFile(filepath.Join(owned[0].Path, "undeclared"), []byte("manual"), 0644); err != nil {
		t.Fatalf("write undeclared: %v", err)
	}

	declared := []string{"skill/declared-fix", "skill/declared-missing"}
	plan, err := buildReconcilePlan(repoDir, owned, declared)
	if err != nil {
		t.Fatalf("buildReconcilePlan failed: %v", err)
	}

	if len(plan.Fixes) != 1 || plan.Fixes[0].Resource != "skill/declared-fix" {
		t.Fatalf("expected one fix for declared-fix, got %+v", plan.Fixes)
	}
	if len(plan.Installs) != 1 || plan.Installs[0].Resource != "skill/declared-missing" {
		t.Fatalf("expected one install for declared-missing, got %+v", plan.Installs)
	}
	if len(plan.Removals) != 1 || !strings.Contains(plan.Removals[0].Path, "undeclared") {
		t.Fatalf("expected undeclared removal, got %+v", plan.Removals)
	}
}

func TestRepairDryRun_DoesNotModifyFilesystem(t *testing.T) {
	projectDir := t.TempDir()
	repoDir := t.TempDir()

	owned := []OwnedResourceDir{{
		ResourceType: resource.Skill,
		Path:         filepath.Join(projectDir, ".opencode", "skills"),
	}}
	if err := os.MkdirAll(owned[0].Path, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	undeclared := filepath.Join(owned[0].Path, "undeclared")
	if err := os.WriteFile(undeclared, []byte("manual"), 0644); err != nil {
		t.Fatalf("write undeclared: %v", err)
	}

	plan, err := buildReconcilePlan(repoDir, owned, []string{})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if len(plan.Removals) == 0 {
		t.Fatalf("expected planned removal")
	}

	if _, err := os.Stat(undeclared); err != nil {
		t.Fatalf("expected undeclared to exist after plan: %v", err)
	}
}

func TestRepair_JSONOutputSchema(t *testing.T) {
	result := RepairResult{
		DryRun: true,
		Planned: RepairPlan{
			Installs:     []RepairAction{{Resource: "skill/s1", IssueType: "not-installed"}},
			Fixes:        []RepairAction{},
			Removals:     []RepairAction{},
			PrunePackage: []RepairAction{},
		},
		Applied: RepairPlan{
			Installs:     []RepairAction{},
			Fixes:        []RepairAction{},
			Removals:     []RepairAction{},
			PrunePackage: []RepairAction{},
		},
		Failed:  []RepairErr{},
		Summary: RepairStats{PlannedInstalls: 1},
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	jsonStr := string(b)
	for _, key := range []string{"\"dry_run\"", "\"planned\"", "\"applied\"", "\"removals\"", "\"prune_package\"", "\"summary\""} {
		if !strings.Contains(jsonStr, key) {
			t.Fatalf("expected key %s in JSON: %s", key, jsonStr)
		}
	}
}

func TestRepairDisplayNoIssues_JSON(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = repairDisplayNoIssues(output.JSON)
	_ = w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatalf("repairDisplayNoIssues failed: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	var parsed RepairResult
	if err := json.Unmarshal(buf[:n], &parsed); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, string(buf[:n]))
	}
}

func TestRepairFindInvalidManifestRefs(t *testing.T) {
	repoDir := t.TempDir()
	manager := repo.NewManagerWithPath(repoDir)
	if err := manager.Init(); err != nil {
		t.Fatalf("Failed to init repo: %v", err)
	}

	m := &manifest.Manifest{Resources: []string{"skill/missing-skill", "package/missing-pkg"}}
	invalidRefs, partial := findInvalidManifestRefs(m, manager)
	if len(invalidRefs) != 2 {
		t.Fatalf("expected 2 invalid refs, got %v", invalidRefs)
	}
	if len(partial) != 0 {
		t.Fatalf("expected no partial package warnings, got %v", partial)
	}
}
