package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type capturedOutput struct {
	Stdout string
	Stderr string
}

func captureOutput(t *testing.T, fn func()) capturedOutput {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	stdoutCh := make(chan string, 1)
	stderrCh := make(chan string, 1)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutR)
		stdoutCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrR)
		stderrCh <- buf.String()
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	stdout := <-stdoutCh
	stderr := <-stderrCh

	_ = stdoutR.Close()
	_ = stderrR.Close()

	return capturedOutput{Stdout: stdout, Stderr: stderr}
}

func assertGoldenText(t *testing.T, relPath string, actual string) {
	t.Helper()

	goldenPath := filepath.Join("testdata", "golden", relPath)
	actual = strings.ReplaceAll(actual, "\r\n", "\n")

	if os.Getenv("AIMGR_UPDATE_BASELINES") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatalf("failed to create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0644); err != nil {
			t.Fatalf("failed to write golden file %s: %v", goldenPath, err)
		}
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file %s: %v", goldenPath, err)
	}

	if actual != string(expected) {
		t.Fatalf("golden mismatch for %s\nset AIMGR_UPDATE_BASELINES=1 to refresh", goldenPath)
	}
}
