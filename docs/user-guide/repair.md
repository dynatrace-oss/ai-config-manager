# Repairing Resources

`aimgr repair` diagnoses and fixes issues with installed resources in your project. It replaces the deprecated `verify --fix` workflow with a dedicated command that has clear, composable flags.

---

## Quick Start

```bash
# Check what's wrong (read-only)
aimgr verify

# Fix it
aimgr repair
```

---

## What `repair` Fixes

By default (no flags), `aimgr repair` handles the most common issues:

| Issue | What Happened | What Repair Does |
|-------|---------------|------------------|
| **Broken symlinks** | Target file was deleted or moved | Removes broken symlink, reinstalls from repository |
| **Wrong-repo symlinks** | Symlink points to a different repository | Removes wrong symlink, reinstalls from correct repository |
| **Missing resources** | Listed in `ai.package.yaml` but not installed | Installs from repository |
| **Orphaned resources** | Installed but not in `ai.package.yaml` | Prints a hint (not auto-removed) |

```bash
$ aimgr repair

  Fixing skill/pdf-processing...
    ✓ Reinstalled skill/pdf-processing
  Installing command/test...
    ✓ Installed command/test

✓ Fixed 2 issue(s)

1 orphaned resource(s) found (not auto-removed):
  - skill/old-skill (claude): Run 'aimgr uninstall skill/old-skill' to remove, or run 'aimgr install skill/old-skill' to add to ai.package.yaml
```

---

## Flags

### `--reset` — Remove Unmanaged Files

After standard repair, scans resource directories (`commands/`, `skills/`, `agents/`) for files that weren't installed by aimgr and offers to remove them.

A file is **unmanaged** if it's not a symlink pointing to your aimgr repository. This includes:
- Regular files manually placed in resource directories
- Symlinks pointing to non-repository locations
- Stale files left behind from manual operations

```bash
# Interactive — asks for confirmation
aimgr repair --reset

# Preview what would be removed
aimgr repair --reset --dry-run

# Remove without prompting
aimgr repair --reset --force
```

Example output:
```bash
$ aimgr repair --reset

Found 2 unmanaged file(s) in resource directories:
  /home/user/project/.claude/commands/old-script.md
  /home/user/project/.claude/skills/manual-skill/SKILL.md

Remove all 2 unmanaged files? [y/N]: y
  ✓ Removed: /home/user/project/.claude/commands/old-script.md
  ✓ Removed: /home/user/project/.claude/skills/manual-skill/SKILL.md

✓ Removed 2 unmanaged file(s)
```

### `--prune-package` — Clean Up `ai.package.yaml`

Validates every resource reference in `ai.package.yaml` against the repository. Removes references to resources or packages that no longer exist.

```bash
# Interactive — offers escalation choices per invalid reference
aimgr repair --prune-package

# Preview what would be removed
aimgr repair --prune-package --dry-run

# Remove all invalid references without prompting
aimgr repair --prune-package --force
```

**Interactive mode** offers a smart escalation flow for each invalid reference:

```
⚠ skill/code-review not found in repo

? How to resolve:
  [1] Run repo sync first (repo sources may be outdated)
  [2] Run repo repair first (repo metadata may be broken)
  [3] Remove from ai.package.yaml
  [4] Skip (do nothing)
Choice [1-4]:
```

This helps you try less destructive options before removing entries. If a sync or repair resolves the issue, the reference is kept.

**Package validation:** For package references (`package/foo`), repair checks both that the package exists AND that all its member resources exist. If some members are missing, you'll see a warning directing you to `aimgr repo repair`.

### `--dry-run` — Preview Without Changes

Shows what would happen without modifying anything. Works with all other flags.

```bash
aimgr repair --dry-run                          # Preview standard repair
aimgr repair --reset --dry-run                  # Preview file removal
aimgr repair --prune-package --dry-run          # Preview manifest cleanup
aimgr repair --reset --prune-package --dry-run  # Preview everything
```

### `--force` — Skip Confirmation Prompts

Skips all interactive confirmation prompts. Use in scripts or CI/CD.

```bash
aimgr repair --reset --force
aimgr repair --prune-package --force
aimgr repair --reset --prune-package --force  # Full cleanup, no prompts
```

### `--project-path` — Target a Different Directory

Repair a project other than the current directory.

```bash
aimgr repair --project-path ~/other-project
```

### `--format` — Output Format

Control output format. Supports `table` (default) and `json`.

```bash
aimgr repair --format json
```

JSON output structure:
```json
{
  "fixed": [
    {
      "resource": "skill/pdf-processing",
      "tool": "claude",
      "issue_type": "broken",
      "description": "Reinstalled skill/pdf-processing"
    }
  ],
  "failed": [],
  "hints": [
    {
      "resource": "skill/old-skill",
      "tool": "claude",
      "issue_type": "orphaned",
      "description": "Run 'aimgr uninstall skill/old-skill' to remove..."
    }
  ],
  "summary": {
    "fixed": 1,
    "failed": 0,
    "hints": 1
  }
}
```

---

## Combining Flags

Flags compose naturally. Use them together for a complete cleanup:

```bash
# Full repair: fix symlinks + remove unmanaged files + clean manifest
aimgr repair --reset --prune-package --force

# Same but preview first
aimgr repair --reset --prune-package --dry-run
```

Execution order:
1. Standard symlink repair (always runs)
2. `--reset` removes unmanaged files
3. `--prune-package` cleans manifest references

---

## Repository Repair

`aimgr repo repair` is a separate command for fixing repository-level metadata issues (not project installations).

```bash
# Fix repository metadata
aimgr repo repair

# Preview changes
aimgr repo repair --dry-run

# JSON output
aimgr repo repair --format json
```

### What It Fixes

| Issue | What Repair Does |
|-------|------------------|
| **Resources without metadata** | Creates missing `.metadata.yaml` files |
| **Orphaned metadata** | Removes `.metadata.yaml` for resources that no longer exist |

### What It Reports (Cannot Auto-Fix)

| Issue | Guidance |
|-------|----------|
| **Type mismatches** | Resource type differs from metadata — manual fix needed |
| **Packages with missing refs** | Package references resources that don't exist — update package definition |

Example output:
```bash
$ aimgr repo repair

Repository Repair
=================

✓ Created metadata for 2 resource(s):
  • command/deploy
  • skill/testing

✓ Removed 1 orphaned metadata file(s):
  • /home/user/.local/share/ai-config/repo/skills/.metadata/old-skill.yaml

Summary: 3 fixed, 0 unfixable
```

---

## Migrating from `verify --fix`

The `--fix` flag on `verify` is deprecated. Replace it with `repair`:

| Old Command | New Command |
|-------------|-------------|
| `aimgr verify --fix` | `aimgr repair` |
| `aimgr repo verify --fix` | `aimgr repo repair` |

The `repair` command provides the same fixes plus additional capabilities (`--reset`, `--prune-package`, `--dry-run`, `--force`).

---

## Common Workflows

### After Cloning a Project

When you clone a project that has an `ai.package.yaml` manifest, resources need to be installed:

```bash
cd newly-cloned-project/
aimgr verify                  # See what's missing
aimgr repair                  # Install everything from manifest
```

### After Updating Repository Sources

When you sync new versions of resources, some project symlinks may become stale:

```bash
aimgr repo sync               # Update resources from sources
aimgr repair                  # Fix any broken symlinks in project
```

### Cleaning Up a Messy Project

When a project has accumulated manual files and stale manifest entries:

```bash
# Preview the full cleanup
aimgr repair --reset --prune-package --dry-run

# Execute it
aimgr repair --reset --prune-package --force
```

### CI/CD Verification

In automated pipelines, verify and repair with JSON output:

```bash
# Check health
output=$(aimgr repair --format json)
failed=$(echo "$output" | jq '.summary.failed')
if [ "$failed" -gt 0 ]; then
  echo "Repair failed for $failed resources"
  echo "$output" | jq '.failed[]'
  exit 1
fi
```

---

## See Also

- **[Getting Started](getting-started.md)** — Installation and basic usage
- **[Output Formats](../reference/output-formats.md)** — JSON output for scripting
- **[Troubleshooting](../reference/troubleshooting.md)** — Common issues and solutions
