package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/install"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
	"github.com/spf13/cobra"
)

// RepairResult is the structured result of a repair operation.
type RepairResult struct {
	Fixed   []RepairAction `json:"fixed"`
	Failed  []RepairAction `json:"failed"`
	Hints   []RepairAction `json:"hints"`
	Summary RepairSummary  `json:"summary"`
}

// RepairAction records what was done (or attempted) for a single issue.
type RepairAction struct {
	Resource    string `json:"resource"`
	Tool        string `json:"tool"`
	IssueType   string `json:"issue_type"`
	Description string `json:"description"`
}

// RepairSummary holds counts from a repair run.
type RepairSummary struct {
	Fixed  int `json:"fixed"`
	Failed int `json:"failed"`
	Hints  int `json:"hints"`
}

var (
	repairFormatFlag  string
	repairResetFlag   bool
	repairPruneFlag   bool
	repairForceFlag   bool
	repairDryRunFlag  bool
	repairProjectPath string
)

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Repair installed resources in the current project",
	Long: `Repair all installed resources in the current project directory.

This command diagnoses issues with installed resources (reusing the same checks
as 'aimgr verify') and then automatically fixes them:

  - Broken symlinks        → remove and reinstall from repository
  - Wrong-repo symlinks    → remove and reinstall from correct repository
  - Missing resources      → install from repository (in ai.package.yaml)
  - Orphaned resources     → print hint to use 'aimgr uninstall'

Packages in ai.package.yaml are expanded to their constituent resources
before checking, so individual members are repaired as needed.

Examples:
  aimgr repair                           # Repair current directory
  aimgr repair --project-path ~/project  # Repair specific directory
  aimgr repair --format json             # JSON output for scripts
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get project path
		projectPath := repairProjectPath
		if projectPath == "" {
			var err error
			projectPath, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		// Get repo manager
		manager, err := NewManagerWithLogLevel()
		if err != nil {
			return err
		}
		repoPath := manager.GetRepoPath()

		// Detect tools
		detectedTools, err := tools.DetectExistingTools(projectPath)
		if err != nil {
			return fmt.Errorf("failed to detect tools: %w", err)
		}

		if len(detectedTools) == 0 && !repairPruneFlag {
			fmt.Println("No tool directories found in this project.")
			return nil
		}

		// Parse format flag
		parsedFormat, err := output.ParseFormat(repairFormatFlag)
		if err != nil {
			return err
		}

		var result RepairResult
		result.Fixed = make([]RepairAction, 0)
		result.Failed = make([]RepairAction, 0)
		result.Hints = make([]RepairAction, 0)

		if len(detectedTools) > 0 {
			// Phase 1: scan symlinks on disk
			issues, err := scanProjectIssues(projectPath, detectedTools, repoPath)
			if err != nil {
				return fmt.Errorf("repair scan failed: %w", err)
			}

			// Phase 2: check manifest vs disk
			manifestIssues, err := checkManifestSync(projectPath, detectedTools, repoPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to check manifest sync: %v\n", err)
			} else {
				issues = deduplicateIssues(issues, manifestIssues)
			}

			if len(issues) > 0 {
				result = applyRepairFixes(projectPath, issues, manager)
			}
		} else {
			fmt.Println("No tool directories found in this project.")
		}

		// --reset: scan for unmanaged files and offer to remove them
		if repairResetFlag {
			unmanaged, err := findUnmanagedFiles(projectPath, detectedTools, repoPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to scan for unmanaged files: %v\n", err)
			} else if len(unmanaged) > 0 {
				removed, err := promptAndRemoveUnmanaged(unmanaged, repairDryRunFlag, repairForceFlag)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: error removing unmanaged files: %v\n", err)
				}
				// Record removed files as additional repair actions
				for _, path := range removed {
					result.Fixed = append(result.Fixed, RepairAction{
						Resource:    path,
						Tool:        "",
						IssueType:   "unmanaged",
						Description: fmt.Sprintf("Removed unmanaged file: %s", path),
					})
				}
				result.Summary.Fixed += len(removed)
			} else {
				fmt.Println("No unmanaged files found.")
			}
		}

		// --prune-package: validate manifest refs against the repo
		if repairPruneFlag {
			manifestPath := filepath.Join(projectPath, manifest.ManifestFileName)
			if !manifest.Exists(manifestPath) {
				fmt.Printf("ℹ No %s found — nothing to prune\n", manifest.ManifestFileName)
			} else {
				m, err := manifest.Load(manifestPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to load manifest: %v\n", err)
				} else if len(m.Resources) == 0 {
					fmt.Printf("ℹ %s has no resource entries — nothing to prune\n", manifest.ManifestFileName)
				} else {
					invalidRefs, partialPkgs := findInvalidManifestRefs(m, manager)

					// Warn about partial packages (repo issue, not manifest issue)
					for _, pp := range partialPkgs {
						fmt.Printf("⚠ package/%s exists but has missing members: %v\n", pp.PackageName, pp.MissingMembers)
						fmt.Println("  → This is a repo issue. Run 'aimgr repo repair' to fix package definitions.")
					}

					if len(invalidRefs) == 0 {
						fmt.Println("✓ All manifest references are valid")
					} else {
						if err := resolveInvalidRefs(invalidRefs, m, manifestPath, manager, repairDryRunFlag, repairForceFlag, os.Stdin); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: error resolving invalid refs: %v\n", err)
						}
					}
				}
			}
		}

		// If nothing was done at all
		if result.Summary.Fixed == 0 && result.Summary.Failed == 0 && result.Summary.Hints == 0 && !repairPruneFlag && !repairResetFlag {
			return repairDisplayNoIssues(parsedFormat)
		}

		if result.Summary.Fixed > 0 || result.Summary.Failed > 0 || result.Summary.Hints > 0 {
			return repairDisplayResult(result, parsedFormat)
		}

		return nil
	},
}

// applyRepairFixes processes each issue and applies the appropriate fix.
// Returns a RepairResult summarising what was done.
func applyRepairFixes(projectPath string, issues []VerifyIssue, repoManager *repo.Manager) RepairResult {
	var result RepairResult
	result.Fixed = make([]RepairAction, 0)
	result.Failed = make([]RepairAction, 0)
	result.Hints = make([]RepairAction, 0)

	// Detect tools once for all reinstallation operations
	detectedTools, err := tools.DetectExistingTools(projectPath)
	if err != nil {
		// Can't continue without tools
		fmt.Fprintf(os.Stderr, "Failed to detect tools: %v\n", err)
		return result
	}

	for _, issue := range issues {
		switch issue.IssueType {

		case issueTypeBroken, issueTypeWrongRepo:
			action := applyRepairBrokenOrWrongRepo(projectPath, issue, repoManager, detectedTools)
			if action.Description == "" {
				// success — Description is filled only on error paths; use a fixed message
				result.Fixed = append(result.Fixed, RepairAction{
					Resource:    issue.Resource,
					Tool:        issue.Tool,
					IssueType:   issue.IssueType,
					Description: fmt.Sprintf("Reinstalled %s", issue.Resource),
				})
			} else {
				result.Failed = append(result.Failed, action)
			}

		case issueTypeNotInstalled:
			action := applyRepairNotInstalled(projectPath, issue, repoManager, detectedTools)
			if action.Description == "" {
				result.Fixed = append(result.Fixed, RepairAction{
					Resource:    issue.Resource,
					Tool:        issue.Tool,
					IssueType:   issue.IssueType,
					Description: fmt.Sprintf("Installed %s", issue.Resource),
				})
			} else {
				result.Failed = append(result.Failed, action)
			}

		case issueTypeOrphaned:
			resType, resName := parseResourceFromIssue(issue)
			ref := string(resType) + "/" + resName
			result.Hints = append(result.Hints, RepairAction{
				Resource:    issue.Resource,
				Tool:        issue.Tool,
				IssueType:   issue.IssueType,
				Description: fmt.Sprintf("Run 'aimgr uninstall %s' to remove, or run 'aimgr install %s' to add to %s", ref, ref, manifest.ManifestFileName),
			})

		case issueTypeUnreadable:
			result.Failed = append(result.Failed, RepairAction{
				Resource:    issue.Resource,
				Tool:        issue.Tool,
				IssueType:   issue.IssueType,
				Description: fmt.Sprintf("Unreadable symlink at %s — manual intervention required", issue.Path),
			})
		}
	}

	result.Summary = RepairSummary{
		Fixed:  len(result.Fixed),
		Failed: len(result.Failed),
		Hints:  len(result.Hints),
	}

	return result
}

// applyRepairBrokenOrWrongRepo fixes a broken or wrong-repo symlink by
// removing it and reinstalling from the repository.
// Returns an empty RepairAction on success, or one with a Description on failure.
func applyRepairBrokenOrWrongRepo(projectPath string, issue VerifyIssue, repoManager *repo.Manager, detectedTools []tools.Tool) RepairAction {
	fmt.Printf("  Fixing %s...\n", issue.Resource)

	// Remove broken symlink
	if err := os.Remove(issue.Path); err != nil {
		msg := fmt.Sprintf("Failed to remove symlink: %v", err)
		fmt.Fprintf(os.Stderr, "    %s\n", msg)
		return RepairAction{Resource: issue.Resource, Tool: issue.Tool, IssueType: issue.IssueType, Description: msg}
	}

	// Determine resource type and name
	resType, resName := parseResourceFromIssue(issue)

	// Check if resource still exists in repo
	if _, err := repoManager.Get(resName, resType); err != nil {
		msg := fmt.Sprintf("Resource '%s' no longer exists in repository — consider removing from %s", resName, manifest.ManifestFileName)
		fmt.Printf("    ✗ Removed broken symlink. %s\n", msg)
		fmt.Printf("      Run: aimgr uninstall %s/%s\n", resType, resName)
		// Symlink was removed; count this as partially fixed (symlink gone), but
		// we can't reinstall, so we put it in failed with an informative message.
		return RepairAction{Resource: issue.Resource, Tool: issue.Tool, IssueType: issue.IssueType, Description: msg}
	}

	// Reinstall using the installer
	installer, err := install.NewInstallerWithTargets(projectPath, detectedTools)
	if err != nil {
		msg := fmt.Sprintf("Failed to create installer: %v", err)
		fmt.Fprintf(os.Stderr, "    %s\n", msg)
		return RepairAction{Resource: issue.Resource, Tool: issue.Tool, IssueType: issue.IssueType, Description: msg}
	}

	if installErr := runInstall(installer, resType, resName, repoManager); installErr != nil {
		msg := fmt.Sprintf("Failed to reinstall: %v", installErr)
		fmt.Fprintf(os.Stderr, "    %s\n", msg)
		return RepairAction{Resource: issue.Resource, Tool: issue.Tool, IssueType: issue.IssueType, Description: msg}
	}

	fmt.Printf("    ✓ Reinstalled %s/%s\n", resType, resName)
	return RepairAction{} // empty = success
}

// applyRepairNotInstalled installs a resource that's listed in the manifest
// but not present on disk.
// Returns an empty RepairAction on success, or one with a Description on failure.
func applyRepairNotInstalled(projectPath string, issue VerifyIssue, repoManager *repo.Manager, detectedTools []tools.Tool) RepairAction {
	fmt.Printf("  Installing %s...\n", issue.Resource)

	// Parse "type/name" from the resource reference
	resType, resName, err := resource.ParseResourceReference(issue.Resource)
	if err != nil {
		msg := fmt.Sprintf("Cannot parse resource reference '%s': %v", issue.Resource, err)
		fmt.Fprintf(os.Stderr, "    %s\n", msg)
		return RepairAction{Resource: issue.Resource, Tool: issue.Tool, IssueType: issue.IssueType, Description: msg}
	}

	// Check if resource exists in repo
	if _, err := repoManager.Get(resName, resType); err != nil {
		msg := fmt.Sprintf("Resource '%s' not found in repository — remove from %s or run 'aimgr repo add'", issue.Resource, manifest.ManifestFileName)
		fmt.Printf("    ✗ %s\n", msg)
		return RepairAction{Resource: issue.Resource, Tool: issue.Tool, IssueType: issue.IssueType, Description: msg}
	}

	// Install it
	installer, err := install.NewInstallerWithTargets(projectPath, detectedTools)
	if err != nil {
		msg := fmt.Sprintf("Failed to create installer: %v", err)
		fmt.Fprintf(os.Stderr, "    %s\n", msg)
		return RepairAction{Resource: issue.Resource, Tool: issue.Tool, IssueType: issue.IssueType, Description: msg}
	}

	if installErr := runInstall(installer, resType, resName, repoManager); installErr != nil {
		msg := fmt.Sprintf("Failed to install: %v", installErr)
		fmt.Fprintf(os.Stderr, "    %s\n", msg)
		return RepairAction{Resource: issue.Resource, Tool: issue.Tool, IssueType: issue.IssueType, Description: msg}
	}

	fmt.Printf("    ✓ Installed %s\n", issue.Resource)
	return RepairAction{} // empty = success
}

// runInstall dispatches to the correct installer method based on resource type.
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

// repairDisplayNoIssues prints the "nothing to fix" message in the requested format.
func repairDisplayNoIssues(format output.Format) error {
	switch format {
	case output.JSON:
		result := RepairResult{
			Fixed:   []RepairAction{},
			Failed:  []RepairAction{},
			Hints:   []RepairAction{},
			Summary: RepairSummary{},
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)

	case output.Table:
		fmt.Println("✓ All installed resources are valid — nothing to repair")
		return nil

	default:
		fmt.Println("✓ All installed resources are valid — nothing to repair")
		return nil
	}
}

// repairDisplayResult outputs the repair result in the requested format.
func repairDisplayResult(result RepairResult, format output.Format) error {
	switch format {
	case output.JSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)

	case output.Table:
		fmt.Println()
		if result.Summary.Fixed > 0 {
			fmt.Printf("✓ Fixed %d issue(s)\n", result.Summary.Fixed)
		}
		if result.Summary.Failed > 0 {
			fmt.Printf("✗ Failed to fix %d issue(s)\n", result.Summary.Failed)
			for _, a := range result.Failed {
				fmt.Printf("  - %s (%s): %s\n", a.Resource, a.IssueType, a.Description)
			}
		}
		if result.Summary.Hints > 0 {
			fmt.Printf("\n%d orphaned resource(s) found (not auto-removed):\n", result.Summary.Hints)
			for _, h := range result.Hints {
				fmt.Printf("  - %s (%s): %s\n", h.Resource, h.Tool, h.Description)
			}
		}
		return nil

	default:
		fmt.Println()
		if result.Summary.Fixed > 0 {
			fmt.Printf("✓ Fixed %d issue(s)\n", result.Summary.Fixed)
		}
		if result.Summary.Failed > 0 {
			fmt.Printf("✗ Failed to fix %d issue(s)\n", result.Summary.Failed)
		}
		return nil
	}
}

func init() {
	rootCmd.AddCommand(repairCmd)
	repairCmd.Flags().StringVar(&repairProjectPath, "project-path", "", "Project directory path (default: current directory)")
	repairCmd.Flags().StringVar(&repairFormatFlag, "format", "table", "Output format (table|json)")
	repairCmd.Flags().BoolVar(&repairResetFlag, "reset", false, "Remove unmanaged files from resource directories after repair")
	repairCmd.Flags().BoolVar(&repairPruneFlag, "prune-package", false, "Remove invalid resource references from ai.package.yaml")
	repairCmd.Flags().BoolVar(&repairForceFlag, "force", false, "Skip confirmation prompts when removing unmanaged files")
	repairCmd.Flags().BoolVar(&repairDryRunFlag, "dry-run", false, "Show what would be removed without making changes")

	// Register completion functions
	_ = repairCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)
}

// findUnmanagedFiles scans resource subdirectories (commands/, skills/, agents/) for each
// detected tool and returns the absolute paths of files that are NOT managed by aimgr.
// A file is managed if it is a symlink whose target starts with repoPath.
// Regular files, broken symlinks to non-repo locations, and symlinks to non-repo locations
// are all considered unmanaged.
// Namespace directories (one-level sub-directories in commands/) are recursed one level.
// The root tool directory itself (settings.json etc.) is NOT scanned.
func findUnmanagedFiles(projectPath string, detectedTools []tools.Tool, repoPath string) ([]string, error) {
	var unmanaged []string

	for _, tool := range detectedTools {
		toolInfo := tools.GetToolInfo(tool)

		if toolInfo.SupportsCommands {
			dir := filepath.Join(projectPath, toolInfo.CommandsDir)
			found, err := scanResourceDirForUnmanaged(dir, repoPath, true)
			if err != nil {
				return nil, fmt.Errorf("scanning %s: %w", dir, err)
			}
			unmanaged = append(unmanaged, found...)
		}

		if toolInfo.SupportsSkills {
			dir := filepath.Join(projectPath, toolInfo.SkillsDir)
			found, err := scanResourceDirForUnmanaged(dir, repoPath, false)
			if err != nil {
				return nil, fmt.Errorf("scanning %s: %w", dir, err)
			}
			unmanaged = append(unmanaged, found...)
		}

		if toolInfo.SupportsAgents {
			dir := filepath.Join(projectPath, toolInfo.AgentsDir)
			found, err := scanResourceDirForUnmanaged(dir, repoPath, false)
			if err != nil {
				return nil, fmt.Errorf("scanning %s: %w", dir, err)
			}
			unmanaged = append(unmanaged, found...)
		}
	}

	return unmanaged, nil
}

// scanResourceDirForUnmanaged scans a single resource directory for unmanaged files.
// If allowNamespace is true, subdirectories are recursed one level (for commands/).
// Returns absolute paths of unmanaged files/symlinks.
func scanResourceDirForUnmanaged(dir, repoPath string, allowNamespace bool) ([]string, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var unmanaged []string
	for _, entry := range entries {
		entryPath := filepath.Join(dir, entry.Name())

		info, err := os.Lstat(entryPath)
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			// It's a symlink — check if it's managed
			target, err := os.Readlink(entryPath)
			if err != nil {
				// Unreadable symlink — treat as unmanaged
				unmanaged = append(unmanaged, entryPath)
				continue
			}
			if !strings.HasPrefix(target, repoPath) {
				unmanaged = append(unmanaged, entryPath)
			}
			// else: managed symlink — skip
		} else if info.IsDir() {
			if !allowNamespace {
				// Non-symlink directory in a non-namespace resource dir — unmanaged
				unmanaged = append(unmanaged, entryPath)
				continue
			}
			// Recurse one level for namespace commands
			subEntries, err := os.ReadDir(entryPath)
			if err != nil {
				continue
			}
			hasUnmanaged := false
			for _, subEntry := range subEntries {
				if subEntry.IsDir() {
					continue // Only one level of nesting
				}
				subPath := filepath.Join(entryPath, subEntry.Name())
				subInfo, err := os.Lstat(subPath)
				if err != nil {
					continue
				}
				if subInfo.Mode()&os.ModeSymlink != 0 {
					target, err := os.Readlink(subPath)
					if err != nil {
						unmanaged = append(unmanaged, subPath)
						hasUnmanaged = true
						continue
					}
					if !strings.HasPrefix(target, repoPath) {
						unmanaged = append(unmanaged, subPath)
						hasUnmanaged = true
					}
				} else {
					// Regular file inside namespace dir — unmanaged
					unmanaged = append(unmanaged, subPath)
					hasUnmanaged = true
				}
			}
			// If the namespace directory itself is now empty after all its contents
			// are flagged as unmanaged, it will be cleaned up in promptAndRemoveUnmanaged.
			// We need to track whether to add the dir itself.
			_ = hasUnmanaged
		} else {
			// Regular file in resource dir — unmanaged
			unmanaged = append(unmanaged, entryPath)
		}
	}

	return unmanaged, nil
}

// promptAndRemoveUnmanaged shows the list of unmanaged files and handles removal.
// In dry-run mode: prints what would be removed, returns empty slice.
// In force mode: removes all without prompting.
// Otherwise: prompts user for confirmation.
// Returns the list of files that were actually removed.
func promptAndRemoveUnmanaged(unmanaged []string, dryRun, force bool) ([]string, error) {
	if len(unmanaged) == 0 {
		return nil, nil
	}

	if dryRun {
		fmt.Printf("\nWould remove %d unmanaged file(s):\n", len(unmanaged))
		for _, path := range unmanaged {
			fmt.Printf("  Would remove: %s\n", path)
		}
		return nil, nil
	}

	// Show the list of unmanaged files
	fmt.Printf("\nFound %d unmanaged file(s) in resource directories:\n", len(unmanaged))
	for _, path := range unmanaged {
		fmt.Printf("  %s\n", path)
	}
	fmt.Println()

	if !force {
		fmt.Printf("Remove all %d unmanaged files? [y/N]: ", len(unmanaged))
		var response string
		_, _ = fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Reset cancelled.")
			return nil, nil
		}
	}

	// Remove all unmanaged files
	var removed []string
	var firstErr error
	for _, path := range unmanaged {
		info, err := os.Lstat(path)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to stat %s: %w", path, err)
			}
			continue
		}

		if info.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ Failed to remove directory %s: %v\n", path, err)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
		} else {
			if err := os.Remove(path); err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ Failed to remove %s: %v\n", path, err)
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
		}

		fmt.Printf("  ✓ Removed: %s\n", path)
		removed = append(removed, path)
	}

	// Clean up any empty namespace directories left behind
	cleanupEmptyNamespaceDirs(unmanaged)

	fmt.Printf("\n✓ Removed %d unmanaged file(s)\n", len(removed))
	return removed, firstErr
}

// cleanupEmptyNamespaceDirs removes any empty parent directories that were left
// behind after removing unmanaged files from namespace subdirectories.
func cleanupEmptyNamespaceDirs(removedPaths []string) {
	// Collect unique parent directories
	parentDirs := make(map[string]struct{})
	for _, path := range removedPaths {
		parent := filepath.Dir(path)
		parentDirs[parent] = struct{}{}
	}

	for dir := range parentDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		if len(entries) == 0 {
			if err := os.Remove(dir); err == nil {
				fmt.Printf("  ✓ Removed empty directory: %s\n", dir)
			}
		}
	}
}

// PartialPackageWarning describes a package that exists in the repo but has
// members that are missing on disk. This is a repo-level issue, not a manifest issue.
type PartialPackageWarning struct {
	PackageName    string
	MissingMembers []string
}

// findInvalidManifestRefs checks each resource reference in the manifest against
// the repository. It returns:
//   - invalidRefs: references that should be removed (resource/package not found)
//   - partialPkgs: packages that exist but have missing member resources (repo issue)
func findInvalidManifestRefs(m *manifest.Manifest, manager *repo.Manager) (invalidRefs []string, partialPkgs []PartialPackageWarning) {
	for _, ref := range m.Resources {
		// Split on first "/" to get type and name
		idx := strings.Index(ref, "/")
		if idx < 0 {
			// Invalid format — treat as invalid ref
			invalidRefs = append(invalidRefs, ref)
			continue
		}
		typeStr := ref[:idx]
		name := ref[idx+1:]

		if typeStr == "package" {
			// Check if package exists
			pkg, err := manager.GetPackage(name)
			if err != nil {
				// Package not found in repo
				invalidRefs = append(invalidRefs, ref)
				continue
			}

			// Package exists — check if all its members exist
			if len(pkg.Resources) > 0 {
				missing := manager.ValidatePackageResources(pkg)
				if len(missing) > 0 {
					// Members missing — this is a repo issue, not a manifest issue
					partialPkgs = append(partialPkgs, PartialPackageWarning{
						PackageName:    name,
						MissingMembers: missing,
					})
				}
			}
			// Empty package or all members present — valid
		} else {
			// Individual resource: command, skill, or agent
			var resType resource.ResourceType
			switch typeStr {
			case "command":
				resType = resource.Command
			case "skill":
				resType = resource.Skill
			case "agent":
				resType = resource.Agent
			default:
				// Unknown type — treat as invalid
				invalidRefs = append(invalidRefs, ref)
				continue
			}

			if _, err := manager.Get(name, resType); err != nil {
				invalidRefs = append(invalidRefs, ref)
			}
		}
	}
	return invalidRefs, partialPkgs
}

// resolveInvalidRefs handles invalid manifest references based on the mode:
//   - dry-run: print what would be removed
//   - force: remove all without prompting
//   - interactive: offer escalation choices per reference
//
// The input parameter is used for reading interactive responses (typically os.Stdin
// but can be replaced in tests).
func resolveInvalidRefs(invalidRefs []string, m *manifest.Manifest, manifestPath string, manager *repo.Manager, dryRun, force bool, input *os.File) error {
	if dryRun {
		fmt.Printf("\nWould remove from %s:\n", manifest.ManifestFileName)
		for _, ref := range invalidRefs {
			fmt.Printf("  Would remove: %s\n", ref)
		}
		return nil
	}

	if force {
		for _, ref := range invalidRefs {
			if err := m.Remove(ref); err != nil {
				return fmt.Errorf("failed to remove %s from manifest: %w", ref, err)
			}
			fmt.Printf("  ✓ Removed %s from %s\n", ref, manifest.ManifestFileName)
		}
		return m.Save(manifestPath)
	}

	// Interactive mode: escalation flow
	scanner := bufio.NewScanner(input)
	repoSyncTried := false
	repoRepairTried := false
	manifestModified := false

	for _, ref := range invalidRefs {
		for {
			fmt.Printf("\n⚠ %s not found in repo\n\n", ref)
			fmt.Println("? How to resolve:")

			// Build options dynamically
			type option struct {
				label  string
				action string
			}
			var opts []option
			if !repoSyncTried {
				opts = append(opts, option{"Run repo sync first (repo sources may be outdated)", "sync"})
			}
			if !repoRepairTried {
				opts = append(opts, option{"Run repo repair first (repo metadata may be broken)", "repair"})
			}
			opts = append(opts, option{"Remove from " + manifest.ManifestFileName, "remove"})
			opts = append(opts, option{"Skip (do nothing)", "skip"})

			for i, opt := range opts {
				fmt.Printf("  [%d] %s\n", i+1, opt.label)
			}
			fmt.Printf("Choice [1-%d]: ", len(opts))

			var choice int
			scanner.Scan()
			line := strings.TrimSpace(scanner.Text())
			_, err := fmt.Sscanf(line, "%d", &choice)
			if err != nil || choice < 1 || choice > len(opts) {
				fmt.Println("Invalid choice, please try again.")
				continue
			}

			selectedAction := opts[choice-1].action
			switch selectedAction {
			case "sync":
				repoSyncTried = true
				fmt.Println("\nPlease run 'aimgr repo sync' in another terminal, then press Enter to re-check...")
				scanner.Scan() // wait for Enter
				// Re-check if the ref is now valid
				if refIsValid(ref, manager) {
					fmt.Printf("  ✓ %s is now valid after repo sync\n", ref)
					goto nextRef
				}
				fmt.Printf("  ✗ %s still not found after repo sync\n", ref)
				// Continue loop without the sync option

			case "repair":
				repoRepairTried = true
				fmt.Println("\nPlease run 'aimgr repo repair' in another terminal, then press Enter to re-check...")
				scanner.Scan() // wait for Enter
				// Re-check if the ref is now valid
				if refIsValid(ref, manager) {
					fmt.Printf("  ✓ %s is now valid after repo repair\n", ref)
					goto nextRef
				}
				fmt.Printf("  ✗ %s still not found after repo repair\n", ref)
				// Continue loop without the repair option

			case "remove":
				if err := m.Remove(ref); err != nil {
					return fmt.Errorf("failed to remove %s: %w", ref, err)
				}
				fmt.Printf("  ✓ Removed %s from %s\n", ref, manifest.ManifestFileName)
				manifestModified = true
				goto nextRef

			case "skip":
				fmt.Printf("  → Skipped %s\n", ref)
				goto nextRef
			}
		}
	nextRef:
	}

	if manifestModified {
		if err := m.Save(manifestPath); err != nil {
			return fmt.Errorf("failed to save manifest: %w", err)
		}
		fmt.Printf("\n✓ Saved %s\n", manifest.ManifestFileName)
	}

	return nil
}

// refIsValid checks whether a manifest reference (e.g. "skill/foo" or "package/bar")
// currently exists in the repository.
func refIsValid(ref string, manager *repo.Manager) bool {
	idx := strings.Index(ref, "/")
	if idx < 0 {
		return false
	}
	typeStr := ref[:idx]
	name := ref[idx+1:]

	if typeStr == "package" {
		_, err := manager.GetPackage(name)
		return err == nil
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
		return false
	}

	_, err := manager.Get(name, resType)
	return err == nil
}
