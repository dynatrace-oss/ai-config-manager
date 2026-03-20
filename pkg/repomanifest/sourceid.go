package repomanifest

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/giturl"
)

// GenerateSourceID produces a deterministic hash-based source ID from a source's
// canonical location (URL or absolute path). The returned ID has the format
// "src-" followed by 12 hex characters derived from a SHA-256 hash.
//
// For URL sources, the URL is normalized by lowercasing, stripping trailing
// slashes, and removing .git suffixes before hashing.
//
// For path sources, the path is resolved to an absolute path before hashing.
//
// Returns an empty string if the source has neither URL nor path.
func GenerateSourceID(source *Source) string {
	if source == nil {
		return ""
	}

	var canonical string

	if source.URL != "" {
		canonical = normalizeURL(source.URL)
	} else if source.Path != "" {
		canonical = normalizePath(source.Path)
	} else {
		return ""
	}

	hash := sha256.Sum256([]byte(canonical))
	hexStr := hex.EncodeToString(hash[:])

	return "src-" + hexStr[:12]
}

// normalizeURL normalizes a URL for consistent hashing by lowercasing,
// stripping trailing slashes, and removing .git suffixes.
func normalizeURL(url string) string {
	return giturl.NormalizeURL(url)
}

// normalizePath normalizes a filesystem path by resolving it to an absolute path.
func normalizePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		// Fall back to the original path if resolution fails
		return path
	}

	return abs
}
