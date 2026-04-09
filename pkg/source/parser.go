package source

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// SourceType represents the type of source
type SourceType string

const (
	// GitHub represents a GitHub repository
	GitHub SourceType = "github"
	// GitLab represents a GitLab repository
	GitLab SourceType = "gitlab"
	// Local represents a local filesystem path
	Local SourceType = "local"
	// GitURL represents a git URL (http/https/git protocol)
	GitURL SourceType = "git-url"
)

// ParsedSource represents a parsed source specification
type ParsedSource struct {
	Type      SourceType // Type of source
	URL       string     // Full URL for git sources
	LocalPath string     // Path for local sources
	Ref       string     // Branch/tag reference (optional)
	Subpath   string     // Path within repository (optional)
}

var (
	// GitHub owner/repo pattern
	githubOwnerRepoRegex = regexp.MustCompile(`^([a-zA-Z0-9_-]+)/([a-zA-Z0-9_.-]+)$`)
	// GitHub URL patterns
	githubURLRegex = regexp.MustCompile(`^https?://github\.com/([a-zA-Z0-9_-]+)/([a-zA-Z0-9_.-]+)`)
	// GitLab URL patterns
	gitlabURLRegex = regexp.MustCompile(`^https?://gitlab\.com/`)
	// Git SSH pattern
	gitSSHRegex = regexp.MustCompile(`^git@`)
)

const marketplaceManifestFileName = "marketplace.json"

// ParseSource parses a source specification and returns a ParsedSource.
//
// Supported formats (explicit prefix or scheme required):
//   - gh:owner/repo                    GitHub shorthand
//   - gh:owner/repo@ref                GitHub with branch/tag
//   - gh:owner/repo/path               GitHub with subpath
//   - gh:owner/repo@ref/path           GitHub with ref and subpath
//   - local:./relative/path            Local directory (relative)
//   - local:/absolute/path             Local directory (absolute)
//   - https://host/owner/repo          HTTPS Git URL (any host)
//   - http://host/owner/repo           HTTP Git URL (any host)
//   - git@host:owner/repo.git          SSH Git URL (any host)
//
// No implicit formats are supported. Bare "owner/repo" or "./path" will return
// an error with guidance on the correct format.
func ParseSource(input string) (*ParsedSource, error) {
	if input == "" {
		return nil, fmt.Errorf("source cannot be empty")
	}

	input = strings.TrimSpace(input)

	// Handle prefixed sources
	if strings.HasPrefix(input, "gh:") {
		return parseGitHubPrefix(strings.TrimPrefix(input, "gh:"))
	}

	if strings.HasPrefix(input, "local:") {
		return parseLocalPrefix(strings.TrimPrefix(input, "local:"))
	}

	// Handle HTTP/HTTPS URLs
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return parseHTTPURL(input)
	}

	// Handle Git SSH URLs
	if gitSSHRegex.MatchString(input) {
		return parseGitSSH(input)
	}

	// No implicit inference — provide helpful error messages
	if githubOwnerRepoRegex.MatchString(input) {
		return nil, fmt.Errorf("ambiguous source %q — use \"gh:%s\" for GitHub or provide a full URL (e.g., https://bitbucket.org/%s)", input, input, input)
	}

	if strings.HasPrefix(input, "./") || strings.HasPrefix(input, "../") || filepath.IsAbs(input) {
		return nil, fmt.Errorf("ambiguous source %q — use \"local:%s\" for local paths", input, input)
	}

	return nil, fmt.Errorf(`unrecognized source format: %s

Supported formats:
  gh:owner/repo                  GitHub repository
  local:./path or local:/path    Local directory
  https://host/owner/repo        HTTPS Git URL (GitHub, GitLab, Bitbucket, etc.)
  http://host/owner/repo         HTTP Git URL
  git@host:owner/repo.git        SSH Git URL`, input)
}

// parseGitHubPrefix parses a GitHub source with gh: prefix removed
// Formats:
//   - owner/repo
//   - owner/repo@branch
//   - owner/repo/path/to/resource
//   - owner/repo@branch/path/to/resource
func parseGitHubPrefix(input string) (*ParsedSource, error) {
	if input == "" {
		return nil, fmt.Errorf("GitHub source cannot be empty")
	}

	// Split on @ to extract ref if present
	var ref string
	var repoPath string

	atIndex := strings.Index(input, "@")
	if atIndex != -1 {
		repoPath = input[:atIndex]
		// Everything after @ is either just ref or ref/subpath
		refAndPath := input[atIndex+1:]

		// Find the next slash to separate ref from subpath
		slashIndex := strings.Index(refAndPath, "/")
		if slashIndex != -1 {
			ref = refAndPath[:slashIndex]
			// Subpath will be extracted later
		} else {
			ref = refAndPath
		}
	} else {
		repoPath = input
	}

	// Extract owner/repo and optional subpath
	parts := strings.SplitN(repoPath, "/", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GitHub source format: must be owner/repo")
	}

	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")

	if owner == "" || repo == "" {
		return nil, fmt.Errorf("GitHub owner and repo cannot be empty")
	}

	var subpath string
	if len(parts) > 2 {
		subpath = parts[2]
	}

	// If we have a ref, we need to reconstruct subpath from the original input
	if ref != "" && atIndex != -1 {
		// Find subpath after ref
		refAndPath := input[atIndex+1:]
		slashIndex := strings.Index(refAndPath, "/")
		if slashIndex != -1 {
			subpath = refAndPath[slashIndex+1:]
		}
	}

	githubURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	if ref != "" {
		githubURL = fmt.Sprintf("%s/tree/%s", githubURL, ref)
	}

	return &ParsedSource{
		Type:    GitHub,
		URL:     githubURL,
		Ref:     ref,
		Subpath: subpath,
	}, nil
}

// parseLocalPrefix parses a local source with local: prefix removed
func parseLocalPrefix(input string) (*ParsedSource, error) {
	if input == "" {
		return nil, fmt.Errorf("local path cannot be empty")
	}

	// Clean the path
	cleanPath := filepath.Clean(input)

	return &ParsedSource{
		Type:      Local,
		LocalPath: cleanPath,
	}, nil
}

// parseHTTPURL parses an HTTP/HTTPS URL
func parseHTTPURL(input string) (*ParsedSource, error) {
	parsedURL, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if normalized, rawErr := parseGitHubRawMarketplaceURL(parsedURL, input); rawErr != nil {
		return nil, rawErr
	} else if normalized != nil {
		return normalized, nil
	}

	// Check if it's a GitHub URL
	if githubURLRegex.MatchString(input) {
		parsed, parseErr := parseGitHubURL(input)
		if parseErr != nil {
			return nil, parseErr
		}

		// URLs that point to marketplace.json must be repo-backed file URLs that can
		// be normalized into clone URL + ref + subpath.
		if looksLikeMarketplaceManifestPath(parsedURL.Path) && parsed.Subpath == "" {
			return nil, fmt.Errorf("unsupported GitHub marketplace manifest URL %q: use a repo-backed /blob/<ref>/.../marketplace.json or raw.githubusercontent.com URL", input)
		}

		return parsed, nil
	}

	// Check if it's a GitLab URL
	if gitlabURLRegex.MatchString(input) {
		if looksLikeMarketplaceManifestPath(parsedURL.Path) {
			return nil, fmt.Errorf("unsupported remote marketplace manifest URL %q: only repo-backed URLs that normalize to clone URL + ref + manifest path are supported; standalone remote manifest fetching is not supported", input)
		}
		return parseGitLabURL(input)
	}

	if looksLikeMarketplaceManifestPath(parsedURL.Path) {
		return nil, fmt.Errorf("unsupported remote marketplace manifest URL %q: only repo-backed URLs that normalize to clone URL + ref + manifest path are supported; standalone remote manifest fetching is not supported", input)
	}

	// Generic git URL — extract subpath from .git/ delimiter if present
	// e.g., https://host/scm/PROJECT/repo.git/subpath → clone URL + subpath
	urlStr := parsedURL.String()
	var subpath string
	if idx := strings.Index(urlStr, ".git/"); idx != -1 {
		subpath = urlStr[idx+5:] // everything after ".git/"
		urlStr = urlStr[:idx+4]  // keep up to and including ".git"
	}

	return &ParsedSource{
		Type:    GitURL,
		URL:     urlStr,
		Subpath: subpath,
	}, nil
}

func parseGitHubRawMarketplaceURL(parsedURL *url.URL, input string) (*ParsedSource, error) {
	if parsedURL == nil {
		return nil, nil
	}

	if !strings.EqualFold(parsedURL.Host, "raw.githubusercontent.com") {
		return nil, nil
	}

	if !looksLikeMarketplaceManifestPath(parsedURL.Path) {
		return nil, fmt.Errorf("unsupported raw GitHub URL %q: only repo-backed marketplace.json file URLs are supported", input)
	}

	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) < 4 {
		return nil, fmt.Errorf("unable to normalize raw GitHub marketplace URL %q: expected /<owner>/<repo>/<ref>/.../marketplace.json", input)
	}

	owner := pathParts[0]
	repo := strings.TrimSuffix(pathParts[1], ".git")
	ref := pathParts[2]
	subpath := strings.Join(pathParts[3:], "/")

	if owner == "" || repo == "" || ref == "" || subpath == "" {
		return nil, fmt.Errorf("unable to normalize raw GitHub marketplace URL %q: expected /<owner>/<repo>/<ref>/.../marketplace.json", input)
	}

	return &ParsedSource{
		Type:    GitHub,
		URL:     fmt.Sprintf("https://github.com/%s/%s", owner, repo),
		Ref:     ref,
		Subpath: subpath,
	}, nil
}

func looksLikeMarketplaceManifestPath(rawPath string) bool {
	if rawPath == "" {
		return false
	}
	return strings.EqualFold(path.Base(rawPath), marketplaceManifestFileName)
}

// parseGitHubURL parses a full GitHub URL
// Supports:
//   - https://github.com/owner/repo
//   - https://github.com/owner/repo.git
//   - https://github.com/owner/repo/tree/branch
//   - https://github.com/owner/repo/tree/branch/path/to/resource
func parseGitHubURL(input string) (*ParsedSource, error) {
	// Remove trailing .git if present
	input = strings.TrimSuffix(input, ".git")

	parsedURL, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid GitHub URL: %w", err)
	}

	// Extract owner/repo from path
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) < 2 {
		return nil, fmt.Errorf("invalid GitHub URL: must include owner and repo")
	}

	owner := pathParts[0]
	repo := pathParts[1]

	if owner == "" || repo == "" {
		return nil, fmt.Errorf("GitHub owner and repo cannot be empty")
	}

	var ref string
	var subpath string

	// Check for /tree/branch or /blob/branch format
	if len(pathParts) >= 4 && (pathParts[2] == "tree" || pathParts[2] == "blob") {
		ref = pathParts[3]
		// Subpath is everything after the ref
		if len(pathParts) > 4 {
			subpath = strings.Join(pathParts[4:], "/")
		}
	}

	githubURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)

	return &ParsedSource{
		Type:    GitHub,
		URL:     githubURL,
		Ref:     ref,
		Subpath: subpath,
	}, nil
}

// parseGitLabURL parses a full GitLab URL
func parseGitLabURL(input string) (*ParsedSource, error) {
	// Remove trailing .git if present
	input = strings.TrimSuffix(input, ".git")

	parsedURL, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid GitLab URL: %w", err)
	}

	return &ParsedSource{
		Type: GitLab,
		URL:  parsedURL.String(),
	}, nil
}

// parseGitSSH parses a Git SSH URL
// Format: git@github.com:owner/repo.git
func parseGitSSH(input string) (*ParsedSource, error) {
	// Remove git@ prefix
	input = strings.TrimPrefix(input, "git@")

	// Split on :
	parts := strings.SplitN(input, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid Git SSH URL format")
	}

	host := parts[0]
	repoPath := strings.TrimSuffix(parts[1], ".git")

	// Determine source type based on host
	var sourceType SourceType
	if strings.Contains(host, "github.com") {
		sourceType = GitHub
	} else if strings.Contains(host, "gitlab.com") {
		sourceType = GitLab
	} else {
		sourceType = GitURL
	}

	// Reconstruct as HTTPS URL
	httpsURL := fmt.Sprintf("https://%s/%s", host, repoPath)

	return &ParsedSource{
		Type: sourceType,
		URL:  httpsURL,
	}, nil
}

// GetCloneURL converts a ParsedSource to a git clone URL
// This is useful for getting the clone URL from a parsed source
func GetCloneURL(ps *ParsedSource) (string, error) {
	if ps == nil {
		return "", fmt.Errorf("parsed source cannot be nil")
	}

	switch ps.Type {
	case GitHub:
		// Convert GitHub URL to git clone URL
		// https://github.com/owner/repo/tree/branch -> https://github.com/owner/repo
		url := ps.URL
		// Remove /tree/ref suffix if present
		if ps.Ref != "" {
			url = strings.TrimSuffix(url, fmt.Sprintf("/tree/%s", ps.Ref))
		}
		return url, nil

	case GitLab:
		return ps.URL, nil

	case GitURL:
		return ps.URL, nil

	case Local:
		return "", fmt.Errorf("local sources cannot be cloned")

	default:
		return "", fmt.Errorf("unsupported source type: %s", ps.Type)
	}
}
