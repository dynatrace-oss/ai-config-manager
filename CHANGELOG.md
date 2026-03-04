# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [3.0.0] - 2026-03-04

### Breaking Changes
- **Strict source format prefixes required** — `aimgr repo add` no longer infers GitHub from bare `owner/repo` or local paths from `./path`. All sources must use explicit format prefixes:
  - `gh:owner/repo` — GitHub shorthand (with optional `@ref` and `/subpath`)
  - `local:path` — Local directories
  - `https://` / `http://` — Any Git host (GitHub, GitLab, Bitbucket, self-hosted)
  - `git@host:owner/repo.git` — SSH URLs
- **Module path migrated** — Go module path changed from `github.com/hk9890/ai-config-manager` to `github.com/dynatrace-oss/ai-config-manager`

### Added
- **`.git/` subpath delimiter** — Generic HTTPS URLs now support subpath extraction via `.git/` delimiter (e.g., `https://host/path/repo.git/subpath` splits into clone URL + subpath)
- **Actionable error messages** — Invalid source formats now show exactly what format to use, with contextual suggestions based on the input pattern

### Fixed
- **E2E test binary build path** — E2E tests now build from `./cmd/aimgr/` after main.go relocation
- **Binary name from `go install`** — Moved `main.go` to `cmd/aimgr/` so `go install` produces `aimgr` binary (not `ai-config-manager`)

### Changed
- **Help text rewritten** — `aimgr repo add --help` now shows complete format guide with all 4 source categories
- **Error passthrough** — Parser errors are passed through directly without redundant wrapping
- **Documentation updated** — All `aimgr repo add` examples across 15+ files updated to use explicit prefixes


## [2.10.0] - 2026-03-03

### Added
- **Package visibility in `aimgr list`** — Installed packages from `ai.package.yaml` now appear in `aimgr list` output alongside commands, skills, and agents, showing resource count, sync status, and description. Supports filtering with `aimgr list "package/*"`

### Changed
- **ai-resource-manager skill restructured** — Slimmed SKILL.md to lightweight router, trimmed discover guide


## [2.9.0] - 2026-03-01

### Added
- **`--prune-package` flag for `aimgr repair`** — Repair command now supports `--prune-package` to remove packages during repair operations

### Fixed
- **Shell completions** — Deprecated `--fix` flags removed from completions; added missing completions for repair commands
- **Verify help text** — `verify` command help text and hints now recommend using `repair` instead of the deprecated `--fix` flag
- **Error messages for deprecated flags** — Corrected error messages that incorrectly referenced deprecated flags and non-existent config keys
- **Install saves to manifest** — `aimgr install` now correctly saves already-installed resources to the manifest
- **5 broken tests resolved** — Fixed broken tests across logging, completions, verify, and parser packages
- **Lint errors resolved** — Fixed `unparam` lint errors in `repo_repair_test.go` and resolved `golangci-lint` failures (goconst, gocyclo, staticcheck)

### Changed
- **Integration tests for repair** — Added comprehensive integration tests for `aimgr repair` and `repo repair` commands
- **Documentation structure** — Added standard doc structure and slimmed AGENTS.md to focus on routing/reference


## [2.8.0] - 2026-03-01

### Fixed
- **Orphaned metadata cleanup** — `repo remove` now handles cases where resource file is gone but metadata persists, preventing "resource not found" errors during sync cleanup
- **Verify subdirectory scanning** — `verify` now scans subdirectories for nested commands and detects orphaned resources
- **List legend labels** — Improved list legend labels and shortened broken status display
- **Package verify output** — Named missing resources in package verify output  
- **Command references** — Corrected 'aimgr project verify' references to 'aimgr verify' in sync/remove warning messages
- **Test fixes** — Corrected TestListEnhanced tests to use listInstalledCmd


## [2.7.0] - 2026-02-28

### Added
- **Multi-argument describe** — `aimgr repo describe` now accepts multiple patterns/arguments (e.g., `aimgr repo describe skill/* command/*`)
- **Package-aware sync status** — `aimgr list --installed` now correctly shows sync status for resources installed as part of a package
- **Namespaced resource scanning** — Installer now detects resources in subdirectories (namespaced resources) during listing

### Fixed
- **golangci-lint v2 compliance** — Resolved all violations across 44 files, including a real bug where error variable shadowing in install.go silently lost errors
- **Shell completion** — Single-argument commands no longer suggest completions after the first argument
- **Install result accuracy** — Package install results now report correct resource type and name
- **Duplicate manifest update removed** — Package installation no longer double-updates the manifest

### Changed
- **CI updated** — golangci-lint v2 now used in GitHub Actions workflow
- **Linter config modernized** — `.golangci.yml` updated for v2 with strict rules enabled


## [2.6.1] - 2026-02-22

### Fixed
- **Multi-resource package install** — `aimgr install skill/x package/y` now works correctly. Previously packages only worked when passed as a single argument.
- **Multi-resource package uninstall** — Same fix applied to uninstall command

### Added  
- **Tool-specific field mappings** — Frontmatter fields are now mapped to tool-specific equivalents during installation

### Changed
- **Documentation restructured** — Contributor guide reorganized into clear sections (architecture, code style, testing, release process)
- **Fixed broken documentation links** — README links to contributor guide corrected


## [2.6.0] - 2026-02-19

### Added
- **Hash-based source IDs** — Deterministic, collision-resistant source identity for reliable tracking across renames (`pkg/repomanifest/sourceid.go`)
- **Source ID threading** — Source IDs flow through `repo add` and `repo sync` pipelines for consistent tracking
- **Sync orphan cleanup** — `repo sync` now detects and removes resources that were deleted from their source
- **Cross-source collision detection** — Sync warns when resources from different sources would collide
- **Remove by source** — `repo remove --source` removes all resources from a specific source
- **Broken symlink warnings** — `repo remove` warns before removing resources with active project symlinks
- **Broken symlink detection** — `list` shows resource health indicator (ok/broken)
- **Broken symlink recovery** — `verify --fix` reinstalls recoverable broken resources
- **E2E tests** — Add/remove cycles, sync lifecycle, source rename scenarios
- **Comprehensive broken symlink tests** — Unit tests for broken symlink detection across all resource types

### Changed
- Manifest now stores source ID alongside source name for reliable identity
- `repo sync` tracks resource-to-source mapping and cleans up orphans
- `repo remove` checks for active project symlinks before removing


## [2.5.0] - 2026-02-17

### Added
- **Codecov integration** - Comprehensive code coverage tracking with Codecov service
- **golangci-lint in CI** - Automated linting checks in GitHub Actions workflow
- **Comprehensive test coverage** - Added 1000+ lines of tests covering:
  - 8 previously untested cmd files (clean, completion, project verify, repo init/verify, root)
  - Package metadata functionality
  - Agent integration tests

### Changed
- **Code organization** - Refactored `pkg/repo/manager.go` (1800+ lines) into focused, maintainable files:
  - `add.go` - Resource addition operations
  - `remove.go` - Resource removal operations  
  - `bulk.go` - Bulk import operations
  - `list.go` - Resource listing
  - `package.go` - Package management
  - `fileops.go` - File operations
  - `git.go` - Git operations
- **Discovery logic** - Consolidated duplicate traversal logic across discovery functions
- **Strategy pattern** - Unified Add* methods using strategy pattern for cleaner code

## [2.4.0] - 2026-02-17

### Added
- **`aimgr verify` command** - Verify repository integrity by checking for broken symlinks, orphaned metadata, and missing resources
- **`aimgr clean` command** - Clean broken symlinks and orphaned metadata files with optional dry-run mode

### Changed
- **BREAKING**: `aimgr uninstall` now requires explicit resource arguments (safety improvement to prevent accidental bulk uninstalls)
- **BREAKING**: `aimgr uninstall` now modifies `ai.package.yaml` by default when uninstalling (use `--no-save` to opt-out)
- Enhanced `aimgr uninstall` to automatically clean up orphaned metadata files after uninstallation


## [2.3.0] - 2026-02-16

### Added
- **DEBUG logging system** - Comprehensive logging across all operations
  - New `--log-level` flag (DEBUG, INFO, WARN, ERROR)
  - DEBUG logging for validation, discovery, install, sync, and repo operations
  - ERROR logging for critical failures in repo operations
  - Orphan detection logging for better troubleshooting
- **JSON standardization** - All JSON output now uses camelCase field names for consistency
- **E2E sync tests** - Added end-to-end integration tests for sync operations

### Fixed
- **Broken symlink detection** - Install now detects and replaces broken symlinks instead of failing
- **Git auto-commit issues** - Fixed metadata and manifest changes not being committed to git
  - `repo add` now commits manifest changes
  - `repo remove` now commits both manifest and metadata changes
  - `repo sync` now commits metadata changes
- **Repo remove bugs** - Fixed packages not being removed and name mismatch issues
- **Autocomplete** - Added source name completion for `repo remove` command
- **CI stability** - Configured git identity for integration tests

### Changed
- Enhanced error reporting with structured logging levels
- Improved debugging experience with comprehensive DEBUG output


## [2.2.1] - 2026-02-15

### Documentation
- Fixed command references and broken links across all documentation
- Drastically simplified README.md (from 2.7k to 128 lines)
- Removed untested Fish and PowerShell completion instructions
- Fixed documentation validation issues
- Refactored contributor documentation structure
- Consolidated and simplified documentation structure


## [2.2.0] - 2026-02-15

### Added
- **Go 1.25.6 enforcement** - Using mise to ensure consistent Go version across all environments
- **E2E test coverage** - Added tests for SyncEmptyConfig and SyncInvalidSource scenarios

### Fixed
- **E2E test reliability** - Tests now handle lowercase JSON fields for resources vs uppercase for packages
- **E2E test modernization** - Updated tests to use ai.repo.yaml instead of obsolete config sync.sources
- **Test isolation** - Tests now properly isolated from user's global config
- **Configuration handling** - Config is now optional with sensible defaults
- **CI stability** - Resolved flaky TestCLIRepoVerifyFixFlag test
- **Error handling** - Comprehensive cleanup of errcheck violations
- **Nested command support** - Manifest validation now properly handles nested commands

### Changed
- **Linting approach** - Removed golangci-lint, now using go vet for consistency
- **CI tooling** - Upgraded to golangci-lint-action v7 for v2 support

### Documentation
- Added session summaries for CI consistency, config defaults, and test isolation fixes


## [1.23.0] - 2026-02-13

### Added
- **Package support in `repo describe` command** - Complete implementation for describing packages
  - Works with all output formats: table, JSON, YAML
  - Pattern matching support: `aimgr repo describe 'package/*'`
  - Shows package name, description, resource count, and full resource list
  - Displays complete metadata (source, timestamps, original format)
  - Integrated with existing describe infrastructure

### Security
- **Agent Safety Rules for AI Resource Manager Skill** - Added prominent safety guidelines
  - New "Agent Safety Rules" section at top of skill documentation
  - Clear categorization of mutating vs read-only operations
  - Explicit requirement: "Never assume permission. Always ask first."
  - Warning labels on all destructive commands throughout skill
  - Agent safety notes in Use Cases 2 and 3
  - Prevents AI agents from running destructive operations without user approval

### Fixed
- **Column widths in `repo verify`** - Adjusted column widths in missing refs table for better readability
- **Build documentation** - Fixed references to removed `install.sh` script, updated to proper build methods
- **OpenCode plugin support** - Added OpenCode as valid tool option in repository init

### Documentation
- **AI Resource Manager Skill improvements**
  - Shortened Use Case 3 (Validation) from 44 to 28 lines while maintaining essential content
  - Removed misleading "If validation passes, add it" example
  - Added safety warnings and agent notes throughout
  - Better structure with consistent use case lengths
- **Updated build instructions** - All docs now reference `go install` or `make install`

### Changed
- Consolidated repository initialization logic for better maintainability
- Improved error messages for unsupported tools



## [1.20.0] - 2026-02-05

### Added
- **VSCode / GitHub Copilot support** - Added full support for VSCode with GitHub Copilot
  - New tool constants: `Copilot` and `VSCode` (alias)
  - Install skills with `--tool=copilot` or `--tool=vscode`
  - Skills install to `.github/skills/` directory
  - Multi-tool support: `--tool=claude,opencode,copilot`
  - Tool detection finds `.github/skills/` directories
  - Copilot supports skills only (commands and agents not supported)
  - Comprehensive integration tests for full workflow
  - Updated documentation with VSCode/Copilot examples

### Documentation
- **Architecture documentation enhanced**
  - Added Rule 5: Symlink Handling for Filesystem Operations
  - Comprehensive symlink best practices
  - Test utilities for symlink testing (`test/testutil/symlinks.go`)
  - Updated AGENTS.md with symlink guidelines
  - Examples throughout user documentation

### Fixed
- **Test compilation error** - Fixed `repo_describe_test.go` missing format parameter

## [1.15.0] - 2026-01-29

### Fixed
- **Repo sync idempotency** - Fixed critical bug where `repo sync` incorrectly reported existing commands as "Added"
  - Commands now correctly show "Updated" status when already present in repository
  - Fixed duplicate existence checks that caused incorrect status reporting
  - Improved test coverage with CLI-based integration tests
  - Ensures accurate reporting for idempotent sync operations

### Documentation
- **Major documentation restructure** - Reorganized documentation for better discoverability
  - Added `docs/user-guide/getting-started.md` - Comprehensive getting started guide
  - Added `docs/user-guide/testing.md` - Testing guide consolidated from TEST_ISOLATION.md
  - Created `docs/architecture/` structure for architectural documentation
  - Created `docs/planning/archive/` for historical planning documents
  - Streamlined AGENTS.md for AI agent workflows

### Changed
- **Repository cleanup** - Prepared repository for public release
  - Updated .gitignore for build artifacts
  - Moved dev-completion.sh to scripts/ directory
  - Archived historical planning documents
  - All beads issues resolved

## [1.14.0] - 2026-01-29

### Breaking Changes
- **Removed `aimgr repo update` command** - Command has been fully removed from the codebase
  - Users should use `aimgr repo import` or `aimgr repo sync` instead
  - All documentation updated to reflect this change
  - Tests and references cleaned up

### Added
- **Auto-detection of base path in LoadCommand** - Commands can now be loaded without explicitly specifying base path
  - `LoadCommand` automatically finds nearest `commands/` directory
  - Simplified API for command loading
  - See deprecated `LoadCommandWithBase` for migration path

### Fixed
- **Discovery parsing prevention** - Discovery now skips files outside resource directories
  - Prevents incorrect parsing of documentation, node_modules, etc.
  - Fixes issue where `.md` files in wrong locations were incorrectly identified as agents
  - Discovery now only looks in proper resource directories (agents/, commands/, skills/)

- **Error reporting improvements**
  - Discovery errors now properly reported for commands and agents
  - Prevents metadata creation for failed imports
  - Better error messages for malformed resources

- **Test suite cleanup**
  - Removed orphaned test `TestCLIMetadataUpdatedOnUpdate`
  - Fixed test fixtures to use proper `commands/` directory structure
  - All tests passing

### Deprecated
- **LoadCommandWithBase** - Use `LoadCommand` instead (auto-detects base path)
  - Scheduled for removal in v2.0.0
- **LoadCommandResourceWithBase** - Use `LoadCommandResource` instead
  - Scheduled for removal in v2.0.0

### Documentation
- Removed all `repo update` references from README
- Updated AGENTS.md with LoadCommand changes
- Updated MIGRATION.md for deprecated functions
- Added comprehensive documentation for consolidation epic

## [1.13.0] - 2026-01-28

### Fixed
- **Nested Command Path Detection** - Fixed multiple bugs in nested command path handling
  - Fixed `DetectType()` to correctly handle nested command and agent paths
  - Fixed `repo Get()` to use `LoadCommandWithBase` for proper nested names
  - Fixed `repo describe` command to support nested paths correctly
  - Fixed `repo verify` nested paths bug
  - Commands now correctly use nested paths in Name field throughout the system
  
- **Import and Display** - Fixed nested command path display issues
  - Fixed display of nested command paths in sync/import output
  - Consolidated import logic to use discovered `.Path` directly as single source of truth
  - Fixed basePath detection from incorrectly using system directories in tests

### Changed
- **Test Infrastructure** - Major test suite refactoring for improved maintainability
  - Migrated to unified fixture service helpers across test suite
  - Refactored tests to use fixture helpers: `repo_verify_test`, `repo_update_batching_test`, CLI integration tests, `filter_test`, `package_test`, `marketplace_test`
  - Added comprehensive test fixtures for nested resources
  - Improved test reliability and reduced code duplication
  - Enhanced integration tests with file and metadata validation

- **LoadCommand API** - Improved command loading with auto-detection
  - `LoadCommand` now auto-detects base path by finding nearest commands/ directory
  - Commands must be in a `commands/` directory (or tool-specific variant like `.claude/commands/`)
  - Clear error messages when commands are not in proper structure
  - Nested command structure automatically preserved

### Deprecated
- **LoadCommandWithBase** - Use `LoadCommand` instead (auto-detects base path)
  - `LoadCommand` now handles base path detection automatically
  - Scheduled for removal in v2.0.0
- **LoadCommandResourceWithBase** - Use `LoadCommandResource` instead
  - Uses `LoadCommand` internally which auto-detects base path
  - Scheduled for removal in v2.0.0

### Testing
- Added integration test for nested command layout verification
- Enhanced integration tests to verify both files and metadata
- Added test helper functions for creating resources with valid names
- Improved test coverage for nested command workflows

## [1.12.0] - 2026-01-27

### Breaking Changes
- **Command Rename** - `aimgr repo add` is now `aimgr repo import`
  - More intuitive name that reflects the operation
  - All documentation and examples updated
  - Test suite updated to use new command name
- **Command Removal** - Removed `aimgr repo create-package` command
  - Users should manually create `.package.json` files instead
  - Simpler workflow with direct file editing
  - See `docs/resource-formats.md` for package format specification

### Added
- **Uninstall All** - New functionality to uninstall all installed resources at once
  - Use `aimgr uninstall` without arguments to remove all resources
  - Prompts for confirmation before proceeding
  - Efficient bulk removal operation
  
- **Nested Command Structure** - Commands can now be organized in nested directories
  - Import commands from subdirectories (e.g., `commands/api/deploy.md`)
  - Repository preserves directory structure: stored as `commands/api/deploy.md`
  - Prevents name conflicts for same filename in different directories
  - Backward compatible with flat command structure
  - Example: `commands/dev/test.md` and `commands/prod/test.md` can coexist
  - Discovery automatically detects nested commands during import
  - Symlinks created with full nested path preserved

### Fixed
- **Nested Command Installation** - Fixed symlink creation to preserve nested structure
  - Installer now creates parent directories as needed
  - Absolute paths correctly preserved during basePath calculation
  - All nested commands install to correct locations in tool directories

### Testing
- Added comprehensive tests for nested command workflows
- Added tests for LoadCommandWithBase and RelativePath functionality
- Integration tests verify end-to-end nested import and installation
- Updated test suite to use `repo import` command name


## [1.11.0] - 2026-01-27

### Performance
- **Test Suite Optimization** - 99%+ improvement (3 minutes → <30 seconds)
  - Unit tests now run in <5 seconds with no network dependency
  - Integration tests are optional with `//go:build integration` tag
  - Removed 14 slow tests that cloned GitHub repositories (1,118 lines)
  - Added 7 committed test fixtures in `testdata/repos/`
  - Added 19 new fast discovery unit tests
  - Added 4 minimal Git integration tests

### Added
- **Test Infrastructure**
  - New `test/testutil` package with test helpers (`GetFixturePath()`, `SkipIfNoGit()`)
  - Test fixtures for skills, commands, agents, mixed resources, and edge cases
  - `test/git_integration_test.go` - Minimal opt-in Git integration tests
  - `test/discovery_skills_test.go` - Fast skills discovery unit tests
  - `test/discovery_commands_test.go` - Fast commands discovery unit tests
  - `test/discovery_agents_test.go` - Fast agents discovery unit tests
  - `test/discovery_mixed_test.go` - Mixed resource discovery tests
  
- **Documentation**
  - `testdata/repos/README.md` - Fixture documentation and usage guide
  - `docs/architecture-rules.md` - Workspace cache architecture requirements
  - Test strategy guide in `docs/test-refactoring.md`
  
- **Package Resources**
  - Imported missing dynatrace-core resources (3 agents, 4 commands)

### Changed
- **Testing Strategy**
  - Unit tests (default): Use fixtures, no network, <5 seconds execution
  - Integration tests (opt-in): Real Git operations, run with `make test-integration`
  - Updated `Makefile` with `test-integration` target
  - Updated AGENTS.md with new testing approach

### Fixed
- **Package Management**
  - Fixed opencode-coder package references to use flat command names
  - Fixed repo verify to skip packages in metadata validation
  - Improved error messages in verification output
  
- **Test Coverage**
  - Added comprehensive test coverage for package validation
  - Maintained coverage at workspace 42.8%, discovery 79.5%

### Removed
- Legacy Git cloning implementation (deprecated temp cloning code)
- 14 slow tests that cloned anthropics/* repositories
- Network connectivity checks in unit tests (`isOnline()` function)

## [1.10.0] - 2026-01-26

### Added
- **Config Precedence System**
  - Project-level `ai.package.yaml` `install.targets` now overrides global config
  - Enables per-project tool configuration without modifying global settings
  - Provides more granular control over installation targets for different projects

### Changed
- **Table Format Improvements**
  - Table format now uses unified `type/name` column instead of separate columns
  - Cleaner, more compact output for resource listings
  - Consistent with pattern matching syntax used throughout CLI
  
- **Documentation Refactoring**
  - Split comprehensive documentation from AGENTS.md into specialized files
  - Restructured AGENTS.md for better clarity and navigation
  - Created detailed standalone documentation for specific topics

### Fixed
- **Test Suite Fixes**
  - Fixed `TestBulkImportConflicts` error handling to match new behavior
  - Improved test reliability and accuracy
  
- **Package Discovery**
  - Fixed recursive package discovery to support nested directories
  - Packages now properly discovered at any depth in directory structure
  
- **Package Validation**
  - Fixed various package validation bugs
  - More robust error checking for package format and content
  - Added comprehensive test coverage for package validation in `repo verify`
  - Validates package resource references against actual repository contents
  
- **Bulk Operations**
  - Improved error handling - continues on validation/resource errors instead of aborting
  - Better resilience when processing multiple resources with some failures
  
- **Dry-Run Mode**
  - Fixed dry-run mode to correctly count resources in JSON output
  - Accurate reporting of what would be affected without making changes
  
- **Install Command**
  - Fixed install command to read global config from XDG location
  - Ensures consistent config loading across all commands

### Documentation
- **New Comprehensive Guides**
  - `docs/pattern-matching.md` - Complete pattern matching guide with examples (458 lines)
  - `docs/workspace-caching.md` - Git repository caching documentation (273 lines)
  - `docs/resource-formats.md` - Complete resource format specifications (810 lines)
  - `docs/output-formats.md` - CLI output formats documentation
  
- **Documentation Organization**
  - Historical release notes archived to `docs/archive/release-notes/`
  - Cleaned up and audited `docs/` folder structure
  - Improved discoverability of specialized documentation

## [1.9.0] - 2026-01-26

### Added
- **Output Format Options**
  - New `--format` flag for bulk operation commands: `table` (default), `json`, `yaml`
  - Structured JSON/YAML output for scripting and automation
  - Applies to: `repo add`, `repo sync`, `repo update`, `list`
  - JSON output includes detailed error information for programmatic handling
  - See `docs/output-formats.md` for comprehensive documentation and examples
  
- **Workspace Cache Integration**
  - `repo add` now uses workspace caching for Git repositories (10-50x faster on reuse)
  - `repo sync` leverages cached repositories for efficient multi-source sync
  - First operation clones, subsequent operations reuse cache
  - Automatic batching: resources from same repository share one cached clone
  
- **Enhanced Error Reporting**
  - Structured error messages with clear categorization
  - Deduplicated discovery error messages for cleaner output
  - Better validation error messages for missing/invalid fields
  - Improved GitHub reference handling (empty ref defaults to default branch)

### Changed
- Repository commands now provide consistent, structured output across formats
- Success messages standardized across `repo add`, `repo sync`, `repo update`
- Git operations automatically use workspace cache when available

### Fixed
- Added `git pull` to repo sync for cached repositories to ensure up-to-date sources
- Fixed duplicate discovery error messages during bulk imports
- Fixed empty ref handling for GitHub repos (now defaults to repository's default branch)
- Improved workspace cache test reliability

### Documentation
- Added comprehensive output format documentation (`docs/output-formats.md`)
- Updated AGENTS.md with output format usage patterns
- Added JSON parsing examples with jq and Go
- Documented error structure for programmatic handling

### Testing
- Added 627 lines of integration tests for output formats
- Added 344 lines of integration tests for workspace cache in add/sync
- Added 77 lines of workspace cache tests
- All tests pass with comprehensive coverage

## [1.8.0] - 2026-01-25

### Added
- **Package System**
  - New package resource type for bundling multiple resources together
  - Packages are JSON files that reference commands, skills, and agents
  - Install all resources in a package with a single command: `aimgr install package/web-tools`
  - Package discovery with `aimgr repo add` auto-discovers `*.package.json` files
  - Create packages with `aimgr repo create-package` command
  - Pattern matching support: `package/*`, `package/web-*`
  - Integrated into all core commands (list, install, uninstall, repo commands)
  
- **Marketplace Import**
  - Import Claude plugin marketplaces with `aimgr marketplace import` command
  - Auto-discovers resources from plugin directories (commands, skills, agents)
  - Generates aimgr packages from marketplace plugins automatically
  - Supports local paths and GitHub URLs (e.g., `gh:owner/repo/.claude-plugin/marketplace.json`)
  - Resource discovery searches standard locations: `commands/`, `skills/`, `agents/`, `.claude/`, `.opencode/`
  - Filter plugins during import with `--filter` flag
  - `--dry-run` and `--force` options for safe imports
  
- **Project Manifests (ai.package.yaml)**
  - New `ai.package.yaml` manifest file for declarative project dependencies (similar to npm's package.json)
  - Zero-argument install: `aimgr install` reads manifest and installs all resources
  - Auto-save by default: `aimgr install skill/test` adds to manifest automatically
  - `--no-save` flag to skip manifest updates
  - `aimgr init` command creates new manifest file
  - Optional `targets` field to override default install targets per-project
  - Enables consistent AI tooling across team members
  
- **Auto-Discovery Enhancements**
  - `aimgr repo add` now discovers packages from `packages/` directory
  - Marketplace auto-discovery: finds `marketplace.json` files in `.claude-plugin/` directories
  - Filter support: `--filter "package/*"` or `--filter "web-*"` to selectively import
  - Works with local paths, Git URLs, and GitHub shortcuts

### Changed
- **Install Command Improvements**
  - Zero-argument mode: `aimgr install` reads `ai.package.yaml` if present
  - Auto-save behavior: resources automatically added to manifest (disable with `--no-save`)
  - Supports installing packages: `aimgr install package/web-tools`
  - Better error messages when manifest not found
  
- **Repository Management**
  - `repo add` now supports package discovery alongside commands, skills, and agents
  - Enhanced bulk import with marketplace and package support
  - Improved resource counting and progress reporting

### Documentation
- Added comprehensive Package System section to AGENTS.md
- Added Marketplace Format documentation with examples
- Added Project Manifests section with workflows and best practices
- Updated README with package and manifest examples
- Added marketplace example files in `examples/marketplace/`
- Added ai.package.yaml examples in `examples/ai-package/`

### Testing
- Added 28 integration tests for ai.package.yaml workflows (975 lines)
- Added 18 integration tests for marketplace import (853 lines)
- Added 15 integration tests for package auto-import (838 lines)
- Added 11 unit tests for marketplace parser (714 lines)
- Added 22 unit tests for marketplace generator (893 lines)
- Added 13 unit tests for package discovery (528 lines)
- Added 11 unit tests for manifest package (485 lines)
- All tests pass with comprehensive edge case coverage

### Fixed
- Package filter support in pattern matcher now handles `package/*` patterns correctly
- Autocomplete support for package resources in install command


## [1.6.0] - 2026-01-25

### Added
- **Workspace caching for Git repositories**
  - Git repositories are now cached in `.workspace/` directory for reuse across updates
  - First update clones the full repository, subsequent updates only pull changes
  - Dramatically improves update performance: subsequent updates 10-50x faster
  - Caches are automatically managed and shared across resources from the same source
  - Each repository is stored by SHA256 hash of normalized URL for collision-free storage
  - Cache metadata tracked in `.cache-metadata.json` for quick lookups
- **`aimgr repo prune` command**
  - Removes Git repository caches from `.workspace/` that are no longer referenced
  - Frees disk space from outdated or unused Git clones
  - Shows detailed list of caches to be removed with sizes
  - Interactive confirmation with `--force` and `--dry-run` options
  - Run after removing many resources to reclaim disk space
- **Progress output for `repo update` command**
  - Shows total resource count at start (e.g., "Updating 15 resources...")
  - Displays `[N/M]` counter for each resource being updated
  - Shows operation type during execution ("Cloning from..." or "Updating from local source...")
  - Provides immediate inline feedback with status symbols (✓, ✗, ⊘)
  - No more silent pauses during long git clone operations
  - Works with all update modes (all/pattern/single resource) and `--dry-run` flag

### Changed
- **Standardized pattern syntax across all commands**
  - All commands now use `type/pattern` syntax (e.g., `skill/pdf*`, `command/test-*`)
  - Consistent pattern matching with glob support (`*`, `?`, `[abc]`, `{a,b}`)
  - Applies to: `repo list`, `repo describe`, `repo update`, `repo remove`, `list`, `uninstall`
  - Backward compatible: bare names still work (e.g., `my-skill` → searches all types)
  - Type prefix required for ambiguity: `skill/my-skill` vs `command/my-skill`

### Improved
- **Optimized `repo update` with Git repository batching**
  - Resources from the same Git repository now share a single clone operation
  - Dramatically improves update speed for bulk operations from the same source
  - Example: Updating 39 skills from one repository now requires 1 clone instead of 39
  - Batching is automatic and requires no user configuration
  - Batch progress displayed as "Batch: Updating N resources from <url>"
- Created shared pattern expansion utility for consistent behavior across commands
- Refactored command implementations to use centralized pattern matching logic
- Enhanced user experience with real-time feedback during long operations

### Fixed
- Updated integration tests to use new pattern syntax
- Ensured all commands handle pattern matching consistently

## [1.5.0] - 2026-01-24

### Added
- **New `aimgr repo sync` command** for declarative multi-source repository synchronization
  - Reads source URLs from config file (`~/.config/aimgr/aimgr.yaml`)
  - Supports importing from multiple repositories in a single operation
  - Per-source filtering with glob patterns (e.g., `skill/*`, `agent/beads-*`)
  - Defaults to force-overwrite behavior (like external `aimgr-init` scripts)
  - Supports `--skip-existing` and `--dry-run` flags
  - Replaces need for external initialization scripts
- New `sync.sources` configuration section in aimgr.yaml
  - Each source can have optional filter pattern
  - Supports GitHub URLs, Git URLs, and local paths
  - Pattern matching using glob syntax (`*`, `?`, `[abc]`, `{a,b}`)

### Changed
- Refactored `addBulkFromLocal` and `addBulkFromGitHub` to accept filter parameters
- Improved filter handling to support both CLI flags and config-based filters

### Documentation
- Added comprehensive "Repository Sync" section to README
- Documented config file format with multiple examples
- Added migration guide from external scripts to declarative config
- Included per-source filtering examples and use cases

### Testing
- Added 9 comprehensive integration tests for sync command
- Added 13 unit tests for sync config validation (85.7% coverage)
- All tests pass with no regressions

## [1.4.0] - 2026-01-24

### Fixed
- **[CRITICAL]** Fixed P0 bug where Git source metadata stored temporary clone paths instead of original URLs
  - Resources from GitHub/Git URLs now correctly store source URLs in metadata
  - Enables successful updates for Git-sourced resources
  - Affects all resources added from Git sources since v1.0.0

### Added
- New `aimgr repo prune` command to remove orphaned metadata entries
  - Interactive confirmation with detailed list of orphaned entries
  - Supports `--dry-run` and `--force` flags
  - Focuses on local/file sources (Git sources are transient)
- New `aimgr repo verify` command for repository health checks
  - Detects resources without metadata, orphaned metadata, missing sources, type mismatches
  - Supports `--fix` flag for automatic resolution
  - Supports `--json` flag for machine-readable output
  - Returns appropriate exit codes for CI/CD integration
- Added `AIMGR_REPO_PATH` environment variable for test isolation
- Added `TEST_ISOLATION.md` documentation for test patterns

### Changed
- Improved `aimgr repo update` UX for handling missing source paths
  - Continues updating all resources even when some sources are missing
  - Distinguishes between skipped (⊘) and failed (✗) resources  
  - Provides actionable hints to resolve issues
  - Separates warnings from errors in summary

### Testing
- Added 25+ comprehensive integration tests for new commands
- All tests now use isolated temporary repositories
- Tests no longer pollute user's actual repository
- Can safely run tests in parallel

## [1.3.0] - 2026-01-24

### Added
- Metadata reorganization into cleaner `.metadata/` directory hierarchy
- Pattern matching support for install and uninstall commands
  - Supports glob operators: `*`, `?`, `[abc]`, `{a,b}`
  - Type-specific patterns: `skill/pdf*`, `command/*test*`
  - Cross-type patterns: `*review*`
- Unified `repo add` command with auto-discovery
  - Replaces separate `add-command`, `add-skill`, `add-agent` commands
  - Supports `--filter` flag for pattern-based filtering
  - Auto-discovers from `.claude/`, `.opencode/` directories

### Fixed
- Fixed agent detection for minimal agents (only `description` field)
- Fixed bulk import conflict handling due to incorrect type detection

### Changed
- Backward compatibility for deprecated `default-tool` configuration key

## [1.2.0] - 2026-01-23

### Added
- New `aimgr repo add bulk <folder|url>` command for auto-discovering and importing all resource types from a directory
  - Automatically detects commands, skills, and agents in standard locations (.claude/, .opencode/, etc.)
  - Simplifies bulk imports - no need to specify resource types individually
  - Supports Claude folders, OpenCode folders, and plugin directories
- Enhanced shell completion support:
  - Autocomplete for `aimgr list` with resource type filtering
  - Autocomplete for `aimgr config` with configuration keys
  - Autocomplete for `aimgr repo update` with resource names

### Changed
- Removed redundant commands in favor of unified bulk add:
  - Removed `aimgr repo add opencode` (use `aimgr repo add bulk` instead)
  - Removed `aimgr repo add claude` (use `aimgr repo add bulk` instead)
  - Removed `aimgr repo add plugin` (use `aimgr repo add bulk` instead)
- Updated README with simplified bulk import workflow
- Improved discovery logic to avoid duplicate detection in commands/skills subdirectories

### Fixed
- Discovery now correctly skips nested directories to prevent duplicate resource detection

## [1.1.0] - 2026-01-23

### Added
- Shell completion support for Bash, Zsh, Fish, and PowerShell
- Dynamic resource name completion for install commands
- Completion helper functions for resource types and config keys
- Shell completion troubleshooting guide

### Changed
- Enhanced CLI with completion flags
- Improved user experience with tab completion

## [1.0.0] - 2026-01-22

### Added
- Initial stable release
- Repository management for AI resources
- Multi-tool support (Claude Code, OpenCode, GitHub Copilot)
- Agent resource support with OpenCode and Claude formats
- Command and skill management
- Symlink-based installation
- Format validation and type safety
- Cross-platform support (Linux, macOS)
- Configuration management with XDG base directory support

[3.0.0]: https://github.com/dynatrace-oss/ai-config-manager/compare/v2.10.0...v3.0.0
[1.4.0]: https://github.com/dynatrace-oss/ai-config-manager/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/dynatrace-oss/ai-config-manager/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/dynatrace-oss/ai-config-manager/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/dynatrace-oss/ai-config-manager/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/dynatrace-oss/ai-config-manager/releases/tag/v1.0.0

[1.5.0]: https://github.com/dynatrace-oss/ai-config-manager/compare/v1.4.0...v1.5.0
[1.8.0]: https://github.com/dynatrace-oss/ai-config-manager/compare/v1.7.0...v1.8.0
