package repomanifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSourceID_URLNormalization(t *testing.T) {
	tests := []struct {
		name     string
		sourceA  *Source
		sourceB  *Source
		wantSame bool
	}{
		{
			name:     "same URL with and without .git suffix",
			sourceA:  &Source{Name: "a", URL: "https://github.com/user/repo"},
			sourceB:  &Source{Name: "b", URL: "https://github.com/user/repo.git"},
			wantSame: true,
		},
		{
			name:     "same URL with different casing",
			sourceA:  &Source{Name: "a", URL: "https://GitHub.com/User/Repo"},
			sourceB:  &Source{Name: "b", URL: "https://github.com/user/repo"},
			wantSame: true,
		},
		{
			name:     "same URL with and without trailing slash",
			sourceA:  &Source{Name: "a", URL: "https://github.com/user/repo"},
			sourceB:  &Source{Name: "b", URL: "https://github.com/user/repo/"},
			wantSame: true,
		},
		{
			name:     "all normalizations combined",
			sourceA:  &Source{Name: "a", URL: "https://GitHub.com/User/Repo.git"},
			sourceB:  &Source{Name: "b", URL: "https://github.com/user/repo/"},
			wantSame: true,
		},
		{
			name:     "dot git slash normalizes to same ID",
			sourceA:  &Source{Name: "a", URL: "https://github.com/user/repo.git/"},
			sourceB:  &Source{Name: "b", URL: "https://github.com/user/repo"},
			wantSame: true,
		},
		{
			name:     "slash dot git normalizes to same ID",
			sourceA:  &Source{Name: "a", URL: "https://github.com/user/repo/.git"},
			sourceB:  &Source{Name: "b", URL: "https://github.com/user/repo"},
			wantSame: true,
		},
		{
			name:     "multiple trailing slashes normalize to same ID",
			sourceA:  &Source{Name: "a", URL: "https://github.com/user/repo///"},
			sourceB:  &Source{Name: "b", URL: "https://github.com/user/repo"},
			wantSame: true,
		},
		{
			name:     "surrounding whitespace normalizes to same ID",
			sourceA:  &Source{Name: "a", URL: "  https://github.com/user/repo  "},
			sourceB:  &Source{Name: "b", URL: "https://github.com/user/repo"},
			wantSame: true,
		},
		{
			name:     "different URLs produce different IDs",
			sourceA:  &Source{Name: "a", URL: "https://github.com/user/repo-a"},
			sourceB:  &Source{Name: "b", URL: "https://github.com/user/repo-b"},
			wantSame: false,
		},
		{
			name:     "different hosts produce different IDs",
			sourceA:  &Source{Name: "a", URL: "https://github.com/user/repo"},
			sourceB:  &Source{Name: "b", URL: "https://gitlab.com/user/repo"},
			wantSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idA := GenerateSourceID(tt.sourceA)
			idB := GenerateSourceID(tt.sourceB)

			if idA == "" {
				t.Fatal("GenerateSourceID returned empty string for sourceA")
			}
			if idB == "" {
				t.Fatal("GenerateSourceID returned empty string for sourceB")
			}

			if tt.wantSame && idA != idB {
				t.Errorf("expected same ID, got %q and %q", idA, idB)
			}
			if !tt.wantSame && idA == idB {
				t.Errorf("expected different IDs, got same: %q", idA)
			}
		})
	}
}

func TestGenerateSourceID_CompatibilityWithLegacyUntrimmedURLSourceID(t *testing.T) {
	legacyPersisted := GenerateSourceID(&Source{Name: "legacy", URL: "https://github.com/user/repo.git/"})
	currentCanonical := GenerateSourceID(&Source{Name: "current", URL: "https://github.com/user/repo"})

	if legacyPersisted != currentCanonical {
		t.Fatalf("expected legacy persisted URL source identity to remain stable, got %q vs %q", legacyPersisted, currentCanonical)
	}
}

func TestGenerateSourceID_PathSources(t *testing.T) {
	t.Run("absolute path produces stable ID", func(t *testing.T) {
		source := &Source{Name: "test", Path: "/home/user/my-resources"}
		id1 := GenerateSourceID(source)
		id2 := GenerateSourceID(source)

		if id1 == "" {
			t.Fatal("GenerateSourceID returned empty string")
		}
		if id1 != id2 {
			t.Errorf("expected stable ID, got %q and %q", id1, id2)
		}
	})

	t.Run("same relative path resolved to same absolute produces same ID", func(t *testing.T) {
		// Create a real temp directory to use as the relative path target
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "resources")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("failed to create test dir: %v", err)
		}

		// Save and restore working directory
		origDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}
		t.Cleanup(func() {
			os.Chdir(origDir) //nolint:errcheck
		})

		// Change to tmpDir so "resources" resolves to subDir
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to chdir: %v", err)
		}

		sourceRelative := &Source{Name: "rel", Path: "resources"}
		sourceAbsolute := &Source{Name: "abs", Path: subDir}

		idRel := GenerateSourceID(sourceRelative)
		idAbs := GenerateSourceID(sourceAbsolute)

		if idRel != idAbs {
			t.Errorf("relative and absolute paths should produce same ID, got %q and %q", idRel, idAbs)
		}
	})

	t.Run("different paths produce different IDs", func(t *testing.T) {
		sourceA := &Source{Name: "a", Path: "/path/to/resources-a"}
		sourceB := &Source{Name: "b", Path: "/path/to/resources-b"}

		idA := GenerateSourceID(sourceA)
		idB := GenerateSourceID(sourceB)

		if idA == idB {
			t.Errorf("expected different IDs for different paths, got same: %q", idA)
		}
	})
}

func TestGenerateSourceID_Format(t *testing.T) {
	tests := []struct {
		name   string
		source *Source
	}{
		{
			name:   "URL source",
			source: &Source{Name: "test", URL: "https://github.com/user/repo"},
		},
		{
			name:   "path source",
			source: &Source{Name: "test", Path: "/home/user/resources"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateSourceID(tt.source)

			// Check format: "src-" + 12 hex chars = 16 chars total
			if len(id) != 16 {
				t.Errorf("expected ID length 16, got %d: %q", len(id), id)
			}

			if id[:4] != "src-" {
				t.Errorf("expected ID to start with 'src-', got %q", id)
			}

			// Verify remaining chars are valid hex
			hexPart := id[4:]
			for _, c := range hexPart {
				if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
					t.Errorf("expected hex characters after 'src-', got %q in %q", string(c), id)
					break
				}
			}
		})
	}
}

func TestGenerateSourceID_EmptySource(t *testing.T) {
	t.Run("nil source", func(t *testing.T) {
		id := GenerateSourceID(nil)
		if id != "" {
			t.Errorf("expected empty string for nil source, got %q", id)
		}
	})

	t.Run("source with no URL or path", func(t *testing.T) {
		source := &Source{Name: "test"}
		id := GenerateSourceID(source)
		if id != "" {
			t.Errorf("expected empty string for empty source, got %q", id)
		}
	})
}

func TestGenerateSourceID_URLTakesPrecedence(t *testing.T) {
	// When a source has both URL and Path (shouldn't happen normally),
	// URL should take precedence
	sourceWithBoth := &Source{
		Name: "both",
		URL:  "https://github.com/user/repo",
		Path: "/local/path",
	}
	sourceURLOnly := &Source{
		Name: "url-only",
		URL:  "https://github.com/user/repo",
	}

	idBoth := GenerateSourceID(sourceWithBoth)
	idURLOnly := GenerateSourceID(sourceURLOnly)

	if idBoth != idURLOnly {
		t.Errorf("URL should take precedence when both set: got %q (both) vs %q (url-only)", idBoth, idURLOnly)
	}
}

func TestGenerateSourceID_Deterministic(t *testing.T) {
	source := &Source{
		Name: "test",
		URL:  "https://github.com/user/repo",
	}

	// Generate multiple times and verify consistency
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateSourceID(source)
		ids[id] = true
	}

	if len(ids) != 1 {
		t.Errorf("expected exactly 1 unique ID across 100 calls, got %d", len(ids))
	}
}
