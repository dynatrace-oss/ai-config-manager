package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/giturl"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/workspace"
)

func TestFindUnreferencedCaches_NoWorkspace(t *testing.T) {
	// Create temp directory for test repo
	tempDir := t.TempDir()

	// Create repo manager
	manager := repo.NewManagerWithPath(tempDir)

	// Create workspace manager
	workspaceManager, err := workspace.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	// Find unreferenced caches (should return empty list, not error)
	unreferenced, err := findUnreferencedCaches(manager, workspaceManager)
	if err != nil {
		t.Fatalf("findUnreferencedCaches failed: %v", err)
	}

	if len(unreferenced) != 0 {
		t.Errorf("Expected 0 unreferenced caches, got %d", len(unreferenced))
	}
}

func TestFindUnreferencedCaches_NoCaches(t *testing.T) {
	// Create temp directory for test repo
	tempDir := t.TempDir()

	// Create workspace directory (but empty)
	workspaceDir := filepath.Join(tempDir, ".workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create repo manager
	manager := repo.NewManagerWithPath(tempDir)

	// Create workspace manager
	workspaceManager, err := workspace.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	// Find unreferenced caches
	unreferenced, err := findUnreferencedCaches(manager, workspaceManager)
	if err != nil {
		t.Fatalf("findUnreferencedCaches failed: %v", err)
	}

	if len(unreferenced) != 0 {
		t.Errorf("Expected 0 unreferenced caches, got %d", len(unreferenced))
	}
}

func TestFindUnreferencedCaches_AllReferenced(t *testing.T) {
	// Create temp directory for test repo
	tempDir := t.TempDir()

	// Create metadata directory
	metadataDir := filepath.Join(tempDir, ".metadata", "commands")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workspace directory
	workspaceDir := filepath.Join(tempDir, ".workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// URL to cache
	testURL := "https://github.com/test/repo"

	// Create a fake cached repo
	hash := workspace.ComputeHash(testURL)
	cachePath := filepath.Join(workspaceDir, hash)
	if err := os.MkdirAll(filepath.Join(cachePath, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create workspace metadata pointing to this URL
	meta := &metadata.ResourceMetadata{
		Name:           "test-command",
		Type:           resource.Command,
		SourceType:     "github",
		SourceURL:      testURL,
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta, tempDir, "test-source"); err != nil {
		t.Fatal(err)
	}

	// Create workspace manager and add cache metadata
	workspaceManager, err := workspace.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	// Manually create cache metadata so ListCached works
	metadataPath := filepath.Join(workspaceDir, ".cache-metadata.json")
	metadataJSON := `{"version":"1.0","caches":{"` + hash + `":{"url":"` + normalizeURL(testURL) + `","last_accessed":"2026-01-01T00:00:00Z","last_updated":"2026-01-01T00:00:00Z","ref":"main"}}}`
	if err := os.WriteFile(metadataPath, []byte(metadataJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Create repo manager
	manager := repo.NewManagerWithPath(tempDir)

	// Find unreferenced caches (should be empty - cache is referenced)
	unreferenced, err := findUnreferencedCaches(manager, workspaceManager)
	if err != nil {
		t.Fatalf("findUnreferencedCaches failed: %v", err)
	}

	if len(unreferenced) != 0 {
		t.Errorf("Expected 0 unreferenced caches (cache is referenced), got %d", len(unreferenced))
	}
}

func TestFindUnreferencedCaches_WithUnreferenced(t *testing.T) {
	// Create temp directory for test repo
	tempDir := t.TempDir()

	// Create metadata directory
	metadataDir := filepath.Join(tempDir, ".metadata", "commands")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workspace directory
	workspaceDir := filepath.Join(tempDir, ".workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create two cached repos
	referencedURL := "https://github.com/test/referenced"
	unreferencedURL := "https://github.com/test/unreferenced"

	referencedHash := workspace.ComputeHash(referencedURL)
	unreferencedHash := workspace.ComputeHash(unreferencedURL)

	// Create fake cached repos
	referencedPath := filepath.Join(workspaceDir, referencedHash)
	unreferencedPath := filepath.Join(workspaceDir, unreferencedHash)

	if err := os.MkdirAll(filepath.Join(referencedPath, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(unreferencedPath, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Add some content to unreferenced cache for size testing
	if err := os.WriteFile(filepath.Join(unreferencedPath, "README.md"), []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create metadata only for the referenced repo
	meta := &metadata.ResourceMetadata{
		Name:           "test-command",
		Type:           resource.Command,
		SourceType:     "github",
		SourceURL:      referencedURL,
		FirstInstalled: time.Now(),
		LastUpdated:    time.Now(),
	}
	if err := metadata.Save(meta, tempDir, "test-source"); err != nil {
		t.Fatal(err)
	}

	// Create workspace manager and add cache metadata for both repos
	workspaceManager, err := workspace.NewManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create cache metadata
	metadataPath := filepath.Join(workspaceDir, ".cache-metadata.json")
	metadataJSON := `{"version":"1.0","caches":{
		"` + referencedHash + `":{"url":"` + normalizeURL(referencedURL) + `","last_accessed":"2026-01-01T00:00:00Z","last_updated":"2026-01-01T00:00:00Z","ref":"main"},
		"` + unreferencedHash + `":{"url":"` + normalizeURL(unreferencedURL) + `","last_accessed":"2026-01-01T00:00:00Z","last_updated":"2026-01-01T00:00:00Z","ref":"main"}
	}}`
	if err := os.WriteFile(metadataPath, []byte(metadataJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Create repo manager
	manager := repo.NewManagerWithPath(tempDir)

	// Find unreferenced caches
	unreferenced, err := findUnreferencedCaches(manager, workspaceManager)
	if err != nil {
		t.Fatalf("findUnreferencedCaches failed: %v", err)
	}

	// Should find exactly one unreferenced cache
	if len(unreferenced) != 1 {
		t.Fatalf("Expected 1 unreferenced cache, got %d", len(unreferenced))
	}

	// Verify it's the correct one
	if unreferenced[0].URL != normalizeURL(unreferencedURL) {
		t.Errorf("Expected unreferenced URL %s, got %s", normalizeURL(unreferencedURL), unreferenced[0].URL)
	}

	// Verify size is calculated
	if unreferenced[0].Size == 0 {
		t.Error("Expected non-zero size for unreferenced cache")
	}

	// Verify path is correct
	expectedPath := filepath.Join(workspaceDir, unreferencedHash)
	if unreferenced[0].Path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, unreferenced[0].Path)
	}
}

func TestCollectReferencedGitURLs(t *testing.T) {
	// Create temp directory for test repo
	tempDir := t.TempDir()

	// Create metadata directories
	commandsDir := filepath.Join(tempDir, ".metadata", "commands")
	skillsDir := filepath.Join(tempDir, ".metadata", "skills")
	agentsDir := filepath.Join(tempDir, ".metadata", "agents")

	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create metadata with various source types
	metas := []metadata.ResourceMetadata{
		{
			Name:           "git-command",
			Type:           resource.Command,
			SourceType:     "github",
			SourceURL:      "https://github.com/test/repo1",
			FirstInstalled: time.Now(),
			LastUpdated:    time.Now(),
		},
		{
			Name:           "local-command",
			Type:           resource.Command,
			SourceType:     "local",
			SourceURL:      "/path/to/local",
			FirstInstalled: time.Now(),
			LastUpdated:    time.Now(),
		},
		{
			Name:           "git-skill",
			Type:           resource.Skill,
			SourceType:     "git-url",
			SourceURL:      "https://example.com/repo.git",
			FirstInstalled: time.Now(),
			LastUpdated:    time.Now(),
		},
		{
			Name:           "git-agent",
			Type:           resource.Agent,
			SourceType:     "gitlab",
			SourceURL:      "https://gitlab.com/test/repo",
			FirstInstalled: time.Now(),
			LastUpdated:    time.Now(),
		},
	}

	for _, meta := range metas {
		m := meta // Create copy for pointer
		if err := metadata.Save(&m, tempDir, "test-source"); err != nil {
			t.Fatal(err)
		}
	}

	// Create repo manager
	manager := repo.NewManagerWithPath(tempDir)

	// Collect referenced Git URLs
	urls, err := collectReferencedGitURLs(manager)
	if err != nil {
		t.Fatalf("collectReferencedGitURLs failed: %v", err)
	}

	// Should find 3 Git URLs (not the local one)
	if len(urls) != 3 {
		t.Fatalf("Expected 3 Git URLs, got %d: %v", len(urls), urls)
	}

	// Verify URLs are correct
	expectedURLs := map[string]bool{
		"https://github.com/test/repo1": false,
		"https://example.com/repo.git":  false,
		"https://gitlab.com/test/repo":  false,
	}

	for _, url := range urls {
		if _, exists := expectedURLs[url]; exists {
			expectedURLs[url] = true
		} else {
			t.Errorf("Unexpected URL found: %s", url)
		}
	}

	for url, found := range expectedURLs {
		if !found {
			t.Errorf("Expected URL not found: %s", url)
		}
	}
}

func TestIsGitSource(t *testing.T) {
	tests := []struct {
		sourceType string
		expected   bool
	}{
		{"github", true},
		{"git-url", true},
		{"gitlab", true},
		{"local", false},
		{"file", false},
		{"", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.sourceType, func(t *testing.T) {
			result := isGitSource(tt.sourceType)
			if result != tt.expected {
				t.Errorf("isGitSource(%q) = %v, expected %v", tt.sourceType, result, tt.expected)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://github.com/test/repo", "https://github.com/test/repo"},
		{"HTTPS://GitHub.com/Test/Repo", "https://github.com/test/repo"},
		{"https://github.com/test/repo.git", "https://github.com/test/repo"},
		{"https://github.com/test/repo/", "https://github.com/test/repo"},
		{"https://github.com/test/repo.git/", "https://github.com/test/repo"},
		{"https://github.com/test/repo/.git", "https://github.com/test/repo"},
		{"https://github.com/test/repo///", "https://github.com/test/repo"},
		{"  https://github.com/test/repo  ", "https://github.com/test/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeURL(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeURL_ConsistencyWithCanonicalHelper(t *testing.T) {
	inputs := []string{
		"https://github.com/test/repo",
		"HTTPS://GitHub.com/Test/Repo",
		"https://github.com/test/repo/",
		"https://github.com/test/repo.git",
		"https://github.com/test/repo.git/",
		"https://github.com/test/repo/.git",
		"https://github.com/test/repo///",
		"  https://github.com/test/repo  ",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			got := normalizeURL(input)
			want := giturl.NormalizeURL(input)
			if got != want {
				t.Fatalf("normalizeURL(%q) = %q, want canonical %q", input, got, want)
			}
		})
	}
}

func TestURLNormalization_ConsistencyAcrossWorkspacePruneAndSourceID(t *testing.T) {
	input := "  HTTPS://github.com/Test/Repo.git/  "

	normalized := normalizeURL(input)
	workspaceHash := workspace.ComputeHash(input)
	canonicalHash := workspace.ComputeHash(normalized)
	if workspaceHash != canonicalHash {
		t.Fatalf("workspace hash mismatch: %s != %s", workspaceHash, canonicalHash)
	}

	canonicalSourceID := repomanifest.GenerateSourceID(&repomanifest.Source{URL: normalized})
	inputSourceID := repomanifest.GenerateSourceID(&repomanifest.Source{URL: input})
	if canonicalSourceID != inputSourceID {
		t.Fatalf("source ID mismatch for equivalent URLs: %s != %s", canonicalSourceID, inputSourceID)
	}

	if normalized != giturl.NormalizeURL(input) {
		t.Fatalf("prune normalization mismatch with canonical helper")
	}
}

func TestGetDirSize(t *testing.T) {
	// Create temp directory with some files
	tempDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")
	subDir := filepath.Join(tempDir, "subdir")
	file3 := filepath.Join(subDir, "file3.txt")

	if err := os.WriteFile(file1, []byte("12345"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("1234567890"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file3, []byte("123"), 0644); err != nil {
		t.Fatal(err)
	}

	// Calculate size
	size, err := getDirSize(tempDir)
	if err != nil {
		t.Fatalf("getDirSize failed: %v", err)
	}

	// Expected: 5 + 10 + 3 = 18 bytes
	expected := int64(18)
	if size != expected {
		t.Errorf("Expected size %d, got %d", expected, size)
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatSize(%d) = %q, expected %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		singular string
		plural   string
		count    int
		expected string
	}{
		{"repository", "repositories", 0, "repositories"},
		{"repository", "repositories", 1, "repository"},
		{"repository", "repositories", 2, "repositories"},
		{"repository", "repositories", 100, "repositories"},
		{"file", "files", 1, "file"},
		{"file", "files", 5, "files"},
	}

	for _, tt := range tests {
		result := pluralize(tt.singular, tt.plural, tt.count)
		if result != tt.expected {
			t.Errorf("pluralize(%q, %q, %d) = %q, expected %q",
				tt.singular, tt.plural, tt.count, result, tt.expected)
		}
	}
}
