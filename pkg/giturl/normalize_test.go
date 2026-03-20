package giturl

import "testing"

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain URL",
			input:    "https://github.com/test/repo",
			expected: "https://github.com/test/repo",
		},
		{
			name:     "uppercase URL",
			input:    "HTTPS://GitHub.com/Test/Repo",
			expected: "https://github.com/test/repo",
		},
		{
			name:     "trailing slash",
			input:    "https://github.com/test/repo/",
			expected: "https://github.com/test/repo",
		},
		{
			name:     "dot git",
			input:    "https://github.com/test/repo.git",
			expected: "https://github.com/test/repo",
		},
		{
			name:     "dot git slash",
			input:    "https://github.com/test/repo.git/",
			expected: "https://github.com/test/repo",
		},
		{
			name:     "slash dot git",
			input:    "https://github.com/test/repo/.git",
			expected: "https://github.com/test/repo",
		},
		{
			name:     "multiple trailing slashes",
			input:    "https://github.com/test/repo///",
			expected: "https://github.com/test/repo",
		},
		{
			name:     "surrounding whitespace",
			input:    "  https://github.com/test/repo  ",
			expected: "https://github.com/test/repo",
		},
		{
			name:     "repeated strip until stable",
			input:    " https://github.com/test/repo.git/.git/// ",
			expected: "https://github.com/test/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeURL(tt.input); got != tt.expected {
				t.Fatalf("NormalizeURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
