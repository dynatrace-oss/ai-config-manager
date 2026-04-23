package repo

import (
	"strings"
	"testing"

	resmeta "github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

func TestCanonicalSourceID_PrefersStableManifestID(t *testing.T) {
	src := &repomanifest.Source{ID: "src-explicit", URL: "https://example.com/repo.git"}
	if got := CanonicalSourceID(src); got != "src-explicit" {
		t.Fatalf("CanonicalSourceID() = %q, want %q", got, "src-explicit")
	}
}

func TestSourceLegacyRemoteIDAlias_ForOverrideSource(t *testing.T) {
	src := &repomanifest.Source{
		URL:                     "https://mirror.example.com/repo.git",
		OverrideOriginalURL:     "https://github.com/org/repo.git",
		Subpath:                 "plugins/a",
		OverrideOriginalSubpath: "plugins/a",
	}

	legacyID, canonicalID := SourceLegacyRemoteIDAlias(src)
	if legacyID == "" || canonicalID == "" {
		t.Fatalf("expected both legacy and canonical ids, got legacy=%q canonical=%q", legacyID, canonicalID)
	}
	if legacyID == canonicalID {
		t.Fatalf("expected different ids for alias remap")
	}
}

func TestRemapLegacyRemoteSourceIDs_MovesResources(t *testing.T) {
	src := &repomanifest.Source{URL: "https://github.com/org/repo.git", Subpath: "plugins/a"}
	legacyID, canonicalID := SourceLegacyRemoteIDAlias(src)
	if legacyID == "" || canonicalID == "" {
		t.Fatalf("expected alias ids for test source")
	}

	pre := map[string][]SyncResourceInfo{
		legacyID: {{Name: "demo", Type: resource.Command}},
	}
	manifest := &repomanifest.Manifest{Sources: []*repomanifest.Source{src}}

	RemapLegacyRemoteSourceIDs(pre, manifest)

	if _, ok := pre[legacyID]; ok {
		t.Fatalf("expected legacy key to be removed")
	}
	if len(pre[canonicalID]) != 1 {
		t.Fatalf("expected canonical key to contain moved resources, got %#v", pre)
	}
}

func TestApplyIncludeFilterToDiscovered_FiltersAndPrunesEmptyTypes(t *testing.T) {
	resources := map[resource.ResourceType]map[string]bool{
		resource.Command: {"keep-cmd": true, "drop-cmd": true},
		resource.Skill:   {"drop-skill": true},
	}

	if err := ApplyIncludeFilterToDiscovered(resources, []string{"command/keep-*"}); err != nil {
		t.Fatalf("ApplyIncludeFilterToDiscovered() error = %v", err)
	}

	if !resources[resource.Command]["keep-cmd"] || resources[resource.Command]["drop-cmd"] {
		t.Fatalf("unexpected command filtering result: %#v", resources[resource.Command])
	}
	if _, ok := resources[resource.Skill]; ok {
		t.Fatalf("expected empty skill set to be removed")
	}
}

func TestPreSyncResourcesForSource_MergesLegacyNameAndDedups(t *testing.T) {
	pre := map[string][]SyncResourceInfo{
		"src-id": {
			{Name: "demo", Type: resource.Command},
		},
		"legacy-name": {
			{Name: "demo", Type: resource.Command},
			{Name: "assistant", Type: resource.Agent},
		},
	}

	got := PreSyncResourcesForSource(pre, "src-id", "legacy-name")
	if len(got) != 2 {
		t.Fatalf("expected deduped merged resources, got %#v", got)
	}
}

func TestDetectSyncResourceCollisions_ReportsConflicts(t *testing.T) {
	manifest := &repomanifest.Manifest{Sources: []*repomanifest.Source{
		{Name: "source-a", ID: "src-a", Path: "/a"},
		{Name: "source-b", ID: "src-b", Path: "/b"},
	}}

	resolve := func(src *repomanifest.Source, _ *Manager) (string, error) {
		return src.Path, nil
	}
	scanner := func(sourcePath, _ string) (map[resource.ResourceType]map[string]bool, error) {
		if sourcePath == "/a" || sourcePath == "/b" {
			return map[resource.ResourceType]map[string]bool{resource.Command: {"collision": true}}, nil
		}
		return nil, nil
	}

	err := DetectSyncResourceCollisions(manifest, NewManagerWithPath(t.TempDir()), resolve, scanner)
	if err == nil {
		t.Fatalf("expected collision error")
	}
	if !strings.Contains(err.Error(), "command/collision") {
		t.Fatalf("expected resource reference in error, got: %v", err)
	}
}

func TestResourceBelongsToSource_PackageUsesSourceIDAndNameFallback(t *testing.T) {
	repoPath := t.TempDir()
	if err := resmeta.SavePackageMetadata(&resmeta.PackageMetadata{
		Name:       "bundle",
		SourceType: "local",
		SourceID:   "src-id",
		SourceName: "legacy-name",
	}, repoPath); err != nil {
		t.Fatalf("failed to write package metadata: %v", err)
	}

	if !ResourceBelongsToSource("bundle", resource.PackageType, "src-id", "legacy-name", repoPath) {
		t.Fatalf("expected package to belong by source id")
	}
	if !ResourceBelongsToSource("bundle", resource.PackageType, "legacy-name", "legacy-name", repoPath) {
		t.Fatalf("expected package to belong by source name fallback")
	}
}
