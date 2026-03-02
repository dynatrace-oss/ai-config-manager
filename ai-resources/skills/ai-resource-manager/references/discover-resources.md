# Discover Useful Resources

Scan a project and recommend AI resources that would be useful based on
the project's tech stack, tooling, and configuration.

**Sections:** [Workflow](#workflow) · [What to Check](#what-to-check) · [Notes for Agents](#notes-for-agents)

---

## Workflow

### 1. Scan the Project

Look for files and directories that indicate technologies and frameworks:

| Signal | What It Indicates |
|--------|-------------------|
| `package.json` | Node.js/JavaScript/TypeScript project |
| `tsconfig.json` | TypeScript |
| `go.mod` | Go project |
| `pyproject.toml`, `requirements.txt`, `setup.py` | Python project |
| `Cargo.toml` | Rust project |
| `pom.xml`, `build.gradle` | Java/Kotlin project |
| `Gemfile` | Ruby project |
| `Dockerfile`, `docker-compose.yml` | Container workflows |
| `.github/workflows/` | GitHub Actions CI/CD |
| `.gitlab-ci.yml` | GitLab CI/CD |
| `Makefile` | Build automation |
| `.env`, `.env.example` | Environment configuration |
| `*.pptx` files | PowerPoint/presentation needs |
| `*.pdf` files | PDF processing needs |
| `.beads/` | Beads task tracking |
| `CONTRIBUTING.md` | Open-source contribution workflow |
| `docs/` | Documentation project |
| `terraform/`, `*.tf` | Infrastructure as code |
| `k8s/`, `*.yaml` (with apiVersion) | Kubernetes |

```bash
# Quick scan for project signals
ls -la
ls package.json go.mod pyproject.toml Cargo.toml Dockerfile 2>/dev/null
ls -d .github/workflows .beads docs 2>/dev/null
```

### 2. Query Available Resources

```bash
aimgr repo list --format=json
```

Parse the output to get resource names and descriptions. Match against
discovered project signals.

### 3. Match and Recommend

Compare project signals against resource descriptions. Matching should be
**dynamic** — based on what's actually in the repository, not a hardcoded list.

**Example reasoning:**

> This project has `go.mod` → Go project. Available skills include
> `skill/go-testing` ("Test Go projects") — this is relevant.
> Project also has `.github/workflows/` → CI/CD present. `skill/ci-cd`
> would complement existing workflows.

### 4. Present Recommendations

Present a concise table with rationale:

```text
Based on your project, these resources look useful:

| Resource | Why |
|----------|-----|
| skill/go-testing | Go project detected (go.mod) |
| skill/pptx | PowerPoint files found in docs/ |
| skill/observability-triage | Monitoring config detected |

Want me to install any of these?
```

**Rules:**
- Only recommend resources not already installed (`aimgr list` to check)
- Explain *why* each recommendation is relevant
- Let the user choose — don't auto-install
- Group by relevance (most useful first)

### 5. Install Chosen Resources

After user selects:

```bash
aimgr install skill/go-testing skill/pptx
```

⚠️ **Restart Required:** Remind the user to restart their AI tool after installation.

---

## What to Check

### Already Installed

Before recommending, check what's already installed:

```bash
aimgr list
```

Don't recommend resources that are already present.

### Packages

Check for packages that bundle multiple related resources:

```bash
aimgr repo list package/*
aimgr repo describe package/some-package
```

A single package may cover multiple needs more cleanly than individual skills.

---

## Notes for Agents

- **Don't hardcode mappings.** The repository contents change over time.
  Always query `aimgr repo list` for current availability.
- **Be conservative.** Only recommend resources with a clear signal match.
  Don't recommend everything in the repo.
- **Respect user choice.** Present options, don't install without asking.
- **Check descriptions.** Resource descriptions (from `repo list --format=json`)
  are the primary matching signal.

📚 Run `aimgr repo list --help` and `aimgr repo describe --help` for full options.
