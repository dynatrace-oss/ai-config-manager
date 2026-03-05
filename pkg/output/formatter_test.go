package output

import (
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      Format
		wantError bool
	}{
		{
			name:      "table format",
			input:     "table",
			want:      Table,
			wantError: false,
		},
		{
			name:      "json format",
			input:     "json",
			want:      JSON,
			wantError: false,
		},
		{
			name:      "yaml format",
			input:     "yaml",
			want:      YAML,
			wantError: false,
		},
		{
			name:      "uppercase table",
			input:     "TABLE",
			want:      Table,
			wantError: false,
		},
		{
			name:      "mixed case json",
			input:     "JsOn",
			want:      JSON,
			wantError: false,
		},
		{
			name:      "invalid format",
			input:     "xml",
			want:      "",
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			want:      "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFormat(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ParseFormat() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if got != tt.want {
				t.Errorf("ParseFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromBulkImportResult(t *testing.T) {
	// Create sample bulk import result
	bulkResult := &repo.BulkImportResult{
		Added:        []string{"/path/to/commands/test.md", "/path/to/skills/pdf-processing"},
		Skipped:      []string{"/path/to/commands/existing.md"},
		Failed:       []repo.ImportError{{Path: "/path/to/agents/bad.md", Message: "invalid format"}},
		CommandCount: 1,
		SkillCount:   1,
		AgentCount:   0,
		PackageCount: 0,
	}

	result := FromBulkImportResult(bulkResult)

	// Check counts
	if result.CommandCount != 1 {
		t.Errorf("CommandCount = %d, want 1", result.CommandCount)
	}
	if result.SkillCount != 1 {
		t.Errorf("SkillCount = %d, want 1", result.SkillCount)
	}
	if result.AgentCount != 0 {
		t.Errorf("AgentCount = %d, want 0", result.AgentCount)
	}
	if result.PackageCount != 0 {
		t.Errorf("PackageCount = %d, want 0", result.PackageCount)
	}

	// Check added resources
	if len(result.Added) != 2 {
		t.Errorf("Added count = %d, want 2", len(result.Added))
	}
	if len(result.Added) > 0 && result.Added[0].Name != "test" {
		t.Errorf("Added[0].Name = %s, want test", result.Added[0].Name)
	}
	if len(result.Added) > 0 && result.Added[0].Type != "command" {
		t.Errorf("Added[0].Type = %s, want command", result.Added[0].Type)
	}

	// Check skipped resources
	if len(result.Skipped) != 1 {
		t.Errorf("Skipped count = %d, want 1", len(result.Skipped))
	}
	if len(result.Skipped) > 0 && result.Skipped[0].Message != "already exists" {
		t.Errorf("Skipped[0].Message = %s, want 'already exists'", result.Skipped[0].Message)
	}

	// Check failed resources
	if len(result.Failed) != 1 {
		t.Errorf("Failed count = %d, want 1", len(result.Failed))
	}
	if len(result.Failed) > 0 && result.Failed[0].Message != "invalid format" {
		t.Errorf("Failed[0].Message = %s, want 'invalid format'", result.Failed[0].Message)
	}
}

func TestExtractResourceName(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "command file",
			path: "/path/to/commands/test.md",
			want: "test",
		},
		{
			name: "skill directory",
			path: "/path/to/skills/pdf-processing",
			want: "pdf-processing",
		},
		{
			name: "package file",
			path: "/path/to/packages/web-tools.package.json",
			want: "web-tools",
		},
		{
			name: "windows path",
			path: "C:\\path\\to\\commands\\build.md",
			want: "build",
		},
		{
			name: "just filename",
			path: "test.md",
			want: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractResourceName(tt.path)
			if got != tt.want {
				t.Errorf("extractResourceName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractResourceType(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "command file",
			path: "/path/to/commands/test.md",
			want: "command",
		},
		{
			name: "skill directory",
			path: "/path/to/skills/pdf-processing",
			want: "skill",
		},
		{
			name: "agent file",
			path: "/path/to/agents/code-reviewer.md",
			want: "agent",
		},
		{
			name: "package file by directory",
			path: "/path/to/packages/web-tools.package.json",
			want: "package",
		},
		{
			name: "package file by extension",
			path: "/some/other/location/tool.package.json",
			want: "package",
		},
		{
			name: "windows path",
			path: "C:\\path\\to\\skills\\test",
			want: "skill",
		},
		{
			name: "unknown md file",
			path: "/some/path/unknown.md",
			want: "command",
		},
		{
			name: "unknown file",
			path: "/some/path/file.txt",
			want: "resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractResourceType(tt.path)
			if got != tt.want {
				t.Errorf("extractResourceType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatBulkResult_Validation(t *testing.T) {
	result := &BulkOperationResult{
		Added:        []ResourceResult{{Name: "test", Type: "command"}},
		Skipped:      []ResourceResult{},
		Failed:       []ResourceResult{},
		CommandCount: 1,
		SkillCount:   0,
		AgentCount:   0,
		PackageCount: 0,
	}

	tests := []struct {
		name      string
		format    Format
		wantError bool
	}{
		{
			name:      "table format",
			format:    Table,
			wantError: false,
		},
		{
			name:      "json format",
			format:    JSON,
			wantError: false,
		},
		{
			name:      "yaml format",
			format:    YAML,
			wantError: false,
		},
		{
			name:      "invalid format",
			format:    Format("invalid"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatBulkResult(result, tt.format)
			if (err != nil) != tt.wantError {
				t.Errorf("FormatBulkResult() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestFormatAsJSON(t *testing.T) {
	result := &BulkOperationResult{
		Added: []ResourceResult{
			{Name: "test", Type: "command", Path: "/path/test.md"},
		},
		Skipped:      []ResourceResult{},
		Failed:       []ResourceResult{},
		CommandCount: 1,
		SkillCount:   0,
		AgentCount:   0,
		PackageCount: 0,
	}

	// Just verify it doesn't error - actual output goes to stdout
	err := formatAsJSON(result)
	if err != nil {
		t.Errorf("formatAsJSON() error = %v", err)
	}
}

func TestFormatAsYAML(t *testing.T) {
	result := &BulkOperationResult{
		Added: []ResourceResult{
			{Name: "test", Type: "command", Path: "/path/test.md"},
		},
		Skipped:      []ResourceResult{},
		Failed:       []ResourceResult{},
		CommandCount: 1,
		SkillCount:   0,
		AgentCount:   0,
		PackageCount: 0,
	}

	// Just verify it doesn't error - actual output goes to stdout
	err := formatAsYAML(result)
	if err != nil {
		t.Errorf("formatAsYAML() error = %v", err)
	}
}

func TestFormatAsTable(t *testing.T) {
	result := &BulkOperationResult{
		Added: []ResourceResult{
			{Name: "test", Type: "command"},
			{Name: "pdf", Type: "skill"},
		},
		Skipped: []ResourceResult{
			{Name: "existing", Type: "command", Message: "already exists"},
		},
		Failed: []ResourceResult{
			{Name: "bad", Type: "agent", Message: "invalid format"},
		},
		CommandCount: 1,
		SkillCount:   1,
		AgentCount:   0,
		PackageCount: 0,
	}

	// Just verify it doesn't error - actual output goes to stdout
	err := formatAsTable(result)
	if err != nil {
		t.Errorf("formatAsTable() error = %v", err)
	}
}

func TestResourceResultJSON(t *testing.T) {
	result := &BulkOperationResult{
		Added: []ResourceResult{
			{Name: "test", Type: "command", Path: "/test.md"},
		},
		Skipped:      []ResourceResult{},
		Failed:       []ResourceResult{},
		CommandCount: 1,
		SkillCount:   0,
		AgentCount:   0,
		PackageCount: 0,
	}

	// Check resource name
	data := result.Added[0].Name
	if data != "test" {
		t.Errorf("Resource name = %v, want test", data)
	}
}

func TestBulkOperationResultEmpty(t *testing.T) {
	result := &BulkOperationResult{
		Added:        []ResourceResult{},
		Skipped:      []ResourceResult{},
		Failed:       []ResourceResult{},
		CommandCount: 0,
		SkillCount:   0,
		AgentCount:   0,
		PackageCount: 0,
	}

	// Test all formats with empty result
	if err := formatAsTable(result); err != nil {
		t.Errorf("formatAsTable() with empty result error = %v", err)
	}
	if err := formatAsJSON(result); err != nil {
		t.Errorf("formatAsJSON() with empty result error = %v", err)
	}
	if err := formatAsYAML(result); err != nil {
		t.Errorf("formatAsYAML() with empty result error = %v", err)
	}
}

func TestFormatConstantValues(t *testing.T) {
	if Table != "table" {
		t.Errorf("Table constant = %v, want 'table'", Table)
	}
	if JSON != "json" {
		t.Errorf("JSON constant = %v, want 'json'", JSON)
	}
	if YAML != "yaml" {
		t.Errorf("YAML constant = %v, want 'yaml'", YAML)
	}
}

func TestResourceResultWithMessage(t *testing.T) {
	res := ResourceResult{
		Name:    "test-resource",
		Type:    "skill",
		Message: "some error occurred",
		Path:    "/path/to/resource",
	}

	if res.Name != "test-resource" {
		t.Errorf("Name = %v, want test-resource", res.Name)
	}
	if res.Type != "skill" {
		t.Errorf("Type = %v, want skill", res.Type)
	}
	if res.Message != "some error occurred" {
		t.Errorf("Message = %v, want 'some error occurred'", res.Message)
	}
	if res.Path != "/path/to/resource" {
		t.Errorf("Path = %v, want /path/to/resource", res.Path)
	}
}

func TestExtractResourceTypeEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "empty path",
			path: "",
			want: "resource",
		},
		{
			name: "path with multiple slashes",
			path: "//path///to////commands///test.md",
			want: "command",
		},
		{
			name: "path with spaces",
			path: "/path/to/commands/test file.md",
			want: "command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractResourceType(tt.path)
			if got != tt.want {
				t.Errorf("extractResourceType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractResourceNameEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "empty path",
			path: "",
			want: "",
		},
		{
			name: "path ending with slash",
			path: "/path/to/skills/test/",
			want: "",
		},
		{
			name: "multiple extensions",
			path: "/path/test.backup.md",
			want: "test.backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractResourceName(tt.path)
			if got != tt.want {
				t.Errorf("extractResourceName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseFormatCaseSensitivity(t *testing.T) {
	formats := []string{"table", "TABLE", "Table", "TaBlE", "json", "JSON", "Json", "yaml", "YAML", "Yaml"}
	for _, format := range formats {
		_, err := ParseFormat(format)
		if err != nil {
			t.Errorf("ParseFormat(%q) returned error: %v", format, err)
		}
	}
}

func TestBulkOperationResultCountsAccuracy(t *testing.T) {
	bulkResult := &repo.BulkImportResult{
		Added:        []string{"/commands/a.md", "/commands/b.md", "/skills/c", "/agents/d.md"},
		Skipped:      []string{},
		Failed:       []repo.ImportError{},
		CommandCount: 2,
		SkillCount:   1,
		AgentCount:   1,
		PackageCount: 0,
	}

	result := FromBulkImportResult(bulkResult)

	total := result.CommandCount + result.SkillCount + result.AgentCount + result.PackageCount
	if total != 4 {
		t.Errorf("Total count = %d, want 4", total)
	}
}

func TestFromBulkImportResultPreservesAllFields(t *testing.T) {
	bulkResult := &repo.BulkImportResult{
		Added: []string{
			"/path/commands/cmd1.md",
			"/path/skills/skill1",
		},
		Skipped: []string{
			"/path/commands/cmd2.md",
		},
		Failed: []repo.ImportError{
			{Path: "/path/agents/agent1.md", Message: "error1"},
			{Path: "/path/packages/pkg1.package.json", Message: "error2"},
		},
		CommandCount: 1,
		SkillCount:   1,
		AgentCount:   0,
		PackageCount: 0,
	}

	result := FromBulkImportResult(bulkResult)

	// Verify all lists are populated
	if len(result.Added) != 2 {
		t.Errorf("len(Added) = %d, want 2", len(result.Added))
	}
	if len(result.Skipped) != 1 {
		t.Errorf("len(Skipped) = %d, want 1", len(result.Skipped))
	}
	if len(result.Failed) != 2 {
		t.Errorf("len(Failed) = %d, want 2", len(result.Failed))
	}

	// Verify error messages are preserved
	if len(result.Failed) >= 1 && result.Failed[0].Message != "error1" {
		t.Errorf("Failed[0].Message = %s, want error1", result.Failed[0].Message)
	}
	if len(result.Failed) >= 2 && result.Failed[1].Message != "error2" {
		t.Errorf("Failed[1].Message = %s, want error2", result.Failed[1].Message)
	}
}

func TestFormatErrorMessages(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr string
	}{
		{
			name:    "invalid format xml",
			format:  "xml",
			wantErr: "invalid format: xml",
		},
		{
			name:    "invalid format csv",
			format:  "csv",
			wantErr: "invalid format: csv",
		},
		{
			name:    "empty format",
			format:  "",
			wantErr: "invalid format:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFormat(tt.format)
			if err == nil {
				t.Error("ParseFormat() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseFormat() error = %v, want to contain %v", err.Error(), tt.wantErr)
			}
		})
	}
}
