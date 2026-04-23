package repo

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

type SyncSourceFunc func(src *repomanifest.Source, manager *Manager) (string, any, []resource.DiscoveryError, error)

type SyncSourceRunResult struct {
	Source          *repomanifest.Source
	SourcePath      string
	Result          any
	DiscoveryErrors []resource.DiscoveryError
	Error           error
	RemovedCount    int
	Warnings        []string
	Failed          bool
}

type SyncRunResult struct {
	SourceResults    []SyncSourceRunResult
	SourcesProcessed int
	SourcesFailed    int
	FailedSources    []string
	RemovedResources map[string][]SyncResourceInfo
	Warnings         []string
}

type SyncOrchestratorDeps struct {
	Manager          *Manager
	Manifest         *repomanifest.Manifest
	RepoPath         string
	PreSyncResources map[string][]SyncResourceInfo
	DryRun           bool
	Prune            bool
	Stdout           io.Writer
	Logger           *slog.Logger
	SyncSource       SyncSourceFunc
	SourceScanner    SourceScanner
	OnSourceStart    func(src *repomanifest.Source)
	OnDiscoveryError func(sourceName string, discoveryErrors []resource.DiscoveryError)
	UpdateMetadata   func(src *repomanifest.Source)
}

func RunSyncOrchestrator(deps SyncOrchestratorDeps) SyncRunResult {
	result := SyncRunResult{
		FailedSources:    make([]string, 0),
		RemovedResources: make(map[string][]SyncResourceInfo),
		SourceResults:    make([]SyncSourceRunResult, 0, len(deps.Manifest.Sources)),
	}

	for _, src := range deps.Manifest.Sources {
		if deps.OnSourceStart != nil {
			deps.OnSourceStart(src)
		}

		run := SyncSourceRunResult{Source: src}
		sourcePath, bulkResult, discoveryErrors, syncErr := deps.SyncSource(src, deps.Manager)
		run.SourcePath = sourcePath
		run.Result = bulkResult
		run.DiscoveryErrors = discoveryErrors

		if len(discoveryErrors) > 0 {
			run.Warnings = append(run.Warnings, fmt.Sprintf("%d resource(s) were skipped during discovery due to validation issues", len(discoveryErrors)))
			result.Warnings = append(result.Warnings, fmt.Sprintf("source '%s': %d resource(s) were skipped during discovery due to validation issues", src.Name, len(discoveryErrors)))
			if deps.OnDiscoveryError != nil {
				deps.OnDiscoveryError(src.Name, discoveryErrors)
			}
		}

		if syncErr != nil {
			run.Failed = true
			run.Error = syncErr
			result.SourcesFailed++
			result.FailedSources = append(result.FailedSources, src.Name)
			result.SourceResults = append(result.SourceResults, run)
			continue
		}

		sourceKey := CanonicalSourceID(src)
		if sourceKey == "" {
			sourceKey = src.Name
		}
		if deps.Prune {
			removed, detectWarnings := DetectRemovedForSource(src, sourcePath, deps.RepoPath, deps.PreSyncResources, deps.SourceScanner, deps.Logger)
			run.RemovedCount = len(removed)
			run.Warnings = append(run.Warnings, detectWarnings...)
			if len(removed) > 0 {
				result.RemovedResources[sourceKey] = removed
			}
			result.Warnings = append(result.Warnings, detectWarnings...)
		}

		if !deps.DryRun && deps.UpdateMetadata != nil {
			deps.UpdateMetadata(src)
		}

		result.SourcesProcessed++
		result.SourceResults = append(result.SourceResults, run)
	}

	return result
}
