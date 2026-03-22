package repomanifest

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var windowsDrivePathPattern = regexp.MustCompile(`^[a-zA-Z]:[\\/]`)

// LoadForApply loads a shareable manifest from either:
//   - a local ai.repo.yaml path
//   - stdin via "-" or "/dev/stdin"
//   - an HTTP(S) URL that points directly to ai.repo.yaml
//
// For local manifests, relative source path values are resolved relative to the
// manifest file directory. For remote and stdin manifests, relative source path
// values are rejected because no safe receiver-local resolution exists.
func LoadForApply(input string) (*Manifest, error) {
	return loadForApplyWithClient(input, http.DefaultClient)
}

func loadForApplyWithClient(input string, client *http.Client) (*Manifest, error) {
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("manifest input cannot be empty")
	}

	parsedURL, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest input: %w", err)
	}

	if parsedURL.Scheme == "http" || parsedURL.Scheme == "https" {
		return loadRemoteForApply(parsedURL, client)
	}

	if isStdinManifestInput(input) {
		return loadStdinForApply()
	}

	if parsedURL.Scheme != "" && !isWindowsDrivePath(input) {
		return nil, fmt.Errorf("manifest input must be a local %s path, stdin (- or /dev/stdin), or HTTP(S) URL", ManifestFileName)
	}

	return loadLocalForApply(input)
}

func isWindowsDrivePath(input string) bool {
	return windowsDrivePathPattern.MatchString(input)
}

func loadLocalForApply(input string) (*Manifest, error) {
	absPath, err := filepath.Abs(input)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve manifest path: %w", err)
	}

	if filepath.Base(absPath) != ManifestFileName {
		return nil, fmt.Errorf("local manifest path must point to %s", ManifestFileName)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read local manifest: %w", err)
	}

	manifest, err := parseManifestYAML(data)
	if err != nil {
		return nil, err
	}

	manifestDir := filepath.Dir(absPath)
	for _, source := range manifest.Sources {
		if source.Path == "" || filepath.IsAbs(source.Path) {
			continue
		}
		source.Path = filepath.Clean(filepath.Join(manifestDir, source.Path))
	}

	return manifest, nil
}

func isStdinManifestInput(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "-" {
		return true
	}

	return filepath.Clean(trimmed) == "/dev/stdin"
}

func loadStdinForApply() (*Manifest, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest from stdin: %w", err)
	}

	manifest, err := parseManifestYAML(data)
	if err != nil {
		return nil, err
	}

	for _, source := range manifest.Sources {
		if source.Path == "" || filepath.IsAbs(source.Path) {
			continue
		}
		return nil, fmt.Errorf("stdin manifest source %q has relative path %q: relative path sources are not supported for stdin manifests", source.Name, source.Path)
	}

	return manifest, nil
}

func loadRemoteForApply(manifestURL *url.URL, client *http.Client) (*Manifest, error) {
	if err := validateRemoteManifestURL(manifestURL); err != nil {
		return nil, err
	}

	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Get(manifestURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch remote manifest: unexpected status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote manifest response: %w", err)
	}

	manifest, err := parseManifestYAML(data)
	if err != nil {
		return nil, err
	}

	for _, source := range manifest.Sources {
		if source.Path == "" || filepath.IsAbs(source.Path) {
			continue
		}
		return nil, fmt.Errorf("remote manifest source %q has relative path %q: relative path sources are not supported for remote manifests", source.Name, source.Path)
	}

	return manifest, nil
}

func validateRemoteManifestURL(manifestURL *url.URL) error {
	if path.Base(manifestURL.Path) != ManifestFileName {
		return fmt.Errorf("remote manifest URL must point directly to %s", ManifestFileName)
	}

	host := strings.ToLower(manifestURL.Hostname())
	if host != "github.com" {
		return nil
	}

	segments := strings.Split(strings.Trim(manifestURL.EscapedPath(), "/"), "/")
	for _, segment := range segments {
		switch segment {
		case "blob", "tree":
			return fmt.Errorf("GitHub web URLs using '/%s/' are not supported for repo apply-manifest; use a raw file URL that points directly to %s (for example https://raw.githubusercontent.com/<owner>/<repo>/<tag-or-ref>/path/%s)", segment, ManifestFileName, ManifestFileName)
		}
	}

	return nil
}

func parseManifestYAML(data []byte) (*Manifest, error) {
	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	return &manifest, nil
}
