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

	for _, in := range incoming.Sources {
		existing, found := merged.GetSource(in.Name)
		if !found {
			merged.Sources = append(merged.Sources, cloneSource(in))
			report.Changes = append(report.Changes, ApplyChange{
				Name:    in.Name,
				Action:  ApplyActionAdd,
				Message: "added new source",
			})
			continue
		}

		if !sameCanonicalLocation(existing, in) {
			report.Changes = append(report.Changes, ApplyChange{
				Name:    in.Name,
				Action:  ApplyActionConflict,
				Message: "source name already exists with different location",
			})
			continue
		}

		if equalStringSlices(existing.Include, in.Include) {
			report.Changes = append(report.Changes, ApplyChange{
				Name:    in.Name,
				Action:  ApplyActionNoOp,
				Message: "identical source already configured",
			})
			continue
		}

		if mode == IncludeMergePreserve {
			report.Changes = append(report.Changes, ApplyChange{
				Name:    in.Name,
				Action:  ApplyActionNoOp,
				Message: "kept existing include filters (preserve mode)",
			})
			continue
		}

		existing.Include = copyStringSlice(in.Include)
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

func sameCanonicalLocation(a, b *Source) bool {
	if a == nil || b == nil {
		return false
	}

	return a.Path == b.Path && a.URL == b.URL && a.Ref == b.Ref && a.Subpath == b.Subpath
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
