//go:build integration

package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/metadata"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repo"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/repomanifest"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/resource"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/source"
	"github.com/dynatrace-oss/ai-config-manager/v3/pkg/workspace"
)

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}

	return string(output)
}

func TestAddBulkFromGitHub_NonAmbiguousLegacyInlineRefSubpathWorksInDryRun(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	remoteOrigin, worktreePath := createRemoteGitSource(t)
	if err := os.MkdirAll(filepath.Join(worktreePath, "skills", "example-skill"), 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}
	skillContent := "---\nname: example-skill\ndescription: example\n---\n# Example\n"
	if err := os.WriteFile(filepath.Join(worktreePath, "skills", "example-skill", "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}
	runGit(t, worktreePath, "add", ".")
	runGit(t, worktreePath, "-c", "user.name=Test User", "-c", "user.email=test@example.com", "commit", "-m", "add example skill")
	runGit(t, worktreePath, "push", "origin", "main")

	parsed, err := source.ParseSource("gh:owner/repo@main/skills")
	if err != nil {
		t.Fatalf("expected non-ambiguous legacy shorthand to parse, got: %v", err)
	}
	// Redirect clone target to local test remote while preserving parsed ref/subpath.
	parsed.URL = remoteOrigin

	withRepoAddFlagsReset(t, func() {
		dryRunFlag = true
		if err := addBulkFromGitHub(parsed, manager); err != nil {
			t.Fatalf("expected addBulkFromGitHub dry-run to succeed for legacy non-ambiguous shorthand, got: %v", err)
		}
	})
}

func TestAddBulkFromGitHub_GenericRemoteExplicitSubpathGenericDiscovery_SkillLayouts(t *testing.T) {
	layouts := []struct {
		name       string
		skillPath  string
		sourceName string
	}{
		{
			name:       "catalog/skills/example-skill/SKILL.md",
			skillPath:  filepath.Join("catalog", "skills", "example-skill", "SKILL.md"),
			sourceName: "generic-layout-priority",
		},
		{
			name:       "catalog/example-skill/SKILL.md",
			skillPath:  filepath.Join("catalog", "example-skill", "SKILL.md"),
			sourceName: "generic-layout-recursive",
		},
		{
			name:       "catalog/.claude/skills/example-skill/SKILL.md",
			skillPath:  filepath.Join("catalog", ".claude", "skills", "example-skill", "SKILL.md"),
			sourceName: "generic-layout-claude",
		},
	}

	for _, tt := range layouts {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			repoPath := t.TempDir()
			t.Setenv("AIMGR_REPO_PATH", repoPath)

			manager := repo.NewManagerWithPath(repoPath)
			if err := manager.Init(); err != nil {
				t.Fatalf("failed to init repo: %v", err)
			}

			remoteOrigin, worktreePath := createRemoteGitSource(t)
			if err := os.MkdirAll(filepath.Dir(filepath.Join(worktreePath, tt.skillPath)), 0755); err != nil {
				t.Fatalf("failed to create skill dir: %v", err)
			}

			skillContent := "---\nname: example-skill\ndescription: example\n---\n# Example\n"
			if err := os.WriteFile(filepath.Join(worktreePath, tt.skillPath), []byte(skillContent), 0644); err != nil {
				t.Fatalf("failed to write SKILL.md: %v", err)
			}

			runGit(t, worktreePath, "add", ".")
			runGit(t, worktreePath, "-c", "user.name=Test User", "-c", "user.email=test@example.com", "commit", "-m", "add layout")
			runGit(t, worktreePath, "push", "origin", "main")

			parsed := &source.ParsedSource{
				Type:    source.GitURL,
				URL:     remoteOrigin,
				Ref:     "main",
				Subpath: "catalog",
			}

			withRepoAddFlagsReset(t, func() {
				discoveryFlag = repomanifest.DiscoveryModeGeneric
				nameFlag = tt.sourceName
				if err := addBulkFromGitHub(parsed, manager); err != nil {
					t.Fatalf("addBulkFromGitHub failed: %v", err)
				}
			})

			skill, err := manager.Get("example-skill", resource.Skill)
			if err != nil {
				t.Fatalf("failed to get imported skill %q: %v", "example-skill", err)
			}
			if skill == nil {
				t.Fatalf("expected skill %q to be imported", "example-skill")
			}
		})
	}
}

func TestAddSourceToManifest_PersistsDiscoveryMode(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	srcPath := t.TempDir()
	parsed, err := source.ParseSource("local:" + srcPath)
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	withRepoAddFlagsReset(t, func() {
		if err := addSourceToManifest(manager, parsed, nil, repomanifest.DiscoveryModeMarketplace); err != nil {
			t.Fatalf("addSourceToManifest failed: %v", err)
		}
	})

	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if len(manifest.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(manifest.Sources))
	}
	if manifest.Sources[0].Discovery != repomanifest.DiscoveryModeMarketplace {
		t.Fatalf("expected discovery %q, got %q", repomanifest.DiscoveryModeMarketplace, manifest.Sources[0].Discovery)
	}
}

func TestAddSourceToManifest_StoresNormalizedRepoBackedMarketplaceSource(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	parsed, err := source.ParseSource("https://raw.githubusercontent.com/example/tools/main/.claude-plugin/marketplace.json")
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	withRepoAddFlagsReset(t, func() {
		if err := addSourceToManifest(manager, parsed, nil, repomanifest.DiscoveryModeAuto); err != nil {
			t.Fatalf("addSourceToManifest failed: %v", err)
		}
	})

	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if len(manifest.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(manifest.Sources))
	}

	src := manifest.Sources[0]
	if src.URL != "https://github.com/example/tools" {
		t.Fatalf("source URL = %q, want %q", src.URL, "https://github.com/example/tools")
	}
	if src.Ref != "main" {
		t.Fatalf("source ref = %q, want %q", src.Ref, "main")
	}
	if src.Subpath != ".claude-plugin/marketplace.json" {
		t.Fatalf("source subpath = %q, want %q", src.Subpath, ".claude-plugin/marketplace.json")
	}
}

func TestAddSourceToManifest_RemoteCanonicalReuseKeepsExistingName(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	firstParsed, err := source.ParseSource("https://GitHub.com/Example/Tools.git/")
	if err != nil {
		t.Fatalf("failed to parse first source: %v", err)
	}

	withRepoAddFlagsReset(t, func() {
		nameFlag = "primary-alias"
		if err := addSourceToManifest(manager, firstParsed, []string{"skill/*"}, repomanifest.DiscoveryModeAuto); err != nil {
			t.Fatalf("first addSourceToManifest failed: %v", err)
		}
	})

	// Capture stderr warning for alias/ref mismatch reuse path.
	originalStderr := os.Stderr
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create stderr pipe: %v", pipeErr)
	}
	os.Stderr = w

	secondParsed, err := source.ParseSource("https://github.com/example/tools")
	if err != nil {
		t.Fatalf("failed to parse second source: %v", err)
	}
	secondParsed.Ref = "release/v2"

	withRepoAddFlagsReset(t, func() {
		nameFlag = "second-alias"
		if err := addSourceToManifest(manager, secondParsed, []string{"command/*"}, repomanifest.DiscoveryModeGeneric); err != nil {
			t.Fatalf("second addSourceToManifest failed: %v", err)
		}
	})

	_ = w.Close()
	os.Stderr = originalStderr

	var errBuf bytes.Buffer
	if _, err := io.Copy(&errBuf, r); err != nil {
		t.Fatalf("failed reading captured stderr: %v", err)
	}
	stderrText := errBuf.String()
	if !strings.Contains(stderrText, "already exists as 'primary-alias'") {
		t.Fatalf("expected alias reuse warning, got stderr: %s", stderrText)
	}
	if !strings.Contains(stderrText, "ignoring requested ref 'release/v2'") {
		t.Fatalf("expected ref mismatch reuse warning, got stderr: %s", stderrText)
	}

	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if len(manifest.Sources) != 1 {
		t.Fatalf("expected one canonical remote source, got %d", len(manifest.Sources))
	}

	src := manifest.Sources[0]
	if src.Name != "primary-alias" {
		t.Fatalf("expected existing source name to be preserved, got %q", src.Name)
	}
	if src.Ref != "" {
		t.Fatalf("expected existing ref to remain unchanged (empty), got %q", src.Ref)
	}
	if got := strings.Join(src.Include, ","); got != "command/*" {
		t.Fatalf("expected include to be replaced on reuse, got %q", got)
	}
	if src.Discovery != repomanifest.DiscoveryModeGeneric {
		t.Fatalf("expected discovery mode update on reuse, got %q", src.Discovery)
	}
}

func TestAddSourceToManifest_RemoteCanonicalReuseDistinctSubpaths(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	firstParsed, err := source.ParseSource("gh:example/tools/skills")
	if err != nil {
		t.Fatalf("failed to parse first source: %v", err)
	}
	secondParsed, err := source.ParseSource("gh:example/tools/agents")
	if err != nil {
		t.Fatalf("failed to parse second source: %v", err)
	}

	withRepoAddFlagsReset(t, func() {
		nameFlag = "skills-source"
		if err := addSourceToManifest(manager, firstParsed, nil, repomanifest.DiscoveryModeAuto); err != nil {
			t.Fatalf("failed adding first subpath source: %v", err)
		}
	})

	withRepoAddFlagsReset(t, func() {
		nameFlag = "agents-source"
		if err := addSourceToManifest(manager, secondParsed, nil, repomanifest.DiscoveryModeAuto); err != nil {
			t.Fatalf("failed adding second subpath source: %v", err)
		}
	})

	manifest, err := repomanifest.Load(repoPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if len(manifest.Sources) != 2 {
		t.Fatalf("expected distinct subpaths to remain separate sources, got %d", len(manifest.Sources))
	}
}

func TestAddBulkFromGitHub_UsesCanonicalSourceIDWithSubpath(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	remoteURL := "https://example.com/team/tools"
	remoteOrigin, worktreePath := createRemoteGitSource(t)
	writeAndCommitRemoteCommand(t, worktreePath, "alpha", "Alpha command")
	writeAndCommitRemoteCommand(t, worktreePath, "beta", "Beta command")

	wsMgr, err := workspace.NewManager(repoPath)
	if err != nil {
		t.Fatalf("failed to create workspace manager: %v", err)
	}
	cachePath := filepath.Join(repoPath, ".workspace", workspace.ComputeHash(remoteURL))
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		t.Fatalf("failed to create workspace dir: %v", err)
	}
	runGit(t, repoPath, "clone", "-b", "main", remoteOrigin, cachePath)
	if err := wsMgr.Update(remoteURL, ""); err != nil {
		t.Fatalf("failed to update workspace cache: %v", err)
	}

	parsed, err := source.ParseSource(remoteURL + ".git/commands")
	if err != nil {
		t.Fatalf("failed to parse remote source with subpath: %v", err)
	}

	withRepoAddFlagsReset(t, func() {
		addFormatFlag = "json"
		if err := addBulkFromGitHub(parsed, manager); err != nil {
			t.Fatalf("addBulkFromGitHub failed: %v", err)
		}
	})

	manifestSource := &repomanifest.Source{URL: parsed.URL, Subpath: parsed.Subpath}
	expectedID := repomanifest.GenerateSourceID(manifestSource)
	legacyID := repomanifest.GenerateSourceID(&repomanifest.Source{URL: parsed.URL})
	if expectedID == legacyID {
		t.Fatalf("test precondition failed: expected canonical and legacy IDs to differ")
	}

	for _, cmdName := range []string{"alpha", "beta"} {
		meta, metaErr := metadata.Load(cmdName, resource.Command, repoPath)
		if metaErr != nil {
			t.Fatalf("failed to load metadata for %s: %v", cmdName, metaErr)
		}
		if meta.SourceID != expectedID {
			t.Fatalf("command %s source ID = %q, want canonical %q", cmdName, meta.SourceID, expectedID)
		}
	}
}

func TestRepoAdd_ManifestCommitIsScopedToManifestFiles(t *testing.T) {
	repoPath := t.TempDir()
	t.Setenv("AIMGR_REPO_PATH", repoPath)

	manager := repo.NewManagerWithPath(repoPath)
	if err := manager.Init(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	unrelated := filepath.Join(repoPath, "unrelated.txt")
	if err := os.WriteFile(unrelated, []byte("base\n"), 0644); err != nil {
		t.Fatalf("failed to create unrelated file: %v", err)
	}
	if err := manager.CommitChangesForPaths("test: add unrelated file", []string{"unrelated.txt"}); err != nil {
		t.Fatalf("failed to commit unrelated baseline: %v", err)
	}
	if err := os.WriteFile(unrelated, []byte("base\nlocal change\n"), 0644); err != nil {
		t.Fatalf("failed to modify unrelated file: %v", err)
	}

	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create source commands dir: %v", err)
	}
	cmdContent := "---\ndescription: Add test command\n---\n# add-test\n"
	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "add-test.md"), []byte(cmdContent), 0644); err != nil {
		t.Fatalf("failed to write source command: %v", err)
	}

	withRepoAddFlagsReset(t, func() {
		if err := repoAddCmd.RunE(repoAddCmd, []string{"local:" + sourceDir}); err != nil {
			t.Fatalf("repo add failed: %v", err)
		}
	})

	status := gitOutput(t, repoPath, "status", "--porcelain")
	if !strings.Contains(status, " M unrelated.txt") {
		t.Fatalf("expected unrelated.txt to remain unstaged after repo add, status:\n%s", status)
	}

	manifestCommitFiles := gitOutput(t, repoPath, "show", "--name-only", "--pretty=format:", "HEAD")
	if !strings.Contains(manifestCommitFiles, "ai.repo.yaml") {
		t.Fatalf("expected manifest tracking commit to include ai.repo.yaml, got:\n%s", manifestCommitFiles)
	}
	if !strings.Contains(manifestCommitFiles, ".metadata/sources.json") {
		t.Fatalf("expected manifest tracking commit to include .metadata/sources.json, got:\n%s", manifestCommitFiles)
	}
	if strings.Contains(manifestCommitFiles, "unrelated.txt") {
		t.Fatalf("manifest tracking commit must not include unrelated.txt, got:\n%s", manifestCommitFiles)
	}
}

func TestImportFromLocalPathWithMode_DiscoveryModes(t *testing.T) {
	sourceDir := createSourceWithMarketplaceAndLooseResources(t)

	tests := []struct {
		name              string
		discoveryMode     string
		expectLoose       bool
		expectPlugin      bool
		expectPackage     bool
		expectImportError string
	}{
		{
			name:          "auto prefers marketplace only",
			discoveryMode: repomanifest.DiscoveryModeAuto,
			expectLoose:   false,
			expectPlugin:  true,
			expectPackage: true,
		},
		{
			name:          "marketplace imports marketplace only",
			discoveryMode: repomanifest.DiscoveryModeMarketplace,
			expectLoose:   false,
			expectPlugin:  true,
			expectPackage: true,
		},
		{
			name:          "generic ignores marketplace",
			discoveryMode: repomanifest.DiscoveryModeGeneric,
			expectLoose:   true,
			expectPlugin:  true,
			expectPackage: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withRepoAddFlagsReset(t, func() {
				repoPath := t.TempDir()
				manager := repo.NewManagerWithPath(repoPath)
				if err := manager.Init(); err != nil {
					t.Fatalf("failed to init repo: %v", err)
				}

				_, err := importFromLocalPathWithMode(
					sourceDir,
					manager,
					nil,
					"file://"+sourceDir,
					string(source.Local),
					"",
					"symlink",
					tt.discoveryMode,
					"test-source",
					"src-test",
				)
				if tt.expectImportError != "" {
					if err == nil || !strings.Contains(err.Error(), tt.expectImportError) {
						t.Fatalf("expected error containing %q, got %v", tt.expectImportError, err)
					}
					return
				}
				if err != nil {
					t.Fatalf("import failed: %v", err)
				}

				loose, _ := manager.Get("loose-command", resource.Command)
				if tt.expectLoose && loose == nil {
					t.Fatalf("expected loose-command to be imported")
				}
				if !tt.expectLoose && loose != nil {
					t.Fatalf("expected loose-command to be excluded")
				}

				plugin, _ := manager.Get("plugin-command", resource.Command)
				if tt.expectPlugin && plugin == nil {
					t.Fatalf("expected plugin-command to be imported")
				}
				if !tt.expectPlugin && plugin != nil {
					t.Fatalf("expected plugin-command to be excluded")
				}

				pkgPath := filepath.Join(repoPath, "packages", "market-plugin.package.json")
				_, pkgErr := os.Stat(pkgPath)
				hasPkg := pkgErr == nil
				if tt.expectPackage && !hasPkg {
					t.Fatalf("expected marketplace package to exist")
				}
				if !tt.expectPackage && hasPkg {
					t.Fatalf("expected marketplace package to be absent")
				}
			})
		})
	}
}

func TestImportFromLocalPathWithMode_MarketplaceRequirementsAndZeroResolvable(t *testing.T) {
	t.Run("marketplace mode requires marketplace file", func(t *testing.T) {
		withRepoAddFlagsReset(t, func() {
			sourceDir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
				t.Fatalf("failed to create commands dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(sourceDir, "commands", "just-command.md"), []byte("---\ndescription: only command\n---\n# just-command\n"), 0644); err != nil {
				t.Fatalf("failed to write command: %v", err)
			}

			repoPath := t.TempDir()
			manager := repo.NewManagerWithPath(repoPath)
			if err := manager.Init(); err != nil {
				t.Fatalf("failed to init repo: %v", err)
			}

			_, err := importFromLocalPathWithMode(sourceDir, manager, nil, "file://"+sourceDir, string(source.Local), "", "symlink", repomanifest.DiscoveryModeMarketplace, "test-source", "src-test")
			if err == nil {
				t.Fatal("expected marketplace-mode error when marketplace.json is missing")
			}
			if !strings.Contains(err.Error(), "requires marketplace.json") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})

	t.Run("zero-resolvable marketplace errors in auto and marketplace", func(t *testing.T) {
		sourceDir := createSourceWithMarketplaceNoResolvablePlugins(t)

		for _, mode := range []string{repomanifest.DiscoveryModeAuto, repomanifest.DiscoveryModeMarketplace} {
			t.Run(mode, func(t *testing.T) {
				withRepoAddFlagsReset(t, func() {
					repoPath := t.TempDir()
					manager := repo.NewManagerWithPath(repoPath)
					if err := manager.Init(); err != nil {
						t.Fatalf("failed to init repo: %v", err)
					}

					_, err := importFromLocalPathWithMode(sourceDir, manager, nil, "file://"+sourceDir, string(source.Local), "", "symlink", mode, "test-source", "src-test")
					if err == nil {
						t.Fatalf("expected zero-resolvable error for mode %q", mode)
					}
					if !strings.Contains(err.Error(), "no plugin resources were resolvable") {
						t.Fatalf("unexpected error for mode %q: %v", mode, err)
					}
				})
			})
		}
	})
}

func TestDiscoverImportResourcesByMode_MissingNormalizedMarketplacePath(t *testing.T) {
	baseDir := t.TempDir()
	missingMarketplacePath := filepath.Join(baseDir, ".missing", "marketplace.json")

	for _, mode := range []string{repomanifest.DiscoveryModeAuto, repomanifest.DiscoveryModeMarketplace} {
		t.Run(mode, func(t *testing.T) {
			_, err := discoverImportResourcesByMode(missingMarketplacePath, mode)
			if err == nil {
				t.Fatalf("expected missing normalized marketplace path error for mode %q", mode)
			}

			errMsg := err.Error()
			if !strings.Contains(errMsg, "marketplace.json/subpath lookup failed") {
				t.Fatalf("expected marketplace lookup error for mode %q, got: %v", mode, err)
			}
			if !strings.Contains(errMsg, missingMarketplacePath) {
				t.Fatalf("expected error to mention normalized path %q, got: %v", missingMarketplacePath, err)
			}
			if strings.Contains(errMsg, "failed to discover commands") {
				t.Fatalf("expected failure before generic discovery for mode %q, got: %v", mode, err)
			}
		})
	}
}

func TestAddBulkFromLocalWithMode_DirectMarketplaceFileInput(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		sourceDir := createSourceWithMarketplaceAndLooseResources(t)
		marketplaceFile := filepath.Join(sourceDir, "marketplace.json")

		repoPath := t.TempDir()
		manager := repo.NewManagerWithPath(repoPath)
		if err := manager.Init(); err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		discoveryFlag = repomanifest.DiscoveryModeAuto
		if err := addBulkFromLocalWithMode(marketplaceFile, manager, nil, "src-local-file", "symlink", "file-source"); err != nil {
			t.Fatalf("addBulkFromLocalWithMode failed for direct marketplace file: %v", err)
		}

		plugin, _ := manager.Get("plugin-command", resource.Command)
		if plugin == nil {
			t.Fatal("expected plugin-command to be imported from direct marketplace file")
		}

		loose, _ := manager.Get("loose-command", resource.Command)
		if loose != nil {
			t.Fatal("expected loose-command to remain excluded in auto marketplace mode")
		}
	})
}

func TestImportFromLocalPathWithMode_RepoMarketplaceSubpathFileInput(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		sourceDir := createSourceWithMarketplaceAndLooseResources(t)
		marketplaceFile := filepath.Join(sourceDir, "marketplace.json")

		repoPath := t.TempDir()
		manager := repo.NewManagerWithPath(repoPath)
		if err := manager.Init(); err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		// Simulate remote clone + subpath resolution ending in marketplace.json.
		_, err := importFromLocalPathWithMode(
			marketplaceFile,
			manager,
			nil,
			"https://github.com/example/repo",
			"github",
			"main",
			"copy",
			repomanifest.DiscoveryModeAuto,
			"remote-file-source",
			"src-remote-file",
		)
		if err != nil {
			t.Fatalf("importFromLocalPathWithMode failed for repo marketplace subpath file: %v", err)
		}

		plugin, _ := manager.Get("plugin-command", resource.Command)
		if plugin == nil {
			t.Fatal("expected plugin-command to be imported from subpath marketplace file")
		}
	})
}

func createSourceWithPluginDirectoryMarketplace(t *testing.T, marketplaceDirectory string) string {
	t.Helper()

	sourceDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
		t.Fatalf("failed to create loose commands dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "plugins", "dt-github", "commands"), 0755); err != nil {
		t.Fatalf("failed to create plugin commands dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, marketplaceDirectory), 0755); err != nil {
		t.Fatalf("failed to create marketplace dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(sourceDir, "commands", "loose-command.md"), []byte("---\ndescription: loose\n---\n# loose-command\n"), 0644); err != nil {
		t.Fatalf("failed to write loose command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "plugins", "dt-github", "commands", "plugin-command.md"), []byte("---\ndescription: plugin\n---\n# plugin-command\n"), 0644); err != nil {
		t.Fatalf("failed to write plugin command: %v", err)
	}

	marketplaceContent := `{
		"name": "plugin-manifest",
		"description": "plugin dir manifest",
		"plugins": [
			{
				"name": "dt-github",
				"description": "test plugin",
				"source": "plugins/dt-github"
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(sourceDir, marketplaceDirectory, "marketplace.json"), []byte(marketplaceContent), 0644); err != nil {
		t.Fatalf("failed to write marketplace.json: %v", err)
	}

	return sourceDir
}

func TestAddBulkFromLocalWithMode_DirectPluginDirectoryMarketplaceFileInput(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		tests := []struct {
			name                 string
			marketplaceDirectory string
		}{
			{name: "claude-plugin manifest", marketplaceDirectory: ".claude-plugin"},
			{name: "opencode-plugin manifest", marketplaceDirectory: ".opencode-plugin"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sourceDir := createSourceWithPluginDirectoryMarketplace(t, tt.marketplaceDirectory)
				marketplaceFile := filepath.Join(sourceDir, tt.marketplaceDirectory, "marketplace.json")

				repoPath := t.TempDir()
				manager := repo.NewManagerWithPath(repoPath)
				if err := manager.Init(); err != nil {
					t.Fatalf("failed to init repo: %v", err)
				}

				discoveryFlag = repomanifest.DiscoveryModeAuto
				if err := addBulkFromLocalWithMode(marketplaceFile, manager, nil, "src-local-file", "symlink", "file-source"); err != nil {
					t.Fatalf("addBulkFromLocalWithMode failed for direct plugin-dir marketplace file: %v", err)
				}

				plugin, _ := manager.Get("plugin-command", resource.Command)
				if plugin == nil {
					t.Fatal("expected plugin-command to be imported from direct plugin-dir marketplace file")
				}

				loose, _ := manager.Get("loose-command", resource.Command)
				if loose != nil {
					t.Fatal("expected loose-command to remain excluded in auto marketplace mode")
				}
			})
		}
	})
}

func TestImportFromLocalPathWithMode_RepoPluginDirectoryMarketplaceSubpathFileInput(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		tests := []struct {
			name                 string
			marketplaceDirectory string
			subpath              string
		}{
			{name: "claude-plugin manifest", marketplaceDirectory: ".claude-plugin", subpath: ".claude-plugin/marketplace.json"},
			{name: "opencode-plugin manifest", marketplaceDirectory: ".opencode-plugin", subpath: ".opencode-plugin/marketplace.json"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sourceDir := createSourceWithPluginDirectoryMarketplace(t, tt.marketplaceDirectory)
				marketplaceFile := filepath.Join(sourceDir, tt.marketplaceDirectory, "marketplace.json")

				repoPath := t.TempDir()
				manager := repo.NewManagerWithPath(repoPath)
				if err := manager.Init(); err != nil {
					t.Fatalf("failed to init repo: %v", err)
				}

				_, err := importFromLocalPathWithMode(
					marketplaceFile,
					manager,
					nil,
					"https://github.com/example/repo",
					"github",
					"main",
					"copy",
					repomanifest.DiscoveryModeAuto,
					"remote-file-source",
					"src-remote-file",
				)
				if err != nil {
					t.Fatalf("importFromLocalPathWithMode failed for repo plugin-dir marketplace subpath file (%s): %v", tt.subpath, err)
				}

				plugin, _ := manager.Get("plugin-command", resource.Command)
				if plugin == nil {
					t.Fatal("expected plugin-command to be imported from plugin-dir subpath marketplace file")
				}

				loose, _ := manager.Get("loose-command", resource.Command)
				if loose != nil {
					t.Fatal("expected loose-command to remain excluded in auto marketplace mode")
				}
			})
		}
	})
}

func TestMarketplaceSourceBasePath(t *testing.T) {
	tests := []struct {
		name            string
		localPath       string
		marketplacePath string
		want            string
	}{
		{
			name:            "directory discovery at repo root",
			localPath:       "/tmp/repo",
			marketplacePath: "/tmp/repo/.claude-plugin/marketplace.json",
			want:            "/tmp/repo",
		},
		{
			name:            "directory discovery in subpath",
			localPath:       "/tmp/repo/subdir",
			marketplacePath: "/tmp/repo/subdir/.opencode-plugin/marketplace.json",
			want:            "/tmp/repo/subdir",
		},
		{
			name:            "direct file path import",
			localPath:       "/tmp/repo/.claude-plugin/marketplace.json",
			marketplacePath: "/tmp/repo/.claude-plugin/marketplace.json",
			want:            "/tmp/repo",
		},
		{
			name:            "direct root marketplace file import",
			localPath:       "/tmp/repo/marketplace.json",
			marketplacePath: "/tmp/repo/marketplace.json",
			want:            "/tmp/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := marketplaceSourceBasePath(tt.localPath, tt.marketplacePath)
			if got != tt.want {
				t.Fatalf("marketplaceSourceBasePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestImportFromLocalPathWithMode_PluginDirectoryMarketplaceUsesRepoRootBase(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		tests := []struct {
			name                 string
			marketplaceDirectory string
		}{
			{name: "claude-plugin manifest", marketplaceDirectory: ".claude-plugin"},
			{name: "opencode-plugin manifest", marketplaceDirectory: ".opencode-plugin"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sourceDir := t.TempDir()

				if err := os.MkdirAll(filepath.Join(sourceDir, "commands"), 0755); err != nil {
					t.Fatalf("failed to create loose commands dir: %v", err)
				}
				if err := os.MkdirAll(filepath.Join(sourceDir, "plugins", "dt-github", "commands"), 0755); err != nil {
					t.Fatalf("failed to create plugin commands dir: %v", err)
				}
				if err := os.MkdirAll(filepath.Join(sourceDir, tt.marketplaceDirectory), 0755); err != nil {
					t.Fatalf("failed to create marketplace dir: %v", err)
				}

				if err := os.WriteFile(filepath.Join(sourceDir, "commands", "loose-command.md"), []byte("---\ndescription: loose\n---\n# loose-command\n"), 0644); err != nil {
					t.Fatalf("failed to write loose command: %v", err)
				}
				if err := os.WriteFile(filepath.Join(sourceDir, "plugins", "dt-github", "commands", "plugin-command.md"), []byte("---\ndescription: plugin\n---\n# plugin-command\n"), 0644); err != nil {
					t.Fatalf("failed to write plugin command: %v", err)
				}

				marketplaceContent := `{
					"name": "plugin-manifest",
					"description": "plugin dir manifest",
					"plugins": [
						{
							"name": "dt-github",
							"description": "test plugin",
							"source": "plugins/dt-github"
						}
					]
				}`
				if err := os.WriteFile(filepath.Join(sourceDir, tt.marketplaceDirectory, "marketplace.json"), []byte(marketplaceContent), 0644); err != nil {
					t.Fatalf("failed to write marketplace.json: %v", err)
				}

				repoPath := t.TempDir()
				manager := repo.NewManagerWithPath(repoPath)
				if err := manager.Init(); err != nil {
					t.Fatalf("failed to init repo: %v", err)
				}

				_, err := importFromLocalPathWithMode(
					sourceDir,
					manager,
					nil,
					"file://"+sourceDir,
					string(source.Local),
					"",
					"symlink",
					repomanifest.DiscoveryModeMarketplace,
					"test-source",
					"src-test",
				)
				if err != nil {
					t.Fatalf("importFromLocalPathWithMode failed: %v", err)
				}

				plugin, _ := manager.Get("plugin-command", resource.Command)
				if plugin == nil {
					t.Fatal("expected plugin-command to be imported from plugin source")
				}

				loose, _ := manager.Get("loose-command", resource.Command)
				if loose != nil {
					t.Fatal("expected loose command to remain excluded in marketplace mode")
				}
			})
		}
	})
}

func TestImportFromLocalPathWithMode_MarketplaceImportsReferencedDotAgentFiles(t *testing.T) {
	withRepoAddFlagsReset(t, func() {
		sourceDir := t.TempDir()

		if err := os.MkdirAll(filepath.Join(sourceDir, "plugins", "dt-service-onboarding", "commands"), 0755); err != nil {
			t.Fatalf("failed to create plugin commands dir: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(sourceDir, "plugins", "dt-service-onboarding", "agents"), 0755); err != nil {
			t.Fatalf("failed to create plugin agents dir: %v", err)
		}

		commandContent := "---\ndescription: onboarding command\n---\n# dt-onboarding\n"
		if err := os.WriteFile(filepath.Join(sourceDir, "plugins", "dt-service-onboarding", "commands", "dt-onboarding.md"), []byte(commandContent), 0644); err != nil {
			t.Fatalf("failed to write command: %v", err)
		}

		agentContent := "---\ndescription: onboarding agent\ntype: onboarding\n---\n# onboarding\n"
		if err := os.WriteFile(filepath.Join(sourceDir, "plugins", "dt-service-onboarding", "agents", "dt-service-onboarding.agent.md"), []byte(agentContent), 0644); err != nil {
			t.Fatalf("failed to write agent: %v", err)
		}

		marketplaceContent := `{
			"name": "agent-marketplace",
			"description": "marketplace with agent references",
			"plugins": [
				{
					"name": "dt-service-onboarding",
					"description": "plugin with command + agent",
					"source": "plugins/dt-service-onboarding"
				}
			]
		}`
		if err := os.WriteFile(filepath.Join(sourceDir, "marketplace.json"), []byte(marketplaceContent), 0644); err != nil {
			t.Fatalf("failed to write marketplace.json: %v", err)
		}

		repoPath := t.TempDir()
		manager := repo.NewManagerWithPath(repoPath)
		if err := manager.Init(); err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		_, err := importFromLocalPathWithMode(
			sourceDir,
			manager,
			nil,
			"file://"+sourceDir,
			string(source.Local),
			"",
			"symlink",
			repomanifest.DiscoveryModeMarketplace,
			"test-source",
			"src-test",
		)
		if err != nil {
			t.Fatalf("importFromLocalPathWithMode failed: %v", err)
		}

		agent, _ := manager.Get("dt-service-onboarding", resource.Agent)
		if agent == nil {
			t.Fatal("expected referenced agent to be imported")
		}

		pkgPath := filepath.Join(repoPath, "packages", "dt-service-onboarding.package.json")
		pkg, err := resource.LoadPackage(pkgPath)
		if err != nil {
			t.Fatalf("failed to load generated marketplace package: %v", err)
		}

		foundAgentRef := false
		for _, ref := range pkg.Resources {
			if ref == "agent/dt-service-onboarding" {
				foundAgentRef = true
				break
			}
		}
		if !foundAgentRef {
			t.Fatalf("expected package to reference imported agent, resources: %v", pkg.Resources)
		}

		index, err := repo.BuildPackageReferenceIndexFromRoots([]string{repoPath})
		if err != nil {
			t.Fatalf("failed to build package reference index: %v", err)
		}
		issues := repo.ValidatePackageReferences(pkg, index)
		if len(issues) != 0 {
			t.Fatalf("expected no missing references in generated package, got: %#v", issues)
		}

		pkgMeta, err := metadata.LoadPackageMetadata("dt-service-onboarding", repoPath)
		if err != nil {
			t.Fatalf("failed to load generated package metadata: %v", err)
		}
		if pkgMeta.SourceName != "test-source" {
			t.Fatalf("package metadata source_name = %q, want %q", pkgMeta.SourceName, "test-source")
		}
		if pkgMeta.SourceID != "src-test" {
			t.Fatalf("package metadata source_id = %q, want %q", pkgMeta.SourceID, "src-test")
		}
		if pkgMeta.SourceType != string(source.Local) {
			t.Fatalf("package metadata source_type = %q, want %q", pkgMeta.SourceType, string(source.Local))
		}
		if pkgMeta.SourceURL != "file://"+sourceDir {
			t.Fatalf("package metadata source_url = %q, want %q", pkgMeta.SourceURL, "file://"+sourceDir)
		}
	})
}
