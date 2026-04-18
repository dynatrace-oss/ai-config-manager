# Manage Repository

Add sources, sync resources, validate, and maintain the aimgr repository.

The repository (`~/.local/share/ai-config/repo/`) is the central store for all
AI resources. Sources are tracked in `ai.repo.yaml` inside the repository.

For project-level install/uninstall, see [install-uninstall.md](install-uninstall.md).

**Sections:** [Safety](#️-mutating-operations-require-user-approval) · [Init](#initialize-repository) · [Manifest Workflow](#manifest-workflow) · [Add Sources](#add-sources) · [Sync](#sync-sources) · [Remove Sources](#remove-sources) · [Browse & Inspect](#browse--inspect) · [Validate](#validate-resources-for-developers) · [Verify & Repair](#verify--repair-repository) · [Nuclear Options](#nuclear-options) · [Rebuild](#rebuild-repository-content) · [Troubleshooting](#troubleshooting)

---

## ⚠️ Mutating Operations Require User Approval

**Always ask before running:**

- `aimgr repo add` — imports resources from a source
- `aimgr repo apply-manifest` — merges an external manifest into local `ai.repo.yaml`
- `aimgr repo sync` — re-imports from all configured sources
- `aimgr repo rebuild` — soft reset + re-import from configured sources
- `aimgr repo remove` — removes a source and its resources
- `aimgr repo drop` — removes all resources or deletes repository
- `aimgr repo repair` — fixes metadata

**Safe read-only operations:**

- `aimgr repo list`, `aimgr repo info`, `aimgr repo describe`, `aimgr repo verify`
- `aimgr repo show-manifest`

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

## Manifest Workflow

`ai.repo.yaml` is the local repository manifest. The main collaboration model is to publish that file somewhere central and let others apply it into their own local repo.

```bash
# Import a shared manifest from a central location
aimgr repo apply-manifest https://example.com/platform/ai.repo.yaml

# Import another shared manifest; sources are merged locally
aimgr repo apply-manifest https://example.com/team/ai.repo.yaml

# Inspect or publish your current local manifest
aimgr repo show-manifest
aimgr repo show-manifest > ai.repo.yaml

# Local/shared file also works
aimgr repo apply-manifest ./ai.repo.yaml
```

### Command roles

- `aimgr repo init` — bootstrap an empty local repository and create `ai.repo.yaml`
- `aimgr repo show-manifest` — read and print the current local `ai.repo.yaml` so you can inspect it or publish it for others
- `aimgr repo apply-manifest <path-or-url>` — read a shared manifest and merge it into the local `ai.repo.yaml`

### Intended sharing model

- one team or user publishes a real `ai.repo.yaml` somewhere central
- other users run `apply-manifest` against that shared file
- users may apply multiple shared manifests; aimgr merges the sources into one local manifest
- if a user wants to share their current setup, they use `show-manifest` and commit or upload that output

### Accepted `apply-manifest` inputs

- local path to `ai.repo.yaml`
- HTTP(S) URL pointing directly to `ai.repo.yaml`
- stdin via `-` or `/dev/stdin` (convenience input)

### Notes

- `apply-manifest` is mutating, so ask before running it
- the primary collaboration flow is publishing and consuming a real `ai.repo.yaml`
- stdin round-trips like `show-manifest | apply-manifest -` are supported, but only as a convenience
- stdin and remote manifests reject relative `path:` sources because they have no safe local resolution base

---

## Add Sources

Sources are locations containing AI resources. Adding a source imports its
resources and tracks the source in `ai.repo.yaml` for future syncing.

```bash
# Local directory (symlinked — live editing)
aimgr repo add local:~/my-skills
aimgr repo add local:./local-resources

# GitHub (copied — stable, versioned)
aimgr repo add gh:owner/repo
aimgr repo add gh:owner/repo --ref v1.0.0  # Preferred pinned version syntax
aimgr repo add gh:owner/repo --subpath skills/core
aimgr repo add gh:owner/repo --ref main --subpath skills/core

# Legacy compatibility forms (still accepted when unambiguous)
aimgr repo add gh:owner/repo@v1.0.0
aimgr repo add gh:owner/repo@main/skills
aimgr repo add https://github.com/owner/repo
aimgr repo add git@github.com:owner/repo.git  # SSH

# Any Git host (HTTPS or SSH)
aimgr repo add https://bitbucket.org/org/repo
aimgr repo add https://gitlab.com/group/project
aimgr repo add git@bitbucket.org:org/repo.git

# Monorepo subpath (via .git/ delimiter)
aimgr repo add https://bitbucket.org/org/monorepo.git/path/to/resources
aimgr repo add https://gitlab.com/group/project.git/ai-resources

# Options
aimgr repo add gh:owner/repo --name=my-source  # Custom source name
aimgr repo add gh:owner/repo --filter "skill/*" # Only import skills
aimgr repo add local:./resources --force              # Overwrite existing
aimgr repo add local:./resources --skip-existing      # Skip conflicts
aimgr repo add local:./resources --dry-run            # Preview only
```

### Source Types

| Format | Example | Import Mode |
|--------|---------|-------------|
| GitHub shorthand (preferred) | `gh:owner/repo` + `--ref` / `--subpath` flags | Copy |
| GitHub shorthand (legacy compatibility) | `gh:owner/repo@ref`, `gh:owner/repo/subpath`, `gh:owner/repo@ref/path` (unambiguous only) | Copy |
| Local directory | `local:./path/to/dir` | Symlink |
| HTTPS Git URL | `https://github.com/owner/repo.git` | Copy |
| HTTPS monorepo | `https://host/org/repo.git/subpath` | Copy |
| SSH Git URL | `git@github.com:owner/repo.git` | Copy |

Compatibility caveat: inline `gh:owner/repo@ref/path` is compatibility-only and
only unambiguous for single-segment `ref` + single-segment `path` (for example
`@main/skills`). Slash-containing refs or multi-segment paths are rejected as
ambiguous. In those cases use explicit flags, for example:

```bash
aimgr repo add gh:owner/repo --ref release/v1 --subpath skills/core
```

⚠️ **v3.0+:** All sources require explicit prefixes. Bare paths like `owner/repo` or `./path` are rejected with actionable error messages.

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

Maintained automatically by `repo add`, `repo remove`, and `repo apply-manifest`.

---

## Sync Sources

Re-import resources from all configured sources in `ai.repo.yaml`:

```bash
aimgr repo sync                   # Overwrites existing (default)
aimgr repo sync --skip-existing   # Don't overwrite
aimgr repo sync --dry-run         # Preview only
aimgr repo sync --prune           # Reconcile stale source-owned resources/packages
```

**When to sync:** After upstream changes, to refresh all sources, or after
editing `ai.repo.yaml` manually.

By default, `repo sync` is non-pruning: it refreshes configured sources without
removing previously imported source-owned resources/packages that have become
stale due to include, subpath, or discovery changes.

Use `--prune` when you need reconciliation cleanup. In prune mode, sync removes
stale source-owned resources/packages that no longer match each source's
effective definition after include/subpath/discovery filtering.

`repo sync --prune` is different from `repo prune`: `repo sync --prune`
reconciles source-owned resources/packages, while `repo prune` removes
unreferenced `.workspace/` Git caches.

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
aimgr repo add local:./my-skill --dry-run

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
aimgr repo add local:./my-skill --dry-run

# 2. Add to repository
aimgr repo add local:./my-skill

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
    go install github.com/dynatrace-oss/ai-config-manager/v3/cmd/aimgr@latest
    aimgr repo add local:. --dry-run --format=json
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

### Drop All Resources / Full Delete

```bash
# Preferred full refresh: soft reset + full re-import in one command
aimgr repo rebuild

# Full delete: remove entire repository directory
aimgr repo drop --full-delete         # Confirmation prompt
aimgr repo drop --full-delete --force # Skip prompt
```

`repo rebuild` is the primary reset workflow when you want a clean re-import
from current `ai.repo.yaml` source definitions.

Use manual soft-drop + sync only if you intentionally want to separate those
steps:

```bash
# Soft drop only (preserves ai.repo.yaml and sources)
aimgr repo drop

# Re-import later when ready
aimgr repo sync
```

---

## Rebuild Repository Content

`repo rebuild` performs the soft drop + sync lifecycle in one locked operation.

```bash
aimgr repo rebuild
aimgr repo rebuild --dry-run
```

Use rebuild when imported state or metadata looks unreliable and you want a
clean re-import from existing `ai.repo.yaml` source definitions without running
separate drop/sync commands.

Behavior summary:

- Preserves `ai.repo.yaml` and configured sources
- Clears imported resources and derived local state, then re-imports sources
- `--dry-run` previews sync results only (does not perform the soft drop)

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
aimgr repo rebuild
```

### Duplicate Source Names

```bash
# Use --name to differentiate
aimgr repo add local:~/resources-v2 --name=my-source-v2
```

📚 Run `aimgr repo --help` or `aimgr repo [command] --help` for full flag reference.
