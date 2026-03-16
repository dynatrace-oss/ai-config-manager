package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/install"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
	"github.com/spf13/cobra"
)

type RepairResult struct {
	DryRun  bool        `json:"dry_run"`
	Planned RepairPlan  `json:"planned"`
	Applied RepairPlan  `json:"applied"`
	Failed  []RepairErr `json:"failed"`
	Summary RepairStats `json:"summary"`
}

type RepairPlan struct {
	Installs     []RepairAction `json:"installs"`
	Fixes        []RepairAction `json:"fixes"`
	Removals     []RepairAction `json:"removals"`
	PrunePackage []RepairAction `json:"prune_package"`
}

type RepairAction struct {
	Resource    string `json:"resource"`
	Tool        string `json:"tool,omitempty"`
	Path        string `json:"path,omitempty"`
	IssueType   string `json:"issue_type"`
	Description string `json:"description"`
}

type RepairErr struct {
	IssueType string `json:"issue_type"`
	Resource  string `json:"resource,omitempty"`
	Path      string `json:"path,omitempty"`
	Message   string `json:"message"`
}

type RepairStats struct {
	PlannedInstalls     int `json:"planned_installs"`
	PlannedFixes        int `json:"planned_fixes"`
	PlannedRemovals     int `json:"planned_removals"`
	PlannedPrunePackage int `json:"planned_prune_package"`
	AppliedInstalls     int `json:"applied_installs"`
	AppliedFixes        int `json:"applied_fixes"`
	AppliedRemovals     int `json:"applied_removals"`
	AppliedPrunePackage int `json:"applied_prune_package"`
	Failures            int `json:"failures"`
}

var (
	repairFormatFlag  string
	repairPruneFlag   bool
	repairDryRunFlag  bool
	repairProjectPath string
)

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Reconcile project resources with ai.package.yaml",
	Long: `Reconcile owned resource directories with ai.package.yaml.

This command:
  - Validates ai.package.yaml before any destructive action
  - Expands package/* references to concrete resources
  - Installs/fixes declared resources first
  - Removes undeclared content from owned resource directories afterwards

Optional manifest cleanup:
  --prune-package removes invalid references from ai.package.yaml

Use --dry-run to preview all planned actions without changing files.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath := repairProjectPath
		if projectPath == "" {
			var err error
			projectPath, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		parsedFormat, err := output.ParseFormat(repairFormatFlag)
		if err != nil {
			return err
		}

		manager, err := NewManagerWithLogLevel()
		if err != nil {
			return err
		}

		ownedDirs, err := detectOwnedResourceDirs(projectPath)
		if err != nil {
			return fmt.Errorf("failed to detect owned resource directories: %w", err)
		}
		if len(ownedDirs) == 0 && !repairPruneFlag {
			fmt.Println("No tool directories found in this project.")
			return nil
		}

		manifestPath := filepath.Join(projectPath, manifest.ManifestFileName)
		mf, err := manifest.Load(manifestPath)
		if err != nil {
			return fmt.Errorf("failed to load %s: %w", manifest.ManifestFileName, err)
		}

		expanded, expandErrs := expandManifestRefs(mf, manager.GetRepoPath())

		result := RepairResult{
			DryRun: repairDryRunFlag,
			Planned: RepairPlan{
				Installs:     make([]RepairAction, 0),
				Fixes:        make([]RepairAction, 0),
				Removals:     make([]RepairAction, 0),
				PrunePackage: make([]RepairAction, 0),
			},
			Applied: RepairPlan{
				Installs:     make([]RepairAction, 0),
				Fixes:        make([]RepairAction, 0),
				Removals:     make([]RepairAction, 0),
				PrunePackage: make([]RepairAction, 0),
			},
			Failed: make([]RepairErr, 0),
		}

		for _, e := range expandErrs {
			result.Failed = append(result.Failed, RepairErr{IssueType: "manifest", Message: e.Error()})
		}

		reconcilePlan, err := buildReconcilePlan(manager.GetRepoPath(), ownedDirs, expanded)
		if err != nil {
			return err
		}
		result.Planned.Installs = append(result.Planned.Installs, reconcilePlan.Installs...)
		result.Planned.Fixes = append(result.Planned.Fixes, reconcilePlan.Fixes...)
		result.Planned.Removals = append(result.Planned.Removals, reconcilePlan.Removals...)

		if repairPruneFlag {
			invalidRefs, partialPkgs := findInvalidManifestRefs(mf, manager)
			for _, pp := range partialPkgs {
				result.Failed = append(result.Failed, RepairErr{
					IssueType: "prune-package-warning",
					Resource:  "package/" + pp.PackageName,
					Message:   fmt.Sprintf("package has missing members in repo metadata: %s", strings.Join(pp.MissingMembers, ", ")),
				})
			}
			for _, ref := range invalidRefs {
				result.Planned.PrunePackage = append(result.Planned.PrunePackage, RepairAction{
					Resource:    ref,
					IssueType:   "prune-package",
					Description: fmt.Sprintf("Remove invalid reference from %s", manifest.ManifestFileName),
				})
			}
		}

		if !repairDryRunFlag {
			if len(result.Failed) == 0 {
				if err := applyReconcilePlan(projectPath, manager, ownedDirs, reconcilePlan, &result); err != nil {
					result.Failed = append(result.Failed, RepairErr{IssueType: "repair", Message: err.Error()})
				}
			}

			if repairPruneFlag && len(result.Planned.PrunePackage) > 0 {
				for _, action := range result.Planned.PrunePackage {
					if err := mf.Remove(action.Resource); err != nil {
						result.Failed = append(result.Failed, RepairErr{IssueType: "prune-package", Resource: action.Resource, Message: err.Error()})
						continue
					}
					result.Applied.PrunePackage = append(result.Applied.PrunePackage, action)
				}
				if len(result.Applied.PrunePackage) > 0 {
					if err := mf.Save(manifestPath); err != nil {
						result.Failed = append(result.Failed, RepairErr{IssueType: "prune-package", Message: fmt.Sprintf("failed to save %s: %v", manifest.ManifestFileName, err)})
						result.Applied.PrunePackage = nil
					}
				}
			}
		}

		result.Summary = repairStats(result)
		if noPlannedWork(result) && len(result.Failed) == 0 {
			return repairDisplayNoIssues(parsedFormat)
		}

		return repairDisplayResult(result, parsedFormat)
	},
}

type reconcilePlan struct {
	Installs []RepairAction
	Fixes    []RepairAction
	Removals []RepairAction
}

func buildReconcilePlan(repoPath string, ownedDirs []OwnedResourceDir, declaredRefs []string) (reconcilePlan, error) {
	plan := reconcilePlan{
		Installs: make([]RepairAction, 0),
		Fixes:    make([]RepairAction, 0),
		Removals: make([]RepairAction, 0),
	}

	declaredSet := make(map[string]struct{}, len(declaredRefs))
	declaredPaths := make(map[string]struct{})

	for _, ref := range declaredRefs {
		declaredSet[ref] = struct{}{}
		resType, resName, err := resource.ParseResourceReference(ref)
		if err != nil {
			continue
		}

		paths := desiredInstallPaths(ownedDirs, resType, resName)
		if len(paths) == 0 {
			plan.Fixes = append(plan.Fixes, RepairAction{
				Resource:    ref,
				IssueType:   "no-target",
				Description: "No detected owned directory supports this resource type",
			})
			continue
		}

		needsInstall := false
		fixReason := ""
		for _, targetPath := range paths {
			declaredPaths[targetPath.path] = struct{}{}
			state, err := inspectPath(targetPath.path, repoPath)
			if err != nil {
				return plan, err
			}
			switch state {
			case "missing":
				needsInstall = true
			case "healthy":
				// noop
			default:
				fixReason = state
			}
		}

		if fixReason != "" {
			plan.Fixes = append(plan.Fixes, RepairAction{
				Resource:    ref,
				IssueType:   fixReason,
				Description: "Replace conflicting or broken installation",
			})
			continue
		}
		if needsInstall {
			plan.Installs = append(plan.Installs, RepairAction{
				Resource:    ref,
				IssueType:   "not-installed",
				Description: "Install declared resource",
			})
		}
	}

	removals, err := collectUndeclaredPaths(ownedDirs, declaredPaths)
	if err != nil {
		return plan, err
	}
	for _, p := range removals {
		plan.Removals = append(plan.Removals, RepairAction{
			Resource:    p,
			Path:        p,
			IssueType:   "undeclared",
			Description: "Remove undeclared content from owned directory",
		})
	}

	return plan, nil
}

func applyReconcilePlan(projectPath string, manager *repo.Manager, ownedDirs []OwnedResourceDir, plan reconcilePlan, result *RepairResult) error {
	targetTools := toolsFromOwnedDirs(ownedDirs)
	installer, err := install.NewInstallerWithTargets(projectPath, targetTools)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	declaredFailures := 0
	for _, action := range plan.Fixes {
		if err := removeDeclaredPathsForRef(ownedDirs, action.Resource); err != nil {
			declaredFailures++
			result.Failed = append(result.Failed, RepairErr{IssueType: action.IssueType, Resource: action.Resource, Message: err.Error()})
			continue
		}
		if err := installRef(installer, manager, action.Resource); err != nil {
			declaredFailures++
			result.Failed = append(result.Failed, RepairErr{IssueType: action.IssueType, Resource: action.Resource, Message: err.Error()})
			continue
		}
		result.Applied.Fixes = append(result.Applied.Fixes, action)
	}

	for _, action := range plan.Installs {
		if err := installRef(installer, manager, action.Resource); err != nil {
			declaredFailures++
			result.Failed = append(result.Failed, RepairErr{IssueType: action.IssueType, Resource: action.Resource, Message: err.Error()})
			continue
		}
		result.Applied.Installs = append(result.Applied.Installs, action)
	}

	if declaredFailures > 0 {
		return nil
	}

	for _, action := range plan.Removals {
		if err := os.RemoveAll(action.Path); err != nil {
			result.Failed = append(result.Failed, RepairErr{IssueType: action.IssueType, Path: action.Path, Message: err.Error()})
			continue
		}
		result.Applied.Removals = append(result.Applied.Removals, action)
	}

	return nil
}

func installRef(installer *install.Installer, manager *repo.Manager, ref string) error {
	resType, resName, err := resource.ParseResourceReference(ref)
	if err != nil {
		return err
	}
	return runInstall(installer, resType, resName, manager)
}

func runInstall(installer *install.Installer, resType resource.ResourceType, resName string, repoManager *repo.Manager) error {
	switch resType {
	case resource.Command:
		return installer.InstallCommand(resName, repoManager)
	case resource.Skill:
		return installer.InstallSkill(resName, repoManager)
	case resource.Agent:
		return installer.InstallAgent(resName, repoManager)
	default:
		return fmt.Errorf("unsupported resource type: %s", resType)
	}
}

type installPath struct {
	tool tools.Tool
	path string
}

func desiredInstallPaths(ownedDirs []OwnedResourceDir, resType resource.ResourceType, resName string) []installPath {
	result := make([]installPath, 0)
	for _, owned := range ownedDirs {
		if owned.ResourceType != resType {
			continue
		}
		switch resType {
		case resource.Command:
			result = append(result, installPath{tool: owned.Tool, path: filepath.Join(owned.Path, resName+".md")})
		case resource.Skill:
			result = append(result, installPath{tool: owned.Tool, path: filepath.Join(owned.Path, resName)})
		case resource.Agent:
			result = append(result, installPath{tool: owned.Tool, path: filepath.Join(owned.Path, tools.AgentArtifactName(owned.Tool, resName))})
		}
	}
	return result
}

func removeDeclaredPathsForRef(ownedDirs []OwnedResourceDir, ref string) error {
	resType, resName, err := resource.ParseResourceReference(ref)
	if err != nil {
		return err
	}
	for _, p := range desiredInstallPaths(ownedDirs, resType, resName) {
		_, statErr := os.Lstat(p.path)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			return statErr
		}
		if err := os.RemoveAll(p.path); err != nil {
			return err
		}
	}
	return nil
}

func inspectPath(path, repoPath string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "missing", nil
		}
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "conflict", nil
	}
	if _, err := os.Stat(path); err != nil {
		return "broken", nil
	}
	target, err := os.Readlink(path)
	if err != nil {
		return "unreadable", nil
	}
	if !filepath.IsAbs(target) {
		target = filepath.Clean(filepath.Join(filepath.Dir(path), target))
	}
	if !strings.HasPrefix(target, repoPath) {
		return "wrong-repo", nil
	}
	return "healthy", nil
}

func collectUndeclaredPaths(ownedDirs []OwnedResourceDir, declaredPaths map[string]struct{}) ([]string, error) {
	removeSet := make(map[string]struct{})

	for _, owned := range ownedDirs {
		if _, err := os.Stat(owned.Path); os.IsNotExist(err) {
			continue
		}

		walk := []string{owned.Path}
		for len(walk) > 0 {
			current := walk[0]
			walk = walk[1:]

			entries, err := os.ReadDir(current)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				full := filepath.Join(current, entry.Name())
				if _, ok := declaredPaths[full]; ok {
					continue
				}
				if entry.IsDir() {
					walk = append(walk, full)
					if hasDeclaredChild(full, declaredPaths) {
						continue
					}
					removeSet[full] = struct{}{}
					continue
				}
				removeSet[full] = struct{}{}
			}
		}
	}

	removals := make([]string, 0, len(removeSet))
	for p := range removeSet {
		removals = append(removals, p)
	}
	sort.Slice(removals, func(i, j int) bool {
		if strings.Count(removals[i], string(os.PathSeparator)) == strings.Count(removals[j], string(os.PathSeparator)) {
			return removals[i] < removals[j]
		}
		return strings.Count(removals[i], string(os.PathSeparator)) > strings.Count(removals[j], string(os.PathSeparator))
	})
	return removals, nil
}

func hasDeclaredChild(path string, declaredPaths map[string]struct{}) bool {
	prefix := path + string(os.PathSeparator)
	for candidate := range declaredPaths {
		if strings.HasPrefix(candidate, prefix) {
			return true
		}
	}
	return false
}

func expandManifestRefs(mf *manifest.Manifest, repoPath string) ([]string, []error) {
	ordered := make([]string, 0)
	seen := make(map[string]struct{})
	errs := make([]error, 0)

	for _, ref := range mf.Resources {
		if strings.HasPrefix(ref, "package/") {
			pkgName := strings.TrimPrefix(ref, "package/")
			pkg, err := resource.LoadPackage(resource.GetPackagePath(pkgName, repoPath))
			if err != nil {
				errs = append(errs, fmt.Errorf("package/%s: %w", pkgName, err))
				continue
			}
			for _, member := range pkg.Resources {
				if _, ok := seen[member]; ok {
					continue
				}
				seen[member] = struct{}{}
				ordered = append(ordered, member)
			}
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		ordered = append(ordered, ref)
	}

	return ordered, errs
}

func repairStats(result RepairResult) RepairStats {
	return RepairStats{
		PlannedInstalls:     len(result.Planned.Installs),
		PlannedFixes:        len(result.Planned.Fixes),
		PlannedRemovals:     len(result.Planned.Removals),
		PlannedPrunePackage: len(result.Planned.PrunePackage),
		AppliedInstalls:     len(result.Applied.Installs),
		AppliedFixes:        len(result.Applied.Fixes),
		AppliedRemovals:     len(result.Applied.Removals),
		AppliedPrunePackage: len(result.Applied.PrunePackage),
		Failures:            len(result.Failed),
	}
}

func noPlannedWork(result RepairResult) bool {
	return len(result.Planned.Installs) == 0 &&
		len(result.Planned.Fixes) == 0 &&
		len(result.Planned.Removals) == 0 &&
		len(result.Planned.PrunePackage) == 0
}

func repairDisplayNoIssues(format output.Format) error {
	switch format {
	case output.JSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(RepairResult{
			DryRun: false,
			Planned: RepairPlan{
				Installs:     []RepairAction{},
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
			Summary: RepairStats{},
		})
	default:
		fmt.Println("✓ Project already matches ai.package.yaml")
		return nil
	}
}

func repairDisplayResult(result RepairResult, format output.Format) error {
	switch format {
	case output.JSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case output.Table:
		if result.DryRun {
			fmt.Println("Repair dry-run plan:")
		} else {
			fmt.Println("Repair reconciliation result:")
		}
		printRepairActions("Installs", result.Planned.Installs, result.Applied.Installs, result.DryRun)
		printRepairActions("Fixes/Replacements", result.Planned.Fixes, result.Applied.Fixes, result.DryRun)
		printRepairActions("Removals", result.Planned.Removals, result.Applied.Removals, result.DryRun)
		printRepairActions("Prune-package", result.Planned.PrunePackage, result.Applied.PrunePackage, result.DryRun)

		if len(result.Failed) > 0 {
			fmt.Println("\nFailures:")
			for _, f := range result.Failed {
				if f.Resource != "" {
					fmt.Printf("  - [%s] %s: %s\n", f.IssueType, f.Resource, f.Message)
					continue
				}
				if f.Path != "" {
					fmt.Printf("  - [%s] %s: %s\n", f.IssueType, f.Path, f.Message)
					continue
				}
				fmt.Printf("  - [%s] %s\n", f.IssueType, f.Message)
			}
		}

		fmt.Printf("\nSummary: planned installs=%d, fixes=%d, removals=%d, prune-package=%d",
			result.Summary.PlannedInstalls,
			result.Summary.PlannedFixes,
			result.Summary.PlannedRemovals,
			result.Summary.PlannedPrunePackage,
		)
		if !result.DryRun {
			fmt.Printf(" | applied installs=%d, fixes=%d, removals=%d, prune-package=%d",
				result.Summary.AppliedInstalls,
				result.Summary.AppliedFixes,
				result.Summary.AppliedRemovals,
				result.Summary.AppliedPrunePackage,
			)
		}
		fmt.Printf(" | failures=%d\n", result.Summary.Failures)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func printRepairActions(title string, planned, applied []RepairAction, dryRun bool) {
	if len(planned) == 0 {
		return
	}
	fmt.Printf("\n%s (%d):\n", title, len(planned))
	for _, action := range planned {
		status := "planned"
		if !dryRun && containsAction(applied, action) {
			status = "applied"
		}
		pathInfo := ""
		if action.Path != "" {
			pathInfo = " [" + action.Path + "]"
		}
		fmt.Printf("  - (%s) %s%s\n", status, action.Resource, pathInfo)
	}
}

func containsAction(actions []RepairAction, candidate RepairAction) bool {
	for _, action := range actions {
		if action.Resource == candidate.Resource && action.Path == candidate.Path && action.IssueType == candidate.IssueType {
			return true
		}
	}
	return false
}

type PartialPackageWarning struct {
	PackageName    string
	MissingMembers []string
}

func findInvalidManifestRefs(m *manifest.Manifest, manager *repo.Manager) (invalidRefs []string, partialPkgs []PartialPackageWarning) {
	for _, ref := range m.Resources {
		idx := strings.Index(ref, "/")
		if idx < 0 {
			invalidRefs = append(invalidRefs, ref)
			continue
		}
		typeStr := ref[:idx]
		name := ref[idx+1:]

		if typeStr == "package" {
			pkg, err := manager.GetPackage(name)
			if err != nil {
				invalidRefs = append(invalidRefs, ref)
				continue
			}
			if len(pkg.Resources) > 0 {
				missing := manager.ValidatePackageResources(pkg)
				if len(missing) > 0 {
					partialPkgs = append(partialPkgs, PartialPackageWarning{PackageName: name, MissingMembers: missing})
				}
			}
			continue
		}

		var resType resource.ResourceType
		switch typeStr {
		case "command":
			resType = resource.Command
		case "skill":
			resType = resource.Skill
		case "agent":
			resType = resource.Agent
		default:
			invalidRefs = append(invalidRefs, ref)
			continue
		}

		if _, err := manager.Get(name, resType); err != nil {
			invalidRefs = append(invalidRefs, ref)
		}
	}
	return invalidRefs, partialPkgs
}

func init() {
	rootCmd.AddCommand(repairCmd)
	repairCmd.Flags().StringVar(&repairProjectPath, "project-path", "", "Project directory path (default: current directory)")
	repairCmd.Flags().StringVar(&repairFormatFlag, "format", "table", "Output format (table|json)")
	repairCmd.Flags().BoolVar(&repairPruneFlag, "prune-package", false, "Remove invalid resource references from ai.package.yaml")
	repairCmd.Flags().BoolVar(&repairDryRunFlag, "dry-run", false, "Preview planned actions without making changes")

	_ = repairCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)
}
