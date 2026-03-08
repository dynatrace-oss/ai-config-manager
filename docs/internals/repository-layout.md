# Repository Layout

This document describes the internal folder structure of an aimgr repository. Understanding this layout is helpful for debugging, advanced usage, or contributing to aimgr.

## Default Location

By default, the repository is located at:

```
~/.local/share/ai-config/repo/
```

This can be customized via the `AIMGR_REPO_PATH` environment variable or `repo.path` in `~/.config/aimgr/aimgr.yaml`. See the [Configuration Guide](../user-guide/configuration.md) for details.

## Directory Structure

```
~/.local/share/ai-config/repo/
├── ai.repo.yaml           # Source manifest
├── .gitignore             # Git ignore rules
├── .git/                  # Git tracking
├── skills/                # Skill resources
│   └── <skill-name>/
│       └── SKILL.md
├── commands/              # Command resources (can be nested)
│   ├── <command>.md
│   └── <namespace>/
│       └── <command>.md
├── agents/                # Agent resources
│   └── <agent>.md
├── packages/              # Package resources
│   └── <package-name>/
├── .metadata/             # Resource & source metadata
│   ├── sources.json       # Source tracking state
│   ├── skills/
│   │   └── <name>-metadata.json
│   ├── commands/
│   │   └── <name>-metadata.json
│   ├── agents/
│   │   └── <name>-metadata.json
│   └── packages/
│       └── <name>-metadata.json
├── .modifications/        # Tool-specific file variants
│   ├── opencode/
│   │   ├── skills/
│   │   ├── agents/
│   │   └── commands/
│   └── claude/
│       ├── skills/
│       ├── agents/
│       └── commands/
├── .workspace/            # Git clone cache (gitignored)
│   └── <hash>/            # Cached repository clones
└── logs/                  # Operation logs (gitignored)
    └── aimgr.log
```

## Folder Descriptions

### Resource Directories

| Directory | Purpose | Contents |
|-----------|---------|----------|
| `skills/` | Skill resources | Each skill is a directory containing `SKILL.md` and optional files |
| `commands/` | Command resources | Markdown files (`.md`), can be nested in subdirectories for namespacing |
| `agents/` | Agent resources | Markdown files (`.md`) defining agent behaviors |
| `packages/` | Package resources | Bundles of multiple resources |

### Configuration Files

| File | Purpose |
|------|---------|
| `ai.repo.yaml` | Source manifest - defines which remote sources to sync from |
| `.gitignore` | Excludes workspace cache and logs from Git tracking |

### .metadata/ - Resource and Source Tracking

The `.metadata/` directory contains two types of tracking data:

**Source Tracking (`sources.json`)**

Tracks state for sources defined in `ai.repo.yaml`:

```json
{
  "version": 1,
  "sources": {
    "my-skills": {
      "source_id": "abc123",
      "added": "2024-01-15T10:30:00Z",
      "last_synced": "2024-02-01T14:00:00Z"
    }
  }
}
```

**Resource Metadata (`<type>/<name>-metadata.json`)**

Each imported resource has a metadata file tracking its origin:

```json
{
  "name": "code-review",
  "type": "skill",
  "source_type": "github",
  "source_url": "https://github.com/example/skills",
  "source_name": "my-skills",
  "source_id": "abc123",
  "ref": "main",
  "first_installed": "2024-01-15T10:30:00Z",
  "last_updated": "2024-02-01T14:00:00Z"
}
```

This metadata enables:
- Tracking where each resource came from
- Knowing when resources were last updated
- Identifying orphaned resources after source removal

### .modifications/ - Tool-Specific Variants

The `.modifications/` directory contains transformed versions of resources with tool-specific field values. This is generated automatically when [field mappings](../user-guide/configuration.md#field-mappings) are configured.

**Structure:**

```
.modifications/
├── opencode/
│   ├── skills/
│   │   └── my-skill/
│   │       └── SKILL.md    # model: langdock/claude-sonnet-4-5
│   └── agents/
│       └── reviewer.md
└── claude/
    ├── skills/
    │   └── my-skill/
    │       └── SKILL.md    # model: claude-sonnet-4
    └── agents/
        └── reviewer.md
```

**Key Points:**
- Original files are never modified
- Variants are generated during `repo add` and `repo sync`
- Install symlinks to `.modifications/` when a variant exists, otherwise to the original
- No mappings configured = no `.modifications/` folder created

### .workspace/ - Git Clone Cache

The `.workspace/` directory caches Git repository clones for remote sources. This is automatically managed and gitignored.

See [Workspace Caching](workspace-caching.md) for details on how this cache works.

### logs/ - Operation Logs

Operation logs are written to `logs/aimgr.log` for debugging purposes. This directory is gitignored.

## Git Tracking

The repository is a Git repository, enabling:
- Version history of all resources
- Easy rollback if something goes wrong
- Syncing with remote backup

See [Git Tracking](git-tracking.md) for details on how aimgr uses Git.

## What Gets Committed

| Path | Committed | Notes |
|------|-----------|-------|
| `ai.repo.yaml` | Yes | Source definitions |
| `.gitignore` | Yes | Git configuration |
| `skills/`, `commands/`, `agents/`, `packages/` | Yes | All resources |
| `.metadata/` | Yes | Source and resource tracking |
| `.modifications/` | Yes | Tool-specific variants |
| `.workspace/` | No | Temporary cache |
| `logs/` | No | Debug logs |

## See Also

- [Configuration Guide](../user-guide/configuration.md) - Repository path configuration
- [Field Mappings](../user-guide/configuration.md#field-mappings) - How .modifications/ is generated
- [Workspace Caching](workspace-caching.md) - Details on .workspace/ cache
- [Git Tracking](git-tracking.md) - How aimgr uses Git internally

## `repo init` vs `repo show-manifest` / `repo apply-manifest` (v1 contract)

This section documents command boundaries for shareable manifests.

- `aimgr repo init`
  - Local bootstrap only
  - Creates repository directories, git repo, `.gitignore`, and initial `ai.repo.yaml`
  - Does **not** consume external manifests

- `aimgr repo show-manifest`
  - Reads and prints the current local `ai.repo.yaml`
  - Does not modify repository state

- `aimgr repo apply-manifest <path-or-url>`
  - Loads a manifest from `<path-or-url>` and merges sources into local `ai.repo.yaml`
  - v1 accepts only explicit `ai.repo.yaml` inputs:
    1. local filesystem path to `ai.repo.yaml`
    2. stdin via `-` or `/dev/stdin`
    3. HTTP(S) URL directly to `ai.repo.yaml`
  - Bare repository URLs are out of scope in v1

- Deferred (future work)
  - Manifest export command(s)
  - Lockfile/version pinning workflows beyond current `ref`

### Shareable manifest vs local state

`ai.repo.yaml` used for sharing should contain portable source definitions only.

- Internal/generated `sources[].id` values are local-only state
- `id` may exist in local persisted manifests, but `repo apply-manifest` must not require it in input
- Source state such as sync timestamps remains in `.metadata/sources.json`

### apply-manifest merge rules (v1)

When applying an incoming manifest to the local manifest:

1. Validate incoming manifest (`version`, source shape, duplicate names, include patterns)
2. For each incoming source:
   - **Name not present locally**: add source
   - **Name present + identical definition** (`path/url/ref/subpath/include`): no-op
   - **Name present + different definition**: report conflict; do not silently overwrite
3. Re-applying an unchanged manifest is idempotent

Incoming manifests with duplicate source names are invalid and rejected.

### Relative path resolution for apply-manifest

- **Local manifest input** (`./ai.repo.yaml`): resolve relative `path` entries against the manifest file directory
- **Stdin manifest input** (`-` or `/dev/stdin`): reject relative `path` entries in v1 (no manifest directory exists)
- **Remote HTTP(S) manifest input**: reject relative `path` entries in v1 (receiver-local filesystem target is ambiguous)
- Absolute `path` remains literal (usable for local-machine manifests)

Guidance: for remote/shared manifests, prefer `url` sources.

### Concrete v1 apply examples

```bash
aimgr repo show-manifest
aimgr repo apply-manifest ./ai.repo.yaml
aimgr repo show-manifest | aimgr repo apply-manifest -
aimgr repo apply-manifest /tmp/team/ai.repo.yaml
aimgr repo apply-manifest https://example.com/team/ai.repo.yaml
```
