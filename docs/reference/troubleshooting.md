# Troubleshooting

Common issues and solutions when using aimgr.

---

## Installation Issues

### Command not found after installation

**Problem:** Running `aimgr` returns "command not found".

**Solution:** Ensure aimgr is in your PATH:
```bash
# Check if /usr/local/bin is in PATH
echo $PATH

# Add to PATH if needed (add to ~/.bashrc or ~/.zshrc)
export PATH="/usr/local/bin:$PATH"
```

---

## Resource Visibility Issues

### Resources not showing in my AI tool

**Problem:** Installed resources don't appear in Claude Code, OpenCode, etc.

**Solutions:**
1. Check that resources are installed: `aimgr list`
   - Look at the **TARGETS** column to see which tools have the resource
   - If your tool isn't listed, the resource isn't installed for that tool
2. Verify your tool is using the right directory (`.claude/`, `.opencode/`, etc.)
3. Check installation targets: `aimgr config get install.targets`
4. Restart your AI tool to pick up new resources

---

## Sync Status Issues

### Resource marked with * (not in manifest)

**What it means:** The resource is installed but not declared in your `ai.package.yaml` file.

**Solutions:**
- To track as a project dependency, add it to `ai.package.yaml`:
  ```yaml
  resources:
    - skill/my-skill
    - command/my-command
  ```
- If it's a temporary or local-only resource, you can ignore the `*` symbol

### Resource marked with warning (not installed)

**What it means:** The resource is declared in `ai.package.yaml` but not installed.

**Solution:**
```bash
# Install the specific resource
aimgr install skill/my-skill

# Or install all missing resources from manifest
aimgr install $(aimgr repo list --format=json | jq -r '.resources[] | select(.sync_status == "not-installed") | .name')
```

---

## Source Management Issues

### Source not showing in repo info

**What it means:** The source was not successfully added to `ai.repo.yaml`.

**Solutions:**
1. Check if `ai.repo.yaml` exists:
   ```bash
   aimgr repo info
   ```
2. If missing, initialize the repository:
   ```bash
   aimgr repo init
   ```
3. Re-add your sources:
   ```bash
   aimgr repo add ~/my-resources/
   ```

---

## Common Errors

### "Resource already exists" error

**Problem:** Adding or installing a resource fails because it already exists.

**Solution:** Use `--force` to overwrite:
```bash
aimgr repo add ~/resource/ --force
aimgr install skill/foo --force
```

### Broken symlinks after removing resource

**Problem:** After removing a resource from the repository, projects have broken symlinks.

**Solution:** Use `aimgr repair` to fix broken symlinks automatically:
```bash
cd ~/project1
aimgr repair    # Fixes broken symlinks, reinstalls from repo if possible
```

Or uninstall cleanly before removing from repository:
```bash
# 1. First uninstall from all projects
cd ~/project1 && aimgr uninstall skill/foo
cd ~/project2 && aimgr uninstall skill/foo

# 2. Then remove from repository
aimgr repo remove skill foo
```

### Stale entries in ai.package.yaml

**Problem:** `ai.package.yaml` references resources that no longer exist in the repository.

**Solution:** Use `aimgr repair --prune-package` to clean up:
```bash
# Preview what would be removed
aimgr repair --prune-package --dry-run

# Remove invalid references
aimgr repair --prune-package --force
```

### Unmanaged files in resource directories

**Problem:** Resource directories contain files that weren't installed by aimgr.

**Solution:** Use `aimgr repair --reset` to find and remove unmanaged files:
```bash
# Preview what would be removed
aimgr repair --reset --dry-run

# Remove unmanaged files
aimgr repair --reset --force
```

### Repository metadata issues

**Problem:** `aimgr repo verify` reports missing metadata or orphaned metadata files.

**Solution:** Use `aimgr repo repair` to fix repository-level metadata:
```bash
aimgr repo repair              # Fix auto-fixable issues
aimgr repo repair --dry-run    # Preview what would be fixed
```

---

## Shell Completion

Enable tab completion for faster workflow:

```bash
# Bash
aimgr completion bash > /etc/bash_completion.d/aimgr

# Zsh
aimgr completion zsh > "${fpath[1]}/_aimgr"

# Fish
aimgr completion fish > ~/.config/fish/completions/aimgr.fish
```

Now you can tab-complete resource names:
```bash
aimgr install skill/<TAB>
# Shows: atlassian-cli  dynatrace-api  github-docs  pdf-processing
```

---

## See Also

- **[Repairing Resources](../user-guide/repair.md)** - Complete guide to `aimgr repair` and `aimgr repo repair`

## Getting More Help

- **Command help:** Run `aimgr --help` or `aimgr <command> --help`
- **Documentation:** [GitHub Repository](https://github.com/hk9890/ai-config-manager)
- **Issues:** [Report bugs or request features](https://github.com/hk9890/ai-config-manager/issues)
