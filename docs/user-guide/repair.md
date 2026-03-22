# Repairing Resources

`aimgr repair` reconciles your project's **owned resource directories** with the
effective project manifest (`ai.package.yaml` plus optional `ai.package.local.yaml`).
It is the replacement for the deprecated `verify --fix` workflow.

Owned resource directories are tool-specific `commands/`, `skills/`, and `agents/` folders
(for example `.claude/commands`, `.opencode/skills`, `.github/skills`).

---

## Quick Start

```bash
# Diagnose drift (read-only)
aimgr verify

# Reconcile owned directories to the effective manifest view
aimgr repair
```

---

## New Model: Owned Directories + Manifest Reconciliation

The project model is now:

- `aimgr clean` empties owned resource directories (without editing `ai.package.yaml`)
- `aimgr repair` restores/reconciles those directories to match the effective manifest view
  (`ai.package.yaml` + optional `ai.package.local.yaml` overlay)

`repair` performs reconciliation in this order:

1. Validate and load project manifest data (committed + optional local overlay)
2. Expand `package/*` entries to concrete resources
3. Install/fix declared resources first
4. Remove remaining undeclared content from owned directories
5. Optionally prune invalid manifest refs when `--prune-package` is used

This keeps recovery safer: declared resources are restored before undeclared content is removed.

If the local overlay references resources whose sources are unavailable in your local
repository, `repair` reports those failures explicitly rather than silently ignoring them.

---

## What `repair` Does

With a valid effective project manifest (`ai.package.yaml` + optional
`ai.package.local.yaml`), `aimgr repair`:

| Category | What It Does |
|---|---|
| Declared but missing | Installs resource |
| Declared but broken/conflicting | Replaces and reinstalls resource |
| Declared but manually deleted | Reinstalls resource |
| Undeclared content in owned dirs | Removes it |

### Important: Manual Deletion Is Not Permanent

If a resource is still declared in the effective project manifest, `aimgr repair` will reinstall it.

To remove a resource permanently, update the manifest that declares it (for example, use
`aimgr uninstall <resource>` **without** `--no-save`).

---

## Flags

### `--dry-run` — Preview Reconciliation Plan

Show planned installs, fixes/replacements, removals, and prune actions without applying changes.

```bash
aimgr repair --dry-run
aimgr repair --prune-package --dry-run
```

### `--prune-package` — Manifest Cleanup (Separate Concern)

`--prune-package` only cleans invalid references from `ai.package.yaml`.
It is **separate** from folder reconciliation.

- Reconciliation aligns owned directories to declared resources
- `--prune-package` edits the manifest by removing invalid references

```bash
# Reconcile folders + prune invalid manifest references
aimgr repair --prune-package

# Preview both phases
aimgr repair --prune-package --dry-run
```

### `--project-path` — Target Another Project

```bash
aimgr repair --project-path ~/other-project
```

### `--format` — Output Format

Supported: `table` (default), `json`

```bash
aimgr repair --format json
```

JSON output includes `dry_run`, `planned`, `applied`, `failed`, and `summary` sections.

---

## `clean` + `repair` Workflow

For "wipe then restore" behavior:

```bash
aimgr clean && aimgr repair
```

Use this when you want a deterministic reset of owned resource directories,
then restore exactly what the effective manifest view declares.

---

## Migration / Upgrade Notes

CLI behavior changed for project cleanup/repair commands.

| Old usage | New usage |
|---|---|
| `aimgr repair --reset` | `aimgr repair` |
| `aimgr repair --force` | `aimgr repair` |
| `aimgr clean --yes` | `aimgr clean` |
| `aimgr repair --reset --force` | `aimgr clean && aimgr repair` |

Notes:

- `--reset` and `--force` are removed from `aimgr repair`
- `--yes` is removed from `aimgr clean`
- `--dry-run` remains the safety preview mechanism
- `--prune-package` remains available for manifest cleanup

---

## `verify --fix` Deprecation

`aimgr verify --fix` is deprecated.

Use:

| Old Command | New Command |
|---|---|
| `aimgr verify --fix` | `aimgr repair` |
| `aimgr repo verify --fix` | `aimgr repo repair` |

The deprecated wrapper follows repair reconciliation behavior and emits a deprecation warning.

---

## Repository Repair (Different Scope)

`aimgr repo repair` is for repository metadata integrity, not project directory reconciliation.

```bash
aimgr repo repair
aimgr repo repair --dry-run
aimgr repo repair --format json
```

---

## Common Workflows

### After Cloning a Project

```bash
cd newly-cloned-project/
aimgr verify
aimgr repair
```

### After Manual Deletions in Tool Folders

```bash
# repair restores what manifest still declares
aimgr repair

# if you intended permanent removal, uninstall and save manifest change first
aimgr uninstall skill/some-skill
```

### Clean Rebuild of Owned Dirs

```bash
# optional preview first
aimgr repair --dry-run

# wipe + restore from manifest
aimgr clean && aimgr repair
```

### Preview Before Applying

```bash
aimgr repair --prune-package --dry-run
aimgr repair --format json --dry-run
```

---

## See Also

- **[Getting Started](getting-started.md)** — Installation and basic usage
- **[Concepts](concepts.md)** — `ai.package.yaml` and `ai.package.local.yaml` roles and merge behavior
- **[Team and Multi-Project Workflows](team-workflows.md)** — shared baseline + local overlay workflows
- **[Output Formats](../reference/output-formats.md)** — JSON output for scripting
- **[Troubleshooting](../reference/troubleshooting.md)** — Common issues and solutions
