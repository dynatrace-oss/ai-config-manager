# Output Formats

`aimgr` supports multiple output formats to suit different use cases: human-readable tables for interactive use, JSON for scripting and automation, and YAML for configuration files.

## Table of Contents

- [Available Formats](#available-formats)
- [Using Output Formats](#using-output-formats)
- [Table Format (Default)](#table-format-default)
- [JSON Format](#json-format)
- [YAML Format](#yaml-format)
- [Error Reporting](#error-reporting)
- [Scripting Examples](#scripting-examples)

## Available Formats

| Format | Use Case | Flag |
|--------|----------|------|
| **table** | Human-readable, interactive use | `--format=table` (default) |
| **json** | Scripting, automation, CI/CD | `--format=json` |
| **yaml** | Configuration files, human-editable structured data | `--format=yaml` |

## Using Output Formats

The `--format` flag is supported by commands that perform bulk operations:

```bash
# Commands that support --format flag
aimgr repo add <source> --format=<format>
aimgr repo sync --format=<format>
aimgr repo list --format=<format>
aimgr repo describe <pattern> --format=<format>
aimgr repo info --format=<format>           # NEW
aimgr repo verify --format=<format>         # NEW (replaces --json)
aimgr repo prune --format=<format>          # NEW
aimgr repair --format=<format>              # NEW
aimgr repo repair --format=<format>         # NEW
aimgr list --format=<format>
```

## Table Format (Default)

The table format provides human-readable output ideal for interactive terminal use.

### Features

- **Structured layout**: Clean columns with headers
- **Color-coded status**: SUCCESS, SKIPPED, FAILED
- **Summary statistics**: Count of operations and resources
- **Error hints**: Suggestions for debugging failures
- **Responsive sizing**: Tables adapt to your terminal width at render time (TTY only)

### Terminal Output Behavior

#### Interactive Terminal (TTY)

When running `aimgr` in an interactive terminal, table output uses responsive sizing to adapt to your terminal width (when implemented). Columns dynamically adjust to use available space.

```bash
# Interactive terminal - responsive mode active
$ aimgr list
# Table adapts to terminal width
```

#### Piped or Redirected Output (non-TTY)

When output is piped or redirected, responsive mode is **automatically disabled**. Tables use fixed widths for consistent, predictable output.

```bash
# Piped output - fixed width tables
$ aimgr list | less
$ aimgr list | grep "skill"
$ aimgr list > file.txt
```

**Why this matters:**
- Scripts get consistent table formatting across runs
- CI/CD logs have readable, predictable output
- Redirected files have stable layouts for version control
- Output can be reliably parsed by downstream tools

#### Structured Formats (Terminal-Independent)

JSON and YAML formats (`--format=json`, `--format=yaml`) are **always independent of terminal width**. They produce identical output in all contexts:

```bash
# These produce identical output regardless of terminal width
$ aimgr list --format=json
$ aimgr list --format=json | jq
$ aimgr list --format=json > data.json
```

**Best practice:** Use structured formats for scripting and automation to avoid any terminal-related formatting variations.

### Terminal Resize Behavior

Tables are formatted based on your terminal width at the time the command runs. If you resize your terminal after the table has been rendered, the existing output will not automatically reflow or adjust.

**This is expected behavior** for CLI tools and works the same way as other standard tools like `ls`, `git log`, and `grep`:
- Once rendered, output is immutable text in your shell buffer
- The shell controls display, not the application
- To see the table formatted for your new terminal size, simply re-run the command

**Example:**
```bash
# Terminal is 120 columns wide
$ aimgr repo list
┌──────────────────────┬───────────────────┬──────────┬────────────────────┐
│         NAME         │      TARGETS      │   SYNC   │    DESCRIPTION     │
├──────────────────────┼───────────────────┼──────────┼────────────────────┤
│ skill/skill-creator  │ claude, opencode  │    ✓     │ Guide for creating │
└──────────────────────┴───────────────────┴──────────┴────────────────────┘

# (User resizes terminal to 80 columns)
# Existing output above stays the same - no reflow

# Re-run command to see new layout
$ aimgr repo list
┌─────────────────┬──────────┬──────┬─────────┐
│      NAME       │ TARGETS  │ SYNC │  DESC   │
├─────────────────┼──────────┼──────┼─────────┤
│ skill/skill-cr  │ claude,  │  ✓   │ Guide   │
│ eator           │ opencode │      │ for...  │
└─────────────────┴──────────┴──────┴─────────┘
```

### Example: Adding Resources

```bash
$ aimgr repo add ~/my-resources/

┌─────────┬─────────────────────┬─────────┬──────────────────────┐
│ TYPE    │ NAME                │ STATUS  │ MESSAGE              │
├─────────┼─────────────────────┼─────────┼──────────────────────┤
│ skill   │ pdf-processing      │ SUCCESS │ Added to repository  │
│ skill   │ typescript-helper   │ SUCCESS │ Added to repository  │
│ command │ test                │ SUCCESS │ Added to repository  │
│ command │ build               │ SKIPPED │ Already exists       │
│ agent   │ code-reviewer       │ SUCCESS │ Added to repository  │
│ package │ web-tools           │ SUCCESS │ Added to repository  │
└─────────┴─────────────────────┴─────────┴──────────────────────┘

Summary: 5 added, 0 failed, 1 skipped (6 total)
```

### Example: With Errors

```bash
$ aimgr repo add ~/broken-resources/

┌─────────┬──────────────┬────────┬─────────────────────────────────────┐
│ TYPE    │ NAME         │ STATUS │ MESSAGE                             │
├─────────┼──────────────┼────────┼─────────────────────────────────────┤
│ skill   │ valid-skill  │ SUCCESS│ Added to repository                 │
│ skill   │ broken-skill │ FAILED │ missing required field: description │
│ command │ test         │ SUCCESS│ Added to repository                 │
└─────────┴──────────────┴────────┴─────────────────────────────────────┘

Summary: 2 added, 1 failed, 0 skipped (3 total)

⚠ Use --format=json to see detailed error messages
```

### Example: Listing Repository Resources with Sync Status

The `aimgr repo list` command shows resources in your repository with installation targets and sync status:

```bash
$ aimgr repo list --format=table

┌──────────────────────┬───────────────────┬──────────┬────────────────────┐
│         NAME         │      TARGETS      │   SYNC   │    DESCRIPTION     │
├──────────────────────┼───────────────────┼──────────┼────────────────────┤
│ skill/skill-creator  │ claude, opencode  │    ✓     │ Guide for creating │
│ skill/webapp-testing │ claude            │    *     │ Toolkit for inter  │
│ command/test         │ claude, opencode  │    ⚠     │ Run tests          │
│ agent/code-reviewer  │ opencode          │    -     │ Review code chan   │
└──────────────────────┴───────────────────┴──────────┴────────────────────┘

Legend:
  ✓ = In sync  * = Not in manifest  ⚠ = Not installed  - = No manifest
```

**Columns explained:**
- **NAME**: Resource reference (type/name format)
- **TARGETS**: Which AI tools have this resource installed (claude, opencode, copilot)
- **SYNC**: Synchronization status with ai.package.yaml manifest
- **DESCRIPTION**: Brief description (truncated to fit)

**Sync Status Symbols:**
- **✓ (In sync)**: Resource is both in ai.package.yaml and installed
- **\* (Not in manifest)**: Resource is installed but not declared in ai.package.yaml
- **⚠ (Not installed)**: Resource is in ai.package.yaml but not installed yet
- **\- (No manifest)**: No ai.package.yaml file exists in current directory

### Example: Listing Installed Resources (Simple)

To see only resources installed in your current directory, use `aimgr list`:

```bash
$ aimgr list --format=table

┌──────────────────────┬───────────────────┬───────────────────────────────────┐
│         NAME         │      TARGETS      │           DESCRIPTION             │
├──────────────────────┼───────────────────┼───────────────────────────────────┤
│ skill/skill-creator  │ claude, opencode  │ Guide for creating effective skills│
│ skill/webapp-testing │ claude            │ Toolkit for testing web apps       │
│ command/test         │ claude, opencode  │ Run tests                          │
└──────────────────────┴───────────────────┴───────────────────────────────────┘
```

This simpler view shows only what's installed, without sync status information.

## JSON Format

The JSON format provides structured, machine-readable output perfect for scripting, automation, and CI/CD pipelines.

### Features

- **Structured data**: Complete operation results
- **Detailed errors**: Full error messages and context
- **Easy parsing**: Standard JSON for use with `jq`, Python, etc.
- **Programmatic access**: All data available for scripts

### Output Structure

```json
{
  "added": [
    {
      "name": "pdf-processing",
      "type": "skill",
      "path": "/path/to/skills/pdf-processing",
      "message": ""
    }
  ],
  "skipped": [
    {
      "name": "build",
      "type": "command",
      "path": "/path/to/commands/build.md",
      "message": "already exists"
    }
  ],
  "failed": [
    {
      "name": "broken-skill",
      "type": "skill",
      "path": "/path/to/skills/broken-skill",
      "message": "missing required field: description"
    }
  ],
  "command_count": 2,
  "skill_count": 3,
  "agent_count": 1,
  "package_count": 1
}
```

### Example: Adding Resources

```bash
$ aimgr repo add ~/my-resources/ --format=json
{
  "added": [
    {
      "name": "pdf-processing",
      "type": "skill",
      "path": "/home/user/.local/share/ai-config/repo/skills/pdf-processing"
    },
    {
      "name": "test",
      "type": "command",
      "path": "/home/user/.local/share/ai-config/repo/commands/test.md"
    },
    {
      "name": "code-reviewer",
      "type": "agent",
      "path": "/home/user/.local/share/ai-config/repo/agents/code-reviewer.md"
    }
  ],
  "skipped": [],
  "failed": [],
  "command_count": 1,
  "skill_count": 1,
  "agent_count": 1,
  "package_count": 0
}
```

### Example: Listing Repository Resources with Sync Status (JSON)

The `aimgr repo list` command with JSON format includes installation targets and sync status:

```bash
$ aimgr repo list --format=json
{
  "resources": [
    {
      "type": "skill",
      "name": "skill-creator",
      "description": "Guide for creating effective skills",
      "version": "1.0.0",
      "targets": ["claude", "opencode"],
      "sync_status": "in-sync"
    },
    {
      "type": "skill",
      "name": "webapp-testing",
      "description": "Toolkit for interacting with local web apps",
      "version": "1.2.0",
      "targets": ["claude"],
      "sync_status": "not-in-manifest"
    },
    {
      "type": "command",
      "name": "test",
      "description": "Run tests",
      "targets": ["claude", "opencode"],
      "sync_status": "not-installed"
    }
  ],
  "packages": []
}
```

**New fields in repo list:**
- **targets**: Array of tool names where resource is installed
- **sync_status**: One of "in-sync", "not-in-manifest", "not-installed", or "no-manifest"

### Example: Listing Installed Resources (JSON)

To see only installed resources (simpler output without sync status):

```bash
$ aimgr list --format=json
[
  {
    "type": "skill",
    "name": "skill-creator",
    "description": "Guide for creating effective skills",
    "version": "1.0.0",
    "targets": ["claude", "opencode"]
  },
  {
    "type": "skill",
    "name": "webapp-testing",
    "description": "Toolkit for testing web apps",
    "version": "1.2.0",
    "targets": ["claude"]
  }
]
```

**Note:** The `list` command shows simpler output focused on what's installed. Use `repo list` for sync status information.

## YAML Format

The YAML format provides human-readable structured output that's easy to edit and version control.

### Features

- **Human-readable**: More readable than JSON
- **Comments supported**: Can add documentation
- **Configuration-friendly**: Easy to use in config files
- **Version control**: Clean diffs in git

### Output Structure

```yaml
added:
  - name: pdf-processing
    type: skill
    path: /path/to/skills/pdf-processing
  - name: test
    type: command
    path: /path/to/commands/test.md
skipped:
  - name: build
    type: command
    path: /path/to/commands/build.md
    message: already exists
failed: []
command_count: 2
skill_count: 1
agent_count: 0
package_count: 0
```

### Example: Adding Resources

```bash
$ aimgr repo add ~/my-resources/ --format=yaml
added:
  - name: pdf-processing
    type: skill
    path: /home/user/.local/share/ai-config/repo/skills/pdf-processing
  - name: typescript-helper
    type: skill
    path: /home/user/.local/share/ai-config/repo/skills/typescript-helper
  - name: code-reviewer
    type: agent
    path: /home/user/.local/share/ai-config/repo/agents/code-reviewer.md
skipped: []
failed: []
command_count: 0
skill_count: 2
agent_count: 1
package_count: 0
```

### Example: Saving to File

```bash
# Save import results for auditing
$ aimgr repo add gh:myorg/resources --format=yaml > import-log.yaml

# Review what was imported
$ cat import-log.yaml
```

## Understanding Sync Status

The `aimgr repo list` command tracks synchronization between installed resources and your project's `ai.package.yaml` manifest. This helps ensure your installations match your declared dependencies.

### Sync Status Values

| Status | Symbol | Meaning | Action Needed |
|--------|--------|---------|---------------|
| **in-sync** | ✓ | Resource is in manifest and installed | None - everything is synchronized |
| **not-in-manifest** | * | Resource is installed but not in ai.package.yaml | Add to manifest if you want to track it |
| **not-installed** | ⚠ | Resource is in manifest but not installed | Run `aimgr install <resource>` |
| **no-manifest** | - | No ai.package.yaml file exists | Create manifest with `aimgr init` (if needed) |

### Common Scenarios

#### Scenario 1: Resource marked with * (Not in manifest)

**What happened:** You installed a resource, but it's not declared in your `ai.package.yaml`.

```bash
$ aimgr repo list
┌──────────────────────┬──────────┬──────┬─────────────────┐
│ skill/webapp-testing │ claude   │  *   │ Toolkit for...  │
└──────────────────────┴──────────┴──────┴─────────────────┘
```

**Solutions:**
1. **Add to manifest** (recommended if this is a project dependency):
   ```bash
   # Manually add to ai.package.yaml:
   resources:
     - skill/webapp-testing
   ```

2. **Keep as-is** (if it's a temporary/local-only resource):
   ```bash
   # No action needed - the * reminds you it's not tracked
   ```

#### Scenario 2: Resource marked with ⚠ (Not installed)

**What happened:** The resource is in your `ai.package.yaml` but not installed yet.

```bash
$ aimgr repo list
┌─────────────────┬──────────┬──────┬─────────────────┐
│ command/test    │ -        │  ⚠   │ Run tests       │
└─────────────────┴──────────┴──────┴─────────────────┘
```

**Solution:** Install the resource:
```bash
$ aimgr install command/test
```

This commonly occurs when:
- You just cloned a repository with an `ai.package.yaml`
- Someone else added resources to the manifest
- You manually edited `ai.package.yaml`

#### Scenario 3: All resources marked with - (No manifest)

**What happened:** No `ai.package.yaml` file exists in your current directory.

```bash
$ aimgr repo list
┌──────────────────────┬──────────┬──────┬─────────────────┐
│ skill/skill-creator  │ claude   │  -   │ Guide for...    │
│ skill/webapp-testing │ claude   │  -   │ Toolkit for...  │
└──────────────────────┴──────────┴──────┴─────────────────┘
```

**Solutions:**
1. **Create a manifest** (recommended for tracking dependencies):
   ```bash
   # Option 1: Create empty manifest
   echo "resources: []" > ai.package.yaml
   
   # Option 2: Generate from installed resources
   # (Future feature - not yet implemented)
   ```

2. **No action needed** if you don't want to track dependencies with a manifest.

### Checking Sync Status Programmatically

Use JSON format to check sync status in scripts:

```bash
# Find all resources not in manifest
$ aimgr repo list --format=json | jq -r '.resources[] | select(.sync_status == "not-in-manifest") | .name'
skill/webapp-testing
command/build

# Find all resources needing installation
$ aimgr repo list --format=json | jq -r '.resources[] | select(.sync_status == "not-installed") | .name'
command/test
agent/reviewer

# Count out-of-sync resources
$ aimgr repo list --format=json | jq '[.resources[] | select(.sync_status != "in-sync" and .sync_status != "no-manifest")] | length'
3
```

### Best Practices

1. **Use manifests for projects**: Create `ai.package.yaml` files for projects you share or deploy
2. **Keep manifests updated**: When you install new resources, add them to the manifest
3. **Review sync status regularly**: Run `aimgr list` to catch drift between installations and manifest
4. **Use CI/CD checks**: Fail builds if sync status shows warnings (see scripting examples below)

## Repository Maintenance Commands

The following commands support structured output for automation and monitoring.

### repo info - Repository Statistics

Display repository information in various formats:

**Table format (default):**
```bash
$ aimgr repo info

Repository Information
======================

Location: /home/user/.local/share/ai-config/repo

Total Resources: 15
  Commands:      5
  Skills:        8
  Agents:        2

Disk Usage:    2.3 MB
```

**JSON format:**
```bash
$ aimgr repo info --format=json
{
  "location": "/home/user/.local/share/ai-config/repo",
  "total_resources": 15,
  "command_count": 5,
  "skill_count": 8,
  "agent_count": 2,
  "disk_usage_bytes": 2411520,
  "disk_usage_human": "2.3 MB"
}
```

**CI/CD Example:**
```bash
# Check resource count in CI
RESOURCE_COUNT=$(aimgr repo info --format=json | jq '.total_resources')
if [ "$RESOURCE_COUNT" -lt 10 ]; then
  echo "::warning::Only $RESOURCE_COUNT resources in repository"
fi
```

### repo verify - Repository Integrity

Check repository health with structured output:

**JSON format:**
```bash
$ aimgr repo verify --format=json
{
  "resources_without_metadata": [],
  "orphaned_metadata": [],
  "missing_source_paths": [],
  "type_mismatches": [],
  "packages_with_missing_refs": [],
  "has_errors": false,
  "has_warnings": false
}
```

**CI/CD Example:**
```bash
# Fail build on repository errors
output=$(aimgr repo verify --format=json)
if [ $(echo "$output" | jq '.has_errors') == "true" ]; then
  echo "::error::Repository verification failed"
  echo "$output" | jq -r '.orphaned_metadata[] | "- \(.name) (\(.type))"'
  exit 1
fi
```

**Note:** The `--json` flag is deprecated. Use `--format=json` instead.

### repo prune - Workspace Cleanup

Clean up unreferenced Git caches with structured output:

**JSON format:**
```bash
$ aimgr repo prune --dry-run --format=json
{
  "unreferenced_caches": [
    {
      "url": "https://github.com/user/old-repo",
      "path": "/home/user/.local/share/ai-config/repo/.workspace/abc123",
      "size_bytes": 1258291,
      "size_human": "1.2 MB"
    }
  ],
  "total_count": 1,
  "total_size_bytes": 1258291,
  "total_size_human": "1.2 MB",
  "dry_run": true
}
```

**Automation Example:**
```bash
# Automated cleanup with logging
aimgr repo prune --force --format=json >> /var/log/aimgr-cleanup.jsonl

# Monitor freed space
FREED=$(aimgr repo prune --force --format=json | jq '.freed_bytes')
echo "Freed $FREED bytes"
```

## Error Reporting

All output formats include detailed error reporting to help diagnose issues.

### Table Format Errors

```bash
$ aimgr repo add ~/resources/ --format=table

┌─────────┬──────────────┬────────┬─────────────────────────────────────┐
│ TYPE    │ NAME         │ STATUS │ MESSAGE                             │
├─────────┼──────────────┼────────┼─────────────────────────────────────┤
│ skill   │ valid-skill  │ SUCCESS│ Added to repository                 │
│ skill   │ no-desc      │ FAILED │ missing required field: description │
│ command │ invalid-yaml │ FAILED │ failed to parse YAML frontmatter    │
│ agent   │ bad-name!    │ FAILED │ invalid name format                 │
└─────────┴──────────────┴────────┴─────────────────────────────────────┘

Summary: 1 added, 3 failed, 0 skipped (4 total)

⚠ Use --format=json to see detailed error messages
```

### JSON Format Errors

JSON format provides the most detailed error information:

```json
{
  "added": [
    {
      "name": "valid-skill",
      "type": "skill",
      "path": "/path/to/skills/valid-skill"
    }
  ],
  "skipped": [],
  "failed": [
    {
      "name": "no-desc",
      "type": "skill",
      "path": "/path/to/skills/no-desc",
      "message": "missing required field: description"
    },
    {
      "name": "invalid-yaml",
      "type": "command",
      "path": "/path/to/commands/invalid-yaml.md",
      "message": "failed to parse YAML frontmatter: yaml: line 5: mapping values are not allowed in this context"
    },
    {
      "name": "bad-name!",
      "type": "agent",
      "path": "/path/to/agents/bad-name!.md",
      "message": "invalid name format: must be lowercase alphanumeric with hyphens only"
    }
  ],
  "command_count": 0,
  "skill_count": 1,
  "agent_count": 0,
  "package_count": 0
}
```

### Common Error Messages

| Error | Meaning | Solution |
|-------|---------|----------|
| `missing required field: description` | Resource missing description in frontmatter | Add `description:` field to YAML frontmatter |
| `failed to parse YAML frontmatter` | Invalid YAML syntax | Check YAML syntax, ensure proper indentation |
| `invalid name format` | Resource name doesn't follow naming rules | Use lowercase, alphanumeric, hyphens only |
| `already exists` | Resource with same name exists | Use `--force` to overwrite or `--skip-existing` |
| `source not found` | Path or repository doesn't exist | Check path spelling and existence |

## Scripting Examples

### Using jq with JSON Output

**Check sync status of repository resources:**
```bash
# List all resource names in repository
$ aimgr repo list --format=json | jq -r '.resources[].name'
skill/skill-creator
skill/webapp-testing
command/test

# List resources by sync status
$ aimgr repo list --format=json | jq -r '.resources[] | select(.sync_status == "not-in-manifest") | .name'
skill/webapp-testing

# List resources and their installation targets
$ aimgr repo list --format=json | jq -r '.resources[] | "\(.name): \(.targets | join(", "))"'
skill/skill-creator: claude, opencode
skill/webapp-testing: claude
command/test: claude, opencode

# Check if any resources are out of sync
$ aimgr repo list --format=json | jq -e '.resources[] | select(.sync_status != "in-sync" and .sync_status != "no-manifest")' > /dev/null
# Exit code 0 if out-of-sync resources exist, 1 if all in sync
```

**Check installed resources (simpler):**
```bash
# List installed resource names
$ aimgr list --format=json | jq -r '.[].name'
skill/skill-creator
skill/webapp-testing

# List with targets
$ aimgr list --format=json | jq -r '.[] | "\(.name): \(.targets | join(", "))"'
skill/skill-creator: claude, opencode
skill/webapp-testing: claude
```

**Extract only successful additions:**
```bash
$ aimgr repo add ~/resources/ --format=json | jq '.added[].name'
"pdf-processing"
"typescript-helper"
"code-reviewer"
```

**Count resources by type:**
```bash
$ aimgr repo add ~/resources/ --format=json | jq '{
  skills: .skill_count,
  commands: .command_count,
  agents: .agent_count,
  packages: .package_count
}'
{
  "skills": 2,
  "commands": 1,
  "agents": 1,
  "packages": 0
}
```

**Get error messages:**
```bash
$ aimgr repo add ~/resources/ --format=json | jq '.failed[] | {
  name: .name,
  error: .message
}'
{
  "name": "broken-skill",
  "error": "missing required field: description"
}
```

**Check if any operations failed:**
```bash
$ aimgr repo add ~/resources/ --format=json | jq '.failed | length'
2

# Use in scripts
if [ $(aimgr repo add ~/resources/ --format=json | jq '.failed | length') -gt 0 ]; then
  echo "Import failed!"
  exit 1
fi
```

**Filter by resource type:**
```bash
$ aimgr repo add ~/resources/ --format=json | jq '.added[] | select(.type == "skill")'
{
  "name": "pdf-processing",
  "type": "skill",
  "path": "/path/to/skills/pdf-processing"
}
```

### Repository Maintenance

**Monitor repository health:**
```bash
# Check for any repository issues
$ aimgr repo verify --format=json | jq '{
  has_errors: .has_errors,
  has_warnings: .has_warnings,
  orphaned: (.orphaned_metadata | length),
  missing_metadata: (.resources_without_metadata | length)
}'
{
  "has_errors": false,
  "has_warnings": true,
  "orphaned": 0,
  "missing_metadata": 3
}
```

**Track repository growth:**
```bash
# Log repository statistics over time
$ aimgr repo info --format=json | jq '{
  date: now | strftime("%Y-%m-%d"),
  resources: .total_resources,
  disk_mb: (.disk_usage_bytes / 1024 / 1024 | floor)
}' >> repo-stats.jsonl
```

**Automated cleanup:**
```bash
# Clean up workspace caches weekly
#!/bin/bash
LOG_FILE="/var/log/aimgr-prune.log"

echo "$(date): Starting prune" >> "$LOG_FILE"
result=$(aimgr repo prune --force --format=json 2>&1)

if [ $? -eq 0 ]; then
  freed=$(echo "$result" | jq '.freed_human')
  echo "$(date): Freed $freed" >> "$LOG_FILE"
else
  echo "$(date): Prune failed" >> "$LOG_FILE"
fi
```

### CI/CD Integration

**Check sync status in CI/CD:**
```yaml
# GitHub Actions - Verify resources are in sync
- name: Check AI Resource Sync
  run: |
    # List repository resources with sync status
    output=$(aimgr repo list --format=json)
    
    # Check for out-of-sync resources
    out_of_sync=$(echo "$output" | jq '[.resources[] | select(.sync_status != "in-sync" and .sync_status != "no-manifest")] | length')
    
    if [ "$out_of_sync" -gt 0 ]; then
      echo "::warning::Found $out_of_sync resources out of sync"
      echo "$output" | jq -r '.resources[] | select(.sync_status != "in-sync" and .sync_status != "no-manifest") | "- \(.name): \(.sync_status)"'
      
      # Optionally fail the build
      echo "::error::Resources are out of sync with ai.package.yaml"
      exit 1
    fi
    
    echo "::notice::All resources are in sync"
```

**GitHub Actions - Import resources example:**
```yaml
- name: Import AI Resources
  id: import
  run: |
    output=$(aimgr repo add gh:myorg/resources --format=json)
    echo "$output" > import-results.json
    
    # Check for failures
    failed=$(echo "$output" | jq '.failed | length')
    if [ "$failed" -gt 0 ]; then
      echo "::error::Failed to import $failed resources"
      echo "$output" | jq '.failed[]'
      exit 1
    fi
    
    # Report summary
    added=$(echo "$output" | jq '.added | length')
    echo "::notice::Successfully imported $added resources"

- name: Upload import results
  uses: actions/upload-artifact@v3
  with:
    name: import-results
    path: import-results.json
```

**GitLab CI example:**
```yaml
import-resources:
  script:
    - output=$(aimgr repo add gh:myorg/resources --format=json)
    - echo "$output" > import-results.json
    - |
      if [ $(echo "$output" | jq '.failed | length') -gt 0 ]; then
        echo "Import failed!"
        echo "$output" | jq '.failed[]'
        exit 1
      fi
  artifacts:
    paths:
      - import-results.json
    reports:
      dotenv: import-results.json
```

### Python Integration

**Check sync status:**
```python
#!/usr/bin/env python3
import json
import subprocess
import sys

def list_resources():
    """List repository resources with sync status and return parsed results."""
    result = subprocess.run(
        ["aimgr", "repo", "list", "--format=json"],
        capture_output=True,
        text=True
    )
    
    if result.returncode != 0:
        print(f"Error running aimgr: {result.stderr}", file=sys.stderr)
        sys.exit(1)
    
    return json.loads(result.stdout)

def check_sync_status():
    """Check if resources are in sync with manifest."""
    data = list_resources()
    resources = data.get("resources", [])
    
    # Find out-of-sync resources
    out_of_sync = [
        r for r in resources 
        if r["sync_status"] not in ["in-sync", "no-manifest"]
    ]
    
    if out_of_sync:
        print("⚠ Warning: Found resources out of sync:\n")
        
        for resource in out_of_sync:
            status = resource["sync_status"]
            name = resource["name"]
            targets = ", ".join(resource.get("targets", []))
            
            if status == "not-in-manifest":
                print(f"  * {name}")
                print(f"    Installed in: {targets}")
                print(f"    Action: Add to ai.package.yaml")
            elif status == "not-installed":
                print(f"  ⚠ {name}")
                print(f"    Action: Run 'aimgr install {name}'")
            
            print()
        
        return False
    
    print("✓ All resources are in sync")
    return True

def main():
    if not check_sync_status():
        sys.exit(1)

if __name__ == "__main__":
    main()
```

**Import resources:**
```python
#!/usr/bin/env python3
import json
import subprocess
import sys

def import_resources(source_path):
    """Import resources and return parsed results."""
    result = subprocess.run(
        ["aimgr", "repo", "add", source_path, "--format=json"],
        capture_output=True,
        text=True
    )
    
    if result.returncode != 0:
        print(f"Error running aimgr: {result.stderr}", file=sys.stderr)
        sys.exit(1)
    
    return json.loads(result.stdout)

def main():
    # Import resources
    results = import_resources("~/my-resources/")
    
    # Check for failures
    if results["failed"]:
        print("Failed to import the following resources:")
        for failure in results["failed"]:
            print(f"  - {failure['name']} ({failure['type']}): {failure['message']}")
        sys.exit(1)
    
    # Report success
    print(f"Successfully imported {len(results['added'])} resources:")
    for resource in results["added"]:
        print(f"  - {resource['name']} ({resource['type']})")
    
    # Print statistics
    print(f"\nStatistics:")
    print(f"  Commands: {results['command_count']}")
    print(f"  Skills: {results['skill_count']}")
    print(f"  Agents: {results['agent_count']}")
    print(f"  Packages: {results['package_count']}")

if __name__ == "__main__":
    main()
```

### Shell Script Integration

```bash
#!/bin/bash
# import-and-verify.sh - Import resources with error handling

set -euo pipefail

SOURCE_PATH="${1:-}"
if [ -z "$SOURCE_PATH" ]; then
  echo "Usage: $0 <source-path>" >&2
  exit 1
fi

# Import resources
echo "Importing resources from $SOURCE_PATH..."
output=$(aimgr repo add "$SOURCE_PATH" --format=json)

# Parse results using jq
added_count=$(echo "$output" | jq '.added | length')
failed_count=$(echo "$output" | jq '.failed | length')
skipped_count=$(echo "$output" | jq '.skipped | length')

# Report results
echo "Results:"
echo "  Added: $added_count"
echo "  Failed: $failed_count"
echo "  Skipped: $skipped_count"

# Handle failures
if [ "$failed_count" -gt 0 ]; then
  echo ""
  echo "Failed resources:"
  echo "$output" | jq -r '.failed[] | "  - \(.name) (\(.type)): \(.message)"'
  exit 1
fi

# List added resources
if [ "$added_count" -gt 0 ]; then
  echo ""
  echo "Successfully added:"
  echo "$output" | jq -r '.added[] | "  - \(.name) (\(.type))"'
fi

echo ""
echo "Import completed successfully!"
```

### Monitoring and Alerting

**Check for import errors in cron job:**
```bash
#!/bin/bash
# /etc/cron.daily/sync-ai-resources

LOG_FILE="/var/log/aimgr-sync.log"

output=$(aimgr repo sync --format=json 2>&1)
failed=$(echo "$output" | jq '.failed | length')

if [ "$failed" -gt 0 ]; then
  echo "$(date): Resource sync failed" >> "$LOG_FILE"
  echo "$output" | jq '.failed[]' >> "$LOG_FILE"
  
  # Send alert (example using mail)
  echo "$output" | jq '.failed[]' | mail -s "aimgr sync failed" admin@example.com
  exit 1
fi

echo "$(date): Resource sync successful ($added added)" >> "$LOG_FILE"
```

## Best Practices

### When to Use Each Format

**Use TABLE format when:**
- Working interactively in a terminal
- You want a quick overview of results
- Human readability is the priority
- You're exploring or debugging manually

**Use JSON format when:**
- Writing scripts or automation
- Integrating with CI/CD pipelines
- You need detailed error messages
- Processing output programmatically
- Generating reports or logs

**Use YAML format when:**
- Creating configuration files
- You want human-readable structured data
- Version controlling import results
- Documenting import operations
- Generating audit trails

### Error Handling

1. **Always check for failures** in scripts:
   ```bash
   output=$(aimgr repo add ~/resources/ --format=json)
   if [ $(echo "$output" | jq '.failed | length') -gt 0 ]; then
     echo "Import failed!"
     exit 1
   fi
   ```

2. **Log detailed errors** in automation:
   ```bash
   aimgr repo add ~/resources/ --format=json | \
     jq '.failed[]' > error-log.json
   ```

3. **Use descriptive error messages** for debugging:
   ```bash
   aimgr repo add ~/resources/ --format=json | \
     jq -r '.failed[] | "\(.name): \(.message)"'
   ```

### Performance Considerations

- JSON and YAML formats have similar performance
- Table format may be slightly slower for large outputs (formatting overhead)
- For batch operations, JSON is recommended for programmatic processing
- Use `--dry-run` with any format to preview without making changes

## Command Format Support Summary

| Command | Formats | Use Case |
|---------|---------|----------|
| `repo add` | table, json, yaml | Import resources with results |
| `repo sync` | table, json, yaml | Sync from sources |
| `repo list` | table, json, yaml | List with sync status |
| `repo describe` | table, json, yaml | Resource details |
| `repo info` | table, json, yaml | Repository statistics |
| `repo verify` | table, json, yaml | Repository integrity checks |
| `repo prune` | table, json, yaml | Workspace cleanup |
| `repair` | table, json | Fix project installations |
| `repo repair` | text, json | Fix repository metadata |
| `list` | table, json, yaml | Installed resources (simple) |
| `install` | table, json, yaml | Install resources |
| `uninstall` | table, json, yaml | Uninstall resources |
