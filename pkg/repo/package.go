package repo

import (
	"fmt"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func (m *Manager) validatePackageResources(pkg *resource.Package) []string {
	index, err := BuildPackageReferenceIndexFromRoots([]string{m.repoPath})
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to build package reference index",
				"package", pkg.Name,
				"repo_path", m.repoPath,
				"error", err.Error(),
			)
		}
		return append([]string{}, pkg.Resources...)
	}

	var missing []string

	// DEBUG: Log package validation start
	if m.logger != nil {
		m.logger.Debug("validating package resources",
			"package", pkg.Name,
			"resource_count", len(pkg.Resources),
		)
	}

	issues := ValidatePackageReferences(pkg, index)
	for _, issue := range issues {
		if issue.Reference != "" {
			missing = append(missing, issue.Reference)
		} else {
			missing = append(missing, fmt.Sprintf("<invalid>: %s", issue.Message))
		}

		if m.logger != nil {
			m.logger.Debug("package resource not found",
				"package", pkg.Name,
				"reference", issue.Reference,
				"message", issue.Message,
				"suggestion", issue.Suggestion,
			)
		}
	}

	// DEBUG: Log validation summary
	if m.logger != nil {
		m.logger.Debug("package validation complete",
			"package", pkg.Name,
			"total", len(pkg.Resources),
			"missing", len(missing),
			"missing_list", missing,
		)
	}

	return missing
}

// List lists all resources, optionally filtered by type

func (m *Manager) ValidatePackageResources(pkg *resource.Package) []string {
	return m.validatePackageResources(pkg)
}

// GetPackage loads a package by name from the repository
func (m *Manager) GetPackage(name string) (*resource.Package, error) {
	// DEBUG: Log get package operation
	if m.logger != nil {
		m.logger.Debug("repo get package",
			"package", name,
		)
	}

	pkgPath := resource.GetPackagePath(name, m.repoPath)
	pkg, err := resource.LoadPackageLenient(pkgPath)
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to load package",
				"package", name,
				"path", pkgPath,
				"error", err.Error(),
			)
		}
		return nil, err
	}

	// DEBUG: Log package details after successful load
	if m.logger != nil {
		m.logger.Debug("package retrieved",
			"name", pkg.Name,
			"description", pkg.Description,
			"resource_count", len(pkg.Resources),
			"resources", pkg.Resources,
		)
	}

	return pkg, nil
}

// Drop removes the entire repository directory and recreates the empty structure.
// WARNING: This is a destructive operation that cannot be undone.
// NOTE: After drop, Init() is called which recreates an empty ai.repo.yaml manifest
// with no sources. This is the expected behavior for soft drop - the repository
// is ready to accept new sources via 'repo add' or 'repo sync'.
