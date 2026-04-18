# Contributing to aimgr

## Quick Start

### Prerequisites

- **Go 1.25.6+** (or use [mise](https://mise.jdx.dev/) for automatic version management)
- **Make**
- **Git**

### Clone, build, and test

```bash
git clone https://github.com/dynatrace-oss/ai-config-manager.git
cd ai-config-manager

make build

# Baseline contributor checks
make test
```

If your local install path matters, run `make os-info` before `make install`.
For PATH setup, IDE notes, and longer environment guidance, use [docs/contributor-guide/development-environment.md](docs/contributor-guide/development-environment.md).

## Where guidance lives

Use this file as the contributor front door, then switch to the focused docs for repository-local rules.

| Need | Read |
| --- | --- |
| Repository architecture and repo map | [docs/OVERVIEW.md](docs/OVERVIEW.md) |
| Implementation constraints and safety rules | [docs/CODING.md](docs/CODING.md) |
| Minimum checks and test-layer policy | [docs/TESTING.md](docs/TESTING.md) |
| Branch, commit, push, PR, and merge workflow | [docs/CHANGE-WORKFLOW.md](docs/CHANGE-WORKFLOW.md) |
| Longer setup/examples/background | [docs/contributor-guide/README.md](docs/contributor-guide/README.md) |

## Contributor flow

1. Choose the landing path in [docs/CHANGE-WORKFLOW.md](docs/CHANGE-WORKFLOW.md).
2. Use [docs/CODING.md](docs/CODING.md) while implementing repository-local changes.
3. Run the minimum checks from [docs/TESTING.md](docs/TESTING.md).
4. Update docs in the same change when commands, paths, workflows, or user-facing behavior changed.
5. Commit with the conventional commit format below.

## Before submitting

- [ ] Minimum checks from [docs/TESTING.md](docs/TESTING.md) passed for the change type
- [ ] New code has tests when behavior changed
- [ ] Documentation updated when commands, workflows, or user-facing behavior changed
- [ ] Commit messages follow the format below

## Commit Message Format

Use conventional commits:

```
type(scope): short description

Longer description if needed, explaining what and why.

Fixes #issue-number
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Formatting, missing semicolons, etc.
- `refactor`: Code restructuring
- `test`: Adding tests
- `chore`: Maintenance, dependencies, etc.

**Examples:**

```
feat(repo): add bulk import support for plugins

Add ability to import multiple commands and skills from Claude plugins
in a single operation.

Fixes #42
```

```
fix(install): handle symlink creation on Windows

Use junction points instead of symlinks for Windows compatibility.
```

## Focused repo docs

Use the focused project docs when you need repository-local detail:

- [docs/OVERVIEW.md](docs/OVERVIEW.md) - architecture map and where common work starts
- [docs/CODING.md](docs/CODING.md) - implementation constraints, build commands, and safety rules
- [docs/TESTING.md](docs/TESTING.md) - test selection, isolation rules, and minimum checks
- [docs/CHANGE-WORKFLOW.md](docs/CHANGE-WORKFLOW.md) - commit, push, branch, PR, and merge expectations
- [docs/contributor-guide/README.md](docs/contributor-guide/README.md) - deeper contributor references for setup, architecture, and test authoring

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/dynatrace-oss/ai-config-manager/issues)
- **Discussions**: [GitHub Discussions](https://github.com/dynatrace-oss/ai-config-manager/discussions)
- **Documentation**: 
  - User docs: [README.md](README.md)
  - AI agent guide: [AGENTS.md](AGENTS.md)
  - Contributor docs: [docs/contributor-guide/](docs/contributor-guide/)

---

Thank you for contributing to aimgr! 🎉
