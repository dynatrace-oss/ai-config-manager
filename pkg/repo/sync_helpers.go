package repo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	resmeta "github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
)

// SyncResourceInfo holds the name and type of a resource for sync inventory tracking.
type SyncResourceInfo struct {
	Name string
	Type resource.ResourceType
}

// SourceScanner scans a source path and returns discovered resources by type/name.
type SourceScanner func(sourcePath, discoveryMode string) (map[resource.ResourceType]map[string]bool, error)

type sourceResourceClaim struct {
	canonicalSourceID string
	sourceName        string
	sourceLocation    string
}

// CollectResourcesBySource returns a map of source identifier -> []SyncResourceInfo
// for all resources in the repo that have a source assigned.
func CollectResourcesBySource(repoPath string) (map[string][]SyncResourceInfo, error) {
	bySource := make(map[string][]SyncResourceInfo)

	types := []struct {
		resType resource.ResourceType
		metaDir string
	}{
		{resource.Command, "commands"},
		{resource.Skill, "skills"},
		{resource.Agent, "agents"},
	}

	for _, rt := range types {
		metaDirPath := filepath.Join(repoPath, ".metadata", rt.metaDir)
		entries, err := os.ReadDir(metaDirPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read metadata dir %s: %w", rt.metaDir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-metadata.json") {
				continue
			}

			fallbackName := strings.TrimSuffix(entry.Name(), "-metadata.json")
			meta, err := resmeta.Load(fallbackName, rt.resType, repoPath)
			if err != nil {
				continue
			}

			name := meta.Name
			if name == "" {
				name = fallbackName
			}

			key := meta.SourceID
			if key == "" {
				key = meta.SourceName
			}
			if key == "" {
				continue
			}

			bySource[key] = append(bySource[key], SyncResourceInfo{Name: name, Type: rt.resType})
		}
	}

	pkgMetaDir := filepath.Join(repoPath, ".metadata", "packages")
	pkgEntries, err := os.ReadDir(pkgMetaDir)
	if err == nil {
		for _, entry := range pkgEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-metadata.json") {
				continue
			}

			fallbackName := strings.TrimSuffix(entry.Name(), "-metadata.json")
			pkgMeta, err := resmeta.LoadPackageMetadata(fallbackName, repoPath)
			if err != nil {
				continue
			}

			name := pkgMeta.Name
			if name == "" {
				name = fallbackName
			}

			key := pkgMeta.SourceID
			if key == "" {
				key = pkgMeta.SourceName
			}
			if key == "" {
				continue
			}

			bySource[key] = append(bySource[key], SyncResourceInfo{Name: name, Type: resource.PackageType})
		}
	}

	return bySource, nil
}

func ApplyIncludeFilterToDiscovered(sourceResources map[resource.ResourceType]map[string]bool, include []string) error {
	if len(include) == 0 {
		return nil
	}

	mm, err := pattern.NewMultiMatcher(include)
	if err != nil {
		return fmt.Errorf("invalid include patterns: %w", err)
	}

	for resType, typeSet := range sourceResources {
		for name := range typeSet {
			res := &resource.Resource{Type: resType, Name: name}
			if !mm.Match(res) {
				delete(typeSet, name)
			}
		}
		if len(typeSet) == 0 {
			delete(sourceResources, resType)
		}
	}

	return nil
}

func CanonicalSourceID(src *repomanifest.Source) string {
	if src == nil {
		return ""
	}

	if src.ID != "" && src.OverrideOriginalURL == "" {
		return src.ID
	}

	return repomanifest.GenerateSourceID(src)
}

func SourceLegacyRemoteIDAlias(src *repomanifest.Source) (legacyID string, canonicalID string) {
	if src == nil {
		return "", ""
	}

	var remoteURL string
	var remoteSubpath string
	if src.OverrideOriginalURL != "" {
		remoteURL = src.OverrideOriginalURL
		remoteSubpath = src.OverrideOriginalSubpath
	} else {
		remoteURL = src.URL
		remoteSubpath = src.Subpath
	}

	if remoteURL == "" {
		return "", ""
	}

	canonicalID = CanonicalSourceID(src)
	if canonicalID == "" {
		return "", ""
	}

	legacyID = repomanifest.GenerateSourceID(&repomanifest.Source{URL: remoteURL})
	if legacyID == "" || legacyID == canonicalID {
		return "", ""
	}

	if repomanifest.GenerateSourceID(&repomanifest.Source{URL: remoteURL, Subpath: remoteSubpath}) == legacyID {
		return "", ""
	}

	return legacyID, canonicalID
}

func RemapLegacyRemoteSourceIDs(preSyncResources map[string][]SyncResourceInfo, manifest *repomanifest.Manifest) {
	if len(preSyncResources) == 0 || manifest == nil {
		return
	}

	for _, src := range manifest.Sources {
		legacyID, canonicalID := SourceLegacyRemoteIDAlias(src)
		if legacyID == "" || canonicalID == "" {
			continue
		}

		resources, exists := preSyncResources[legacyID]
		if !exists || len(resources) == 0 {
			continue
		}

		preSyncResources[canonicalID] = append(preSyncResources[canonicalID], resources...)
		delete(preSyncResources, legacyID)
	}
}

func SourceLocationSummary(src *repomanifest.Source) string {
	if src == nil {
		return "unknown location"
	}

	if src.URL != "" {
		if src.Ref != "" {
			if src.Subpath != "" {
				return fmt.Sprintf("url %q (ref %q, subpath %q)", src.URL, src.Ref, src.Subpath)
			}
			return fmt.Sprintf("url %q (ref %q)", src.URL, src.Ref)
		}
		if src.Subpath != "" {
			return fmt.Sprintf("url %q (subpath %q)", src.URL, src.Subpath)
		}
		return fmt.Sprintf("url %q", src.URL)
	}

	if src.Path != "" {
		return fmt.Sprintf("path %q", src.Path)
	}

	return "unknown location"
}

func DetectSyncResourceCollisions(manifest *repomanifest.Manifest, manager *Manager, resolve func(*repomanifest.Source, *Manager) (string, error), scanner SourceScanner) error {
	if manifest == nil || len(manifest.Sources) == 0 {
		return nil
	}

	claims := make(map[string]sourceResourceClaim)
	conflicts := make([]string, 0)

	for _, src := range manifest.Sources {
		sourcePath, err := resolve(src, manager)
		if err != nil {
			continue
		}

		sourceResources, err := scanner(sourcePath, src.Discovery)
		if err != nil {
			continue
		}

		if err := ApplyIncludeFilterToDiscovered(sourceResources, src.Include); err != nil {
			return fmt.Errorf("source %q has invalid include filters for sync collision precheck: %w", src.Name, err)
		}

		currentSourceID := CanonicalSourceID(src)
		currentLocation := SourceLocationSummary(src)

		for resType, typeSet := range sourceResources {
			for name := range typeSet {
				resourceRef := fmt.Sprintf("%s/%s", resType, name)
				if claim, exists := claims[resourceRef]; exists {
					if claim.canonicalSourceID != currentSourceID {
						conflicts = append(conflicts, fmt.Sprintf(
							"%s provided by source %q (%s) and source %q (%s)",
							resourceRef,
							claim.sourceName,
							claim.sourceLocation,
							src.Name,
							currentLocation,
						))
					}
					continue
				}

				claims[resourceRef] = sourceResourceClaim{
					canonicalSourceID: currentSourceID,
					sourceName:        src.Name,
					sourceLocation:    currentLocation,
				}
			}
		}
	}

	if len(conflicts) == 0 {
		return nil
	}

	sort.Strings(conflicts)
	return fmt.Errorf("sync rejected: conflicting resource names across different sources:\n  - %s\nResolve by renaming one resource, narrowing include filters, or removing one of the conflicting sources", strings.Join(conflicts, "\n  - "))
}

func PreSyncResourcesForSource(preSyncResources map[string][]SyncResourceInfo, sourceKey, sourceName string) []SyncResourceInfo {
	preSyncSet := preSyncResources[sourceKey]

	if sourceName != "" && sourceName != sourceKey {
		preSyncSet = append(preSyncSet, preSyncResources[sourceName]...)
	}

	if len(preSyncSet) <= 1 {
		return preSyncSet
	}

	seen := make(map[string]bool, len(preSyncSet))
	deduped := make([]SyncResourceInfo, 0, len(preSyncSet))
	for _, res := range preSyncSet {
		key := string(res.Type) + "/" + res.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, res)
	}

	return deduped
}

func ResourceBelongsToSource(name string, resType resource.ResourceType, sourceIdentifier, sourceName, repoPath string) bool {
	if resType != resource.PackageType {
		return resmeta.HasSource(name, resType, sourceIdentifier, repoPath)
	}

	pkgMeta, err := resmeta.LoadPackageMetadata(name, repoPath)
	if err != nil {
		return false
	}

	if pkgMeta.SourceID != "" && pkgMeta.SourceID == sourceIdentifier {
		return true
	}
	if pkgMeta.SourceName != "" && pkgMeta.SourceName == sourceIdentifier {
		return true
	}

	return sourceName != "" && pkgMeta.SourceName == sourceName
}

func DetectRemovedForSource(src *repomanifest.Source, sourcePath, repoPath string,
	preSyncResources map[string][]SyncResourceInfo, scanner SourceScanner, logger *slog.Logger) ([]SyncResourceInfo, []string) {

	sourceKey := CanonicalSourceID(src)
	if sourceKey == "" {
		sourceKey = src.Name
	}

	preSyncSet := PreSyncResourcesForSource(preSyncResources, sourceKey, src.Name)
	if len(preSyncSet) == 0 {
		return nil, nil
	}

	sourceResources, scanErr := scanner(sourcePath, src.Discovery)
	if scanErr != nil {
		return nil, []string{fmt.Sprintf("could not scan source %s for removal detection: %v", src.Name, scanErr)}
	}

	if len(src.Include) > 0 {
		if err := ApplyIncludeFilterToDiscovered(sourceResources, src.Include); err != nil {
			if logger != nil {
				logger.Warn("invalid include patterns in source, skipping include filter for orphan detection", "source", src.Name, "error", err)
			}
		}
	}

	var removed []SyncResourceInfo
	var warnings []string
	for _, res := range preSyncSet {
		if typeSet, ok := sourceResources[res.Type]; ok {
			if typeSet[res.Name] {
				continue
			}
		}

		belongsToSource := ResourceBelongsToSource(res.Name, res.Type, sourceKey, src.Name, repoPath)
		if !belongsToSource && sourceKey != src.Name {
			belongsToSource = ResourceBelongsToSource(res.Name, res.Type, src.Name, src.Name, repoPath)
		}
		if !belongsToSource {
			warnings = append(warnings,
				fmt.Sprintf("skipping removal of %s/%s from %s: metadata now points to a different source", res.Type, res.Name, src.Name),
			)
			continue
		}

		removed = append(removed, res)
	}
	return removed, warnings
}
