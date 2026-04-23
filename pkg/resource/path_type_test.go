package resource

import "testing"

func TestInferTypeFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantType ResourceType
		wantOK   bool
	}{
		{name: "commands subtree", path: "/tmp/repo/commands/build.md", wantType: Command, wantOK: true},
		{name: "claude commands subtree", path: "/tmp/repo/.claude/commands/review.md", wantType: Command, wantOK: true},
		{name: "opencode commands nested", path: "/tmp/repo/.opencode/commands/api/doctor.md", wantType: Command, wantOK: true},
		{name: "windows commands path", path: `C:\repo\commands\ops\deploy.md`, wantType: Command, wantOK: true},
		{name: "agents subtree", path: "/tmp/repo/agents/reviewer.md", wantType: Agent, wantOK: true},
		{name: "windows agents path", path: `C:\repo\agents\reviewer.md`, wantType: Agent, wantOK: true},
		{name: "skills subtree", path: "/tmp/repo/skills/summarize", wantType: Skill, wantOK: true},
		{name: "packages subtree", path: "/tmp/repo/packages/team.package.json", wantType: PackageType, wantOK: true},
		{name: "package suffix outside subtree", path: "/tmp/other/team.package.json", wantType: PackageType, wantOK: true},
		{name: "dotted path with commands segment", path: "/tmp/.config/repo.v2/commands/hello.md", wantType: Command, wantOK: true},
		{name: "bare markdown is ambiguous", path: "/tmp/ambiguous.md", wantType: "", wantOK: false},
		{name: "unknown extension", path: "/tmp/notes.txt", wantType: "", wantOK: false},
		{name: "empty path", path: "", wantType: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotOK := InferTypeFromPath(tt.path)
			if gotOK != tt.wantOK {
				t.Fatalf("InferTypeFromPath() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotType != tt.wantType {
				t.Fatalf("InferTypeFromPath() type = %v, want %v", gotType, tt.wantType)
			}
		})
	}
}
