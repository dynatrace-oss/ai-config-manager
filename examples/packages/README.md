# Package Examples

This directory contains example package files demonstrating the package system in `aimgr`.

## What Are Packages?

Packages are collections of resources (commands, skills, agents) that can be installed together as a unit. They are useful for:

- **Themed Collections**: Group resources for specific workflows
- **Project Templates**: Create reusable resource sets
- **Distribution**: Share curated collections with your team
- **Dependency Management**: Install all needed resources in one command

## Example Packages

### minimal-package.package.json

A minimal package with a single command resource. Demonstrates the simplest possible package structure.

```json
{
  "name": "minimal-package",
  "description": "Minimal package with single command resource",
  "resources": [
    "command/test"
  ]
}
```

**Use case:** Simple single-purpose packages

### example-toolkit.package.json

Basic package with a command and skill. Good starting point for creating your own packages.

```json
{
  "name": "example-toolkit",
  "description": "Example package demonstrating package system with basic tools",
  "resources": [
    "command/test",
    "skill/pdf-processing"
  ]
}
```

**Use case:** Basic multi-resource collections

### web-dev-tools.package.json

Complete web development toolkit with multiple commands, skills, and an agent. Demonstrates a real-world package for a specific workflow.

```json
{
  "name": "web-dev-tools",
  "description": "Complete web development toolkit with build commands, TypeScript skill, and code review agent",
  "resources": [
    "command/build",
    "command/dev",
    "skill/typescript-helper",
    "skill/react-helper",
    "agent/code-reviewer"
  ]
}
```

**Use case:** Workflow-specific toolkits (web development, testing, documentation, etc.)

### testing-suite.package.json

Complete testing toolkit with test commands, test generation skill, and QA agent. Shows how to package related testing resources together.

```json
{
  "name": "testing-suite",
  "description": "Complete testing toolkit with test commands, test generation skill, and QA agent",
  "resources": [
    "command/test",
    "command/coverage",
    "skill/test-generator",
    "skill/mock-helper",
    "agent/qa-tester"
  ]
}
```

**Use case:** Testing workflows and QA automation

## Package Format

All packages follow this JSON format:

```json
{
  "name": "package-name",
  "description": "Human-readable description",
  "resources": [
    "command/name",
    "skill/name",
    "agent/name"
  ]
}
```

### Fields

- **name** (required): Package name following agentskills.io naming rules
  - Lowercase alphanumeric + hyphens only
  - Cannot start/end with hyphen
  - No consecutive hyphens
  - 1-64 characters max

- **description** (required): Human-readable description (1-1024 characters)

- **resources** (required): Array of resource references in `type/name` format
  - Valid types: `command`, `skill`, `agent`
  - All resources must exist in the repository

## Auto-Import Packages

Packages are automatically discovered and imported when using `aimgr repo add` with folders or repositories:

```bash
# Import all resources including packages from a repository
aimgr repo add gh:myorg/ai-resources

# Import only packages using filters
aimgr repo add gh:myorg/resources --filter "package/*"
aimgr repo add local:./my-resources/ --filter "package/*"

# Import packages matching a pattern
aimgr repo add gh:myorg/resources --filter "package/web-*"
aimgr repo add local:./resources/ --filter "package/*-tools"
```

**How Auto-Discovery Works:**

Packages are discovered from `packages/*.package.json` files in:
- Repository root (`packages/` directory)
- Any subdirectory containing a `packages/` folder
- Recursive search with common exclusions (node_modules, .git, etc.)

**Example:**
```bash
# Discover and import all resources including packages
aimgr repo add gh:myorg/ai-resources

# Output:
# Found: 5 commands, 3 skills, 2 agents, 2 packages
# ✓ Added package 'web-dev-tools'
# ✓ Added package 'testing-suite'
# ✓ Added command 'build'
# ...
```

## Using These Examples

### Create Package from Example

These examples are for reference only. To create actual packages, use the `aimgr repo create-package` command:

```bash
# Create a package based on the web-dev-tools example
aimgr repo create-package web-dev-tools \
  --description="Web development toolkit" \
  --resources="command/build,command/dev,skill/typescript-helper"
```

### Install Package

Once a package exists in your repository:

```bash
# Install all resources in the package
aimgr install package/web-dev-tools

# Install to specific project
aimgr install package/testing-suite --project-path ~/my-project
```

### Uninstall Package

Remove all resources from a package:

```bash
# Uninstall from current project
aimgr uninstall package/web-dev-tools

# Uninstall from specific project
aimgr uninstall package/testing-suite --project-path ~/my-project
```

### Remove Package

Remove package from repository:

```bash
# Remove package only (keeps resources)
aimgr repo remove package/web-dev-tools

# Remove package and all its resources
aimgr repo remove package/web-dev-tools --with-resources
```

## Creating Your Own Packages

1. **Identify Resources**: Determine which commands, skills, and agents belong together
2. **Validate Resources**: Ensure all resources exist in your repository
3. **Choose a Name**: Follow agentskills.io naming rules (lowercase, hyphens, no special chars)
4. **Create Package**: Use `aimgr repo create-package` command
5. **Test**: Install the package in a test project to verify

### Example Workflow

```bash
# 1. Add resources to repository (or use auto-import)
aimgr repo add local:~/my-resources/  # Auto-discovers all resources including packages

# Or add resources individually
aimgr repo add local:~/my-commands/build.md
aimgr repo add local:~/my-skills/typescript-helper
aimgr repo add local:~/my-agents/reviewer.md

# 2. Create package
aimgr repo create-package my-tools \
  --description="My development tools" \
  --resources="command/build,skill/typescript-helper,agent/reviewer"

# 3. Test installation
cd ~/test-project
aimgr install package/my-tools

# 4. Verify resources are installed
aimgr list
```

## Best Practices

1. **Group Related Resources**: Package resources that work together or serve a common purpose
2. **Clear Descriptions**: Write descriptive package descriptions explaining the use case
3. **Sensible Names**: Use names that clearly indicate the package purpose
4. **Version Resources**: Consider versioning resources if you create multiple package versions
5. **Document Usage**: Add documentation for how to use the packaged resources together
6. **Test Before Sharing**: Always test packages before sharing with your team
7. **Use Auto-Import for Bulk Operations**: When adding many packages at once, use `aimgr repo add` with filters for efficiency
8. **Organize Package Files**: Store packages in a `packages/` directory for automatic discovery

## See Also

- [Main README](../../README.md) - Full aimgr documentation
- [AGENTS.md](../../AGENTS.md) - Package format specification
- [Resource Examples](../) - Example commands, skills, and agents
