# Discover & Recommend Resources

Scan project context, match against the aimgr repository, and recommend relevant resources.

---

## Workflow

### 1. Read Project Context

Check for `.coder/project.yaml` first (written by opencode-coder plugin at startup):

```bash
cat .coder/project.yaml 2>/dev/null
```

If present, parse:
- `git.platform` — github, gitlab, bitbucket, or null
- `beads.initialized` — whether beads is set up
- `aimgr.installed` — whether aimgr is available

**If absent:** Fall back to file-based scanning (step 2). Do NOT run inline detection commands.

### 2. Scan for Tech Signals

Look for files that indicate the project's tech stack (e.g. `go.mod` → Go, `.github/workflows/` → GitHub CI, `Dockerfile` → containers):

```bash
ls package.json go.mod pyproject.toml Cargo.toml Dockerfile 2>/dev/null
ls -d .github/workflows .beads docs 2>/dev/null
```

### 3. Query Available Resources

```bash
aimgr repo list --format=json
aimgr list                      # Already installed — don't re-recommend
```

Match resource descriptions against discovered signals. Only recommend what's relevant — e.g. don't recommend a Bitbucket skill for a GitHub repo.

### 4. Present Recommendations

Use the `question()` tool — **mandatory user interaction, do NOT auto-install**:

```text
Based on your project, these resources look useful:

| Resource | Why |
|----------|-----|
| skill/go-testing | Go project (go.mod) |
| skill/github-releases | GitHub repo detected |
| skill/pptx | .pptx files found |

Want me to install any of these?
```

After selection: `aimgr install <chosen-resources>`

⚠️ Remind user to restart their AI tool after install.

---

## Notes

- Always query `aimgr repo list` — repository contents change, don't hardcode mappings.
- Be conservative — only recommend with a clear signal match.
- Present options, never auto-install.
- Check for packages that bundle related resources: `aimgr repo describe package/*`
