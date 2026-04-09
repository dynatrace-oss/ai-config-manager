# Sources Guide

This guide explains how to manage sources in your aimgr repository. Sources are locations (local paths or remote URLs) containing AI resources that you want to import and track.

---

## Source Syntax

Every source requires an **explicit prefix or scheme**. There is no implicit format guessing — this avoids ambiguity (e.g., `owner/repo` could be GitHub, Bitbucket, or GitLab).

### Quick Reference

| Format | Example | Type |
|--------|---------|------|
| `gh:owner/repo` | `gh:my-org/ai-tools` | GitHub |
| `gh:owner/repo/path/to/marketplace.json` | `gh:my-org/ai-tools/.claude-plugin/marketplace.json` | GitHub marketplace file |
| `gh:owner/repo@ref` | `gh:my-org/ai-tools@v1.0.0` | GitHub (pinned) |
| `gh:owner/repo@ref/path` | `gh:my-org/ai-tools@main/skills` | GitHub (subpath) |
| `https://host/path` | `https://github.com/owner/repo` | HTTPS Git URL |
| `https://host/repo.git/path` | `https://git.example.com/scm/proj/repo.git/skills` | HTTPS + subpath |
| `https://host/repo.git/path/to/marketplace.json` | `https://github.com/owner/repo.git/.claude-plugin/marketplace.json` | HTTPS marketplace file |
| `http://host/path` | `http://git.internal.com/owner/repo` | HTTP Git URL |
| `git@host:owner/repo.git` | `git@github.com:owner/repo.git` | SSH Git URL |
| `local:path` | `local:./my-resources` | Local directory |
| `local:path/to/marketplace.json` | `local:./.claude-plugin/marketplace.json` | Local marketplace file |

### GitHub Shorthand (`gh:`)

The `gh:` prefix is a convenience for GitHub repositories. It constructs the full `https://github.com/...` URL automatically.

```bash
gh:owner/repo                  # → https://github.com/owner/repo
gh:owner/repo@v1.0.0           # Pinned to tag v1.0.0
gh:owner/repo@main             # Pinned to branch main
gh:owner/repo/skills/frontend  # Only resources under skills/frontend
gh:owner/repo@v1.0.0/skills    # Pinned version + subpath
```

### HTTPS / HTTP URLs

Use full URLs for **any Git host** — GitHub, GitLab, Bitbucket, self-hosted Gitea, etc.

```bash
https://github.com/owner/repo
https://gitlab.com/group/project
https://bitbucket.org/org/repo
http://git.internal.company.com/team/resources
```

#### Subpath via `.git/` Delimiter

For non-GitHub hosts that don't support `/tree/ref/path` syntax, you can specify a subpath by including `.git/` in the URL. Everything before `.git/` is the clone URL; everything after is the subpath within the repo.

```bash
# Clone https://git.example.com/scm/PROJ/repo.git, then look in skills/
https://git.example.com/scm/PROJ/repo.git/skills

# Bitbucket Server example — clone URL + subpath
https://bitbucket.example.com/scm/TEAM/ai-resources.git/skills/frontend

# Deep subpath
https://gitlab.internal.com/group/mono-repo.git/packages/ai/skills
```

> **Note:** This only applies to generic HTTPS/HTTP URLs. For GitHub, prefer the `gh:owner/repo/subpath` shorthand or `/tree/ref/path` URL syntax instead.

GitHub HTTPS URLs also support `/tree/ref` and `/tree/ref/path` syntax:

```bash
https://github.com/owner/repo/tree/v1.0.0
https://github.com/owner/repo/tree/main/skills/frontend
```

### SSH URLs (`git@`)

Use SSH URLs when your Git host is configured with SSH keys:

```bash
git@github.com:owner/repo.git
git@gitlab.com:group/project.git
git@bitbucket.org:org/repo.git
```

> **Note:** SSH URLs are converted to HTTPS internally for cloning. Ensure your system Git has proper credentials configured (e.g., via `gh auth login` or SSH agent).

### Local Paths (`local:`)

Use the `local:` prefix for directories on your filesystem:

```bash
local:./my-resources           # Relative to current directory
local:../shared-resources      # Parent directory
local:/home/user/my-skills     # Absolute path
local:~/my-skills              # Home directory
```

Local sources are **symlinked** (not copied), so changes to the source files immediately reflect in the repository.

### Common Mistakes

```bash
# WRONG — bare owner/repo is ambiguous
aimgr repo add my-org/ai-tools
# → Error: use "gh:my-org/ai-tools" for GitHub or provide a full URL

# WRONG — bare path without local: prefix
aimgr repo add ./my-resources
# → Error: use "local:./my-resources" for local paths

# CORRECT
aimgr repo add gh:my-org/ai-tools
aimgr repo add local:./my-resources
```

---

## Overview

**aimgr** uses a **repository-local** approach to source management. Each repository tracks its own sources in an `ai.repo.yaml` manifest file, which is version-controlled and portable.

### Key Concepts

| Concept | Description |
|---------|-------------|
| **Source** | A location (local path or remote URL) containing AI resources |
| **ai.repo.yaml** | Manifest file tracking all configured sources |
| **Import Mode** | How resources are stored - `symlink` for paths, `copy` for URLs |
| **Sync** | Re-import resources from configured sources to get latest changes |

### Benefits

- **Self-contained repositories**: All source information lives in the repository
- **Git-tracked manifests**: Share source configurations via version control
- **Automatic synchronization**: Re-import resources with a single command
- **Orphan cleanup**: Automatically remove resources when sources are removed

---

## The ai.repo.yaml File

The `ai.repo.yaml` file is automatically created and maintained in your repository root (`~/.local/share/ai-config/repo/ai.repo.yaml` by default). It tracks all sources you've added and their metadata.

### File Format

```yaml
version: 1
sources:
  # Local source (uses symlink mode)
  - name: my-local-commands
    path: /home/user/my-resources
    
  # Remote source (uses copy mode)
  - name: agentskills-catalog
    url: https://github.com/agentskills/catalog
    ref: main
    subpath: resources
```

### Field Reference

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `version` | integer | Manifest format version (currently 1) | Yes |
| `sources` | array | List of source configurations | Yes |
| `name` | string | Unique identifier for the source | Yes |
| `path` | string | Absolute path to local directory (for local sources) | One of path/url |
| `url` | string | Git repository URL (for remote sources) | One of path/url |
| `ref` | string | Git branch/tag/commit (for remote sources) | No |
| `subpath` | string | Subdirectory within repository (for remote sources) | No |
| `include` | array of string | Resource filter patterns (same syntax as `--filter`) | No |

**Note:** Import mode is implicit based on source type. Path sources use `symlink` mode; URL sources use `copy` mode.

---

## Sharing source configuration with `ai.repo.yaml` (`show-manifest` / `apply-manifest` v1)

Use `aimgr repo apply-manifest <path-or-url>` to import a shared `ai.repo.yaml` into your local repository config, and `aimgr repo show-manifest` to print your current local manifest when you want to publish it for others.

### Command responsibilities

- `aimgr repo init`: local repository bootstrap only (create repo layout, git, initial `ai.repo.yaml`)
- `aimgr repo show-manifest`: print the current local `ai.repo.yaml` so you can inspect it or publish it somewhere shareable
- `aimgr repo apply-manifest <path-or-url>`: load a shared manifest and merge its sources into local `ai.repo.yaml` (auto-initializes if needed)
- Deferred for future versions: export/lockfile workflows (not part of `repo apply-manifest` v1)

### Collaboration model (team baseline)

The intended sharing model is:

1. A team agrees on shared sources for a project and publishes them as one central `ai.repo.yaml`.
2. Developers and CI run `aimgr repo apply-manifest <path-or-url>` against that shared file.
3. Users can apply more than one shared manifest; each apply merges additional sources into the same local `ai.repo.yaml`.
4. If a user wants to share their own current setup, they run `aimgr repo show-manifest` and commit or upload that output somewhere others can access it.

Example:

```bash
# import team baseline
aimgr repo apply-manifest https://example.com/team/project-a/ai.repo.yaml

# add another shared manifest on top
aimgr repo apply-manifest https://example.com/data-science/ai.repo.yaml

# publish your resulting local config for someone else
aimgr repo show-manifest > ai.repo.yaml
```

### Accepted `repo apply-manifest` inputs in v1

`repo apply-manifest` accepts only explicit manifest inputs:

1. Local file path to `ai.repo.yaml`
2. HTTP(S) URL pointing directly to `ai.repo.yaml`
3. Stdin via `-` or `/dev/stdin` (convenience input, not the primary sharing model)

For team/shared manifests on GitHub, use a raw file URL when pinning to a tag/ref.
Example:

```bash
aimgr repo apply-manifest https://raw.githubusercontent.com/my-org/platform-configs/v1.2.0/manifests/ai.repo.yaml
```

Branch-based raw file URLs are also valid, but tags/releases are recommended for reproducibility:

```bash
aimgr repo apply-manifest https://raw.githubusercontent.com/my-org/platform-configs/main/manifests/ai.repo.yaml
```

GitHub web page URLs such as `/blob/<ref>/.../ai.repo.yaml` and `/tree/<ref>/.../ai.repo.yaml` are not valid `repo apply-manifest` inputs because they do not represent a direct manifest file endpoint.

Examples:

```bash
aimgr repo show-manifest
aimgr repo apply-manifest ./ai.repo.yaml
aimgr repo apply-manifest /tmp/team/ai.repo.yaml
aimgr repo apply-manifest https://example.com/platform/ai.repo.yaml
```

Not supported in v1:

- Bare repository URLs (for example `https://github.com/org/repo`)
- Implicit discovery of manifests inside a repository URL

### Bootstrap and merge flows

Fresh repository bootstrap from a shared manifest:

```bash
# No prior repo init required
aimgr repo apply-manifest ./ai.repo.yaml
aimgr repo sync
aimgr install
```

Recommended team baseline flow (local and CI):

```bash
aimgr repo apply-manifest <shared-manifest-url>
aimgr repo sync
aimgr install
```

Merge into an existing repository with local sources:

```bash
# Existing ai.repo.yaml already contains local/team sources
aimgr repo apply-manifest https://example.com/platform/ai.repo.yaml
aimgr repo sync
```

In merge mode:
- Existing sources are kept unless there is a name/location conflict
- Identical sources become no-ops (idempotent)
- `include` filters are replaced by default for same-location updates (`--include-mode replace`)
- Use `--include-mode preserve` to keep existing local include filters

Important for re-apply workflows:
- `repo apply-manifest` is **additive**. Re-applying an updated shared manifest does not remove local sources that are missing from the new incoming file.
- If a shared source is intentionally removed upstream, remove stale local entries explicitly with `aimgr repo remove <name|path|url>`.

Example stale-source cleanup after a shared manifest update:

```bash
# Re-apply the updated shared manifest
aimgr repo apply-manifest https://example.com/team/project-a/ai.repo.yaml

# Remove a source that was intentionally dropped from the shared manifest
aimgr repo remove source-a

# Refresh resources and install
aimgr repo sync
aimgr install
```

Applying multiple manifests is also valid:

```bash
aimgr repo apply-manifest https://example.com/platform/ai.repo.yaml
aimgr repo apply-manifest https://example.com/team/ai.repo.yaml
aimgr repo sync
```

After those commands, the local `ai.repo.yaml` contains the merged source list from both shared manifests.

### Shareable manifest schema (v1)

Shareable manifests are human-authored and portable:

```yaml
version: 1
sources:
  - name: team-local
    path: ./resources
    include:
      - skill/pdf*
      - command/lint-*

  - name: community-tools
    url: https://github.com/example/ai-tools
    ref: v1.2.0
    include:
      - skill/*
      - package/web-*
```

Rules:

- `source.include` uses the same glob syntax as `aimgr repo add --filter`
- `id` is local/internal state and must not be required in shareable manifests
- A source must specify exactly one of `path` or `url`

### Merge and conflict behavior

When applying a manifest onto the local `ai.repo.yaml` with `repo apply-manifest`:

- **New source name** → add source
- **Same source name + identical canonical source** (`path/url/subpath`) with updated `ref` → supported update (no conflict)
- **Same source name + identical effective definition** → no-op (idempotent)
- **Same source name + different canonical source location/identity** (for example different `path`, `url`, or `subpath`) → conflict (must be explicit, no silent overwrite)
- **Duplicate names within the incoming manifest** → validation error

Canonical resource collisions are also explicit failures: if different sources resolve to the same canonical resource ID (`type/name`), apply fails and reports the collision instead of silently choosing one.

Repeated apply of the same manifest should be idempotent.

Team guidance:

- standardize source naming conventions (canonical source names)
- ensure only one source owns each canonical resource name
- use `include` filters to avoid overlapping canonical names between sources

### Relative path resolution

- Applying a **local manifest file**: relative `path` values are resolved relative to the manifest file's directory
- Applying a **stdin manifest** (`-` or `/dev/stdin`): relative `path` values are rejected in v1 (no manifest directory exists for resolution)
- Applying a **remote HTTP(S) manifest**: relative `path` values are rejected in v1 (ambiguous on the receiver machine)
- Absolute `path` values remain valid but are only practical for machine-local setups

For cross-machine sharing, prefer `url` sources in remote manifests.

Stdin support (`-` or `/dev/stdin`) is available for advanced shell workflows, but the normal collaboration flow is publishing a real `ai.repo.yaml` and having others apply it from a file path or URL.

---

## Local Sources

Local sources point to directories on your filesystem. Resources are automatically **symlinked** for live editing.

### Adding Local Sources

```bash
# Add from local directory
aimgr repo add local:~/my-skills
aimgr repo add local:/opt/team-resources
aimgr repo add local:./local-resources

# With custom name
aimgr repo add local:~/my-skills --name=personal-skills
```

### How Symlink Mode Works

When you add a local source:
1. Resources are discovered in the source directory
2. Symbolic links are created in your repository pointing to the original files
3. Changes to source files immediately reflect in the repository

### Benefits

- **Live editing**: Changes appear instantly without re-importing
- **No duplication**: Original files stay in their location
- **Perfect for development**: Quick iteration without sync commands

### Use Cases

- Local development and testing
- Personal resource collections
- Active development with rapid iteration

### Example ai.repo.yaml Entry

```yaml
sources:
  - name: my-local-skills
    path: /home/user/dev/my-skills
```

---

## Remote Sources

Remote sources point to Git repositories (GitHub, GitLab, etc.). Resources are automatically **copied** to your repository.

### Adding Remote Sources

```bash
# Basic GitHub URL
aimgr repo add https://github.com/owner/repo

# With custom name
aimgr repo add https://github.com/owner/repo --name=community-tools
```

### How Copy Mode Works

When you add a remote source:
1. The repository is cloned to a workspace cache
2. Resources are discovered and copied to your repository
3. Source is tracked in `ai.repo.yaml` for future syncing

### Benefits

- **Stable, versioned resources**: Pin to specific versions
- **Works offline**: No network needed after initial import
- **No external dependencies**: Resources are self-contained

### Use Cases

- Production environments
- Shared team resources
- Community packages
- Versioned resource collections

### Example ai.repo.yaml Entry

```yaml
sources:
  - name: community-catalog
    url: https://github.com/owner/repo
    ref: v1.2.0
```

---

## GitHub-Specific Syntax

GitHub sources support special syntax for specifying branches, tags, and subdirectories via the `gh:` shorthand or full HTTPS URLs.

### URL Formats

```bash
# GitHub shorthand
aimgr repo add gh:owner/repo

# Standard GitHub URL
aimgr repo add https://github.com/owner/repo

# Git URL with .git extension
aimgr repo add https://github.com/owner/repo.git

# SSH URL (requires configured keys)
aimgr repo add git@github.com:owner/repo.git
```

### Specifying Refs (Branches/Tags)

Use `@ref` with the `gh:` shorthand:

```bash
# Specific branch
aimgr repo add gh:owner/repo@develop

# Specific tag (recommended for stability)
aimgr repo add gh:owner/repo@v1.2.0
```

Or use `/tree/ref` with full GitHub URLs:

```bash
aimgr repo add https://github.com/owner/repo/tree/v1.2.0
aimgr repo add https://github.com/owner/repo/tree/develop
```

### Specifying Subpaths

Add a subpath after the repository to target a specific directory:

```bash
# Resources in a subdirectory (shorthand)
aimgr repo add gh:owner/repo/skills/frontend

# Subpath with ref (shorthand)
aimgr repo add gh:owner/repo@v1.0.0/skills

# Subpath via full URL
aimgr repo add https://github.com/owner/repo/tree/main/skills/frontend
```

### Complete Examples

```bash
# All resources from latest main branch
aimgr repo add gh:owner/repo

# Specific version
aimgr repo add gh:owner/repo@v2.0.0 --name=stable-tools

# Specific directory and version
aimgr repo add gh:owner/mono-repo@v1.0.0/skills/frontend

# SSH with custom name
aimgr repo add git@github.com:owner/repo.git --name=my-tools

# Non-GitHub hosts
aimgr repo add https://bitbucket.org/org/repo
aimgr repo add https://gitlab.com/group/project
aimgr repo add git@bitbucket.org:org/repo.git
```

### Auto-Discovery

When adding from GitHub, `aimgr` automatically discovers resources in standard locations:

**Skills** are searched in order:
1. Direct path (if subpath specified)
2. `skills/`, `.claude/skills/`, `.opencode/skills/`, `.github/skills/`, etc.
3. Recursive search (max depth 5)

**Commands** are searched in:
1. `commands/`, `.claude/commands/`, `.opencode/commands/`
2. Recursive search for `.md` files

**Agents** are searched in:
1. `agents/`, `.claude/agents/`, `.opencode/agents/`
2. Recursive search for `.md` files

### Authentication

Authentication is handled by your system Git configuration. `aimgr` does not manage credentials.

For **HTTPS** access to private repositories:

```bash
# Recommended: use GitHub CLI (configures credential helper)
brew install gh  # or apt install gh
gh auth login

# Alternative: configure Git credential helper directly
git config --global credential.helper store
```

For **SSH** access:

```bash
# Test SSH authentication
ssh -T git@github.com
ssh -T git@bitbucket.org

# Then use SSH URLs
aimgr repo add git@github.com:owner/private-repo.git
```

> **Note:** GitHub does not support password authentication for Git operations. Use a Personal Access Token or `gh auth login` instead.

### Workspace Caching

Remote Git operations use a workspace cache stored inside your configured repo path:

- **Cache root**: `<repo>/.workspace/`
- **Per-repo cache dirs**: `<repo>/.workspace/<cache-hash>/`
- **Shared cache metadata**: `<repo>/.workspace/.cache-metadata.json`

Concurrency behavior for mutations:

- Repo-wide mutation lock: `<repo>/.workspace/locks/repo.lock`
- Per-cache mutation lock: `<repo>/.workspace/locks/caches/<cache-hash>.lock`
- Shared workspace metadata lock: `<repo>/.workspace/locks/workspace-metadata.lock`

Lock ordering is: **repo -> cache -> workspace metadata**.

This means concurrent `repo add/sync/remove/drop/apply-manifest/init/repair/verify --fix/prune`
operations are serialized safely, and cache metadata updates are protected against
lost updates.

Repo-managed state files are also written with atomic replacement (temp file in
same directory, file sync, rename replacement, and parent-directory sync where
supported). This includes `ai.repo.yaml`, `.metadata/sources.json`, resource
metadata under `.metadata/...`, and `.workspace/.cache-metadata.json`.

---

## Syncing and Updating Sources

The `repo sync` command re-imports resources from all configured sources.

### Basic Usage

```bash
# Sync all sources
aimgr repo sync

# Preview without changes
aimgr repo sync --dry-run

# Skip existing resources (don't overwrite)
aimgr repo sync --skip-existing
```

### What Sync Does

For each source in `ai.repo.yaml`:
- **Path sources**: Re-create symlinks to source files
- **URL sources**: Download latest version, copy to repository

`repo sync` also reuses each source's persisted `sources[].discovery` mode from
`ai.repo.yaml` (set by `repo add --discovery`). This preserves marketplace-first
(`auto`) vs `marketplace` vs `generic` behavior on every sync.

### When to Sync

- After upstream changes to remote repositories
- To refresh all sources at once
- After manually editing `ai.repo.yaml`
- To verify source availability

### Options

| Flag | Description |
|------|-------------|
| `--skip-existing` | Don't overwrite existing resources |
| `--dry-run` | Preview without importing |
| `--format=<format>` | Output format: table, json, yaml |

### Handling Failures

If a source becomes unavailable:
1. Error is reported
2. Remaining sources continue syncing
3. Command exits with error status

The failed source remains in `ai.repo.yaml` for future retries. Remove it with `repo remove` if no longer needed.

---

## Removing Sources

Use `repo remove` to remove a source and optionally clean up its resources.

### Basic Usage

```bash
# Remove by name
aimgr repo remove my-source

# Remove by path (local sources)
aimgr repo remove ~/my-resources/

# Remove by URL (remote sources)
aimgr repo remove https://github.com/owner/repo
```

### Options

| Flag | Description |
|------|-------------|
| `--keep-resources` | Keep resources, only remove source entry |
| `--dry-run` | Preview what would be removed |

### Behavior

By default, `remove`:
1. Removes the source entry from `ai.repo.yaml`
2. Deletes resources that came from that source (orphan cleanup)

Use `--keep-resources` to preserve resources (they become "untracked").

### Examples

```bash
# Preview removal
aimgr repo remove my-source --dry-run

# Remove source and its resources
aimgr repo remove my-source

# Remove source but keep resources
aimgr repo remove my-source --keep-resources
```

---

## Commands Reference

### repo add

Import resources from a source and track it in `ai.repo.yaml`.

```bash
aimgr repo add <source> [flags]
```

**Options:**

| Flag | Description |
|------|-------------|
| `--name=<name>` | Custom name for source |
| `--filter=<pattern>` | Only import matching resources |
| `--force` | Overwrite existing resources |
| `--skip-existing` | Skip existing resources |
| `--dry-run` | Preview without importing |
| `--discovery=<mode>` | Discovery mode: `auto`, `marketplace`, `generic` |
| `--format=<format>` | Output format |

### Discovery modes (`repo add`)

`repo add` supports three discovery modes:

- `auto` (default): marketplace-first. If `marketplace.json` exists, import marketplace-defined plugin resources and generated packages. If not, fall back to generic discovery.
- `marketplace`: require `marketplace.json`; fail if not found.
- `generic`: ignore `marketplace.json`; only import generic commands/skills/agents/packages.

```bash
# Default marketplace-first behavior
aimgr repo add local:~/dev/team-resources

# Force generic discovery
aimgr repo add local:~/dev/team-resources --discovery generic

# Require marketplace discovery
aimgr repo add local:~/dev/team-resources --discovery marketplace
```

You can also point directly at marketplace manifests:

```bash
# Direct local marketplace file
aimgr repo add local:/home/user/repo/.claude-plugin/marketplace.json

# Repo-backed remote marketplace file URL
aimgr repo add gh:owner/repo/.claude-plugin/marketplace.json
aimgr repo add https://github.com/owner/repo.git/.claude-plugin/marketplace.json
```

### repo sync

Re-import resources from all configured sources.

```bash
aimgr repo sync [flags]
```

### repo remove

Remove a source and optionally clean up orphaned resources.

```bash
aimgr repo remove <name|path|url> [flags]
```

### repo info

Display repository information including configured sources.

```bash
aimgr repo info [flags]
```

**Example output:**
```
Repository: /home/user/.local/share/ai-config/repo

Resources:
  Commands: 12
  Skills:   8
  Agents:   3
  Packages: 2
  Total:    25

Configured Sources (2):
  my-local-skills (symlink)
    Path: /home/user/dev/my-skills
    Last synced: 2026-02-14 15:45:00
    
  community-catalog (copy)
    URL: https://github.com/owner/repo@v1.2.0
    Last synced: 2026-02-14 15:45:00
```

### repo drop

Delete the entire repository.

```bash
aimgr repo drop [flags]
```

| Flag | Description |
|------|-------------|
| `--force` | Required confirmation |
| `--full-delete` | Delete everything including `ai.repo.yaml` |

**Recovery after soft drop:**
```bash
# ai.repo.yaml is preserved, rebuild from sources
aimgr repo sync
```

---

## Workflows

### Initial Setup

```bash
# Add your first source
aimgr repo add gh:agentskills/catalog

# Check what was imported
aimgr repo list

# View source configuration
aimgr repo info

# Install resources in your project
cd ~/my-project
aimgr install skill/pdf-processing
```

### Multi-Source Setup

```bash
# Add community resources
aimgr repo add gh:anthropics/skills --name=anthropic-skills

# Add company resources
aimgr repo add gh:mycompany/ai-resources --name=company-resources

# Add local development resources
aimgr repo add local:~/dev/my-skills --name=dev-skills

# Verify all sources
aimgr repo info
```

### Promoting Local to Remote

When ready to share your local resources:

```bash
# Currently using local source
cd ~/dev/my-skills

# Push to GitHub
git init && git add . && git commit -m "Initial commit"
git remote add origin https://github.com/myuser/my-skills.git
git push -u origin main
git tag v1.0.0 && git push origin v1.0.0

# Remove local source, add remote
aimgr repo remove my-skills --keep-resources
aimgr repo add gh:myuser/my-skills@v1.0.0 --name=my-skills --force
```

### Converting Remote to Local

For active development on an upstream repository:

```bash
# Clone locally
git clone https://github.com/owner/repo ~/dev/upstream-repo

# Remove remote source
aimgr repo remove upstream-repo

# Add as local source (symlink mode)
aimgr repo add local:~/dev/upstream-repo --name=upstream-repo

# Now changes reflect immediately via symlinks
```

### Temporary local override (supported dev-testing workflow)

When you want to test local changes for a source that is normally remote, use `repo override-source` instead of editing manifests or swapping source entries manually.

```bash
# Baseline source points to a remote
aimgr repo info

# Temporarily switch that source to a local checkout and auto-sync
aimgr repo override-source team-tools local:~/dev/team-tools

# See active override and restore target
aimgr repo info

# Restore the original remote definition and auto-sync
aimgr repo override-source team-tools --clear
```

Behavior and constraints:

- `repo info` is the user-facing read path for active override state.
- `repo show-manifest` intentionally prints a shareable baseline view and does **not** leak local override paths or override breadcrumb fields.
- Local override breadcrumbs are stored in `.metadata/sources.json` (local runtime state), not in shareable `ai.repo.yaml` output.
- `repo apply-manifest` is for baseline sharing/merge, not overlay transport switching.
- A second override attempt on an already-overridden source is rejected. Clear the active override first: `aimgr repo override-source <name> --clear`.
- Overriding a source that is already a local `path` source is not supported; `repo override-source` is for temporarily switching remote-backed sources to a local checkout.

Why overlay manifests are not supported for this:

- overlaying local `path:` entries into shared manifests is easy to leak or commit accidentally
- override state is intentionally local-only so baseline manifests stay reproducible for teams and CI

Removing an overridden source:

- `aimgr repo remove <source>` removes the source entry and cleanup metadata
- for overridden sources, this permanently removes both active override and stored restore target
- if you only want to get back to the remote, use `aimgr repo override-source <name> --clear` first

Sharing/committing while override is active:

- safe sharing path: `aimgr repo show-manifest` (shareable baseline output)
- local active state visibility: `aimgr repo info`

Recovery steps if state looks unexpected:

```bash
aimgr repo info
aimgr repo override-source <name> --clear
aimgr repo sync
```

---

## Troubleshooting

### Source Not Found After Moving Directory

**Problem:** Local source was moved to a new location.

```
Error syncing source 'my-skills': path does not exist: /old/path/my-skills
```

**Solutions:**

```bash
# Option 1: Edit ai.repo.yaml
vim ~/.local/share/ai-config/repo/ai.repo.yaml
# Change path: /old/path/my-skills to /new/path/my-skills

# Option 2: Remove and re-add
aimgr repo remove my-skills --keep-resources
aimgr repo add local:/new/path/my-skills --name=my-skills --force
```

### Remote Repository Unavailable

**Problem:** Remote repository was deleted, made private, or URL changed.

```
Error syncing source 'company-resources': failed to clone: repository not found
```

**Solutions:**

```bash
# If temporarily unavailable - other sources still sync
# Sync will continue and report the error

# If permanently unavailable
aimgr repo remove company-resources --keep-resources

# If URL changed - edit ai.repo.yaml
vim ~/.local/share/ai-config/repo/ai.repo.yaml
```

### Git Not Installed

**Problem:** Git executable not found.

```
git clone failed: exec: "git": executable file not found
```

**Solution:** Install Git:

```bash
# Ubuntu/Debian
sudo apt-get install git

# macOS
brew install git
```

### Authentication Failed

**Problem:** Cannot access private repository.

```
git clone failed: authentication required
```

**Solutions:**

```bash
# For HTTPS - configure credential helper
git config --global credential.helper store

# For SSH - ensure keys are configured
ssh -T git@github.com
```

### No Resources Found

**Problem:** Repository exists but no resources discovered.

```
Error: no resources found in repository
```

**Solutions:**

- Check repository structure matches expected locations
- Use a subpath: `aimgr repo add https://github.com/owner/repo/path/to/resources`
- Verify resources have valid frontmatter (SKILL.md with name and description)

### Duplicate Source Names

**Problem:** Source name collision.

```
Error: source with name 'my-source' already exists
```

**Solutions:**

```bash
# Use custom name
aimgr repo add local:~/new-resources --name=my-source-v2

# Or remove old source first
aimgr repo remove my-source
aimgr repo add local:~/new-resources
```

### Corrupted ai.repo.yaml

**Problem:** Invalid YAML syntax.

```
Error: failed to load manifest: invalid YAML
```

**Solutions:**

```bash
# Validate syntax
yamllint ~/.local/share/ai-config/repo/ai.repo.yaml

# Or restore from backup
cp ~/.local/share/ai-config/repo/ai.repo.yaml.backup \
   ~/.local/share/ai-config/repo/ai.repo.yaml

# Or recreate from scratch
aimgr repo drop --force
aimgr repo add gh:owner/repo --name=source1
```

### Symlinks Not Working on Windows

**Problem:** Windows requires special permissions for symlinks.

**Solutions:**

```bash
# Option 1: Enable Developer Mode
# Settings > Update & Security > For Developers > Developer Mode

# Option 2: Use remote source instead (copy mode)
# Push local source to Git, then add as URL source
aimgr repo remove my-source --keep-resources
aimgr repo add https://github.com/myuser/my-resources --name=my-source --force
```

---

## Best Practices

### 1. Use Descriptive Source Names

```bash
# Good - clear and descriptive
aimgr repo add gh:owner/repo --name=community-pdf-tools

# Less clear - auto-generated name
aimgr repo add gh:owner/repo  # → "repo"
```

### 2. Pin Remote Sources to Versions

```bash
# Stable - pinned to version
aimgr repo add gh:owner/repo@v1.2.0

# Risky - tracks latest (may break)
aimgr repo add gh:owner/repo
```

### 3. Use Filters for Selective Importing

```bash
# Only import skills
aimgr repo add gh:owner/all-resources --filter "skill/*"
```

### 4. Separate Development and Production

```yaml
# ai.repo.yaml
sources:
  # Production (versioned, copy mode)
  - name: prod-resources
    url: https://github.com/company/resources
    ref: v2.1.0
    
  # Development (local, symlink mode)
  - name: dev-resources
    path: /home/user/dev/resources
```

### 5. Preview Before Destructive Operations

```bash
aimgr repo remove old-source --dry-run
aimgr repo sync --dry-run
```

### 6. Back Up ai.repo.yaml

```bash
# Manual backup
cp ~/.local/share/ai-config/repo/ai.repo.yaml \
   ~/.local/share/ai-config/repo/ai.repo.yaml.backup

# Or version control it
cd ~/.local/share/ai-config/repo
git init && git add ai.repo.yaml && git commit -m "Backup sources"
```

### 7. Sync Regularly

```bash
# Keep resources up-to-date
aimgr repo sync

# Or automate with cron
0 9 * * * /usr/local/bin/aimgr repo sync
```

---

## Related Documentation

- [Getting Started](./getting-started.md) - First steps with aimgr
- [Configuration Guide](./configuration.md) - Global and project configuration
- [Pattern Matching](../reference/pattern-matching.md) - Filter pattern syntax
- [Supported Tools](../reference/supported-tools.md) - Tool support and resource format documentation
