# aimgr Concepts

This page explains the main concepts of **aimgr** from a user perspective.

## What aimgr does

`aimgr` helps you manage AI resources such as **skills**, **agents**, and **commands** across projects and AI tools.

It separates two concerns:

1. **Where resources come from**
2. **Which resources a project wants**

Those are represented by two different manifest files.

---

## 1. The repository: `ai.repo.yaml`

The aimgr repository is your local catalog of available resources.

- You add **sources** to it
- You **sync** those sources to refresh available resources
- You can **remove** sources you no longer want

Typical source commands:

```bash
aimgr repo add gh:your-org/ai-tools
aimgr repo add local:~/my-resources
aimgr repo sync
aimgr repo remove my-source
```

### What is a source?

A source is a place where resources live:

- a GitHub repository
- another Git repository URL
- a local folder on your machine

### What `repo sync` does

`aimgr repo sync` refreshes your repository from all configured sources.

- For **remote sources**, it fetches the latest content
- For **local sources**, aimgr keeps them connected for live editing workflows

Think of `ai.repo.yaml` as:

> “Where can aimgr find resources?”

---

## 2. The project manifest: `ai.package.yaml`

`ai.package.yaml` belongs to a specific project.

It defines which resources that project wants installed.

Typical project commands:

```bash
aimgr install
aimgr install skill/code-review
aimgr uninstall skill/code-review
aimgr list
aimgr verify
aimgr repair
```

If an `ai.package.yaml` exists in the current project, running:

```bash
aimgr install
```

installs everything declared in that manifest.

Think of `ai.package.yaml` as:

> “What does this project want to use?”

---

## 3. Optional local overlay: `ai.package.local.yaml`

Projects can also use an **optional** local overlay file: `ai.package.local.yaml`.

Use it when you need durable, private project-local additions without editing the committed
`ai.package.yaml`.

- Good fit: personal helper skills/commands you want to keep across `verify`/`repair`
- Not ideal: one-off experiments (use `aimgr install ... --no-save` for temporary installs)

Important behavior:

- Overlay is **opt-in only**.
- aimgr reads `ai.package.local.yaml` **when present**.
- aimgr does **not** auto-create `ai.package.local.yaml`.
- aimgr does **not** auto-edit `.gitignore` for you.

### Merge semantics (base + local overlay)

When both files exist, aimgr computes an effective manifest view:

- `resources` = union of base + local
  - preserve base order
  - append local-only additions
  - de-duplicate exact duplicates
- `install.targets` = union of base targets + local targets
- Explicit CLI `--target` always overrides manifest targets

This merged view is used by project reconciliation commands such as:

- `aimgr install`
- `aimgr verify`
- `aimgr repair`
- `aimgr list`

Uninstall persistence also follows merged-manifest behavior: when persisting removals,
`aimgr uninstall` removes the resource from **every** manifest file that declares it.

### Expected failure mode for missing overlay resources

If `ai.package.local.yaml` references resources that cannot be resolved from your local
repository sources, those failures are surfaced explicitly by install/verify/repair flows.
They are not silently ignored.

---

## How they work together

The two manifests have different jobs:

| File | Purpose |
|------|---------|
| `ai.repo.yaml` | Tracks resource sources for your local aimgr repository |
| `ai.package.yaml` | Tracks committed project dependencies |
| `ai.package.local.yaml` | Optional local-only overlay for private project additions |

In short:

1. Add and sync sources into your repository
2. Install selected resources into a project
3. Track project dependencies in `ai.package.yaml`

---

## Mental model

You can think about aimgr like this:

- **`ai.repo.yaml`** = your supply side
- **`ai.package.yaml`** = your project requirements
- **`aimgr install`** = bring required resources into the project
- **`aimgr repo sync`** = refresh the supply side

---

## Common workflow

```bash
# 1. Configure where resources come from
aimgr repo add gh:your-org/ai-tools
aimgr repo sync

# 2. Move into a project
cd ~/my-project

# 3. Install what the project needs
aimgr install
```

If the project has no `ai.package.yaml` yet, you can still install resources directly,
and aimgr can track them in the project manifest.

---

## When to use which file

Use **`ai.repo.yaml`** when you want to:

- add a new source
- sync updates from upstream
- remove a source
- manage your available resource catalog

Use **`ai.package.yaml`** when you want to:

- define project dependencies
- reproduce the same setup in another clone
- install all project resources with `aimgr install`
- verify whether a project is in sync

Use **`ai.package.local.yaml`** when you want to:

- keep private project-local additions out of git
- make local additions survive install/verify/repair/list reconciliation
- layer personal resources on top of the team baseline declared in `ai.package.yaml`

---

## Owned folders and reconciliation

For detected tools, aimgr treats project resource directories as **owned state**:

- commands directories
- skills directories
- agents directories

Examples include `.claude/commands`, `.opencode/skills`, or `.github/skills`.

Two commands define cleanup and recovery behavior:

- `aimgr clean` empties owned resource directories
- `aimgr repair` reconciles owned directories to the effective manifest view
  (`ai.package.yaml` + optional `ai.package.local.yaml`)

In practice:

- If a resource is declared in the effective manifest view, `repair` ensures it is installed
- If undeclared content exists in owned directories, `repair` removes it
- If you manually delete a declared resource, `repair` reinstalls it

If you want to remove a resource permanently, update the manifest that declares it
(committed and/or local overlay)
(for example with `aimgr uninstall <resource>` without `--no-save`).

For a full reset-and-restore flow:

```bash
aimgr clean && aimgr repair
```

---

## See also

- [Getting Started](getting-started.md)
- [Sources](sources.md)
- [Configuration](configuration.md)
- [Repairing Resources](repair.md)
