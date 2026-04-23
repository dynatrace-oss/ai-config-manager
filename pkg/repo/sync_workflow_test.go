package repo

import (
	"errors"
	"testing"

	resmeta "github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestRunSyncOrchestrator_TracksFailuresWarningsAndPrune(t *testing.T) {
	manifest := &repomanifest.Manifest{Sources: []*repomanifest.Source{
		{Name: "ok-source", Path: "/ok", ID: "src-ok"},
		{Name: "bad-source", Path: "/bad", ID: "src-bad"},
	}}

	updated := 0
	started := 0
	discoveryPrinted := 0
	repoPath := t.TempDir()
	if err := resmeta.Save(&resmeta.ResourceMetadata{
		Name:       "gone",
		Type:       resource.Command,
		SourceID:   "src-ok",
		SourceName: "ok-source",
	}, repoPath, "ok-source"); err != nil {
		t.Fatalf("failed to seed metadata: %v", err)
	}

	deps := SyncOrchestratorDeps{
		Manager:          NewManagerWithPath(t.TempDir()),
		Manifest:         manifest,
		RepoPath:         repoPath,
		PreSyncResources: map[string][]SyncResourceInfo{"src-ok": {{Name: "gone", Type: resource.Command}}},
		DryRun:           false,
		Prune:            true,
		SyncSource: func(src *repomanifest.Source, _ *Manager) (string, any, []resource.DiscoveryError, error) {
			if src.Name == "bad-source" {
				return "", nil, nil, errors.New("boom")
			}
			return src.Path, "bulk", []resource.DiscoveryError{{Path: "commands/bad.md", Error: errors.New("invalid")}}, nil
		},
		SourceScanner: func(sourcePath, _ string) (map[resource.ResourceType]map[string]bool, error) {
			if sourcePath == "/ok" {
				return map[resource.ResourceType]map[string]bool{resource.Command: {}}, nil
			}
			return map[resource.ResourceType]map[string]bool{}, nil
		},
		OnSourceStart: func(_ *repomanifest.Source) { started++ },
		OnDiscoveryError: func(_ string, _ []resource.DiscoveryError) {
			discoveryPrinted++
		},
		UpdateMetadata: func(_ *repomanifest.Source) { updated++ },
	}

	got := RunSyncOrchestrator(deps)

	if started != 2 {
		t.Fatalf("expected start callback for each source, got %d", started)
	}
	if discoveryPrinted != 1 {
		t.Fatalf("expected discovery callback once, got %d", discoveryPrinted)
	}
	if updated != 1 {
		t.Fatalf("expected metadata update only for successful source, got %d", updated)
	}
	if got.SourcesProcessed != 1 || got.SourcesFailed != 1 {
		t.Fatalf("unexpected source counters: %+v", got)
	}
	if len(got.FailedSources) != 1 || got.FailedSources[0] != "bad-source" {
		t.Fatalf("unexpected failed sources: %#v", got.FailedSources)
	}
	if len(got.RemovedResources["src-ok"]) != 1 {
		t.Fatalf("expected prune candidate for ok source, got %#v", got.RemovedResources)
	}
	if len(got.Warnings) == 0 {
		t.Fatalf("expected orchestrator warnings to include discovery summary")
	}
}

func TestRunSyncOrchestrator_DryRunSkipsMetadataUpdates(t *testing.T) {
	manifest := &repomanifest.Manifest{Sources: []*repomanifest.Source{{Name: "ok", Path: "/ok"}}}
	updated := 0

	got := RunSyncOrchestrator(SyncOrchestratorDeps{
		Manager:          NewManagerWithPath(t.TempDir()),
		Manifest:         manifest,
		RepoPath:         t.TempDir(),
		PreSyncResources: map[string][]SyncResourceInfo{},
		DryRun:           true,
		Prune:            false,
		SyncSource: func(src *repomanifest.Source, _ *Manager) (string, any, []resource.DiscoveryError, error) {
			return src.Path, nil, nil, nil
		},
		SourceScanner:  func(string, string) (map[resource.ResourceType]map[string]bool, error) { return nil, nil },
		UpdateMetadata: func(_ *repomanifest.Source) { updated++ },
	})

	if updated != 0 {
		t.Fatalf("expected no metadata updates during dry-run, got %d", updated)
	}
	if got.SourcesProcessed != 1 || got.SourcesFailed != 0 {
		t.Fatalf("unexpected dry-run counters: %+v", got)
	}
}
