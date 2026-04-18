---
name: ai-resource-manager
description: "Manage AI resources (skills, commands, agents) using aimgr CLI. Use when user asks to: (1) Install/uninstall resources, (2) Manage repository, (3) Discover/recommend resources for a project, (4) Troubleshoot aimgr issues."
---

# AI Resource Manager

Manage AI resources via `aimgr`. Resources live in `~/.local/share/ai-config/repo/` and are symlinked to projects.

## ⚠️ Safety Rules

**Ask user approval before ANY mutating command.** Read-only commands are safe.

| Mutating (ask first) | Read-only (safe) |
|---|---|
| `install`, `uninstall`, `init`, `repair`, `clean` | `list`, `verify` |
| `repo add`, `repo sync`, `repo remove`, `repo apply-manifest` | `repo list`, `repo describe`, `repo info`, `repo show-manifest` |
| `repo repair`, `repo rebuild`, `repo drop`, `repo prune` | `repo verify`, `repo add --dry-run` |

## Use Cases

**UC1 — Install / Uninstall:** Install, verify, repair resources in a project. Covers `install`, `uninstall`, `list`, `verify`, `repair`, `clean`, `init`, `ai.package.yaml`.
→ [references/install-uninstall.md](references/install-uninstall.md)

Project repair/cleanup semantics to follow in guidance:

- `aimgr clean` empties owned resource directories (no confirmation flags)
- `aimgr repair` reconciles owned directories to `ai.package.yaml`
- `aimgr clean && aimgr repair` replaces old `repair --reset --force` workflows
- `repair --dry-run` previews actions; `--prune-package` is separate manifest cleanup
- Declared resources removed manually are reinstalled on `repair` until manifest is updated

**UC2 — Manage Repository:** Add sources, inspect or apply `ai.repo.yaml`, sync, remove, validate, maintain the global repo. Covers all `repo` subcommands.
→ [references/manage-repository.md](references/manage-repository.md)

**UC3 — Discover & Recommend:** Scan project context, match against available resources, recommend relevant ones.
→ [references/discover-resources.md](references/discover-resources.md)

## Troubleshooting

| Issue | Fix |
|-------|-----|
| Skills not loading | Restart AI tool |
| `aimgr` not found | `mise use -g github:dynatrace-oss/ai-config-manager` or `go install github.com/dynatrace-oss/ai-config-manager/v3/cmd/aimgr@latest` |
| Resource not found | `aimgr repo sync` |
| Broken symlinks | `aimgr repair` or `aimgr repo repair` |

Details in [install-uninstall.md](references/install-uninstall.md) and [manage-repository.md](references/manage-repository.md).

## Resources

📚 Run `aimgr [command] --help` for command syntax.

| Tool | Skills | Commands | Agents |
|------|--------|----------|--------|
| Claude Code | ✅ | ✅ | ✅ |
| OpenCode | ✅ | ✅ | ✅ |
| GitHub Copilot | ✅ | ❌ | ❌ |
| Windsurf | ✅ | ❌ | ❌ |

- Repo: <https://github.com/dynatrace-oss/ai-config-manager>
- Issues: <https://github.com/dynatrace-oss/ai-config-manager/issues>
