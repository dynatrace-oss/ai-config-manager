# Getting Started with aimgr

**aimgr** manages AI resources (commands, skills, agents) across multiple AI coding tools. It uses a centralized repository with symlink-based installation.

**Key concept:** Sources (tracked in `ai.repo.yaml`) provide resources, which you install into projects.

---

## Quick Start

```bash
# 1. Initialize your repository
aimgr repo init

# 2. Add a source (local directory or GitHub repo)
aimgr repo add gh:your-org/ai-tools

# 3. View what's available
aimgr repo list

# 4. Install resources into your project
cd ~/my-project
aimgr install skill/pdf-processing
```

### Bootstrap from a shared team manifest (recommended for teams)

If your team agrees on shared sources for a project, publish that source set as one central `ai.repo.yaml` and have developers/CI bootstrap from that URL:

```bash
# Shared manifest URL
aimgr repo apply-manifest https://example.com/team/project-a/ai.repo.yaml

# Pinned/tagged GitHub raw file URL (stable baseline)
aimgr repo apply-manifest https://raw.githubusercontent.com/your-org/team-configs/v1.2.0/manifests/project-a.ai.repo.yaml

# Or a checked-out local copy of the same manifest
aimgr repo apply-manifest ./ai.repo.yaml
```

`repo apply-manifest` will auto-initialize a fresh repository when needed, then merge sources into local `ai.repo.yaml`.

Recommended bootstrap flow for local onboarding:

```bash
aimgr repo apply-manifest <shared-manifest-url>
aimgr repo sync
aimgr install
```

Recommended bootstrap flow for CI (same commands, non-interactive):

```bash
aimgr repo apply-manifest <shared-manifest-url>
aimgr repo sync
aimgr install
```

Important URL constraint for GitHub-hosted manifests:
- supported: URLs that fetch the manifest file content directly (for example `raw.githubusercontent.com/.../ai.repo.yaml`)
- not supported: GitHub web page URLs such as `https://github.com/<owner>/<repo>/blob/<ref>/.../ai.repo.yaml` or `/tree/<ref>/...`

Important workflow:
- a team publishes one shared `ai.repo.yaml` as the project's baseline
- users and CI apply that shared manifest into their own local repo
- users can apply multiple manifests over time, and aimgr merges all sources into the same local `ai.repo.yaml`
- if someone wants to publish their own current setup, they can use `aimgr repo show-manifest` and commit that output somewhere shareable

For example, a user can publish their local manifest like this:

```bash
aimgr repo show-manifest > ai.repo.yaml
# commit that file to a repo, wiki, gist, or shared config location
```

Stdin support exists as a convenience for advanced workflows, but it is not the primary sharing model.

---

## Installation

Use the one-line installer for your shell:

```bash
curl -fsSL https://raw.githubusercontent.com/dynatrace-oss/ai-config-manager/main/scripts/install.sh | sh
```

```powershell
# Run from a PowerShell prompt (PowerShell 5.1+ or pwsh 7+)
irm https://raw.githubusercontent.com/dynatrace-oss/ai-config-manager/main/scripts/install.ps1 | iex
```

The installer downloads the correct release asset for your OS and CPU architecture automatically. Set `AIMGR_VERSION` to install a specific release or `AIMGR_INSTALL_DIR` to override the install location.

Verify installation:
```bash
aimgr --version
```

---

## First Steps

### 1. Configure Your AI Tool

Tell aimgr which AI tool(s) you're using:

```bash
aimgr config set install.targets claude          # Claude Code
aimgr config set install.targets opencode        # OpenCode
aimgr config set install.targets copilot         # VSCode / GitHub Copilot
aimgr config set install.targets claude,opencode # Multiple tools
```

### 2. Initialize Your Repository

```bash
aimgr repo init
```

This creates the repository directory and `ai.repo.yaml` manifest.

If you already have a shared manifest, use `aimgr repo apply-manifest <path-or-url>` instead.

### 3. Add Sources

Add a local directory:
```bash
aimgr repo add local:~/.claude/           # Existing tool directory
aimgr repo add local:~/my-resources/      # Your own resources
```

Add from GitHub:
```bash
aimgr repo add gh:your-org/ai-tools
aimgr repo add gh:your-org/ai-tools --filter "skill/*"  # Filtered
aimgr repo add gh:your-org/ai-tools@v1.0.0          # Specific version
```

**Note:** Local sources are symlinked (live editing). GitHub sources are copied.

### 4. View Your Resources

```bash
aimgr repo info              # View sources and status
aimgr repo list              # List all resources
aimgr repo list skill        # List only skills
```

---

## Core Commands

### Source Management

```bash
# Add sources
aimgr repo add local:~/my-resources/          # Local (symlinked)
aimgr repo add gh:owner/repo            # GitHub (copied)
aimgr repo add gh:owner/repo@v1.0.0     # Specific version

# Import one or more shared manifests into local ai.repo.yaml
aimgr repo apply-manifest ./ai.repo.yaml
aimgr repo apply-manifest https://example.com/platform/ai.repo.yaml

# Show the current local ai.repo.yaml
aimgr repo show-manifest

# Temporarily override a remote source with a local checkout for dev/testing
aimgr repo override-source team-tools local:~/dev/team-tools
aimgr repo info
aimgr repo override-source team-tools --clear

# Update from sources
aimgr repo sync

# View sources
aimgr repo info

# Remove a source
aimgr repo remove source-name
```

Notes for override workflows:

- `repo info` shows whether a source is currently overridden and where it restores to.
- `repo show-manifest` prints shareable baseline source definitions (safe to commit/share) and does not include local override breadcrumb state.
- If a source is already overridden, a second `repo override-source` attempt is rejected; run `aimgr repo override-source <name> --clear` first.
- `repo override-source` does not support overriding sources that are already local path sources.

See [sources.md](sources.md) for detailed source management.

### Project Usage

Install resources into your project:
```bash
cd ~/my-project
aimgr install skill/pdf-processing
aimgr install command/my-command agent/code-reviewer  # Multiple
aimgr install "skill/*"                               # Pattern
```

List installed resources:
```bash
aimgr list
```

Uninstall resources (removes from project, keeps in repository):
```bash
aimgr uninstall skill/pdf-processing
```

**Packages** install multiple related resources at once:
```bash
aimgr install package/web-dev-tools
```

### Verify and Repair

Check your project for installation issues:
```bash
aimgr verify                              # Diagnose issues (read-only)
aimgr repair                              # Reconcile owned dirs to ai.package.yaml
aimgr repair --prune-package              # Also clean invalid manifest references
aimgr repair --dry-run                    # Preview planned reconcile actions
aimgr clean && aimgr repair               # Wipe owned dirs, then restore from manifest
```

Notes:
- `aimgr clean` now empties owned resource directories without confirmation flags.
- `aimgr repair` reinstalls declared resources that were manually removed.
- Use `aimgr uninstall <resource>` (without `--no-save`) for permanent removal.
- Use `--dry-run` to preview repair actions.

Fix repository metadata:
```bash
aimgr repo repair                         # Fix missing/orphaned metadata
aimgr repo repair --dry-run               # Preview what would be fixed
```

See [Repairing Resources](repair.md) for the complete guide.

### Repository Management

List resources in repository:
```bash
aimgr repo list
```

Output includes sync status:
- **checkmark** = In sync with manifest
- **\*** = Installed but not in manifest
- **warning** = In manifest but not installed

Remove a resource from repository:
```bash
aimgr repo remove skill old-skill
aimgr repo remove command test-cmd --force  # Skip confirmation
```

**Warning:** Remove from projects first to avoid broken symlinks.

---

## Useful Options

Preview operations before executing:
```bash
aimgr repo add local:~/resources/ --dry-run
aimgr repo sync --dry-run
```

Overwrite existing resources:
```bash
aimgr repo add local:~/resource/ --force
aimgr install skill/foo --force
```

---

## Next Steps

- **[Sources](sources.md)** - Detailed guide to managing sources in ai.repo.yaml
- **[Repairing Resources](repair.md)** - Fix broken installations and clean up projects
- **[Configuration](configuration.md)** - Repository path, targets, and settings
- **[Pattern Matching](../reference/pattern-matching.md)** - Advanced pattern syntax
- **[Output Formats](../reference/output-formats.md)** - JSON/YAML output for scripting
- **[Resource Validation](../reference/resource-validation.md)** - Validate resources by path or canonical ID before sync/install
- **[Troubleshooting](../reference/troubleshooting.md)** - Common issues and solutions

### Creating Your Own Resources

- **[Supported Tools](../reference/supported-tools.md)** - Tool documentation links
- **[AgentSkills.io](https://agentskills.io/home)** - Community skill format specification

---

## Getting Help

- **Command help:** `aimgr --help` or `aimgr <command> --help`
- **Documentation:** [GitHub Repository](https://github.com/dynatrace-oss/ai-config-manager)
- **Issues:** [Report bugs or request features](https://github.com/dynatrace-oss/ai-config-manager/issues)
