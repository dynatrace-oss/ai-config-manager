# Manage Repository

Add sources, sync resources, validate, and maintain the aimgr repository.

The repository (`~/.local/share/ai-config/repo/`) is the central store for all
AI resources. Sources are tracked in `ai.repo.yaml` inside the repository.

For project-level install/uninstall, see [install-uninstall.md](install-uninstall.md).

**Sections:** [Safety](#️-mutating-operations-require-user-approval) · [Init](#initialize-repository) · [Add Sources](#add-sources) · [Sync](#sync-sources) · [Remove Sources](#remove-sources) · [Browse & Inspect](#browse--inspect) · [Validate](#validate-resources-for-developers) · [Verify & Repair](#verify--repair-repository) · [Nuclear Options](#nuclear-options) · [Troubleshooting](#troubleshooting)

---

## ⚠️ Mutating Operations Require User Approval

**Always ask before running:**

- `aimgr repo add` — imports resources from a source
- `aimgr repo sync` — re-imports from all configured sources
- `aimgr repo remove` — removes a source and its resources
- `aimgr repo drop` — removes all resources or deletes repository
- `aimgr repo repair` — fixes metadata

**Safe read-only operations:**

- `aimgr repo list`, `aimgr repo info`, `aimgr repo describe`, `aimgr repo verify`

---

## Initialize Repository

```bash
aimgr repo init
```

Creates the directory structure (`commands/`, `skills/`, `agents/`, `packages/`),
initializes git tracking, and creates `ai.repo.yaml`.

Idempotent — safe to run multiple times. Location determined by:
1. `AIMGR_REPO_PATH` env var
2. `repo.path` in `~/.config/aimgr/aimgr.yaml`
3. Default: `~/.local/share/ai-config/repo/`

---

## Add Sources

Sources are locations containing AI resources. Adding a source imports its
resources and tracks the source in `ai.repo.yaml` for future syncing.

```bash
# Local directory (symlinked — live editing)
aimgr repo add ~/my-skills
aimgr repo add ./local-resources

# GitHub (copied — stable, versioned)
aimgr repo add gh:owner/repo
aimgr repo add gh:owner/repo@v1.0.0       # Pinned version
aimgr repo add https://github.com/owner/repo
aimgr repo add git@github.com:owner/repo.git  # SSH

# Options
aimgr repo add gh:owner/repo --name=my-source  # Custom source name
aimgr repo add gh:owner/repo --filter "skill/*" # Only import skills
aimgr repo add ./resources --force              # Overwrite existing
aimgr repo add ./resources --skip-existing      # Skip conflicts
aimgr repo add ./resources --dry-run            # Preview only
```

### Source Types

| Type | Import Mode | Best For |
|------|-------------|----------|
| Local path | Symlink | Development, rapid iteration |
| GitHub URL | Copy | Production, versioned resources |

### ai.repo.yaml Format

```yaml
version: 1
sources:
  - name: my-local-skills
    path: /home/user/dev/my-skills
  - name: community-catalog
    url: https://github.com/owner/repo
    ref: v1.2.0
```

Maintained automatically by `repo add` and `repo remove`.

---

## Sync Sources

Re-import resources from all configured sources in `ai.repo.yaml`:

```bash
aimgr repo sync                   # Overwrites existing (default)
aimgr repo sync --skip-existing   # Don't overwrite
aimgr repo sync --dry-run         # Preview only
```

**When to sync:** After upstream changes, to refresh all sources, or after
editing `ai.repo.yaml` manually.

---

## Remove Sources

`repo remove` operates on **sources** (symmetrical with `repo add`), not
individual resources. It removes the source entry from `ai.repo.yaml` and
by default cleans up resources that came from that source.

```bash
# By name, path, or URL
aimgr repo remove my-source
aimgr repo remove ~/my-resources/
aimgr repo remove https://github.com/owner/repo

# Preview
aimgr repo remove my-source --dry-run

# Remove source but keep its resources
aimgr repo remove my-source --keep-resources
```

---

## Browse & Inspect

```bash
aimgr repo list                       # All resources
aimgr repo list skill/*               # Filter by type
aimgr repo list --source my-source    # Filter by source
aimgr repo list --format=json         # JSON output

aimgr repo describe skill/name        # Detailed resource info
aimgr repo describe "skill/*"         # Multiple matches → summary
aimgr repo info                       # Repository stats and sources
aimgr repo info --format=json         # JSON output
```

---

## Validate Resources (for Developers)

Validate that resources are compatible with aimgr before publishing.

### Quick Validation

```bash
# Dry-run: validate without importing (read-only)
aimgr repo add ./my-skill --dry-run

# Exit code: 0 = valid, 1 = failed
```

### What Gets Validated

- **Skills** — Directory with `SKILL.md`, valid frontmatter, naming rules
- **Agents** — Single `.md` file with YAML frontmatter
- **Commands** — `.md` files in a `commands/` directory
- **Packages** — `.package.json` with valid resource references

### Naming Rules

- Lowercase alphanumeric + hyphens only
- No consecutive hyphens, no leading/trailing hyphens
- Max 64 characters per segment
- Directory name must match `name` in frontmatter

### Validation Workflow

```bash
# 1. Validate format
aimgr repo add ./my-skill --dry-run

# 2. Add to repository
aimgr repo add ./my-skill

# 3. Test installation
cd /tmp/test-project
aimgr install skill/my-skill
ls .claude/skills/my-skill/SKILL.md
```

### CI/CD Integration

```yaml
# GitHub Actions
- name: Validate resources
  run: |
    go install github.com/hk9890/ai-config-manager@latest
    aimgr repo add . --dry-run --format=json
```

---

## Verify & Repair Repository

### Verify (read-only)

Check metadata consistency, orphaned files, and package references:

```bash
aimgr repo verify                     # Check all
aimgr repo verify skill/*             # Check only skills
aimgr repo verify --format=json       # JSON output
```

**Exit code:** 0 = clean, 1 = errors found.

### Repair

Auto-fix metadata issues:

```bash
aimgr repo repair                     # Fix all auto-fixable issues
aimgr repo repair --dry-run           # Preview fixes
```

| Issue | Action |
|-------|--------|
| Resource without metadata | Creates missing `.metadata.yaml` |
| Orphaned metadata (resource gone) | Removes orphaned metadata file |
| Type mismatch | Reports — requires manual fix |
| Package with missing refs | Reports — update package definition |

---

## Nuclear Options

### Drop All Resources

```bash
# Soft drop: remove resources, keep ai.repo.yaml and git history
aimgr repo drop

# Rebuild from sources
aimgr repo sync

# Full delete: remove entire repository directory
aimgr repo drop --full-delete         # Confirmation prompt
aimgr repo drop --full-delete --force # Skip prompt
```

### Prune Workspace Cache

Remove cached git clones no longer referenced by any source:

```bash
aimgr repo prune                      # Interactive
aimgr repo prune --dry-run            # Preview
aimgr repo prune --force              # Skip confirmation
```

---

## Troubleshooting

### Resource Not Found

```bash
aimgr repo list | grep resource-name  # Check exact name
aimgr repo sync                       # Refresh from sources
```

Names are case-sensitive, use hyphens: `skill/my-skill` not `skill/My_Skill`.

### Sync Fails

```bash
# Check source accessibility
aimgr repo info                       # View configured sources

# For GitHub: verify access
ssh -T git@github.com
curl -I https://github.com

# Failed sources don't block others — remaining sources still sync
```

### Repository Corruption

```bash
aimgr repo verify                     # Diagnose
aimgr repo repair                     # Auto-fix metadata

# If unfixable: rebuild from sources
aimgr repo drop
aimgr repo sync
```

### Duplicate Source Names

```bash
# Use --name to differentiate
aimgr repo add ~/resources-v2 --name=my-source-v2
```

📚 Run `aimgr repo --help` or `aimgr repo [command] --help` for full flag reference.
