package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/output"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// VerifyResult contains the results of repository verification
type VerifyResult struct {
	ResourcesWithoutMetadata []ResourceIssue `json:"resources_without_metadata,omitempty" yaml:"resources_without_metadata,omitempty"`
	OrphanedMetadata         []MetadataIssue `json:"orphaned_metadata,omitempty" yaml:"orphaned_metadata,omitempty"`
	MissingSourcePaths       []MetadataIssue `json:"missing_source_paths,omitempty" yaml:"missing_source_paths,omitempty"`
	TypeMismatches           []TypeMismatch  `json:"type_mismatches,omitempty" yaml:"type_mismatches,omitempty"`
	PackagesWithMissingRefs  []PackageIssue  `json:"packages_with_missing_refs,omitempty" yaml:"packages_with_missing_refs,omitempty"`
	HasErrors                bool            `json:"has_errors" yaml:"has_errors"`
	HasWarnings              bool            `json:"has_warnings" yaml:"has_warnings"`
}

// ResourceIssue represents a resource with an issue
type ResourceIssue struct {
	Name string                `json:"name" yaml:"name"`
	Type resource.ResourceType `json:"type" yaml:"type"`
	Path string                `json:"path" yaml:"path"`
}

// MetadataIssue represents a metadata file with an issue
type MetadataIssue struct {
	Name       string                `json:"name" yaml:"name"`
	Type       resource.ResourceType `json:"type" yaml:"type"`
	Path       string                `json:"path" yaml:"path"`
	SourcePath string                `json:"source_path,omitempty" yaml:"source_path,omitempty"`
}

// TypeMismatch represents a mismatch between resource and metadata types
type TypeMismatch struct {
	Name         string                `json:"name" yaml:"name"`
	ResourceType resource.ResourceType `json:"resource_type" yaml:"resource_type"`
	MetadataType resource.ResourceType `json:"metadata_type" yaml:"metadata_type"`
	ResourcePath string                `json:"resource_path" yaml:"resource_path"`
	MetadataPath string                `json:"metadata_path" yaml:"metadata_path"`
}

// PackageIssue represents a package with missing resource references
type PackageIssue struct {
	Name             string   `json:"name" yaml:"name"`
	Path             string   `json:"path" yaml:"path"`
	MissingResources []string `json:"missing_resources" yaml:"missing_resources"`
}

var (
	verifyFix        bool
	verifyJSON       bool // Deprecated: use --format=json
	verifyFormatFlag string
)

// repoVerifyCmd represents the repo verify command
var repoVerifyCmd = &cobra.Command{
	Use:               "verify [pattern]",
	Short:             "Check repository metadata and package integrity",
	ValidArgsFunction: completeVerifyPatterns,
	Long: `Check for consistency issues between resources and metadata, and validate package references.

This command performs the following checks:
  - Resources without metadata (warning)
  - Orphaned metadata files with missing resources (error)
  - Metadata with non-existent source paths (warning)
  - Type mismatches between resource and metadata (error)
  - Packages with missing resource references (error)

Use 'aimgr repo repair' to automatically resolve issues (--fix is deprecated):
  - Create missing metadata for resources
  - Remove orphaned metadata files

Patterns support wildcards (* for multiple characters, ? for single character) 
and optional type prefixes.

Output Formats:
  --format=table (default): Human-readable with colored status
  --format=json:  Structured JSON for parsing
  --format=yaml:  Structured YAML for configuration

Exit status:
  0 - No errors found
  1 - Errors found (orphaned metadata, type mismatches, or broken package references)

Examples:
  aimgr repo verify                  # Check all resources
  aimgr repo verify skill/*          # Check only skills
  aimgr repo verify command/test*    # Check commands starting with "test"
  aimgr repo repair                  # Fix issues automatically
  aimgr repo verify --format=json    # Machine-readable output
  aimgr repo verify --format=yaml    # YAML output`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create a new repo manager
		manager, err := NewManagerWithLogLevel()
		if err != nil {
			return err
		}

		// Determine output format (handle both --json and --format for backward compatibility)
		outputFormat := verifyFormatFlag
		if verifyJSON {
			outputFormat = "json"
		}

		// Validate format
		parsedFormat, err := output.ParseFormat(outputFormat)
		if err != nil {
			return err
		}

		repoPath := manager.GetRepoPath()

		// Check if repository exists
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			result := VerifyResult{HasErrors: false, HasWarnings: false}
			return outputVerifyResults(&result, parsedFormat, verifyFix)
		}

		// Print deprecation warning if --fix is used
		if verifyFix {
			// TODO: Remove --fix flag in a future version. Replaced by 'aimgr repo repair'.
			fmt.Fprintln(os.Stderr, "Warning: --fix is deprecated. Use 'aimgr repo repair' instead.")
		}

		// Parse pattern if provided
		var matcher *pattern.Matcher
		if len(args) > 0 {
			matcher, err = pattern.NewMatcher(args[0])
			if err != nil {
				return fmt.Errorf("invalid pattern '%s': %w", args[0], err)
			}
		}

		// Run verification
		result, err := verifyRepository(manager, verifyFix, matcher)
		if err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}

		// Output results in requested format
		if err := outputVerifyResults(result, parsedFormat, verifyFix); err != nil {
			return err
		}

		// Exit with non-zero status if errors found
		if result.HasErrors {
			os.Exit(1)
		}

		return nil
	},
}

// verifyRepository performs repository verification checks
func verifyRepository(manager *repo.Manager, fix bool, matcher *pattern.Matcher) (*VerifyResult, error) {
	result := &VerifyResult{}
	repoPath := manager.GetRepoPath()

	// Get all resources in the repository
	allResources, err := manager.List(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	// Get all packages
	packageInfos, err := manager.ListPackages()
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	// Filter resources by pattern if provided
	if matcher != nil {
		var filtered []resource.Resource
		for _, res := range allResources {
			if matcher.Match(&res) {
				filtered = append(filtered, res)
			}
		}
		allResources = filtered

		// Filter packages by pattern
		if matcher.GetResourceType() == "" || matcher.GetResourceType() == resource.PackageType {
			var filteredPackages []repo.PackageInfo
			for _, pkg := range packageInfos {
				tempRes := resource.Resource{
					Type: resource.PackageType,
					Name: pkg.Name,
				}
				if matcher.Match(&tempRes) {
					filteredPackages = append(filteredPackages, pkg)
				}
			}
			packageInfos = filteredPackages
		} else if matcher.GetResourceType() != resource.PackageType {
			// If filtering for specific non-package type, clear packages
			packageInfos = nil
		}
	}

	// Build resource map for quick lookup (including packages)
	resourceMap := make(map[string]resource.Resource)
	for _, res := range allResources {
		key := fmt.Sprintf("%s:%s", res.Type, res.Name)
		resourceMap[key] = res
	}
	// Add packages to resource map
	for _, pkg := range packageInfos {
		key := fmt.Sprintf("%s:%s", resource.PackageType, pkg.Name)
		resourceMap[key] = resource.Resource{
			Type: resource.PackageType,
			Name: pkg.Name,
		}
	}

	// Check 1: Resources without metadata
	for _, res := range allResources {
		// Skip packages - they use PackageMetadata and are validated separately
		if res.Type == resource.PackageType {
			continue
		}

		meta, err := manager.GetMetadata(res.Name, res.Type)
		if err != nil {
			// Metadata doesn't exist
			issue := ResourceIssue{
				Name: res.Name,
				Type: res.Type,
				Path: manager.GetPath(res.Name, res.Type),
			}
			result.ResourcesWithoutMetadata = append(result.ResourcesWithoutMetadata, issue)
			result.HasWarnings = true

			// Fix: Create metadata
			if fix {
				if err := createMetadataForResource(manager, res); err != nil {
					return nil, fmt.Errorf("failed to create metadata for %s: %w", res.Name, err)
				}
			}
			continue
		}

		// Check 2: Type mismatches
		if meta.Type != res.Type {
			mismatch := TypeMismatch{
				Name:         res.Name,
				ResourceType: res.Type,
				MetadataType: meta.Type,
				ResourcePath: manager.GetPath(res.Name, res.Type),
				MetadataPath: metadata.GetMetadataPath(res.Name, meta.Type, repoPath),
			}
			result.TypeMismatches = append(result.TypeMismatches, mismatch)
			result.HasErrors = true
		}

		// Check 3: Metadata with missing source paths
		if meta.SourceURL != "" && strings.HasPrefix(meta.SourceURL, "file://") {
			sourcePath := strings.TrimPrefix(meta.SourceURL, "file://")
			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				issue := MetadataIssue{
					Name:       meta.Name,
					Type:       meta.Type,
					Path:       metadata.GetMetadataPath(meta.Name, meta.Type, repoPath),
					SourcePath: sourcePath,
				}
				result.MissingSourcePaths = append(result.MissingSourcePaths, issue)
				result.HasWarnings = true
			}
		}
	}

	// Check 4: Orphaned metadata (metadata without resources)
	metadataDir := filepath.Join(repoPath, ".metadata")
	if _, err := os.Stat(metadataDir); err == nil {
		orphanedMeta, err := findVerifyOrphanedMetadata(resourceMap, metadataDir, fix, matcher)
		if err != nil {
			return nil, fmt.Errorf("failed to check for orphaned metadata: %w", err)
		}
		result.OrphanedMetadata = orphanedMeta
		if len(orphanedMeta) > 0 {
			result.HasErrors = true
		}
	}

	// Check 5: Packages with missing resource references
	// packageInfos is already filtered by pattern earlier
	for _, pkgInfo := range packageInfos {
		// Load the full package
		pkg, err := manager.GetPackage(pkgInfo.Name)
		if err != nil {
			// Skip packages that cannot be loaded
			continue
		}

		missingRefs := manager.ValidatePackageResources(pkg)
		if len(missingRefs) > 0 {
			issue := PackageIssue{
				Name:             pkg.Name,
				Path:             resource.GetPackagePath(pkg.Name, repoPath),
				MissingResources: missingRefs,
			}
			result.PackagesWithMissingRefs = append(result.PackagesWithMissingRefs, issue)
			result.HasErrors = true
		}
	}

	return result, nil
}

// findVerifyOrphanedMetadata finds metadata files without corresponding resources
func findVerifyOrphanedMetadata(resourceMap map[string]resource.Resource, metadataDir string, fix bool, matcher *pattern.Matcher) ([]MetadataIssue, error) {
	var orphaned []MetadataIssue

	// Determine which resource types to check based on the matcher
	typesToCheck := []resource.ResourceType{resource.Command, resource.Skill, resource.Agent, resource.PackageType}
	if matcher != nil && matcher.GetResourceType() != "" {
		// If pattern specifies a type, only check that type
		typesToCheck = []resource.ResourceType{matcher.GetResourceType()}
	}

	// Check each resource type
	for _, resType := range typesToCheck {
		typeDir := filepath.Join(metadataDir, string(resType)+"s")
		if _, err := os.Stat(typeDir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(typeDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read metadata directory %s: %w", typeDir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-metadata.json") {
				continue
			}

			// Read metadata file to get actual resource name
			// This handles nested paths correctly (e.g., "dt/cluster/overview")
			// Filename uses hyphens but metadata content preserves slashes
			metaPath := filepath.Join(typeDir, entry.Name())
			data, err := os.ReadFile(metaPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read metadata file %s: %w", metaPath, err)
			}
			var meta metadata.ResourceMetadata
			if err := json.Unmarshal(data, &meta); err != nil {
				return nil, fmt.Errorf("failed to parse metadata file %s: %w", metaPath, err)
			}

			name := meta.Name

			// If matcher is provided, check if this resource matches the pattern
			if matcher != nil {
				tempRes := resource.Resource{
					Type: resType,
					Name: name,
				}
				if !matcher.Match(&tempRes) {
					// Skip resources that don't match the pattern
					continue
				}
			}

			key := fmt.Sprintf("%s:%s", resType, name)

			// Check if corresponding resource exists
			if _, exists := resourceMap[key]; !exists {
				issue := MetadataIssue{
					Name: name,
					Type: resType,
					Path: metaPath,
				}
				orphaned = append(orphaned, issue)

				// Fix: Remove orphaned metadata
				if fix {
					if err := os.Remove(metaPath); err != nil {
						return nil, fmt.Errorf("failed to remove orphaned metadata %s: %w", metaPath, err)
					}
				}
			}
		}
	}

	return orphaned, nil
}

// createMetadataForResource creates metadata for a resource that's missing it
func createMetadataForResource(manager *repo.Manager, res resource.Resource) error {
	repoPath := manager.GetRepoPath()
	resourcePath := manager.GetPath(res.Name, res.Type)

	// Get absolute path for source URL
	absPath, err := filepath.Abs(resourcePath)
	if err != nil {
		absPath = resourcePath
	}

	// Get file modification time
	fileInfo, err := os.Stat(resourcePath)
	if err != nil {
		return fmt.Errorf("failed to stat resource file: %w", err)
	}

	meta := &metadata.ResourceMetadata{
		Name:           res.Name,
		Type:           res.Type,
		SourceType:     "local",
		SourceURL:      "file://" + absPath,
		FirstInstalled: fileInfo.ModTime(),
		LastUpdated:    fileInfo.ModTime(),
	}

	// Derive source name from the file path
	sourceName := metadata.DeriveSourceName("file://" + absPath)
	return metadata.Save(meta, repoPath, sourceName)
}

// outputVerifyResults outputs verification results in the requested format
func outputVerifyResults(result *VerifyResult, format output.Format, fixed bool) error {
	switch format {
	case output.JSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)

	case output.YAML:
		encoder := yaml.NewEncoder(os.Stdout)
		defer func() { _ = encoder.Close() }()
		return encoder.Encode(result)

	case output.Table:
		displayVerifyResults(result, fixed)
		return nil

	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// displayVerifyResults displays verification results in human-readable format
func displayVerifyResults(result *VerifyResult, fixed bool) {
	fmt.Println("Repository Verification")
	fmt.Println("=======================")
	fmt.Println()

	hasIssues := false

	// Display resources without metadata (warning)
	if len(result.ResourcesWithoutMetadata) > 0 {
		hasIssues = true
		fmt.Printf("⚠ Resources without metadata: %d\n\n", len(result.ResourcesWithoutMetadata))

		table := output.NewTable("Name", "Status")
		table.WithResponsive().
			WithDynamicColumn(0).       // Name stretches
			WithMinColumnWidths(40, 20) // Name min=40, Status min=20
		for _, issue := range result.ResourcesWithoutMetadata {
			status := "Missing metadata"
			if fixed {
				status = "✓ Created metadata"
			}
			resourceRef := formatResourceReference(issue.Type, issue.Name)
			table.AddRow(resourceRef, status)
		}
		_ = table.Format(output.Table)
		fmt.Println()
	}

	// Display orphaned metadata (error)
	if len(result.OrphanedMetadata) > 0 {
		hasIssues = true
		fmt.Printf("✗ Orphaned metadata (resource missing): %d\n\n", len(result.OrphanedMetadata))

		table := output.NewTable("Name", "Status")
		table.WithResponsive().
			WithDynamicColumn(0).       // Name stretches
			WithMinColumnWidths(40, 20) // Name min=40, Status min=20
		for _, issue := range result.OrphanedMetadata {
			status := "Resource missing"
			if fixed {
				status = "✓ Removed metadata"
			}
			resourceRef := formatResourceReference(issue.Type, issue.Name)
			table.AddRow(resourceRef, status)
		}
		_ = table.Format(output.Table)
		fmt.Println()
	}

	// Display metadata with missing source paths (warning)
	if len(result.MissingSourcePaths) > 0 {
		hasIssues = true
		fmt.Printf("⚠ Metadata with missing source paths: %d\n\n", len(result.MissingSourcePaths))

		table := output.NewTable("Name", "Source Path")
		table.WithResponsive().
			WithDynamicColumn(1).       // Source Path stretches
			WithMinColumnWidths(40, 20) // Name min=40, Source Path min=20
		for _, issue := range result.MissingSourcePaths {
			resourceRef := formatResourceReference(issue.Type, issue.Name)
			table.AddRow(resourceRef, issue.SourcePath)
		}
		_ = table.Format(output.Table)
		fmt.Println()
	}

	// Display type mismatches (error)
	if len(result.TypeMismatches) > 0 {
		hasIssues = true
		fmt.Printf("✗ Type mismatches: %d\n\n", len(result.TypeMismatches))

		table := output.NewTable("Name", "Resource Type", "Metadata Type")
		table.WithResponsive().
			WithDynamicColumn(0).           // Name stretches
			WithMinColumnWidths(40, 13, 13) // Name min=40, Resource Type min=13, Metadata Type min=13
		for _, mismatch := range result.TypeMismatches {
			metaTypeStr := string(mismatch.MetadataType)
			if metaTypeStr == "" {
				metaTypeStr = "(empty/corrupted)"
			}
			table.AddRow(mismatch.Name, string(mismatch.ResourceType), metaTypeStr)
		}
		_ = table.Format(output.Table)
		fmt.Println()
	}

	// Display packages with missing resource references (error)
	if len(result.PackagesWithMissingRefs) > 0 {
		hasIssues = true
		fmt.Printf("✗ Packages with missing resource references: %d\n\n", len(result.PackagesWithMissingRefs))

		table := output.NewTable("Package", "Missing Count", "Missing Resources")
		table.WithResponsive().
			WithDynamicColumn(2).           // Missing Resources stretches
			WithMinColumnWidths(40, 13, 20) // Package min=40, Missing Count min=13, Missing Resources min=20
		for _, issue := range result.PackagesWithMissingRefs {
			missingStr := strings.Join(issue.MissingResources, ", ")
			table.AddRow(issue.Name, fmt.Sprintf("%d", len(issue.MissingResources)), missingStr)
		}
		_ = table.Format(output.Table)
		fmt.Println()
	}

	// Summary
	if !hasIssues {
		fmt.Println("✓ No issues found. Repository is healthy!")
	} else {
		if result.HasErrors {
			fmt.Println("Status: ERRORS found (exit code 1)")
		} else if result.HasWarnings {
			fmt.Println("Status: Warnings only (exit code 0)")
		}

		if !fixed && (len(result.ResourcesWithoutMetadata) > 0 || len(result.OrphanedMetadata) > 0) {
			fmt.Println()
			fmt.Println("Run 'aimgr repo repair' to automatically resolve these issues.")
		}
	}
}

// formatResourceReference returns a formatted resource reference like "command/skip" or "skill/pdf-skill"
func formatResourceReference(resourceType resource.ResourceType, name string) string {
	switch resourceType {
	case resource.Command:
		return fmt.Sprintf("command/%s", name)
	case resource.Skill:
		return fmt.Sprintf("skill/%s", name)
	case resource.Agent:
		return fmt.Sprintf("agent/%s", name)
	case resource.PackageType:
		return fmt.Sprintf("package/%s", name)
	default:
		return name
	}
}

func init() {
	repoCmd.AddCommand(repoVerifyCmd)
	repoVerifyCmd.Flags().BoolVar(&verifyFix, "fix", false, "Automatically fix issues (create missing metadata, remove orphaned)")
	_ = repoVerifyCmd.Flags().MarkDeprecated("fix", "use 'aimgr repo repair' instead")

	// Add new --format flag
	repoVerifyCmd.Flags().StringVar(&verifyFormatFlag, "format", "table", "Output format (table|json|yaml)")
	_ = repoVerifyCmd.RegisterFlagCompletionFunc("format", completeFormatFlag)

	// Keep --json for backward compatibility but mark deprecated
	repoVerifyCmd.Flags().BoolVar(&verifyJSON, "json", false, "Output results in JSON format (deprecated: use --format=json)")
	_ = repoVerifyCmd.Flags().MarkDeprecated("json", "use --format=json instead")
}
