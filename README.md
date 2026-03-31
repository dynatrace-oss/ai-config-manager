# aimgr - AI Resources Manager

[![Build Status](https://github.com/dynatrace-oss/ai-config-manager/actions/workflows/build.yml/badge.svg)](https://github.com/dynatrace-oss/ai-config-manager/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/dynatrace-oss/ai-config-manager/v3)](https://goreportcard.com/report/github.com/dynatrace-oss/ai-config-manager/v3)
[![Release](https://img.shields.io/github/v/release/dynatrace-oss/ai-config-manager)](https://github.com/dynatrace-oss/ai-config-manager/releases)
[![License](https://img.shields.io/github/license/dynatrace-oss/ai-config-manager)](https://github.com/dynatrace-oss/ai-config-manager/blob/main/LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/dynatrace-oss/ai-config-manager)](https://github.com/dynatrace-oss/ai-config-manager/blob/main/go.mod)

A command-line tool for discovering, installing, and managing AI resources (commands, skills, agents, packages) across multiple AI coding tools including Claude Code, OpenCode, GitHub Copilot, and Windsurf.

## Features

- 📦 **Centralized Repository**: Manage all AI resources in one place
- 🔗 **Symlink Installation**: Install resources without duplication
- 🤖 **Multi-Tool Support**: Works with Claude Code, OpenCode, GitHub Copilot, and Windsurf
- 🌐 **GitHub Integration**: Import resources directly from GitHub repositories
- 🎯 **Pattern Matching**: Install multiple resources using glob patterns
- ⚡ **Workspace Caching**: Git repositories cached for 10-50x faster operations
- ✅ **Format Validation**: Automatic validation of resource formats
- 🗂️ **Package Support**: Group related resources together

## Supported AI Tools

`aimgr` supports four major AI coding tools:

| Tool | Commands | Skills | Agents | Directory |
|------|----------|--------|--------|-----------|
| **[Claude Code](https://code.claude.com/)** | ✅ | ✅ | ✅ | `.claude/` |
| **[OpenCode](https://opencode.ai/)** | ✅ | ✅ | ✅ | `.opencode/` |
| **[VSCode / GitHub Copilot](https://github.com/features/copilot)** | ❌* | ✅ | ✅* | `.github/skills/`, `.github/agents/` |
| **[Windsurf](https://codeium.com/windsurf)** | ❌ | ✅ | ❌ | `.windsurf/skills/` |

**Notes:** 
- The support matrix reflects current aimgr direct-install support, not every upstream customization surface a tool may expose
- VSCode / GitHub Copilot and Windsurf support direct skill installation via aimgr
- Skills for Copilot and Windsurf use the same `SKILL.md` format as other tools
- Use `--target copilot` or `--target vscode` for GitHub Copilot installs (both names work)
- Use `--target windsurf` for Windsurf installs
- GitHub Copilot / VS Code agent installs use `.github/agents/*.agent.md` (installed artifact naming)
- Repository source agents remain logical aimgr resources in `agents/*.md`
- GitHub Copilot / VS Code prompt files use `.github/prompts/*.prompt.md`, but aimgr intentionally does not map/install `command` resources there
- GitHub Copilot CLI has its own plugin/customization model for commands and slash commands, which is not the same as project-level `commands/*.md` installs

## Installation

### One-line installer

Install the latest release with a bootstrap script:

```bash
curl -fsSL https://raw.githubusercontent.com/dynatrace-oss/ai-config-manager/main/scripts/install.sh | sh
```

```powershell
# Run from a PowerShell prompt (PowerShell 5.1+ or pwsh 7+)
irm https://raw.githubusercontent.com/dynatrace-oss/ai-config-manager/main/scripts/install.ps1 | iex
```

Use `AIMGR_VERSION` to pin a release (`3.7.0` and `v3.7.0` both work) or `AIMGR_INSTALL_DIR` to override the install location.

### Using Go

If you already have Go installed, you can also install `aimgr` with:

```bash
go install github.com/dynatrace-oss/ai-config-manager/v3/cmd/aimgr@latest
```

### From Source

```bash
# Clone the repository
git clone https://github.com/dynatrace-oss/ai-config-manager.git
cd ai-config-manager

# Build and install to the OS-specific path from `make os-info`
make install

# Or just build (outputs to ./aimgr)
make build
```

Precompiled binaries are published on the [GitHub Releases](https://github.com/dynatrace-oss/ai-config-manager/releases) page.

## Quick Start

```bash
# 1. Configure default tools
aimgr config set install.targets claude

# 2. Add resources from GitHub
aimgr repo add gh:owner/repo

# 3. Add resources from local directory
aimgr repo add local:./my-resources

# 4. List available resources
aimgr repo list

# 5. Install resources in a project
cd your-project/
aimgr install skill/pdf-processing
aimgr install command/test agent/code-reviewer

# 6. Install multiple resources with patterns
aimgr install "skill/pdf*"
```

## Documentation

Complete documentation is available in the `docs/` directory:

### User Guide

- **[Getting Started](docs/user-guide/getting-started.md)** - Installation, setup, and common workflows
- **[Configuration](docs/user-guide/configuration.md)** - Config file, environment variables, field mappings
- **[Sources](docs/user-guide/sources.md)** - Managing local and GitHub sources (`ai.repo.yaml`)
- **[Repairing Resources](docs/user-guide/repair.md)** - Fix broken installations, clean up projects

### Reference

- **[Pattern Matching](docs/reference/pattern-matching.md)** - Glob patterns for batch operations
- **[Output Formats](docs/reference/output-formats.md)** - JSON, YAML, table output for scripting
- **[Resource Validation](docs/reference/resource-validation.md)** - Validate skills, agents, commands, and packages before sync/install
- **[Supported Tools](docs/reference/supported-tools.md)** - Tool compatibility and external documentation links
- **[Troubleshooting](docs/reference/troubleshooting.md)** - Common issues and solutions

### Internals

- **[Repository Layout](docs/internals/repository-layout.md)** - Internal folder structure
- **[Workspace Caching](docs/internals/workspace-caching.md)** - Git repository caching for performance
- **[Git Tracking](docs/internals/git-tracking.md)** - Git-backed repository with change history

### For Contributors

- **[Contributing Guide](CONTRIBUTING.md)** - How to contribute to aimgr
- **[Repository Overview](docs/OVERVIEW.md)** - High-level architecture and repo map
- **[Change Workflow](docs/CHANGE-WORKFLOW.md)** - Commit, push, branch, PR, and merge expectations
- **[Architecture](docs/contributor-guide/architecture.md)** - System design and components
- **[Code Style](docs/contributor-guide/code-style.md)** - Coding standards and conventions
- **[Testing](docs/contributor-guide/testing.md)** - Test guidelines and patterns
- **[Development Environment](docs/contributor-guide/development-environment.md)** - Setup for development
- **[Release Process](docs/contributor-guide/release-process.md)** - Creating releases
- **[AGENTS.md](AGENTS.md)** - Quick reference for AI coding agents

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, code style, and how to submit changes.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
