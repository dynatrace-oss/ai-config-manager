package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maybeHoldAfterRepoLock provides deterministic, file-based process coordination
// for concurrency tests. It is inert unless AIMGR_TEST_REPO_HOLD_OP is set.
//
// Environment contract:
//   - AIMGR_TEST_REPO_HOLD_OP: operation key to hold (for example: "init")
//   - AIMGR_TEST_REPO_SIGNAL_DIR: directory where ready/release markers live
//
// Marker files:
//   - <signal-dir>/<op>.ready
//   - <signal-dir>/<op>.release
func maybeHoldAfterRepoLock(ctx context.Context, op string) error {
	holdOp := strings.TrimSpace(os.Getenv("AIMGR_TEST_REPO_HOLD_OP"))
	if holdOp == "" || holdOp != op {
		return nil
	}

	signalDir := strings.TrimSpace(os.Getenv("AIMGR_TEST_REPO_SIGNAL_DIR"))
	if signalDir == "" {
		return fmt.Errorf("AIMGR_TEST_REPO_SIGNAL_DIR must be set when AIMGR_TEST_REPO_HOLD_OP is used")
	}

	// #nosec G703 -- signalDir is used only for opt-in test coordination markers.
	if err := os.MkdirAll(signalDir, 0755); err != nil {
		return fmt.Errorf("failed to create test signal directory: %w", err)
	}

	readyPath := filepath.Join(signalDir, op+".ready")
	releasePath := filepath.Join(signalDir, op+".release")
	// #nosec G703 -- readyPath is a test-only marker under AIMGR_TEST_REPO_SIGNAL_DIR.
	if err := os.WriteFile(readyPath, []byte("ready"), 0644); err != nil {
		return fmt.Errorf("failed to write test ready marker: %w", err)
	}

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		// #nosec G703 -- releasePath is a test-only marker under AIMGR_TEST_REPO_SIGNAL_DIR.
		if _, err := os.Stat(releasePath); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("test hold canceled: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
