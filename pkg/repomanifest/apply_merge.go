package repomanifest

import "fmt"

// IncludeMergeMode controls how include filters are handled when an incoming
// source matches an existing source by name and canonical location.
type IncludeMergeMode string

const (
	// IncludeMergeReplace replaces existing include filters with incoming values.
	IncludeMergeReplace IncludeMergeMode = "replace"
	// IncludeMergePreserve keeps existing include filters unchanged.
	IncludeMergePreserve IncludeMergeMode = "preserve"
)

// ApplyAction describes the merge result for one source.
type ApplyAction string

const (
	ApplyActionAdd      ApplyAction = "add"
	ApplyActionUpdate   ApplyAction = "update"
	ApplyActionNoOp     ApplyAction = "noop"
	ApplyActionConflict ApplyAction = "conflict"
)

// ApplyMergeOptions configures merge behavior for repo apply-manifest.
type ApplyMergeOptions struct {
	IncludeMode IncludeMergeMode
}

// ApplyChange reports what happened for one source during merge planning.
type ApplyChange struct {
	Name    string
	Action  ApplyAction
	Message string
}

// ApplyMergeReport summarizes merge outcomes across all incoming sources.
type ApplyMergeReport struct {
	Changes []ApplyChange
}

// Added returns how many sources are new additions.
func (r *ApplyMergeReport) Added() int {
	return r.countByAction(ApplyActionAdd)
}

// Updated returns how many existing sources were updated.
func (r *ApplyMergeReport) Updated() int {
	return r.countByAction(ApplyActionUpdate)
}

// NoOp returns how many sources produced no changes.
func (r *ApplyMergeReport) NoOp() int {
	return r.countByAction(ApplyActionNoOp)
}

// Conflicts returns how many conflicting sources were detected.
func (r *ApplyMergeReport) Conflicts() int {
	return r.countByAction(ApplyActionConflict)
}

func (r *ApplyMergeReport) countByAction(action ApplyAction) int {
	if r == nil {
		return 0
	}

	count := 0
	for _, change := range r.Changes {
		if change.Action == action {
			count++
		}
	}

	return count
}

// HasConflicts reports whether at least one conflict exists.
func (r *ApplyMergeReport) HasConflicts() bool {
	return r.Conflicts() > 0
}

// MergeForApply merges incoming sources into a copy of current manifest using
// repo apply-manifest semantics.
func MergeForApply(current, incoming *Manifest, opts ApplyMergeOptions) (*Manifest, *ApplyMergeReport, error) {
	if current == nil {
		return nil, nil, fmt.Errorf("current manifest is nil")
	}
	if incoming == nil {
		return nil, nil, fmt.Errorf("incoming manifest is nil")
	}
	if err := current.Validate(); err != nil {
		return nil, nil, fmt.Errorf("current manifest invalid: %w", err)
	}
	if err := incoming.Validate(); err != nil {
		return nil, nil, fmt.Errorf("incoming manifest invalid: %w", err)
	}

	mode := opts.IncludeMode
	if mode == "" {
		mode = IncludeMergeReplace
	}
	if mode != IncludeMergeReplace && mode != IncludeMergePreserve {
		return nil, nil, fmt.Errorf("invalid include merge mode %q", mode)
	}

	merged := cloneManifest(current)
	report := &ApplyMergeReport{Changes: make([]ApplyChange, 0, len(incoming.Sources))}
	byName := make(map[string]*Source, len(merged.Sources))
	byCanonicalID := make(map[string]*Source, len(merged.Sources))

	for _, src := range merged.Sources {
		if src == nil {
			continue
		}

		byName[src.Name] = src

		canonicalID := canonicalSourceIdentity(src)
		if canonicalID == "" {
			continue
		}

		if existing, ok := byCanonicalID[canonicalID]; ok && existing.Name != src.Name {
			return nil, nil, fmt.Errorf("current manifest invalid: canonical source collision between %q and %q (%s)", existing.Name, src.Name, describeSourceLocation(src))
		}

		byCanonicalID[canonicalID] = src
	}

	for _, in := range incoming.Sources {
		incomingCanonicalID := canonicalSourceIdentity(in)

		existingByName, foundByName := byName[in.Name]
		if !foundByName {
			if existingByCanonicalID, exists := byCanonicalID[incomingCanonicalID]; exists {
				report.Changes = append(report.Changes, ApplyChange{
					Name:   in.Name,
					Action: ApplyActionConflict,
					Message: fmt.Sprintf(
						"source %q has same canonical location as existing source %q (%s); use one canonical source name for this upstream",
						in.Name,
						existingByCanonicalID.Name,
						describeSourceLocation(existingByCanonicalID),
					),
				})
				continue
			}

			merged.Sources = append(merged.Sources, cloneSource(in))
			added := merged.Sources[len(merged.Sources)-1]
			byName[added.Name] = added
			if canonicalID := canonicalSourceIdentity(added); canonicalID != "" {
				byCanonicalID[canonicalID] = added
			}
			report.Changes = append(report.Changes, ApplyChange{
				Name:    in.Name,
				Action:  ApplyActionAdd,
				Message: "added new source",
			})
			continue
		}

		if canonicalSourceIdentity(existingByName) != incomingCanonicalID {
			report.Changes = append(report.Changes, ApplyChange{
				Name:   in.Name,
				Action: ApplyActionConflict,
				Message: fmt.Sprintf(
					"source %q already exists at %s but incoming manifest uses %s; keep source names stable for canonical locations",
					in.Name,
					describeSourceLocation(existingByName),
					describeSourceLocation(in),
				),
			})
			continue
		}

		refUpdated := false
		if existingByName.Ref != in.Ref {
			existingByName.Ref = in.Ref
			refUpdated = true
		}

		if equalStringSlices(existingByName.Include, in.Include) {
			if refUpdated {
				report.Changes = append(report.Changes, ApplyChange{
					Name:    in.Name,
					Action:  ApplyActionUpdate,
					Message: "updated source ref",
				})
				continue
			}

			report.Changes = append(report.Changes, ApplyChange{
				Name:    in.Name,
				Action:  ApplyActionNoOp,
				Message: "identical source already configured",
			})
			continue
		}

		if mode == IncludeMergePreserve {
			if refUpdated {
				report.Changes = append(report.Changes, ApplyChange{
					Name:    in.Name,
					Action:  ApplyActionUpdate,
					Message: "updated source ref; kept existing include filters (preserve mode)",
				})
				continue
			}

			report.Changes = append(report.Changes, ApplyChange{
				Name:    in.Name,
				Action:  ApplyActionNoOp,
				Message: "kept existing include filters (preserve mode)",
			})
			continue
		}

		existingByName.Include = copyStringSlice(in.Include)
		if refUpdated {
			report.Changes = append(report.Changes, ApplyChange{
				Name:    in.Name,
				Action:  ApplyActionUpdate,
				Message: "updated source ref and replaced include filters",
			})
			continue
		}

		report.Changes = append(report.Changes, ApplyChange{
			Name:    in.Name,
			Action:  ApplyActionUpdate,
			Message: "replaced include filters",
		})
	}

	if err := merged.Validate(); err != nil {
		return nil, nil, fmt.Errorf("merged manifest invalid: %w", err)
	}

	return merged, report, nil
}

func canonicalSourceIdentity(source *Source) string {
	if source == nil {
		return ""
	}

	if source.ID != "" {
		return source.ID
	}

	return GenerateSourceID(source)
}

func describeSourceLocation(source *Source) string {
	if source == nil {
		return "unknown location"
	}

	if source.URL != "" {
		if source.Ref != "" {
			if source.Subpath != "" {
				return fmt.Sprintf("url %q (ref %q, subpath %q)", source.URL, source.Ref, source.Subpath)
			}
			return fmt.Sprintf("url %q (ref %q)", source.URL, source.Ref)
		}
		if source.Subpath != "" {
			return fmt.Sprintf("url %q (subpath %q)", source.URL, source.Subpath)
		}
		return fmt.Sprintf("url %q", source.URL)
	}

	if source.Path != "" {
		return fmt.Sprintf("path %q", source.Path)
	}

	return "unknown location"
}

func cloneManifest(m *Manifest) *Manifest {
	if m == nil {
		return nil
	}

	cloned := &Manifest{Version: m.Version, Sources: make([]*Source, 0, len(m.Sources))}
	for _, src := range m.Sources {
		cloned.Sources = append(cloned.Sources, cloneSource(src))
	}

	return cloned
}

func cloneSource(s *Source) *Source {
	if s == nil {
		return nil
	}

	return &Source{
		ID:      s.ID,
		Name:    s.Name,
		Path:    s.Path,
		URL:     s.URL,
		Ref:     s.Ref,
		Subpath: s.Subpath,
		Include: copyStringSlice(s.Include),
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func copyStringSlice(in []string) []string {
	if in == nil {
		return nil
	}

	out := make([]string, len(in))
	copy(out, in)
	return out
}
