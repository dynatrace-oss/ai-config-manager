package repomanifest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/fileutil"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/pattern"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/sourcemetadata"
)

// Pre-compiled regexps for source name validation and generation.
var (
	validSourceNameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	invalidNameCharRe = regexp.MustCompile(`[^a-z0-9-]`)
)

const (
	// ManifestFileName is the default name for repository manifest files
	ManifestFileName = "ai.repo.yaml"
)

// Manifest represents the ai.repo.yaml file that tracks synced sources
type Manifest struct {
	Version int       `yaml:"version"`
	Sources []*Source `yaml:"sources,omitempty"`
}

// Source represents a single synced source in the repository
type Source struct {
	ID      string   `yaml:"id,omitempty"`
	Name    string   `yaml:"name"`
	Path    string   `yaml:"path,omitempty"`
	URL     string   `yaml:"url,omitempty"`
	Ref     string   `yaml:"ref,omitempty"`
	Subpath string   `yaml:"subpath,omitempty"`
	Include []string `yaml:"include,omitempty"`

	// Override breadcrumbs are runtime-only on Source and persisted locally in
	// .metadata/sources.json (not in shareable ai.repo.yaml output).
	OverrideOriginalURL     string `yaml:"-"`
	OverrideOriginalRef     string `yaml:"-"`
	OverrideOriginalSubpath string `yaml:"-"`
}

// MarshalYAML writes shareable source config to ai.repo.yaml.
// Local-only runtime state (ID) is intentionally omitted.
func (s *Source) MarshalYAML() (interface{}, error) {
	type sourceYAML struct {
		Name    string   `yaml:"name"`
		Path    string   `yaml:"path,omitempty"`
		URL     string   `yaml:"url,omitempty"`
		Ref     string   `yaml:"ref,omitempty"`
		Subpath string   `yaml:"subpath,omitempty"`
		Include []string `yaml:"include,omitempty"`
	}

	if s == nil {
		return sourceYAML{}, nil
	}

	return sourceYAML{
		Name:    s.Name,
		Path:    s.Path,
		URL:     s.URL,
		Ref:     s.Ref,
		Subpath: s.Subpath,
		Include: s.Include,
	}, nil
}

// GetMode returns the implicit mode for this source
// path sources use symlink, url sources use copy
func (s *Source) GetMode() string {
	if s.Path != "" {
		return "symlink"
	}
	if s.URL != "" {
		return "copy"
	}
	return ""
}

// Load loads a manifest from the repository's ai.repo.yaml file.
//
// This function is read-only and never writes to disk.
// If the file doesn't exist, it returns an empty manifest (not an error).
func Load(repoPath string) (*Manifest, error) {
	path := filepath.Join(repoPath, ManifestFileName)

	// If file doesn't exist, return empty manifest
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Manifest{
			Version: 1,
			Sources: []*Source{},
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	if err := m.hydrateOverrideBreadcrumbs(repoPath); err != nil {
		return nil, fmt.Errorf("failed to load override breadcrumbs: %w", err)
	}

	// Ensure every source has an in-memory runtime ID. IDs are local-only state
	// derived from URL/path and are intentionally not persisted in ai.repo.yaml.
	m.migrateSourceIDs()

	// Validate the manifest
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	return &m, nil
}

// LoadForMutation loads a manifest and applies legacy migration writes when needed.
//
// IMPORTANT: this function may mutate repository-managed state files
// (ai.repo.yaml and .metadata/sources.json). Callers must hold the repo-wide lock
// before invoking this method.
func LoadForMutation(repoPath string) (*Manifest, error) {
	m, err := Load(repoPath)
	if err != nil {
		return nil, err
	}

	if err := migrateIfNeeded(repoPath, m); err != nil {
		return nil, fmt.Errorf("failed to migrate manifest: %w", err)
	}

	return m, nil
}

// Save writes the manifest to the repository's ai.repo.yaml file
func (m *Manifest) Save(repoPath string) error {
	if m == nil {
		return fmt.Errorf("cannot save nil manifest")
	}

	// Validate before saving
	if err := m.Validate(); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to YAML with pretty printing
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Write to file
	path := filepath.Join(repoPath, ManifestFileName)
	if err := fileutil.AtomicWrite(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	if err := m.persistOverrideBreadcrumbs(repoPath); err != nil {
		return fmt.Errorf("failed to persist override breadcrumbs: %w", err)
	}

	return nil
}

// Validate checks if the manifest is valid
func (m *Manifest) Validate() error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}

	if m.Version != 1 {
		return fmt.Errorf("invalid version: %d (expected 1)", m.Version)
	}

	// Check for duplicate names and IDs
	names := make(map[string]bool)
	ids := make(map[string]bool)
	for _, source := range m.Sources {
		if err := validateSource(source); err != nil {
			return fmt.Errorf("invalid source '%s': %w", source.Name, err)
		}

		if names[source.Name] {
			return fmt.Errorf("duplicate source name: %s", source.Name)
		}
		names[source.Name] = true

		if source.ID != "" {
			if ids[source.ID] {
				return fmt.Errorf("duplicate source ID: %s", source.ID)
			}
			ids[source.ID] = true
		}
	}

	return nil
}

// AddSource adds a new source to the manifest
// Auto-generates name if not provided
// Returns error if source with same name already exists
func (m *Manifest) AddSource(source *Source) error {
	if m == nil {
		return fmt.Errorf("cannot add to nil manifest")
	}

	if source == nil {
		return fmt.Errorf("cannot add nil source")
	}

	// Auto-generate name if not provided
	if source.Name == "" {
		source.Name = generateSourceName(source)
	}

	// Auto-generate ID if not provided
	if source.ID == "" {
		source.ID = GenerateSourceID(source)
	}

	// Validate source
	if err := validateSource(source); err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}

	// Check for duplicate source ID (same canonical location, different name)
	if source.ID != "" {
		for _, existing := range m.Sources {
			if existing.ID == source.ID && existing.Name != source.Name {
				return fmt.Errorf("source with same location already exists as '%s' (ID: %s)", existing.Name, existing.ID)
			}
		}
	}

	// Check for duplicate name
	if m.HasSource(source.Name) {
		return fmt.Errorf("source with name '%s' already exists", source.Name)
	}

	m.Sources = append(m.Sources, source)
	return nil
}

// RemoveSource removes a source by ID, name, path, or URL.
// ID matching has highest priority, followed by name, then path/URL.
// Returns the removed source and nil error if found.
// Returns nil and error if not found.
func (m *Manifest) RemoveSource(identifier string) (*Source, error) {
	if m == nil {
		return nil, fmt.Errorf("cannot remove from nil manifest")
	}

	if identifier == "" {
		return nil, fmt.Errorf("nameOrPath cannot be empty")
	}

	// First pass: match by ID (highest priority)
	for i, source := range m.Sources {
		if source.ID != "" && source.ID == identifier {
			m.Sources = append(m.Sources[:i], m.Sources[i+1:]...)
			return source, nil
		}
	}

	// Second pass: match by name, path, or URL
	for i, source := range m.Sources {
		if source.Name == identifier || source.Path == identifier || source.URL == identifier {
			m.Sources = append(m.Sources[:i], m.Sources[i+1:]...)
			return source, nil
		}
	}

	return nil, fmt.Errorf("source not found: %s", identifier)
}

// GetSource finds a source by ID, name, path, or URL.
// ID matching has highest priority, followed by name, then path/URL.
// Returns the source and true if found, nil and false otherwise.
func (m *Manifest) GetSource(identifier string) (*Source, bool) {
	if m == nil || identifier == "" {
		return nil, false
	}

	// First pass: match by ID (highest priority)
	for _, source := range m.Sources {
		if source.ID != "" && source.ID == identifier {
			return source, true
		}
	}

	// Second pass: match by name, path, or URL
	for _, source := range m.Sources {
		if source.Name == identifier || source.Path == identifier || source.URL == identifier {
			return source, true
		}
	}

	return nil, false
}

// HasSource checks if a source exists by name, path, or URL
func (m *Manifest) HasSource(nameOrPath string) bool {
	_, found := m.GetSource(nameOrPath)
	return found
}

// validateSource validates a single source
func validateSource(source *Source) error {
	if source == nil {
		return fmt.Errorf("source is nil")
	}

	if source.Name == "" {
		return fmt.Errorf("source name cannot be empty")
	}

	// Validate name format (agentskills.io naming rules)
	if !isValidSourceName(source.Name) {
		return fmt.Errorf("invalid source name '%s': must be lowercase alphanumeric with hyphens, 1-64 chars", source.Name)
	}

	hasOverrideBreadcrumbs := source.OverrideOriginalURL != "" || source.OverrideOriginalRef != "" || source.OverrideOriginalSubpath != ""

	if hasOverrideBreadcrumbs {
		if source.Path == "" {
			return fmt.Errorf("override breadcrumbs require an active local path source")
		}
		if source.URL != "" {
			return fmt.Errorf("override breadcrumbs require local path source; url must be empty")
		}
		if source.OverrideOriginalURL == "" {
			return fmt.Errorf("override breadcrumbs require original remote url")
		}
	} else {
		// Must have either path or URL
		if source.Path == "" && source.URL == "" {
			return fmt.Errorf("source must have either path or url")
		}

		// Cannot have both path and URL
		if source.Path != "" && source.URL != "" {
			return fmt.Errorf("source cannot have both path and url")
		}
	}

	// Validate include patterns
	for _, entry := range source.Include {
		if _, err := pattern.NewMatcher(entry); err != nil {
			return fmt.Errorf("invalid include pattern %q: %w", entry, err)
		}
	}

	return nil
}

// isValidSourceName validates source name follows agentskills.io naming rules
func isValidSourceName(name string) bool {
	if len(name) < 1 || len(name) > 64 {
		return false
	}

	// Must be lowercase alphanumeric + hyphens only
	// Cannot start/end with hyphen
	// No consecutive hyphens
	if !validSourceNameRe.MatchString(name) {
		return false
	}

	// Check for consecutive hyphens
	if strings.Contains(name, "--") {
		return false
	}

	return true
}

// generateSourceName generates a filesystem-safe name from path or URL
func generateSourceName(source *Source) string {
	var base string

	if source.Path != "" {
		// Use last component of path
		base = filepath.Base(source.Path)
	} else if source.URL != "" {
		// Extract repo name from URL
		// Examples:
		// https://github.com/user/repo -> repo
		// https://github.com/user/repo.git -> repo
		// git@github.com:user/repo.git -> repo
		url := source.URL
		url = strings.TrimSuffix(url, ".git")
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			base = parts[len(parts)-1]
		}
		// Handle git@ format
		if strings.Contains(base, ":") {
			parts := strings.Split(base, ":")
			if len(parts) > 1 {
				base = parts[len(parts)-1]
			}
		}
	}

	if base == "" {
		base = "source"
	}

	// Convert to lowercase
	base = strings.ToLower(base)

	// Replace invalid characters with hyphens
	name := invalidNameCharRe.ReplaceAllString(base, "-")

	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Replace consecutive hyphens with single hyphen
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	// Ensure not empty
	if name == "" {
		name = "source"
	}

	// Truncate to 64 chars
	if len(name) > 64 {
		name = name[:64]
		name = strings.TrimRight(name, "-")
	}

	return name
}

// migrateSourceIDs generates runtime IDs for any sources that lack them.
// Returns true if any IDs were generated.
func (m *Manifest) migrateSourceIDs() bool {
	migrated := false
	for i := range m.Sources {
		if m.Sources[i].ID == "" {
			m.Sources[i].ID = GenerateSourceID(m.Sources[i])
			if m.Sources[i].ID != "" {
				migrated = true
			}
		}
	}
	return migrated
}

// migrateIfNeeded migrates old manifest format to new format
// Old format had Mode, Added, LastSynced in Source struct
// New format moves these to .metadata/sources.json
func migrateIfNeeded(repoPath string, m *Manifest) error {
	// Check if migration is needed by looking for old-format data in raw YAML
	// We need to re-parse to detect fields that aren't in our struct anymore
	path := filepath.Join(repoPath, ManifestFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Parse into a generic map to detect old fields
	var rawManifest map[string]interface{}
	if err := yaml.Unmarshal(data, &rawManifest); err != nil {
		return err
	}

	sources, ok := rawManifest["sources"].([]interface{})
	if !ok || len(sources) == 0 {
		return nil // No sources, nothing to migrate
	}

	// Check if any source has old-format fields
	needsMigration := false
	for _, src := range sources {
		srcMap, ok := src.(map[string]interface{})
		if !ok {
			continue
		}
		if _, hasMode := srcMap["mode"]; hasMode {
			needsMigration = true
			break
		}
		if _, hasAdded := srcMap["added"]; hasAdded {
			needsMigration = true
			break
		}
	}

	if !needsMigration {
		return nil // Already migrated or new format
	}

	// Load existing sourcemetadata or create new
	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		return fmt.Errorf("failed to load source metadata: %w", err)
	}

	// Extract state from each source
	for _, src := range sources {
		srcMap, ok := src.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := srcMap["name"].(string)
		if name == "" {
			continue
		}

		// Extract Added timestamp
		if addedStr, ok := srcMap["added"].(string); ok {
			if added, err := time.Parse(time.RFC3339, addedStr); err == nil {
				metadata.SetAdded(name, added)
			}
		}

		// Extract LastSynced timestamp
		if syncedStr, ok := srcMap["last_synced"].(string); ok {
			if synced, err := time.Parse(time.RFC3339, syncedStr); err == nil {
				metadata.SetLastSynced(name, synced)
			}
		}
	}

	// Save metadata
	if err := metadata.Save(repoPath); err != nil {
		return fmt.Errorf("failed to save source metadata: %w", err)
	}

	// Save cleaned manifest (without Mode, Added, LastSynced)
	if err := m.Save(repoPath); err != nil {
		return fmt.Errorf("failed to save migrated manifest: %w", err)
	}

	return nil
}

func (m *Manifest) hydrateOverrideBreadcrumbs(repoPath string) error {
	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		return err
	}

	for _, src := range m.Sources {
		if src == nil {
			continue
		}

		state := metadata.Get(src.Name)
		if state == nil {
			continue
		}

		src.OverrideOriginalURL = state.OverrideOriginalURL
		src.OverrideOriginalRef = state.OverrideOriginalRef
		src.OverrideOriginalSubpath = state.OverrideOriginalSubpath
	}

	return nil
}

func (m *Manifest) persistOverrideBreadcrumbs(repoPath string) error {
	metadata, err := sourcemetadata.Load(repoPath)
	if err != nil {
		return err
	}

	sourceNames := make(map[string]struct{}, len(m.Sources))
	for _, src := range m.Sources {
		if src == nil || src.Name == "" {
			continue
		}

		sourceNames[src.Name] = struct{}{}
		state := metadata.Get(src.Name)
		if state == nil {
			state = &sourcemetadata.SourceState{}
			metadata.Sources[src.Name] = state
		}

		state.OverrideOriginalURL = src.OverrideOriginalURL
		state.OverrideOriginalRef = src.OverrideOriginalRef
		state.OverrideOriginalSubpath = src.OverrideOriginalSubpath
	}

	for sourceName, state := range metadata.Sources {
		if _, exists := sourceNames[sourceName]; exists {
			continue
		}
		if state == nil {
			continue
		}

		state.OverrideOriginalURL = ""
		state.OverrideOriginalRef = ""
		state.OverrideOriginalSubpath = ""
	}

	return metadata.Save(repoPath)
}
