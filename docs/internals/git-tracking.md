# Git-Backed Repository Tracking

**aimgr** supports git-based tracking of your AI resource repository. This provides:

- **Full audit trail** - Every import, sync, and remove operation is recorded
- **Change history** - View what changed, when, and why
- **Recoverability** - Easily revert unwanted changes
- **Collaboration** - Share repository state across teams

## Quick Start

### Initialize Git Tracking

```bash
# Initialize repository with git tracking
aimgr repo init

# Output:
# ✓ Repository structure initialized at: ~/.local/share/ai-config/repo
# ✓ Git repository initialized
# ✓ .gitignore created
# ✓ Initial commit created
# 
# ✨ Repository ready for git-tracked operations
```

This command:
1. Creates the repository directory structure
2. Initializes a git repository
3. Creates `.gitignore` to exclude `.workspace/` cache
4. Makes an initial commit

### Import Resources (Tracked)

```bash
# Import resources - automatically creates commit
aimgr repo add local:~/my-resources/

# Output:
# Importing from: ~/my-resources
# ...
# Summary: 3 added, 0 updated, 0 failed, 0 skipped (3 total)
# 
# Git commit created: "aimgr: import 3 resource(s) (2 skills, 1 command)"
```

Every successful import creates a descriptive commit.

### Remove Resources (Tracked)

```bash
# Remove resource - automatically creates commit
aimgr repo remove --force skill/old-skill

# Output:
# ✓ Removed skill/old-skill
# 
# Git commit created: "aimgr: remove skill: old-skill"
```

## Viewing History

### Show Recent Changes

```bash
cd ~/.local/share/ai-config/repo
git log --oneline

# Example output:
# a1b2c3d aimgr: remove skill: old-skill
# e4f5g6h aimgr: import 3 resource(s) (2 skills, 1 command)
# i7j8k9l aimgr: import 1 resource(s) (1 command)
# m0n1o2p aimgr: initialize repository
```

### Show Detailed Changes

```bash
# Show what changed in last commit
cd ~/.local/share/ai-config/repo
git show

# Show changes for specific commit
git show a1b2c3d

# Show diff between commits
git diff m0n1o2p..e4f5g6h
```

### View File History

```bash
# See history of a specific resource
cd ~/.local/share/ai-config/repo
git log --follow skills/pdf-processing/SKILL.md

# See what changed in that file
git log --follow -p skills/pdf-processing/SKILL.md
```

## Reverting Changes

### Undo Last Operation

```bash
cd ~/.local/share/ai-config/repo

# See what the last commit changed
git show --stat

# Revert the last commit
git revert HEAD

# Or reset to previous state (destructive!)
git reset --hard HEAD~1
```

### Restore Deleted Resource

```bash
# Find when resource was deleted
cd ~/.local/share/ai-config/repo
git log --oneline --all -- skills/old-skill

# Restore from before deletion
git checkout <commit-before-delete> -- skills/old-skill
git checkout <commit-before-delete> -- .metadata/skills/old-skill-metadata.json

# Commit the restoration
git add skills/old-skill .metadata/skills/old-skill-metadata.json
git commit -m "Restore skill: old-skill"
```

### Revert to Specific Point in Time

```bash
cd ~/.local/share/ai-config/repo

# See commits
git log --oneline

# Reset to specific commit (destructive!)
git reset --hard <commit-hash>
```

## What Gets Tracked

### Tracked Files

- ✅ **Commands** - `commands/*.md`
- ✅ **Skills** - `skills/*/SKILL.md` and all skill files
- ✅ **Agents** - `agents/*.md`
- ✅ **Packages** - `packages/*.package.json`
- ✅ **Metadata** - `.metadata/` directory (source URLs, dates, etc.)
- ✅ **Config** - `.gitignore`

### Not Tracked

- ❌ **Workspace cache** - `.workspace/` (Git clones of remote sources)
  - Excluded via `.gitignore`
  - Can be regenerated with `aimgr repo sync`

## Commit Message Format

All automated commits follow a consistent format:

```
aimgr: <operation> <details>
```

### Import Operations

```
aimgr: import 3 resource(s) (2 skills, 1 command)
aimgr: import 1 resource(s) (1 skill)
```

### Remove Operations

```
aimgr: remove skill: pdf-processing
aimgr: remove command: api/deploy
aimgr: remove agent: code-reviewer
```

### Sync Operations

```
aimgr: sync from gh:your-org/ai-tools (3 resources updated)
```

## Working with Remote Repositories

### Share Repository Across Machines

```bash
# On first machine - initialize and add remote
cd ~/.local/share/ai-config/repo
git remote add origin git@github.com:your-org/ai-config-repo.git
git push -u origin main

# On other machines - clone
git clone git@github.com:your-org/ai-config-repo.git ~/.local/share/ai-config/repo

# Configure aimgr to use this location
echo "repo:
  path: ~/.local/share/ai-config/repo" > ~/.config/aimgr/aimgr.yaml
```

### Pull Changes from Remote

```bash
cd ~/.local/share/ai-config/repo
git pull origin main

# Reinstall resources if needed
aimgr install '*'
```

### Push Local Changes

```bash
cd ~/.local/share/ai-config/repo
git push origin main
```

## Advanced Usage

### Custom Repository Location

```bash
# Set via environment variable
export AIMGR_REPO_PATH=~/custom-ai-repo
aimgr repo init

# Or via config file
echo "repo:
  path: ~/custom-ai-repo" > ~/.config/aimgr/aimgr.yaml
aimgr repo init
```

### Branching Strategy

```bash
cd ~/.local/share/ai-config/repo

# Create experimental branch
git checkout -b experimental

# Import experimental resources
aimgr repo add local:~/experimental-resources/

# Switch back to main
git checkout main

# Merge when ready
git merge experimental
```

### Tags for Milestones

```bash
cd ~/.local/share/ai-config/repo

# Tag stable states
git tag -a v1.0 -m "Stable resource set v1.0"

# List tags
git tag

# Restore to tagged state
git checkout v1.0
```

## Troubleshooting

### Operations Work Without Git

If the repository is not git-initialized, operations still work normally:

```bash
# This works even without git init
aimgr repo add local:~/resources/

# No git commits created, but resources are added
```

Git tracking is **optional** - aimgr works fine without it.

### Workspace Cache Growing

The `.workspace/` directory (not tracked) may grow over time:

```bash
# View cache size
du -sh ~/.local/share/ai-config/repo/.workspace

# Clean old cached repos
aimgr repo prune

# Or manually remove
rm -rf ~/.local/share/ai-config/repo/.workspace
```

### Merge Conflicts

If sharing across machines, you may encounter conflicts:

```bash
cd ~/.local/share/ai-config/repo
git pull origin main

# If conflicts occur
git status  # See conflicted files
# Manually resolve conflicts in files
git add <resolved-files>
git commit
```

### Re-initialize Git

If you need to start fresh:

```bash
cd ~/.local/share/ai-config/repo
rm -rf .git
aimgr repo init
```

## Best Practices

### 1. Initialize Early

```bash
# Initialize before importing resources
aimgr repo init
aimgr repo add local:~/my-resources/
```

### 2. Use Descriptive Sync Sources

```yaml
# ~/.config/aimgr/aimgr.yaml
sync:
  sources:
    - url: gh:myorg/team-resources
    - path: ~/personal-resources
```

### 3. Regular Backups

```bash
# Push to remote regularly
cd ~/.local/share/ai-config/repo
git push origin main
```

### 4. Review Before Major Changes

```bash
# Check current state
cd ~/.local/share/ai-config/repo
git status
git log --oneline -10

# Make changes
aimgr repo sync

# Verify
git show
```

### 5. Use Tags for Releases

```bash
# Tag known-good states
cd ~/.local/share/ai-config/repo
git tag -a v2.0 -m "Production-ready resource set"
```

## Integration with CI/CD

### Automated Testing

```yaml
# .github/workflows/test-resources.yml
name: Test AI Resources

on:
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Install aimgr
        run: |
          curl -sSL https://raw.githubusercontent.com/dynatrace-oss/ai-config-manager/main/install.sh | sh
      - name: Verify resources
        run: |
          aimgr repo verify
```

### Automated Sync

```yaml
# .github/workflows/sync-resources.yml
name: Sync AI Resources

on:
  schedule:
    - cron: '0 0 * * *'  # Daily

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Sync resources
        run: |
          aimgr repo sync
          git add .
          git commit -m "aimgr: automated sync $(date)"
          git push
```

## See Also

- [Sources](../user-guide/sources.md) - Configuring sync sources
- [Workspace Caching](workspace-caching.md) - Understanding `.workspace/`
- [Repository Layout](repository-layout.md) - Internal folder structure
- [Pattern Matching](../reference/pattern-matching.md) - Selecting resources
