package cmd

import (
	"fmt"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/manifest"
)

func loadProjectManifestView(projectPath string) (*manifest.ProjectManifests, error) {
	view, err := manifest.LoadProjectManifests(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load project manifest: %w", err)
	}
	return view, nil
}

func loadEffectiveProjectManifest(projectPath string) (*manifest.Manifest, *manifest.ProjectManifests, error) {
	view, err := loadProjectManifestView(projectPath)
	if err != nil {
		return nil, nil, err
	}
	if !view.HasAny() {
		return nil, view, nil
	}
	return view.Effective, view, nil
}
