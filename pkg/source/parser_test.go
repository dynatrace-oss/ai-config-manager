package source

import (
	"strings"
	"testing"
)

func TestParseSource_GitHubPrefix(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantType    SourceType
		wantURL     string
		wantRef     string
		wantSubpath string
		wantError   bool
	}{
		{
			name:        "basic owner/repo",
			input:       "gh:owner/repo",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantSubpath: "",
			wantError:   false,
		},
		{
			name:        "with subpath",
			input:       "gh:owner/repo/path/to/skill",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantSubpath: "path/to/skill",
			wantError:   false,
		},
		{
			name:        "with ref",
			input:       "gh:owner/repo@main",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo/tree/main",
			wantRef:     "main",
			wantSubpath: "",
			wantError:   false,
		},
		{
			name:        "with ref and subpath",
			input:       "gh:owner/repo@main/path/to/skill",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo/tree/main",
			wantRef:     "main",
			wantSubpath: "path/to/skill",
			wantError:   false,
		},
		{
			name:        "with .git suffix",
			input:       "gh:owner/repo.git",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantSubpath: "",
			wantError:   false,
		},
		{
			name:      "empty after prefix",
			input:     "gh:",
			wantError: true,
		},
		{
			name:      "missing repo",
			input:     "gh:owner",
			wantError: true,
		},
		{
			name:      "empty owner",
			input:     "gh:/repo",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSource(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ParseSource(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseSource(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got.Type != tt.wantType {
				t.Errorf("ParseSource(%q).Type = %v, want %v", tt.input, got.Type, tt.wantType)
			}
			if got.URL != tt.wantURL {
				t.Errorf("ParseSource(%q).URL = %v, want %v", tt.input, got.URL, tt.wantURL)
			}
			if got.Ref != tt.wantRef {
				t.Errorf("ParseSource(%q).Ref = %v, want %v", tt.input, got.Ref, tt.wantRef)
			}
			if got.Subpath != tt.wantSubpath {
				t.Errorf("ParseSource(%q).Subpath = %v, want %v", tt.input, got.Subpath, tt.wantSubpath)
			}
		})
	}
}

func TestParseSource_LocalPrefix(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantType      SourceType
		wantLocalPath string
		wantError     bool
	}{
		{
			name:          "relative path",
			input:         "local:./path/to/skill",
			wantType:      Local,
			wantLocalPath: "path/to/skill",
			wantError:     false,
		},
		{
			name:          "absolute path",
			input:         "local:/abs/path/to/skill",
			wantType:      Local,
			wantLocalPath: "/abs/path/to/skill",
			wantError:     false,
		},
		{
			name:      "empty after prefix",
			input:     "local:",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSource(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ParseSource(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseSource(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got.Type != tt.wantType {
				t.Errorf("ParseSource(%q).Type = %v, want %v", tt.input, got.Type, tt.wantType)
			}
			if got.LocalPath != tt.wantLocalPath {
				t.Errorf("ParseSource(%q).LocalPath = %v, want %v", tt.input, got.LocalPath, tt.wantLocalPath)
			}
		})
	}
}

func TestParseSource_GitHubURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantType    SourceType
		wantURL     string
		wantRef     string
		wantSubpath string
		wantError   bool
	}{
		{
			name:        "basic GitHub URL",
			input:       "https://github.com/owner/repo",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantSubpath: "",
			wantError:   false,
		},
		{
			name:        "GitHub URL with .git",
			input:       "https://github.com/owner/repo.git",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "",
			wantSubpath: "",
			wantError:   false,
		},
		{
			name:        "GitHub tree URL",
			input:       "https://github.com/owner/repo/tree/main",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantSubpath: "",
			wantError:   false,
		},
		{
			name:        "GitHub tree URL with subpath",
			input:       "https://github.com/owner/repo/tree/main/path/to/skill",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantSubpath: "path/to/skill",
			wantError:   false,
		},
		{
			name:        "GitHub blob URL",
			input:       "https://github.com/owner/repo/blob/main/README.md",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantSubpath: "README.md",
			wantError:   false,
		},
		{
			name:        "GitHub blob marketplace URL",
			input:       "https://github.com/owner/repo/blob/main/.claude-plugin/marketplace.json",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantSubpath: ".claude-plugin/marketplace.json",
			wantError:   false,
		},
		{
			name:        "raw GitHub marketplace URL normalized",
			input:       "https://raw.githubusercontent.com/owner/repo/main/.claude-plugin/marketplace.json",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo",
			wantRef:     "main",
			wantSubpath: ".claude-plugin/marketplace.json",
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSource(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ParseSource(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseSource(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got.Type != tt.wantType {
				t.Errorf("ParseSource(%q).Type = %v, want %v", tt.input, got.Type, tt.wantType)
			}
			if got.URL != tt.wantURL {
				t.Errorf("ParseSource(%q).URL = %v, want %v", tt.input, got.URL, tt.wantURL)
			}
			if got.Ref != tt.wantRef {
				t.Errorf("ParseSource(%q).Ref = %v, want %v", tt.input, got.Ref, tt.wantRef)
			}
			if got.Subpath != tt.wantSubpath {
				t.Errorf("ParseSource(%q).Subpath = %v, want %v", tt.input, got.Subpath, tt.wantSubpath)
			}
		})
	}
}

func TestParseSource_RepoBackedMarketplaceFailures(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		errContain string
	}{
		{
			name:       "github marketplace path without blob ref is rejected",
			input:      "https://github.com/owner/repo/marketplace.json",
			errContain: "repo-backed /blob/<ref>/.../marketplace.json",
		},
		{
			name:       "gitlab marketplace url rejected",
			input:      "https://gitlab.com/owner/repo/-/raw/main/marketplace.json",
			errContain: "only repo-backed URLs that normalize to clone URL + ref + manifest path are supported",
		},
		{
			name:       "non repo marketplace url rejected",
			input:      "https://example.com/marketplace.json",
			errContain: "standalone remote manifest fetching is not supported",
		},
		{
			name:       "raw github non marketplace url rejected",
			input:      "https://raw.githubusercontent.com/owner/repo/main/README.md",
			errContain: "only repo-backed marketplace.json file URLs are supported",
		},
		{
			name:       "raw github marketplace missing ref rejected",
			input:      "https://raw.githubusercontent.com/owner/repo/marketplace.json",
			errContain: "unable to normalize raw GitHub marketplace URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSource(tt.input)
			if err == nil {
				t.Fatalf("ParseSource(%q) expected error, got nil", tt.input)
			}
			if !strings.Contains(err.Error(), tt.errContain) {
				t.Fatalf("ParseSource(%q) error = %q, want contain %q", tt.input, err.Error(), tt.errContain)
			}
		})
	}
}

func TestParseSource_GitSSH(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  SourceType
		wantURL   string
		wantError bool
	}{
		{
			name:      "GitHub SSH",
			input:     "git@github.com:owner/repo.git",
			wantType:  GitHub,
			wantURL:   "https://github.com/owner/repo",
			wantError: false,
		},
		{
			name:      "GitHub SSH without .git",
			input:     "git@github.com:owner/repo",
			wantType:  GitHub,
			wantURL:   "https://github.com/owner/repo",
			wantError: false,
		},
		{
			name:      "GitLab SSH",
			input:     "git@gitlab.com:owner/repo.git",
			wantType:  GitLab,
			wantURL:   "https://gitlab.com/owner/repo",
			wantError: false,
		},
		{
			name:      "generic Git SSH",
			input:     "git@example.com:owner/repo.git",
			wantType:  GitURL,
			wantURL:   "https://example.com/owner/repo",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSource(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ParseSource(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseSource(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got.Type != tt.wantType {
				t.Errorf("ParseSource(%q).Type = %v, want %v", tt.input, got.Type, tt.wantType)
			}
			if got.URL != tt.wantURL {
				t.Errorf("ParseSource(%q).URL = %v, want %v", tt.input, got.URL, tt.wantURL)
			}
		})
	}
}

func TestParseSource_GitLabURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  SourceType
		wantURL   string
		wantError bool
	}{
		{
			name:      "basic GitLab URL",
			input:     "https://gitlab.com/owner/repo",
			wantType:  GitLab,
			wantURL:   "https://gitlab.com/owner/repo",
			wantError: false,
		},
		{
			name:      "GitLab URL with .git",
			input:     "https://gitlab.com/owner/repo.git",
			wantType:  GitLab,
			wantURL:   "https://gitlab.com/owner/repo",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSource(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ParseSource(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseSource(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got.Type != tt.wantType {
				t.Errorf("ParseSource(%q).Type = %v, want %v", tt.input, got.Type, tt.wantType)
			}
			if got.URL != tt.wantURL {
				t.Errorf("ParseSource(%q).URL = %v, want %v", tt.input, got.URL, tt.wantURL)
			}
		})
	}
}

func TestParseSource_GenericGitURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantType    SourceType
		wantURL     string
		wantSubpath string
		wantError   bool
	}{
		{
			name:      "generic HTTPS URL",
			input:     "https://example.com/repo.git",
			wantType:  GitURL,
			wantURL:   "https://example.com/repo.git",
			wantError: false,
		},
		{
			name:      "generic HTTP URL",
			input:     "http://example.com/repo.git",
			wantType:  GitURL,
			wantURL:   "http://example.com/repo.git",
			wantError: false,
		},
		{
			name:        "generic HTTPS URL with subpath via .git delimiter",
			input:       "https://bitbucket.example.com/scm/PROJECT/repo.git/knowledge-base",
			wantType:    GitURL,
			wantURL:     "https://bitbucket.example.com/scm/PROJECT/repo.git",
			wantSubpath: "knowledge-base",
			wantError:   false,
		},
		{
			name:        "generic HTTPS URL with deep subpath via .git delimiter",
			input:       "https://git.internal.com/scm/TEAM/mono-repo.git/path/to/resources",
			wantType:    GitURL,
			wantURL:     "https://git.internal.com/scm/TEAM/mono-repo.git",
			wantSubpath: "path/to/resources",
			wantError:   false,
		},
		{
			name:        "URL ending in .git has no subpath",
			input:       "https://example.com/owner/repo.git",
			wantType:    GitURL,
			wantURL:     "https://example.com/owner/repo.git",
			wantSubpath: "",
			wantError:   false,
		},
		{
			name:        "generic URL without .git has no subpath extraction",
			input:       "https://example.com/owner/repo",
			wantType:    GitURL,
			wantURL:     "https://example.com/owner/repo",
			wantSubpath: "",
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSource(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ParseSource(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseSource(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got.Type != tt.wantType {
				t.Errorf("ParseSource(%q).Type = %v, want %v", tt.input, got.Type, tt.wantType)
			}
			if got.URL != tt.wantURL {
				t.Errorf("ParseSource(%q).URL = %v, want %v", tt.input, got.URL, tt.wantURL)
			}
			if got.Subpath != tt.wantSubpath {
				t.Errorf("ParseSource(%q).Subpath = %v, want %v", tt.input, got.Subpath, tt.wantSubpath)
			}
		})
	}
}

func TestParseSource_InferredTypes_NowError(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantError  bool
		errContain string
	}{
		{
			name:       "bare owner/repo no longer inferred as GitHub",
			input:      "owner/repo",
			wantError:  true,
			errContain: "gh:owner/repo",
		},
		{
			name:       "relative path no longer inferred as local",
			input:      "./path/to/skill",
			wantError:  true,
			errContain: "local:./path/to/skill",
		},
		{
			name:       "parent path no longer inferred as local",
			input:      "../path/to/skill",
			wantError:  true,
			errContain: "local:../path/to/skill",
		},
		{
			name:       "absolute path no longer inferred as local",
			input:      "/abs/path/to/skill",
			wantError:  true,
			errContain: "local:/abs/path/to/skill",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSource(tt.input)
			if !tt.wantError {
				t.Fatal("all cases in this test should expect errors")
			}
			if err == nil {
				t.Errorf("ParseSource(%q) expected error, got nil", tt.input)
				return
			}
			if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
				t.Errorf("ParseSource(%q) error = %q, want it to contain %q", tt.input, err.Error(), tt.errContain)
			}
		})
	}
}

func TestParseSource_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "empty string",
			input:     "",
			wantError: true,
		},
		{
			name:      "whitespace only",
			input:     "   ",
			wantError: true,
		},
		{
			name:      "invalid format",
			input:     "invalid-format",
			wantError: true,
		},
		{
			name:      "just a slash",
			input:     "/",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSource(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("ParseSource(%q) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("ParseSource(%q) unexpected error: %v", tt.input, err)
				}
			}
		})
	}
}

func TestParseSource_TrailingSlashes(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantType    SourceType
		wantURL     string
		wantSubpath string
	}{
		{
			name:        "GitHub with trailing slash",
			input:       "gh:owner/repo/",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo",
			wantSubpath: "",
		},
		{
			name:        "GitHub URL with trailing slash",
			input:       "https://github.com/owner/repo/",
			wantType:    GitHub,
			wantURL:     "https://github.com/owner/repo",
			wantSubpath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSource(tt.input)
			if err != nil {
				t.Errorf("ParseSource(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got.Type != tt.wantType {
				t.Errorf("ParseSource(%q).Type = %v, want %v", tt.input, got.Type, tt.wantType)
			}
			if got.URL != tt.wantURL {
				t.Errorf("ParseSource(%q).URL = %v, want %v", tt.input, got.URL, tt.wantURL)
			}
			if got.Subpath != tt.wantSubpath {
				t.Errorf("ParseSource(%q).Subpath = %v, want %v", tt.input, got.Subpath, tt.wantSubpath)
			}
		})
	}
}

func TestSourceType_String(t *testing.T) {
	tests := []struct {
		sourceType SourceType
		expected   string
	}{
		{GitHub, "github"},
		{GitLab, "gitlab"},
		{Local, "local"},
		{GitURL, "git-url"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := string(tt.sourceType); got != tt.expected {
				t.Errorf("SourceType = %v, want %v", got, tt.expected)
			}
		})
	}
}
