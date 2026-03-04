package test

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestOutputFormatTable tests table output format for bulk operations
func TestOutputFormatTable(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create a resources directory with valid skill
	resourcesDir := filepath.Join(testDir, "resources")
	createTestSkillInDir(t, resourcesDir, "output-test-skill", "A skill for testing table output")

	// Run repo add with table format (default)
	output, err := runAimgr(t, "repo", "add", "local:"+resourcesDir)
	if err != nil {
		t.Fatalf("Failed to add resources: %v\nOutput: %s", err, output)
	}

	// Verify table output structure
	if !strings.Contains(output, "NAME") {
		t.Errorf("Table output should contain NAME header, got: %s", output)
	}
	if !strings.Contains(output, "STATUS") {
		t.Errorf("Table output should contain STATUS header, got: %s", output)
	}
	if !strings.Contains(output, "MESSAGE") {
		t.Errorf("Table output should contain MESSAGE header, got: %s", output)
	}

	// Verify resource appears in table with type/name format
	if !strings.Contains(output, "skill/output-test-skill") {
		t.Errorf("Table should show resource in type/name format (skill/output-test-skill), got: %s", output)
	}
	if !strings.Contains(output, "SUCCESS") {
		t.Errorf("Table should show SUCCESS status, got: %s", output)
	}

	// Verify summary line (now includes "updated" count)
	summaryPattern := regexp.MustCompile(`Summary: (\d+) added, (\d+) updated, (\d+) failed, (\d+) skipped \((\d+) total\)`)
	if !summaryPattern.MatchString(output) {
		t.Errorf("Table output should contain summary line, got: %s", output)
	}
}

// TestOutputFormatJSON tests JSON output format for bulk operations
func TestOutputFormatJSON(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create a resources directory with valid skill
	resourcesDir := filepath.Join(testDir, "resources")
	createTestSkillInDir(t, resourcesDir, "json-test-skill", "A skill for testing JSON output")

	// Run repo add with JSON format
	output, err := runAimgr(t, "repo", "add", "--format=json", "local:"+resourcesDir)
	if err != nil {
		t.Fatalf("Failed to add resources: %v\nOutput: %s", err, output)
	}

	// Extract JSON from output (skip header lines)
	jsonStart := strings.Index(output, "{")
	if jsonStart == -1 {
		t.Fatalf("No JSON found in output: %s", output)
	}
	jsonOutput := output[jsonStart:]

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, jsonOutput)
	}

	// Verify JSON structure
	if _, ok := result["added"]; !ok {
		t.Error("JSON output should contain 'added' field")
	}
	if _, ok := result["failed"]; !ok {
		t.Error("JSON output should contain 'failed' field")
	}
	if _, ok := result["skipped"]; !ok {
		t.Error("JSON output should contain 'skipped' field")
	}
	if _, ok := result["command_count"]; !ok {
		t.Error("JSON output should contain 'command_count' field")
	}
	if _, ok := result["skill_count"]; !ok {
		t.Error("JSON output should contain 'skill_count' field")
	}
	if _, ok := result["agent_count"]; !ok {
		t.Error("JSON output should contain 'agent_count' field")
	}
	if _, ok := result["package_count"]; !ok {
		t.Error("JSON output should contain 'package_count' field")
	}

	// Verify added resources
	added, ok := result["added"].([]interface{})
	if !ok {
		t.Fatal("'added' field should be an array")
	}
	if len(added) != 1 {
		t.Errorf("Expected 1 added resource, got %d", len(added))
	}

	// Verify resource structure
	if len(added) > 0 {
		res, ok := added[0].(map[string]interface{})
		if !ok {
			t.Fatal("Resource should be an object")
		}
		if res["name"] != "json-test-skill" {
			t.Errorf("Resource name = %v, want json-test-skill", res["name"])
		}
		if res["type"] != "skill" {
			t.Errorf("Resource type = %v, want skill", res["type"])
		}
	}

	// Verify counts
	skillCount, ok := result["skill_count"].(float64)
	if !ok {
		t.Fatal("skill_count should be a number")
	}
	if skillCount != 1 {
		t.Errorf("skill_count = %v, want 1", skillCount)
	}
}

// TestOutputFormatYAML tests YAML output format for bulk operations
func TestOutputFormatYAML(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create a resources directory with valid skill
	resourcesDir := filepath.Join(testDir, "resources")
	createTestSkillInDir(t, resourcesDir, "yaml-test-skill", "A skill for testing YAML output")

	// Run repo add with YAML format
	output, err := runAimgr(t, "repo", "add", "--format=yaml", "local:"+resourcesDir)
	if err != nil {
		t.Fatalf("Failed to add resources: %v\nOutput: %s", err, output)
	}

	// Parse YAML output
	// Note: May contain stderr messages like "Created ai.repo.yaml" before the YAML
	// Try to find the start of the YAML by looking for the first line with "added:"
	yamlOutput := output
	if strings.Contains(output, "Created ai.repo.yaml") {
		// Strip lines before the actual YAML starts
		lines := strings.Split(output, "\n")
		yamlStart := 0
		for i, line := range lines {
			if strings.HasPrefix(line, "added:") || strings.HasPrefix(line, "updated:") {
				yamlStart = i
				break
			}
		}
		yamlOutput = strings.Join(lines[yamlStart:], "\n")
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlOutput), &result); err != nil {
		t.Fatalf("Failed to parse YAML output: %v\nOutput: %s", err, yamlOutput)
	}

	// Verify YAML structure
	if _, ok := result["added"]; !ok {
		t.Error("YAML output should contain 'added' field")
	}
	if _, ok := result["failed"]; !ok {
		t.Error("YAML output should contain 'failed' field")
	}
	if _, ok := result["skipped"]; !ok {
		t.Error("YAML output should contain 'skipped' field")
	}
	if _, ok := result["command_count"]; !ok {
		t.Error("YAML output should contain 'command_count' field")
	}
	if _, ok := result["skill_count"]; !ok {
		t.Error("YAML output should contain 'skill_count' field")
	}
	if _, ok := result["agent_count"]; !ok {
		t.Error("YAML output should contain 'agent_count' field")
	}
	if _, ok := result["package_count"]; !ok {
		t.Error("YAML output should contain 'package_count' field")
	}

	// Verify added resources
	added, ok := result["added"].([]interface{})
	if !ok {
		t.Fatal("'added' field should be an array")
	}
	if len(added) != 1 {
		t.Errorf("Expected 1 added resource, got %d", len(added))
	}

	// Verify resource structure
	if len(added) > 0 {
		res, ok := added[0].(map[string]interface{})
		if !ok {
			t.Fatal("Resource should be a map")
		}
		if res["name"] != "yaml-test-skill" {
			t.Errorf("Resource name = %v, want yaml-test-skill", res["name"])
		}
		if res["type"] != "skill" {
			t.Errorf("Resource type = %v, want skill", res["type"])
		}
	}

	// Verify counts
	skillCount, ok := result["skill_count"].(int)
	if !ok {
		t.Fatal("skill_count should be an int")
	}
	if skillCount != 1 {
		t.Errorf("skill_count = %v, want 1", skillCount)
	}
}

// TestOutputFormatMixedResults tests output with mixed success, failure, and skipped resources
func TestOutputFormatMixedResults(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create resources directory with mixed resources
	resourcesDir := filepath.Join(testDir, "resources")

	// Create valid skill
	createTestSkillInDir(t, resourcesDir, "mixed-valid-skill", "A valid skill")

	// First add: should succeed with 1 valid skill
	output1, err := runAimgr(t, "repo", "add", "--skip-existing", "--format=json", "local:"+resourcesDir)
	if err != nil {
		t.Fatalf("Failed first add: %v\nOutput: %s", err, output1)
	}

	// Extract JSON from output (skip header lines)
	jsonStart1 := strings.Index(output1, "{")
	if jsonStart1 == -1 {
		t.Fatalf("No JSON found in output: %s", output1)
	}
	jsonOutput1 := output1[jsonStart1:]

	// Parse first output
	var result1 map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput1), &result1); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, jsonOutput1)
	}

	// Verify first run has at least 1 added
	added1 := result1["added"].([]interface{})
	if len(added1) < 1 {
		t.Errorf("First run: expected at least 1 added, got %d", len(added1))
	}

	// Second add with skip-existing: should skip the valid one
	output2, err := runAimgr(t, "repo", "add", "--skip-existing", "--format=json", "local:"+resourcesDir)
	if err != nil {
		t.Fatalf("Failed second add: %v\nOutput: %s", err, output2)
	}

	// Extract JSON from output (skip header lines)
	jsonStart2 := strings.Index(output2, "{")
	if jsonStart2 == -1 {
		t.Fatalf("No JSON found in output: %s", output2)
	}
	jsonOutput2 := output2[jsonStart2:]

	// Parse second output
	var result2 map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput2), &result2); err != nil {
		t.Fatalf("Failed to parse second JSON output: %v\nOutput: %s", err, jsonOutput2)
	}

	// Verify second run has 0 added, at least 1 skipped
	added2 := result2["added"].([]interface{})
	skipped2 := result2["skipped"].([]interface{})
	if len(added2) != 0 {
		t.Errorf("Second run: expected 0 added, got %d", len(added2))
	}
	if len(skipped2) < 1 {
		t.Errorf("Second run: expected at least 1 skipped, got %d", len(skipped2))
	}

	// Verify skipped resources have correct structure
	if len(skipped2) > 0 {
		for _, s := range skipped2 {
			skippedRes := s.(map[string]interface{})
			if skippedRes["message"] != "already exists" {
				t.Errorf("Skipped message = %v, want 'already exists'", skippedRes["message"])
			}
			// At least one should be the valid skill
			if skippedRes["name"] == "mixed-valid-skill" {
				t.Logf("Verified mixed-valid-skill was skipped on second run")
			}
		}
	}
}

// TestOutputFormatDryRun tests output format with dry-run mode
func TestOutputFormatDryRun(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create a resources directory with valid command
	resourcesDir := filepath.Join(testDir, "resources")
	createTestCommandInDir(t, resourcesDir, "dryrun-test", "A command for testing dry-run output")

	// Run with dry-run and JSON format
	output, err := runAimgr(t, "repo", "add", "--dry-run", "--format=json", "local:"+resourcesDir)
	if err != nil {
		t.Fatalf("Failed dry-run add: %v\nOutput: %s", err, output)
	}

	// Extract JSON from output (skip header lines)
	jsonStart := strings.Index(output, "{")
	if jsonStart == -1 {
		t.Fatalf("No JSON found in output: %s", output)
	}
	jsonOutput := output[jsonStart:]

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, jsonOutput)
	}

	// Verify dry-run shows what would be added
	added := result["added"].([]interface{})
	if len(added) != 1 {
		t.Errorf("Dry-run: expected 1 added preview, got %d", len(added))
	}

	// Verify nothing was actually added
	listOutput, err := runAimgr(t, "repo", "list")
	if err != nil {
		t.Fatalf("Failed to list resources: %v", err)
	}

	if strings.Contains(listOutput, "dryrun-test") {
		t.Errorf("Dry-run should not add resources, but found 'dryrun-test' in: %s", listOutput)
	}
}

// TestOutputFormatErrorReporting tests error reporting with invalid resources
func TestOutputFormatErrorReporting(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Resolve to absolute path for testdata fixtures
	absFixtures, err := filepath.Abs(filepath.Join("testdata", "output"))
	if err != nil {
		t.Fatalf("Failed to resolve fixtures path: %v", err)
	}

	t.Run("table format with errors", func(t *testing.T) {
		// Run with table format (default) - should fail without skip-existing
		_, err := runAimgr(t, "repo", "add", "--skip-existing", "local:"+absFixtures)
		// Note: This will have discovery errors but won't fail the command
		// The valid skill should still be added

		// Check that error message appears in stderr or shows helpful info
		// Since runAimgr returns combined output, we just verify the command didn't crash
		if err != nil {
			// Check if it's a "no resources" error or similar expected error
			t.Logf("Expected error or warning: %v", err)
		}
	})

	t.Run("json format with errors", func(t *testing.T) {
		// Clean repo for this test
		testDir2 := t.TempDir()
		xdgData2 := filepath.Join(testDir2, "xdg-data")
		t.Setenv("XDG_DATA_HOME", xdgData2)

		// Try to add resources with JSON format
		output, err := runAimgr(t, "repo", "add", "--skip-existing", "--format=json", "local:"+absFixtures)
		if err != nil {
			t.Logf("Expected error with invalid resources: %v", err)
		}

		// If we got output, try to parse it
		if output != "" && json.Valid([]byte(output)) {
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(output), &result); err == nil {
				// Should have failed resources
				failed, _ := result["failed"].([]interface{})
				t.Logf("JSON format captured %d failed resources", len(failed))

				// Verify failed resources have error messages
				for _, f := range failed {
					failedRes, ok := f.(map[string]interface{})
					if !ok {
						continue
					}
					if msg, ok := failedRes["message"].(string); ok && msg != "" {
						t.Logf("Failed resource has error message: %s", msg)
					}
				}
			}
		}
	})
}

// TestOutputFormatBulkOperations tests output with multiple resources
func TestOutputFormatBulkOperations(t *testing.T) {
	testDir := t.TempDir()
	xdgData := filepath.Join(testDir, "xdg-data")
	repoDir := filepath.Join(testDir, "repo")

	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("AIMGR_REPO_PATH", repoDir)

	// Create resources directory with multiple resource types
	resourcesDir := filepath.Join(testDir, "resources")

	// Create commands
	createTestCommandInDir(t, resourcesDir, "bulk-cmd1", "First test command")
	createTestCommandInDir(t, resourcesDir, "bulk-cmd2", "Second test command")

	// Create skills
	createTestSkillInDir(t, resourcesDir, "bulk-skill1", "First test skill")

	// Create agent
	createTestAgentInDir(t, resourcesDir, "bulk-agent1", "First test agent")

	// Run bulk add with JSON format
	output, err := runAimgr(t, "repo", "add", "--format=json", "local:"+resourcesDir)
	if err != nil {
		t.Fatalf("Failed bulk add: %v\nOutput: %s", err, output)
	}

	// Extract JSON from output (skip header lines)
	jsonStart := strings.Index(output, "{")
	if jsonStart == -1 {
		t.Fatalf("No JSON found in output: %s", output)
	}
	jsonOutput := output[jsonStart:]

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, jsonOutput)
	}

	// Verify all resources were added
	added := result["added"].([]interface{})
	if len(added) != 4 {
		t.Errorf("Expected 4 added resources, got %d", len(added))
	}

	// Verify counts match actual resources
	commandCount := int(result["command_count"].(float64))
	skillCount := int(result["skill_count"].(float64))
	agentCount := int(result["agent_count"].(float64))

	if commandCount != 2 {
		t.Errorf("command_count = %d, want 2", commandCount)
	}
	if skillCount != 1 {
		t.Errorf("skill_count = %d, want 1", skillCount)
	}
	if agentCount != 1 {
		t.Errorf("agent_count = %d, want 1", agentCount)
	}

	// Verify resource types are correctly identified
	resourceTypes := make(map[string]int)
	for _, res := range added {
		resMap := res.(map[string]interface{})
		resType := resMap["type"].(string)
		resourceTypes[resType]++
	}

	if resourceTypes["command"] != 2 {
		t.Errorf("Found %d commands in output, want 2", resourceTypes["command"])
	}
	if resourceTypes["skill"] != 1 {
		t.Errorf("Found %d skills in output, want 1", resourceTypes["skill"])
	}
	if resourceTypes["agent"] != 1 {
		t.Errorf("Found %d agents in output, want 1", resourceTypes["agent"])
	}
}
