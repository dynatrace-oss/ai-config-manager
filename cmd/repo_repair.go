package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/spf13/cobra"
)

// RepoRepairResult contains the results of a repository repair operation
type RepoRepairResult struct {
	Status string          `json:"status" yaml:"status"`
	Error  *CommandFailure `json:"error,omitempty" yaml:"error,omitempty"`
	// Fixed issues
	MetadataCreated []ResourceIssue `json:"metadata_created,omitempty" yaml:"metadata_created,omitempty"`
	OrphanedRemoved []MetadataIssue `json:"orphaned_removed,omitempty" yaml:"orphaned_removed,omitempty"`
	// Unfixable issues (reported but not auto-fixed)
	TypeMismatches          []TypeMismatch `json:"type_mismatches,omitempty" yaml:"type_mismatches,omitempty"`
	PackagesWithMissingRefs []PackageIssue `json:"packages_with_missing_refs,omitempty" yaml:"packages_with_missing_refs,omitempty"`
	// Dry-run flag
	DryRun bool `json:"dry_run" yaml:"dry_run"`
	// Summary
	FixedCount     int `json:"fixed_count" yaml:"fixed_count"`
	UnfixableCount int `json:"unfixable_count" yaml:"unfixable_count"`
}

const (
	repoRepairStatusClean                 = "clean"
	repoRepairStatusCompletedWithFindings = "completed_with_findings"
	repoRepairStatusExecutionFailed       = "execution_failed"
)

var (
	repoRepairFormatFlag string
	repoRepairDryRun     bool
)

// repoRepairCmd represents the repo repair command
var repoRepairCmd = &cobra.Command{
	Use:          "repair",
	Short:        "Repair repository metadata issues",
	SilenceUsage: true,
	Long: `Diagnose and fix repository metadata issues.

This command performs the same checks as 'repo verify' and automatically fixes
what it can. Issues that cannot be auto-fixed are reported with guidance.

Fixable issues (auto-repaired):
  - Resources without metadata → creates missing metadata
  - Orphaned metadata (resource gone) → removes orphaned metadata file

Unfixable issues (reported with guidance):
  - Type mismatches between resource and metadata
  - Packages with missing resource references

Use --dry-run to see what would be repaired without making any changes.

Output Formats:
  --format=text (default): Human-readable summary
  --format=json:           Structured JSON output

Exit status:
  0 - Repair completed cleanly (or only fixable issues were resolved)
  1 - Repair completed but unfixable issues remain
  2 - Repair failed operationally (e.g. lock contention/timeout, I/O, parse, internal)

Status model (JSON):
  - status=clean: repair completed with no remaining issues
  - status=completed_with_findings: repair completed but unfixable findings remain
  - status=execution_failed: repair could not complete (error.category explains why)

Examples:
  aimgr repo repair              # Fix all auto-fixable issues
  aimgr repo repair --dry-run    # Preview what would be fixed
  aimgr repo repair --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate format — for repo repair we support text and json only
		var parsedFormat output.Format
		switch repoRepairFormatFlag {
		case "text", "table", "":
			parsedFormat = output.Table
		case "json":
			parsedFormat = output.JSON
		default:
			return newOperationalFailureError(fmt.Errorf("invalid format: %s (valid: text, json)", repoRepairFormatFlag))
		}

		// Create a new repo manager
		manager, err := NewManagerWithLogLevel()
		if err != nil {
			if outErr := outputRepoRepairOperationalFailure(parsedFormat, err); outErr != nil {
				return outErr
			}
			if parsedFormat == output.JSON {
				return newSuppressedOperationalFailureError(err)
			}
			return newOperationalFailureError(err)
		}

		repoExists, err := repoPathExists(manager.GetRepoPath())
		if err != nil {
			if outErr := outputRepoRepairOperationalFailure(parsedFormat, err); outErr != nil {
				return outErr
			}
			if parsedFormat == output.JSON {
				return newSuppressedOperationalFailureError(err)
			}
			return newOperationalFailureError(err)
		}
		if !repoExists {
			result := &RepoRepairResult{DryRun: repoRepairDryRun, Status: repoRepairStatusClean}
			return outputRepoRepairResults(result, parsedFormat)
		}

		repoLock, err := manager.AcquireRepoWriteLock(cmd.Context())
		if err != nil {
			if outErr := outputRepoRepairOperationalFailure(parsedFormat, err); outErr != nil {
				return outErr
			}
			if parsedFormat == output.JSON {
				return newSuppressedOperationalFailureErrorWithCategory(
					fmt.Errorf("failed to acquire repository lock at %s: %w", manager.RepoLockPath(), err),
					commandErrorCategoryRepoBusy,
				)
			}
			return wrapLockAcquireError(manager.RepoLockPath(), err)
		}
		defer func() {
			_ = repoLock.Unlock()
		}()

		if err := maybeHoldAfterRepoLock(cmd.Context(), "repair"); err != nil {
			if outErr := outputRepoRepairOperationalFailure(parsedFormat, err); outErr != nil {
				return outErr
			}
			if parsedFormat == output.JSON {
				return newSuppressedOperationalFailureError(err)
			}
			return newOperationalFailureError(err)
		}

		// Run diagnostic scan without applying fixes
		verifyResult, err := verifyRepository(manager, false, nil)
		if err != nil {
			if outErr := outputRepoRepairOperationalFailure(parsedFormat, err); outErr != nil {
				return outErr
			}
			if parsedFormat == output.JSON {
				return newSuppressedOperationalFailureError(fmt.Errorf("diagnostic scan failed: %w", err))
			}
			return newOperationalFailureError(fmt.Errorf("diagnostic scan failed: %w", err))
		}

		result := &RepoRepairResult{
			DryRun:                  repoRepairDryRun,
			TypeMismatches:          verifyResult.TypeMismatches,
			PackagesWithMissingRefs: verifyResult.PackagesWithMissingRefs,
		}
		result.UnfixableCount = len(result.TypeMismatches) + len(result.PackagesWithMissingRefs)

		if repoRepairDryRun {
			// In dry-run mode, populate "would fix" lists without touching the filesystem
			result.MetadataCreated = verifyResult.ResourcesWithoutMetadata
			result.OrphanedRemoved = verifyResult.OrphanedMetadata
			result.FixedCount = len(result.MetadataCreated) + len(result.OrphanedRemoved)
			setRepoRepairStatusForCompletedResult(result)
			if err := outputRepoRepairResults(result, parsedFormat); err != nil {
				return err
			}
			if result.Status == repoRepairStatusCompletedWithFindings {
				return completedWithFindingsErrorForFormat(parsedFormat, "repository repair completed with unfixable findings")
			}
			return nil
		}

		// Apply fixes: create missing metadata
		for _, issue := range verifyResult.ResourcesWithoutMetadata {
			res := resource.Resource{Name: issue.Name, Type: issue.Type}
			if err := createMetadataForResource(manager, res); err != nil {
				if outErr := outputRepoRepairOperationalFailure(parsedFormat, err); outErr != nil {
					return outErr
				}
				if parsedFormat == output.JSON {
					return newSuppressedOperationalFailureError(fmt.Errorf("failed to create metadata for %s/%s: %w", issue.Type, issue.Name, err))
				}
				return newOperationalFailureError(fmt.Errorf("failed to create metadata for %s/%s: %w", issue.Type, issue.Name, err))
			}
			result.MetadataCreated = append(result.MetadataCreated, issue)
		}

		// Apply fixes: remove orphaned metadata
		for _, issue := range verifyResult.OrphanedMetadata {
			if err := os.Remove(issue.Path); err != nil {
				if outErr := outputRepoRepairOperationalFailure(parsedFormat, err); outErr != nil {
					return outErr
				}
				if parsedFormat == output.JSON {
					return newSuppressedOperationalFailureError(fmt.Errorf("failed to remove orphaned metadata %s: %w", issue.Path, err))
				}
				return newOperationalFailureError(fmt.Errorf("failed to remove orphaned metadata %s: %w", issue.Path, err))
			}
			result.OrphanedRemoved = append(result.OrphanedRemoved, issue)
		}

		result.FixedCount = len(result.MetadataCreated) + len(result.OrphanedRemoved)
		setRepoRepairStatusForCompletedResult(result)
		if err := outputRepoRepairResults(result, parsedFormat); err != nil {
			return err
		}
		if result.Status == repoRepairStatusCompletedWithFindings {
			return completedWithFindingsErrorForFormat(parsedFormat, "repository repair completed with unfixable findings")
		}

		return nil
	},
}

func setRepoRepairStatusForCompletedResult(result *RepoRepairResult) {
	if result == nil {
		return
	}

	result.Error = nil
	if result.UnfixableCount > 0 {
		result.Status = repoRepairStatusCompletedWithFindings
		return
	}

	result.Status = repoRepairStatusClean
}

func outputRepoRepairOperationalFailure(format output.Format, err error) error {
	if format != output.JSON {
		return nil
	}

	result := &RepoRepairResult{
		Status: repoRepairStatusExecutionFailed,
		Error: &CommandFailure{
			Category: string(classifyOperationalError(err)),
			Message:  err.Error(),
		},
	}

	return outputRepoRepairResults(result, format)
}

func completedWithFindingsErrorForFormat(format output.Format, message string) error {
	if format == output.JSON {
		return &commandExitError{
			ExitCode:       commandExitCodeCompletedWithFindings,
			SuppressStderr: true,
			Cause:          errors.New(message),
		}
	}

	return newCompletedWithFindingsError(message)
}

// outputRepoRepairResults outputs the repair results in the requested format
func outputRepoRepairResults(result *RepoRepairResult, format output.Format) error {
	switch format {
	case output.JSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	default:
		displayRepoRepairResults(result)
		return nil
	}
}

// displayRepoRepairResults displays repair results in human-readable format
func displayRepoRepairResults(result *RepoRepairResult) {
	if result.DryRun {
		fmt.Println("Dry run — no changes will be made:")
		fmt.Println()
	} else {
		fmt.Println("Repository Repair")
		fmt.Println("=================")
		fmt.Println()
	}

	hasOutput := false

	// Fixed: metadata created
	if len(result.MetadataCreated) > 0 {
		hasOutput = true
		if result.DryRun {
			for _, issue := range result.MetadataCreated {
				fmt.Printf("  Would create metadata for %s\n",
					formatResourceReference(issue.Type, issue.Name))
			}
		} else {
			fmt.Printf("✓ Created metadata for %d resource(s):\n", len(result.MetadataCreated))
			for _, issue := range result.MetadataCreated {
				fmt.Printf("  • %s\n", formatResourceReference(issue.Type, issue.Name))
			}
			fmt.Println()
		}
	}

	// Fixed: orphaned metadata removed
	if len(result.OrphanedRemoved) > 0 {
		hasOutput = true
		if result.DryRun {
			for _, issue := range result.OrphanedRemoved {
				fmt.Printf("  Would remove orphaned metadata: %s\n", issue.Path)
			}
		} else {
			fmt.Printf("✓ Removed %d orphaned metadata file(s):\n", len(result.OrphanedRemoved))
			for _, issue := range result.OrphanedRemoved {
				fmt.Printf("  • %s\n", issue.Path)
			}
			fmt.Println()
		}
	}

	// Unfixable: type mismatches
	if len(result.TypeMismatches) > 0 {
		hasOutput = true
		if result.DryRun {
			for _, mismatch := range result.TypeMismatches {
				fmt.Printf("  Cannot auto-fix: type mismatch for %s (metadata says %q)\n",
					formatResourceReference(mismatch.ResourceType, mismatch.Name),
					mismatch.MetadataType)
			}
		} else {
			fmt.Printf("✗ Cannot auto-fix: %d type mismatch(es) — manual intervention required:\n",
				len(result.TypeMismatches))
			for _, mismatch := range result.TypeMismatches {
				fmt.Printf("  • %s — resource type: %s, metadata type: %s\n",
					mismatch.Name, mismatch.ResourceType, mismatch.MetadataType)
				fmt.Printf("    Fix: delete %s and re-add the resource, or correct the metadata file at %s\n",
					mismatch.ResourcePath, mismatch.MetadataPath)
			}
			fmt.Println()
		}
	}

	// Unfixable: packages with missing refs
	if len(result.PackagesWithMissingRefs) > 0 {
		hasOutput = true
		if result.DryRun {
			for _, pkg := range result.PackagesWithMissingRefs {
				fmt.Printf("  Cannot auto-fix: package %q has missing resource references: %v\n",
					pkg.Name, pkg.MissingResources)
			}
		} else {
			fmt.Printf("✗ Cannot auto-fix: %d package(s) with missing resource references:\n",
				len(result.PackagesWithMissingRefs))
			for _, pkg := range result.PackagesWithMissingRefs {
				fmt.Printf("  • %s — missing: %v\n", pkg.Name, pkg.MissingResources)
				fmt.Printf("    Fix: install missing resources or update the package definition at %s\n",
					pkg.Path)
			}
			fmt.Println()
		}
	}

	// Summary
	if result.DryRun {
		if !hasOutput {
			fmt.Println("  No issues found — repository is healthy.")
		} else {
			fmt.Println()
			fmt.Printf("Summary: %d action(s) planned, %d unfixable issue(s)\n",
				result.FixedCount, result.UnfixableCount)
		}
	} else {
		if !hasOutput {
			fmt.Println("✓ No issues found. Repository is healthy!")
		} else {
			fmt.Printf("Summary: %d fixed, %d unfixable\n",
				result.FixedCount, result.UnfixableCount)
			if result.UnfixableCount > 0 {
				fmt.Println()
				fmt.Println("Run 'aimgr repo verify' for detailed diagnostics on unfixable issues.")
			}
		}
	}
}

func init() {
	repoCmd.AddCommand(repoRepairCmd)
	repoRepairCmd.Flags().StringVar(&repoRepairFormatFlag, "format", "text", "Output format (text|json)")
	repoRepairCmd.Flags().BoolVar(&repoRepairDryRun, "dry-run", false, "Show what would be fixed without making changes")

	// Register completion functions
	_ = repoRepairCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)
}
