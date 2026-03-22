package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/tools"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var projectVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify installed resources in the current project",
	Long: `Verify all installed resources in the current project directory.

This command checks for common installation issues:
  - Broken symlinks (target doesn't exist)
  - Symlinks pointing to wrong repository
  - Resources in ai.package.yaml that aren't installed
  - Undeclared content in owned directories (not in ai.package.yaml)

Use 'aimgr repair' to reconcile project resources to ai.package.yaml.

Examples:
  aimgr verify                           # Check current directory
  aimgr verify --project-path ~/project  # Check specific directory
  aimgr repair                           # Reconcile owned directories to ai.package.yaml
  aimgr verify --format json             # JSON output for scripts
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get project path
		projectPath := verifyProjectPath
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

		ownedDirs, err := detectOwnedResourceDirs(projectPath)
		if err != nil {
			return fmt.Errorf("failed to detect tools: %w", err)
		}
		detectedTools := toolsFromOwnedDirs(ownedDirs)

		if len(ownedDirs) == 0 && !manifest.Exists(filepath.Join(projectPath, manifest.ManifestFileName)) && !manifest.Exists(filepath.Join(projectPath, manifest.LocalManifestFileName)) {
			fmt.Println("No tool directories found in this project.")
			return nil
		}

		// Parse format flag
		parsedFormat, err := output.ParseFormat(verifyFormatFlag2)
		if err != nil {
			return err
		}

		// --fix is deprecated and now acts as a warning-emitting wrapper around repair.
		if verifyFixFlag {
			fmt.Fprintln(os.Stderr, "Warning: --fix is deprecated. Running 'aimgr repair' reconciliation.")
			return runVerifyFixWrapper(projectPath, manager, parsedFormat, ownedDirs)
		}

		// Scan for issues (Phase 1: check symlink integrity on disk)
		issues, err := scanProjectIssues(projectPath, detectedTools, repoPath)
		if err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}

		// Check manifest vs installed (Phase 2: check manifest references)
		manifestIssues, err := checkManifestSync(projectPath, detectedTools, repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to check manifest sync: %v\n", err)
		} else {
			// Deduplicate: Phase 1 may already report a broken symlink for a resource
			// that Phase 2 also reports as "not-installed" (since os.Stat fails for
			// broken symlinks). Remove Phase 2 duplicates to avoid double-reporting.
			issues = deduplicateIssues(issues, manifestIssues)
		}

		// Report results
		if len(issues) == 0 {
			return displayNoIssues(parsedFormat)
		}

		// Display issues
		if parsedFormat == output.Table {
			fmt.Printf("Found %d issue(s):\n\n", len(issues))
		}
		if err := displayVerifyIssues(issues, parsedFormat); err != nil {
			return err
		}

		// Show fix suggestion only for table format
		if parsedFormat == output.Table {
			fmt.Println("\nRun 'aimgr repair' to reconcile these issues")
		}
		return nil
	},
}

type VerifyIssue struct {
	Resource    string
	Tool        string
	IssueType   string // issueTypeBroken, issueTypeWrongRepo, issueTypeNotInstalled, issueTypeUndeclared
	Description string
	Path        string
	Severity    string // "error", "warning"
}

// Issue type constants for VerifyIssue.IssueType
const (
	issueTypeBroken       = "broken"
	issueTypeWrongRepo    = "wrong-repo"
	issueTypeNotInstalled = "not-installed"
	issueTypeUndeclared   = "undeclared"
	issueTypeUnreadable   = "unreadable"
)

// deduplicateIssues merges manifest issues into existing issues, dropping any
// manifest "not-installed" issue whose resource name matches an existing issue
// (e.g., a "broken" symlink already detected in Phase 1).
func deduplicateIssues(existing, manifestIssues []VerifyIssue) []VerifyIssue {
	// Build a set of resource names already reported by Phase 1
	seen := make(map[string]bool, len(existing))
	for _, issue := range existing {
		seen[issue.Resource] = true
	}

	result := make([]VerifyIssue, len(existing))
	copy(result, existing)

	for _, issue := range manifestIssues {
		// For "not-installed" issues, extract the resource name (strip type prefix)
		// and check if Phase 1 already reported it
		if issue.IssueType == issueTypeNotInstalled {
			resName := issue.Resource
			if parts := strings.SplitN(resName, "/", 2); len(parts) == 2 {
				resName = parts[1]
			}
			if seen[resName] {
				continue // Skip duplicate — already reported by Phase 1
			}
		}
		result = append(result, issue)
	}

	return result
}

func scanProjectIssues(projectPath string, detectedTools []tools.Tool, repoPath string) ([]VerifyIssue, error) {
	var issues []VerifyIssue

	for _, tool := range detectedTools {
		toolInfo := tools.GetToolInfo(tool)

		// Check commands
		if toolInfo.SupportsCommands {
			commandsDir := filepath.Join(projectPath, toolInfo.CommandsDir)
			found, err := verifyDirectory(commandsDir, tool, repoPath)
			if err != nil {
				return nil, err
			}
			issues = append(issues, found...)
		}

		// Check skills
		if toolInfo.SupportsSkills {
			skillsDir := filepath.Join(projectPath, toolInfo.SkillsDir)
			found, err := verifyDirectory(skillsDir, tool, repoPath)
			if err != nil {
				return nil, err
			}
			issues = append(issues, found...)
		}

		// Check agents
		if toolInfo.SupportsAgents {
			agentsDir := filepath.Join(projectPath, toolInfo.AgentsDir)
			found, err := verifyDirectory(agentsDir, tool, repoPath)
			if err != nil {
				return nil, err
			}
			issues = append(issues, found...)
		}
	}

	return issues, nil
}

func verifyDirectory(dir string, tool tools.Tool, repoPath string) ([]VerifyIssue, error) {
	toolName := tool.String()
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var issues []VerifyIssue
	for _, entry := range entries {
		symlinkPath := filepath.Join(dir, entry.Name())

		// Check if it's a symlink
		linkInfo, err := os.Lstat(symlinkPath)
		if err != nil {
			continue
		}

		if linkInfo.Mode()&os.ModeSymlink == 0 {
			// Not a symlink — if it's a directory, recurse one level
			// for nested resources (e.g., namespaced commands like api/deploy.md)
			if linkInfo.IsDir() {
				subEntries, err := os.ReadDir(symlinkPath)
				if err != nil {
					continue
				}
				for _, subEntry := range subEntries {
					if subEntry.IsDir() {
						continue // Only one level of nesting
					}
					subPath := filepath.Join(symlinkPath, subEntry.Name())
					subInfo, err := os.Lstat(subPath)
					if err != nil {
						continue
					}
					if subInfo.Mode()&os.ModeSymlink == 0 {
						continue
					}
					namePart := strings.TrimSuffix(subEntry.Name(), ".md")
					if strings.Contains(strings.ToLower(dir), "/agents") {
						logicalName, ok := tools.AgentLogicalName(tool, subEntry.Name())
						if !ok {
							continue
						}
						namePart = logicalName
					}
					namespacedName := entry.Name() + "/" + namePart
					if issue := verifySymlink(subPath, namespacedName, toolName, repoPath); issue != nil {
						issues = append(issues, *issue)
					}
				}
			}
			continue
		}

		resourceName := entry.Name()
		if strings.Contains(strings.ToLower(dir), "/agents") {
			logicalName, ok := tools.AgentLogicalName(tool, entry.Name())
			if !ok {
				continue
			}
			resourceName = logicalName
		}

		if issue := verifySymlink(symlinkPath, resourceName, toolName, repoPath); issue != nil {
			issues = append(issues, *issue)
		}
	}

	return issues, nil
}

// verifySymlink checks a single symlink and returns a VerifyIssue if there is a problem.
// Returns nil when the symlink is healthy.
func verifySymlink(symlinkPath, resourceName, tool, repoPath string) *VerifyIssue {
	// Read target
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		return &VerifyIssue{
			Resource:    resourceName,
			Tool:        tool,
			IssueType:   issueTypeUnreadable,
			Description: "Cannot read symlink target",
			Path:        symlinkPath,
			Severity:    "error",
		}
	}

	// Check if target exists (os.Stat follows symlinks, detects broken ones)
	if _, err := os.Stat(symlinkPath); err != nil {
		return &VerifyIssue{
			Resource:    resourceName,
			Tool:        tool,
			IssueType:   issueTypeBroken,
			Description: fmt.Sprintf("Symlink target doesn't exist: %s", target),
			Path:        symlinkPath,
			Severity:    "error",
		}
	}

	// Check if points to correct repo
	if !strings.HasPrefix(target, repoPath) {
		return &VerifyIssue{
			Resource:    resourceName,
			Tool:        tool,
			IssueType:   issueTypeWrongRepo,
			Description: fmt.Sprintf("Points to wrong repo: %s (expected: %s)", target, repoPath),
			Path:        symlinkPath,
			Severity:    "warning",
		}
	}

	return nil
}

func checkManifestSync(projectPath string, detectedTools []tools.Tool, repoPath string) ([]VerifyIssue, error) {
	mf, view, err := loadEffectiveProjectManifest(projectPath)
	if err != nil {
		return nil, err
	}
	if mf == nil {
		return nil, nil // No project manifest files, no sync issues
	}

	var issues []VerifyIssue

	// Phase 2a: Manifest → disk (check each resource in manifest is installed)
	for _, resourceRef := range mf.Resources {
		declManifestName, declManifestPath := declaringManifestForResource(view, resourceRef)

		// Parse resource type and name
		parts := strings.SplitN(resourceRef, "/", 2)
		if len(parts) != 2 {
			continue
		}
		resType := parts[0] // "skill", "command", "agent", "package"
		resName := parts[1]

		// Special handling for packages - check if all constituent resources are installed
		if resType == "package" {
			pkgIssues := checkPackageInstalled(resourceRef, resName, projectPath, declManifestName, declManifestPath, detectedTools, repoPath)
			issues = append(issues, pkgIssues...)
			continue
		}

		// Check if regular resource is installed in any tool
		if !isResourceInstalledInTools(resType, resName, projectPath, detectedTools) {
			issues = append(issues, VerifyIssue{
				Resource:    resourceRef,
				Tool:        "any",
				IssueType:   issueTypeNotInstalled,
				Description: fmt.Sprintf("Listed in %s but not installed", declManifestName),
				Path:        declManifestPath,
				Severity:    "warning",
			})
		}
	}

	// Phase 2b: Disk → manifest (undeclared content in owned directories)
	undeclaredIssues := findUndeclaredInOwnedDirs(mf, projectPath, repoPath)
	issues = append(issues, undeclaredIssues...)

	return issues, nil
}

// declaringManifestForResource returns the manifest filename/path where a
// resource is declared in the project manifest view. Local-only declarations
// are attributed to ai.package.local.yaml.
func declaringManifestForResource(view *manifest.ProjectManifests, resourceRef string) (string, string) {
	if view == nil {
		return manifest.ManifestFileName, ""
	}

	inBase := view.Base != nil && view.Base.Has(resourceRef)
	inLocal := view.Local != nil && view.Local.Has(resourceRef)

	if inLocal && !inBase {
		return manifest.LocalManifestFileName, view.LocalPath
	}
	if inBase {
		return manifest.ManifestFileName, view.BasePath
	}
	if view.Base == nil && inLocal {
		return manifest.LocalManifestFileName, view.LocalPath
	}
	if view.Base == nil && view.Local != nil {
		return manifest.LocalManifestFileName, view.LocalPath
	}

	return manifest.ManifestFileName, view.BasePath
}

// checkPackageInstalled verifies all resources in a package are installed.
// Returns issues for missing package definitions or uninstalled member resources.
func checkPackageInstalled(resourceRef, resName, projectPath, manifestName, manifestPath string, detectedTools []tools.Tool, repoPath string) []VerifyIssue {
	// Load package definition
	packagePath := resource.GetPackagePath(resName, repoPath)
	pkg, err := resource.LoadPackage(packagePath)
	if err != nil {
		// Package definition doesn't exist or is invalid
		return []VerifyIssue{{
			Resource:    resourceRef,
			Tool:        "any",
			IssueType:   issueTypeNotInstalled,
			Description: fmt.Sprintf("Package definition not found in repository: %v", err),
			Path:        manifestPath,
			Severity:    "warning",
		}}
	}

	// Check if all resources in the package are installed
	var missingResources []string
	for _, pkgRes := range pkg.Resources {
		resParts := strings.SplitN(pkgRes, "/", 2)
		if len(resParts) != 2 {
			continue
		}

		if !isResourceInstalledInTools(resParts[0], resParts[1], projectPath, detectedTools) {
			missingResources = append(missingResources, pkgRes)
		}
	}

	if len(missingResources) > 0 {
		desc := fmt.Sprintf("Listed in %s but %d resource(s) not installed: %s",
			manifestName, len(missingResources), strings.Join(missingResources, ", "))
		return []VerifyIssue{{
			Resource:    resourceRef,
			Tool:        "any",
			IssueType:   issueTypeNotInstalled,
			Description: desc,
			Path:        manifestPath,
			Severity:    "warning",
		}}
	}

	return nil
}

// isResourceInstalledInTools checks if a resource of the given type and name
// is installed in at least one of the detected tools.
func isResourceInstalledInTools(resType, resName, projectPath string, detectedTools []tools.Tool) bool {
	for _, tool := range detectedTools {
		toolInfo := tools.GetToolInfo(tool)
		var checkPaths []string

		switch resType {
		case "skill":
			if !toolInfo.SupportsSkills {
				continue
			}
			checkPaths = []string{filepath.Join(projectPath, toolInfo.SkillsDir, resName)}
		case "command":
			if !toolInfo.SupportsCommands {
				continue
			}
			basePath := filepath.Join(projectPath, toolInfo.CommandsDir, resName)
			checkPaths = []string{basePath, basePath + ".md"}
		case "agent":
			if !toolInfo.SupportsAgents {
				continue
			}
			checkPaths = []string{filepath.Join(projectPath, toolInfo.AgentsDir, tools.AgentArtifactName(tool, resName))}
		default:
			continue
		}

		for _, path := range checkPaths {
			if _, err := os.Stat(path); err == nil {
				return true
			}
		}
	}
	return false
}

// findUndeclaredInOwnedDirs builds the expanded manifest set and scans owned
// resource directories for undeclared content not covered by ai.package.yaml.
func findUndeclaredInOwnedDirs(mf *manifest.Manifest, projectPath string, repoPath string) []VerifyIssue {
	// Build the expanded manifest set (includes package member resources)
	expandedManifest := make(map[string]bool, len(mf.Resources))
	for _, ref := range mf.Resources {
		// Expand packages — resolve each package to its member resources
		if strings.HasPrefix(ref, "package/") {
			packageName := strings.TrimPrefix(ref, "package/")
			pkgPath := resource.GetPackagePath(packageName, repoPath)
			pkg, err := resource.LoadPackage(pkgPath)
			if err != nil {
				continue
			}
			for _, memberRef := range pkg.Resources {
				expandedManifest[memberRef] = true
			}
			continue
		}

		expandedManifest[ref] = true
	}

	ownedDirs, err := detectOwnedResourceDirs(projectPath)
	if err != nil {
		return nil
	}

	declaredPaths := make(map[string]struct{})
	for ref := range expandedManifest {
		resType, resName, err := resource.ParseResourceReference(ref)
		if err != nil {
			continue
		}
		for _, p := range desiredInstallPaths(ownedDirs, resType, resName) {
			declaredPaths[p.path] = struct{}{}
		}
	}

	undeclaredPaths, err := collectUndeclaredPaths(ownedDirs, declaredPaths)
	if err != nil {
		return nil
	}

	// Convert undeclared paths to verify issues
	var issues []VerifyIssue
	for _, undeclaredPath := range undeclaredPaths {
		for _, owned := range ownedDirs {
			if undeclaredPath != owned.Path && !strings.HasPrefix(undeclaredPath, owned.Path+string(os.PathSeparator)) {
				continue
			}

			rel, err := filepath.Rel(owned.Path, undeclaredPath)
			if err != nil {
				rel = filepath.Base(undeclaredPath)
			}
			issues = append(issues, VerifyIssue{
				Resource:    filepath.ToSlash(rel),
				Tool:        owned.Tool.String(),
				IssueType:   issueTypeUndeclared,
				Description: fmt.Sprintf("Exists in owned directory but is not declared in %s", manifest.ManifestFileName),
				Path:        undeclaredPath,
				Severity:    "warning",
			})
			break
		}
	}

	return issues
}

// findUndeclaredResources scans a directory for installed symlinks that are not
// in the expanded manifest. Returns VerifyIssue entries for each undeclared entry found.
// The seen map is used to deduplicate across multiple tool directories.
func findUndeclaredResources(dir, resType string, tool tools.Tool, expandedManifest map[string]bool, seen map[string]bool) []VerifyIssue {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var issues []VerifyIssue
	for _, entry := range entries {
		entryPath := filepath.Join(dir, entry.Name())

		info, err := os.Lstat(entryPath)
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			// Top-level symlink
			name := entry.Name()
			// Commands and agents use .md-stripped names as resource references
			if resType == "command" {
				name = strings.TrimSuffix(name, ".md")
			} else if resType == "agent" {
				logicalName, ok := tools.AgentLogicalName(tool, name)
				if !ok {
					continue
				}
				name = logicalName
			}
			ref := resType + "/" + name
			if !expandedManifest[ref] && !seen[ref] {
				seen[ref] = true
				issues = append(issues, VerifyIssue{
					Resource:    name,
					IssueType:   issueTypeUndeclared,
					Description: fmt.Sprintf("Installed but not listed in %s", manifest.ManifestFileName),
					Severity:    "warning",
				})
			}
		} else if info.IsDir() {
			// Nested directory — scan one level for namespaced resources
			subEntries, err := os.ReadDir(entryPath)
			if err != nil {
				continue
			}
			for _, subEntry := range subEntries {
				if subEntry.IsDir() {
					continue
				}
				subPath := filepath.Join(entryPath, subEntry.Name())
				subInfo, err := os.Lstat(subPath)
				if err != nil {
					continue
				}
				if subInfo.Mode()&os.ModeSymlink == 0 {
					continue
				}
				namePart := strings.TrimSuffix(subEntry.Name(), ".md")
				if resType == "agent" {
					logicalName, ok := tools.AgentLogicalName(tool, subEntry.Name())
					if !ok {
						continue
					}
					namePart = logicalName
				}
				name := entry.Name() + "/" + namePart
				ref := resType + "/" + name
				if !expandedManifest[ref] && !seen[ref] {
					seen[ref] = true
					issues = append(issues, VerifyIssue{
						Resource:    name,
						IssueType:   issueTypeUndeclared,
						Description: fmt.Sprintf("Installed but not listed in %s", manifest.ManifestFileName),
						Path:        subPath,
						Severity:    "warning",
					})
				}
			}
		}
	}

	return issues
}

func displayNoIssues(format output.Format) error {
	switch format {
	case output.JSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string][]VerifyIssue{"issues": {}})

	case output.YAML:
		encoder := yaml.NewEncoder(os.Stdout)
		defer func() { _ = encoder.Close() }()
		return encoder.Encode(map[string][]VerifyIssue{"issues": {}})

	case output.Table:
		fmt.Println("✓ All installed resources are valid")
		return nil

	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func displayVerifyIssues(issues []VerifyIssue, format output.Format) error {
	switch format {
	case output.JSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(issues)

	case output.YAML:
		encoder := yaml.NewEncoder(os.Stdout)
		defer func() { _ = encoder.Close() }()
		return encoder.Encode(issues)

	case output.Table:
		table := output.NewTable("Name", "Tool", "Issue", "Details")
		table.WithResponsive().
			WithDynamicColumn(3).
			WithMinColumnWidths(40, 12, 15, 40)

		for _, issue := range issues {
			symbol := "⚠"
			if issue.Severity == "error" {
				symbol = statusIconFail
			}
			table.AddRow(issue.Resource, issue.Tool, symbol+" "+issue.IssueType, issue.Description)
		}

		return table.Format(output.Table)

	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func runVerifyFixWrapper(projectPath string, manager *repo.Manager, parsedFormat output.Format, ownedDirs []OwnedResourceDir) error {
	repairFormat := parsedFormat
	if repairFormat == output.YAML {
		repairFormat = output.Table
	}

	mf, _, err := loadEffectiveProjectManifest(projectPath)
	if err != nil {
		return err
	}
	if mf == nil {
		return fmt.Errorf("failed to load project manifest: neither %s nor %s found", manifest.ManifestFileName, manifest.LocalManifestFileName)
	}

	expanded, expandErrs := expandManifestRefs(mf, manager.GetRepoPath())

	result := RepairResult{
		DryRun: false,
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

	plan, err := buildReconcilePlan(manager.GetRepoPath(), ownedDirs, expanded)
	if err != nil {
		return err
	}
	result.Planned.Installs = append(result.Planned.Installs, plan.Installs...)
	result.Planned.Fixes = append(result.Planned.Fixes, plan.Fixes...)
	result.Planned.Removals = append(result.Planned.Removals, plan.Removals...)

	if len(result.Failed) == 0 {
		if err := applyReconcilePlan(projectPath, manager, ownedDirs, plan, &result); err != nil {
			result.Failed = append(result.Failed, RepairErr{IssueType: "repair", Message: err.Error()})
		}
	}

	result.Summary = repairStats(result)
	if noPlannedWork(result) && len(result.Failed) == 0 {
		return repairDisplayNoIssues(repairFormat)
	}

	return repairDisplayResult(result, repairFormat)
}

var (
	verifyProjectPath string
	verifyFixFlag     bool
	verifyFormatFlag2 string
)

func init() {
	rootCmd.AddCommand(projectVerifyCmd)
	projectVerifyCmd.Flags().StringVar(&verifyProjectPath, "project-path", "", "Project directory path (default: current directory)")
	projectVerifyCmd.Flags().BoolVar(&verifyFixFlag, "fix", false, "Run repair reconciliation (deprecated, use 'aimgr repair')")
	_ = projectVerifyCmd.Flags().MarkDeprecated("fix", "use 'aimgr repair' (reconcile project resources)")
	projectVerifyCmd.Flags().StringVar(&verifyFormatFlag2, "format", "table", "Output format (table|json|yaml)")

	// Register completion functions
	_ = projectVerifyCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)
}
