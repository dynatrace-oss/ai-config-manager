// Package testutil provides common testing utilities for ai-config-manager tests.
//
// This package contains helper functions for integration tests, including:
//   - Network and Git availability checks
//   - Fixture path resolution
//   - Test environment setup
//
// Usage:
//
//	import "github.com/dynatrace-oss/ai-config-manager/v3/test/testutil"
//
//	func TestExample(t *testing.T) {
//	    testutil.SkipIfNoGit(t) // Skip if Git or network unavailable
//	    fixturePath := testutil.GetFixturePath("test-command.md")
//	    // ... test code
//	}
package testutil
