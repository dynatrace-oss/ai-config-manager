package giturl

import "strings"

// NormalizeURL canonicalizes Git URLs for stable identity comparisons.
//
// Canonical behavior:
//   - trim surrounding whitespace
//   - lowercase
//   - repeatedly strip trailing "/"
//   - repeatedly strip trailing ".git"
//   - stop when stable
func NormalizeURL(url string) string {
	normalized := strings.TrimSpace(url)
	normalized = strings.ToLower(normalized)

	for {
		before := normalized
		normalized = strings.TrimSuffix(normalized, "/")
		normalized = strings.TrimSuffix(normalized, ".git")
		if normalized == before {
			break
		}
	}

	return normalized
}
