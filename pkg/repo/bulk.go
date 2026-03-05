package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/dynatrace-oss/ai-config-manager/v3/pkg/errors"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// BulkImportOptions contains options for bulk import operations
type BulkImportOptions struct {
	SourceName   string // Explicit source name from manifest (overrides derived name)
	SourceID     string // Source ID from manifest (hash-based)
	ImportMode   string // "copy" or "symlink"
	Force        bool   // Overwrite existing resources
	SkipExisting bool   // Skip conflicts silently
	DryRun       bool   // Preview only, don't actually import
	SourceURL    string // Original source URL (for Git sources)
	SourceType   string // Source type (github, git-url, file, local)
	Ref          string // Git ref (branch/tag/commit), defaults to "main" if empty
}

// ImportOptions contains options for single resource import operations
type ImportOptions struct {
	SourceName string // Explicit source name from manifest (overrides derived name)
	SourceID   string // Source ID from manifest (hash-based)
	ImportMode string // "copy" or "symlink"
	Force      bool   // Overwrite existing resources
}

// ImportError represents an error during resource import
type ImportError struct {
	Path    string
	Message string
}

// BulkImportResult contains the results of a bulk import operation
type BulkImportResult struct {
	Added        []string      // Successfully added resources (new)
	Updated      []string      // Successfully updated resources (already existed, re-added with Force)
	Skipped      []string      // Skipped due to conflicts
	Failed       []ImportError // Failed imports with reasons
	CommandCount int           // Number of commands imported
	SkillCount   int           // Number of skills imported
	AgentCount   int           // Number of agents imported
	PackageCount int           // Number of packages imported
}

func (m *Manager) AddBulk(sources []string, opts BulkImportOptions) (*BulkImportResult, error) {
	result := &BulkImportResult{
		Added:        []string{},
		Updated:      []string{},
		Skipped:      []string{},
		Failed:       []ImportError{},
		CommandCount: 0,
		SkillCount:   0,
		AgentCount:   0,
		PackageCount: 0,
	}

	// Ensure repo is initialized (even for dry run, to check paths)
	if !opts.DryRun {
		if err := m.Init(); err != nil {
			return nil, err
		}
	}

	// Process each source
	for _, sourcePath := range sources {
		if err := m.importResource(sourcePath, opts, result); err != nil {
			// Only stop on fatal errors (internal bugs, system failures)
			// Continue on validation/resource errors (they're already collected in result)
			if pkgerrors.IsFatal(err) {
				return result, err
			}
		}
	}

	// Commit changes if not a dry run and if any resources were added/updated
	if !opts.DryRun && (len(result.Added) > 0 || len(result.Updated) > 0) {
		// Build commit message
		totalChanges := len(result.Added) + len(result.Updated)
		commitMsg := fmt.Sprintf("aimgr: import %d resource(s)", totalChanges)

		// Add detail about resource types
		details := []string{}
		if result.CommandCount > 0 {
			details = append(details, fmt.Sprintf("%d command(s)", result.CommandCount))
		}
		if result.SkillCount > 0 {
			details = append(details, fmt.Sprintf("%d skill(s)", result.SkillCount))
		}
		if result.AgentCount > 0 {
			details = append(details, fmt.Sprintf("%d agent(s)", result.AgentCount))
		}
		if result.PackageCount > 0 {
			details = append(details, fmt.Sprintf("%d package(s)", result.PackageCount))
		}
		if len(details) > 0 {
			commitMsg += " (" + strings.Join(details, ", ") + ")"
		}

		if err := m.CommitChanges(commitMsg); err != nil {
			// Log warning but don't fail the operation
			// Git tracking is optional
			fmt.Fprintf(os.Stderr, "Warning: failed to commit changes: %v\n", err)
		}
	}

	return result, nil
}

// importResource imports a single resource (helper for AddBulk)
func (m *Manager) importResource(sourcePath string, opts BulkImportOptions, result *BulkImportResult) error {
	// Check if it's a package file
	if strings.HasSuffix(sourcePath, ".package.json") {
		return m.importPackage(sourcePath, opts, result)
	}

	// Detect resource type
	resourceType, err := resource.DetectType(sourcePath)
	if err != nil {
		// Validation error: invalid resource format
		typedErr := pkgerrors.Validation(err, "failed to detect resource type")
		result.Failed = append(result.Failed, ImportError{
			Path:    sourcePath,
			Message: typedErr.Error(),
		})
		return typedErr
	}

	// Load the resource to get its name
	var res *resource.Resource
	switch resourceType {
	case resource.Command:
		res, err = resource.LoadCommand(sourcePath)
	case resource.Skill:
		res, err = resource.LoadSkill(sourcePath)
	case resource.Agent:
		res, err = resource.LoadAgent(sourcePath)
	default:
		err = fmt.Errorf("unknown resource type: %s", resourceType)
	}

	if err != nil {
		// Validation error: invalid YAML/frontmatter
		typedErr := pkgerrors.Validation(err, "failed to load resource")
		result.Failed = append(result.Failed, ImportError{
			Path:    sourcePath,
			Message: typedErr.Error(),
		})
		return typedErr
	}

	// Check if resource already exists
	destPath := m.GetPath(res.Name, resourceType)
	_, statErr := os.Stat(destPath)
	exists := statErr == nil

	if exists {
		if opts.Force {
			// Force mode: remove existing and continue
			if !opts.DryRun {
				if err := m.Remove(res.Name, resourceType); err != nil {
					// Could be resource error (permission denied) or fatal (system failure)
					// Default to resource error for safety
					typedErr := pkgerrors.Resource(err, "failed to remove existing resource")
					result.Failed = append(result.Failed, ImportError{
						Path:    sourcePath,
						Message: typedErr.Error(),
					})
					return typedErr
				}
			}
		} else if opts.SkipExisting {
			// Skip mode: skip this resource
			result.Skipped = append(result.Skipped, sourcePath)
			return nil
		} else {
			// Default mode: fail on conflict
			// Validation error: resource name conflict
			typedErr := pkgerrors.Validation(fmt.Errorf("resource '%s' already exists in repository", res.Name), "")
			result.Failed = append(result.Failed, ImportError{
				Path:    sourcePath,
				Message: typedErr.Error(),
			})
			return typedErr
		}
	}

	// Import the resource (unless dry run)
	if !opts.DryRun {
		// Determine source URL and type
		var sourceURL, sourceType, ref string

		// Use provided source info if available, otherwise fall back to file://
		if opts.SourceURL != "" && opts.SourceType != "" {
			sourceURL = opts.SourceURL
			sourceType = opts.SourceType
			ref = opts.Ref
		} else {
			// Fall back to file:// for local sources
			absPath, err := filepath.Abs(sourcePath)
			if err != nil {
				absPath = sourcePath
			}
			sourceURL = "file://" + absPath
			sourceType = "file"
			ref = ""
		}

		// Create import options from bulk options
		importOpts := ImportOptions{
			SourceName: opts.SourceName,
			SourceID:   opts.SourceID,
			ImportMode: opts.ImportMode,
			Force:      opts.Force,
		}
		// Default to "copy" if not specified
		if importOpts.ImportMode == "" {
			importOpts.ImportMode = "copy"
		}

		switch resourceType {
		case resource.Command:
			err = m.addCommandWithOptions(sourcePath, sourceURL, sourceType, ref, importOpts)
		case resource.Skill:
			err = m.addSkillWithOptions(sourcePath, sourceURL, sourceType, ref, importOpts)
		case resource.Agent:
			err = m.addAgentWithOptions(sourcePath, sourceURL, sourceType, ref, importOpts)
		}

		if err != nil {
			// Could be various errors - wrap as validation for now
			typedErr := pkgerrors.Validation(err, "failed to import resource")
			result.Failed = append(result.Failed, ImportError{
				Path:    sourcePath,
				Message: typedErr.Error(),
			})
			return typedErr
		}
	}

	// Increment resource type counter (count in both dry-run and real mode)
	switch resourceType {
	case resource.Command:
		result.CommandCount++
	case resource.Skill:
		result.SkillCount++
	case resource.Agent:
		result.AgentCount++
	}

	// Track whether this was an update (existed before) or a new addition
	if exists && opts.Force {
		result.Updated = append(result.Updated, sourcePath)
	} else {
		result.Added = append(result.Added, sourcePath)
	}
	return nil
}

// importPackage imports a single package (helper for AddBulk)
func (m *Manager) importPackage(sourcePath string, opts BulkImportOptions, result *BulkImportResult) error {
	// Load the package to get its name
	pkg, err := resource.LoadPackage(sourcePath)
	if err != nil {
		// Validation error: invalid package format
		typedErr := pkgerrors.Validation(err, "failed to load package")
		result.Failed = append(result.Failed, ImportError{
			Path:    sourcePath,
			Message: typedErr.Error(),
		})
		return typedErr
	}

	// Check if package already exists
	destPath := resource.GetPackagePath(pkg.Name, m.repoPath)
	_, statErr := os.Stat(destPath)
	exists := statErr == nil

	if exists {
		if opts.Force {
			// Force mode: remove existing and continue
			if !opts.DryRun {
				// Remove package file
				if err := os.Remove(destPath); err != nil {
					// Resource error: failed to delete file
					typedErr := pkgerrors.Resource(err, "failed to remove existing package")
					result.Failed = append(result.Failed, ImportError{
						Path:    sourcePath,
						Message: typedErr.Error(),
					})
					return typedErr
				}
				// Remove metadata file
				metadataPath := metadata.GetPackageMetadataPath(pkg.Name, m.repoPath)
				if _, err := os.Stat(metadataPath); err == nil {
					if err := os.Remove(metadataPath); err != nil {
						// Resource error: failed to delete metadata
						typedErr := pkgerrors.Resource(err, "failed to remove metadata")
						result.Failed = append(result.Failed, ImportError{
							Path:    sourcePath,
							Message: typedErr.Error(),
						})
						return typedErr
					}
				}
			}
		} else if opts.SkipExisting {
			// Skip mode: skip this package
			result.Skipped = append(result.Skipped, sourcePath)
			return nil
		} else {
			// Default mode: fail on conflict
			// Validation error: package name conflict
			typedErr := pkgerrors.Validation(fmt.Errorf("package '%s' already exists in repository", pkg.Name), "")
			result.Failed = append(result.Failed, ImportError{
				Path:    sourcePath,
				Message: typedErr.Error(),
			})
			return typedErr
		}
	}

	// Import the package (unless dry run)
	if !opts.DryRun {
		// Determine source URL and type
		var sourceURL, sourceType, ref string

		// Use provided source info if available, otherwise fall back to file://
		if opts.SourceURL != "" && opts.SourceType != "" {
			sourceURL = opts.SourceURL
			sourceType = opts.SourceType
			ref = opts.Ref
		} else {
			// Fall back to file:// for local sources
			absPath, err := filepath.Abs(sourcePath)
			if err != nil {
				absPath = sourcePath
			}
			sourceURL = "file://" + absPath
			sourceType = "file"
			ref = ""
		}

		// Create import options from bulk options
		importOpts := ImportOptions{
			SourceName: opts.SourceName,
			SourceID:   opts.SourceID,
			ImportMode: opts.ImportMode,
			Force:      opts.Force,
		}
		// Default to "copy" if not specified
		if importOpts.ImportMode == "" {
			importOpts.ImportMode = "copy"
		}

		err = m.addPackageWithOptions(sourcePath, sourceURL, sourceType, ref, importOpts)
		if err != nil {
			// Could be various errors - wrap as validation for now
			typedErr := pkgerrors.Validation(err, "failed to import package")
			result.Failed = append(result.Failed, ImportError{
				Path:    sourcePath,
				Message: typedErr.Error(),
			})
			return typedErr
		}
	}

	// Increment package counter (count in both dry-run and real mode)
	result.PackageCount++

	// Track whether this was an update (existed before) or a new addition
	if exists && opts.Force {
		result.Updated = append(result.Updated, sourcePath)
	} else {
		result.Added = append(result.Added, sourcePath)
	}
	return nil
}

// ValidatePackageResources checks if all referenced resources exist in the repository.
// Returns a slice of missing resource references. (Public version for verify command)
