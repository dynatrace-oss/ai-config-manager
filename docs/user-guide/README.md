# User Guide

User-facing documentation for **aimgr** (ai-config-manager), a CLI tool for managing AI resources across multiple AI coding tools.

## Documentation

### [Concepts](concepts.md)

Compact overview of the main aimgr concepts from a user perspective.

**Key Topics:**
- What aimgr does
- `ai.repo.yaml` for sources and syncing
- `ai.package.yaml` for project dependencies
- optional `ai.package.local.yaml` for private project-local overlays
- How repository and project manifests work together

### [Getting Started](getting-started.md)

**Start here if you're new to aimgr!** This guide covers installation, first steps, common operations, and practical workflows.

**Key Topics:**
- Installation on Linux, macOS, and Windows
- Configuring your AI tool targets
- Adding sources with `repo add`
- Team-manifest bootstrap with `repo apply-manifest` + `repo sync` + `install`
- Installing resources into projects
- Common operations and workflows
- Troubleshooting tips

### [Configuration](configuration.md)

Complete guide to configuring aimgr, including repository path, installation targets, and field mappings.

**Key Topics:**
- Config file location (`~/.config/aimgr/aimgr.yaml`)
- Repository path configuration
- Installation targets
- **Field mappings** for tool-specific values (e.g., model names)
- Environment variable interpolation

### [Repairing Resources](repair.md)

Reconcile owned resource directories with `ai.package.yaml`, clean project resource folders safely, and migrate from deprecated workflows.

**Key Topics:**
- `aimgr repair` — reconcile owned directories to the effective project manifest
  (`ai.package.yaml` + optional `ai.package.local.yaml`)
- `aimgr clean` — empty owned resource directories
- `--prune-package` — clean invalid references from `ai.package.yaml` (manifest cleanup)
- `--dry-run` — preview reconcile actions safely
- Migration: `aimgr clean && aimgr repair` replaces old reset/force workflows
- `aimgr repo repair` — fix repository metadata issues
- Migrating from deprecated `verify --fix`

### [Sources](sources.md)

Managing remote and local resource sources using `ai.repo.yaml`.

**Key Topics:**
- `ai.repo.yaml` manifest format
- Adding GitHub repositories (`gh:owner/repo`)
- Adding local paths (symlinked)
- Shared team-manifest publishing/consumption model
- URL bootstrap guidance (including pinned/tagged GitHub raw URLs)
- Syncing resources with `repo sync`
- Development workflows

### [Team and Multi-Project Workflows](team-workflows.md)

Practical patterns for using aimgr in larger team setups and across many projects.

**Key Topics:**
- Central shared-manifest workflow as the default team model
- Team bootstrap flow for developers and CI: `repo apply-manifest` + `repo sync` + `install`
- Canonical naming and collision troubleshooting guidance
- Project-local resource repositories
- Personal extras vs committed project dependencies
- Multi-project workflows with one local aimgr repository

## Quick Start

```bash
# Initialize repository
aimgr repo init

# Add sources
aimgr repo add gh:example/ai-tools
aimgr repo add local:~/my-local-resources

# Install resources to your project
cd ~/my-project
aimgr install skill/code-review
```

## Quick Reference

| Command | Description |
|---------|-------------|
| `aimgr repo init` | Initialize repository |
| `aimgr repo add <source>` | Add source and import resources |
| `aimgr repo sync` | Sync all sources |
| `aimgr repo list` | List all resources in repository |
| `aimgr install <pattern>` | Install resources to project |
| `aimgr uninstall <pattern>` | Uninstall resources from project |
| `aimgr verify` | Check installation health |
| `aimgr repair` | Fix broken installations |
| `aimgr repo repair` | Fix repository metadata |

## See Also

For more detailed technical information:

- **[Reference Documentation](../reference/)** - Pattern matching, output formats, supported tools
- **[Internals](../internals/)** - Repository layout, workspace caching, git tracking
- **[Supported Tools](../reference/supported-tools.md)** - Tool support and resource format documentation

For contributing to aimgr:

- **[Contributor Guide](../contributor-guide/)** - Development setup and guidelines
- **[CONTRIBUTING.md](../../CONTRIBUTING.md)** - How to contribute
