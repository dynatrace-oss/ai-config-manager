package resource

// DiscoveryError represents an error encountered during resource discovery.
type DiscoveryError struct {
	Path  string // Path where the error occurred.
	Error error  // The error that occurred.
}
