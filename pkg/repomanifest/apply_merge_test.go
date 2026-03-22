package repomanifest

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeForApply_AddAndIdempotent(t *testing.T) {
	current := &Manifest{Version: 1, Sources: []*Source{}}
	incoming := &Manifest{Version: 1, Sources: []*Source{{
		Name:    "team-tools",
		URL:     "https://github.com/example/tools",
		Include: []string{"skill/*"},
	}}}

	merged, report, err := MergeForApply(current, incoming, ApplyMergeOptions{})
	if err != nil {
		t.Fatalf("MergeForApply() error = %v", err)
	}
	if len(merged.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(merged.Sources))
	}
	if report.Added() != 1 || report.NoOp() != 0 || report.Conflicts() != 0 || report.Updated() != 0 {
		t.Fatalf("unexpected report counts: add=%d update=%d noop=%d conflict=%d", report.Added(), report.Updated(), report.NoOp(), report.Conflicts())
	}

	mergedAgain, reportAgain, err := MergeForApply(merged, incoming, ApplyMergeOptions{})
	if err != nil {
		t.Fatalf("MergeForApply() second run error = %v", err)
	}
	if len(mergedAgain.Sources) != 1 {
		t.Fatalf("expected 1 source after re-apply, got %d", len(mergedAgain.Sources))
	}
	if reportAgain.NoOp() != 1 || reportAgain.Added() != 0 || reportAgain.Updated() != 0 || reportAgain.Conflicts() != 0 {
		t.Fatalf("unexpected re-apply counts: add=%d update=%d noop=%d conflict=%d", reportAgain.Added(), reportAgain.Updated(), reportAgain.NoOp(), reportAgain.Conflicts())
	}
}

func TestMergeForApply_ConflictOnDifferentLocation(t *testing.T) {
	current := &Manifest{Version: 1, Sources: []*Source{{
		Name: "team-tools",
		URL:  "https://github.com/example/tools",
	}}}
	incoming := &Manifest{Version: 1, Sources: []*Source{{
		Name: "team-tools",
		URL:  "https://github.com/example/other-tools",
	}}}

	merged, report, err := MergeForApply(current, incoming, ApplyMergeOptions{})
	if err != nil {
		t.Fatalf("MergeForApply() error = %v", err)
	}
	if !report.HasConflicts() || report.Conflicts() != 1 {
		t.Fatalf("expected 1 conflict, got %d", report.Conflicts())
	}
	if len(merged.Sources) != 1 || merged.Sources[0].URL != "https://github.com/example/tools" {
		t.Fatalf("conflict must not overwrite existing source")
	}
}

func TestMergeForApply_IncludeReplaceAndPreserve(t *testing.T) {
	current := &Manifest{Version: 1, Sources: []*Source{{
		Name:    "team-tools",
		URL:     "https://github.com/example/tools",
		Include: []string{"skill/pdf*"},
	}}}
	incoming := &Manifest{Version: 1, Sources: []*Source{{
		Name:    "team-tools",
		URL:     "https://github.com/example/tools",
		Include: []string{"command/lint-*"},
	}}}

	mergedReplace, reportReplace, err := MergeForApply(current, incoming, ApplyMergeOptions{IncludeMode: IncludeMergeReplace})
	if err != nil {
		t.Fatalf("replace merge error = %v", err)
	}
	if reportReplace.Updated() != 1 {
		t.Fatalf("expected 1 update in replace mode, got %d", reportReplace.Updated())
	}
	if got := strings.Join(mergedReplace.Sources[0].Include, ","); got != "command/lint-*" {
		t.Fatalf("replace mode include mismatch: %s", got)
	}

	mergedPreserve, reportPreserve, err := MergeForApply(current, incoming, ApplyMergeOptions{IncludeMode: IncludeMergePreserve})
	if err != nil {
		t.Fatalf("preserve merge error = %v", err)
	}
	if reportPreserve.NoOp() != 1 || reportPreserve.Updated() != 0 {
		t.Fatalf("expected noop in preserve mode, got noop=%d update=%d", reportPreserve.NoOp(), reportPreserve.Updated())
	}
	if got := strings.Join(mergedPreserve.Sources[0].Include, ","); got != "skill/pdf*" {
		t.Fatalf("preserve mode include mismatch: %s", got)
	}
}

func TestMergeForApply_UpdatesRefWhenCanonicalSourceMatches(t *testing.T) {
	current := &Manifest{Version: 1, Sources: []*Source{{
		Name:    "team-tools",
		URL:     "https://github.com/example/tools",
		Ref:     "v1.2.0",
		Include: []string{"skill/pdf*"},
	}}}
	incoming := &Manifest{Version: 1, Sources: []*Source{{
		Name:    "team-tools",
		URL:     "https://github.com/example/tools",
		Ref:     "v1.3.0",
		Include: []string{"skill/pdf*"},
	}}}

	merged, report, err := MergeForApply(current, incoming, ApplyMergeOptions{})
	if err != nil {
		t.Fatalf("MergeForApply() error = %v", err)
	}
	if got := merged.Sources[0].Ref; got != "v1.3.0" {
		t.Fatalf("expected ref to be updated to v1.3.0, got %q", got)
	}
	if report.Updated() != 1 || report.NoOp() != 0 || report.Added() != 0 || report.Conflicts() != 0 {
		t.Fatalf("unexpected report counts: add=%d update=%d noop=%d conflict=%d", report.Added(), report.Updated(), report.NoOp(), report.Conflicts())
	}
	if !strings.Contains(report.Changes[0].Message, "updated source ref") {
		t.Fatalf("expected update message to mention ref update, got %q", report.Changes[0].Message)
	}
}

func TestMergeForApply_UpdatesRefAndPreservesIncludeInPreserveMode(t *testing.T) {
	current := &Manifest{Version: 1, Sources: []*Source{{
		Name:    "team-tools",
		URL:     "https://github.com/example/tools",
		Ref:     "v1.2.0",
		Include: []string{"skill/pdf*"},
	}}}
	incoming := &Manifest{Version: 1, Sources: []*Source{{
		Name:    "team-tools",
		URL:     "https://github.com/example/tools",
		Ref:     "v1.3.0",
		Include: []string{"command/lint-*"},
	}}}

	merged, report, err := MergeForApply(current, incoming, ApplyMergeOptions{IncludeMode: IncludeMergePreserve})
	if err != nil {
		t.Fatalf("MergeForApply() error = %v", err)
	}
	if got := merged.Sources[0].Ref; got != "v1.3.0" {
		t.Fatalf("expected ref to be updated to v1.3.0, got %q", got)
	}
	if got := strings.Join(merged.Sources[0].Include, ","); got != "skill/pdf*" {
		t.Fatalf("expected preserve mode to keep include filters, got %s", got)
	}
	if report.Updated() != 1 || report.NoOp() != 0 {
		t.Fatalf("expected one update and no noops, got update=%d noop=%d", report.Updated(), report.NoOp())
	}
	if !strings.Contains(report.Changes[0].Message, "updated source ref") {
		t.Fatalf("expected update message to mention ref update, got %q", report.Changes[0].Message)
	}
}

func TestMergeForApply_InvalidIncludeMode(t *testing.T) {
	current := &Manifest{Version: 1, Sources: []*Source{}}
	incoming := &Manifest{Version: 1, Sources: []*Source{}}

	_, _, err := MergeForApply(current, incoming, ApplyMergeOptions{IncludeMode: IncludeMergeMode("invalid")})
	if err == nil {
		t.Fatal("expected error for invalid include mode")
	}
	if !strings.Contains(err.Error(), "invalid include merge mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeForApply_ConflictOnSameCanonicalURLDifferentName(t *testing.T) {
	current := &Manifest{Version: 1, Sources: []*Source{{
		Name: "team-tools",
		URL:  "https://github.com/example/tools.git/",
	}}}
	incoming := &Manifest{Version: 1, Sources: []*Source{{
		Name: "platform-tools",
		URL:  "https://github.com/EXAMPLE/tools",
	}}}

	merged, report, err := MergeForApply(current, incoming, ApplyMergeOptions{})
	if err != nil {
		t.Fatalf("MergeForApply() error = %v", err)
	}
	if report.Conflicts() != 1 {
		t.Fatalf("expected 1 conflict, got %d", report.Conflicts())
	}
	if !strings.Contains(report.Changes[0].Message, "same canonical location") {
		t.Fatalf("expected canonical location conflict message, got %q", report.Changes[0].Message)
	}
	if len(merged.Sources) != 1 || merged.Sources[0].Name != "team-tools" {
		t.Fatalf("expected existing source to be preserved, got %+v", merged.Sources)
	}
}

func TestMergeForApply_ConflictOnSameCanonicalPathDifferentName(t *testing.T) {
	baseDir := t.TempDir()
	current := &Manifest{Version: 1, Sources: []*Source{{
		Name: "team-local",
		Path: baseDir,
	}}}
	incoming := &Manifest{Version: 1, Sources: []*Source{{
		Name: "platform-local",
		Path: filepath.Join(baseDir, "."),
	}}}

	merged, report, err := MergeForApply(current, incoming, ApplyMergeOptions{})
	if err != nil {
		t.Fatalf("MergeForApply() error = %v", err)
	}
	if report.Conflicts() != 1 {
		t.Fatalf("expected 1 conflict, got %d", report.Conflicts())
	}
	if !strings.Contains(report.Changes[0].Message, "same canonical location") {
		t.Fatalf("expected canonical location conflict message, got %q", report.Changes[0].Message)
	}
	if len(merged.Sources) != 1 || merged.Sources[0].Name != "team-local" {
		t.Fatalf("expected existing source to be preserved, got %+v", merged.Sources)
	}
}
