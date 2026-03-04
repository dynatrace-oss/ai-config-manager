# AI Resources Examples

This directory contains example command, skill, and agent resources that demonstrate the correct format and serve as templates for creating your own resources.

## Resource Types

### Commands
- **Single .md file** with YAML frontmatter
- Quick, focused tasks for AI agents
- Stored in `~/.local/share/ai-config/repo/commands/`
- Example: `sample-command.md`

### Skills
- **Directory** containing `SKILL.md` plus optional subdirectories
- More complex capabilities with scripts, references, and assets
- Stored in `~/.local/share/ai-config/repo/skills/`
- Example: `sample-skill/`

### Agents
- **Single .md file** with YAML frontmatter
- AI agents with specialized roles and capabilities
- Supports both OpenCode and Claude Code formats
- Stored in `~/.local/share/ai-config/repo/agents/`
- Examples: `sample-agent.md`, `minimal-agent.md`, `claude-style-agent.md`

## Command Format

Commands follow the Claude Code slash command format.

**Minimum required frontmatter:**
```yaml
---
description: What this command does (required)
---
```

**Full example:**
```yaml
---
description: Run tests with coverage report
agent: build
model: anthropic/claude-3-5-sonnet-20241022
allowed-tools:
  - bash
  - read
  - write
---

# Command Body

Your command instructions go here.
```

**Specification:** https://code.claude.com/docs/en/slash-commands

## Skill Format

Skills follow the agentskills.io specification.

**Required structure:**
```
skill-name/
└── SKILL.md          # Required: metadata + documentation
```

**Full structure:**
```
skill-name/
├── SKILL.md          # Required: metadata + documentation
├── README.md         # Optional: additional documentation
├── scripts/          # Optional: executable scripts
│   └── *.sh
├── references/       # Optional: reference documentation
│   └── *.md
└── assets/           # Optional: images, diagrams, etc.
    └── *.png
```

**Minimum required frontmatter in SKILL.md:**
```yaml
---
name: skill-name      # Must match directory name (required)
description: What this skill does (1-1024 chars, required)
---
```

**Full example:**
```yaml
---
name: skill-name
description: What this skill does
license: MIT
metadata:
  author: your-name
  version: "1.0.0"
  tags: [example, demo]
compatibility:
  - claude-code
  - opencode
---

# Skill Documentation

Your skill documentation goes here.
```

**Specification:** https://agentskills.io/specification

## Agent Format

Agents are single `.md` files with YAML frontmatter that define AI agents with specialized roles.

**Minimum required frontmatter:**
```yaml
---
description: What this agent does (required)
---
```

**OpenCode format example:**
```yaml
---
description: Code review agent that checks for best practices
type: code-reviewer
instructions: Review code for quality, security, and performance
capabilities:
  - static-analysis
  - security-scan
  - performance-review
version: "1.0.0"
author: your-name
license: MIT
---

# Agent Documentation

Your agent details go here.
```

**Claude Code format example:**
```yaml
---
description: Test automation agent
version: "2.0.0"
author: team-name
license: Apache-2.0
metadata:
  category: testing
  tags: qa,automation
---

# Agent Documentation

Instructions and guidelines go in the markdown body for Claude format.
```

**Key Differences:**
- **OpenCode format**: Uses `type`, `instructions`, and `capabilities` in frontmatter
- **Claude format**: Puts all instructions in the markdown body, uses standard metadata fields
- Both formats require `description` field
- Choose the format that fits your workflow

**Specifications:**
- OpenCode: https://opencode.ai/docs/agents
- Claude Code: https://code.claude.com/docs/agents

## Name Validation Rules

Commands, skills, and agents must follow these naming rules:
- **Length:** 1-64 characters
- **Characters:** Lowercase letters (a-z), numbers (0-9), hyphens (-)
- **Start/End:** Must start and end with alphanumeric character
- **Consecutive hyphens:** Not allowed

**Valid examples:**
- `test`
- `run-coverage`
- `pdf-processing`
- `test-e2e`
- `skill123`
- `code-reviewer`
- `qa-tester`

**Invalid examples:**
- `Test` (uppercase)
- `test_coverage` (underscore)
- `-test` (starts with hyphen)
- `test-` (ends with hyphen)
- `test--coverage` (consecutive hyphens)
- `test@example` (special character)

## Creating Your Own Resources

### Commands

1. Create a new `.md` file with a valid name (e.g., `my-command.md`)
2. Add YAML frontmatter with at least a `description`
3. Write the command body in markdown
4. Test locally: `aimgr repo add command ./my-command.md`

### Skills

1. Create a directory with a valid name (e.g., `my-skill/`)
2. Create `SKILL.md` with frontmatter (name must match directory)
3. Add optional `scripts/`, `references/`, or `assets/` directories
4. Test locally: `aimgr repo add skill ./my-skill`

### Agents

1. Create a new `.md` file with a valid name (e.g., `my-agent.md`)
2. Add YAML frontmatter with at least a `description`
3. Choose format:
   - **OpenCode**: Add `type`, `instructions`, `capabilities` to frontmatter
   - **Claude**: Put instructions in markdown body
4. Write agent documentation in markdown
5. Test locally: `aimgr repo add agent ./my-agent.md`

## Testing Before Adding

### Validate a command:
```bash
# Add to repository - validation happens automatically (type auto-detected)
aimgr repo add local:./my-command.md
```

### Validate a skill:
```bash
# Check that SKILL.md exists and is valid
cat my-skill/SKILL.md

# Verify name matches directory
ls -d my-skill/

# Add to repository - validation happens automatically (type auto-detected)
aimgr repo add local:./my-skill
```

## Using the Examples

### Add examples to your repository:
```bash
# Add all example resources at once (auto-discovery)
aimgr repo add examples/

# Or add individually (type auto-detected)
aimgr repo add examples/sample-command.md
aimgr repo add examples/sample-skill
aimgr repo add examples/sample-agent.md
aimgr repo add examples/minimal-agent.md
aimgr repo add examples/claude-style-agent.md

# List to verify
aimgr repo list
```

### Install in a project:
```bash
cd your-project/

# Install the command
aimgr install command/sample-command

# Install the skill
aimgr install skill/sample-skill

# Install an agent
aimgr install agent/sample-agent

# Install multiple resources at once
aimgr install command/sample-command skill/sample-skill agent/sample-agent

# Or install all sample resources with a pattern
aimgr install "*/sample*"

# Verify installation
ls -la .claude/commands/
ls -la .claude/skills/
ls -la .claude/agents/
```

## Resources

- **Claude Code Commands:** https://code.claude.com/docs/en/slash-commands
- **Claude Code Agents:** https://code.claude.com/docs/agents
- **OpenCode Agents:** https://opencode.ai/docs/agents
- **AgentSkills Specification:** https://agentskills.io/specification
- **aimgr Documentation:** ../README.md
- **Migration Guide:** ../MIGRATION.md (for users migrating from ai-repo)

## Contributing

If you create useful commands or skills, consider:
1. Testing them thoroughly
2. Adding clear documentation
3. Following the naming conventions
4. Sharing with the community
