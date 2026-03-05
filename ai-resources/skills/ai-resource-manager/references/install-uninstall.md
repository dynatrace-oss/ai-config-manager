# Install & Uninstall Resources

Install, manage, and troubleshoot AI resources in your project.

Resources live in a centralized repository (`~/.local/share/ai-config/repo/`)
and are **symlinked** into projects. This doc covers project-level operations only.

For repository management (adding sources, syncing), see [manage-repository.md](manage-repository.md).

**Sections:** [Initialize Manifest](#initialize-a-project-manifest) · [Browse](#browse-available-resources) · [Install](#install-resources) · [Restart](#️-restart-required) · [Verify](#verify-installations) · [Uninstall](#uninstall-resources) · [Repair](#repair-installations) · [Troubleshooting](#troubleshooting)

---

## Initialize a Project Manifest

Create an `ai.package.yaml` to track resource dependencies (like `package.json`):

```bash
aimgr init            # Creates empty ai.package.yaml
aimgr init --yes      # Non-interactive
```

This is optional — you can install resources without a manifest, but installs
won't be tracked or reproducible.

---

## Browse Available Resources

```bash
aimgr repo list                       # All available resources
aimgr repo list skill/*               # Only skills
aimgr repo list --format=json         # JSON for parsing
aimgr repo list --source my-source    # Filter by source
aimgr repo describe skill/name        # Detailed info on a resource
```

Present results in a friendly format — don't dump raw JSON to the user.

---

## Install Resources

```bash
# From ai.package.yaml (install all declared dependencies)
aimgr install

# Single resource
aimgr install skill/pdf-processing

# Multiple resources
aimgr install skill/pdf-processing command/test agent/reviewer

# Packages (installs all bundled resources)
aimgr install package/web-tools

# Pattern matching
aimgr install "skill/*"           # All skills
aimgr install "*test*"            # Anything with "test"
aimgr install "skill/pdf*"        # Skills starting with "pdf"
```

### Flags

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing installations |
| `--save` | Save to ai.package.yaml (default: true) |
| `--no-save` | Install without updating manifest |
| `--target` | Target specific tools: `--target claude,opencode` |
| `--project-path` | Install to a different directory |

### Multi-Tool Behavior

- If tool directories exist (`.claude/`, `.opencode/`, `.github/skills/`), installs to **all** of them
- If none exist, creates directory for your configured default tool
- Override with `--target`: `aimgr install skill/foo --target claude`
- Set default: `aimgr config set install.targets claude`

### Supported Tools

| Tool | Skills | Commands | Agents |
|------|--------|----------|--------|
| Claude Code | ✅ | ✅ | ✅ |
| OpenCode | ✅ | ✅ | ✅ |
| GitHub Copilot | ✅ | ❌ | ❌ |

---

## ⚠️ Restart Required

**Skills load at startup.** After installing or uninstalling resources,
users **must** restart their AI tool (close and reopen Claude Code, OpenCode, VS Code, etc.).

Always remind the user:

```text
⚠️ Restart Required: Close and reopen [Tool Name] to load the new resources.
```

---

## Verify Installations

Check for broken symlinks, missing resources, and sync status:

```bash
aimgr list                    # Show installed resources with sync status
aimgr verify                  # Diagnose issues (read-only)
```

### Sync Status Symbols (`aimgr list`)

| Symbol | Meaning |
|--------|---------|
| ✓ | In sync — installed and in manifest |
| * | Not in ai.package.yaml — installed but not declared |
| ⚠ | Not installed — declared but missing |
| - | No ai.package.yaml exists |

---

## Uninstall Resources

```bash
aimgr uninstall skill/pdf-processing       # Remove and update manifest
aimgr uninstall command/test --no-save     # Remove but keep in manifest
aimgr uninstall "skill/*"                  # Pattern matching
```

To remove **all** installed resources:

```bash
aimgr clean                   # Interactive confirmation
aimgr clean --yes             # Skip confirmation
```

`clean` removes all symlinks but preserves `ai.package.yaml`, so you can
reinstall with `aimgr install`.

---

## Repair Installations

Fix broken symlinks, missing resources, and stale files:

```bash
# Diagnose (read-only)
aimgr verify

# Fix automatically
aimgr repair

# Full cleanup
aimgr repair --reset --prune-package --force
```

### What `repair` Fixes

| Issue | Action |
|-------|--------|
| Broken symlinks | Reinstalls from repository |
| Wrong-repo symlinks | Reinstalls from correct repository |
| Missing resources (in manifest) | Installs from repository |
| Orphaned resources (not in manifest) | Prints hint to uninstall |

### Additional Flags

| Flag | Description |
|------|-------------|
| `--reset` | Remove unmanaged files (not installed by aimgr) |
| `--prune-package` | Remove invalid references from ai.package.yaml |
| `--dry-run` | Preview without changes |
| `--force` | Skip confirmation prompts |

### Common Workflow

```bash
# After cloning a project with ai.package.yaml
aimgr verify && aimgr repair

# After updating repository sources
aimgr repo sync && aimgr repair

# Full reset
aimgr clean --yes && aimgr install
```

---

## Troubleshooting

### Skills Not Loading After Install

**Most common cause:** AI tool not restarted. Close and reopen the tool.

If still not loading:

```bash
aimgr list                              # Verify installation
aimgr verify                            # Check for issues
ls -la .claude/skills/some-skill/SKILL.md  # Verify symlink target exists
```

### Broken Symlinks

```bash
aimgr verify                            # Identify broken links
aimgr repair                            # Auto-fix
```

If repair fails, reinstall manually:

```bash
aimgr uninstall skill/broken-skill
aimgr install skill/broken-skill
```

### Wrong Tool Directory

Resources installing to unexpected directory:

```bash
# Check current default
aimgr config get install.targets

# Set preferred tool
aimgr config set install.targets claude

# Or use --target per-install
aimgr install skill/foo --target claude
```

### Resource Not Found

```bash
# Check repository has the resource
aimgr repo list | grep resource-name

# If missing, sync repository
aimgr repo sync

# Names are case-sensitive and use hyphens
# ✅ skill/my-skill  ❌ skill/My_Skill
```

### Permission Denied

```bash
chmod +x $(which aimgr)
```

### aimgr Not Found

Install from [GitHub releases](https://github.com/dynatrace-oss/ai-config-manager/releases)
or via mise / go install:

```bash
mise use -g github:dynatrace-oss/ai-config-manager
# or
go install github.com/dynatrace-oss/ai-config-manager/v3/cmd/aimgr@latest
```

Ensure `~/go/bin` or `~/.local/bin` is in your `PATH`.

📚 Run `aimgr install --help`, `aimgr repair --help` for full flag reference.
