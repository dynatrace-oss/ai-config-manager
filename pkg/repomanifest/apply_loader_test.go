package repomanifest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadForApply_LocalManifest(t *testing.T) {
	manifestDir := filepath.Join(t.TempDir(), "team")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatalf("failed to create manifest directory: %v", err)
	}

	manifestPath := filepath.Join(manifestDir, ManifestFileName)
	content := `version: 1
sources:
  - name: local-source
    path: ./resources
`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	m, err := LoadForApply(manifestPath)
	if err != nil {
		t.Fatalf("LoadForApply() error = %v", err)
	}

	if len(m.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(m.Sources))
	}

	wantPath := filepath.Join(manifestDir, "resources")
	if m.Sources[0].Path != wantPath {
		t.Fatalf("expected resolved path %q, got %q", wantPath, m.Sources[0].Path)
	}
}

func TestLoadForApply_LocalManifest_InvalidYAML(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), ManifestFileName)
	if err := os.WriteFile(manifestPath, []byte("version: [oops"), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	_, err := LoadForApply(manifestPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse manifest YAML") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestLoadForApply_LocalManifest_RequiresExplicitFilename(t *testing.T) {
	otherFile := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := os.WriteFile(otherFile, []byte("version: 1\nsources: []\n"), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	_, err := LoadForApply(otherFile)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must point to ai.repo.yaml") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadForApply_InvalidScheme(t *testing.T) {
	_, err := LoadForApply("ftp://example.com/ai.repo.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "local ai.repo.yaml path, stdin (- or /dev/stdin), or HTTP(S) URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadForApply_StdinDash(t *testing.T) {
	withStdin(t, "version: 1\nsources:\n  - name: stdin-source\n    url: https://github.com/example/tools\n", func() {
		m, err := LoadForApply("-")
		if err != nil {
			t.Fatalf("LoadForApply() error = %v", err)
		}
		if len(m.Sources) != 1 || m.Sources[0].Name != "stdin-source" {
			t.Fatalf("unexpected manifest from stdin: %+v", m.Sources)
		}
	})
}

func TestLoadForApply_StdinDevStdin(t *testing.T) {
	withStdin(t, "version: 1\nsources:\n  - name: stdin-source\n    url: https://github.com/example/tools\n", func() {
		m, err := LoadForApply("/dev/stdin")
		if err != nil {
			t.Fatalf("LoadForApply() error = %v", err)
		}
		if len(m.Sources) != 1 || m.Sources[0].Name != "stdin-source" {
			t.Fatalf("unexpected manifest from /dev/stdin: %+v", m.Sources)
		}
	})
}

func TestLoadForApply_StdinRejectsRelativeSourcePath(t *testing.T) {
	withStdin(t, "version: 1\nsources:\n  - name: stdin-path\n    path: ./resources\n", func() {
		_, err := LoadForApply("-")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "relative path sources are not supported for stdin manifests") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestLoadForApply_RemoteManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/team/ai.repo.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`version: 1
sources:
  - name: remote-source
    url: https://github.com/example/tools
`))
	}))
	defer server.Close()

	m, err := LoadForApply(server.URL + "/team/ai.repo.yaml")
	if err != nil {
		t.Fatalf("LoadForApply() error = %v", err)
	}
	if len(m.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(m.Sources))
	}
	if got := m.Sources[0].URL; got != "https://github.com/example/tools" {
		t.Fatalf("expected URL source, got %q", got)
	}
}

func TestLoadForApply_RemoteManifest_RejectsNonFileURL(t *testing.T) {
	_, err := LoadForApply("https://github.com/org/repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must point directly to ai.repo.yaml") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadForApply_RemoteManifest_InvalidYAML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("version: [bad"))
	}))
	defer server.Close()

	_, err := LoadForApply(server.URL + "/ai.repo.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse manifest YAML") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestLoadForApply_RemoteManifest_RejectsRelativeSourcePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`version: 1
sources:
  - name: team-path
    path: ./resources
`))
	}))
	defer server.Close()

	_, err := LoadForApply(server.URL + "/ai.repo.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "relative path sources are not supported for remote manifests") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func withStdin(t *testing.T, content string, fn func()) {
	t.Helper()

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdin pipe: %v", err)
	}

	if _, err := io.WriteString(w, content); err != nil {
		_ = w.Close()
		_ = r.Close()
		t.Fatalf("failed to write stdin content: %v", err)
	}
	_ = w.Close()
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()

	fn()
}
