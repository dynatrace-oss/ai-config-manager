# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.4.x   | :white_check_mark: |
| 1.3.x   | :white_check_mark: |
| 1.2.x   | :x:                |
| 1.1.x   | :x:                |
| 1.0.x   | :x:                |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of `aimgr` seriously. If you discover a security vulnerability, please follow these steps:

### How to Report

**Create a GitHub Security Advisory:** Use the [GitHub Security Advisory](https://github.com/dynatrace-oss/ai-config-manager/security/advisories/new) feature for private disclosure.

**Please include:**
- A description of the vulnerability
- Steps to reproduce the issue
- Potential impact of the vulnerability
- Any suggested fixes (if applicable)

### Security Advisory Process

1. **Confirmation:** We validate the reported vulnerability
2. **Fix Development:** A patch is developed and tested
3. **Coordinated Disclosure:** We coordinate the release timing with you
4. **Public Disclosure:** A security advisory is published with the fix
5. **CVE Assignment:** Critical vulnerabilities receive a CVE identifier

## Security Best Practices

When using `aimgr`, follow these best practices to maintain security:

### Safe Resource Management

**⚠️ CRITICAL: Prompt Injection Vulnerability**

AI resources (commands, skills, agents) are markdown files read by LLMs. **These files are inherently vulnerable to prompt injection attacks**, and there is no technical solution to prevent this.

**You MUST manually review all markdown content from untrusted sources before adding them to your repository.**

1. **Verify Sources Before Adding:**
   ```bash
   # Review repository contents before importing
   aimgr repo add gh:owner/repo --dry-run
   
   # Check resource details before installing
   aimgr repo describe skill resource-name
   ```

2. **Use Trusted Sources:**
   - Only add resources from repositories you trust
   - **Always review resource content before installation**
   - Be cautious with resources that execute system commands

3. **Regularly Update Resources:**
   ```bash
   # Keep your resources up-to-date
   aimgr repo sync
   ```

**The repository model assumes all content is reviewed and trusted. Be extremely careful when adding new resources.**

### Command Execution Safety

`aimgr` commands and skills may contain shell commands that are executed by AI tools. Follow these guidelines:

1. **Review Command Content:**
   - Always review command files before installation
   - Commands are markdown files with embedded shell scripts
   - Look for potentially dangerous commands (rm, dd, curl | bash, etc.)

2. **Skills with Custom Scripts:**
   - Skills may include scripts in the `scripts/` directory
   - Review all scripts before installing skills
   - Be especially cautious with skills that modify system files

3. **Agents with System Access:**
   - Agents may have extensive system access depending on the AI tool
   - Only install agents from sources you completely trust
   - Review agent capabilities in their metadata

### Package Security

When using packages (collections of resources):

1. **Review Package Contents:**
   ```bash
   # Check what resources a package includes
   aimgr repo describe package package-name
   ```

2. **Verify All Resources:**
   - A package is only as secure as its least secure resource
   - Review each resource referenced by the package

### Environment Variables

1. **Never Store Secrets in Plain Text:**
   - Do not put API keys, passwords, or tokens in configuration files
   - Use environment variables or secure credential storage

2. **Validate Environment Variables:**
   - Sanitize user input when using environment variables in commands
   - Be aware of environment variable injection risks

## Known Security Considerations

### Symlink Attacks

- `aimgr` creates symlinks from projects to the central repository
- Symlinks are created with proper path validation
- The tool checks for existing files/directories before creating symlinks
- Broken symlinks are handled gracefully

### Path Traversal

- All file paths are normalized and validated
- Path traversal attempts (../, ..\, etc.) are blocked
- Resources cannot be installed outside designated directories

### Arbitrary Code Execution

- Resources (commands, skills, agents) may contain code that is executed by AI tools
- `aimgr` itself does not execute resource content
- Users are responsible for reviewing resource content before installation
- We recommend using code review practices for resources from untrusted sources

## Security Updates

Security updates are released as soon as possible after a vulnerability is confirmed. Update methods:

```bash
# Update via go install
go install github.com/dynatrace-oss/ai-config-manager/v3/cmd/aimgr@latest

# Or rebuild from source
cd ai-config-manager
git pull
make install
```

Subscribe to releases on GitHub to be notified of security updates:
https://github.com/dynatrace-oss/ai-config-manager/releases

Thank you for helping keep `aimgr` and its users safe!
