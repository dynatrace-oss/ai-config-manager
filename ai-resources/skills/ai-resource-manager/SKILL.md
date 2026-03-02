---
name: ai-resource-manager
description: "Manage AI resources (skills, commands, agents) using aimgr CLI. Use when user asks to: (1) Install/uninstall resources, (2) Manage repository, (3) Validate resources for developers, (4) Discover useful resources for a project, (5) Troubleshoot aimgr issues."
---

# AI Resource Manager

Manage AI resources using `aimgr` CLI. Resources are stored once in
`~/.local/share/ai-config/repo/` and symlinked to projects.

---

## ⚠️ IMPORTANT: Agent Safety Rules

**Before running any mutating command, you MUST ask the user for explicit approval.**

**Mutating operations (require approval):**

- `aimgr install` / `aimgr uninstall` — Modifies project symlinks
- `aimgr init` — Creates `ai.package.yaml` manifest
- `aimgr repair` — Fixes broken installations
- `aimgr clean` — Removes orphaned symlinks
- `aimgr repo add` — Adds sources to repository
- `aimgr repo sync` — Updates repository from remote sources
- `aimgr repo remove` — Removes sources (and all their resources) from repository
- `aimgr repo repair` / `aimgr repo drop` / `aimgr repo prune` — Repository maintenance

**Read-only operations (safe to run):**

- `aimgr list` — Show installed resources
- `aimgr verify` — Check project installation health
- `aimgr repo list` — Show available resources
- `aimgr repo describe` — Show resource details
- `aimgr repo info` — Show repository metadata
- `aimgr repo verify` — Check repository health
- `aimgr repo add --dry-run` — Validate without changes

**Never assume permission. Always ask first.**

---

## Use Case 1: Install / Uninstall Resources

Install, uninstall, verify, and repair AI resources in the current project.
Covers `aimgr install`, `uninstall`, `list`, `verify`, `repair`, `clean`, `init`,
and the `ai.package.yaml` manifest.

📖 **Full guide:** [references/install-uninstall.md](references/install-uninstall.md)

---

## Use Case 2: Manage Repository

Add sources, sync resources, validate with dry-run, and maintain the global
repository. Covers `aimgr repo` subcommands: `add`, `sync`, `remove`, `list`,
`describe`, `info`, `init`, `verify`, `repair`, `drop`, `prune`.

📖 **Full guide:** [references/manage-repository.md](references/manage-repository.md)

---

## Use Case 3: Discover Useful Resources

Scan a project's tech stack and recommend resources from the repository.
Look for signal files (e.g., `go.mod`, `package.json`, `.github/`), match
against available resources, and offer to install relevant ones.

📖 **Full guide:** [references/discover-resources.md](references/discover-resources.md)

---

## Troubleshooting

| Issue | Fix |
|-------|-----|
| Skills not loading | Restart AI tool — skills load at startup |
| `aimgr` not found | `go install github.com/hk9890/ai-config-manager@latest` |
| Resource not found | `aimgr repo sync` to pull latest |
| Broken symlinks | `aimgr repair` (project) or `aimgr repo repair` (repo) |
| Permission denied | `chmod +x $(which aimgr)` |

For project-level troubleshooting, see [references/install-uninstall.md](references/install-uninstall.md).
For repository-level troubleshooting, see [references/manage-repository.md](references/manage-repository.md).

---

## Additional Resources

📚 **Command syntax:** Run `aimgr [command] --help` for detailed usage and examples.

**Supported Tools:**

| Tool | Skills | Commands | Agents |
|------|--------|----------|--------|
| Claude Code | ✅ | ✅ | ✅ |
| OpenCode | ✅ | ✅ | ✅ |
| GitHub Copilot | ✅ | ❌ | ❌ |
| Windsurf | ✅ | ❌ | ❌ |

**Links:**

- Repository: <https://github.com/hk9890/ai-config-manager>
- Issues: <https://github.com/hk9890/ai-config-manager/issues>
