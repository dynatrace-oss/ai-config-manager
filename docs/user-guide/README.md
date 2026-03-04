# User Guide

User-facing documentation for **aimgr** (ai-config-manager), a CLI tool for managing AI resources across multiple AI coding tools.

## Documentation

### [Getting Started](getting-started.md)

**Start here if you're new to aimgr!** This guide covers installation, first steps, common operations, and practical workflows.

**Key Topics:**
- Installation on Linux, macOS, and Windows
- Configuring your AI tool targets
- Adding sources with `repo add`
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

Diagnose and fix broken installations, clean up unmanaged files, and prune stale manifest entries.

**Key Topics:**
- `aimgr repair` — fix broken symlinks, reinstall missing resources
- `--reset` — remove unmanaged files from resource directories
- `--prune-package` — clean invalid references from `ai.package.yaml`
- `aimgr repo repair` — fix repository metadata issues
- Migrating from deprecated `verify --fix`

### [Sources](sources.md)

Managing remote and local resource sources using `ai.repo.yaml`.

**Key Topics:**
- `ai.repo.yaml` manifest format
- Adding GitHub repositories (`gh:owner/repo`)
- Adding local paths (symlinked)
- Syncing resources with `repo sync`
- Development workflows

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
