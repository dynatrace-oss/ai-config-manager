# Documentation Index

Welcome to the **aimgr** (ai-config-manager) documentation! This page serves as the central index for all documentation in the project.

## Documentation Structure

The documentation is organized into four sections:

### [User Guide](user-guide/)

User-facing documentation for using aimgr day-to-day.

**Contents:**
- [Getting Started](user-guide/getting-started.md) - Installation, first steps, and common operations
- [Configuration](user-guide/configuration.md) - Repository path, installation targets, and field mappings
- [Sources](user-guide/sources.md) - Managing remote and local resource sources
- [Team and Multi-Project Workflows](user-guide/team-workflows.md) - Recommended patterns for shared project setup, `ai.package.local.yaml` overlays, and multi-project use

**Use this section for:** Learning how to use aimgr, configuring your setup, and managing resources.

---

### [Reference](reference/)

Technical reference documentation for syntax and specifications.

**Contents:**
- [Supported Tools](reference/supported-tools.md) - AI tools supported by aimgr and their capabilities
- [Pattern Matching](reference/pattern-matching.md) - Glob patterns for filtering and matching resources
- [Output Formats](reference/output-formats.md) - CLI output formats (table, JSON, YAML) and scripting
- [Resource Validation](reference/resource-validation.md) - Validate resources by path or canonical ID

**Use this section for:** Looking up exact syntax, understanding tool support, and scripting with aimgr.

---

### [Contributor Guide](contributor-guide/)

Documentation for developers contributing to aimgr.

**Contents:**
- [Overview](contributor-guide/README.md) - Getting started with development
- [Code Style](contributor-guide/code-style.md) - Naming, imports, error handling, and best practices
- [Architecture](contributor-guide/architecture.md) - System design, package structure, and design rules
- [Testing](contributor-guide/testing.md) - Test types, patterns, and troubleshooting
- [Release Process](contributor-guide/release-process.md) - How to create releases

**See also:** [CONTRIBUTING.md](../CONTRIBUTING.md) for setup and workflow.

**Use this section for:** Setting up a development environment and contributing code.

---

### [Internals](internals/)

Implementation details for curious users and contributors.

**Contents:**
- [Repository Layout](internals/repository-layout.md) - Internal folder structure of an aimgr repository
- [Workspace Caching](internals/workspace-caching.md) - How the workspace cache optimizes Git operations
- [Git Tracking](internals/git-tracking.md) - How aimgr tracks repository changes with Git

**Use this section for:** Understanding how aimgr works internally, debugging, and advanced troubleshooting.

---

## For AI Agents

If you're an AI coding agent working on this repository, see **[AGENTS.md](../AGENTS.md)** for:
- Quick reference for build & test commands
- Code style guidelines
- Common patterns and resource loading
- Testing best practices

---

## Quick Links

### Project Information
- **[README.md](../README.md)** - Project overview, installation, and command reference
- **[CONTRIBUTING.md](../CONTRIBUTING.md)** - How to contribute to aimgr
- **[CHANGELOG.md](../CHANGELOG.md)** - Version history

### External Resources
- **[GitHub Repository](https://github.com/dynatrace-oss/ai-config-manager)** - Source code and issue tracker
- **[GitHub Releases](https://github.com/dynatrace-oss/ai-config-manager/releases)** - Download releases and binaries
- **[AgentSkills.io](https://agentskills.io/home)** - Community skill format specification

---

## Getting Help

- **Questions**: [GitHub Discussions](https://github.com/dynatrace-oss/ai-config-manager/discussions)
- **Bug Reports**: [GitHub Issues](https://github.com/dynatrace-oss/ai-config-manager/issues)
- **Feature Requests**: [GitHub Issues](https://github.com/dynatrace-oss/ai-config-manager/issues)
